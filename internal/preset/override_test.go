package preset

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveOverride(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		preset    Preset
		wantState OverrideState
		wantCrs   string
	}{
		{
			name:      "no override",
			preset:    Preset{},
			wantState: OverrideNone,
			wantCrs:   "",
		},
		{
			name: "once when until is null",
			preset: Preset{
				OverrideCourse: sql.NullString{String: "PLC", Valid: true},
			},
			wantState: OverrideOnce,
			wantCrs:   "PLC",
		},
		{
			name: "active when until is in the future",
			preset: Preset{
				OverrideCourse: sql.NullString{String: "BRC", Valid: true},
				OverrideUntil:  sql.NullTime{Time: now.Add(time.Hour), Valid: true},
			},
			wantState: OverrideActive,
			wantCrs:   "BRC",
		},
		{
			name: "expired when until has passed",
			preset: Preset{
				OverrideCourse: sql.NullString{String: "BRC", Valid: true},
				OverrideUntil:  sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
			},
			wantState: OverrideExpired,
			wantCrs:   "",
		},
		{
			name: "empty course string treated as no override",
			preset: Preset{
				OverrideCourse: sql.NullString{String: "", Valid: true},
			},
			wantState: OverrideNone,
			wantCrs:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, course := ResolveOverride(tc.preset, now)
			assert.Equal(t, tc.wantState, state)
			assert.Equal(t, tc.wantCrs, course)
		})
	}
}
