package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestStaticRESTClientGetterSharesDiscoveryAndMapper(t *testing.T) {
	g := NewStaticRESTClientGetter(&rest.Config{Host: "https://fake-cluster"})

	d1, err := g.ToDiscoveryClient()
	require.NoError(t, err)
	d2, err := g.ToDiscoveryClient()
	require.NoError(t, err)
	m1, err := g.ToRESTMapper()
	require.NoError(t, err)
	m2, err := g.ToRESTMapper()
	require.NoError(t, err)

	assert.Same(t, d1, d2)
	assert.Same(t, m1, m2)
}
