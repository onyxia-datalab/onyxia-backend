package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
)

type Lifecycle struct {
	records    ports.ServiceRecordGateway
	helm       ports.ReleaseGateway
	catalogSvc ports.CatalogService
	namespaces namespace.Authorizer
}

var _ ports.ServiceLifecycle = (*Lifecycle)(nil)

func NewLifecycle(
	records ports.ServiceRecordGateway,
	helm ports.ReleaseGateway,
	catalogSvc ports.CatalogService,
	namespaces namespace.Authorizer,
) *Lifecycle {
	return &Lifecycle{records: records, helm: helm, catalogSvc: catalogSvc, namespaces: namespaces}
}

func (uc *Lifecycle) Start(
	ctx context.Context,
	req domain.StartRequest,
) (domain.StartResponse, error) {

	// Go through the catalog use case rather than the raw package repository
	// so catalog restrictions apply to installs, not just to browsing.
	pkg, err := uc.catalogSvc.GetPackage(ctx, req.CatalogID, req.PackageName)
	if err != nil {
		return domain.StartResponse{}, fmt.Errorf("get package: %w", err)
	}

	if req.Share {
		if err := uc.catalogSvc.CheckSharingAllowed(ctx, req.CatalogID); err != nil {
			return domain.StartResponse{}, fmt.Errorf("check sharing allowed: %w", err)
		}
	}

	// The Helm check catches pre-existing releases. CreateServiceRecord below
	// is the atomic reservation that also protects the interval until Helm
	// creates its release.
	state, err := uc.helm.GetReleaseState(ctx, req.Namespace, req.ReleaseID)
	if err != nil {
		return domain.StartResponse{}, fmt.Errorf("get release state: %w", err)
	}
	if state.Exists {
		return domain.StartResponse{}, fmt.Errorf(
			"%w: release %q already exists in namespace %q",
			domain.ErrAlreadyExists,
			req.ReleaseID,
			req.Namespace,
		)
	}

	record := ports.ServiceRecord{
		ReleaseID:    req.ReleaseID,
		CatalogID:    req.CatalogID,
		FriendlyName: req.FriendlyName,
		Owner:        req.Username,
		Share:        req.Share,
	}
	if err := uc.records.CreateServiceRecord(ctx, req.Namespace, record); err != nil {
		return domain.StartResponse{}, fmt.Errorf("create service record: %w", err)
	}

	opts := ports.InstallOptions{
		Callbacks: ports.InstallCallbacks{
			OnStart: func(release, chart string) {
				slog.InfoContext(ctx, "helm install started",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
				)
			},
			OnSuccess: func(release, chart string) {
				slog.InfoContext(ctx, "helm install succeeded",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
				)
			},
			OnError: func(release, chart string, err error) {
				slog.ErrorContext(ctx, "helm install failed",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
					slog.Any("error", err),
				)
			},
		},
	}

	if err := uc.helm.StartInstall(ctx, req.Namespace, req.ReleaseID, &pkg, req.Version, req.Values, opts); err != nil {
		return domain.StartResponse{}, fmt.Errorf("helm start: %w", err)
	}

	return domain.StartResponse{}, nil
}

// authorize enforces the same owner/share rule as the query side: a caller
// may only act on a service they can see. A service they can't see is
// reported as ErrNotFound, exactly as GetService does. The backend talks to
// Kubernetes with its own service account, so nothing downstream enforces
// this per user.
// It returns the service's record on success.
func (uc *Lifecycle) authorize(
	ctx context.Context,
	username, namespace, releaseName string,
) (ports.ServiceRecord, error) {
	rec, err := uc.records.GetServiceRecord(ctx, namespace, releaseName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ports.ServiceRecord{}, fmt.Errorf("service %q: %w", releaseName, domain.ErrNotFound)
		}
		return ports.ServiceRecord{}, fmt.Errorf("read service record: %w", err)
	}

	if !uc.namespaces.CanAccessService(username, namespace, rec.Owner, rec.Share) {
		return ports.ServiceRecord{}, fmt.Errorf("service %q: %w", releaseName, domain.ErrNotFound)
	}
	return rec, nil
}

func (uc *Lifecycle) Suspend(ctx context.Context, req domain.SuspendRequest) error {
	if _, err := uc.authorize(ctx, req.Username, req.Namespace, req.ReleaseName); err != nil {
		return err
	}
	return uc.helm.SuspendRelease(ctx, req.Namespace, req.ReleaseName)
}

func (uc *Lifecycle) Resume(ctx context.Context, req domain.ResumeRequest) error {
	if _, err := uc.authorize(ctx, req.Username, req.Namespace, req.ReleaseName); err != nil {
		return err
	}
	return uc.helm.ResumeRelease(ctx, req.Namespace, req.ReleaseName)
}

func (uc *Lifecycle) Delete(ctx context.Context, req domain.DeleteRequest) error {
	if _, err := uc.authorize(ctx, req.Username, req.Namespace, req.ReleaseName); err != nil {
		return err
	}
	if err := uc.helm.UninstallRelease(ctx, req.Namespace, req.ReleaseName); err != nil {
		return fmt.Errorf("helm uninstall: %w", err)
	}
	if err := uc.records.DeleteServiceRecord(ctx, req.Namespace, req.ReleaseName); err != nil {
		return fmt.Errorf("delete service record: %w", err)
	}
	return nil
}

// SetShared lets the owner of a service share it with the members of its
// project namespace, or make it private again. Other callers who can see the
// service (because it is shared) get ErrForbidden; callers who can't see it
// get ErrNotFound, as for every other operation.
func (uc *Lifecycle) SetShared(ctx context.Context, req domain.SetSharedRequest) error {
	rec, err := uc.authorize(ctx, req.Username, req.Namespace, req.ReleaseName)
	if err != nil {
		return err
	}

	if !strings.EqualFold(rec.Owner, req.Username) {
		return fmt.Errorf(
			"%w: only the owner of service %q can change its sharing",
			domain.ErrForbidden,
			req.ReleaseName,
		)
	}

	if req.Shared {
		if err := uc.catalogSvc.CheckSharingAllowed(ctx, rec.CatalogID); err != nil {
			return fmt.Errorf("check sharing allowed: %w", err)
		}
	}

	if err := uc.records.SetServiceShared(ctx, req.Namespace, req.ReleaseName, req.Shared); err != nil {
		return fmt.Errorf("update service record: %w", err)
	}
	return nil
}
