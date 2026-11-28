package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/waisuan/alfred/internal/preset"
	"github.com/waisuan/alfred/internal/runner"
)

// Concrete dates whose weekdays we depend on in tests:
//
//	2026/04/29 → Wednesday (date-based fallback yields PLC)
//	2026/05/02 → Saturday
const (
	wednesdayTxnDate = "2026/04/29"
	saturdayTxnDate  = "2026/05/02"
)

func TestResolveCourseForRun_NoOverride_UsesPresetCourse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	p := preset.Preset{UserName: "u", Course: sql.NullString{String: "BRC", Valid: true}}
	course, clear, overrideActive := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "BRC", course)
	assert.False(t, clear)
	assert.False(t, overrideActive)
}

func TestResolveCourseForRun_NoOverride_NoPresetCourse_FallsBackToDate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	p := preset.Preset{UserName: "u"}
	course, clear, overrideActive := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "PLC", course)
	assert.False(t, clear)
	assert.False(t, overrideActive)
}

func TestResolveCourseForRun_OverrideOnce_UsesOverrideAndMarksClear(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	p := preset.Preset{
		UserName:       "u",
		Course:         sql.NullString{String: "BRC", Valid: true},
		OverrideCourse: sql.NullString{String: "PLC", Valid: true},
	}
	course, clear, overrideActive := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "PLC", course)
	assert.True(t, clear, "next-run-only override must be cleared after the run")
	assert.True(t, overrideActive, "once-off override still counts as an active override for fan-out gating")
}

func TestResolveCourseForRun_OverrideActive_UsesOverrideButDoesNotClear(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	now := time.Now()
	p := preset.Preset{
		UserName:       "u",
		Course:         sql.NullString{String: "BRC", Valid: true},
		OverrideCourse: sql.NullString{String: "PLC", Valid: true},
		OverrideUntil:  sql.NullTime{Time: now.Add(48 * time.Hour), Valid: true},
	}
	course, clear, overrideActive := resolveCourseForRun(svc, p, wednesdayTxnDate, now)
	assert.Equal(t, "PLC", course)
	assert.False(t, clear, "until-bounded override is cleared lazily, not after each run")
	assert.True(t, overrideActive)
}

func TestResolveCourseForRun_OverrideExpired_ClearsImmediatelyAndUsesDefault(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	svc.EXPECT().ClearCourseOverride("u").Return(nil)

	now := time.Now()
	p := preset.Preset{
		UserName:       "u",
		Course:         sql.NullString{String: "BRC", Valid: true},
		OverrideCourse: sql.NullString{String: "PLC", Valid: true},
		OverrideUntil:  sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
	}
	course, clear, overrideActive := resolveCourseForRun(svc, p, wednesdayTxnDate, now)
	assert.Equal(t, "BRC", course)
	assert.False(t, clear)
	assert.False(t, overrideActive, "expired override does not gate alt-course fan-out")
}

