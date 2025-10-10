package env

import (
	"errors"
	"fmt"
)

func ValidateCatalogsConfig(catalogs []CatalogConfig) error {
	seen := make(map[string]struct{}, len(catalogs))
	for _, c := range catalogs {
		if err := ValidateCatalogConfig(c); err != nil {
			return err
		}
		if _, dup := seen[c.ID]; dup {
			return fmt.Errorf("catalog %q: duplicate id", c.ID)
		}
		seen[c.ID] = struct{}{}
	}
	return nil
}

func ValidateCatalogConfig(c CatalogConfig) error {

	if err := validateCommon(c); err != nil {
		return err
	}

	switch c.Type {
	case CatalogTypeHelm:
		if c.Packages != nil {
			return errors.New("helm catalog should not have packages")
		}
	case CatalogTypeOCI:
		if err := validateOCI(c); err != nil {
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

func validateCommon(cc CatalogConfig) error {
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

func validateOCI(o CatalogConfig) error {
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
