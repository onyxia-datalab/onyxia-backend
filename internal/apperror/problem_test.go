package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"invalid input", ErrInvalidInput, http.StatusBadRequest},
		{"wrapped invalid input", fmt.Errorf("wrap: %w", ErrInvalidInput), http.StatusBadRequest},
		{"unauthorized", ErrUnauthorized, http.StatusUnauthorized},
		{"forbidden", ErrForbidden, http.StatusForbidden},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"already exists", ErrAlreadyExists, http.StatusConflict},
		{"not supported", ErrNotSupported, http.StatusUnprocessableEntity},
		{"unclassified error defaults to 500", errors.New("boom"), http.StatusInternalServerError},
		{
			"ogen security error maps to 401",
			&ogenerrors.SecurityError{Err: errors.New("missing token")},
			http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Classify(tt.err))
		})
	}
}

func TestOgenHandler_WritesStatusAndBody(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/services", nil)

	OgenHandler(r.Context(), w, r, ErrNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"status":404`)
	assert.Contains(t, w.Body.String(), ErrNotFound.Error())
}
