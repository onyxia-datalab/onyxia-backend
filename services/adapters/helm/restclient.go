package helm

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type StaticRESTClientGetter struct {
	config *rest.Config
}

var _ genericclioptions.RESTClientGetter = (*StaticRESTClientGetter)(nil)

func (g *StaticRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.config, nil
}

func (g *StaticRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	client, err := discovery.NewDiscoveryClientForConfig(g.config)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(client), nil
}

func (g *StaticRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	disco, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(disco), nil
}

func (g *StaticRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	// not used
	return nil
}
