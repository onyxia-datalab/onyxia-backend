package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

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
	// Auth middleware should have populated the context.
	if u, ok := ic.userGetter.GetUser(ctx); !ok || u == nil {
		slog.ErrorContext(ctx, "user not found in context")
		return &api.InstallServiceForbidden{}, errors.New("user not found")
	}

	if req == nil || req.Chart == "" {
		return &api.InstallServiceBadRequest{}, errors.New("chart is required")
	}

	isOCI := strings.HasPrefix(req.Chart, "oci://")

	var repoURL *url.URL
	if u, ok := req.RepoUrl.Get(); ok && u.String() != "" {
		if !isOCI {
			repoURL = &u
		}
	} else if !isOCI {
		return &api.InstallServiceBadRequest{}, errors.New("repoUrl is required for non-OCI charts")
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
	dreq := domain.StartRequest{
		ReleaseID: params.ReleaseId,
		Chart:     req.Chart,
		RepoURL:   repoURL,
		Version:   req.Version.Or("latest"),
		Values:    values,
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
