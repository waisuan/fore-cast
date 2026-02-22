package slotutil

import (
	"testing"
	"time"

	"github.com/waisuan/alfred/internal/saujana"
)

func TestCourseForDate(t *testing.T) {
	tests := []struct {
		date string
		want string
		desc string
	}{
		{"2026/02/23", saujana.CourseBRC, "Monday -> BRC"},
		{"2026/02/24", saujana.CourseBRC, "Tuesday -> BRC"},
		{"2026/02/22", saujana.CourseBRC, "Sunday -> BRC"},
		{"2026/02/25", saujana.CoursePLC, "Wednesday -> PLC"},
		{"2026/02/26", saujana.CoursePLC, "Thursday -> PLC"},
		{"2026/02/27", saujana.CoursePLC, "Friday -> PLC"},
		{"2026/02/28", saujana.CoursePLC, "Saturday -> PLC"},
		{"invalid", saujana.CoursePLC, "invalid date -> fallback PLC"},
		{"", saujana.CoursePLC, "empty -> fallback PLC"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := CourseForDate(tt.date)
			if got != tt.want {
				t.Errorf("CourseForDate(%q) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

func TestParseCutoff(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
		desc string
	}{
		{"", DefaultCutoffTeeTime, false, "empty -> default"},
		{"   ", DefaultCutoffTeeTime, false, "whitespace -> default"},
		{"8:15", "1899-12-30T08:15:00", false, "8:15"},
		{"08:15", "1899-12-30T08:15:00", false, "08:15"},
		{"07:30", "1899-12-30T07:30:00", false, "07:30"},
		{"7:30", "1899-12-30T07:30:00", false, "7:30"},
		{" 7:45 ", "1899-12-30T07:45:00", false, "trimmed"},
		{"25:00", "", true, "invalid hour"},
		{"abc", "", true, "invalid format"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := ParseCutoff(tt.in)
			if tt.err {
				if err == nil {
					t.Errorf("ParseCutoff(%q) expected error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCutoff(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseCutoff(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSlotsBeforeCutoff(t *testing.T) {
	cutoff := "1899-12-30T08:15:00"
	mkSlot := func(teeTime string) saujana.TeeTimeSlot {
		return saujana.TeeTimeSlot{TeeTime: teeTime, CourseID: "BRC", Session: "Morning", TeeBox: "1"}
	}
	tests := []struct {
		name   string
		slots  []saujana.TeeTimeSlot
		cutoff string
		want   []string // TeeTime values in order
	}{
		{"empty", nil, cutoff, nil},
		{"all before", []saujana.TeeTimeSlot{mkSlot("1899-12-30T07:00:00"), mkSlot("1899-12-30T08:00:00")}, cutoff, []string{"1899-12-30T07:00:00", "1899-12-30T08:00:00"}},
		{"all after", []saujana.TeeTimeSlot{mkSlot("1899-12-30T08:30:00"), mkSlot("1899-12-30T09:00:00")}, cutoff, nil},
		{"mixed", []saujana.TeeTimeSlot{mkSlot("1899-12-30T09:00:00"), mkSlot("1899-12-30T07:30:00"), mkSlot("1899-12-30T08:00:00")}, cutoff, []string{"1899-12-30T07:30:00", "1899-12-30T08:00:00"}},
		{"single before", []saujana.TeeTimeSlot{mkSlot("1899-12-30T07:37:00")}, cutoff, []string{"1899-12-30T07:37:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlotsBeforeCutoff(tt.slots, tt.cutoff)
			if len(got) != len(tt.want) {
				t.Fatalf("len(SlotsBeforeCutoff) = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].TeeTime != tt.want[i] {
					t.Errorf("result[%d].TeeTime = %q, want %q", i, got[i].TeeTime, tt.want[i])
				}
			}
		})
	}
}

func TestValidateDate(t *testing.T) {
	tests := []struct {
		date  string
		valid bool
		desc  string
	}{
		{"2026/02/25", true, "valid"},
		{"2026/01/01", true, "valid"},
		{"invalid", false, "invalid"},
		{"2026-02-25", false, "wrong separator"},
		{"", false, "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := ValidateDate(tt.date)
			if tt.valid && err != nil {
				t.Errorf("ValidateDate(%q) unexpected error: %v", tt.date, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("ValidateDate(%q) expected error", tt.date)
			}
		})
	}
}

func TestFormatCutoffDisplay(t *testing.T) {
	tests := []struct {
		in   string
		want string
		desc string
	}{
		{"1899-12-30T08:15:00", "8:15 AM", "8:15 AM"},
		{"1899-12-30T07:30:00", "7:30 AM", "7:30 AM"},
		{"1899-12-30T13:00:00", "1:00 PM", "1 PM"},
		{"short", "short", "short string returned as-is"},
		{"", "", "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := FormatCutoffDisplay(tt.in)
			if got != tt.want {
				t.Errorf("FormatCutoffDisplay(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDateOneWeekAhead(t *testing.T) {
	got := DateOneWeekAhead()
	_, err := time.Parse("2006/01/02", got)
	if err != nil {
		t.Errorf("DateOneWeekAhead() = %q, invalid format: %v", got, err)
	}
	expected := time.Now().AddDate(0, 0, 7).Format("2006/01/02")
	if got != expected {
		t.Errorf("DateOneWeekAhead() = %q, want %q", got, expected)
	}
}
