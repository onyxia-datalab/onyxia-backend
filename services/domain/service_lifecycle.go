package domain

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
