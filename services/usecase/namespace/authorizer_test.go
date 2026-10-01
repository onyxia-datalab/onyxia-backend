package namespace

import "testing"

func TestAuthorizer_Allowed(t *testing.T) {
	a := NewAuthorizer("user-", "projet-")

	tests := []struct {
		name      string
		username  string
		groups    []string
		namespace string
		want      bool
	}{
		{"own personal namespace", "alice", nil, "user-alice", true},
		{"someone else's personal namespace", "alice", nil, "user-bob", false},
		{"member group namespace", "alice", []string{"data-team"}, "projet-data-team", true},
		{"non-member group namespace", "alice", []string{"data-team"}, "projet-other-team", false},
		{"empty namespace", "alice", nil, "", false},
		{"unprefixed namespace never matches", "alice", nil, "alice", false},
		{"group prefix alone is not a namespace", "alice", []string{""}, "projet-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.Allowed(tt.username, tt.groups, tt.namespace)
			if got != tt.want {
				t.Errorf("Allowed(%q, %v, %q) = %v, want %v", tt.username, tt.groups, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestAuthorizer_IsPersonal(t *testing.T) {
	a := NewAuthorizer("user-", "projet-")

	tests := []struct {
		name      string
		username  string
		namespace string
		want      bool
	}{
		{"own personal namespace", "alice", "user-alice", true},
		{"someone else's personal namespace", "alice", "user-bob", false},
		{"group namespace", "alice", "projet-alice", false},
		{"empty username", "", "user-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.IsPersonal(tt.username, tt.namespace); got != tt.want {
				t.Errorf("IsPersonal(%q, %q) = %v, want %v", tt.username, tt.namespace, got, tt.want)
			}
		})
	}
}

func TestAuthorizer_CanAccessService(t *testing.T) {
	a := NewAuthorizer("user-", "projet-")

	tests := []struct {
		name      string
		username  string
		namespace string
		owner     string
		share     bool
		want      bool
	}{
		{"personal namespace, any owner", "alice", "user-alice", "bob", false, true},
		{"group, own service", "alice", "projet-team", "alice", false, true},
		{"group, own service case-insensitive", "alice", "projet-team", "Alice", false, true},
		{"group, shared foreign service", "alice", "projet-team", "bob", true, true},
		{"group, unshared foreign service", "alice", "projet-team", "bob", false, false},
		{"empty username never matches empty owner", "", "projet-team", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.CanAccessService(tt.username, tt.namespace, tt.owner, tt.share)
			if got != tt.want {
				t.Errorf("CanAccessService(%q, %q, %q, %v) = %v, want %v",
					tt.username, tt.namespace, tt.owner, tt.share, got, tt.want)
			}
		})
	}
}
