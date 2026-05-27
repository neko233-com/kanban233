package db

import (
	"context"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) WriteAudit(ctx context.Context, userID int64, action, resourceType string, resourceID int64, detail, ip string) error {
	_, err := s.db.ExecContext(ctx, s.q(`
INSERT INTO audit_logs (user_id, action, resource_type, resource_id, detail, ip, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`),
		userID, action, resourceType, resourceID, detail, ip, time.Now().UTC())
	return err
}

func (s *Store) ListAuditLogs(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT a.id, a.user_id, u.username, a.action, a.resource_type, a.resource_id, a.detail, a.ip, a.created_at
FROM audit_logs a
JOIN users u ON u.id = a.user_id
ORDER BY a.created_at DESC
LIMIT ? OFFSET ?`), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.ResourceType, &l.ResourceID, &l.Detail, &l.IP, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}
	return logs, rows.Err()
}
