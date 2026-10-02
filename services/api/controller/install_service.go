package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
)

const userNotFoundInContextMessage = "user not found in context"

type InstallController struct {
	serviceLifecycleUc domain.ServiceLifecycle
	userGetter         usercontext.UserGetter
	namespaceAuthz     namespace.Authorizer
}

func NewInstallController(
	serviceLifecycleUc domain.ServiceLifecycle,
	userGetter usercontext.UserGetter,
	namespaceAuthz namespace.Authorizer,
) *InstallController {
	return &InstallController{
		serviceLifecycleUc: serviceLifecycleUc,
		userGetter:         userGetter,
		namespaceAuthz:     namespaceAuthz,
	}
}

func (ic *InstallController) SetServiceSuspended(
	ctx context.Context,
	req *api.SetServiceSuspendedReq,
	params api.SetServiceSuspendedParams,
) (api.SetServiceSuspendedRes, error) {
	u, ok := ic.userGetter.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, userNotFoundInContextMessage)
		return nil, domain.ErrForbidden
	}

	if !ic.namespaceAuthz.Allowed(u.Username, u.Groups, params.XOnyxiaProject) {
		slog.ErrorContext(ctx, "namespace access denied",
			slog.String("namespace", params.XOnyxiaProject),
			slog.String("username", u.Username),
		)
		return nil, domain.ErrForbidden
	}

	if req.Suspended {
		suspendReq := domain.SuspendRequest{
			Username:    u.Username,
			ReleaseName: params.ReleaseId,
			Namespace:   params.XOnyxiaProject,
		}
		if err := ic.serviceLifecycleUc.Suspend(ctx, suspendReq); err != nil {
			slog.ErrorContext(ctx, "suspend failed", slog.Any("error", err))
			return nil, err
		}
	} else {
		resumeReq := domain.ResumeRequest{
			Username:    u.Username,
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
	u, ok := ic.userGetter.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, userNotFoundInContextMessage)
		return nil, domain.ErrForbidden
	}

	if !ic.namespaceAuthz.Allowed(u.Username, u.Groups, params.XOnyxiaProject) {
		slog.ErrorContext(ctx, "namespace access denied",
			slog.String("namespace", params.XOnyxiaProject),
			slog.String("username", u.Username),
		)
		return nil, domain.ErrForbidden
	}

	if err := ic.serviceLifecycleUc.SetShared(ctx, domain.SetSharedRequest{
		Username:    u.Username,
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
	u, ok := ic.userGetter.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, userNotFoundInContextMessage)
		return nil, domain.ErrForbidden
	}

	if !ic.namespaceAuthz.Allowed(u.Username, u.Groups, params.XOnyxiaProject) {
		slog.ErrorContext(ctx, "namespace access denied",
			slog.String("namespace", params.XOnyxiaProject),
			slog.String("username", u.Username),
		)
		return nil, domain.ErrForbidden
	}

	req := domain.DeleteRequest{
		Username:    u.Username,
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

	u, ok := ic.userGetter.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, userNotFoundInContextMessage)
		return nil, domain.ErrForbidden
	}

	if !ic.namespaceAuthz.Allowed(u.Username, u.Groups, params.XOnyxiaProject) {
		slog.ErrorContext(ctx, "namespace access denied",
			slog.String("namespace", params.XOnyxiaProject),
			slog.String("username", u.Username),
		)
		return nil, domain.ErrForbidden
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

	version := req.PackageVersion.Or(req.Version.Or(""))

	values := make(map[string]interface{}, len(req.Options))

	for k, raw := range req.Options {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("%w: unmarshal values[%q]: %s", domain.ErrInvalidInput, k, err)
		}
		values[k] = v
	}

	dreq := domain.StartRequest{
		Username:     u.Username,
		CatalogID:    req.CatalogId,
		PackageName:  req.PackageName,
		Name:         req.Name,
		Version:      version,
		ReleaseID:    params.ReleaseId,
		Namespace:    params.XOnyxiaProject,
		FriendlyName: req.FriendlyName.Or(req.PackageName),
		Share:        req.Share.Or(false),
		Values:       values,
	}

	// Execute use case.
	_, err := ic.serviceLifecycleUc.Start(ctx, dreq)

	if err != nil {
		slog.ErrorContext(ctx, "install failed", slog.Any("error", err))
		if errors.Is(err, domain.ErrNotFound) {
			// installService has no 404 response in the spec: a missing or
			// restricted-but-hidden catalog/package is reported as a bad
			// request instead.
			err = fmt.Errorf("%w: %s", domain.ErrInvalidInput, err)
		}
		return nil, err
	}

	// Success: 202 Accepted + headers/body per ogen schema.
	return &api.InstallAcceptedHeaders{
		Location: api.NewOptString(""),
		Response: api.InstallAccepted{
			EventsUrl: api.InstallAcceptedEventsUrl{
				Release:   "",
				Resources: "",
			},
		},
	}, nil
}
