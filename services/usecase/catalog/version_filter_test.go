package catalog

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(n int) *int { return &n }

// ---------- allVersions ----------

func TestAllVersions_ReturnsAll(t *testing.T) {
	versions := []string{"1.2.3", "1.1.0", "1.0.0"}
	assert.Equal(t, versions, allVersions{}.apply(versions))
}

func TestAllVersions_Empty(t *testing.T) {
	assert.Empty(t, allVersions{}.apply(nil))
}

// ---------- latestOnly ----------

func TestLatestOnly_ReturnsSingle(t *testing.T) {
	result := latestOnly{}.apply([]string{"2.0.0", "1.9.0", "1.0.0"})
	assert.Equal(t, []string{"2.0.0"}, result)
}

func TestLatestOnly_Empty(t *testing.T) {
	assert.Empty(t, latestOnly{}.apply(nil))
}

// ---------- skipPatches ----------

func TestSkipPatches_KeepsLatestPerMinor(t *testing.T) {
	// Helm index is sorted newest-first, so first occurrence of each minor wins.
	input := []string{"1.2.3", "1.2.1", "1.1.5", "1.1.0", "1.0.0"}
	result := skipPatches{}.apply(input)
	assert.Equal(t, []string{"1.2.3", "1.1.5", "1.0.0"}, result)
}

func TestSkipPatches_NonSemver_PassedThrough(t *testing.T) {
	result := skipPatches{}.apply([]string{"latest", "1.0.0"})
	assert.Equal(t, []string{"latest", "1.0.0"}, result)
}

func TestSkipPatches_Empty(t *testing.T) {
	assert.Empty(t, skipPatches{}.apply(nil))
}

// ---------- maxNumber ----------

func TestMaxNumber_Truncates(t *testing.T) {
	result := maxNumber{n: 2}.apply([]string{"1.2.0", "1.1.0", "1.0.0"})
	assert.Equal(t, []string{"1.2.0", "1.1.0"}, result)
}

func TestMaxNumber_NGreaterThanLen_ReturnsAll(t *testing.T) {
	versions := []string{"1.0.0"}
	result := maxNumber{n: 10}.apply(versions)
	assert.Equal(t, versions, result)
}

func TestMaxNumber_Zero_ReturnsEmpty(t *testing.T) {
	result := maxNumber{n: 0}.apply([]string{"1.0.0", "2.0.0"})
	assert.Empty(t, result)
}

// ---------- versionFilterFrom ----------

func TestVersionFilterFrom_All(t *testing.T) {
	f, err := versionFilterFrom(env.CatalogConfig{MultipleServicesMode: env.MultipleServicesAll})
	require.NoError(t, err)
	assert.IsType(t, allVersions{}, f)
}

func TestVersionFilterFrom_Default(t *testing.T) {
	f, err := versionFilterFrom(env.CatalogConfig{})
	require.NoError(t, err)
	assert.IsType(t, allVersions{}, f)
}

func TestVersionFilterFrom_Latest(t *testing.T) {
	f, err := versionFilterFrom(env.CatalogConfig{MultipleServicesMode: env.MultipleServicesLatest})
	require.NoError(t, err)
	assert.IsType(t, latestOnly{}, f)
}

func TestVersionFilterFrom_SkipPatches(t *testing.T) {
	f, err := versionFilterFrom(env.CatalogConfig{MultipleServicesMode: env.MultipleServicesSkipPatches})
	require.NoError(t, err)
	assert.IsType(t, skipPatches{}, f)
}

func TestVersionFilterFrom_MaxNumber(t *testing.T) {
	f, err := versionFilterFrom(env.CatalogConfig{
		MultipleServicesMode: env.MultipleServicesMaxNumber,
		MaxNumberOfVersions:  ptr(3),
	})
	require.NoError(t, err)
	assert.Equal(t, maxNumber{n: 3}, f)
}

func TestVersionFilterFrom_MaxNumber_MissingN_ReturnsError(t *testing.T) {
	_, err := versionFilterFrom(env.CatalogConfig{
		ID:                   "my-catalog",
		MultipleServicesMode: env.MultipleServicesMaxNumber,
		MaxNumberOfVersions:  nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-catalog")
	assert.Contains(t, err.Error(), "maxNumberOfVersions")
}
