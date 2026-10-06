package query

import (
	"context"
	"errors"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
)

type Reader struct {
	records    ports.ServiceRecordGateway
	helm       ports.ReleaseGateway
	pods       ports.WorkloadStateGateway
	namespaces namespace.Authorizer
}

var _ ports.ServiceQuery = (*Reader)(nil)

func NewReader(
	records ports.ServiceRecordGateway,
	helm ports.ReleaseGateway,
	pods ports.WorkloadStateGateway,
	namespaces namespace.Authorizer,
) *Reader {
	return &Reader{
		records:    records,
		helm:       helm,
		pods:       pods,
		namespaces: namespaces,
	}
}

// GetService returns the full service state including error details.
// It applies the same visibility rule as ListServices: a service the caller
// cannot see is reported as ErrNotFound, so a direct GetService call can't
// be used to bypass the filtering ListServices already applies.
func (uc *Reader) GetService(
	ctx context.Context,
	user usercontext.User,
	namespace, releaseID string,
) (domain.Service, error) {
	if err := uc.namespaces.Check(user, namespace); err != nil {
		return domain.Service{}, err
	}

	rec, err := uc.records.GetServiceRecord(ctx, namespace, releaseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Service{}, domain.ErrNotFound
		}
		return domain.Service{}, fmt.Errorf("read service record: %w", err)
	}

	if !uc.namespaces.CanAccessService(user.Username, namespace, rec.Owner, rec.Share) {
		return domain.Service{}, domain.ErrNotFound
	}

	release, err := uc.helm.GetReleaseState(ctx, namespace, releaseID)
	if err != nil {
		return domain.Service{}, fmt.Errorf("get release state: %w", err)
	}
	var pods []ports.PodInfo
	if needsPods(release) {
		if pods, err = uc.pods.GetPodsForRelease(ctx, namespace, releaseID); err != nil {
			return domain.Service{}, fmt.Errorf("get pods: %w", err)
		}
	}

	status, svcErr := deriveStatus(release, pods)

	svc := toService(namespace, rec, status)
	svc.Error = svcErr
	return svc, nil
}

// ListServices returns services visible to the current user
// (see namespace.Authorizer.CanAccessService), with the same status
// GetService derives but without the error detail.
// The number of calls to the cluster does not depend on the number of
// services: one list of the records, one of the releases and, if any service
// needs it, one of the pods.
func (uc *Reader) ListServices(
	ctx context.Context,
	user usercontext.User,
	namespace string,
) ([]domain.Service, error) {
	if err := uc.namespaces.Check(user, namespace); err != nil {
		return nil, err
	}

	records, err := uc.records.ListServiceRecords(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list service records: %w", err)
	}

	visible := make([]ports.ServiceRecord, 0, len(records))
	for _, rec := range records {
		if uc.namespaces.CanAccessService(user.Username, namespace, rec.Owner, rec.Share) {
			visible = append(visible, rec)
		}
	}
	if len(visible) == 0 {
		return []domain.Service{}, nil
	}

	releases, err := uc.helm.ListReleaseStates(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list release states: %w", err)
	}

	var podsByRelease map[string][]ports.PodInfo
	podsFetched := false
	services := make([]domain.Service, 0, len(visible))
	for _, rec := range visible {
		// A record without a release yields the zero state: a Ghost.
		release := releases[rec.ReleaseID]
		if needsPods(release) && !podsFetched {
			if podsByRelease, err = uc.pods.ListPodsByRelease(ctx, namespace); err != nil {
				return nil, fmt.Errorf("list pods: %w", err)
			}
			podsFetched = true
		}
		// The list carries the status alone; the detail is GetService's.
		status, _ := deriveStatus(release, podsByRelease[rec.ReleaseID])
		services = append(services, toService(namespace, rec, status))
	}
	return services, nil
}

func toService(namespace string, rec ports.ServiceRecord, status domain.ServiceStatus) domain.Service {
	return domain.Service{
		ReleaseID:    rec.ReleaseID,
		Namespace:    namespace,
		FriendlyName: rec.FriendlyName,
		Owner:        rec.Owner,
		CatalogID:    rec.CatalogID,
		Share:        rec.Share,
		Status:       status,
	}
}
