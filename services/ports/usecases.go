package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// Driving ports: the use cases the API layer calls. They are implemented in
// usecase/; the other files of this package are the driven ports the use
// cases call, implemented in adapters/.

// CatalogService exposes the catalogs to a caller. user is nil for an
// anonymous caller: only the unrestricted catalogs are visible then.
type CatalogService interface {
	// ListCatalogs returns the catalogs user may see.
	ListCatalogs(ctx context.Context, user *usercontext.User) ([]domain.Catalog, error)
	// GetPackage returns ErrNotFound if the package is excluded, or if the
	// catalog is restricted and user does not satisfy any restriction.
	GetPackage(ctx context.Context, user *usercontext.User, catalogID, packageName string) (domain.Package, error)
	GetAvailableVersions(
		ctx context.Context,
		user *usercontext.User,
		catalogID, packageName string,
	) ([]string, error)
	GetPackageSchema(
		ctx context.Context,
		user *usercontext.User,
		catalogID, packageName, version string,
	) ([]byte, error)
	// CheckSharingAllowed returns ErrForbidden if the catalog does not allow
	// installed services to be shared, and ErrNotFound under the same
	// conditions as GetPackage.
	CheckSharingAllowed(ctx context.Context, user *usercontext.User, catalogID string) error
}

// ServiceLifecycle acts on the services of a namespace ("project"). Every
// operation returns ErrForbidden when the caller may not act in the
// request's namespace, and ErrNotFound for a service they cannot see.
type ServiceLifecycle interface {
	Start(ctx context.Context, req domain.StartRequest) error
	Suspend(ctx context.Context, req domain.SuspendRequest) error
	Resume(ctx context.Context, req domain.ResumeRequest) error
	Delete(ctx context.Context, req domain.DeleteRequest) error
	// SetShared changes whether a service is shared with the project members.
	// Only the service's owner may do it (ErrForbidden otherwise), and sharing
	// requires the service's catalog to allow it.
	SetShared(ctx context.Context, req domain.SetSharedRequest) error
}

// ServiceQuery is the read side of the service lifecycle. It applies the
// same namespace and visibility rules as ServiceLifecycle.
type ServiceQuery interface {
	GetService(ctx context.Context, user usercontext.User, namespace, releaseID string) (domain.Service, error)
	ListServices(ctx context.Context, user usercontext.User, namespace string) ([]domain.Service, error)
}

// ServiceEvents streams the changes of a service. It applies the same
// namespace and visibility rules as ServiceQuery.
type ServiceEvents interface {
	// Open checks that user may see the service and starts following it. The
	// returned channel first delivers the current state (a status event, then
	// a quota event when the project has a quota), then the changes, and ends
	// with a done event once the service is stable, deleted, or the maximum
	// duration is reached. It is closed after the done event, or without one
	// when ctx is done or following the service fails: the caller may then
	// open a new stream, which starts again from the current state.
	// Authorization errors are returned by Open itself, before any event.
	Open(ctx context.Context, user usercontext.User, namespace, releaseID string) (<-chan domain.ServiceEvent, error)
}

// ProjectQuotaQuery reads the resource quota of a project. It returns
// ErrForbidden when the caller may not act in the namespace.
type ProjectQuotaQuery interface {
	GetProjectQuota(ctx context.Context, user usercontext.User, namespace string) (domain.ProjectQuota, error)
}
