package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

func newExportKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type groupAccess struct {
	GroupID   int64
	GroupName string
	IsPublic  bool
	JoinMode  string
	OwnerID   int64
	CanRead   bool
	CanWrite  bool
}

func (s *Store) getGroupAccess(ctx context.Context, groupID, userID int64) (groupAccess, error) {
	var ga groupAccess
	var isPublic int
	var joinMode string
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT id, owner_id, name, is_public, join_mode FROM project_groups WHERE id = ?`), groupID)
	err := row.Scan(&ga.GroupID, &ga.OwnerID, &ga.GroupName, &isPublic, &joinMode)
	if errors.Is(err, sql.ErrNoRows) {
		return ga, ErrNotFound
	}
	if err != nil {
		return ga, err
	}
	ga.IsPublic = isPublic != 0
	ga.JoinMode = joinMode
	if ga.OwnerID == userID {
		ga.CanRead = true
		ga.CanWrite = true
		return ga, nil
	}
	active, _, err := s.isActiveMember(ctx, groupID, userID)
	if err != nil {
		return ga, err
	}
	if active {
		ga.CanRead = true
		ga.CanWrite = true
		return ga, nil
	}
	if ga.IsPublic {
		ga.CanRead = true
		return ga, nil
	}
	return ga, ErrForbidden
}

func (s *Store) ensureGroupWrite(ctx context.Context, groupID, userID int64) (groupAccess, error) {
	ga, err := s.getGroupAccess(ctx, groupID, userID)
	if err != nil {
		return ga, err
	}
	if !ga.CanWrite {
		return ga, ErrForbidden
	}
	return ga, nil
}

func (s *Store) ensureGroupOwner(ctx context.Context, groupID, userID int64) error {
	ga, err := s.getGroupAccess(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if ga.OwnerID != userID {
		return ErrForbidden
	}
	return nil
}

type boardAccess struct {
	BoardID        int64
	ProjectGroupID int64
	CanRead        bool
	CanWrite       bool
}

func (s *Store) getBoardAccess(ctx context.Context, boardID, userID int64) (boardAccess, error) {
	var ba boardAccess
	var groupID int64
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT boards.id, boards.project_group_id FROM boards WHERE boards.id = ?`), boardID)
	err := row.Scan(&ba.BoardID, &groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return ba, ErrNotFound
	}
	if err != nil {
		return ba, err
	}
	ba.ProjectGroupID = groupID
	ga, err := s.getGroupAccess(ctx, groupID, userID)
	if err != nil {
		return ba, err
	}
	ba.CanRead = ga.CanRead
	ba.CanWrite = ga.CanWrite
	if !ba.CanRead {
		return ba, ErrForbidden
	}
	return ba, nil
}

func (s *Store) ensureBoardWrite(ctx context.Context, boardID, userID int64) error {
	ba, err := s.getBoardAccess(ctx, boardID, userID)
	if err != nil {
		return err
	}
	if !ba.CanWrite {
		return ErrForbidden
	}
	return nil
}

func (s *Store) ensureBoardRead(ctx context.Context, boardID, userID int64) error {
	_, err := s.getBoardAccess(ctx, boardID, userID)
	return err
}

func (s *Store) EnsureDefaultGroup(ctx context.Context, ownerID int64) (*models.ProjectGroup, error) {
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT id, owner_id, name, description, is_public, join_mode, export_key, created_at, updated_at
FROM project_groups WHERE owner_id = ? ORDER BY id ASC LIMIT 1`), ownerID)
	if g, err := scanProjectGroupFull(row); err == nil {
		return g, nil
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return s.CreateProjectGroup(ctx, ownerID, "我的项目", "", false, models.JoinModeFree)
}

func (s *Store) getProjectGroupRow(ctx context.Context, groupID int64) (*models.ProjectGroup, error) {
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT id, owner_id, name, description, is_public, join_mode, export_key, created_at, updated_at
FROM project_groups WHERE id = ?`), groupID)
	return scanProjectGroupFull(row)
}

