package domain

import (
	"context"
)

type StartRequest struct {
	Username      string
	OnyxiaProject string
	CatalogID     string
	PackageName   string
	Version       string
	ReleaseID     string
	Namespace     string
	FriendlyName  string
	Name          string
	Share         bool
	Values        map[string]interface{}
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
