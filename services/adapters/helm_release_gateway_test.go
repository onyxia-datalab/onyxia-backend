package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

func newAdapter(t *testing.T) *Helm {
	t.Helper()
	settings := cli.New()
	settings.SetNamespace("test-ns")
	return New(&action.Configuration{}, settings, ports.HelmStartCallbacks{
		OnStart:   func(_, _ string) {},
		OnSuccess: func(_, _ string) {},
		OnError:   func(_, _ string, _ error) {},
	})
}

func TestStartInstall_EmptyArgs(t *testing.T) {
	i := newAdapter(t)

	err := i.StartInstall(context.Background(), "", "my-chart", nil, ports.HelmStartOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "releaseName")

	err = i.StartInstall(context.Background(), "rel", "", nil, ports.HelmStartOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chartRef")
}

func TestStartInstall_LocateChart_Error(t *testing.T) {
	i := newAdapter(t)

	// Chart inexistant → act.LocateChart renvoie une erreur (pré-flight)
	err := i.StartInstall(
		context.Background(),
		"rel",
		"this-chart-does-not-exist",
		nil,
		ports.HelmStartOptions{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locating chart")
}

func TestStartInstall_Loader_Error_WhenPathIsNotAChart(t *testing.T) {
	i := newAdapter(t)

	tmp := t.TempDir()
	nonChartDir := filepath.Join(tmp, "not-a-chart")
	require.NoError(t, os.MkdirAll(nonChartDir, 0o755))

	err := i.StartInstall(
		context.Background(),
		"rel",
		nonChartDir,
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

	settings := cli.New()
	settings.SetNamespace("test-ns")
	i := New(&action.Configuration{}, settings, ports.HelmStartCallbacks{
		OnStart: func(_, _ string) { startCalled = true },
		OnSuccess: func(_, _ string) {
			successCalled = true
		},
		OnError: func(_, _ string, _ error) {
			errorCalled = true
		},
	})

	err := i.StartInstall(context.Background(), "rel", "unknown-chart", nil, ports.HelmStartOptions{
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
