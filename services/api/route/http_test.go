package route

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/apperror"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/stretchr/testify/require"
)

// These tests drive the *real* generated ogen server (routing, security
// dispatch, response encoding) instead of calling controller methods
// directly. A controller unit test only checks the Go value it returns; it
// can't see that ogen silently drops a typed error response and falls back
// to a generic 500 whenever the handler also returns a non-nil error. Only
// a request that goes all the way through oas.NewServer catches that class
// of bug — which is exactly how it slipped through to review in the first
// place.

// testSecurityHandler stands in for OIDC verification: it trusts two plain
// headers instead of validating a token, so tests can drive the namespace
// authorizer with an arbitrary username/groups without standing up a real
// IdP.
type testSecurityHandler struct {
	writer usercontext.Writer
}

func (h testSecurityHandler) HandleOidc(
	ctx context.Context,
	_ api.OperationName,
	t api.Oidc,
) (context.Context, error) {
	username := t.Request.Header.Get("X-Test-User")
	if username == "" {
		return ctx, errors.New("missing X-Test-User header")
	}
	var groups []string
	if g := t.Request.Header.Get("X-Test-Groups"); g != "" {
		groups = strings.Split(g, ",")
	}
	return h.writer.WithUser(ctx, &usercontext.User{Username: username, Groups: groups}), nil
}

// --- stubs implementing the domain interfaces --------------------------------

type stubLifecycle struct {
	startErr   error
	suspendErr error
	resumeErr  error
	deleteErr  error
	sharedErr  error
}

func (s *stubLifecycle) Start(context.Context, domain.StartRequest) (domain.StartResponse, error) {
	return domain.StartResponse{}, s.startErr
}
func (s *stubLifecycle) Suspend(context.Context, domain.SuspendRequest) error { return s.suspendErr }
func (s *stubLifecycle) Resume(context.Context, domain.ResumeRequest) error   { return s.resumeErr }
func (s *stubLifecycle) Delete(context.Context, domain.DeleteRequest) error   { return s.deleteErr }
func (s *stubLifecycle) SetShared(context.Context, domain.SetSharedRequest) error {
	return s.sharedErr
}

type stubQuery struct {
	getResp  domain.Service
	getErr   error
	listResp []domain.Service
	listErr  error
}

func (s *stubQuery) GetService(context.Context, string, string) (domain.Service, error) {
	return s.getResp, s.getErr
}
func (s *stubQuery) ListServices(context.Context, string) ([]domain.Service, error) {
	return s.listResp, s.listErr
}

// stubCatalog is only wired to satisfy NewHandler's dependencies — the tests
// in this file don't exercise the catalog routes.
type stubCatalog struct{}

