package agent

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/models"
)

var taskTitleRE = regexp.MustCompile(`^【([^】]+)】\s*(\d+)?%?\s*(.*)$`)

func ParseTaskLine(title, description string) (category string, progress int, summary string) {
	title = strings.TrimSpace(title)
	if m := taskTitleRE.FindStringSubmatch(title); len(m) == 4 {
		category = m[1]
		if m[2] != "" {
			fmt.Sscanf(m[2], "%d", &progress)
		}
		summary = strings.TrimSpace(m[3])
	} else {
		summary = title
	}
	if description != "" {
		if summary != "" {
			summary += " - " + strings.TrimSpace(description)
		} else {
			summary = strings.TrimSpace(description)
		}
	}
	return category, progress, summary
}

func FormatTaskLine(index int, item models.AgentTaskItem) string {
	category, progress, summary := item.Category, item.Progress, item.Summary
	if category == "" {
		category = item.ColumnTitle
	}
	if category == "" {
		category = "任务"
	}
	if summary == "" {
		summary = item.Title
	}
	if progress > 0 {
		return fmt.Sprintf("%d. 【%s】%d%% %s", index, category, progress, summary)
	}
	return fmt.Sprintf("%d. 【%s】%s", index, category, summary)
}

func RenderDailyMarkdown(day models.AgentCollaborationDay) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s %s\n", day.Date, day.Weekday))
	if len(day.Users) == 0 {
		b.WriteString("\n（暂无协作数据）\n")
		return b.String()
	}
	for ui, u := range day.Users {
		if ui > 0 {
			b.WriteString("\n")
		}
		b.WriteString(u.Username)
		b.WriteByte('\n')
		if len(u.CompletedToday) == 0 && len(u.InProgressToday) == 0 {
			b.WriteString("（今日无记录）\n")
		} else {
			idx := 1
			for _, item := range u.CompletedToday {
				b.WriteString(FormatTaskLine(idx, item))
				b.WriteByte('\n')
				idx++
			}
			for _, item := range u.InProgressToday {
				p := item.Progress
				if p == 0 {
					p = 100
				}
				item.Progress = p
				b.WriteString(FormatTaskLine(idx, item))
				b.WriteByte('\n')
				idx++
			}
		}
		if len(u.Tomorrow) > 0 {
			b.WriteString("\n明天：\n")
			for i, item := range u.Tomorrow {
				b.WriteString(FormatTaskLine(i+1, item))
				b.WriteByte('\n')
			}
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func WeekdayCN(t time.Time) string {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[t.Weekday()]
}
