// Package server exposes the gateway contract over HTTP: list_tools and call (register is driven
// from manifests at sync time). Human callers are authenticated by ContextForge's OAuth 2.1/PKCE
// in front of this service, which passes the verified subject and committee roles; the deployed
// agent presents a service bearer. This layer is thin — all policy lives in package policy.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/policy"
	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// IdentityResolver maps an authenticated request to a caller identity. The default reads the
// verified subject and committees that the auth front (ContextForge) sets as headers.
type IdentityResolver func(*http.Request) (policy.Identity, error)

// Server wires the policy gateway to HTTP.
type Server struct {
	gw      *policy.Gateway
	token   string
	resolve IdentityResolver
}

// New builds an HTTP server over the policy gateway. token is the service bearer; resolve maps a
// request to an identity.
func New(gw *policy.Gateway, token string, resolve IdentityResolver) *Server {
	if resolve == nil {
		resolve = HeaderIdentity
	}
	return &Server{gw: gw, token: token, resolve: resolve}
}

// HeaderIdentity reads identity from headers set by the authenticating front (ContextForge):
// X-ScottyLabs-Subject and X-ScottyLabs-Committees (comma-separated).
func HeaderIdentity(r *http.Request) (policy.Identity, error) {
	subject := r.Header.Get("X-ScottyLabs-Subject")
	if subject == "" {
		return policy.Identity{}, errors.New("no subject")
	}
	var committees []string
	if raw := r.Header.Get("X-ScottyLabs-Committees"); raw != "" {
		for _, c := range strings.Split(raw, ",") {
			if c = strings.TrimSpace(c); c != "" {
				committees = append(committees, c)
			}
		}
	}
	return policy.Identity{
		Subject:    subject,
		Committees: committees,
		IsAgent:    r.Header.Get("X-ScottyLabs-Agent") == "true",
	}, nil
}

// Handler returns the gateway's HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /tools", s.auth(http.HandlerFunc(s.handleTools)))
	mux.Handle("POST /call", s.auth(http.HandlerFunc(s.handleCall)))
	return mux
}

// auth enforces the service bearer token. ContextForge handles human OAuth ahead of this.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" || r.Header.Get("Authorization") != "Bearer "+s.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	id, err := s.resolve(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no identity"})
		return
	}
	tools, err := s.gw.ListTools(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list failed"})
		return
	}
	if tools == nil {
		tools = []store.VisibleTool{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

type callBody struct {
	SessionID string          `json:"session_id"`
	Server    string          `json:"server"`
	Tool      string          `json:"tool"`
	Args      json.RawMessage `json:"args"`
}

func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	id, err := s.resolve(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no identity"})
		return
	}
	var body callBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	res, err := s.gw.Call(r.Context(), policy.CallRequest{
		Identity: id, SessionID: body.SessionID, ServerName: body.Server, ToolName: body.Tool, Args: body.Args,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"output": res.Output, "audit_id": res.AuditID})
	case errors.Is(err, policy.ErrUnknownTool):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown tool"})
	case errors.Is(err, policy.ErrUnauthorized):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unauthorized"})
	default:
		var are *policy.ApprovalRequiredError
		if errors.As(err, &are) {
			writeJSON(w, http.StatusAccepted, map[string]string{
				"status": "approval_required", "approval_id": are.ApprovalID, "tool": are.Tool,
			})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "call failed"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
