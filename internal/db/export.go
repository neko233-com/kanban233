package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) ExportBoard(ctx context.Context, boardID, userID int64) (*models.BoardExportPayload, error) {
	detail, err := s.GetBoardDetail(ctx, boardID, userID)
	if err != nil {
		return nil, err
	}
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	ga, err := s.getGroupAccess(ctx, detail.Board.ProjectGroupID, userID)
	if err != nil {
		return nil, err
	}

	exportedAt := time.Now().UTC()
	boardExport := buildExportBoard(detail, ga.GroupName, ga.IsPublic, exportedAt)

	if _, err := s.db.ExecContext(ctx, s.q(`
UPDATE boards SET last_export_at = ?, updated_at = ? WHERE id = ?`),
		exportedAt, exportedAt, boardID); err != nil {
		return nil, err
	}

	return &models.BoardExportPayload{
		ExportVersion: models.ExportFormatVersion,
		ExportType:    "single_board",
		ExportedAt:    exportedAt,
		ExportedBy:    user.Username,
		Board:         boardExport,
	}, nil
}

func (s *Store) ExportAll(ctx context.Context, userID int64) (*models.AllExportPayload, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	exportedAt := time.Now().UTC()

	groupRows, err := s.db.QueryContext(ctx, s.q(`
SELECT id, name, description, is_public, export_key FROM project_groups WHERE owner_id = ?`), userID)
	if err != nil {
		return nil, err
	}

	var exportGroups []models.ExportGroup
	var groupNames = map[int64]string{}
	var groupPublic = map[int64]bool{}
	for groupRows.Next() {
		var id int64
		var name, desc, exportKey string
		var isPublic int
		if err := groupRows.Scan(&id, &name, &desc, &isPublic, &exportKey); err != nil {
			groupRows.Close()
			return nil, err
		}
		groupNames[id] = name
		groupPublic[id] = isPublic != 0
		exportGroups = append(exportGroups, models.ExportGroup{
			ExportKey: exportKey, Name: name, Description: desc, IsPublic: isPublic != 0,
		})
	}
	if err := groupRows.Close(); err != nil {
		return nil, err
	}

	boardRows, err := s.db.QueryContext(ctx, s.q(`
SELECT id FROM boards WHERE owner_id = ?`), userID)
	if err != nil {
		return nil, err
	}
	var boardIDs []int64
	for boardRows.Next() {
		var boardID int64
		if err := boardRows.Scan(&boardID); err != nil {
			boardRows.Close()
			return nil, err
		}
		boardIDs = append(boardIDs, boardID)
	}
	if err := boardRows.Close(); err != nil {
		return nil, err
	}

	var exportBoards []models.ExportBoard
	for _, boardID := range boardIDs {
		detail, err := s.GetBoardDetail(ctx, boardID, userID)
		if err != nil {
			return nil, err
		}
		gname := groupNames[detail.Board.ProjectGroupID]
		gpub := groupPublic[detail.Board.ProjectGroupID]
		exportBoards = append(exportBoards, buildExportBoard(detail, gname, gpub, exportedAt))
		if _, err := s.db.ExecContext(ctx, s.q(`
UPDATE boards SET last_export_at = ? WHERE id = ?`), exportedAt, boardID); err != nil {
			return nil, err
		}
	}

	if exportGroups == nil {
		exportGroups = []models.ExportGroup{}
	}
	if exportBoards == nil {
		exportBoards = []models.ExportBoard{}
	}

	return &models.AllExportPayload{
		ExportVersion: models.ExportFormatVersion,
		ExportType:    "all_projects",
		ExportedAt:    exportedAt,
		ExportedBy:    user.Username,
		Groups:        exportGroups,
		Boards:        exportBoards,
	}, nil
}

func buildExportBoard(detail *models.BoardDetail, groupName string, isPublic bool, exportedAt time.Time) models.ExportBoard {
	cols := make([]models.ExportColumn, 0, len(detail.Columns))
	cardsByCol := map[int64][]models.Card{}
	for _, c := range detail.Cards {
		cardsByCol[c.ColumnID] = append(cardsByCol[c.ColumnID], c)
	}
	for _, col := range detail.Columns {
		ec := models.ExportColumn{Title: col.Title, Position: col.Position}
		for _, card := range cardsByCol[col.ID] {
			ec.Cards = append(ec.Cards, models.ExportCard{
				Title: card.Title, Description: card.Description, Position: card.Position,
			})
		}
		if ec.Cards == nil {
			ec.Cards = []models.ExportCard{}
		}
		cols = append(cols, ec)
	}
	return models.ExportBoard{
		ExportKey:     detail.Board.ExportKey,
		Title:         detail.Board.Title,
		ProjectGroup:  groupName,
		IsPublicGroup: isPublic,
		Columns:       cols,
		ExportedAt:    exportedAt,
		LastExportAt:  detail.Board.LastExportAt,
	}
}

