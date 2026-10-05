package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// Driving ports: the use cases the API layer calls. They are implemented in
// usecase/; the other files of this package are the driven ports the use
// cases call, implemented in adapters/.

type CatalogService interface {
	ListPublicCatalogs(ctx context.Context) ([]domain.Catalog, error)
	ListUserCatalogs(ctx context.Context) ([]domain.Catalog, error)
	// GetPackage returns ErrNotFound if the package is excluded, or if the
	// catalog is restricted and the caller does not satisfy any restriction.
	GetPackage(ctx context.Context, catalogID string, packageName string) (domain.Package, error)
	GetAvailableVersions(ctx context.Context, catalogID string, packageName string) ([]string, error)
	GetPackageSchema(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) ([]byte, error)
	// CheckSharingAllowed returns ErrForbidden if the catalog does not allow
	// installed services to be shared, and ErrNotFound under the same
	// conditions as GetPackage.
	CheckSharingAllowed(ctx context.Context, catalogID string) error
}

type ServiceLifecycle interface {
	Start(ctx context.Context, req domain.StartRequest) (domain.StartResponse, error)
	Suspend(ctx context.Context, req domain.SuspendRequest) error
	Resume(ctx context.Context, req domain.ResumeRequest) error
	Delete(ctx context.Context, req domain.DeleteRequest) error
	// SetShared changes whether a service is shared with the project members.
	// Only the service's owner may do it (ErrForbidden otherwise), and sharing
	// requires the service's catalog to allow it.
	SetShared(ctx context.Context, req domain.SetSharedRequest) error
}

// ServiceQuery is the read side of the service lifecycle.
type ServiceQuery interface {
	GetService(ctx context.Context, namespace, releaseID string) (domain.Service, error)
	ListServices(ctx context.Context, namespace string) ([]domain.Service, error)
}
