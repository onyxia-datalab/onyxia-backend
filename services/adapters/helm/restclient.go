package helm

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// StaticRESTClientGetter hands Helm a fixed REST config. The discovery client
// and the REST mapper are built once and shared by every Helm action: Helm
// creates a configuration per action, and a fresh discovery cache would
// query the API server's resource lists again each time. Both are safe for
// concurrent use, and the mapper refreshes the cache on unknown kinds.
type StaticRESTClientGetter struct {
	config *rest.Config

	once      sync.Once
	discovery discovery.CachedDiscoveryInterface
	mapper    meta.RESTMapper
	err       error
}

var _ genericclioptions.RESTClientGetter = (*StaticRESTClientGetter)(nil)

func NewStaticRESTClientGetter(config *rest.Config) *StaticRESTClientGetter {
	return &StaticRESTClientGetter{config: config}
}

func (g *StaticRESTClientGetter) init() {
	g.once.Do(func() {
		client, err := discovery.NewDiscoveryClientForConfig(g.config)
		if err != nil {
			g.err = err
			return
		}
		g.discovery = memory.NewMemCacheClient(client)
		g.mapper = restmapper.NewDeferredDiscoveryRESTMapper(g.discovery)
	})
}

func (g *StaticRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.config, nil
}

func (g *StaticRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	g.init()
	return g.discovery, g.err
}

func (g *StaticRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	g.init()
	return g.mapper, g.err
}

func (g *StaticRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	// not used
	return nil
}
