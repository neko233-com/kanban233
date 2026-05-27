package locale

import (
	"strings"
	"time"
)

const WeekStartMonday = "monday"
const WeekStartSunday = "sunday"

func ParseWeekStart(s string) time.Weekday {
	if strings.EqualFold(s, WeekStartSunday) {
		return time.Sunday
	}
	return time.Monday
}

func WeekBounds(day time.Time, weekStart time.Weekday) (start, end time.Time) {
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	d := day
	for d.Weekday() != weekStart {
		d = d.AddDate(0, 0, -1)
	}
	start = d
	end = d.AddDate(0, 0, 6)
	return start, end
}

func WeekdayCN(t time.Time) string {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[t.Weekday()]
}
