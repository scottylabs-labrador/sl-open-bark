package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

type stats struct {
	ServersTotal     int     `json:"servers_total"`
	ServersApproved  int     `json:"servers_approved"`
	ServersProposed  int     `json:"servers_proposed"`
	ToolsTotal       int     `json:"tools_total"`
	HighImpactTools  int     `json:"high_impact_tools"`
	PendingApprovals int     `json:"pending_approvals"`
	Audit24h         int     `json:"audit_24h"`
	ErrorRate        float64 `json:"error_rate"`
	AvgLatencyMS     int     `json:"avg_latency_ms"`
	RuntimeAvailable bool    `json:"runtime_available"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	var st stats
	st.ServersTotal = len(servers)
	for _, sv := range servers {
		st.ToolsTotal += len(sv.Tools)
		for _, t := range sv.Tools {
			if t.Impact == "high" {
				st.HighImpactTools++
			}
		}
		switch sv.Lifecycle {
		case store.LifecycleApproved:
			st.ServersApproved++
		case store.LifecycleProposed:
			st.ServersProposed++
		}
	}

	audit, err := s.store.ListAudit(ctx, 500)
	if err != nil {
		serverError(w, err)
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	var errs, totalLatency, counted int
	for _, e := range audit {
		if e.CreatedAt.After(cutoff) {
			st.Audit24h++
		}
		if strings.HasPrefix(e.Result, "denied") || strings.HasPrefix(e.Result, "error") {
			errs++
		}
		totalLatency += e.LatencyMS
		counted++
	}
	if counted > 0 {
		st.ErrorRate = float64(errs) / float64(counted)
		st.AvgLatencyMS = totalLatency / counted
	}

	pending, err := s.store.ListPendingApprovals(ctx)
	if err != nil {
		serverError(w, err)
		return
	}
	st.PendingApprovals = len(pending)
	st.RuntimeAvailable = s.runtime.Configured()

	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleRegistry(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if servers == nil {
		servers = []store.Server{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}

func (s *Server) handleLifecycle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Lifecycle string `json:"lifecycle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.SetServerLifecycle(r.Context(), r.PathValue("name"), body.Lifecycle); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleEnabled(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if err := s.store.SetServerEnabled(r.Context(), r.PathValue("name"), body.Enabled); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	audit, err := s.store.ListAudit(r.Context(), limit)
	if err != nil {
		serverError(w, err)
		return
	}
	if audit == nil {
		audit = []store.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit": audit})
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	pending, err := s.store.ListPendingApprovals(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	if pending == nil {
		pending = []store.Approval{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": pending})
}

func (s *Server) handleDecide(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status    string `json:"status"`
		DecidedBy string `json:"decided_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	ap, err := s.store.DecideApproval(r.Context(), r.PathValue("id"), body.Status, body.DecidedBy)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ap)
}

// --- runtime proxy (the agent console) ---

func (s *Server) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.proxyRuntime(w, r, http.MethodPost, "/tasks", body)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	s.proxyRuntime(w, r, http.MethodGet, "/tasks/"+r.PathValue("id"), nil)
}

func (s *Server) handleApproveTask(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.proxyRuntime(w, r, http.MethodPost, "/tasks/"+r.PathValue("id")+"/approve", body)
}

func (s *Server) proxyRuntime(w http.ResponseWriter, r *http.Request, method, path string, body []byte) {
	if !s.runtime.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "runtime not configured"})
		return
	}
	out, code, err := s.runtime.Do(r.Context(), method, path, body)
	if err != nil && len(out) == 0 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(out)
}

func serverError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func badRequest(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
}
