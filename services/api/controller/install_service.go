package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type InstallController struct {
	serviceLifecycleUc ports.ServiceLifecycle
	userGetter         usercontext.UserGetter
}

func NewInstallController(
	serviceLifecycleUc ports.ServiceLifecycle,
	userGetter usercontext.UserGetter,
) *InstallController {
	return &InstallController{
		serviceLifecycleUc: serviceLifecycleUc,
		userGetter:         userGetter,
	}
}

func (ic *InstallController) SetServiceSuspended(
	ctx context.Context,
	req *api.SetServiceSuspendedReq,
	params api.SetServiceSuspendedParams,
) (api.SetServiceSuspendedRes, error) {
	user, err := callerFrom(ctx, ic.userGetter)
	if err != nil {
		return nil, err
	}

	if req.Suspended {
		suspendReq := domain.SuspendRequest{
			User:        user,
			ReleaseName: params.ReleaseId,
			Namespace:   params.XOnyxiaProject,
		}
		if err := ic.serviceLifecycleUc.Suspend(ctx, suspendReq); err != nil {
			slog.ErrorContext(ctx, "suspend failed", slog.Any("error", err))
			return nil, err
		}
	} else {
		resumeReq := domain.ResumeRequest{
			User:        user,
			ReleaseName: params.ReleaseId,
			Namespace:   params.XOnyxiaProject,
		}
		if err := ic.serviceLifecycleUc.Resume(ctx, resumeReq); err != nil {
			slog.ErrorContext(ctx, "resume failed", slog.Any("error", err))
			return nil, err
		}
	}

	return &api.SetServiceSuspendedNoContent{}, nil
}

func (ic *InstallController) SetServiceShared(
	ctx context.Context,
	req *api.SetServiceSharedReq,
	params api.SetServiceSharedParams,
) (api.SetServiceSharedRes, error) {
	user, err := callerFrom(ctx, ic.userGetter)
	if err != nil {
		return nil, err
	}

	if err := ic.serviceLifecycleUc.SetShared(ctx, domain.SetSharedRequest{
		User:        user,
		ReleaseName: params.ReleaseId,
		Namespace:   params.XOnyxiaProject,
		Shared:      req.Shared,
	}); err != nil {
		slog.ErrorContext(ctx, "set shared failed", slog.Any("error", err))
		return nil, err
	}

	return &api.SetServiceSharedNoContent{}, nil
}

func (ic *InstallController) DeleteService(
	ctx context.Context,
	params api.DeleteServiceParams,
) (api.DeleteServiceRes, error) {
	user, err := callerFrom(ctx, ic.userGetter)
	if err != nil {
		return nil, err
	}

	req := domain.DeleteRequest{
		User:        user,
		ReleaseName: params.ReleaseId,
		Namespace:   params.XOnyxiaProject,
	}

	if err := ic.serviceLifecycleUc.Delete(ctx, req); err != nil {
		slog.ErrorContext(ctx, "delete failed", slog.Any("error", err))
		return nil, err
	}

	return &api.DeleteServiceNoContent{}, nil
}

func (ic *InstallController) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	params api.InstallServiceParams,
) (api.InstallServiceRes, error) {

	user, err := callerFrom(ctx, ic.userGetter)
	if err != nil {
		return nil, err
	}

	if req == nil {
		return nil, fmt.Errorf("%w: request body is required", domain.ErrInvalidInput)
	}
	if req.PackageName == "" {
		return nil, fmt.Errorf("%w: packageName is required", domain.ErrInvalidInput)
	}
	if req.CatalogId == "" {
		return nil, fmt.Errorf("%w: catalogId is required", domain.ErrInvalidInput)
	}
	if req.Options == nil {
		return nil, fmt.Errorf("%w: options are required", domain.ErrInvalidInput)
	}

	values := make(map[string]interface{}, len(req.Options))

	for k, raw := range req.Options {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("%w: unmarshal values[%q]: %s", domain.ErrInvalidInput, k, err)
		}
		values[k] = v
	}

	dreq := domain.StartRequest{
		User:         user,
		CatalogID:    req.CatalogId,
		PackageName:  req.PackageName,
		Version:      req.PackageVersion.Or(""),
		ReleaseID:    params.ReleaseId,
		Namespace:    params.XOnyxiaProject,
		FriendlyName: req.FriendlyName.Or(req.PackageName),
		Share:        req.Share.Or(false),
		Values:       values,
	}

	// Execute use case.
	if err := ic.serviceLifecycleUc.Start(ctx, dreq); err != nil {
		slog.ErrorContext(ctx, "install failed", slog.Any("error", err))
		return nil, err
	}

	return &api.InstallServiceAccepted{
		Location: api.NewOptString("/api/services/" + params.ReleaseId),
	}, nil
}
