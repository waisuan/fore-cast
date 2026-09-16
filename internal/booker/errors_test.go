package booker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserFacingReason(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, reason, fallback, want string
	}{
		{
			name:   "window still closed",
			reason: "Flight time will be open after 10pm",
			want:   "Not open yet — the club opens this date after 10:00 PM. Wait, then try once.",
		},
		{
			name:   "time range",
			reason: "Booking is not allowed during this time range.",
			want:   "Booking isn't allowed at this hour. Wait until after 10:00 PM, then try once.",
		},
		{
			name:   "rapid attempts",
			reason: "Rapid attempts detected. Please wait a moment before trying again.",
			want:   "The club locked this account for too many rapid attempts. Wait a minute before trying again.",
		},
		{
			name:   "account locked",
			reason: "Your account is temporarily locked for golf booking due to multiple rapid attempts. Please contact the Management.",
			want:   "The club locked this account for too many rapid attempts. Wait a minute before trying again.",
		},
		{
			name:   "already reserved",
			reason: "The flight has already been reserved",
			want:   "That flight is already reserved. Pick another slot.",
		},
		{
			name:   "not reserve",
			reason: "Tee Time was not reserve",
			want:   "The club failed to reserve that tee time. Try another slot.",
		},
		{
			name:   "20017",
			reason: "ErrNumber 20017: please select your flight again",
			want:   "The club dropped the selected flight. Tap Book once more on that slot.",
		},
		{
			name:   "unknown club text is passed through",
			reason: "slot taken",
			want:   "slot taken",
		},
		{
			name:     "empty uses fallback",
			reason:   "  ",
			fallback: "Slot is no longer available.",
			want:     "Slot is no longer available.",
		},
		{
			name: "empty no fallback",
			want: "Booking failed.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, UserFacingReason(tc.reason, tc.fallback))
		})
	}
}
