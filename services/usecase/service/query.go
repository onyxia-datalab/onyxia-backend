package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type Query struct {
	secrets    ports.OnyxiaSecretGateway
	helm       ports.ReleaseGateway
	pods       ports.WorkloadStateGateway
	userReader usercontext.UsernameGetter
}

var _ domain.ServiceQuery = (*Query)(nil)

func NewQuery(
	secrets ports.OnyxiaSecretGateway,
	helm ports.ReleaseGateway,
	pods ports.WorkloadStateGateway,
	userReader usercontext.UsernameGetter,
) *Query {
	return &Query{secrets: secrets, helm: helm, pods: pods, userReader: userReader}
}

// GetService returns the full service state including error details.
func (uc *Query) GetService(
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

	status, svcErr, err := uc.deriveStatusWithDetail(ctx, namespace, releaseID)
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
		Error:        svcErr,
	}, nil
}

// ListServices returns services visible to the current user: owned by them or shared.
// Pod queries are skipped — status is derived from the Helm release state only.
func (uc *Query) ListServices(
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
		if svc.Owner != username && !svc.Share {
			continue
		}
		services = append(services, svc)
	}
	return services, nil
}

// buildLightService builds a service entry for the list — no pod query, no error detail.
func (uc *Query) buildLightService(
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
func (uc *Query) deriveStatusLight(
	ctx context.Context,
	namespace, releaseID string,
	releaseState ports.ReleaseState,
) (domain.ServiceStatus, error) {
	if releaseState.Status != "deployed" {
		return deriveStatusFromHelm(releaseState), nil
	}
	ready, err := uc.pods.GetWorkloadReadiness(ctx, namespace, releaseID)
	if err != nil {
		return "", err
	}
	if ready {
		return domain.ServiceStatusRunning, nil
	}
	return domain.ServiceStatusDeploying, nil
}

// deriveStatusWithDetail applies the full 6-step derivation including pod queries.
func (uc *Query) deriveStatusWithDetail(
	ctx context.Context,
	namespace, releaseID string,
) (domain.ServiceStatus, *domain.ServiceError, error) {
	releaseState, err := uc.helm.GetReleaseState(ctx, namespace, releaseID)
	if err != nil {
		return "", nil, err
	}

	if !releaseState.Exists {
		return domain.ServiceStatusGhost, nil, nil
	}

	if releaseState.Suspended {
		return domain.ServiceStatusSuspended, nil, nil
	}

	podInfos, err := uc.pods.GetPodsForRelease(ctx, namespace, releaseID)
	if err != nil {
		return "", nil, err
	}

	status, svcErr := derivePodStatus(podInfos)
	return status, svcErr, nil
}
