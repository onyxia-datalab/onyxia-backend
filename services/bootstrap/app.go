package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/kube"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type Application struct {
	Env               *env.Env
	K8sClient         *kube.Client
	UserContextReader usercontext.Reader
	UserContextWriter usercontext.Writer
	Catalogs          []domain.Catalog
}

func NewApplication(ctx context.Context) (*Application, error) {
	userReader, userWriter := usercontext.NewUserContext()

	InitLogger(userReader)

	env, err := env.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)

	}

	k8sClient, err := kube.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	_ = k8sClient.Ping(ctx)

	catalogs := convertToDomainCatalogs(env.Catalogs)

	app := &Application{
		Env:               &env,
		K8sClient:         k8sClient,
		UserContextReader: userReader,
		UserContextWriter: userWriter,
		Catalogs:          catalogs,
	}

	slog.Info("Application initialized successfully")

	return app, nil
}

func convertToDomainCatalogs(catalogs []env.Catalog) []domain.Catalog {
	out := make([]domain.Catalog, 0, len(catalogs))

	for _, c := range catalogs {
		dc := domain.Catalog{
			ID:      c.ID,
			Type:    domain.CatalogType(c.Type),
			RepoURL: c.Location,
		}

		if c.Type == env.CatalogTypeOCI {
			for _, p := range c.Packages {
				dc.Packages = append(dc.Packages, domain.PackageRef{
					PackageName: p.Name,
					Versions:    append([]string(nil), p.Versions...),
					RepoURL:     c.Location,
				})
			}
		}

		out = append(out, dc)
	}

	return out
}
