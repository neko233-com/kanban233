package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/neko233/kanban233/internal/agent"
	"github.com/neko233/kanban233/internal/models"
)

type AgentTaskFilter struct {
	UserID   int64
	Username string
	Status   string
	Category string
	Query    string
	Limit    int
	Offset   int
}

func (s *Store) SearchAgentTasks(ctx context.Context, filter AgentTaskFilter) ([]models.AgentTaskItem, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 100
	}
	if filter.Status == "" {
		filter.Status = models.CardStatusActive
	}

	var args []any
	query := `
SELECT c.id, c.title, c.description, c.assignee_id, b.owner_id,
       COALESCE(au.username, ou.username) AS username,
       b.title, g.name, col.title, col.position, c.updated_at, c.completed_at
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
JOIN users ou ON ou.id = b.owner_id
LEFT JOIN users au ON au.id = c.assignee_id
LEFT JOIN project_group_members m ON m.group_id = g.id AND m.user_id = ? AND m.status = ?
WHERE c.status = ?
AND (g.owner_id = ? OR m.user_id IS NOT NULL OR g.is_public = 1)`
	args = append(args, filter.UserID, models.MemberStatusActive, filter.Status, filter.UserID)

	if filter.Username != "" {
		query += ` AND LOWER(COALESCE(au.username, ou.username)) = LOWER(?)`
		args = append(args, filter.Username)
	}
	if filter.Category != "" {
		pattern := fmt.Sprintf("%%【%s】%%", filter.Category)
		query += ` AND c.title LIKE ?`
		args = append(args, pattern)
	}
	if filter.Query != "" {
		pattern := "%" + filter.Query + "%"
		query += ` AND (c.title LIKE ? OR c.description LIKE ?)`
		args = append(args, pattern, pattern)
	}
	query += ` ORDER BY c.updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.queryAgentCards(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	out := make([]models.AgentTaskItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toItem())
	}
	return out, nil
}

func (s *Store) FindBoardColumn(ctx context.Context, boardID int64, columnTitle string, userID int64) (int64, error) {
	if err := s.ensureBoardRead(ctx, boardID, userID); err != nil {
		return 0, err
	}
	title := strings.TrimSpace(columnTitle)
	if title == "" {
		title = "待办"
	}
	var id int64
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT id FROM columns WHERE board_id = ? AND title = ? ORDER BY position ASC LIMIT 1`),
		boardID, title).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = s.db.QueryRowContext(ctx, s.q(`
SELECT id FROM columns WHERE board_id = ? ORDER BY position ASC LIMIT 1`), boardID).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) CreateAgentCard(ctx context.Context, userID int64, req models.AgentCreateCardRequest) (*models.Card, error) {
	columnID := req.ColumnID
	if columnID == 0 && req.BoardID != 0 {
		id, err := s.FindBoardColumn(ctx, req.BoardID, req.ColumnTitle, userID)
		if err != nil {
			return nil, err
		}
		columnID = id
	}
	if columnID == 0 {
		return nil, fmt.Errorf("column_id or board_id required")
	}

	title := strings.TrimSpace(req.Title)
	if req.Category != "" {
		title = agent.CategoryTitle(req.Category, title, req.Progress)
	}
	return s.CreateCard(ctx, columnID, userID, title, strings.TrimSpace(req.Description))
}
