package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

const (
	demoGroupTutorialKey = "seed-demo-getting-started"
	demoBoardBasicsKey   = "seed-demo-board-basics"
	demoGroupAIKey       = "seed-demo-ai-workflow"
	demoBoardAgentKey    = "seed-demo-board-agent"
)

func (s *Store) SeedDemoContent(ctx context.Context, ownerID int64) error {
	_, err := s.projectGroupByExportKey(ctx, demoGroupTutorialKey)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}
	if err := s.seedGettingStartedDemo(ctx, ownerID); err != nil {
		return err
	}
	if err := s.seedAICollabDemo(ctx, ownerID); err != nil {
		return err
	}
	return nil
}

func (s *Store) seedGettingStartedDemo(ctx context.Context, ownerID int64) error {
	group, err := s.createProjectGroupWithExportKey(ctx, ownerID,
		"入门教学",
		"公开示例项目组：学习看板拖拽、完结归档、导入导出与操作日志。",
		true, models.JoinModeFree, demoGroupTutorialKey)
	if err != nil {
		return err
	}
	board, err := s.createBoardWithExportKey(ctx, group.ID, ownerID, group.Name, demoBoardBasicsKey)
	if err != nil {
		return err
	}
	todoCol := board.Columns[0].ID
	doingCol := board.Columns[1].ID
	cards := []struct {
		columnID int64
		title    string
		desc     string
	}{
		{todoCol, "① 点击卡片可编辑", "单击看板上的卡片，修改标题与描述后保存。"},
		{todoCol, "② 拖拽卡片换列", "将卡片从「待办」拖到「进行中」，体验 Kanban 流转。"},
		{todoCol, "③ 完结并查历史", "拖入底部归档区或点「完结」，在「完结历史」中可追溯。"},
		{doingCol, "【示例】50% 进行中的任务", "带【分类】与进度的标题，便于 Agent 日报识别。"},
	}
	for _, c := range cards {
		if _, err := s.CreateCard(ctx, c.columnID, ownerID, models.CreateCardRequest{Title: c.title, Description: c.desc}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedAICollabDemo(ctx context.Context, ownerID int64) error {
	group, err := s.createProjectGroupWithExportKey(ctx, ownerID,
		"AI 协作示例",
		"演示 Agent API、日报 Markdown 与【AI】任务分类。可在「操作日志」导出记录。",
		true, models.JoinModeApply, demoGroupAIKey)
	if err != nil {
		return err
	}
	board, err := s.createBoardWithExportKey(ctx, group.ID, ownerID, group.Name, demoBoardAgentKey)
	if err != nil {
		return err
	}
	todoCol := board.Columns[0].ID
	doingCol := board.Columns[1].ID
	cards := []struct {
		columnID int64
		title    string
		desc     string
	}{
		{doingCol, "【AI】80% 接入 MCP 工具链", "外部 Agent 可通过 X-Agent-Token 读写任务。"},
		{doingCol, "【后端】100% 操作日志导出", "支持 JSON / CSV 导出审计记录。"},
		{todoCol, "【前端】优化 Apple 风格 UI", "修改 web/styles.css 后开发模式自动热重载。"},
		{todoCol, "明天：验证日报 Markdown 格式", "GET /api/agent/daily-report.md?date=YYYY-MM-DD"},
	}
	for _, c := range cards {
		if _, err := s.CreateCard(ctx, c.columnID, ownerID, models.CreateCardRequest{Title: c.title, Description: c.desc}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) projectGroupByExportKey(ctx context.Context, exportKey string) (*models.ProjectGroup, error) {
	row := s.db.QueryRowContext(ctx, s.q(`
SELECT id, owner_id, name, description, is_public, join_mode, export_key, created_at, updated_at
FROM project_groups WHERE export_key = ?`), exportKey)
	return scanProjectGroupFull(row)
}

func (s *Store) createProjectGroupWithExportKey(ctx context.Context, ownerID int64, name, description string, isPublic bool, joinMode, exportKey string) (*models.ProjectGroup, error) {
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
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), ownerID, name, description, pub, joinMode, exportKey, now, now)
	if err != nil {
		return nil, err
	}
	id, err := s.insertID(ctx, res)
	if err != nil {
		return nil, err
	}
	return s.GetProjectGroup(ctx, id, ownerID)
}

func (s *Store) createBoardWithExportKey(ctx context.Context, groupID, userID int64, title, exportKey string) (*models.BoardDetail, error) {
	if _, err := s.ensureGroupWrite(ctx, groupID, userID); err != nil {
		return nil, err
	}
	var detail models.BoardDetail
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		res, err := tx.ExecContext(ctx, s.q(`
INSERT INTO boards (project_group_id, owner_id, title, export_key, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`), groupID, userID, title, exportKey, now, now)
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
