package domain

import "github.com/onyxia-datalab/onyxia-backend/internal/usercontext"

// The requests carry the caller: the use cases decide what they may do.

type StartRequest struct {
	User         usercontext.User
	CatalogID    string
	PackageName  string
	Version      string
	ReleaseID    string
	Namespace    string
	FriendlyName string
	Share        bool
	Values       map[string]interface{}
}

type SuspendRequest struct {
	User        usercontext.User
	ReleaseName string
	Namespace   string
}

type ResumeRequest struct {
	User        usercontext.User
	ReleaseName string
	Namespace   string
}

type DeleteRequest struct {
	User        usercontext.User
	ReleaseName string
	Namespace   string
}

type SetSharedRequest struct {
	User        usercontext.User
	ReleaseName string
	Namespace   string
	Shared      bool
}