func TestResolveCoursesForRun(t *testing.T) {
	t.Parallel()
	allDaysValid := sql.NullString{String: "MON,TUE,WED,THU,FRI,SAT,SUN", Valid: true}
	tests := []struct {
		name           string
		primary        string
		txnDate        string
		altDays        sql.NullString
		overrideActive bool
		want           []string
	}{
		{
			name:    "altDays NULL -> primary only",
			primary: "BRC",
			txnDate: wednesdayTxnDate,
			altDays: sql.NullString{},
			want:    []string{"BRC"},
		},
		{
			name:    "altDays empty string -> primary only",
			primary: "BRC",
			txnDate: wednesdayTxnDate,
			altDays: sql.NullString{String: "", Valid: true},
			want:    []string{"BRC"},
		},
		{
			name:    "altDays set but today not in set -> primary only",
			primary: "BRC",
			txnDate: wednesdayTxnDate, // Wednesday
			altDays: sql.NullString{String: "SAT,SUN", Valid: true},
			want:    []string{"BRC"},
		},
		{
			name:    "altDays set and today in set -> primary + opposite",
			primary: "BRC",
			txnDate: saturdayTxnDate,
			altDays: sql.NullString{String: "SAT", Valid: true},
			want:    []string{"BRC", "PLC"},
		},
		{
			name:    "altDays set, today in set, primary is PLC -> primary + BRC",
			primary: "PLC",
			txnDate: saturdayTxnDate,
			altDays: sql.NullString{String: "SAT", Valid: true},
			want:    []string{"PLC", "BRC"},
		},
		{
			name:           "override active suppresses alt even on a configured day",
			primary:        "PLC",
			txnDate:        saturdayTxnDate,
			altDays:        allDaysValid,
			overrideActive: true,
			want:           []string{"PLC"},
		},
		{
			name:    "primary lowercased is normalized; alt still resolves",
			primary: "brc",
			txnDate: saturdayTxnDate,
			altDays: allDaysValid,
			want:    []string{"BRC", "PLC"},
		},
		{
			name:    "invalid weekday code in altDays -> conservative primary only",
			primary: "BRC",
			txnDate: saturdayTxnDate,
			altDays: sql.NullString{String: "SAT,XYZ", Valid: true},
			want:    []string{"BRC"},
		},
		{
			name:    "invalid txnDate -> conservative primary only",
			primary: "BRC",
			txnDate: "not-a-date",
			altDays: allDaysValid,
			want:    []string{"BRC"},
		},
		{
			name:    "unknown primary course -> alt undefined -> primary only",
			primary: "FOO",
			txnDate: saturdayTxnDate,
			altDays: allDaysValid,
			want:    []string{"FOO"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCoursesForRun(tc.primary, tc.txnDate, tc.altDays, tc.overrideActive)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOutcomeMessage(t *testing.T) {
	t.Parallel()
	brc := runner.Result{Status: runner.StatusSuccess, Message: "Booked 07:30 [BRC] BK001", CourseID: "BRC"}
	plc := runner.Result{Status: runner.StatusSuccess, Message: "Booked 07:35 [PLC] BK002", CourseID: "PLC"}
	plcFail := runner.Result{Status: runner.StatusFailed, Message: "[PLC] no slot booked", CourseID: "PLC"}
	plcCancelled := runner.Result{Status: runner.StatusCancelled, Message: "[PLC] Run cancelled", CourseID: "PLC"}
	brcFail := runner.Result{Status: runner.StatusFailed, Message: "[BRC] no slot booked", CourseID: "BRC"}
	plcReserved := runner.Result{Status: runner.StatusFailed, Message: "[PLC] all tee times already reserved", CourseID: "PLC"}

	tests := []struct {
		name       string
		in         []runner.Result
		want       string
		wantBooked int
	}{
		{"no results -> sentinel", nil, "no slot booked", 0},
		{"single success verbatim", []runner.Result{brc}, "Booked 07:30 [BRC] BK001", 1},
		{"two successes sorted with cancel hint", []runner.Result{plc, brc},
			"2 bookings made — cancel any you don't want: Booked 07:30 [BRC] BK001; Booked 07:35 [PLC] BK002", 2},
		{"success + fail", []runner.Result{plcFail, brc}, "Booked 07:30 [BRC] BK001 (also: [PLC] no slot booked)", 1},
		{"success + cancelled", []runner.Result{brc, plcCancelled}, "Booked 07:30 [BRC] BK001 (also: [PLC] Run cancelled)", 1},
		{"failures sorted and joined", []runner.Result{plcReserved, brcFail},
			"[BRC] no slot booked; [PLC] all tee times already reserved", 0},
		{"empty failure messages dropped", []runner.Result{{CourseID: "BRC"}, plcFail}, "[PLC] no slot booked", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, booked := outcomeMessage(tc.in)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantBooked, booked)
		})
	}
}
