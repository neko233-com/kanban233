package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (*models.User, error) {
	res, err := s.db.ExecContext(ctx,
		s.q(`INSERT INTO users (username, password_hash) VALUES (?, ?)`),
		username, passwordHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	id, err := s.insertID(ctx, res)
	if err != nil {
		return nil, err
	}
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.EnsureDefaultGroup(ctx, id); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		s.q(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`),
		username,
	)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		s.q(`SELECT id, username, password_hash, created_at FROM users WHERE id = ?`),
		id,
	)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateBoard(ctx context.Context, groupID, userID int64, title string) (*models.BoardDetail, error) {
	ga, err := s.ensureGroupWrite(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	_ = ga

	var detail models.BoardDetail
	err = s.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		res, err := tx.ExecContext(ctx, s.q(`
INSERT INTO boards (project_group_id, owner_id, title, export_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`), groupID, userID, title, newExportKey(), now, now)
		if err != nil {
			return err
		}
		boardID, err := res.LastInsertId()
		if err != nil {
			return err
		}

		defaultColumns := []struct {
			title  string
			isDone bool
		}{
			{"待办", false},
			{"进行中", false},
			{"完成", true},
		}
		for i, col := range defaultColumns {
			done := 0
			if col.isDone {
				done = 1
			}
			if _, err := tx.ExecContext(ctx, s.q(`
INSERT INTO columns (board_id, title, position, is_done) VALUES (?, ?, ?, ?)`),
				boardID, col.title, i, done); err != nil {
				return err
			}
		}

		detail, err = s.getBoardDetailTx(ctx, tx, boardID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *Store) GetBoardDetail(ctx context.Context, boardID, userID int64) (*models.BoardDetail, error) {
	if err := s.ensureBoardRead(ctx, boardID, userID); err != nil {
		return nil, err
	}
	var detail models.BoardDetail
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		detail, err = s.getBoardDetailTx(ctx, tx, boardID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *Store) getBoardDetailTx(ctx context.Context, tx *sql.Tx, boardID, userID int64) (models.BoardDetail, error) {
	var detail models.BoardDetail
	row := tx.QueryRowContext(ctx, s.q(`
SELECT id, project_group_id, owner_id, title, export_key, last_export_at, created_at, updated_at
FROM boards WHERE id = ?`), boardID)
	var lastExport sql.NullTime
	err := row.Scan(&detail.Board.ID, &detail.Board.ProjectGroupID, &detail.Board.OwnerID,
		&detail.Board.Title, &detail.Board.ExportKey, &lastExport, &detail.Board.CreatedAt, &detail.Board.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return detail, ErrNotFound
	}
	if err != nil {
		return detail, err
	}
	if lastExport.Valid {
		t := lastExport.Time
		detail.Board.LastExportAt = &t
	}

	colRows, err := tx.QueryContext(ctx, s.q(`
SELECT id, board_id, title, position, is_done FROM columns
WHERE board_id = ? AND is_done = 0 ORDER BY position ASC`), boardID)
	if err != nil {
		return detail, err
	}
	defer colRows.Close()

	for colRows.Next() {
		var c models.Column
		var isDone int
		if err := colRows.Scan(&c.ID, &c.BoardID, &c.Title, &c.Position, &isDone); err != nil {
			return detail, err
		}
		c.IsDone = isDone != 0
		detail.Columns = append(detail.Columns, c)
	}
	if err := colRows.Err(); err != nil {
		return detail, err
	}

	if len(detail.Columns) == 0 {
		detail.Cards = []models.Card{}
		return detail, nil
	}

	columnIDs := make([]any, len(detail.Columns))
	placeholders := make([]string, len(detail.Columns))
	for i, c := range detail.Columns {
		columnIDs[i] = c.ID
		placeholders[i] = "?"
	}
	query := fmt.Sprintf(s.q(`
SELECT `+cardSelectCols+` FROM cards
WHERE status = ? AND column_id IN (%s) ORDER BY position ASC`), strings.Join(placeholders, ","))

	args := append([]any{models.CardStatusActive}, columnIDs...)
	cardRows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return detail, err
	}
	defer cardRows.Close()

	detail.Cards, err = scanCards(cardRows)
	if err != nil {
		return detail, err
	}
	return detail, nil
}

func (s *Store) UpdateBoard(ctx context.Context, boardID, userID int64, title string) (*models.Board, error) {
	if err := s.ensureBoardWrite(ctx, boardID, userID); err != nil {
		return nil, err
	}
	_, err := s.db.ExecContext(ctx, s.q(`
UPDATE boards SET title = ?, updated_at = ? WHERE id = ?`),
		title, time.Now().UTC(), boardID)
	if err != nil {
		return nil, err
	}
	detail, err := s.GetBoardDetail(ctx, boardID, userID)
	if err != nil {
		return nil, err
	}
	return &detail.Board, nil
}

func (s *Store) DeleteBoard(ctx context.Context, boardID, userID int64) error {
	if err := s.ensureBoardWrite(ctx, boardID, userID); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM boards WHERE id = ?`), boardID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateColumn(ctx context.Context, boardID, userID int64, title string) (*models.Column, error) {
	if err := s.ensureBoardWrite(ctx, boardID, userID); err != nil {
		return nil, err
	}

	var position int
	if err := s.db.QueryRowContext(ctx, s.q(`
SELECT COALESCE(MAX(position), -1) + 1 FROM columns WHERE board_id = ?`), boardID).Scan(&position); err != nil {
		return nil, err
	}

	res, err := s.db.ExecContext(ctx, s.q(`
INSERT INTO columns (board_id, title, position) VALUES (?, ?, ?)`),
		boardID, title, position)
	if err != nil {
		return nil, err
	}
	id, err := s.insertID(ctx, res)
	if err != nil {
		return nil, err
	}
	return &models.Column{ID: id, BoardID: boardID, Title: title, Position: position}, nil
}

func (s *Store) UpdateColumn(ctx context.Context, columnID, userID int64, title string) (*models.Column, error) {
	col, err := s.getColumnWithAccess(ctx, columnID, userID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, s.q(`UPDATE columns SET title = ? WHERE id = ?`), title, columnID)
	if err != nil {
		return nil, err
	}
	col.Title = title
	return col, nil
}

func (s *Store) DeleteColumn(ctx context.Context, columnID, userID int64) error {
	if _, err := s.getColumnWithAccess(ctx, columnID, userID); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM columns WHERE id = ?`), columnID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteCard(ctx context.Context, cardID, userID int64) error {
	if _, err := s.getCardWithAccess(ctx, cardID, userID); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM cards WHERE id = ?`), cardID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MoveCard(ctx context.Context, cardID, userID int64, targetColumnID int64, position int) (*models.Card, error) {
	var updated models.Card
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		card, err := s.getCardWithAccessTx(ctx, tx, cardID, userID)
		if err != nil {
			return err
		}
		targetCol, err := s.getColumnWithAccessTx(ctx, tx, targetColumnID, userID)
		if err != nil {
			return err
		}
		if targetCol.BoardID != card.BoardID {
			return ErrNotFound
		}

		if card.ColumnID == targetColumnID {
			if err := s.reorderCardsTx(ctx, tx, targetColumnID, cardID, position); err != nil {
				return err
			}
		} else {
			if err := s.removeCardFromColumnTx(ctx, tx, card.ColumnID, cardID); err != nil {
				return err
			}
			if err := s.insertCardAtPositionTx(ctx, tx, targetColumnID, cardID, position); err != nil {
				return err
			}
		}

		now := time.Now().UTC()
		status := models.CardStatusActive
		var completedAt any = nil
		if targetCol.IsDone {
			status = models.CardStatusCompleted
			completedAt = now
		}
		if _, err := tx.ExecContext(ctx, s.q(`
UPDATE cards SET column_id = ?, position = ?, status = ?, completed_at = ?, updated_at = ? WHERE id = ?`),
			targetColumnID, position, status, completedAt, now, cardID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, s.q(`UPDATE boards SET updated_at = ? WHERE id = ?`),
			now, targetCol.BoardID); err != nil {
			return err
		}

		row := tx.QueryRowContext(ctx, s.q(`SELECT `+cardSelectCols+` FROM cards WHERE id = ?`), cardID)
		c, err := scanCard(row)
		if err != nil {
			return err
		}
		updated = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

type ownedCard struct {
	models.Card
	BoardID int64
}

func (s *Store) getCardWithAccess(ctx context.Context, cardID, userID int64) (*ownedCard, error) {
	return s.getCardWithAccessTx(ctx, s.db, cardID, userID)
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) getCardWithAccessTx(ctx context.Context, q querier, cardID, userID int64) (*ownedCard, error) {
	var c ownedCard
	row := q.QueryRowContext(ctx, s.q(`
SELECT cards.id, cards.column_id, cards.title, cards.description, cards.position, cards.created_at, cards.updated_at, columns.board_id, boards.project_group_id
FROM cards
JOIN columns ON columns.id = cards.column_id
JOIN boards ON boards.id = columns.board_id
WHERE cards.id = ?`), cardID)
	var groupID int64
	err := row.Scan(&c.ID, &c.ColumnID, &c.Title, &c.Description, &c.Position, &c.CreatedAt, &c.UpdatedAt, &c.BoardID, &groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	ga, err := s.getGroupAccess(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !ga.CanWrite {
		return nil, ErrForbidden
	}
	return &c, nil
}

func (s *Store) getColumnWithAccess(ctx context.Context, columnID, userID int64) (*models.Column, error) {
	return s.getColumnWithAccessTx(ctx, s.db, columnID, userID)
}

func (s *Store) getColumnWithAccessTx(ctx context.Context, q querier, columnID, userID int64) (*models.Column, error) {
	var c models.Column
	var groupID int64
	var isDone int
	row := q.QueryRowContext(ctx, s.q(`
SELECT columns.id, columns.board_id, columns.title, columns.position, columns.is_done, boards.project_group_id
FROM columns
JOIN boards ON boards.id = columns.board_id
WHERE columns.id = ?`), columnID)
	err := row.Scan(&c.ID, &c.BoardID, &c.Title, &c.Position, &isDone, &groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.IsDone = isDone != 0
	ga, err := s.getGroupAccess(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !ga.CanWrite {
		return nil, ErrForbidden
	}
	return &c, nil
}

func (s *Store) reorderCardsTx(ctx context.Context, tx *sql.Tx, columnID, cardID int64, newPos int) error {
	var cards []int64
	rows, err := tx.QueryContext(ctx, s.q(`SELECT id FROM cards WHERE column_id = ? ORDER BY position ASC`), columnID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		cards = append(cards, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	filtered := make([]int64, 0, len(cards))
	for _, id := range cards {
		if id != cardID {
			filtered = append(filtered, id)
		}
	}
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(filtered) {
		newPos = len(filtered)
	}
	filtered = append(filtered[:newPos], append([]int64{cardID}, filtered[newPos:]...)...)

	for i, id := range filtered {
		if _, err := tx.ExecContext(ctx, s.q(`UPDATE cards SET position = ? WHERE id = ?`), i, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) removeCardFromColumnTx(ctx context.Context, tx *sql.Tx, columnID, cardID int64) error {
	rows, err := tx.QueryContext(ctx, s.q(`
SELECT id FROM cards WHERE column_id = ? AND id != ? ORDER BY position ASC`), columnID, cardID)
	if err != nil {
		return err
	}
	defer rows.Close()
	pos := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, s.q(`UPDATE cards SET position = ? WHERE id = ?`), pos, id); err != nil {
			return err
		}
		pos++
	}
	return rows.Err()
}

func (s *Store) insertCardAtPositionTx(ctx context.Context, tx *sql.Tx, columnID, cardID int64, position int) error {
	rows, err := tx.QueryContext(ctx, s.q(`
SELECT id FROM cards WHERE column_id = ? AND id != ? ORDER BY position ASC`), columnID, cardID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if position < 0 {
		position = 0
	}
	if position > len(ids) {
		position = len(ids)
	}
	ids = append(ids[:position], append([]int64{cardID}, ids[position:]...)...)
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, s.q(`UPDATE cards SET position = ? WHERE id = ?`), i, id); err != nil {
			return err
		}
	}
	return nil
}
