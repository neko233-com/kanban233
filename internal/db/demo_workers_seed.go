package db

import (
	"context"
)

// demoCardWorkerPatch maps demo card title prefix → workers for existing databases.
var demoCardWorkerPatch = map[string]string{
	"【AI】80%":        "LiLei",
	"【后端】100%":       "XiaHe",
	"【前端】优化":        "LiLei, XiaHe",
	"明天：验证日报":       "XiaHe",
	"① 点击卡片":         "LiLei",
	"② 拖拽卡片":         "XiaHe",
	"③ 完结":            "LiLei, XiaHe",
	"【示例】50%":        "XiaHe",
	"欢迎使用 Kanban233": "LiLei",
	"创建第一张任务卡片":    "XiaHe",
}

func (s *Store) seedDemoCardWorkers(ctx context.Context, ownerID int64) error {
	for prefix, workers := range demoCardWorkerPatch {
		_, err := s.db.ExecContext(ctx, s.q(`
UPDATE cards SET workers = ?
WHERE (workers IS NULL OR workers = '')
AND title LIKE ? || '%'
AND column_id IN (
  SELECT c.id FROM columns c
  JOIN boards b ON b.id = c.board_id
  JOIN project_groups g ON g.id = b.project_group_id
  WHERE g.owner_id = ?
)`), workers, prefix, ownerID)
		if err != nil {
			return err
		}
	}
	return nil
}
