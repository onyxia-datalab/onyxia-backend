package helm

import (
	"context"
	"errors"
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

func newAdapter(t *testing.T, cb ports.InstallCallbacks) *Helm {
	t.Helper()

	k8sCfg := &rest.Config{
		Host: "https://fake-cluster",
	}

	client, err := NewClient("")
	require.NoError(t, err)

	adapter, err := NewReleaseGtw(k8sCfg, client, cb)
	require.NoError(t, err)

	return adapter
}

func defaultCallbacks() ports.InstallCallbacks {
	return ports.InstallCallbacks{
		OnStart:   func(_, _ string) {},
		OnSuccess: func(_, _ string) {},
		OnError:   func(_, _ string, _ error) {},
	}
}

func TestStartInstallEmptyArgs(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"",
		&domain.Package{},
		"",
		nil,
		ports.InstallOptions{},
	)
	require.Error(t, err)
}

func TestStartInstallRequiresPackage(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"rel",
		nil,
		"",
		nil,
		ports.InstallOptions{},
	)
	require.ErrorContains(t, err, "package is required")
}

func TestStartInstallLocateChartError(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

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
		ports.InstallOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locating chart")
}

func TestStartInstallLoaderErrorWhenPathIsNotAChart(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

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
		ports.InstallOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading chart")
}

func TestStartInstallNoCallbacksOnPreflightErrors(t *testing.T) {

	startCalled := false
	successCalled := false
	errorCalled := false

	i := newAdapter(t, ports.InstallCallbacks{
		OnStart:   func(_, _ string) { startCalled = true },
		OnSuccess: func(_, _ string) { successCalled = true },
		OnError:   func(_, _ string, _ error) { errorCalled = true },
	})

	err := i.StartInstall(
		context.Background(),
		"test-ns",
		"rel",
		&domain.Package{
			CatalogID: "fake-cat",
			Name:      "unknown-chart",
			RepoURL:   "fake-repo",
		},
		"0.1.0",
		nil,
		ports.InstallOptions{
			Callbacks: ports.InstallCallbacks{
				OnStart:   func(_, _ string) { startCalled = true },
				OnSuccess: func(_, _ string) { successCalled = true },
				OnError:   func(_, _ string, _ error) { errorCalled = true },
			},
		},
	)
	require.Error(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.False(t, startCalled, "OnStart should not be called on preflight error")
	assert.False(t, successCalled, "OnSuccess should not be called on preflight error")
	assert.False(t, errorCalled, "OnError should not be called on preflight error")
}

func TestInstallCallbackHelpersIgnoreMissingCallbacks(t *testing.T) {
	require.NotPanics(t, func() {
		notifyInstallStart(ports.InstallCallbacks{}, "release", "chart")
		notifyInstallSuccess(ports.InstallCallbacks{}, "release", "chart")
		notifyInstallError(ports.InstallCallbacks{}, "release", "chart", errors.New("failed"))
	})
}

func TestInstallCallbackHelpersInvokeCallbacks(t *testing.T) {
	startCalled := false
	successCalled := false
	errorCalled := false
	callbacks := ports.InstallCallbacks{
		OnStart:   func(_, _ string) { startCalled = true },
		OnSuccess: func(_, _ string) { successCalled = true },
		OnError:   func(_, _ string, _ error) { errorCalled = true },
	}

	notifyInstallStart(callbacks, "release", "chart")
	notifyInstallSuccess(callbacks, "release", "chart")
	notifyInstallError(callbacks, "release", "chart", errors.New("failed"))

	assert.True(t, startCalled)
	assert.True(t, successCalled)
	assert.True(t, errorCalled)
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

	i := newAdapter(t, defaultCallbacks())
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

	i := newAdapter(t, defaultCallbacks())
	i.configForNamespace = func(string) (*action.Configuration, error) { return cfg, nil }

	require.NoError(t, i.UninstallRelease(context.Background(), "test-ns", "missing"))
}

func TestUninstallReleaseHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	i := newAdapter(t, defaultCallbacks())
	i.configForNamespace = func(string) (*action.Configuration, error) {
		t.Fatal("configuration should not be created for a canceled context")
		return nil, nil
	}

	assert.ErrorIs(t, i.UninstallRelease(ctx, "test-ns", "rel"), context.Canceled)
}
