package db

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/agent"
	"github.com/neko233/kanban233/internal/models"
)

type cardRow struct {
	ID          int64
	Title       string
	Description string
	AssigneeID  sql.NullInt64
	OwnerID     int64
	Username    string
	BoardTitle  string
	GroupName   string
	ColumnTitle string
	UpdatedAt   time.Time
	CompletedAt sql.NullTime
}

func (s *Store) GetAgentCollaboration(ctx context.Context, day time.Time, filterUser string) (*models.AgentCollaborationDay, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	completed, err := s.queryAgentCards(ctx, `
SELECT c.id, c.title, c.description, c.assignee_id, b.owner_id,
       COALESCE(au.username, ou.username) AS username,
       b.title, g.name, col.title, c.updated_at, c.completed_at
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
JOIN users ou ON ou.id = b.owner_id
LEFT JOIN users au ON au.id = c.assignee_id
WHERE c.status = ? AND c.completed_at >= ? AND c.completed_at < ?
ORDER BY c.completed_at ASC`, models.CardStatusCompleted, start, end)
	if err != nil {
		return nil, err
	}

	inProgress, err := s.queryAgentCards(ctx, `
SELECT c.id, c.title, c.description, c.assignee_id, b.owner_id,
       COALESCE(au.username, ou.username) AS username,
       b.title, g.name, col.title, c.updated_at, c.completed_at
FROM cards c
JOIN columns col ON col.id = c.column_id AND col.is_done = 0
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
JOIN users ou ON ou.id = b.owner_id
LEFT JOIN users au ON au.id = c.assignee_id
WHERE c.status = ? AND c.updated_at >= ? AND c.updated_at < ?
AND (c.completed_at IS NULL OR c.completed_at >= ?)
AND (col.title LIKE '%进行%' OR col.title LIKE '%Doing%' OR col.title LIKE '%doing%')
ORDER BY c.updated_at ASC`, models.CardStatusActive, start, end, end)
	if err != nil {
		return nil, err
	}

	tomorrow, err := s.queryAgentCards(ctx, `
SELECT c.id, c.title, c.description, c.assignee_id, b.owner_id,
       COALESCE(au.username, ou.username) AS username,
       b.title, g.name, col.title, c.updated_at, c.completed_at
FROM cards c
JOIN columns col ON col.id = c.column_id AND col.is_done = 0
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
JOIN users ou ON ou.id = b.owner_id
LEFT JOIN users au ON au.id = c.assignee_id
WHERE c.status = ?
AND (col.title LIKE '%待办%' OR col.title LIKE '%明天%' OR col.title LIKE '%TODO%' OR col.title LIKE '%todo%' OR col.position = 0)
ORDER BY g.name, b.title, c.position ASC`, models.CardStatusActive)
	if err != nil {
		return nil, err
	}

	users := map[string]*models.AgentUserDay{}
	ensure := func(username string) *models.AgentUserDay {
		if username == "" {
			username = "unknown"
		}
		if filterUser != "" && !strings.EqualFold(username, filterUser) {
			return nil
		}
		if users[username] == nil {
			users[username] = &models.AgentUserDay{Username: username}
		}
		return users[username]
	}

	for _, row := range completed {
		if u := ensure(row.Username); u != nil {
			u.CompletedToday = append(u.CompletedToday, row.toItem())
		}
	}
	for _, row := range inProgress {
		if u := ensure(row.Username); u != nil {
			u.InProgressToday = append(u.InProgressToday, row.toItem())
		}
	}
	for _, row := range tomorrow {
		if u := ensure(row.Username); u != nil {
			u.Tomorrow = append(u.Tomorrow, row.toItem())
		}
	}

	result := &models.AgentCollaborationDay{
		Date:    start.Format("2006-01-02"),
		Weekday: agent.WeekdayCN(start),
		Users:   make([]models.AgentUserDay, 0, len(users)),
	}
	for _, u := range users {
		result.Users = append(result.Users, *u)
	}
	return result, nil
}

func (s *Store) queryAgentCards(ctx context.Context, query string, args ...any) ([]cardRow, error) {
	rows, err := s.db.QueryContext(ctx, s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []cardRow
	for rows.Next() {
		var r cardRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Description, &r.AssigneeID, &r.OwnerID,
			&r.Username, &r.BoardTitle, &r.GroupName, &r.ColumnTitle, &r.UpdatedAt, &r.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (r cardRow) toItem() models.AgentTaskItem {
	cat, prog, summary := agent.ParseTaskLine(r.Title, r.Description)
	return models.AgentTaskItem{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		Category:    cat,
		Progress:    prog,
		Summary:     summary,
		BoardTitle:  r.BoardTitle,
		GroupName:   r.GroupName,
		ColumnTitle: r.ColumnTitle,
	}
}

func (s *Store) RenderAgentDailyMarkdown(ctx context.Context, day time.Time, filterUser string) (string, error) {
	data, err := s.GetAgentCollaboration(ctx, day, filterUser)
	if err != nil {
		return "", err
	}
	return agent.RenderDailyMarkdown(*data), nil
}
