package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-faster/jx"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type CatalogController struct {
	catalogs   domain.CatalogService
	userReader usercontext.Reader
}

func NewCatalogController(
	catalogs domain.CatalogService,
	userReader usercontext.Reader,
) *CatalogController {
	return &CatalogController{catalogs: catalogs, userReader: userReader}
}

func (cc *CatalogController) GetMyCatalogs(ctx context.Context) (api.GetMyCatalogsRes, error) {
	slog.InfoContext(ctx, "GetMyCatalogs")

	var (
		catalogs []domain.Catalog
		err      error
	)

	if _, authenticated := cc.userReader.GetUser(ctx); authenticated {
		catalogs, err = cc.catalogs.ListUserCatalogs(ctx)
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

		response = append(response, apiCatalog)
	}

	return &response, nil
}

func (cc *CatalogController) GetMyPackage(
	ctx context.Context,
	catalogID string,
	packageName string,
) (api.GetMyPackageRes, error) {
	slog.InfoContext(
		ctx,
		"GetMyPackage",
		slog.String("catalog_id", catalogID),
		slog.String("package_name", packageName),
	)

	pkg, err := cc.catalogs.GetPackage(ctx, catalogID, packageName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			problem := &api.GetMyPackageNotFound{}
			problem.Title.SetTo("Not found")
			problem.Status.SetTo(404)
			problem.Detail.SetTo(err.Error())
			return problem, nil
		}
		slog.ErrorContext(ctx, "Failed to get package", slog.String("error", err.Error()))
		problem := &api.GetMyPackageInternalServerError{}
		problem.Title.SetTo("Unable to get package")
		problem.Status.SetTo(500)
		problem.Detail.SetTo(err.Error())
		return problem, err
	}

	return &api.DetailedPackage{
		Name:        pkg.Name,
		Description: api.NewOptString(pkg.Description),
		Icon:        pkg.IconUrl,
		Home:        api.NewOptURI(pkg.HomeUrl),
		Versions:    pkg.Versions,
	}, nil
}

func (cc *CatalogController) GetPackageSchema(
	ctx context.Context,
	catalogID string,
	packageName string,
	version string,
) (api.GetPackageSchemaRes, error) {
	slog.InfoContext(ctx, "GetPackageSchema",
		slog.String("catalog_id", catalogID),
		slog.String("package_name", packageName),
		slog.String("version", version),
	)

	raw, err := cc.catalogs.GetPackageSchema(ctx, catalogID, packageName, version)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get package schema", slog.String("error", err.Error()))
		problem := &api.GetPackageSchemaInternalServerError{}
		problem.Title.SetTo("Unable to get package schema")
		problem.Status.SetTo(500)
		problem.Detail.SetTo(err.Error())
		return problem, err
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}

	result := make(api.GetPackageSchemaOK, len(rawMap))
	for k, v := range rawMap {
		result[k] = jx.Raw(v)
	}
	return &result, nil
}
