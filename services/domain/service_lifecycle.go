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
	Username    string
	ReleaseName string
	Namespace   string
}

type ResumeRequest struct {
	Username    string
	ReleaseName string
	Namespace   string
}

type DeleteRequest struct {
	Username    string
	ReleaseName string
	Namespace   string
}

type SetSharedRequest struct {
	Username    string
	ReleaseName string
	Namespace   string
	Shared      bool
}

type ServiceLifecycle interface {
	Start(ctx context.Context, req StartRequest) (StartResponse, error)
	Suspend(ctx context.Context, req SuspendRequest) error
	Resume(ctx context.Context, req ResumeRequest) error
	Delete(ctx context.Context, req DeleteRequest) error
	// SetShared changes whether a service is shared with the project members.
	// Only the service's owner may do it (ErrForbidden otherwise), and sharing
	// requires the service's catalog to allow it.
	SetShared(ctx context.Context, req SetSharedRequest) error
}
