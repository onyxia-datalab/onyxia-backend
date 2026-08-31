package controller

import (
	"context"
	"errors"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type ServiceQueryController struct {
	serviceQuery domain.ServiceQuery
	userGetter   usercontext.UserGetter
}

func NewServiceQueryController(
	serviceQuery domain.ServiceQuery,
	userGetter usercontext.UserGetter,
) *ServiceQueryController {
	return &ServiceQueryController{serviceQuery: serviceQuery, userGetter: userGetter}
}

func (c *ServiceQueryController) GetService(
	ctx context.Context,
	params api.GetServiceParams,
) (api.GetServiceRes, error) {
	u, ok := c.userGetter.GetUser(ctx)
	if !ok || u == nil {
		return &api.GetServiceForbidden{}, errors.New("user not found")
	}

	svc, err := c.serviceQuery.GetService(ctx, params.XOnyxiaProject, params.ReleaseId)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return &api.GetServiceNotFound{}, nil
		}
		slog.ErrorContext(ctx, "get service failed", slog.Any("error", err))
		return &api.GetServiceInternalServerError{}, err
	}

	return toAPIService(svc), nil
}

func (c *ServiceQueryController) ListServices(
	ctx context.Context,
	params api.ListServicesParams,
) (api.ListServicesRes, error) {
	u, ok := c.userGetter.GetUser(ctx)
	if !ok || u == nil {
		return &api.ListServicesForbidden{}, errors.New("user not found")
	}

	svcs, err := c.serviceQuery.ListServices(ctx, params.XOnyxiaProject)
	if err != nil {
		slog.ErrorContext(ctx, "list services failed", slog.Any("error", err))
		return &api.ListServicesInternalServerError{}, err
	}

	result := make(api.ListServicesOKApplicationJSON, 0, len(svcs))
	for _, svc := range svcs {
		result = append(result, *toAPIService(svc))
	}
	return &result, nil
}

// toAPIService maps a domain.Service to the generated api.Service type.
func toAPIService(svc domain.Service) *api.Service {
	out := &api.Service{
		ReleaseId:    svc.ReleaseID,
		Status:       toAPIStatus(svc.Status),
		FriendlyName: svc.FriendlyName,
		Owner:        svc.Owner,
		CatalogId:    svc.CatalogID,
		Share:        svc.Share,
	}

	if svc.Error != nil {
		out.Error = api.NewOptServiceError(api.ServiceError{
			Reason:       toAPIErrorReason(svc.Error.Reason),
			PodName:      svc.Error.PodName,
			Message:      api.NewOptString(svc.Error.Message),
			RestartCount: api.NewOptInt(int(svc.Error.RestartCount)),
			ExitCode:     api.NewOptInt(int(svc.Error.ExitCode)),
			Image:        api.NewOptString(svc.Error.Image),
			Limit:        api.NewOptString(svc.Error.Limit),
		})
	}

	return out
}

func toAPIStatus(s domain.ServiceStatus) api.ServiceStatus {
	switch s {
	case domain.ServiceStatusDeploying:
		return api.ServiceStatusDeploying
	case domain.ServiceStatusRunning:
		return api.ServiceStatusRunning
	case domain.ServiceStatusError:
		return api.ServiceStatusError
	case domain.ServiceStatusGhost:
		return api.ServiceStatusGhost
	case domain.ServiceStatusSuspended:
		return api.ServiceStatusSuspended
	case domain.ServiceStatusTerminating:
		return api.ServiceStatusTerminating
	default:
		return api.ServiceStatusDeploying
	}
}

func toAPIErrorReason(r domain.ServiceErrorReason) api.ServiceErrorReason {
	switch r {
	case domain.ServiceErrorReasonCrashLoop:
		return api.ServiceErrorReasonCrashLoop
	case domain.ServiceErrorReasonOOMKilled:
		return api.ServiceErrorReasonOomKilled
	case domain.ServiceErrorReasonImagePull:
		return api.ServiceErrorReasonImagePull
	case domain.ServiceErrorReasonConfigError:
		return api.ServiceErrorReasonConfigError
	case domain.ServiceErrorReasonUnschedulable:
		return api.ServiceErrorReasonUnschedulable
	case domain.ServiceErrorReasonReadinessFailed:
		return api.ServiceErrorReasonReadinessFailed
	default:
		return api.ServiceErrorReasonCrashLoop
	}
}
