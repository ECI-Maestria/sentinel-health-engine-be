package handlers

import (
	"fmt"
	"time"
)

// periodBounds computes the [from, to) UTC time range for a given period string
// and an optional reference date (YYYY-MM-DD). If date is empty, today is used.
//
// Supported periods:
//
//	"day"   → the 24 hours of the reference date
//	"week"  → Monday 00:00 … next Monday 00:00 (ISO week, Monday-first)
//	"month" → first day of month 00:00 … first day of next month 00:00
//	"year"  → Jan 1 00:00 … Jan 1 of next year 00:00
//
// Returns an error for unknown period values or malformed date strings.
// If period is empty, both from and to are zero values — callers must treat
// zero as "no filter".
func periodBounds(period, dateStr string) (from, to time.Time, err error) {
	if period == "" {
		return // zero == no filter
	}

	ref := time.Now().UTC()
	if dateStr != "" {
		ref, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("date must be YYYY-MM-DD, got %q", dateStr)
		}
		ref = ref.UTC()
	}

	// Truncate to start of the reference day.
	day := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)

	switch period {
	case "day":
		from = day
		to = day.Add(24 * time.Hour)

	case "week":
		// ISO week: Monday = day 1, Sunday = day 7.
		weekday := int(day.Weekday()) // 0 = Sunday
		if weekday == 0 {
			weekday = 7
		}
		monday := day.AddDate(0, 0, -(weekday - 1))
		from = monday
		to = monday.AddDate(0, 0, 7)

	case "month":
		from = time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0)

	case "year":
		from = time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(1, 0, 0)

	default:
		return time.Time{}, time.Time{}, fmt.Errorf("period must be one of: day, week, month, year")
	}

	return from, to, nil
}

// inRange reports whether t falls within [from, to).
// If both from and to are zero, every value is considered in range (no filter).
func inRange(t, from, to time.Time) bool {
	if from.IsZero() && to.IsZero() {
		return true
	}
	return !t.Before(from) && t.Before(to)
}
