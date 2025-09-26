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

func newAdapter(t *testing.T, cb ports.HelmStartCallbacks) *Helm {
	t.Helper()

	k8sCfg := &rest.Config{
		Host: "https://fake-cluster",
	}

	adapter, err := NewReleaseGtw(k8sCfg, cb)
	require.NoError(t, err)

	return adapter
}

func defaultCallbacks() ports.HelmStartCallbacks {
	return ports.HelmStartCallbacks{
		OnStart:   func(_, _ string) {},
		OnSuccess: func(_, _ string) {},
		OnError:   func(_, _ string, _ error) {},
	}
}

func TestStartInstall_EmptyArgs(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

	err := i.StartInstall(
		context.Background(),
		"",
		domain.PackageVersion{},
		nil,
		ports.HelmStartOptions{},
	)
	require.Error(t, err)

}

func TestStartInstall_LocateChart_Error(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

	// Chart inexistant → act.LocateChart renvoie une erreur (pré-flight)
	err := i.StartInstall(
		context.Background(),
		"rel",
		domain.PackageVersion{
			Package: domain.Package{
				CatalogID: "fake-cat",
				Name:      "this-chart-does-not-exist",
			},
			Version: "0.1.0",
			RepoURL: "fake-repo",
		},
		nil,
		ports.HelmStartOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locating chart")
}

func TestStartInstall_Loader_Error_WhenPathIsNotAChart(t *testing.T) {
	i := newAdapter(t, defaultCallbacks())

	tmp := t.TempDir()
	nonChartDir := filepath.Join(tmp, "not-a-chart")
	require.NoError(t, os.MkdirAll(nonChartDir, 0o755))

	err := i.StartInstall(
		context.Background(),
		"rel",
		domain.PackageVersion{
			Package: domain.Package{
				CatalogID: "fake-cat",
				Name:      "",
			},
			Version: "0.1.0",
			RepoURL: nonChartDir,
		},
		map[string]interface{}{},
		ports.HelmStartOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading chart")
}

func TestStartInstall_NoCallbacks_OnPreflightErrors(t *testing.T) {

	startCalled := false
	successCalled := false
	errorCalled := false

	i := newAdapter(t, ports.HelmStartCallbacks{
		OnStart:   func(_, _ string) { startCalled = true },
		OnSuccess: func(_, _ string) { successCalled = true },
		OnError:   func(_, _ string, _ error) { errorCalled = true },
	})

	err := i.StartInstall(context.Background(), "rel", domain.PackageVersion{
		Package: domain.Package{
			CatalogID: "fake-cat",
			Name:      "unknown-chart",
		},
		Version: "0.1.0",
		RepoURL: "fake-repo",
	}, nil, ports.HelmStartOptions{
		Callbacks: ports.HelmStartCallbacks{
			OnStart:   func(_, _ string) { startCalled = true },
			OnSuccess: func(_, _ string) { successCalled = true },
			OnError:   func(_, _ string, _ error) { errorCalled = true },
		},
	})
	require.Error(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.False(t, startCalled, "OnStart should not be called on preflight error")
	assert.False(t, successCalled, "OnSuccess should not be called on preflight error")
	assert.False(t, errorCalled, "OnError should not be called on preflight error")
}
