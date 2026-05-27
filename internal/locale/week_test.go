package locale

import (
	"testing"
	"time"
)

func TestWeekBoundsMondayFirst(t *testing.T) {
	day := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC) // Tuesday
	start, end := WeekBounds(day, time.Monday)
	if start.Format("2006-01-02") != "2026-05-25" {
		t.Fatalf("week start = %s", start.Format("2006-01-02"))
	}
	if end.Format("2006-01-02") != "2026-05-31" {
		t.Fatalf("week end = %s", end.Format("2006-01-02"))
	}
	if WeekdayCN(start) != "周一" {
		t.Fatalf("expected Monday, got %s", WeekdayCN(start))
	}
}

func TestParseWeekStartDefaultMonday(t *testing.T) {
	if ParseWeekStart("") != time.Monday {
		t.Fatal("empty should default to Monday")
	}
	if ParseWeekStart("monday") != time.Monday {
		t.Fatal("monday")
	}
}
