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

func TestBuildSuccessMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []runner.Result
		want string
	}{
		{
			name: "single success -> Message verbatim",
			in: []runner.Result{
				{
					Status:    runner.StatusSuccess,
					Message:   "Booked 07:30 1 (TeeBox 1) on 2026/05/24 [BRC]. BookingID: BK001",
					CourseID:  "BRC",
					BookingID: "BK001",
				},
			},
			want: "Booked 07:30 1 (TeeBox 1) on 2026/05/24 [BRC]. BookingID: BK001",
		},
		{
			name: "two successes -> joined with cancel hint",
			in: []runner.Result{
				{
					Status:   runner.StatusSuccess,
					Message:  "Booked 07:30 [BRC] BK001",
					CourseID: "BRC",
				},
				{
					Status:   runner.StatusSuccess,
					Message:  "Booked 07:35 [PLC] BK002",
					CourseID: "PLC",
				},
			},
			want: "2 bookings made — cancel any you don't want: Booked 07:30 [BRC] BK001; Booked 07:35 [PLC] BK002",
		},
		{
			// Workers complete in non-deterministic order; the user-visible
			// message must be stable, ordered by CourseID.
			name: "two successes -> sorted by CourseID regardless of input order",
			in: []runner.Result{
				{
					Status:   runner.StatusSuccess,
					Message:  "Booked 07:35 [PLC] BK002",
					CourseID: "PLC",
				},
				{
					Status:   runner.StatusSuccess,
					Message:  "Booked 07:30 [BRC] BK001",
					CourseID: "BRC",
				},
			},
			want: "2 bookings made — cancel any you don't want: Booked 07:30 [BRC] BK001; Booked 07:35 [PLC] BK002",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSuccessMessage(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildOutcomeMessage(t *testing.T) {
	t.Parallel()
	brcSuccess := runner.Result{
		Status:   runner.StatusSuccess,
		Message:  "Booked 07:30 [BRC] BK001",
		CourseID: "BRC",
	}
	plcFail := runner.Result{
		Status:   runner.StatusFailed,
		Message:  "[PLC] no slot booked",
		CourseID: "PLC",
	}
	plcCancelled := runner.Result{
		Status:   runner.StatusCancelled,
		Message:  "[PLC] Run cancelled",
		CourseID: "PLC",
	}
	tests := []struct {
		name       string
		successes  []runner.Result
		allResults []runner.Result
		want       string
	}{
		{
			name:       "all succeeded -> no parenthetical",
			successes:  []runner.Result{brcSuccess},
			allResults: []runner.Result{brcSuccess},
			want:       "Booked 07:30 [BRC] BK001",
		},
		{
			name:       "mixed: success + fail -> non-success appended",
			successes:  []runner.Result{brcSuccess},
			allResults: []runner.Result{brcSuccess, plcFail},
			want:       "Booked 07:30 [BRC] BK001 (also: [PLC] no slot booked)",
		},
		{
			name:       "mixed: success + cancelled -> non-success appended",
			successes:  []runner.Result{brcSuccess},
			allResults: []runner.Result{brcSuccess, plcCancelled},
			want:       "Booked 07:30 [BRC] BK001 (also: [PLC] Run cancelled)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildOutcomeMessage(tc.successes, tc.allResults)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAggregateFailureMessages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []runner.Result
		want string
	}{
		{
			name: "no results -> sentinel fallback",
			in:   nil,
			want: "no slot booked",
		},
		{
			name: "single result -> Message verbatim",
			in: []runner.Result{
				{CourseID: "BRC", Message: "[BRC] no slot booked"},
			},
			want: "[BRC] no slot booked",
		},
		{
			name: "two results joined with semicolon, sorted by CourseID",
			in: []runner.Result{
				{CourseID: "PLC", Message: "[PLC] all tee times already reserved"},
				{CourseID: "BRC", Message: "[BRC] no slot booked"},
			},
			want: "[BRC] no slot booked; [PLC] all tee times already reserved",
		},
		{
			name: "empty messages dropped",
			in: []runner.Result{
				{CourseID: "BRC", Message: ""},
				{CourseID: "PLC", Message: "[PLC] no slot booked"},
			},
			want: "[PLC] no slot booked",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := aggregateFailureMessages(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}
