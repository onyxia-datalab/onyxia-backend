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
	secrets    ports.OnyxiaSecretGateway
	helm       ports.ReleaseGateway
	pods       ports.WorkloadStateGateway
	userReader usercontext.UsernameGetter
	namespaces namespace.Authorizer
}

var _ ports.ServiceQuery = (*Reader)(nil)

func NewReader(
	secrets ports.OnyxiaSecretGateway,
	helm ports.ReleaseGateway,
	pods ports.WorkloadStateGateway,
	userReader usercontext.UsernameGetter,
	namespaces namespace.Authorizer,
) *Reader {
	return &Reader{
		secrets:    secrets,
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

	secretData, err := uc.secrets.ReadOnyxiaSecretData(ctx, namespace, releaseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Service{}, domain.ErrNotFound
		}
		return domain.Service{}, fmt.Errorf("read secret: %w", err)
	}

	owner := string(secretData["owner"])
	share := string(secretData["share"]) == "true"
	if !uc.namespaces.CanAccessService(username, namespace, owner, share) {
		return domain.Service{}, domain.ErrNotFound
	}

	status, svcErr, err := uc.deriveStatusWithDetail(ctx, namespace, releaseID)
	if err != nil {
		return domain.Service{}, fmt.Errorf("derive status: %w", err)
	}

	return domain.Service{
		ReleaseID:    releaseID,
		Namespace:    namespace,
		FriendlyName: string(secretData["friendlyName"]),
		Owner:        owner,
		CatalogID:    string(secretData["catalog"]),
		Share:        share,
		Status:       status,
		Error:        svcErr,
	}, nil
}

// ListServices returns services visible to the current user
// (see namespace.Authorizer.CanAccessService).
// Pod queries are skipped — status is derived from the Helm release state only.
func (uc *Reader) ListServices(
	ctx context.Context,
	namespace string,
) ([]domain.Service, error) {
	username, _ := uc.userReader.GetUsername(ctx)

	releaseIDs, err := uc.secrets.ListOnyxiaSecretNames(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	services := make([]domain.Service, 0, len(releaseIDs))
	for _, id := range releaseIDs {
		svc, err := uc.buildLightService(ctx, namespace, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				// Secret disappeared between list and get — skip.
				continue
			}
			return nil, err
		}
		if !uc.namespaces.CanAccessService(username, namespace, svc.Owner, svc.Share) {
			continue
		}
		services = append(services, svc)
	}
	return services, nil
}

// buildLightService builds a service entry for the list — no pod query, no error detail.
func (uc *Reader) buildLightService(
	ctx context.Context,
	namespace, releaseID string,
) (domain.Service, error) {
	secretData, err := uc.secrets.ReadOnyxiaSecretData(ctx, namespace, releaseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Service{}, domain.ErrNotFound
		}
		return domain.Service{}, fmt.Errorf("read secret: %w", err)
	}

	releaseState, err := uc.helm.GetReleaseState(ctx, namespace, releaseID)
	if err != nil {
		return domain.Service{}, fmt.Errorf("get release state: %w", err)
	}

	status, err := uc.deriveStatusLight(ctx, namespace, releaseID, releaseState)
	if err != nil {
		return domain.Service{}, fmt.Errorf("derive status: %w", err)
	}

	return domain.Service{
		ReleaseID:    releaseID,
		Namespace:    namespace,
		FriendlyName: string(secretData["friendlyName"]),
		Owner:        string(secretData["owner"]),
		CatalogID:    string(secretData["catalog"]),
		Share:        string(secretData["share"]) == "true",
		Status:       status,
	}, nil
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
