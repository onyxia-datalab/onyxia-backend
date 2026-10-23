package domain

import (
	"regexp"
	"slices"
	"strings"
)

// Resource is a Kubernetes object manifest, chosen by the platform admin,
// applied in every onboarded namespace. Its string values may contain
// placeholders, replaced at each onboarding by Render.
type Resource map[string]any

// Placeholders that may appear in the string values of a Resource, as
// ${onyxia.<name>}:
//   - ${onyxia.namespace}: the onboarded namespace;
//   - ${onyxia.username}: the user who triggered the onboarding (for a
//     group namespace, any member of the group);
//   - ${onyxia.group}: the single group named in the onboarding request
//     (not the user's whole list of groups), empty for a user namespace.
//
// The onyxia. prefix keeps them apart from other ${...} a manifest may
// legitimately hold, such as shell variables in a script: those are left
// untouched. A dot is not valid in a shell variable name, so no script
// uses this form, and envsubst does not expand it either.
const (
	PlaceholderNamespace = "namespace"
	PlaceholderUsername  = "username"
	PlaceholderGroup     = "group"
)

var (
	placeholderPattern = regexp.MustCompile(`\$\{onyxia\.([^}]*)\}`)
	knownPlaceholders  = []string{PlaceholderNamespace, PlaceholderUsername, PlaceholderGroup}
)

// Render returns a deep copy of the resource with the placeholders of its
// string values replaced by values. Substitution happens on the parsed
// object, never on the YAML text, so a value (a username comes from the
// user's token) can only ever end up inside a string: it cannot add fields.
func (r Resource) Render(values map[string]string) Resource {
	replacer := func(s string) string {
		return placeholderPattern.ReplaceAllStringFunc(s, func(match string) string {
			return values[placeholderPattern.FindStringSubmatch(match)[1]]
		})
	}
	return renderValue(map[string]any(r), replacer).(map[string]any)
}

// UnknownPlaceholders lists the ${onyxia.*} placeholders of the resource
// that Render does not know, so that typos are caught at startup.
func (r Resource) UnknownPlaceholders() []string {
	var unknown []string
	renderValue(map[string]any(r), func(s string) string {
		for _, match := range placeholderPattern.FindAllStringSubmatch(s, -1) {
			if !slices.Contains(knownPlaceholders, match[1]) && !slices.Contains(unknown, match[0]) {
				unknown = append(unknown, match[0])
			}
		}
		return s
	})
	return unknown
}

func renderValue(value any, replace func(string) string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = renderValue(item, replace)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = renderValue(item, replace)
		}
		return out
	case string:
		if !strings.Contains(v, "${onyxia.") {
			return v
		}
		return replace(v)
	default:
		return v
	}
}