func scanProjectGroupFull(row *sql.Row) (*models.ProjectGroup, error) {
	var g models.ProjectGroup
	var isPublic int
	var exportKey string
	err := row.Scan(&g.ID, &g.OwnerID, &g.Name, &g.Description, &isPublic, &g.JoinMode, &exportKey, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.IsPublic = isPublic != 0
	if g.JoinMode == "" {
		g.JoinMode = models.JoinModeFree
	}
	return &g, nil
}

func scanProjectGroupRows(rows *sql.Rows, userID int64) ([]models.ProjectGroup, error) {
	var groups []models.ProjectGroup
	for rows.Next() {
		var g models.ProjectGroup
		var isPublic int
		var exportKey string
		if err := rows.Scan(&g.ID, &g.OwnerID, &g.Name, &g.Description, &isPublic, &g.JoinMode, &exportKey, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.IsPublic = isPublic != 0
		if g.JoinMode == "" {
			g.JoinMode = models.JoinModeFree
		}
		g.IsOwner = g.OwnerID == userID
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) CreateProjectGroup(ctx context.Context, ownerID int64, name, description string, isPublic bool, joinMode string) (*models.ProjectGroup, error) {
	if joinMode == "" {
		joinMode = models.JoinModeFree
	}
	now := time.Now().UTC()
	pub := 0
	if isPublic {
		pub = 1
	}
	res, err := s.db.ExecContext(ctx, s.q(`
INSERT INTO project_groups (owner_id, name, description, is_public, join_mode, export_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), ownerID, name, description, pub, joinMode, newExportKey(), now, now)
	if err != nil {
		return nil, err
	}
	id, err := s.insertID(ctx, res)
	if err != nil {
		return nil, err
	}
	group, err := s.GetProjectGroup(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	if _, err := s.CreateBoard(ctx, group.ID, ownerID, name); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *Store) GetProjectGroup(ctx context.Context, groupID, userID int64) (*models.ProjectGroup, error) {
	if _, err := s.getGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT id, owner_id, name, description, is_public, join_mode, export_key, created_at, updated_at
FROM project_groups WHERE id = ?`), groupID)
	g, err := scanProjectGroupFull(row)
	if err != nil {
		return nil, err
	}
	s.enrichGroup(ctx, g, userID)
	return g, nil
}

func (s *Store) ListProjectGroups(ctx context.Context, userID int64) ([]models.ProjectGroup, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT DISTINCT g.id, g.owner_id, g.name, g.description, g.is_public, g.join_mode, g.export_key, g.created_at, g.updated_at
FROM project_groups g
LEFT JOIN project_group_members m ON m.group_id = g.id AND m.user_id = ? AND m.status = ?
WHERE g.owner_id = ? OR m.user_id IS NOT NULL OR g.is_public = 1
ORDER BY g.updated_at DESC`), userID, models.MemberStatusActive, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups, err := scanProjectGroupRows(rows, userID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		s.enrichGroup(ctx, &groups[i], userID)
	}
	return groups, nil
}

func (s *Store) UpdateProjectGroup(ctx context.Context, groupID, userID int64, name, description string, isPublic bool, joinMode string) (*models.ProjectGroup, error) {
	if err := s.ensureGroupOwner(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if joinMode == "" {
		joinMode = models.JoinModeFree
	}
	pub := 0
	if isPublic {
		pub = 1
	}
	_, err := s.db.ExecContext(ctx, s.q(`
UPDATE project_groups SET name = ?, description = ?, is_public = ?, join_mode = ?, updated_at = ? WHERE id = ?`),
		name, description, pub, joinMode, time.Now().UTC(), groupID)
	if err != nil {
		return nil, err
	}
	return s.GetProjectGroup(ctx, groupID, userID)
}

func (s *Store) DeleteProjectGroup(ctx context.Context, groupID, userID int64) error {
	if err := s.ensureGroupOwner(ctx, groupID, userID); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM project_groups WHERE id = ?`), groupID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListBoardsByGroup(ctx context.Context, groupID, userID int64) ([]models.Board, error) {
	if _, err := s.getGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT id, project_group_id, owner_id, title, export_key, last_export_at, created_at, updated_at
FROM boards WHERE project_group_id = ? ORDER BY updated_at DESC`), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBoards(rows)
}

func (s *Store) ListAccessibleBoards(ctx context.Context, userID int64) ([]models.Board, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT DISTINCT b.id, b.project_group_id, b.owner_id, b.title, b.export_key, b.last_export_at, b.created_at, b.updated_at
FROM boards b
JOIN project_groups g ON g.id = b.project_group_id
LEFT JOIN project_group_members m ON m.group_id = g.id AND m.user_id = ? AND m.status = ?
WHERE g.owner_id = ? OR m.user_id IS NOT NULL OR g.is_public = 1
ORDER BY b.updated_at DESC`), userID, models.MemberStatusActive, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBoards(rows)
}

func scanBoards(rows *sql.Rows) ([]models.Board, error) {
	var boards []models.Board
	for rows.Next() {
		var b models.Board
		var lastExport sql.NullTime
		if err := rows.Scan(&b.ID, &b.ProjectGroupID, &b.OwnerID, &b.Title, &b.ExportKey, &lastExport, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if lastExport.Valid {
			t := lastExport.Time
			b.LastExportAt = &t
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func scanBoardRow(row *sql.Row) (models.Board, error) {
	var b models.Board
	var lastExport sql.NullTime
	err := row.Scan(&b.ID, &b.ProjectGroupID, &b.OwnerID, &b.Title, &b.ExportKey, &lastExport, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	if lastExport.Valid {
		t := lastExport.Time
		b.LastExportAt = &t
	}
	return b, nil
}
