package controller

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type CatalogController struct {
	catalogs   domain.CatalogService
	userGetter usercontext.UserGetter
}

func NewCatalogController(
	catalogs domain.CatalogService,
	userGetter usercontext.UserGetter,
) *CatalogController {
	return &CatalogController{catalogs: catalogs, userGetter: userGetter}
}

func (cc *CatalogController) GetMyCatalogs(ctx context.Context) (api.GetMyCatalogsRes, error) {
	user, _ := cc.userGetter.GetUser(ctx)
	slog.InfoContext(ctx, "User in GetMyCatalogs", slog.Any("user", user))

	var (
		catalogs []domain.Catalog
		err      error
	)

	if user != nil {
		catalogs, err = cc.catalogs.ListUserCatalog(ctx)
	} else {
		catalogs, err = cc.catalogs.ListPublicCatalogs(ctx)
	}

	slog.InfoContext(ctx, "Catalogs fetched", slog.Int("count", len(catalogs)))

	if err != nil {
		slog.ErrorContext(ctx, "list catalogs failed", "err", err)
		problem := &api.Problem{}
		problem.Title.SetTo("Unable to list catalogs")
		problem.Status.SetTo(500)
		problem.Detail.SetTo(err.Error())
		return problem, err
	}

	return &api.GetMyCatalogsOKApplicationJSON{}, nil
}
