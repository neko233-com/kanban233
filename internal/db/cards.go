package db

import (
	"context"
	"database/sql"
	"strings"
	"time"
	"unicode"

	"github.com/neko233/kanban233/internal/models"
)

const cardSelectCols = `id, column_id, title, description, workers, start_date, end_date, position, status, completed_at, created_at, updated_at`

func finishedCardStatuses() []string {
	return []string{models.CardStatusCompleted, models.CardStatusArchived}
}

func parseWorkers(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '/' || unicode.IsSpace(r)
	}) {
		name := strings.TrimSpace(part)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func normalizeCardDates(start, end *string) (*string, *string) {
	norm := func(v *string) *string {
		if v == nil {
			return nil
		}
		s := strings.TrimSpace(*v)
		if s == "" {
			return nil
		}
		return &s
	}
	return norm(start), norm(end)
}

func scanCard(scanner interface {
	Scan(dest ...any) error
}) (models.Card, error) {
	var c models.Card
	var workers sql.NullString
	var startDate sql.NullString
	var endDate sql.NullString
	var completed sql.NullTime
	err := scanner.Scan(
		&c.ID, &c.ColumnID, &c.Title, &c.Description, &workers, &startDate, &endDate,
		&c.Position, &c.Status, &completed, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	if workers.Valid {
		c.Workers = workers.String
	}
	if startDate.Valid {
		s := startDate.String
		c.StartDate = &s
	}
	if endDate.Valid {
		s := endDate.String
		c.EndDate = &s
	}
	if completed.Valid {
		t := completed.Time
		c.CompletedAt = &t
	}
	if c.Status == "" {
		c.Status = models.CardStatusActive
	}
	return c, nil
}

func scanCards(rows *sql.Rows) ([]models.Card, error) {
	var cards []models.Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []models.Card{}
	}
	return cards, rows.Err()
}

func (s *Store) CreateCard(ctx context.Context, columnID, userID int64, req models.CreateCardRequest) (*models.Card, error) {
	if _, err := s.getColumnWithAccess(ctx, columnID, userID); err != nil {
		return nil, err
	}

	var position int
	if err := s.db.QueryRowContext(ctx, s.q(`
SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = ?`), columnID).Scan(&position); err != nil {
		return nil, err
	}

	startDate, endDate := normalizeCardDates(req.StartDate, req.EndDate)
	workers := strings.TrimSpace(req.Workers)
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, s.q(`
INSERT INTO cards (column_id, assignee_id, title, description, workers, start_date, end_date, position, created_at, updated_at)
VALUES (?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)`),
		columnID, req.Title, req.Description, workers, startDate, endDate, position, now, now)
	if err != nil {
		return nil, err
	}
	id, err := s.insertID(ctx, res)
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, s.q(`SELECT `+cardSelectCols+` FROM cards WHERE id = ?`), id)
	return scanCardPtr(row)
}

func (s *Store) UpdateCard(ctx context.Context, cardID, userID int64, req models.UpdateCardRequest) (*models.Card, error) {
	if _, err := s.getCardWithAccess(ctx, cardID, userID); err != nil {
		return nil, err
	}
	startDate, endDate := normalizeCardDates(req.StartDate, req.EndDate)
	workers := strings.TrimSpace(req.Workers)
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`
UPDATE cards SET title = ?, description = ?, workers = ?, start_date = ?, end_date = ?, updated_at = ? WHERE id = ?`),
		req.Title, req.Description, workers, startDate, endDate, now, cardID)
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, s.q(`SELECT `+cardSelectCols+` FROM cards WHERE id = ?`), cardID)
	return scanCardPtr(row)
}

func scanCardPtr(row *sql.Row) (*models.Card, error) {
	c, err := scanCard(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) ArchiveCard(ctx context.Context, cardID, userID int64) (*models.Card, error) {
	if _, err := s.getCardWithAccess(ctx, cardID, userID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`
UPDATE cards SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`),
		models.CardStatusArchived, now, now, cardID)
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, s.q(`SELECT `+cardSelectCols+` FROM cards WHERE id = ?`), cardID)
	return scanCardPtr(row)
}
