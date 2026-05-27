package agent

import (
	"fmt"
	"strings"
)

var defaultInProgressKeys = []string{
	"进行", "doing", "开发", "ai", "智能", "research", "研发", "implement", "wip",
}

var defaultTodoKeys = []string{
	"待办", "明天", "todo", "计划", "backlog", "排队", "未开始", "plan",
}

func ColumnMatches(title string, keys []string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, k := range keys {
		if strings.Contains(t, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func IsInProgressColumn(title string) bool {
	return ColumnMatches(title, defaultInProgressKeys)
}

func IsTodoColumn(title string) bool {
	return ColumnMatches(title, defaultTodoKeys)
}

func CategoryTitle(category, summary string, progress int) string {
	category = strings.TrimSpace(category)
	summary = strings.TrimSpace(summary)
	if category == "" {
		return summary
	}
	if progress > 0 {
		return fmt.Sprintf("【%s】%d%% %s", category, progress, summary)
	}
	return fmt.Sprintf("【%s】%s", category, summary)
}
