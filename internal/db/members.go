package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) isActiveMember(ctx context.Context, groupID, userID int64) (bool, string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT status FROM project_group_members WHERE group_id = ? AND user_id = ?`),
		groupID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return status == models.MemberStatusActive, status, nil
}

func (s *Store) JoinGroup(ctx context.Context, groupID, userID int64) (*models.ProjectGroup, error) {
	var ownerID int64
	var joinMode string
	err := s.db.QueryRowContext(ctx, s.q(`
SELECT owner_id, join_mode FROM project_groups WHERE id = ?`), groupID).Scan(&ownerID, &joinMode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerID == userID {
		return s.GetProjectGroup(ctx, groupID, userID)
	}

	active, status, err := s.isActiveMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if active {
		return s.GetProjectGroup(ctx, groupID, userID)
	}
	if status == models.MemberStatusPending {
		return nil, ErrConflict
	}

	now := time.Now().UTC()
	memberStatus := models.MemberStatusActive
	if joinMode == models.JoinModeApply {
		memberStatus = models.MemberStatusPending
	}
	_, err = s.db.ExecContext(ctx, s.q(`
INSERT INTO project_group_members (group_id, user_id, status, joined_at) VALUES (?, ?, ?, ?)`),
		groupID, userID, memberStatus, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	if memberStatus == models.MemberStatusPending {
		g, err := s.getProjectGroupRow(ctx, groupID)
		if err != nil {
			return nil, err
		}
		g.JoinPending = true
		return g, nil
	}
	return s.GetProjectGroup(ctx, groupID, userID)
}

func (s *Store) ApproveMember(ctx context.Context, groupID, ownerID, memberUserID int64, approve bool) error {
	if err := s.ensureGroupOwner(ctx, groupID, ownerID); err != nil {
		return err
	}
	if approve {
		res, err := s.db.ExecContext(ctx, s.q(`
UPDATE project_group_members SET status = ? WHERE group_id = ? AND user_id = ? AND status = ?`),
			models.MemberStatusActive, groupID, memberUserID, models.MemberStatusPending)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	res, err := s.db.ExecContext(ctx, s.q(`
DELETE FROM project_group_members WHERE group_id = ? AND user_id = ? AND status = ?`),
		groupID, memberUserID, models.MemberStatusPending)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListPendingMembers(ctx context.Context, groupID, ownerID int64) ([]models.GroupMember, error) {
	if err := s.ensureGroupOwner(ctx, groupID, ownerID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT m.user_id, u.username, m.status, m.joined_at
FROM project_group_members m
JOIN users u ON u.id = m.user_id
WHERE m.group_id = ? AND m.status = ?
ORDER BY m.joined_at ASC`), groupID, models.MemberStatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.GroupMember
	for rows.Next() {
		var m models.GroupMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.Status, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []models.GroupMember{}
	}
	return members, rows.Err()
}

func (s *Store) ListExploreGroups(ctx context.Context, userID int64) ([]models.ProjectGroup, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT g.id, g.owner_id, g.name, g.description, g.is_public, g.join_mode, g.export_key, g.created_at, g.updated_at
FROM project_groups g
WHERE g.owner_id != ?
AND NOT EXISTS (
  SELECT 1 FROM project_group_members m
  WHERE m.group_id = g.id AND m.user_id = ? AND m.status = ?
)
ORDER BY g.updated_at DESC`), userID, userID, models.MemberStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectGroupRows(rows, userID)
}

func (s *Store) EnrichGroupPublic(ctx context.Context, g *models.ProjectGroup, userID int64) {
	s.enrichGroup(ctx, g, userID)
}

func (s *Store) CompleteCard(ctx context.Context, cardID, userID int64) (*models.Card, error) {
	card, err := s.getCardWithAccess(ctx, cardID, userID)
	if err != nil {
		return nil, err
	}
	var doneColID int64
	err = s.db.QueryRowContext(ctx, s.q(`
SELECT id FROM columns WHERE board_id = ? AND is_done = 1 ORDER BY position ASC LIMIT 1`), card.BoardID).Scan(&doneColID)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC()
		_, err = s.db.ExecContext(ctx, s.q(`
UPDATE cards SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`),
			models.CardStatusCompleted, now, now, cardID)
		if err != nil {
			return nil, err
		}
		card.Status = models.CardStatusCompleted
		card.CompletedAt = &now
		card.UpdatedAt = now
		return &card.Card, nil
	}
	if err != nil {
		return nil, err
	}
	return s.MoveCard(ctx, cardID, userID, doneColID, 0)
}

func (s *Store) enrichGroup(ctx context.Context, g *models.ProjectGroup, userID int64) {
	g.IsOwner = g.OwnerID == userID
	if g.IsOwner {
		g.IsMember = true
		return
	}
	active, status, _ := s.isActiveMember(ctx, g.ID, userID)
	g.IsMember = active
	g.JoinPending = status == models.MemberStatusPending
}
