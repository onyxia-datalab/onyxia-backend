package ports

import "context"

// ServiceRecord is the metadata Onyxia keeps for every service it installs.
// It is written before the install starts and outlives a failed install,
// which is how a service whose release is missing is detected (Ghost).
type ServiceRecord struct {
	ReleaseID    string
	CatalogID    string
	FriendlyName string
	Owner        string
	Share        bool
}

type ServiceRecordGateway interface {
	// CreateServiceRecord writes the record once: it is the atomic
	// reservation of the release name. It returns domain.ErrAlreadyExists
	// when another installation has already reserved rec.ReleaseID.
	CreateServiceRecord(ctx context.Context, namespace string, rec ServiceRecord) error
	// GetServiceRecord returns domain.ErrNotFound when the record does not exist.
	GetServiceRecord(ctx context.Context, namespace, releaseID string) (ServiceRecord, error)
	// ListServiceRecords returns every service record of the namespace.
	ListServiceRecords(ctx context.Context, namespace string) ([]ServiceRecord, error)
	// SetServiceShared updates the share flag of an existing record and
	// returns domain.ErrNotFound when it does not exist. Callers must
	// authorize the update before invoking it.
	SetServiceShared(ctx context.Context, namespace, releaseID string, share bool) error
	// DeleteServiceRecord is a no-op when the record does not exist.
	DeleteServiceRecord(ctx context.Context, namespace, releaseID string) error
}
