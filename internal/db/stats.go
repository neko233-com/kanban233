package db

import (
	"context"
	"database/sql"
	"strings"

	"github.com/neko233/kanban233/internal/models"
)

type statsCardRow struct {
	Status     string
	Column     string
	Workers    string
}

func (s *Store) GetGroupStats(ctx context.Context, groupID, userID int64) (*models.GroupStats, error) {
	if _, err := s.getGroupAccess(ctx, groupID, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, s.q(`
SELECT c.status, col.title, COALESCE(c.workers, '')
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
WHERE b.project_group_id = ?`), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return aggregateStats(rows)
}

func (s *Store) GetResearchStats(ctx context.Context, userID int64) (*models.ResearchStats, error) {
	groups, err := s.ListProjectGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return &models.ResearchStats{
			Summary:  models.GroupStatsSummary{},
			ByWorker: []models.GroupWorkerStat{},
			ByStatus: []models.GroupStatusStat{},
		}, nil
	}
	ids := make([]any, len(groups))
	placeholders := make([]string, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
		placeholders[i] = "?"
	}
	query := `
SELECT c.status, col.title, COALESCE(c.workers, '')
FROM cards c
JOIN columns col ON col.id = c.column_id
JOIN boards b ON b.id = col.board_id
WHERE b.project_group_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, s.q(query), ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	gs, err := aggregateStats(rows)
	if err != nil {
		return nil, err
	}
	return &models.ResearchStats{
		Summary:  gs.Summary,
		ByWorker: gs.ByWorker,
		ByStatus: gs.ByStatus,
	}, nil
}

func aggregateStats(rows *sql.Rows) (*models.GroupStats, error) {
	workerMap := map[string]*models.GroupWorkerStat{}
	columnMap := map[string]int{}
	summary := models.GroupStatsSummary{}
	workerNames := map[string]bool{}

	for rows.Next() {
		var row statsCardRow
		if err := rows.Scan(&row.Status, &row.Column, &row.Workers); err != nil {
			return nil, err
		}
		summary.Total++
		switch row.Status {
		case models.CardStatusActive:
			summary.Active++
			columnMap[row.Column]++
		case models.CardStatusArchived:
			summary.Archived++
		case models.CardStatusCompleted:
			summary.Completed++
		}
		names := parseWorkers(row.Workers)
		if len(names) == 0 {
			names = []string{"未指定"}
		}
		for _, name := range names {
			workerNames[name] = true
			ws, ok := workerMap[name]
			if !ok {
				ws = &models.GroupWorkerStat{Name: name}
				workerMap[name] = ws
			}
			ws.Total++
			if row.Status == models.CardStatusActive {
				ws.Active++
			} else {
				ws.Finished++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	summary.Workers = len(workerNames)

	byWorker := make([]models.GroupWorkerStat, 0, len(workerMap))
	for _, w := range workerMap {
		byWorker = append(byWorker, *w)
	}
	sortWorkerStats(byWorker)

	byColumn := make([]models.GroupColumnStat, 0, len(columnMap))
	for col, count := range columnMap {
		byColumn = append(byColumn, models.GroupColumnStat{Column: col, Count: count})
	}
	sortColumnStats(byColumn)

	byStatus := []models.GroupStatusStat{
		{Status: models.CardStatusActive, Count: summary.Active},
		{Status: models.CardStatusArchived, Count: summary.Archived},
		{Status: models.CardStatusCompleted, Count: summary.Completed},
	}

	return &models.GroupStats{
		Summary:  summary,
		ByWorker: byWorker,
		ByColumn: byColumn,
		ByStatus: byStatus,
	}, nil
}

func sortWorkerStats(stats []models.GroupWorkerStat) {
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].Total > stats[i].Total || (stats[j].Total == stats[i].Total && stats[j].Name < stats[i].Name) {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
}

func sortColumnStats(stats []models.GroupColumnStat) {
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].Count > stats[i].Count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}
}
