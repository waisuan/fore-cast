package slotutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/booker"
)

type SlotutilSuite struct {
	suite.Suite
}

func (s *SlotutilSuite) TestCourseForDate() {
	tests := []struct {
		date string
		want string
		desc string
	}{
		{"2026/02/23", booker.CourseBRC, "Monday -> BRC"},
		{"2026/02/24", booker.CourseBRC, "Tuesday -> BRC"},
		{"2026/02/22", booker.CourseBRC, "Sunday -> BRC"},
		{"2026/02/25", booker.CoursePLC, "Wednesday -> PLC"},
		{"2026/02/26", booker.CoursePLC, "Thursday -> PLC"},
		{"2026/02/27", booker.CoursePLC, "Friday -> PLC"},
		{"2026/02/28", booker.CoursePLC, "Saturday -> PLC"},
		{"invalid", booker.CoursePLC, "invalid date -> fallback PLC"},
		{"", booker.CoursePLC, "empty -> fallback PLC"},
	}
	for _, tt := range tests {
		s.Run(tt.desc, func() {
			got := CourseForDate(tt.date)
			s.Assert().Equal(tt.want, got)
		})
	}
}

func (s *SlotutilSuite) TestParseClockHM() {
	h, m, sec, err := ParseClockHM("21:59")
	s.Require().NoError(err)
	s.Assert().Equal(21, h)
	s.Assert().Equal(59, m)
	s.Assert().Equal(0, sec)
	h, m, sec, err = ParseClockHM("22:00:03")
	s.Require().NoError(err)
	s.Assert().Equal(22, h)
	s.Assert().Equal(0, m)
	s.Assert().Equal(3, sec)
	s.Assert().Equal("22:00:03", FormatClockHM(22, 0, 3))
	_, _, _, err = ParseClockHM("")
	s.Assert().Error(err)
	_, _, _, err = ParseClockHM("25:00")
	s.Assert().Error(err)
	norm, err := NormalizeClockHM("9:05")
	s.Require().NoError(err)
	s.Assert().Equal("09:05:00", norm)
}

func (s *SlotutilSuite) TestParseCutoff() {
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
		s.Run(tt.desc, func() {
			got, err := ParseCutoff(tt.in)
			if tt.err {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Assert().Equal(tt.want, got)
		})
	}
}

func (s *SlotutilSuite) TestSlotsBeforeCutoff() {
	cutoff := "1899-12-30T08:15:00"
	mkSlot := func(teeTime string) booker.TeeTimeSlot {
		return booker.TeeTimeSlot{TeeTime: teeTime, CourseID: "BRC", Session: "Morning", TeeBox: "1"}
	}
	tests := []struct {
		name   string
		slots  []booker.TeeTimeSlot
		cutoff string
		want   []string
	}{
		{"empty", nil, cutoff, nil},
		{"all before", []booker.TeeTimeSlot{mkSlot("1899-12-30T07:00:00"), mkSlot("1899-12-30T08:00:00")}, cutoff, []string{"1899-12-30T07:00:00", "1899-12-30T08:00:00"}},
		{"all after", []booker.TeeTimeSlot{mkSlot("1899-12-30T08:30:00"), mkSlot("1899-12-30T09:00:00")}, cutoff, nil},
		{"mixed", []booker.TeeTimeSlot{mkSlot("1899-12-30T09:00:00"), mkSlot("1899-12-30T07:30:00"), mkSlot("1899-12-30T08:00:00")}, cutoff, []string{"1899-12-30T07:30:00", "1899-12-30T08:00:00"}},
		{"single before", []booker.TeeTimeSlot{mkSlot("1899-12-30T07:37:00")}, cutoff, []string{"1899-12-30T07:37:00"}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := SlotsBeforeCutoff(tt.slots, tt.cutoff)
			s.Require().Len(got, len(tt.want))
			for i := range got {
				s.Assert().Equal(tt.want[i], got[i].TeeTime)
			}
		})
	}
}

func (s *SlotutilSuite) TestValidateDate() {
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
		s.Run(tt.desc, func() {
			err := ValidateDate(tt.date)
			if tt.valid {
				s.Assert().NoError(err)
			} else {
				s.Assert().Error(err)
			}
		})
	}
}

func (s *SlotutilSuite) TestFormatCutoffDisplay() {
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
		s.Run(tt.desc, func() {
			got := FormatCutoffDisplay(tt.in)
			s.Assert().Equal(tt.want, got)
		})
	}
}

func (s *SlotutilSuite) TestDateOneWeekAhead() {
	got := DateOneWeekAhead()
	_, err := time.Parse("2006/01/02", got)
	s.Require().NoError(err)
	expected := time.Now().AddDate(0, 0, 7).Format("2006/01/02")
	s.Assert().Equal(expected, got)
}

