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
	slog.InfoContext(ctx, "GetMyCatalogs")

	var (
		catalogs []domain.Catalog
		err      error
	)

	if _, authenticated := cc.userGetter.GetUser(ctx); authenticated {
		catalogs, err = cc.catalogs.ListUserCatalog(ctx)
	} else {
		catalogs, err = cc.catalogs.ListPublicCatalogs(ctx)
	}

	if err != nil {
		slog.ErrorContext(ctx, "Failed to list catalogs", slog.String("error", err.Error()))
		problem := &api.Problem{}
		problem.Title.SetTo("Unable to list catalogs")
		problem.Status.SetTo(500)
		problem.Detail.SetTo(err.Error())
		return problem, err
	}

	slog.InfoContext(ctx, "Catalogs fetched", slog.Int("count", len(catalogs)))

	response := make(api.GetMyCatalogsOKApplicationJSON, 0, len(catalogs))

	for _, catalog := range catalogs {
		apiCatalog := api.Catalog{
			ID:                  catalog.ID,
			HighlightedPackages: append([]string(nil), catalog.HighlightedPackages...),
		}

		if len(catalog.Packages) > 0 {
			apiPackages := make([]api.Package, 0, len(catalog.Packages))
			for _, pkg := range catalog.Packages {
				apiPackages = append(apiPackages, api.Package{
					Name:        pkg.Name,
					Description: api.NewOptString(pkg.Description),
					Icon:        pkg.IconUrl,
					Home:        api.NewOptURI(pkg.HomeUrl),
				})
			}
			apiCatalog.Packages = apiPackages
		}

		if plainName, ok := catalog.Name.GetPlain(); ok {
			apiCatalog.SetName(api.NewStringLocalizedString(plainName))
		}

		if multiName, ok := catalog.Name.GetMulti(); ok {
			apiCatalog.SetName(
				api.NewLocalizedString1LocalizedString(api.LocalizedString1(multiName)),
			)
		}

		if plainDescription, ok := catalog.Description.GetPlain(); ok {
			apiCatalog.SetDescription(
				api.NewOptLocalizedString(api.NewStringLocalizedString(plainDescription)),
			)
		}

		if multiDescription, ok := catalog.Description.GetMulti(); ok {
			apiCatalog.SetDescription(api.NewOptLocalizedString(
				api.NewLocalizedString1LocalizedString(api.LocalizedString1(multiDescription))),
			)
		}

		if status := api.CatalogStatus(catalog.Status); status != "" {
			switch status {
			case api.CatalogStatusPROD, api.CatalogStatusTEST:
				apiCatalog.Status = api.NewOptCatalogStatus(status)

			default:
				slog.WarnContext(ctx, "Unexpected catalog status",
					slog.String("catalog_id", catalog.ID),
					slog.Any("status", catalog.Status),
				)
			}
		}

		apiCatalog.Visible = api.NewOptCatalogVisible(api.CatalogVisible{
			User:    catalog.Visible.User,
			Project: catalog.Visible.Project,
		})

		response = append(response, apiCatalog)
	}

	return &response, nil
}
