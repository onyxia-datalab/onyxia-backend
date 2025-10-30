package tools

import (
	"net/url"
	"strings"
)

func MustParseURL(s string) (u url.URL) {
	p, _ := url.Parse(strings.TrimSpace(s))
	if p != nil {
		return *p
	}
	return url.URL{}
}
