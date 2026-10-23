package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalizedStringPlain(t *testing.T) {
	s, err := NewLocalizedString("hello")

	require.NoError(t, err)
	assert.True(t, s.IsPlain())
	assert.False(t, s.IsMulti())
	assert.Equal(t, LocalizedStringTypePlain, s.Type())
	plain, ok := s.GetPlain()
	assert.True(t, ok)
	assert.Equal(t, "hello", plain)
	multi, ok := s.GetMulti()
	assert.False(t, ok)
	assert.Nil(t, multi)
}

func TestNewLocalizedStringMulti(t *testing.T) {
	for name, v := range map[string]any{
		"MultiLangString":   MultiLangString{"en": "hello", "fr": "bonjour"},
		"map[string]string": map[string]string{"en": "hello", "fr": "bonjour"},
	} {
		t.Run(name, func(t *testing.T) {
			s, err := NewLocalizedString(v)

			require.NoError(t, err)
			assert.True(t, s.IsMulti())
			assert.False(t, s.IsPlain())
			assert.Equal(t, LocalizedStringTypeMulti, s.Type())
			multi, ok := s.GetMulti()
			assert.True(t, ok)
			assert.Equal(t, MultiLangString{"en": "hello", "fr": "bonjour"}, multi)
			plain, ok := s.GetPlain()
			assert.False(t, ok)
			assert.Empty(t, plain)
		})
	}
}

func TestNewLocalizedStringInvalid(t *testing.T) {
	for name, v := range map[string]any{
		"empty MultiLangString":   MultiLangString{},
		"empty map[string]string": map[string]string{},
		"nil":                     nil,
		"unsupported type":        42,
	} {
		t.Run(name, func(t *testing.T) {
			s, err := NewLocalizedString(v)

			assert.Error(t, err)
			assert.False(t, s.IsPlain())
			assert.False(t, s.IsMulti())
		})
	}
}
