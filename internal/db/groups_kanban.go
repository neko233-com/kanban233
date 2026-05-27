package db

import (
	"context"

	"github.com/neko233/kanban233/internal/models"
)

func (s *Store) EnsureGroupKanbanDetail(ctx context.Context, groupID, userID int64) (*models.BoardDetail, error) {
	if _, err := s.getGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	boards, err := s.ListBoardsByGroup(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if len(boards) == 0 {
		g, err := s.getProjectGroupRow(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if _, err := s.ensureGroupWrite(ctx, groupID, userID); err != nil {
			return nil, err
		}
		return s.CreateBoard(ctx, groupID, userID, g.Name)
	}
	return s.GetBoardDetail(ctx, boards[0].ID, userID)
}

func (s *Store) EnsureAllGroupsHaveKanban(ctx context.Context, ownerID int64) error {
	groups, err := s.ListProjectGroups(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if _, err := s.EnsureGroupKanbanDetail(ctx, g.ID, ownerID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedMyProjectWelcome(ctx context.Context, ownerID int64) error {
	groups, err := s.ListProjectGroups(ctx, ownerID)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.Name != "我的项目" {
			continue
		}
		detail, err := s.EnsureGroupKanbanDetail(ctx, g.ID, ownerID)
		if err != nil {
			return err
		}
		if len(detail.Cards) > 0 {
			return nil
		}
		todoCol := detail.Columns[0].ID
		starter := []struct{ title, desc, workers string }{
			{"欢迎使用 Kanban233", "左侧选择项目组即可直接进入看板，一个项目对应一个看板。", "LiLei"},
			{"创建第一张任务卡片", "点击下方「+ 添加卡片」，或拖拽任务到不同列。", "XiaHe"},
		}
		for _, c := range starter {
			if _, err := s.CreateCard(ctx, todoCol, ownerID, models.CreateCardRequest{Title: c.title, Description: c.desc, Workers: c.workers}); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}
