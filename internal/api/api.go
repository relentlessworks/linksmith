package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/relentlessworks/linksmith/internal/auth"
	"github.com/relentlessworks/linksmith/internal/models"
	"github.com/relentlessworks/linksmith/internal/store"
)

// Server is the API server.
type Server struct {
	store *store.Store
	auth  *auth.AuthService
}

// NewServer creates a new API server.
func NewServer(s *store.Store, a *auth.AuthService) *Server {
	return &Server{store: s, auth: a}
}

// Router is the main HTTP router.
func (s *Server) Router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// --- Public routes ---

	// Help / agent manual
	if path == "/help" || path == "/.well-known/agent.md" {
		s.handleHelp(w, r)
		return
	}

	// Health check
	if path == "/health" {
		s.handleHealth(w, r)
		return
	}

	// Auth: request OTP
	if path == "/auth/request" && r.Method == "POST" {
		s.handleRequestOTP(w, r)
		return
	}

	// Auth: verify OTP
	if path == "/auth/verify" && r.Method == "POST" {
		s.handleVerifyOTP(w, r)
		return
	}

	// Redirect: /l/<handle>
	if strings.HasPrefix(path, "/l/") {
		s.handleRedirect(w, r)
		return
	}

	// --- Authenticated routes ---
	// All /api/* routes require a bearer token

	if strings.HasPrefix(path, "/api/") {
		s.handleAPI(w, r)
		return
	}

	// Root: if nothing matched, show help
	if path == "/" {
		s.handleHelp(w, r)
		return
	}

	s.errorResponse(w, r, http.StatusNotFound, "not found", "GET /help to see available endpoints")
}

// --- Helpers ---

func (s *Server) wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if q := r.URL.Query().Get("format"); q == "json" {
		return true
	}
	return false
}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, text string, data interface{}) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(data)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}

func (s *Server) errorResponse(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": msg, "hint": hint})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, "error: %s\nhint: %s\n", msg, hint)
}

func (s *Server) getBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (s *Server) authenticate(r *http.Request) (string, error) {
	token := s.getBearerToken(r)
	if token == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	t, err := s.store.GetToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid or expired token")
	}
	if time.Now().After(t.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return t.Workspace, nil
}

func isValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// --- Handlers ---

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `LinkSmith — Agentic-First Link Shortener
=========================================

LinkSmith is a link shortener designed for AI agents. The API is the product.
No UI, no SDK. Plain text by default, JSON on demand.

AUTHENTICATION
--------------
1. POST /auth/request   body: email=<email>&workspace=<handle>
   → Sends a 6-digit OTP code (returned in plain text for local dev).
2. POST /auth/verify     body: email=<email>&code=<code>
   → Returns a long-lived bearer token. Use it in Authorization: Bearer <token>.

SHORTEN A LINK
--------------
POST /api/links          body: url=<https://example.com>
   → Returns: handle=link_a1b2c url=https://example.com

LIST LINKS
----------
GET /api/links           → One link per line: handle=link_a1b2c url=https://example.com clicks=0

GET A LINK
----------
GET /api/links/<handle>  → handle=link_a1b2c url=https://example.com clicks=0

DELETE A LINK
-------------
DELETE /api/links/<handle>

REDIRECT
--------
GET /l/<handle>          → 301 redirect to the original URL

WORKSPACE INFO
--------------
GET /api/workspace        → name=My Workspace plan=free links=42

FORMATS
-------
- Plain text (default): one labeled, grepable line per record.
- JSON: add Accept: application/json or ?format=json to any request.

ERRORS
------
4xx responses include an "error" and a "hint" field to guide you.

EXAMPLES
--------
  curl -X POST http://localhost:8080/auth/request -d 'email=me@example.com&workspace=ws_demo'
  curl -X POST http://localhost:8080/auth/verify -d 'email=me@example.com&code=123456'
  curl -X POST http://localhost:8080/api/links -H 'Authorization: Bearer ls_xxx' -d 'url=https://example.com'
  curl http://localhost:8080/api/links -H 'Authorization: Bearer ls_xxx'
  curl http://localhost:8080/l/link_a1b2c

STORAGE
-------
Data is persisted to a JSON file (default: linksmith.json). Zero external dependencies.
`
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, help)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeResponse(w, r, http.StatusOK, "ok", map[string]string{"status": "ok"})
}

func (s *Server) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	workspace := r.FormValue("workspace")

	if email == "" || workspace == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or workspace",
			"POST with email=<your-email>&workspace=<handle> (e.g. ws_demo)")
		return
	}

	// Auto-create workspace if it doesn't exist
	exists, err := s.store.WorkspaceExists(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}
	if !exists {
		if err := s.store.CreateWorkspace(workspace, workspace); err != nil {
			s.errorResponse(w, r, http.StatusInternalServerError, "failed to create workspace", "try a different workspace handle")
			return
		}
	}

	code := s.auth.GenerateOTP()
	if err := s.store.SaveOTP(email, code, workspace, auth.OTPExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to save OTP", "try again")
		return
	}

	// In production, email this. In dev, return it directly.
	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("otp_sent=true email=%s code=%s", email, code),
		map[string]string{"status": "otp_sent", "email": email, "code": code, "hint": "use POST /auth/verify with this code to get a token"},
	)
}

