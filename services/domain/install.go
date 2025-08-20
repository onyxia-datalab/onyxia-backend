package domain

import (
	"context"
	"net/url"
)

type InstallRequest struct {
	ReleaseID string
	Chart     string
	RepoURL   *url.URL
	Version   *string
	Values    map[string][]byte
}

type InstallUsecase interface {
	DummyInstall(ctx context.Context, req InstallRequest) (InstallResponse, error)
}

type InstallResponse struct {
}