func (s *SlotutilSuite) TestOtherCourse() {
	tests := []struct {
		in, want, desc string
	}{
		{booker.CourseBRC, booker.CoursePLC, "BRC -> PLC"},
		{booker.CoursePLC, booker.CourseBRC, "PLC -> BRC"},
		{"brc", booker.CoursePLC, "lower-case BRC normalized"},
		{" PLC ", booker.CourseBRC, "padded PLC normalized"},
		{"", "", "empty returns empty"},
		{"FOO", "", "unknown returns empty"},
	}
	for _, tt := range tests {
		s.Run(tt.desc, func() {
			s.Assert().Equal(tt.want, OtherCourse(tt.in))
		})
	}
}

func (s *SlotutilSuite) TestWeekdayCode() {
	s.Assert().Equal("MON", WeekdayCode(time.Monday))
	s.Assert().Equal("SUN", WeekdayCode(time.Sunday))
	s.Assert().Equal("SAT", WeekdayCode(time.Saturday))
	s.Assert().Equal("", WeekdayCode(time.Weekday(-1)), "out-of-range below Sunday")
	s.Assert().Equal("", WeekdayCode(time.Weekday(7)), "out-of-range above Saturday")
}

func (s *SlotutilSuite) TestParseWeekdayCodes() {
	tests := []struct {
		in      string
		want    map[time.Weekday]struct{}
		wantErr bool
		desc    string
	}{
		{"", nil, false, "empty -> nil"},
		{"   ", nil, false, "whitespace -> nil"},
		{"MON", map[time.Weekday]struct{}{time.Monday: {}}, false, "single day"},
		{
			"MON,WED,SAT",
			map[time.Weekday]struct{}{time.Monday: {}, time.Wednesday: {}, time.Saturday: {}},
			false,
			"multi day",
		},
		{
			"mon, wed ,SAT",
			map[time.Weekday]struct{}{time.Monday: {}, time.Wednesday: {}, time.Saturday: {}},
			false,
			"casing + whitespace tolerated",
		},
		{
			"MON,MON,TUE",
			map[time.Weekday]struct{}{time.Monday: {}, time.Tuesday: {}},
			false,
			"duplicates deduped",
		},
		{
			"MON,",
			map[time.Weekday]struct{}{time.Monday: {}},
			false,
			"trailing comma -> empty token skipped",
		},
		{
			"MON, ,SAT",
			map[time.Weekday]struct{}{time.Monday: {}, time.Saturday: {}},
			false,
			"whitespace-only middle token skipped",
		},
		{"FOO", nil, true, "unknown code rejected"},
		{"MON,XYZ", nil, true, "one bad code rejects whole list"},
	}
	for _, tt := range tests {
		s.Run(tt.desc, func() {
			got, err := ParseWeekdayCodes(tt.in)
			if tt.wantErr {
				s.Assert().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Assert().Equal(tt.want, got)
		})
	}
}

func (s *SlotutilSuite) TestCanonicalWeekdayCodes() {
	s.Assert().Nil(CanonicalWeekdayCodes(""), "empty -> nil")
	s.Assert().Nil(CanonicalWeekdayCodes("   "), "whitespace -> nil")
	s.Assert().Nil(CanonicalWeekdayCodes("FOO"), "malformed token -> nil (parse error)")
	s.Assert().Equal([]string{"MON", "SAT"}, CanonicalWeekdayCodes("SAT,MON"),
		"unordered input round-trips to canonical Mon..Sun")
	s.Assert().Equal([]string{"MON", "WED"}, CanonicalWeekdayCodes("mon , WED"),
		"casing + whitespace tolerated, canonical output")
}

func (s *SlotutilSuite) TestSerializeWeekdaySet() {
	s.Assert().Equal("", SerializeWeekdaySet(nil))
	s.Assert().Equal("", SerializeWeekdaySet(map[time.Weekday]struct{}{}))
	s.Assert().Equal(
		"MON,WED,SAT",
		SerializeWeekdaySet(map[time.Weekday]struct{}{
			time.Saturday:  {},
			time.Monday:    {},
			time.Wednesday: {},
		}),
		"output is Mon..Sun ordered regardless of insert order",
	)
	s.Assert().Equal(
		"MON,TUE,WED,THU,FRI,SAT,SUN",
		SerializeWeekdaySet(map[time.Weekday]struct{}{
			time.Sunday: {}, time.Monday: {}, time.Tuesday: {}, time.Wednesday: {},
			time.Thursday: {}, time.Friday: {}, time.Saturday: {},
		}),
	)
}

func TestSlotutilSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SlotutilSuite))
}
