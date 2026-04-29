package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/waisuan/alfred/internal/preset"
)

// 2026/04/29 is a Wednesday → date-based fallback yields PLC.
const wednesdayTxnDate = "2026/04/29"

func TestResolveCourseForRun_NoOverride_UsesPresetCourse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	p := preset.Preset{UserName: "u", Course: sql.NullString{String: "BRC", Valid: true}}
	course, clear := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "BRC", course)
	assert.False(t, clear)
}

func TestResolveCourseForRun_NoOverride_NoPresetCourse_FallsBackToDate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := preset.NewMockService(ctrl)

	p := preset.Preset{UserName: "u"}
	course, clear := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "PLC", course)
	assert.False(t, clear)
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
	course, clear := resolveCourseForRun(svc, p, wednesdayTxnDate, time.Now())
	assert.Equal(t, "PLC", course)
	assert.True(t, clear, "next-run-only override must be cleared after the run")
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
	course, clear := resolveCourseForRun(svc, p, wednesdayTxnDate, now)
	assert.Equal(t, "PLC", course)
	assert.False(t, clear, "until-bounded override is cleared lazily, not after each run")
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
	course, clear := resolveCourseForRun(svc, p, wednesdayTxnDate, now)
	assert.Equal(t, "BRC", course)
	assert.False(t, clear)
}
