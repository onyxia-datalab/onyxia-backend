package tools

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURLOrEmpty(t *testing.T) {
	u := ParseURLOrEmpty("  https://jupyter.org/icon.svg  ")
	assert.Equal(t, "https://jupyter.org/icon.svg", u.String())
	assert.Equal(t, url.URL{}, ParseURLOrEmpty("http://[::1"))
	assert.Equal(t, url.URL{}, ParseURLOrEmpty(""))
}

func TestDeref(t *testing.T) {
	v := 3
	assert.Equal(t, 3, Deref(&v))
	assert.Equal(t, 0, Deref[int](nil))
	assert.Equal(t, "", Deref[string](nil))
}
