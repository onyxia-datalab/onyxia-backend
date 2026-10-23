package helm

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/kube/fake"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// newMemoryAdapter returns an adapter backed by an in-memory Helm storage
// holding rels.
func newMemoryAdapter(t *testing.T, rels ...*releasev1.Release) (*Helm, *action.Configuration) {
	t.Helper()
	cfg := action.NewConfiguration()
	cfg.Releases = storage.Init(driver.NewMemory())
	cfg.KubeClient = &fake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard}
	for _, rel := range rels {
		require.NoError(t, cfg.Releases.Create(rel))
	}

	i := newAdapter(t)
	i.configForNamespace = func(string) (*action.Configuration, error) { return cfg, nil }
	return i, cfg
}

func TestSuspendReleaseNotFound(t *testing.T) {
	i, _ := newMemoryAdapter(t)

	assert.ErrorIs(t, i.SuspendRelease(context.Background(), "test-ns", "missing"), domain.ErrNotFound)
}

func TestGetReleaseStateMissingRelease(t *testing.T) {
	i, _ := newMemoryAdapter(t)

	state, err := i.GetReleaseState(context.Background(), "test-ns", "missing")

	require.NoError(t, err)
	assert.Equal(t, ports.ReleaseState{Exists: false}, state)
}

func TestReleaseOperationsPropagateConfigError(t *testing.T) {
	boom := errors.New("boom")
	i := newAdapter(t)
	i.configForNamespace = func(string) (*action.Configuration, error) { return nil, boom }
	ctx := context.Background()

	assert.ErrorIs(t, i.SuspendRelease(ctx, "test-ns", "rel"), boom)
	assert.ErrorIs(t, i.ResumeRelease(ctx, "test-ns", "rel"), boom)
	assert.ErrorIs(t, i.UninstallRelease(ctx, "test-ns", "rel"), boom)
	_, err := i.GetReleaseState(ctx, "test-ns", "rel")
	assert.ErrorIs(t, err, boom)
	_, err = i.ListReleaseStates(ctx, "test-ns")
	assert.ErrorIs(t, err, boom)
}

func TestUninstallReleaseRequiresName(t *testing.T) {
	assert.ErrorContains(t, newAdapter(t).UninstallRelease(context.Background(), "test-ns", ""), "releaseName is required")
}

func TestListReleaseStatesHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newAdapter(t).ListReleaseStates(ctx, "test-ns")

	assert.ErrorIs(t, err, context.Canceled)
}
