package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Define a sample config structure for testing.
type testConfig struct {
	Server struct {
		Port        int    `mapstructure:"port"`
		ContextPath string `mapstructure:"contextPath"`
	} `mapstructure:"server"`
	Quota struct {
		RequestsCPU string `mapstructure:"requests.cpu"`
		CountPods   string `mapstructure:"count/pods"`
	} `mapstructure:"quota"`
}

func TestLoad_WithDefaultsOnly(t *testing.T) {
	defaultYAML := []byte(`
server:
  port: 8080
  contextPath: /
quota:
  requests.cpu: "500m"
  count/pods: "10"
`)

	cfg, err := Load[testConfig](defaultYAML, "")
	require.NoError(t, err)

	require.Equal(t, 8080, cfg.Server.Port)
	require.Equal(t, "/", cfg.Server.ContextPath)
	require.Equal(t, "500m", cfg.Quota.RequestsCPU)
	require.Equal(t, "10", cfg.Quota.CountPods)
}

func TestLoad_WithExternalFile(t *testing.T) {
	defaultYAML := []byte(`
server:
  port: 8080
quota:
  requests.cpu: "200m"
  count/pods: "5"
`)

	tmpFile := filepath.Join(t.TempDir(), "env.yaml")
	err := os.WriteFile(tmpFile, []byte(`
server:
  port: 9090
quota:
  requests.cpu: "750m"
  count/pods: "15"
`), 0o600)
	require.NoError(t, err)

	cfg, err := Load[testConfig](defaultYAML, tmpFile)
	require.NoError(t, err)

	require.Equal(t, 9090, cfg.Server.Port)
	require.Equal(t, "750m", cfg.Quota.RequestsCPU)
	require.Equal(t, "15", cfg.Quota.CountPods)
}

func TestLoad_WithEnvOverride(t *testing.T) {
	defaultYAML := []byte(`
quota:
  requests.cpu: "100m"
  count/pods: "1"
`)

	require.NoError(t, os.Setenv("QUOTA_REQUESTS_CPU", "999m"))
	defer func() {
		require.NoError(t, os.Unsetenv("QUOTA_REQUESTS_CPU"))
	}()

	cfg, err := Load[testConfig](defaultYAML, "")
	require.NoError(t, err)

	require.Equal(t, "999m", cfg.Quota.RequestsCPU)
	require.Equal(t, "1", cfg.Quota.CountPods)
}
