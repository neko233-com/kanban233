package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/agent"
	"github.com/neko233/kanban233/internal/db"
	"github.com/neko233/kanban233/internal/models"
)

func (h *Handler) registerAgent(mux *http.ServeMux) {
	if !h.agent.Enabled {
		return
	}
	mux.HandleFunc("GET /api/agent/capabilities", h.withAgent(h.handleAgentCapabilities))
	mux.HandleFunc("GET /api/agent/collaboration", h.withAgent(h.handleAgentCollaboration))
	mux.HandleFunc("GET /api/agent/daily-report", h.withAgent(h.handleAgentDailyReport))
	mux.HandleFunc("GET /api/agent/daily-report.md", h.withAgent(h.handleAgentDailyReportMD))
	mux.HandleFunc("GET /api/agent/research/overview", h.withAgent(h.handleAgentResearchOverview))
	mux.HandleFunc("GET /api/agent/groups", h.withAgent(h.handleAgentGroups))
	mux.HandleFunc("GET /api/agent/boards/{id}", h.withAgent(h.handleAgentBoard))
	mux.HandleFunc("GET /api/agent/tasks", h.withAgent(h.handleAgentTasks))
	mux.HandleFunc("POST /api/agent/cards", h.withAgent(h.handleAgentCreateCard))
	mux.HandleFunc("PUT /api/agent/cards/{id}", h.withAgent(h.handleAgentUpdateCard))
	mux.HandleFunc("POST /api/agent/cards/{id}/move", h.withAgent(h.handleAgentMoveCard))
	mux.HandleFunc("POST /api/agent/cards/{id}/complete", h.withAgent(h.handleAgentCompleteCard))
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

func (h *Handler) agentUserID(r *http.Request) (int64, error) {
	username := strings.TrimSpace(r.URL.Query().Get("as"))
	if username == "" {
		username = h.agent.DefaultActAs
	}
	if username == "" {
		username = h.defaultUser.Username
	}
	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (h *Handler) parseAgentDate(r *http.Request) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("date"))
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Parse("2006-01-02", raw)
}

func (h *Handler) handleAgentCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.AgentCapabilities{
		Enabled:        true,
		DefaultActAs:   h.agent.DefaultActAs,
		TaskCategories: h.agent.TaskCategories,
		Endpoints: []string{
			"GET /api/agent/capabilities",
			"GET /api/agent/collaboration?date=&user=",
			"GET /api/agent/daily-report?date=&user=&format=markdown",
			"GET /api/agent/daily-report.md?date=&user=",
			"GET /api/agent/research/overview?as=",
			"GET /api/agent/groups?as=",
			"GET /api/agent/boards/{id}?as=",
			"GET /api/agent/tasks?category=AI&status=active&user=&q=&limit=&offset=&as=",
			"POST /api/agent/cards?as=",
			"PUT /api/agent/cards/{id}?as=",
			"POST /api/agent/cards/{id}/move?as=",
			"POST /api/agent/cards/{id}/complete?as=",
		},
	})
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

func (h *Handler) handleAgentResearchOverview(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	data, err := h.store.GetResearchOverview(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handler) handleAgentGroups(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	groups, err := h.store.ListProjectGroups(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (h *Handler) handleAgentBoard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	boardID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	detail, err := h.store.GetBoardDetail(r.Context(), boardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleAgentTasks(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = models.CardStatusActive
	}
	items, err := h.store.SearchAgentTasks(r.Context(), db.AgentTaskFilter{
		UserID:   userID,
		Username: strings.TrimSpace(r.URL.Query().Get("user")),
		Status:   status,
		Category: strings.TrimSpace(r.URL.Query().Get("category")),
		Query:    strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":      items,
		"categories": h.agent.TaskCategories,
	})
}

func (h *Handler) handleAgentCreateCard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	var req models.AgentCreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	card, err := h.store.CreateAgentCard(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.audit(r, userID, "agent_create", "card", card.ID, card.Title)
	writeJSON(w, http.StatusCreated, card)
}

func (h *Handler) handleAgentUpdateCard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	cardID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	var req models.AgentUpdateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	title := strings.TrimSpace(req.Title)
	if req.Category != "" {
		title = agent.CategoryTitle(req.Category, title, req.Progress)
	}
	card, err := h.store.UpdateCard(r.Context(), cardID, userID, title, strings.TrimSpace(req.Description))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, userID, "agent_update", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) handleAgentMoveCard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	cardID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	var req models.AgentMoveCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := h.store.MoveCard(r.Context(), cardID, userID, req.ColumnID, req.Position)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, userID, "agent_move", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) handleAgentCompleteCard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.agentUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid as user")
		return
	}
	cardID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	card, err := h.store.CompleteCard(r.Context(), cardID, userID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, db.ErrForbidden) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, userID, "agent_complete", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}
