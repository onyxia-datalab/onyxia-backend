package helm

import (
	"fmt"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
)

type versionFilter interface {
	apply(versions []string) []string
}

type allVersions struct{}
type latestOnly struct{}
type skipPatches struct{}
type maxNumber struct{ n int }

func (allVersions) apply(versions []string) []string { return versions }

func (latestOnly) apply(versions []string) []string {
	if len(versions) == 0 {
		return versions
	}
	return versions[:1]
}

func (skipPatches) apply(versions []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		parts := strings.SplitN(v, ".", 3)
		if len(parts) < 2 {
			out = append(out, v)
			continue
		}
		minor := parts[0] + "." + parts[1]
		if seen[minor] {
			continue
		}
		seen[minor] = true
		out = append(out, v)
	}
	return out
}

func (f maxNumber) apply(versions []string) []string {
	if f.n >= len(versions) {
		return versions
	}
	return versions[:f.n]
}

func versionFilterFrom(cfg env.CatalogConfig) (versionFilter, error) {
	switch cfg.MultipleServicesMode {
	case env.MultipleServicesLatest:
		return latestOnly{}, nil
	case env.MultipleServicesSkipPatches:
		return skipPatches{}, nil
	case env.MultipleServicesMaxNumber:
		if cfg.MaxNumberOfVersions == nil {
			return nil, fmt.Errorf("catalog %q: multipleServicesMode=maxNumber requires maxNumberOfVersions", cfg.ID)
		}
		return maxNumber{n: *cfg.MaxNumberOfVersions}, nil
	default: // "all" or unset
		return allVersions{}, nil
	}
}
