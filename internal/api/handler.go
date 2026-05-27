package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/auth"
	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/db"
	"github.com/neko233/kanban233/internal/models"
)

type Handler struct {
	store       *db.Store
	auth        *auth.Service
	agent       config.AgentConfig
	defaultUser config.DefaultUserConfig
	dev         bool
}

func NewHandler(store *db.Store, authSvc *auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		store:       store,
		auth:        authSvc,
		agent:       cfg.Agent,
		defaultUser: cfg.Auth.DefaultUser,
		dev:         cfg.Server.Dev,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.handleRegister)
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("GET /api/auth/me", h.withAuth(h.handleMe))
	mux.HandleFunc("GET /api/config", h.handleConfig)

	mux.HandleFunc("GET /api/groups/explore", h.withAuth(h.handleExploreGroups))
	mux.HandleFunc("POST /api/groups/{id}/join", h.withAuth(h.handleJoinGroup))
	mux.HandleFunc("GET /api/groups/{id}/applications", h.withAuth(h.handleListApplications))
	mux.HandleFunc("POST /api/groups/{id}/applications/{uid}/approve", h.withAuth(h.handleApproveMember))
	mux.HandleFunc("POST /api/groups/{id}/applications/{uid}/reject", h.withAuth(h.handleRejectMember))
	mux.HandleFunc("GET /api/groups/{id}/history", h.withAuth(h.handleGroupHistory))

	mux.HandleFunc("GET /api/research/overview", h.withAuth(h.handleResearchOverview))
	mux.HandleFunc("GET /api/research/stats", h.withAuth(h.handleResearchStats))
	mux.HandleFunc("GET /api/groups", h.withAuth(h.handleListGroups))
	mux.HandleFunc("GET /api/groups/{id}/kanban", h.withAuth(h.handleGetGroupKanban))
	mux.HandleFunc("GET /api/groups/{id}/stats", h.withAuth(h.handleGroupStats))
	mux.HandleFunc("GET /api/groups/{id}", h.withAuth(h.handleGetGroup))
	mux.HandleFunc("PUT /api/groups/{id}", h.withAuth(h.handleUpdateGroup))
	mux.HandleFunc("DELETE /api/groups/{id}", h.withAuth(h.handleDeleteGroup))
	mux.HandleFunc("GET /api/groups/{id}/boards", h.withAuth(h.handleListGroupBoards))
	mux.HandleFunc("POST /api/groups/{id}/boards", h.withAuth(h.handleCreateBoard))

	mux.HandleFunc("GET /api/boards", h.withAuth(h.handleListBoards))
	mux.HandleFunc("GET /api/boards/{id}", h.withAuth(h.handleGetBoard))
	mux.HandleFunc("PUT /api/boards/{id}", h.withAuth(h.handleUpdateBoard))
	mux.HandleFunc("DELETE /api/boards/{id}", h.withAuth(h.handleDeleteBoard))
	mux.HandleFunc("POST /api/groups", h.withAuth(h.handleCreateGroup))
	mux.HandleFunc("GET /api/boards/{id}/history", h.withAuth(h.handleBoardHistory))
	mux.HandleFunc("POST /api/cards/{id}/complete", h.withAuth(h.handleCompleteCard))
	mux.HandleFunc("POST /api/cards/{id}/archive", h.withAuth(h.handleArchiveCard))
	mux.HandleFunc("POST /api/boards/import", h.withAuth(h.handleImportBoard))

	mux.HandleFunc("GET /api/export/all", h.withAuth(h.handleExportAll))

	mux.HandleFunc("POST /api/boards/{id}/columns", h.withAuth(h.handleCreateColumn))
	mux.HandleFunc("PUT /api/columns/{id}", h.withAuth(h.handleUpdateColumn))
	mux.HandleFunc("DELETE /api/columns/{id}", h.withAuth(h.handleDeleteColumn))

	mux.HandleFunc("POST /api/columns/{id}/cards", h.withAuth(h.handleCreateCard))
	mux.HandleFunc("PUT /api/cards/{id}", h.withAuth(h.handleUpdateCard))
	mux.HandleFunc("DELETE /api/cards/{id}", h.withAuth(h.handleDeleteCard))
	mux.HandleFunc("POST /api/cards/{id}/move", h.withAuth(h.handleMoveCard))

	mux.HandleFunc("GET /api/audit-logs", h.withAuth(h.handleListAuditLogs))
	mux.HandleFunc("GET /api/audit-logs/export", h.withAuth(h.handleExportAuditLogs))
	h.registerAgent(mux)
}

func (h *Handler) withAuth(fn func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := h.authUserID(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		fn(w, r, userID)
	}
}

func (h *Handler) authUserID(r *http.Request) (int64, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return 0, auth.ErrUnauthorized
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0, auth.ErrUnauthorized
	}
	return h.auth.ParseToken(parts[1])
}

