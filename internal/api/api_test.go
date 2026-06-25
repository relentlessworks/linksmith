package api

import (
	"net/http"
	"os"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/relentlessworks/linksmith/internal/auth"
	"github.com/relentlessworks/linksmith/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	tmpFile, err := os.CreateTemp("", "linksmith-test-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	s, err := store.New(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	a := auth.New("test-secret")
	return NewServer(s, a)
}

func TestHelp(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "LinkSmith") {
		t.Error("help text should contain 'LinkSmith'")
	}
}

func TestHealth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthFlow(t *testing.T) {
	srv := setupTestServer(t)

	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "code=") {
		t.Fatalf("expected code in response, got: %s", w.Body.String())
	}

	// Extract code
	codeStr := w.Body.String()
	idx := strings.Index(codeStr, "code=")
	if idx == -1 {
		t.Fatal("could not find code in response")
	}
	code := strings.TrimSpace(codeStr[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "token=ls_") {
		t.Fatalf("expected token in response, got: %s", w2.Body.String())
	}
}

func TestCreateAndGetLink(t *testing.T) {
	srv := setupTestServer(t)

	// Get token first
	token := getTestToken(t, srv)

	// Create link
	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "handle=link_") {
		t.Fatalf("expected handle in response, got: %s", w.Body.String())
	}

	// Extract handle
	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	// Handle may be followed by other fields on the same line
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Get link
	req2 := httptest.NewRequest("GET", "/api/links/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "https://example.com") {
		t.Errorf("expected URL in response, got: %s", w2.Body.String())
	}
}

func TestListLinks(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create a link
	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	// List links
	req2 := httptest.NewRequest("GET", "/api/links", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "handle=link_") {
		t.Errorf("expected link in list, got: %s", w2.Body.String())
	}
}

func TestDeleteLink(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create link
	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	// Handle may be followed by other fields on the same line
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Delete link
	req2 := httptest.NewRequest("DELETE", "/api/links/"+handle, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "deleted") {
		t.Errorf("expected 'deleted' in response, got: %s", w2.Body.String())
	}
}

func TestRedirect(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create link
	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	body := w.Body.String()
	idx := strings.Index(body, "handle=")
	handle := strings.TrimSpace(body[idx+7:])
	// Handle may be followed by other fields on the same line
	if sp := strings.Index(handle, " "); sp != -1 {
		handle = handle[:sp]
	}

	// Redirect
	req2 := httptest.NewRequest("GET", "/l/"+handle, nil)
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w2.Code)
	}
	if w2.Header().Get("Location") != "https://example.com" {
		t.Errorf("expected redirect to https://example.com, got %s", w2.Header().Get("Location"))
	}
}

func TestInvalidURL(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	form := url.Values{"url": {"not-a-url"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestNoAuth(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/links", nil)
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hint:") {
		t.Errorf("expected hint in error response, got: %s", w.Body.String())
	}
}

func TestJSONFormat(t *testing.T) {
	srv := setupTestServer(t)
	token := getTestToken(t, srv)

	// Create link
	form := url.Values{"url": {"https://example.com"}}
	req := httptest.NewRequest("POST", "/api/links", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected JSON content type, got: %s", w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "\"handle\"") {
		t.Errorf("expected JSON with handle field, got: %s", w.Body.String())
	}
}

func getTestToken(t *testing.T, srv *Server) string {
	// Request OTP
	form := url.Values{"email": {"agent@test.com"}, "workspace": {"ws_test"}}
	req := httptest.NewRequest("POST", "/auth/request", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("auth/request failed: %d %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	idx := strings.Index(body, "code=")
	if idx == -1 {
		t.Fatalf("no code in response: %s", body)
	}
	code := strings.TrimSpace(body[idx+5:])

	// Verify OTP
	form2 := url.Values{"email": {"agent@test.com"}, "code": {code}}
	req2 := httptest.NewRequest("POST", "/auth/verify", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	srv.Router(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("auth/verify failed: %d %s", w2.Code, w2.Body.String())
	}

	body2 := w2.Body.String()
	idx2 := strings.Index(body2, "token=")
	if idx2 == -1 {
		t.Fatalf("no token in response: %s", body2)
	}
	token := strings.TrimSpace(body2[idx2+6:])
	// Token may be followed by other fields on the same line
	if sp := strings.Index(token, " "); sp != -1 {
		token = token[:sp]
	}

	return token
}
