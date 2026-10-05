package tools

import (
	"net/url"
	"strings"
)

// ParseURLOrEmpty parses s, ignoring surrounding spaces, and returns the
// zero URL when s is not a valid URL (chart metadata is not trusted to be).
func ParseURLOrEmpty(s string) url.URL {
	p, _ := url.Parse(strings.TrimSpace(s))
	if p != nil {
		return *p
	}
	return url.URL{}
}