func (h *Handler) audit(r *http.Request, userID int64, action, resourceType string, resourceID int64, detail string) {
	_ = h.store.WriteAudit(r.Context(), userID, action, resourceType, resourceID, detail, clientIP(r))
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"registration_open": h.auth.RegistrationOpen(),
		"default_join_mode": models.JoinModeFree,
		"dev":               h.dev,
	})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	resp, err := h.auth.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrRegistrationClosed):
			writeError(w, http.StatusForbidden, "registration closed")
		case errors.Is(err, auth.ErrUsernameTaken):
			writeError(w, http.StatusConflict, "username taken")
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusBadRequest, "invalid username or password")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	h.audit(r, resp.User.ID, "register", "user", resp.User.ID, resp.User.Username)
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	resp, err := h.auth.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, resp.User.ID, "login", "user", resp.User.ID, "")
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request, userID int64) {
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleListGroups(w http.ResponseWriter, r *http.Request, userID int64) {
	groups, err := h.store.ListProjectGroups(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if groups == nil {
		groups = []models.ProjectGroup{}
	}
	writeJSON(w, http.StatusOK, groups)
}

func (h *Handler) handleCreateGroup(w http.ResponseWriter, r *http.Request, userID int64) {
	var req models.CreateProjectGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	group, err := h.store.CreateProjectGroup(r.Context(), userID, req.Name, req.Description, req.IsPublic, normalizeJoinMode(req.JoinMode))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, userID, "create", "project_group", group.ID, group.Name)
	writeJSON(w, http.StatusCreated, group)
}

func (h *Handler) handleGetGroupKanban(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	detail, err := h.store.EnsureGroupKanbanDetail(r.Context(), groupID, userID)
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

func (h *Handler) handleGetGroup(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	group, err := h.store.GetProjectGroup(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (h *Handler) handleUpdateGroup(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	var req models.UpdateProjectGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	group, err := h.store.UpdateProjectGroup(r.Context(), groupID, userID, req.Name, req.Description, req.IsPublic, normalizeJoinMode(req.JoinMode))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "update", "project_group", group.ID, group.Name)
	writeJSON(w, http.StatusOK, group)
}

func (h *Handler) handleDeleteGroup(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	if err := h.store.DeleteProjectGroup(r.Context(), groupID, userID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "delete", "project_group", groupID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListGroupBoards(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	boards, err := h.store.ListBoardsByGroup(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if boards == nil {
		boards = []models.Board{}
	}
	writeJSON(w, http.StatusOK, boards)
}

func (h *Handler) handleExploreGroups(w http.ResponseWriter, r *http.Request, userID int64) {
	groups, err := h.store.ListExploreGroups(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if groups == nil {
		groups = []models.ProjectGroup{}
	}
	for i := range groups {
		h.store.EnrichGroupPublic(r.Context(), &groups[i], userID)
	}
	writeJSON(w, http.StatusOK, groups)
}

func (h *Handler) handleJoinGroup(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	group, err := h.store.JoinGroup(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	action := "join"
	if group.JoinPending {
		action = "apply"
	}
	h.audit(r, userID, action, "project_group", group.ID, group.Name)
	writeJSON(w, http.StatusOK, group)
}

func (h *Handler) handleListApplications(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	members, err := h.store.ListPendingMembers(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *Handler) handleApproveMember(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	memberID, err := pathID(r, "uid")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.store.ApproveMember(r.Context(), groupID, userID, memberID, true); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "approve_member", "project_group", groupID, strconv.FormatInt(memberID, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRejectMember(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	memberID, err := pathID(r, "uid")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.store.ApproveMember(r.Context(), groupID, userID, memberID, false); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "reject_member", "project_group", groupID, strconv.FormatInt(memberID, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleResearchOverview(w http.ResponseWriter, r *http.Request, userID int64) {
	overview, err := h.store.GetResearchOverview(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *Handler) handleResearchStats(w http.ResponseWriter, r *http.Request, userID int64) {
	stats, err := h.store.GetResearchStats(r.Context(), userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleGroupStats(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	stats, err := h.store.GetGroupStats(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleArchiveCard(w http.ResponseWriter, r *http.Request, userID int64) {
	cardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	card, err := h.store.ArchiveCard(r.Context(), cardID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "archive", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) handleBoardHistory(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	items, err := h.store.GetBoardHistory(r.Context(), boardID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleGroupHistory(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	items, err := h.store.GetGroupHistory(r.Context(), groupID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleCompleteCard(w http.ResponseWriter, r *http.Request, userID int64) {
	cardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	card, err := h.store.CompleteCard(r.Context(), cardID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "complete", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func normalizeJoinMode(mode string) string {
	if mode == models.JoinModeApply {
		return models.JoinModeApply
	}
	return models.JoinModeFree
}

func (h *Handler) handleListBoards(w http.ResponseWriter, r *http.Request, userID int64) {
	boards, err := h.store.ListAccessibleBoards(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if boards == nil {
		boards = []models.Board{}
	}
	writeJSON(w, http.StatusOK, boards)
}

func (h *Handler) handleCreateBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	groupID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid group id")
		return
	}
	var req models.CreateBoardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	detail, err := h.store.CreateBoard(r.Context(), groupID, userID, req.Title)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "create", "board", detail.Board.ID, detail.Board.Title)
	writeJSON(w, http.StatusCreated, detail)
}

func (h *Handler) handleGetBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	detail, err := h.store.GetBoardDetail(r.Context(), boardID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleUpdateBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	var req models.UpdateBoardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	board, err := h.store.UpdateBoard(r.Context(), boardID, userID, req.Title)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "update", "board", board.ID, board.Title)
	writeJSON(w, http.StatusOK, board)
}

func (h *Handler) handleDeleteBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	if err := h.store.DeleteBoard(r.Context(), boardID, userID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "delete", "board", boardID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleExportBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	payload, err := h.store.ExportBoard(r.Context(), boardID, userID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "export", "board", boardID, payload.Board.Title)
	writeDownloadJSON(w, "kanban-board-"+payload.Board.ExportKey+".json", payload)
}

func (h *Handler) handleExportAll(w http.ResponseWriter, r *http.Request, userID int64) {
	payload, err := h.store.ExportAll(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.audit(r, userID, "export_all", "user", userID, payload.ExportedBy)
	writeDownloadJSON(w, "kanban-all-"+payload.ExportedBy+".json", payload)
}

func (h *Handler) handleImportBoard(w http.ResponseWriter, r *http.Request, userID int64) {
	var payload models.BoardExportPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := h.store.ImportBoard(r.Context(), userID, &payload)
	if err != nil {
		if errors.Is(err, db.ErrImportStale) {
			writeJSON(w, http.StatusConflict, result)
			return
		}
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "import", "board", result.BoardID, result.Action)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCreateColumn(w http.ResponseWriter, r *http.Request, userID int64) {
	boardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid board id")
		return
	}
	var req models.CreateColumnRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	col, err := h.store.CreateColumn(r.Context(), boardID, userID, req.Title)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "create", "column", col.ID, col.Title)
	writeJSON(w, http.StatusCreated, col)
}

func (h *Handler) handleUpdateColumn(w http.ResponseWriter, r *http.Request, userID int64) {
	columnID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid column id")
		return
	}
	var req models.UpdateColumnRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	col, err := h.store.UpdateColumn(r.Context(), columnID, userID, req.Title)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "update", "column", col.ID, col.Title)
	writeJSON(w, http.StatusOK, col)
}

func (h *Handler) handleDeleteColumn(w http.ResponseWriter, r *http.Request, userID int64) {
	columnID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid column id")
		return
	}
	if err := h.store.DeleteColumn(r.Context(), columnID, userID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "delete", "column", columnID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleCreateCard(w http.ResponseWriter, r *http.Request, userID int64) {
	columnID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid column id")
		return
	}
	var req models.CreateCardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	card, err := h.store.CreateCard(r.Context(), columnID, userID, req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "create", "card", card.ID, card.Title)
	writeJSON(w, http.StatusCreated, card)
}

func (h *Handler) handleUpdateCard(w http.ResponseWriter, r *http.Request, userID int64) {
	cardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	var req models.UpdateCardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	card, err := h.store.UpdateCard(r.Context(), cardID, userID, req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "update", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) handleDeleteCard(w http.ResponseWriter, r *http.Request, userID int64) {
	cardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	if err := h.store.DeleteCard(r.Context(), cardID, userID); err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "delete", "card", cardID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMoveCard(w http.ResponseWriter, r *http.Request, userID int64) {
	cardID, err := pathID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}
	var req models.MoveCardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	card, err := h.store.MoveCard(r.Context(), cardID, userID, req.ColumnID, req.Position)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	h.audit(r, userID, "move", "card", card.ID, card.Title)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) handleListAuditLogs(w http.ResponseWriter, r *http.Request, userID int64) {
	_ = userID
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	logs, err := h.store.ListAuditLogs(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *Handler) handleExportAuditLogs(w http.ResponseWriter, r *http.Request, userID int64) {
	logs, err := h.store.ExportAuditLogs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="kanban-audit-%s.csv"`, stamp))
		w.WriteHeader(http.StatusOK)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "user_id", "username", "action", "resource_type", "resource_id", "detail", "ip", "created_at"})
		for _, l := range logs {
			_ = cw.Write([]string{
				strconv.FormatInt(l.ID, 10),
				strconv.FormatInt(l.UserID, 10),
				l.Username,
				l.Action,
				l.ResourceType,
				strconv.FormatInt(l.ResourceID, 10),
				l.Detail,
				l.IP,
				l.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		cw.Flush()
	default:
		payload := map[string]any{
			"export_type": "audit_logs",
			"exported_at": time.Now().UTC().Format(time.RFC3339),
			"exported_by": userID,
			"count":       len(logs),
			"logs":        logs,
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="kanban-audit-%s.json"`, stamp))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
	}
	h.audit(r, userID, "export", "audit_logs", userID, fmt.Sprintf("%d records", len(logs)))
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func pathID(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(r.PathValue(key), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeDownloadJSON(w http.ResponseWriter, filename string, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, db.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, db.ErrConflict):
		writeError(w, http.StatusConflict, "conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