func (stubCatalog) ListPublicCatalogs(context.Context) ([]domain.Catalog, error) { return nil, nil }
func (stubCatalog) ListUserCatalogs(context.Context) ([]domain.Catalog, error)   { return nil, nil }
func (stubCatalog) GetPackage(context.Context, string, string) (domain.Package, error) {
	return domain.Package{}, nil
}
func (stubCatalog) GetAvailableVersions(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (stubCatalog) GetPackageSchema(context.Context, string, string, string) ([]byte, error) {
	return nil, nil
}
func (stubCatalog) CheckSharingAllowed(context.Context, string) error { return nil }

// --- test server ---------------------------------------------------------

const testNamespace = "user-alice"

func newTestServer(t *testing.T, lifecycle *stubLifecycle, query *stubQuery) *httptest.Server {
	t.Helper()

	userReader, userWriter := usercontext.NewUserContext()
	namespaceAuthz := namespace.NewAuthorizer("user-", "projet-")

	h := NewHandler(
		controller.NewInstallController(lifecycle, userReader, namespaceAuthz),
		controller.NewCatalogController(stubCatalog{}, userReader),
		controller.NewServiceQueryController(query, userReader, namespaceAuthz),
	)

	srv, err := api.NewServer(
		h,
		testSecurityHandler{writer: userWriter},
		api.WithErrorHandler(apperror.OgenHandler),
	)
	require.NoError(t, err)

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

func doRequest(t *testing.T, ts *httptest.Server, method, path, username string, body any) *http.Response {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, ts.URL+path, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if username != "" {
		req.Header.Set("X-Test-User", username)
	}
	req.Header.Set("X-Onyxia-Project", testNamespace)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// --- InstallService (PUT /api/services/{releaseId}) --------------------------

func TestHTTP_InstallService(t *testing.T) {
	validBody := map[string]any{
		"catalogId":   "my-catalog",
		"packageName": "jupyter",
		"name":        "my-jupyter",
		"options":     map[string]any{},
	}

	tests := []struct {
		name       string
		startErr   error
		wantStatus int
	}{
		{"success", nil, http.StatusAccepted},
		{"invalid input maps to 400", domain.ErrInvalidInput, http.StatusBadRequest},
		{"forbidden maps to 403", domain.ErrForbidden, http.StatusForbidden},
		{"already exists maps to 409", domain.ErrAlreadyExists, http.StatusConflict},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{startErr: tt.startErr}, &stubQuery{})
			resp := doRequest(t, ts, http.MethodPut, "/api/services/my-release", "alice", validBody)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestHTTP_InstallService_ForeignNamespaceIs403(t *testing.T) {
	ts := newTestServer(t, &stubLifecycle{}, &stubQuery{})

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/services/my-release", bytes.NewReader(
		mustJSON(t, map[string]any{
			"catalogId": "my-catalog", "packageName": "jupyter", "name": "n", "options": map[string]any{},
		}),
	))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "alice")
	req.Header.Set("X-Onyxia-Project", "user-bob")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// --- SetServiceSuspended (PUT /api/services/{releaseId}/suspended) -----------

func TestHTTP_SetServiceSuspended(t *testing.T) {
	tests := []struct {
		name       string
		suspendErr error
		wantStatus int
	}{
		{"success", nil, http.StatusNoContent},
		{"not found maps to 404", domain.ErrNotFound, http.StatusNotFound},
		{"forbidden maps to 403", domain.ErrForbidden, http.StatusForbidden},
		{"not supported maps to 422", domain.ErrNotSupported, http.StatusUnprocessableEntity},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{suspendErr: tt.suspendErr}, &stubQuery{})
			resp := doRequest(t, ts, http.MethodPut, "/api/services/my-release/suspended", "alice",
				map[string]any{"suspended": true})
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// --- DeleteService (DELETE /api/services/{releaseId}) ------------------------

func TestHTTP_DeleteService(t *testing.T) {
	tests := []struct {
		name       string
		deleteErr  error
		wantStatus int
	}{
		{"success", nil, http.StatusNoContent},
		{"not found maps to 404", domain.ErrNotFound, http.StatusNotFound},
		{"forbidden maps to 403", domain.ErrForbidden, http.StatusForbidden},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{deleteErr: tt.deleteErr}, &stubQuery{})
			resp := doRequest(t, ts, http.MethodDelete, "/api/services/my-release", "alice", nil)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// --- SetServiceShared (PUT /api/services/{releaseId}/shared) -----------------

func TestHTTP_SetServiceShared(t *testing.T) {
	tests := []struct {
		name       string
		sharedErr  error
		wantStatus int
	}{
		{"success", nil, http.StatusNoContent},
		{"not owner maps to 403", domain.ErrForbidden, http.StatusForbidden},
		{"not found maps to 404", domain.ErrNotFound, http.StatusNotFound},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{sharedErr: tt.sharedErr}, &stubQuery{})
			resp := doRequest(t, ts, http.MethodPut, "/api/services/my-release/shared", "alice",
				map[string]any{"shared": true})
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// --- GetService (GET /api/services/{releaseId}) ------------------------------

func TestHTTP_GetService(t *testing.T) {
	tests := []struct {
		name       string
		getErr     error
		wantStatus int
	}{
		{"success", nil, http.StatusOK},
		{"not found maps to 404", domain.ErrNotFound, http.StatusNotFound},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{}, &stubQuery{
				getResp: domain.Service{
					ReleaseID: "my-release", Status: domain.ServiceStatusRunning,
					FriendlyName: "svc", Owner: "alice", CatalogID: "cat", Share: false,
				},
				getErr: tt.getErr,
			})
			resp := doRequest(t, ts, http.MethodGet, "/api/services/my-release", "alice", nil)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestHTTP_GetService_ForeignNamespaceIs403(t *testing.T) {
	ts := newTestServer(t, &stubLifecycle{}, &stubQuery{})

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/services/my-release", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-User", "alice")
	req.Header.Set("X-Onyxia-Project", "user-bob")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// --- ListServices (GET /api/services) -----------------------------------------

func TestHTTP_ListServices(t *testing.T) {
	tests := []struct {
		name       string
		listErr    error
		wantStatus int
	}{
		{"success", nil, http.StatusOK},
		{"unexpected error maps to 500", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &stubLifecycle{}, &stubQuery{listErr: tt.listErr})
			resp := doRequest(t, ts, http.MethodGet, "/api/services", "alice", nil)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}
