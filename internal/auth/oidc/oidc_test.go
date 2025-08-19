package oidc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAudience(t *testing.T) {
	tests := []struct {
		name      string
		auth      *Auth
		claims    map[string]any
		expectErr bool
	}{
		{
			name:      "Empty config audience",
			auth:      &Auth{Audience: ""},
			claims:    map[string]any{"aud": "onyxia-onboarding"},
			expectErr: false,
		},
		{
			name:      "Valid string audience",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": "onyxia-onboarding"},
			expectErr: false,
		},
		{
			name:      "Valid array audience",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": []string{"service1", "onyxia-onboarding"}},
			expectErr: false,
		},
		{
			name:      "Valid array audience from interface slice",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": []any{"service1", "onyxia-onboarding"}},
			expectErr: false,
		},
		{
			name:      "Invalid array audience from interface slice with non-string",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": []any{"onyxia-onboarding", 42}},
			expectErr: true,
		},
		{
			name:      "Missing audience in token",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{},
			expectErr: true,
		},
		{
			name:      "Invalid string audience",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": "wrong-audience"},
			expectErr: true,
		},
		{
			name:      "Invalid array audience",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": []string{"service1", "other-service"}},
			expectErr: true,
		},
		{
			name:      "Unexpected format",
			auth:      &Auth{Audience: "onyxia-onboarding"},
			claims:    map[string]any{"aud": 123},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.validateAudience(tt.claims)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractClaim(t *testing.T) {
	auth := &Auth{}

	tests := []struct {
		name      string
		claims    map[string]any
		claimName string
		expected  string
		expectErr bool
	}{
		{"Valid claim", map[string]any{"username": "test-user"}, "username", "test-user", false},
		{"Missing claim", map[string]any{}, "username", "", true},
		{"Wrong format", map[string]any{"username": 123}, "username", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := auth.extractClaim(tt.claims, tt.claimName)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

// ⚠️ Règle "tout ou rien" : s'il y a un élément non-string -> on retourne nil
func TestExtractStringArray(t *testing.T) {
	auth := &Auth{}

	tests := []struct {
		name      string
		claims    map[string]any
		claimName string
		expected  []string
	}{
		{"Empty claim name", map[string]any{"groups": []any{"g1"}}, "", nil},
		{
			name:      "Valid array",
			claims:    map[string]any{"groups": []any{"g1", "g2"}},
			claimName: "groups",
			expected:  []string{"g1", "g2"},
		},
		{"Missing claim", map[string]any{}, "groups", nil},
		{"Wrong format", map[string]any{"groups": "not-an-array"}, "groups", nil},
		{
			name:      "Array with non-string values -> nil",
			claims:    map[string]any{"groups": []any{"g1", 42, true, "g2"}},
			claimName: "groups",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.extractStringArray(tt.claims, tt.claimName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
