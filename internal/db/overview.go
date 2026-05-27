package db

import (
	"context"
	"database/sql"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) GetResearchOverview(ctx context.Context, userID int64) (*models.ResearchOverview, error) {
	groups, err := s.ListProjectGroups(ctx, userID)
	if err != nil {
		return nil, err
	}

	overview := &models.ResearchOverview{
		Groups: make([]models.ResearchGroupView, 0, len(groups)),
	}

	for _, g := range groups {
		boards, err := s.ListBoardsByGroup(ctx, g.ID, userID)
		if err != nil {
			return nil, err
		}
		gv := models.ResearchGroupView{Group: g, Boards: make([]models.ResearchBoardView, 0, len(boards))}
		for _, b := range boards {
			cards, err := s.listActiveCardsByBoard(ctx, b.ID)
			if err != nil {
				return nil, err
			}
			gv.Boards = append(gv.Boards, models.ResearchBoardView{Board: b, ActiveCards: cards})
			overview.Summary.ActiveCards += len(cards)
		}
		overview.Summary.Boards += len(boards)
		overview.Summary.ProjectGroups++
		overview.Groups = append(overview.Groups, gv)
	}

	completed, err := s.countCompletedCards(ctx, userID)
	if err != nil {
		return nil, err
	}
	overview.Summary.CompletedCards = completed
	return overview, nil
}

func (s *Store) listActiveCardsByBoard(ctx context.Context, boardID int64) ([]models.Card, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT c.id, c.column_id, c.title, c.description, c.position, c.status, c.completed_at, c.created_at, c.updated_at
FROM cards c
JOIN columns col ON col.id = c.column_id
WHERE col.board_id = ? AND c.status = ?
ORDER BY c.updated_at DESC`), boardID, models.CardStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func (s *Store) countCompletedCards(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT COUNT(*)
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
LEFT JOIN project_group_members m ON m.group_id = g.id AND m.user_id = ? AND m.status = ?
WHERE c.status = ?
AND (g.owner_id = ? OR m.user_id IS NOT NULL OR g.is_public = 1)`),
		userID, models.MemberStatusActive, models.CardStatusCompleted, userID).Scan(&n)
	return n, err
}

func (s *Store) GetBoardHistory(ctx context.Context, boardID, userID int64) ([]models.CardHistoryItem, error) {
	if err := s.ensureBoardRead(ctx, boardID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT c.id, c.column_id, c.title, c.description, c.position, c.status, c.completed_at, c.created_at, c.updated_at,
       b.id, b.title, g.name
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
WHERE b.id = ? AND c.status = ?
ORDER BY c.completed_at DESC, c.updated_at DESC`), boardID, models.CardStatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CardHistoryItem
	for rows.Next() {
		var item models.CardHistoryItem
		var completed sql.NullTime
		if err := rows.Scan(
			&item.Card.ID, &item.Card.ColumnID, &item.Card.Title, &item.Card.Description, &item.Card.Position,
			&item.Card.Status, &completed, &item.Card.CreatedAt, &item.Card.UpdatedAt,
			&item.BoardID, &item.BoardTitle, &item.GroupName,
		); err != nil {
			return nil, err
		}
		if completed.Valid {
			t := completed.Time
			item.Card.CompletedAt = &t
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.CardHistoryItem{}
	}
	return items, rows.Err()
}

func (s *Store) GetGroupHistory(ctx context.Context, groupID, userID int64) ([]models.CardHistoryItem, error) {
	if _, err := s.getGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT c.id, c.column_id, c.title, c.description, c.position, c.status, c.completed_at, c.created_at, c.updated_at,
       b.id, b.title, g.name
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
WHERE g.id = ? AND c.status = ?
ORDER BY c.completed_at DESC, c.updated_at DESC`), groupID, models.CardStatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CardHistoryItem
	for rows.Next() {
		var item models.CardHistoryItem
		var completed sql.NullTime
		if err := rows.Scan(
			&item.Card.ID, &item.Card.ColumnID, &item.Card.Title, &item.Card.Description, &item.Card.Position,
			&item.Card.Status, &completed, &item.Card.CreatedAt, &item.Card.UpdatedAt,
			&item.BoardID, &item.BoardTitle, &item.GroupName,
		); err != nil {
			return nil, err
		}
		if completed.Valid {
			t := completed.Time
			item.Card.CompletedAt = &t
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.CardHistoryItem{}
	}
	return items, rows.Err()
}

func scanCards(rows *sql.Rows) ([]models.Card, error) {
	var cards []models.Card
	for rows.Next() {
		var c models.Card
		var completed sql.NullTime
		if err := rows.Scan(&c.ID, &c.ColumnID, &c.Title, &c.Description, &c.Position, &c.Status, &completed, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if completed.Valid {
			t := completed.Time
			c.CompletedAt = &t
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []models.Card{}
	}
	return cards, rows.Err()
}
