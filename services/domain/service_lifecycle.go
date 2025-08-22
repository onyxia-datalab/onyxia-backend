package domain

import (
	"context"
	"net/url"
)

type StartRequest struct {
	Namespace string
	Name      string
	ReleaseID string
	Chart     string
	RepoURL   *url.URL
	Version   string
	Values    map[string]interface{}
}

type StartResponse struct {
}

type ServiceLifecycle interface {
	Start(ctx context.Context, req StartRequest) (StartResponse, error)
	Resume(ctx context.Context) error
	Delete(ctx context.Context) error
	Rename(ctx context.Context) error
	Share(ctx context.Context) error
}
