package slotutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/saujana"
)

// Default cutoff: do not book any slot at or after this time.
const DefaultCutoffTeeTime = "1899-12-30T08:15:00"

// CourseForDate returns BRC for Mon/Tue/Sun, PLC otherwise (weekday from date string YYYY/MM/DD).
func CourseForDate(txnDate string) string {
	t, err := time.Parse("2006/01/02", txnDate)
	if err != nil {
		return saujana.CoursePLC
	}
	switch t.Weekday() {
	case time.Sunday, time.Monday, time.Tuesday:
		return saujana.CourseBRC
	default:
		return saujana.CoursePLC
	}
}

// ParseCutoff converts a time like "8:15" or "07:30" to API format "1899-12-30THH:MM:00". Empty string returns default.
func ParseCutoff(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultCutoffTeeTime, nil
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		t, err = time.Parse("3:04", s)
	}
	if err != nil {
		return "", fmt.Errorf("invalid cutoff %q: use HH:MM or H:MM (e.g. 8:15 or 07:30)", s)
	}
	return "1899-12-30T" + t.Format("15:04:05"), nil
}

// SlotsBeforeCutoff returns slots with TeeTime before cutoff, sorted earliest first.
func SlotsBeforeCutoff(slots []saujana.TeeTimeSlot, cutoffTeeTime string) []saujana.TeeTimeSlot {
	var out []saujana.TeeTimeSlot
	for _, s := range slots {
		if s.TeeTime < cutoffTeeTime {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TeeTime < out[j].TeeTime
	})
	return out
}

// ValidateDate checks YYYY/MM/DD and returns an error if invalid.
func ValidateDate(s string) error {
	_, err := time.Parse("2006/01/02", s)
	if err != nil {
		return fmt.Errorf("invalid date %q: use YYYY/MM/DD (e.g. 2026/02/25)", s)
	}
	return nil
}

// FormatCutoffDisplay returns a human-readable cutoff time, e.g. "8:15 AM".
func FormatCutoffDisplay(cutoffTeeTime string) string {
	if len(cutoffTeeTime) < 19 {
		return cutoffTeeTime
	}
	t, err := time.Parse("15:04:05", cutoffTeeTime[11:19])
	if err != nil {
		return cutoffTeeTime[11:19]
	}
	return t.Format("3:04 PM")
}

// DateOneWeekAhead returns the date 7 days from today in YYYY/MM/DD.
func DateOneWeekAhead() string {
	t := time.Now().AddDate(0, 0, 7)
	return t.Format("2006/01/02")
}
