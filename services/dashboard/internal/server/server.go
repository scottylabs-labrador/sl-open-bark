// Package server is the maintainer dashboard + agent console (design 6.3, 10.2, 11.4): read views
// over the platform's Postgres state (registry, audit, pending approvals) plus a console that drives
// the agent runtime. It serves an embedded single-page UI and a small JSON API, gated to maintainers
// by a token.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

//go:embed web
var webFS embed.FS

// Store is the slice of the platform repository the dashboard needs (defined at the consumer).
type Store interface {
	ListServers(ctx context.Context) ([]store.Server, error)
	SetServerLifecycle(ctx context.Context, name, lifecycle string) error
	SetServerEnabled(ctx context.Context, name string, enabled bool) error
	ListAudit(ctx context.Context, limit int) ([]store.AuditEntry, error)
	ListPendingApprovals(ctx context.Context) ([]store.Approval, error)
	DecideApproval(ctx context.Context, id, status, decidedBy string) (store.Approval, error)
}

// Runtime proxies calls to the runtime service's task API.
type Runtime interface {
	Configured() bool
	Do(ctx context.Context, method, path string, body []byte) ([]byte, int, error)
}

// Server wires the store and runtime to HTTP.
type Server struct {
	store   Store
	runtime Runtime
	token   string // maintainer token; empty = dev mode (open)
}

// New builds the dashboard server.
func New(s Store, rt Runtime, token string) *Server {
	return &Server{store: s, runtime: rt, token: token}
}

const cookieName = "sl_dash"

// Handler returns all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public auth endpoints.
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /api/me", s.handleMe)

	// Gated API.
	mux.Handle("GET /api/stats", s.auth(http.HandlerFunc(s.handleStats)))
	mux.Handle("GET /api/registry", s.auth(http.HandlerFunc(s.handleRegistry)))
	mux.Handle("POST /api/registry/{name}/lifecycle", s.auth(http.HandlerFunc(s.handleLifecycle)))
	mux.Handle("POST /api/registry/{name}/enabled", s.auth(http.HandlerFunc(s.handleEnabled)))
	mux.Handle("GET /api/audit", s.auth(http.HandlerFunc(s.handleAudit)))
	mux.Handle("GET /api/approvals", s.auth(http.HandlerFunc(s.handleApprovals)))
	mux.Handle("POST /api/approvals/{id}/decide", s.auth(http.HandlerFunc(s.handleDecide)))
	mux.Handle("POST /api/tasks", s.auth(http.HandlerFunc(s.handleSubmitTask)))
	mux.Handle("GET /api/tasks/{id}", s.auth(http.HandlerFunc(s.handleGetTask)))
	mux.Handle("POST /api/tasks/{id}/approve", s.auth(http.HandlerFunc(s.handleApproveTask)))

	// Static UI (embedded).
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}

// devMode reports whether auth is disabled (no token configured).
func (s *Server) devMode() bool { return s.token == "" }

func (s *Server) authed(r *http.Request) bool {
	if s.devMode() {
		return true
	}
	c, err := r.Cookie(cookieName)
	return err == nil && c.Value == s.token
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if s.devMode() || body.Token == s.token {
		http.SetCookie(w, &http.Cookie{
			Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"authed":            s.authed(r),
		"dev":               s.devMode(),
		"runtime_available": s.runtime.Configured(),
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
