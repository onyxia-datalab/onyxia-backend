package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
