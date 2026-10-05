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
	userReader usercontext.UsernameGetter
	namespaces namespace.Authorizer
}

var _ ports.ServiceQuery = (*Reader)(nil)

func NewReader(
	records ports.ServiceRecordGateway,
	helm ports.ReleaseGateway,
	pods ports.WorkloadStateGateway,
	userReader usercontext.UsernameGetter,
	namespaces namespace.Authorizer,
) *Reader {
	return &Reader{
		records:    records,
		helm:       helm,
		pods:       pods,
		userReader: userReader,
		namespaces: namespaces,
	}
}

// GetService returns the full service state including error details.
// It applies the same visibility rule as ListServices: a service the caller
// cannot see is reported as ErrNotFound, so a direct GetService call can't
// be used to bypass the filtering ListServices already applies.
func (uc *Reader) GetService(
	ctx context.Context,
	namespace, releaseID string,
) (domain.Service, error) {
	username, _ := uc.userReader.GetUsername(ctx)

	rec, err := uc.records.GetServiceRecord(ctx, namespace, releaseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Service{}, domain.ErrNotFound
		}
		return domain.Service{}, fmt.Errorf("read service record: %w", err)
	}

	if !uc.namespaces.CanAccessService(username, namespace, rec.Owner, rec.Share) {
		return domain.Service{}, domain.ErrNotFound
	}

	status, svcErr, err := uc.deriveStatusWithDetail(ctx, namespace, releaseID)
	if err != nil {
		return domain.Service{}, fmt.Errorf("derive status: %w", err)
	}

	svc := toService(namespace, rec, status)
	svc.Error = svcErr
	return svc, nil
}

// ListServices returns services visible to the current user
// (see namespace.Authorizer.CanAccessService).
// Pod queries are skipped — status is derived from the release state and the
// workload controllers only.
func (uc *Reader) ListServices(
	ctx context.Context,
	namespace string,
) ([]domain.Service, error) {
	username, _ := uc.userReader.GetUsername(ctx)

	records, err := uc.records.ListServiceRecords(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list service records: %w", err)
	}

	services := make([]domain.Service, 0, len(records))
	for _, rec := range records {
		if !uc.namespaces.CanAccessService(username, namespace, rec.Owner, rec.Share) {
			continue
		}
		releaseState, err := uc.helm.GetReleaseState(ctx, namespace, rec.ReleaseID)
		if err != nil {
			return nil, fmt.Errorf("get release state: %w", err)
		}
		status, err := uc.deriveStatusLight(ctx, namespace, rec.ReleaseID, releaseState)
		if err != nil {
			return nil, fmt.Errorf("derive status: %w", err)
		}
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

// deriveStatusLight maps a Helm release state to a ServiceStatus using the workload
// controller status when deployed — no pod listing.
func (uc *Reader) deriveStatusLight(
	ctx context.Context,
	namespace, releaseID string,
	releaseState ports.ReleaseState,
) (domain.ServiceStatus, error) {
	if status, decided := statusFromRelease(releaseState); decided {
		return status, nil
	}
	resources, err := uc.helm.GetReleaseResources(ctx, namespace, releaseID)
	if err != nil {
		return "", fmt.Errorf("get release resources: %w", err)
	}
	ready, err := uc.pods.GetControllerReadiness(ctx, namespace, resources)
	if err != nil {
		return "", err
	}
	if ready {
		return domain.ServiceStatusRunning, nil
	}
	return domain.ServiceStatusDeploying, nil
}

// deriveStatusWithDetail applies the full derivation including pod queries.
func (uc *Reader) deriveStatusWithDetail(
	ctx context.Context,
	namespace, releaseID string,
) (domain.ServiceStatus, *domain.ServiceError, error) {
	releaseState, err := uc.helm.GetReleaseState(ctx, namespace, releaseID)
	if err != nil {
		return "", nil, err
	}

	if status, decided := statusFromRelease(releaseState); decided {
		return status, nil, nil
	}

	podInfos, err := uc.pods.GetPodsForRelease(ctx, namespace, releaseID)
	if err != nil {
		return "", nil, err
	}

	status, svcErr := derivePodStatus(podInfos)
	return status, svcErr, nil
}
