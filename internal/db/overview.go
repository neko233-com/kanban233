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
SELECT `+cardSelectCols+`
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
	finished := finishedCardStatuses()
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT COUNT(*)
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
LEFT JOIN project_group_members m ON m.group_id = g.id AND m.user_id = ? AND m.status = ?
WHERE c.status IN (?, ?)
AND (g.owner_id = ? OR m.user_id IS NOT NULL OR g.is_public = 1)`),
		userID, models.MemberStatusActive, finished[0], finished[1], userID).Scan(&n)
	return n, err
}

func (s *Store) GetBoardHistory(ctx context.Context, boardID, userID int64) ([]models.CardHistoryItem, error) {
	if err := s.ensureBoardRead(ctx, boardID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT c.id, c.column_id, c.title, c.description, COALESCE(c.workers,''), c.start_date, c.end_date,
       c.position, c.status, c.completed_at, c.created_at, c.updated_at,
       b.id, b.title, g.name
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
WHERE b.id = ? AND c.status IN (?, ?)
ORDER BY c.completed_at DESC, c.updated_at DESC`), boardID, models.CardStatusCompleted, models.CardStatusArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CardHistoryItem
	for rows.Next() {
		item, err := scanHistoryItem(rows)
		if err != nil {
			return nil, err
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
SELECT c.id, c.column_id, c.title, c.description, COALESCE(c.workers,''), c.start_date, c.end_date,
       c.position, c.status, c.completed_at, c.created_at, c.updated_at,
       b.id, b.title, g.name
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
JOIN project_groups g ON g.id = b.project_group_id
WHERE g.id = ? AND c.status IN (?, ?)
ORDER BY c.completed_at DESC, c.updated_at DESC`), groupID, models.CardStatusCompleted, models.CardStatusArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CardHistoryItem
	for rows.Next() {
		item, err := scanHistoryItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.CardHistoryItem{}
	}
	return items, rows.Err()
}

func scanHistoryItem(rows *sql.Rows) (models.CardHistoryItem, error) {
	var item models.CardHistoryItem
	var workers string
	var startDate sql.NullString
	var endDate sql.NullString
	var completed sql.NullTime
	err := rows.Scan(
		&item.Card.ID, &item.Card.ColumnID, &item.Card.Title, &item.Card.Description, &workers,
		&startDate, &endDate, &item.Card.Position, &item.Card.Status, &completed,
		&item.Card.CreatedAt, &item.Card.UpdatedAt,
		&item.BoardID, &item.BoardTitle, &item.GroupName,
	)
	if err != nil {
		return item, err
	}
	item.Card.Workers = workers
	if startDate.Valid {
		s := startDate.String
		item.Card.StartDate = &s
	}
	if endDate.Valid {
		s := endDate.String
		item.Card.EndDate = &s
	}
	if completed.Valid {
		t := completed.Time
		item.Card.CompletedAt = &t
	}
	return item, nil
}