func (s *Server) handleVerifyOTP(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	code := r.FormValue("code")

	if email == "" || code == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing email or code",
			"POST with email=<your-email>&code=<6-digit-code>")
		return
	}

	workspace, expiresAt, err := s.store.GetOTP(email, code)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, "invalid OTP code",
			"request a new OTP via POST /auth/request")
		return
	}

	if time.Now().After(expiresAt) {
		s.store.DeleteOTP(email, code)
		s.errorResponse(w, r, http.StatusUnauthorized, "OTP expired",
			"request a new OTP via POST /auth/request")
		return
	}

	// Delete used OTP
	s.store.DeleteOTP(email, code)

	// Generate token
	token := s.auth.GenerateToken(workspace)
	if err := s.store.CreateToken(token, workspace, auth.TokenExpiry()); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create token", "try again")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("token=%s workspace=%s", token, workspace),
		map[string]string{"token": token, "workspace": workspace, "hint": "use this token in Authorization: Bearer header"},
	)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.authenticate(r)
	if err != nil {
		s.errorResponse(w, r, http.StatusUnauthorized, err.Error(),
			"POST /auth/request with email and workspace, then POST /auth/verify with the code")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch {
	case path == "/links" && r.Method == "POST":
		s.handleCreateLink(w, r, workspace)
	case path == "/links" && r.Method == "GET":
		s.handleListLinks(w, r, workspace)
	case strings.HasPrefix(path, "/links/") && r.Method == "GET":
		s.handleGetLink(w, r, workspace)
	case strings.HasPrefix(path, "/links/") && r.Method == "DELETE":
		s.handleDeleteLink(w, r, workspace)
	case path == "/workspace" && r.Method == "GET":
		s.handleGetWorkspace(w, r, workspace)
	default:
		s.errorResponse(w, r, http.StatusNotFound, "endpoint not found",
			"GET /help to see available endpoints")
	}
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request, workspace string) {
	rawURL := r.FormValue("url")
	if rawURL == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing url",
			"POST with url=<https://example.com>")
		return
	}

	if !isValidURL(rawURL) {
		s.errorResponse(w, r, http.StatusBadRequest, "invalid url",
			"provide a full http:// or https:// URL")
		return
	}

	handle := auth.GenerateHandle("link")
	if err := s.store.CreateLink(handle, rawURL, workspace); err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "failed to create link", "try again")
		return
	}

	link := &models.Link{Handle: handle, URL: rawURL, Workspace: workspace}
	s.writeResponse(w, r, http.StatusCreated,
		fmt.Sprintf("handle=%s url=%s", handle, rawURL),
		link,
	)
}

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request, workspace string) {
	links, err := s.store.ListLinks(workspace, 50)
	if err != nil {
		s.errorResponse(w, r, http.StatusInternalServerError, "database error", "try again")
		return
	}

	if s.wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if links == nil {
			links = []*models.Link{}
		}
		json.NewEncoder(w).Encode(links)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if len(links) == 0 {
		fmt.Fprintln(w, "no links found. POST /api/links with url=<https://example.com> to create one.")
		return
	}
	for _, l := range links {
		fmt.Fprintf(w, "handle=%s url=%s clicks=%d\n", l.Handle, l.URL, l.Clicks)
	}
}

func (s *Server) handleGetLink(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/links/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /api/links/<handle>")
		return
	}

	link, err := s.store.GetLink(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "link not found",
			"GET /api/links to list all links")
		return
	}

	// Verify ownership
	if link.Workspace != workspace {
		s.errorResponse(w, r, http.StatusNotFound, "link not found",
			"this link belongs to a different workspace")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s url=%s clicks=%d", link.Handle, link.URL, link.Clicks),
		link,
	)
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request, workspace string) {
	handle := strings.TrimPrefix(r.URL.Path, "/api/links/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "DELETE /api/links/<handle>")
		return
	}

	if err := s.store.DeleteLink(handle, workspace); err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "link not found",
			"GET /api/links to list all links")
		return
	}

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("deleted handle=%s", handle),
		map[string]string{"status": "deleted", "handle": handle},
	)
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request, workspace string) {
	ws, err := s.store.GetWorkspace(workspace)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "workspace not found", "create it via POST /auth/request")
		return
	}

	links, _ := s.store.ListLinks(workspace, 1000)
	linkCount := len(links)

	s.writeResponse(w, r, http.StatusOK,
		fmt.Sprintf("handle=%s name=%s plan=%s links=%d", ws.Handle, ws.Name, ws.Plan, linkCount),
		map[string]string{"handle": ws.Handle, "name": ws.Name, "plan": ws.Plan, "links": fmt.Sprintf("%d", linkCount)},
	)
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	handle := strings.TrimPrefix(r.URL.Path, "/l/")
	if handle == "" {
		s.errorResponse(w, r, http.StatusBadRequest, "missing handle", "GET /l/<handle>")
		return
	}

	link, err := s.store.GetLink(handle)
	if err != nil {
		s.errorResponse(w, r, http.StatusNotFound, "link not found",
			"check the handle or GET /api/links to list links")
		return
	}

	// Increment click count
	s.store.IncrementClicks(handle)

	http.Redirect(w, r, link.URL, http.StatusMovedPermanently)
}
