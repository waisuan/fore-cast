package slotutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
)

// Default cutoff: do not book any slot at or after this time.
const DefaultCutoffTeeTime = "1899-12-30T08:15:00"

// IsClubCourse reports whether id is a known club course code (BRC or PLC).
func IsClubCourse(id string) bool {
	return id == booker.CourseBRC || id == booker.CoursePLC
}

// NormalizeCourseCode trims and upper-cases a user-supplied course code so it
// can be compared / persisted in canonical form. Empty input round-trips to "".
func NormalizeCourseCode(s string) string {
	return strings.TrimSpace(strings.ToUpper(s))
}

// CourseForDate returns BRC for Mon/Tue/Sun, PLC otherwise (weekday from date string YYYY/MM/DD).
func CourseForDate(txnDate string) string {
	t, err := time.Parse("2006/01/02", txnDate)
	if err != nil {
		return booker.CoursePLC
	}
	switch t.Weekday() {
	case time.Sunday, time.Monday, time.Tuesday:
		return booker.CourseBRC
	default:
		return booker.CoursePLC
	}
}

// OtherCourse returns the opposite club course code (BRC <-> PLC). Returns ""
// for any input that isn't a known club course.
func OtherCourse(course string) string {
	switch NormalizeCourseCode(course) {
	case booker.CourseBRC:
		return booker.CoursePLC
	case booker.CoursePLC:
		return booker.CourseBRC
	default:
		return ""
	}
}

// weekdayCodes maps the canonical 3-letter upper-case code to time.Weekday.
// Sunday matches Go's time.Weekday zero value so iteration order is stable.
var weekdayCodes = map[string]time.Weekday{
	"SUN": time.Sunday,
	"MON": time.Monday,
	"TUE": time.Tuesday,
	"WED": time.Wednesday,
	"THU": time.Thursday,
	"FRI": time.Friday,
	"SAT": time.Saturday,
}

// weekdayCodeByDay is the inverse lookup of weekdayCodes, indexed by the
// time.Weekday integer value (Sun=0..Sat=6).
var weekdayCodeByDay = [7]string{
	time.Sunday:    "SUN",
	time.Monday:    "MON",
	time.Tuesday:   "TUE",
	time.Wednesday: "WED",
	time.Thursday:  "THU",
	time.Friday:    "FRI",
	time.Saturday:  "SAT",
}

// weekdayOrder is the canonical Mon..Sun ordering used when serializing a set
// of weekdays back to its storage form.
var weekdayOrder = []time.Weekday{
	time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
	time.Friday, time.Saturday, time.Sunday,
}

// WeekdayCode returns the canonical 3-letter code (e.g. "MON") for w. Returns
// "" for any value outside [time.Sunday, time.Saturday].
func WeekdayCode(w time.Weekday) string {
	if w < time.Sunday || w > time.Saturday {
		return ""
	}
	return weekdayCodeByDay[w]
}

// ParseWeekdayCodes parses a comma-separated list of weekday codes (e.g.
// "MON,WED,SAT") into a deduped set. Whitespace and casing are tolerated. An
// empty / whitespace-only input returns nil with no error. Unknown codes
// produce an error.
func ParseWeekdayCodes(s string) (map[time.Weekday]struct{}, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	out := make(map[time.Weekday]struct{}, 7)
	for _, raw := range strings.Split(s, ",") {
		code := strings.TrimSpace(strings.ToUpper(raw))
		if code == "" {
			continue
		}
		w, ok := weekdayCodes[code]
		if !ok {
			return nil, fmt.Errorf("invalid weekday code %q: expected one of MON,TUE,WED,THU,FRI,SAT,SUN", code)
		}
		out[w] = struct{}{}
	}
	return out, nil
}

// SerializeWeekdaySet renders a set back to its canonical comma-separated
// storage form, ordered Mon..Sun. Empty set serializes to "".
func SerializeWeekdaySet(set map[time.Weekday]struct{}) string {
	return strings.Join(orderedWeekdayCodes(set), ",")
}

// CanonicalWeekdayCodes parses a stored alt-course-days string and returns its
// canonical, validated, ordered slice form (Mon..Sun, deduped, unknown tokens
// dropped). Returns nil for empty / malformed input. Use this when reading
// stored state for consumers that may not re-validate (e.g. API responses,
// log fields).
func CanonicalWeekdayCodes(stored string) []string {
	set, err := ParseWeekdayCodes(stored)
	if err != nil || len(set) == 0 {
		return nil
	}
	return orderedWeekdayCodes(set)
}

func orderedWeekdayCodes(set map[time.Weekday]struct{}) []string {
	parts := make([]string, 0, len(set))
	for _, w := range weekdayOrder {
		if _, ok := set[w]; ok {
			parts = append(parts, WeekdayCode(w))
		}
	}
	return parts
}

func parseClock(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("15:04", s)
	if err != nil {
		t, err = time.Parse("3:04", s)
	}
	return t, err
}

// ParseCutoff converts a time like "8:15" or "07:30" to API format "1899-12-30THH:MM:00". Empty string returns default.
func ParseCutoff(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return DefaultCutoffTeeTime, nil
	}
	t, err := parseClock(s)
	if err != nil {
		return "", fmt.Errorf("invalid cutoff %q: use HH:MM or H:MM (e.g. 8:15 or 07:30)", s)
	}
	return "1899-12-30T" + t.Format("15:04:05"), nil
}

// ParseClockHM parses a wall-clock time like "21:59" or "9:05". Empty input is an error
// (callers that want a default should apply it before calling).
func ParseClockHM(s string) (hour, minute int, err error) {
	if strings.TrimSpace(s) == "" {
		return 0, 0, fmt.Errorf("empty clock time")
	}
	t, err := parseClock(s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time %q: use HH:MM (e.g. 21:59 or 22:00)", s)
	}
	return t.Hour(), t.Minute(), nil
}

// NormalizeClockHM parses s and returns canonical HH:MM.
func NormalizeClockHM(s string) (string, error) {
	h, mi, err := ParseClockHM(s)
	if err != nil {
		return "", err
	}
	return FormatClockHM(h, mi), nil
}

// FormatClockHM returns hour:minute as HH:MM (24-hour).
func FormatClockHM(hour, minute int) string {
	return time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC).Format("15:04")
}

// SlotsBeforeCutoff returns slots with TeeTime before cutoff, sorted earliest first.
func SlotsBeforeCutoff(slots []booker.TeeTimeSlot, cutoffTeeTime string) []booker.TeeTimeSlot {
	var out []booker.TeeTimeSlot
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
