// Package apperror defines the small set of sentinel errors shared by every
// API in this repo, and the machinery to turn one into an HTTP response.
//
// Each API keeps its own domain package deliberately decoupled from the
// others (see CLAUDE.md); what's shared here is not business logic, only
// the generic REST vocabulary ("this wasn't found", "you can't do that") and
// the wire format for reporting it. An API's own domain package re-exports
// the sentinels it needs (see e.g. services/domain/errors.go) so call sites
// elsewhere keep using their familiar domain.ErrXxx names.
package apperror

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New(
		"unauthorized",
	) // caller is not authenticated / not who they claim
	ErrForbidden     = errors.New("forbidden")      // business rule denial
	ErrAlreadyExists = errors.New("already exists") // idempotency/conflict
	ErrNotFound      = errors.New("not found")
	ErrNotSupported  = errors.New("operation not supported")
)

// Problem is an RFC7807-shaped error body. It's a plain struct rather than
// one of ogen's generated Problem types on purpose: each API generates its
// own (structurally identical) Problem type from its own spec, and this
// package is shared across APIs, so it can't depend on either. Client-side
// decoding only ever looks at the status code and these JSON field names —
// never at which Go type produced them — so this is wire-compatible with
// every generated Problem schema as long as the fields line up.
type Problem struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Classify maps a sentinel error from this package to its HTTP status.
// Errors that don't wrap one of ours fall back to ogen's own
// classification, which already knows the status for its framework-level
// errors (security failures, param/body decoding failures).
func Classify(err error) int {
	switch {
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, ErrNotSupported):
		return http.StatusUnprocessableEntity
	default:
		return ogenerrors.ErrorCode(err)
	}
}

// OgenHandler is an ogenerrors.ErrorHandler (the type every generated
// `oas.WithErrorHandler` expects) that writes a Problem body for err.
//
// ogen only encodes the response value a handler returns when it also
// returns a nil error; a non-nil error always bypasses that value and lands
// here instead. Handlers rely on that: on failure they return (nil, err)
// and let this function pick the status code, instead of each handler
// hand-building a typed response next to an error that would silently
// discard it. See https://ogen.dev/docs/misc/request_lifecycle for the
// exact dispatch rule this depends on.
func OgenHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	status := Classify(err)

	problem := Problem{
		Title:  http.StatusText(status),
		Status: status,
		Detail: err.Error(),
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if encErr := json.NewEncoder(w).Encode(&problem); encErr != nil {
		slog.ErrorContext(ctx, "failed to encode problem response", slog.Any("error", encErr))
	}
}
