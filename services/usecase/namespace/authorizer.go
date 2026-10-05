// Package namespace decides whether a user is allowed to act within a given
// Kubernetes namespace ("X-Onyxia-Project"). The backend talks to Kubernetes
// and Helm using its own service account, so nothing downstream enforces
// namespace boundaries per user — this check is the only thing standing
// between an authenticated user and any namespace on the cluster.
package namespace

import (
	"fmt"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// Authorizer verifies that a namespace belongs to the caller: either their
// personal namespace (derived from their username) or the namespace of one
// of the groups they belong to.
type Authorizer struct {
	NamespacePrefix      string
	GroupNamespacePrefix string
}

func NewAuthorizer(namespacePrefix, groupNamespacePrefix string) Authorizer {
	return Authorizer{
		NamespacePrefix:      namespacePrefix,
		GroupNamespacePrefix: groupNamespacePrefix,
	}
}

// Allowed reports whether the given user may act within namespace.
func (a Authorizer) Allowed(username string, groups []string, namespace string) bool {
	if namespace == "" {
		return false
	}

	if a.IsPersonal(username, namespace) {
		return true
	}

	for _, g := range groups {
		if g != "" && namespace == a.GroupNamespacePrefix+g {
			return true
		}
	}

	return false
}

// Check returns domain.ErrForbidden unless user may act within namespace.
func (a Authorizer) Check(user usercontext.User, namespace string) error {
	if !a.Allowed(user.Username, user.Groups, namespace) {
		return fmt.Errorf("%w: user %q may not act in namespace %q", domain.ErrForbidden, user.Username, namespace)
	}
	return nil
}

// IsPersonal reports whether namespace is the given user's personal
// namespace (as opposed to a group namespace).
func (a Authorizer) IsPersonal(username string, namespace string) bool {
	return username != "" && namespace == a.NamespacePrefix+username
}

// CanAccessService reports whether username may see and act on a service
// in namespace, given its recorded owner and share flag. Everything in the
// caller's personal namespace is accessible; in a group namespace only
// shared services and the caller's own (owner compared case-insensitively,
// as the Java API does) are. Callers must already have checked Allowed for
// namespace.
func (a Authorizer) CanAccessService(username, namespace, owner string, share bool) bool {
	return a.IsPersonal(username, namespace) ||
		share ||
		(username != "" && strings.EqualFold(owner, username))
}