func (s *Store) ImportBoard(ctx context.Context, userID int64, payload *models.BoardExportPayload) (*models.ImportBoardResult, error) {
	if payload.ExportVersion != models.ExportFormatVersion {
		return nil, ErrConflict
	}
	if payload.ExportType != "single_board" {
		return nil, ErrConflict
	}

	exportKey := payload.Board.ExportKey
	if exportKey == "" {
		exportKey = newExportKey()
	}

	var existingID int64
	var lastExport sql.NullTime
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT b.id, b.last_export_at FROM boards b
JOIN project_groups g ON g.id = b.project_group_id
WHERE b.export_key = ? AND (g.owner_id = ? OR g.is_public = 1)`), exportKey, userID).Scan(&existingID, &lastExport)

	if err == nil {
		if lastExport.Valid && payload.ExportedAt.Before(lastExport.Time) {
			return &models.ImportBoardResult{
				Action:  "skipped",
				BoardID: existingID,
				Message: "导入数据早于本地版本，已跳过",
			}, ErrImportStale
		}
		if err := s.ensureBoardWrite(ctx, existingID, userID); err != nil {
			return nil, err
		}
		if err := s.replaceBoardContent(ctx, existingID, payload); err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, s.q(`
UPDATE boards SET title = ?, last_export_at = ?, updated_at = ? WHERE id = ?`),
			payload.Board.Title, payload.ExportedAt, time.Now().UTC(), existingID); err != nil {
			return nil, err
		}
		detail, err := s.GetBoardDetail(ctx, existingID, userID)
		if err != nil {
			return nil, err
		}
		return &models.ImportBoardResult{Action: "updated", BoardID: existingID, Detail: detail}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	group, err := s.findOrCreateGroupForImport(ctx, userID, payload.Board)
	if err != nil {
		return nil, err
	}

	var boardID int64
	err = s.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		res, err := tx.ExecContext(ctx, s.q(`
INSERT INTO boards (project_group_id, owner_id, title, export_key, last_export_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`),
			group.ID, userID, payload.Board.Title, exportKey, payload.ExportedAt, now, now)
		if err != nil {
			return err
		}
		boardID, err = res.LastInsertId()
		if err != nil {
			return err
		}
		return s.insertBoardContentTx(ctx, tx, boardID, payload.Board.Columns)
	})
	if err != nil {
		return nil, err
	}
	detail, err := s.GetBoardDetail(ctx, boardID, userID)
	if err != nil {
		return nil, err
	}
	return &models.ImportBoardResult{Action: "created", BoardID: boardID, Detail: detail}, nil
}

func (s *Store) findOrCreateGroupForImport(ctx context.Context, userID int64, board models.ExportBoard) (*models.ProjectGroup, error) {
	if board.ProjectGroup != "" {
		row := s.db.QueryRowContext(ctx, s.q(`
SELECT id FROM project_groups WHERE owner_id = ? AND name = ? LIMIT 1`), userID, board.ProjectGroup)
		var id int64
		if err := row.Scan(&id); err == nil {
			return s.GetProjectGroup(ctx, id, userID)
		}
	}
	return s.CreateProjectGroup(ctx, userID, defaultImportGroupName(board.ProjectGroup), "", board.IsPublicGroup, models.JoinModeFree)
}

func defaultImportGroupName(name string) string {
	if name != "" {
		return name
	}
	return "导入项目"
}

func (s *Store) replaceBoardContent(ctx context.Context, boardID int64, payload *models.BoardExportPayload) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, s.q(`DELETE FROM columns WHERE board_id = ?`), boardID); err != nil {
			return err
		}
		return s.insertBoardContentTx(ctx, tx, boardID, payload.Board.Columns)
	})
}

func (s *Store) insertBoardContentTx(ctx context.Context, tx *sql.Tx, boardID int64, columns []models.ExportColumn) error {
	now := time.Now().UTC()
	for _, col := range columns {
		res, err := tx.ExecContext(ctx, s.q(`
INSERT INTO columns (board_id, title, position) VALUES (?, ?, ?)`),
			boardID, col.Title, col.Position)
		if err != nil {
			return err
		}
		colID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, card := range col.Cards {
			if _, err := tx.ExecContext(ctx, s.q(`
INSERT INTO cards (column_id, title, description, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`),
				colID, card.Title, card.Description, card.Position, now, now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) GetBoardByExportKey(ctx context.Context, exportKey string, userID int64) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT b.id FROM boards b
JOIN project_groups g ON g.id = b.project_group_id
WHERE b.export_key = ? AND (g.owner_id = ? OR g.is_public = 1)`), exportKey, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}
