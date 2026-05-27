package agent

import (
	"strings"
	"testing"

	"github.com/neko233/kanban233/internal/models"
)

func TestParseTaskLine(t *testing.T) {
	cat, prog, sum := ParseTaskLine("【后端】100% 排行榜数据维护逻辑调整", "读快照全服缓存")
	if cat != "后端" || prog != 100 || sum == "" {
		t.Fatalf("parse failed: %q %d %q", cat, prog, sum)
	}
}

func TestRenderDailyMarkdown(t *testing.T) {
	md := RenderDailyMarkdown(models.AgentCollaborationDay{
		Date: "2026-05-26", Weekday: "周二",
		Users: []models.AgentUserDay{{
			Username: "zaixiao",
			CompletedToday: []models.AgentTaskItem{{
				Category: "后端", Progress: 100, Summary: "排行榜数据维护逻辑调整 - 读快照",
			}},
			Tomorrow: []models.AgentTaskItem{{
				Category: "后端", Summary: "验证网关架构方案落地进度",
			}},
		}},
	})
	if !strings.Contains(md, "# 2026-05-26 周二") || !strings.Contains(md, "zaixiao") {
		t.Fatalf("unexpected markdown:\n%s", md)
	}
}
