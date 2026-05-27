package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/agent"
)

func (h *Handler) registerAgent(mux *http.ServeMux) {
	if !h.agent.Enabled {
		return
	}
	mux.HandleFunc("GET /api/agent/collaboration", h.withAgent(h.handleAgentCollaboration))
	mux.HandleFunc("GET /api/agent/daily-report", h.withAgent(h.handleAgentDailyReport))
	mux.HandleFunc("GET /api/agent/daily-report.md", h.withAgent(h.handleAgentDailyReportMD))
}

func (h *Handler) withAgent(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.agentAuth(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		fn(w, r)
	}
}

func (h *Handler) agentAuth(r *http.Request) bool {
	if token := r.Header.Get("X-Agent-Token"); token != "" && h.agent.APIToken != "" && token == h.agent.APIToken {
		return true
	}
	if _, err := h.authUserID(r); err == nil {
		return true
	}
	user, pass, ok := r.BasicAuth()
	return ok && user == h.defaultUser.Username && pass == h.defaultUser.Password
}

func (h *Handler) parseAgentDate(r *http.Request) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("date"))
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Parse("2006-01-02", raw)
}

func (h *Handler) handleAgentCollaboration(w http.ResponseWriter, r *http.Request) {
	day, err := h.parseAgentDate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date, use YYYY-MM-DD")
		return
	}
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	data, err := h.store.GetAgentCollaboration(r.Context(), day, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) handleAgentDailyReport(w http.ResponseWriter, r *http.Request) {
	day, err := h.parseAgentDate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date, use YYYY-MM-DD")
		return
	}
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "md" || format == "markdown" || strings.Contains(r.Header.Get("Accept"), "text/markdown") {
		h.writeDailyMarkdown(w, r, day, user)
		return
	}
	data, err := h.store.GetAgentCollaboration(r.Context(), day, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	md := agent.RenderDailyMarkdown(*data)
	writeJSON(w, http.StatusOK, map[string]string{
		"date":     data.Date,
		"weekday":  data.Weekday,
		"markdown": md,
	})
}

func (h *Handler) handleAgentDailyReportMD(w http.ResponseWriter, r *http.Request) {
	day, err := h.parseAgentDate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date, use YYYY-MM-DD")
		return
	}
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	h.writeDailyMarkdown(w, r, day, user)
}

func (h *Handler) writeDailyMarkdown(w http.ResponseWriter, r *http.Request, day time.Time, user string) {
	md, err := h.store.RenderAgentDailyMarkdown(r.Context(), day, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(md))
}
