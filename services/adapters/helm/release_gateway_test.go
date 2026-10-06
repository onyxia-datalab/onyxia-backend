package helm

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/client-go/rest"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

func newAdapter(t *testing.T) *Helm {
	t.Helper()

	k8sCfg := &rest.Config{
		Host: "https://fake-cluster",
	}

	client, err := NewClient("")
	require.NoError(t, err)

	return NewReleaseGtw(k8sCfg, client, nil)
}

func TestStartInstallEmptyArgs(t *testing.T) {
	i := newAdapter(t)

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"",
		&domain.Package{},
		"",
		nil,
	)
	require.Error(t, err)
}

func TestStartInstallRequiresPackage(t *testing.T) {
	i := newAdapter(t)

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"rel",
		nil,
		"",
		nil,
	)
	require.ErrorContains(t, err, "package is required")
}

func TestStartInstallLocateChartError(t *testing.T) {
	i := newAdapter(t)

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"rel",
		&domain.Package{
			CatalogID: "fake-cat",
			Name:      "this-chart-does-not-exist",
			RepoURL:   "fake-repo",
		},
		"0.1.0",
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locating chart")
}

func TestStartInstallLoaderErrorWhenPathIsNotAChart(t *testing.T) {
	i := newAdapter(t)

	tmp := t.TempDir()
	nonChartDir := filepath.Join(tmp, "not-a-chart")
	require.NoError(t, os.MkdirAll(nonChartDir, 0o755))

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"rel",
		&domain.Package{
			CatalogID: "fake-cat",
			Name:      nonChartDir, // local path used as chartRef when no RepoURL is set
		},
		"0.1.0",
		map[string]interface{}{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading chart")
}

func TestUninstallRelease(t *testing.T) {
	cfg := action.NewConfiguration()
	cfg.Releases = storage.Init(driver.NewMemory())
	cfg.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
	rel := &releasev1.Release{
		Name:      "rel",
		Namespace: "test-ns",
		Version:   1,
		Info:      &releasev1.Info{Status: common.StatusDeployed},
	}
	require.NoError(t, cfg.Releases.Create(rel))

	i := newAdapter(t)
	requestedNamespace := ""
	i.configForNamespace = func(namespace string) (*action.Configuration, error) {
		requestedNamespace = namespace
		return cfg, nil
	}

	require.NoError(t, i.UninstallRelease(context.Background(), "test-ns", "rel"))
	assert.Equal(t, "test-ns", requestedNamespace)
	_, err := cfg.Releases.History("rel")
	assert.ErrorIs(t, err, driver.ErrReleaseNotFound)
}

func TestUninstallReleaseIgnoresMissingRelease(t *testing.T) {
	cfg := action.NewConfiguration()
	cfg.Releases = storage.Init(driver.NewMemory())
	cfg.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}

	i := newAdapter(t)
	i.configForNamespace = func(string) (*action.Configuration, error) { return cfg, nil }

	require.NoError(t, i.UninstallRelease(context.Background(), "test-ns", "missing"))
}

func TestUninstallReleaseHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	i := newAdapter(t)
	i.configForNamespace = func(string) (*action.Configuration, error) {
		t.Fatal("configuration should not be created for a canceled context")
		return nil, nil
	}

	assert.ErrorIs(t, i.UninstallRelease(ctx, "test-ns", "rel"), context.Canceled)
}

func TestWaitForInstalls(t *testing.T) {
	i := newAdapter(t)

	release := make(chan struct{})
	i.installs.Go(func() { <-release }) // stands in for a running install

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, i.WaitForInstalls(ctx), context.DeadlineExceeded)

	close(release)
	require.NoError(t, i.WaitForInstalls(context.Background()))
}

func TestGetReleaseStateTranslatesHelmStatus(t *testing.T) {
	tests := []struct {
		helm common.Status
		want ports.ReleaseState
	}{
		{common.StatusPendingInstall, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}},
		{common.StatusPendingUpgrade, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}},
		{common.StatusPendingRollback, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}},
		{common.StatusDeployed, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}},
		{common.StatusSuperseded, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}},
		{common.StatusFailed, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed}},
		{common.StatusUninstalling, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUninstalling}},
		{common.StatusUnknown, ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUnknown}},
		// Uninstalled with --keep-history: only the history is left.
		{common.StatusUninstalled, ports.ReleaseState{Exists: false}},
	}

	for _, tt := range tests {
		t.Run(string(tt.helm), func(t *testing.T) {
			cfg := action.NewConfiguration()
			cfg.Releases = storage.Init(driver.NewMemory())
			cfg.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
			require.NoError(t, cfg.Releases.Create(&releasev1.Release{
				Name:      "rel",
				Namespace: "test-ns",
				Version:   1,
				Info:      &releasev1.Info{Status: tt.helm},
			}))

			i := newAdapter(t)
			i.configForNamespace = func(string) (*action.Configuration, error) { return cfg, nil }

			state, err := i.GetReleaseState(context.Background(), "test-ns", "rel")

			require.NoError(t, err)
			assert.Equal(t, tt.want, state)
		})
	}
}

func TestListReleaseStates(t *testing.T) {
	cfg := action.NewConfiguration()
	cfg.Releases = storage.Init(driver.NewMemory())
	cfg.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
	for _, rel := range []*releasev1.Release{
		// Two revisions: only the latest one counts.
		{Name: "jupyter", Namespace: "ns", Version: 1, Info: &releasev1.Info{Status: common.StatusSuperseded}},
		{
			Name: "jupyter", Namespace: "ns", Version: 2,
			Info:   &releasev1.Info{Status: common.StatusDeployed},
			Config: map[string]interface{}{"global": map[string]interface{}{"suspend": true}},
		},
		{
			Name: "broken", Namespace: "ns", Version: 1,
			Info: &releasev1.Info{Status: common.StatusFailed, Description: "Release \"broken\" failed: timed out"},
		},
		{Name: "gone", Namespace: "ns", Version: 1, Info: &releasev1.Info{Status: common.StatusUninstalled}},
	} {
		require.NoError(t, cfg.Releases.Create(rel))
	}

	i := newAdapter(t)
	i.configForNamespace = func(string) (*action.Configuration, error) { return cfg, nil }

	states, err := i.ListReleaseStates(context.Background(), "ns")

	require.NoError(t, err)
	assert.Equal(t, map[string]ports.ReleaseState{
		"jupyter": {Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed},
		"broken": {
			Exists: true, Status: ports.ReleaseStatusFailed,
			Message: "Release \"broken\" failed: timed out",
		},
	}, states)
}
