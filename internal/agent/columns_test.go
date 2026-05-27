package agent

import "testing"

func TestIsInProgressColumn(t *testing.T) {
	for _, title := range []string{"进行中", "AI 研发", "Doing", "智能开发"} {
		if !IsInProgressColumn(title) {
			t.Fatalf("expected in-progress column: %q", title)
		}
	}
}

func TestCategoryTitle(t *testing.T) {
	got := CategoryTitle("AI", "接入 MCP", 80)
	if got != "【AI】80% 接入 MCP" {
		t.Fatalf("unexpected title: %q", got)
	}
}
