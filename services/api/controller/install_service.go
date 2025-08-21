package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type InstallController struct {
	usecase    domain.InstallUsecase
	userGetter usercontext.UserGetter
}

func NewInstallController(
	usecase domain.InstallUsecase,
	userGetter usercontext.UserGetter,
) *InstallController {
	return &InstallController{
		usecase:    usecase,
		userGetter: userGetter,
	}
}

func (ic *InstallController) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	params api.InstallServiceParams,
) (api.InstallServiceRes, error) {
	// AuthN: middleware should have populated the context.
	if u, ok := ic.userGetter.GetUser(ctx); !ok || u == nil {
		slog.ErrorContext(ctx, "user not found in context")
		// Typically 401 is handled by auth middleware; treat missing user as 403 here.
		return &api.InstallServiceForbidden{}, errors.New("user not found")
	}

	if req == nil || req.Chart == "" {
		return &api.InstallServiceBadRequest{}, errors.New("chart is required")
	}

	// Optional fields to pointers via ogen helpers.
	var repoURL *url.URL
	if u, ok := req.RepoUrl.Get(); ok {
		repoURL = &u
	}
	var version *string
	if v, ok := req.Version.Get(); ok {
		// Keep pointer even if empty string to preserve “latest” explicitly provided.
		version = &v
	}

	values := make(map[string]interface{})

	if rawMap, ok := req.Values.Get(); ok {
		for k, raw := range rawMap {
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
	}

	// Build domain request.
	dreq := domain.InstallRequest{
		ReleaseID: params.ReleaseId,
		Chart:     req.Chart,
		RepoURL:   repoURL,
		Version:   version,
		Values:    values,
	}

	// Execute use case.
	_, err := ic.usecase.DummyInstall(ctx, dreq)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			return &api.InstallServiceBadRequest{}, err
		case errors.Is(err, domain.ErrForbidden):
			return &api.InstallServiceForbidden{}, err
		case errors.Is(err, domain.ErrAlreadyExists):
			// If you want idempotent 202, change this mapping to Accepted with existing URLs.
			return &api.InstallServiceConflict{}, err
		default:
			slog.ErrorContext(ctx, "install failed", slog.Any("error", err))
			return &api.InstallServiceInternalServerError{}, err
		}
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
