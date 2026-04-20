package env

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	case CatalogTypeHelmRepo:
		if c.Packages != nil {
			return errors.New("helm catalog should not have packages")
		}
		if c.IndexTTL < 0 {
			return fmt.Errorf("catalog %q: indexTtl must not be negative", c.ID)
		}
	case CatalogTypeOCI:
		if c.IndexTTL != 0 {
			return fmt.Errorf("catalog %q: indexTtl is not supported for OCI catalogs", c.ID)
		}
		if err := validateOCI(c); err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"catalog: invalid type %q (expected %q or %q)",
			c.Type,
			CatalogTypeHelmRepo,
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
	case MultipleServicesAll, MultipleServicesLatest, MultipleServicesSkipPatches, "":
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

	for _, r := range cc.Restrictions {
		if strings.TrimSpace(r.UserAttributeKey) == "" {
			return fmt.Errorf("catalog %q: restriction missing userAttribute.key", cc.ID)
		}
		if _, err := regexp.Compile(r.Match); err != nil {
			return fmt.Errorf(
				"catalog %q: invalid restriction regex for key %q: %w",
				cc.ID,
				r.UserAttributeKey,
				err,
			)
		}
	}

	return nil
}

func validateOCI(o CatalogConfig) error {
	if o.Location != "" {
		return fmt.Errorf(
			"catalog %q: location must not be set on OCI catalogs (set it per package instead)",
			o.ID,
		)
	}
	if len(o.Packages) == 0 {
		return fmt.Errorf("catalog %q: oci.packages must not be empty", o.ID)
	}
	for _, p := range o.Packages {
		if p.Name == "" {
			return fmt.Errorf("catalog %q: oci package: name is required", o.ID)
		}
		if p.ChartRef == "" {
			return fmt.Errorf(
				"catalog %q: oci package %q: chartRef is required",
				o.ID,
				p.Name,
			)
		}
	}
	return nil
}
