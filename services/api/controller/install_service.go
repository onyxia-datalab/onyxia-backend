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
)

type InstallController struct {
	serviceLifecycleUc domain.ServiceLifecycle
	userGetter         usercontext.UserGetter
}

func NewInstallController(
	serviceLifecycleUc domain.ServiceLifecycle,
	userGetter usercontext.UserGetter,
) *InstallController {
	return &InstallController{
		serviceLifecycleUc: serviceLifecycleUc,
		userGetter:         userGetter,
	}
}

func (ic *InstallController) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	params api.InstallServiceParams,
) (api.InstallServiceRes, error) {

	u, ok := ic.userGetter.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, "user not found in context")
		return &api.InstallServiceForbidden{}, errors.New("user not found")
	}

	if req == nil {
		return &api.InstallServiceBadRequest{}, errors.New("request body is required")
	}
	if req.PackageName == "" {
		return &api.InstallServiceBadRequest{}, errors.New("packageName is required")
	}
	if req.CatalogId == "" {
		return &api.InstallServiceBadRequest{}, errors.New("catalogId is required")
	}
	if req.Options == nil {
		return &api.InstallServiceBadRequest{}, errors.New("options are required")
	}

	values := make(map[string]interface{}, len(req.Options))

	for k, raw := range req.Options {
		var v interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			return &api.InstallServiceBadRequest{}, fmt.Errorf(
				"unmarshal values[%q]: %w",
				k,
				err,
			)
		}
		values[k] = v
	}

	dreq := domain.StartRequest{
		Username:      u.Username,
		CatalogID:     req.CatalogId,
		PackageName:   req.PackageName,
		Name:          req.Name,
		Version:       req.Version.Or("latest"),
		ReleaseID:     params.ReleaseId,
		OnyxiaProject: params.XOnyxiaProject.Or(""),
		FriendlyName:  req.FriendlyName.Or(req.PackageName),
		Values:        values,
	}

	// Execute use case.
	_, err := ic.serviceLifecycleUc.Start(ctx, dreq)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return &api.InstallServiceBadRequest{}, err
		case errors.Is(err, domain.ErrForbidden):
			return &api.InstallServiceForbidden{}, err
		case errors.Is(err, domain.ErrAlreadyExists):
			return &api.InstallServiceConflict{}, err
		default:
			slog.ErrorContext(ctx, "install failed", slog.Any("error", err))
			return &api.InstallServiceInternalServerError{}, err
		}
	}

	// Success: 202 Accepted + headers/body per ogen schema.@
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
