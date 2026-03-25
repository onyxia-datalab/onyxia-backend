package helm

import (
	"fmt"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
)

// Client holds shared Helm SDK state. Create one instance at startup and
// inject it into both NewPackageRepository and NewReleaseGtw.
type Client struct {
	Settings       *cli.EnvSettings
	RegistryClient *registry.Client
	Getters        getter.Providers
}

func NewClient(cacheDir string) (*Client, error) {
	settings := cli.New()
	if cacheDir != "" {
		settings.RepositoryCache = cacheDir
	}

	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	return &Client{
		Settings:       settings,
		RegistryClient: registryClient,
		Getters:        getter.All(settings),
	}, nil
}
