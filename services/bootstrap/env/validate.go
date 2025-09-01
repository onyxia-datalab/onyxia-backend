package env

import (
	"fmt"
)

// ValidateCatalogs validates a slice of catalogs (including duplicate IDs).
func ValidateCatalogs(catalogs []Catalog) error {
	seen := make(map[string]struct{}, len(catalogs))
	for i := range catalogs {
		c := catalogs[i]

		// Per-catalog validation
		if err := ValidateCatalog(c); err != nil {
			return err
		}

		// Duplicate ID check
		id := catalogID(c)
		if _, dup := seen[id]; dup {
			return fmt.Errorf("catalog %q: duplicate id", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// ValidateCatalog validates a single catalog according to its Type.
func ValidateCatalog(c Catalog) error {
	switch c.Type {
	case CatalogTypeHelm:
		if c.Helm == nil || c.OCI != nil {
			return fmt.Errorf(
				"catalog %q: type=helm requires Helm block and forbids OCI block",
				catalogID(c),
			)
		}
		if err := validateCommon(c.Helm.CatalogCommon); err != nil {
			return err
		}
		if err := validateHelm(*c.Helm); err != nil {
			return err
		}

	case CatalogTypeOCI:
		if c.OCI == nil || c.Helm != nil {
			return fmt.Errorf(
				"catalog %q: type=oci requires OCI block and forbids Helm block",
				catalogID(c),
			)
		}
		if err := validateCommon(c.OCI.CatalogCommon); err != nil {
			return err
		}
		if err := validateOCI(*c.OCI); err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"catalog: invalid type %q (expected %q or %q)",
			c.Type,
			CatalogTypeHelm,
			CatalogTypeOCI,
		)
	}
	return nil
}

// --- helpers ---

func catalogID(c Catalog) string {
	switch c.Type {
	case CatalogTypeHelm:
		if c.Helm != nil {
			return c.Helm.ID
		}
	case CatalogTypeOCI:
		if c.OCI != nil {
			return c.OCI.ID
		}
	}
	// Fallback if type is invalid or blocks are missing
	return "<unknown>"
}

func validateCommon(cc CatalogCommon) error {
	// Status enum
	switch cc.Status {
	case StatusProd, StatusTest:
		// ok
	default:
		return fmt.Errorf(
			"catalog %q: invalid status %q (expected %q or %q)",
			cc.ID,
			cc.Status,
			StatusProd,
			StatusTest,
		)
	}

	switch cc.MultipleServicesMode {
	case MultipleServicesAll, MultipleServicesLatest, MultipleServicesSkipPatches:
		if cc.MaxNumberOfVersions != nil {
			return fmt.Errorf(
				"catalog %q: maxNumberOfVersions must not be set when multipleServicesMode=%q",
				cc.ID,
				cc.MultipleServicesMode,
			)
		}
	case MultipleServicesMaxNumber:
		if cc.MaxNumberOfVersions == nil || *cc.MaxNumberOfVersions <= 0 {
			return fmt.Errorf(
				"catalog %q: maxNumberOfVersions must be > 0 when multipleServicesMode=%q",
				cc.ID,
				cc.MultipleServicesMode,
			)
		}
	default:
		return fmt.Errorf(
			"catalog %q: invalid multipleServicesMode %q",
			cc.ID,
			cc.MultipleServicesMode,
		)
	}

	if cc.ID == "" {
		return fmt.Errorf("catalog: id is required")
	}
	if len(cc.Name) == 0 {
		return fmt.Errorf("catalog %q: name is required", cc.ID)
	}

	return nil
}

func validateHelm(h HelmCatalog) error {
	if h.Location == "" {
		return fmt.Errorf("catalog %q: helm.location is required", h.ID)
	}
	return nil
}

func validateOCI(o OCICatalog) error {
	if o.Base == "" {
		return fmt.Errorf("catalog %q: oci.base is required", o.ID)
	}
	if len(o.Packages) == 0 {
		return fmt.Errorf("catalog %q: oci.packages must not be empty", o.ID)
	}
	for _, p := range o.Packages {
		if p.Name == "" {
			return fmt.Errorf("catalog %q: oci.package name is required", o.ID)
		}
	}
	return nil
}
