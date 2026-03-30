package domain

import (
	"context"
)

type StartRequest struct {
	Username     string
	CatalogID    string
	PackageName  string
	Version      string
	ReleaseID    string
	Namespace    string
	FriendlyName string
	Name         string
	Share        bool
	Values       map[string]interface{}
}

type StartResponse struct {
}

type SuspendRequest struct {
	ReleaseName string
	Namespace   string
}

type ResumeRequest struct {
	ReleaseName string
	Namespace   string
}

type DeleteRequest struct {
	ReleaseName string
	Namespace   string
}

type ServiceLifecycle interface {
	Start(ctx context.Context, req StartRequest) (StartResponse, error)
	Suspend(ctx context.Context, req SuspendRequest) error
	Resume(ctx context.Context, req ResumeRequest) error
	Delete(ctx context.Context, req DeleteRequest) error
	Rename(ctx context.Context) error
	Share(ctx context.Context) error
}
