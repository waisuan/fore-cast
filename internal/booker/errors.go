package booker

import (
	"errors"
	"strings"
)

// CodeInvalidToken is returned by the club API when the session token has expired
// or is no longer valid. The user must re-login to obtain a fresh token.
const CodeInvalidToken = "CODE103"

// ErrInvalidToken is returned when the club API indicates the token is invalid (e.g. CODE103).
// Handlers should respond with 401 to force re-login.
var ErrInvalidToken = errors.New("club API token invalid")

// IsInvalidToken returns true if reason indicates the club API token is invalid.
func IsInvalidToken(reason string) bool {
	return strings.Contains(reason, CodeInvalidToken)
}

// UserFacingReason maps a club API Reason to a short message the UI can show.
// Known phrases get an actionable sentence; anything else is passed through.
// fallback is used when reason is empty.
func UserFacingReason(reason, fallback string) string {
	r := strings.TrimSpace(reason)
	switch {
	case containsFold(r, "will be open after"):
		return "Not open yet — the club opens this date after 10:00 PM. Wait, then try once."
	case containsFold(r, "not allowed during this time range"):
		return "Booking isn't allowed at this hour. Wait until after 10:00 PM, then try once."
	case containsFold(r, "rapid attempts", "temporarily locked"):
		return "The club locked this account for too many rapid attempts. Wait a minute before trying again."
	case containsFold(r, "already been reserved"):
		return "That flight is already reserved. Pick another slot."
	case containsFold(r, "was not reserve"):
		return "The club failed to reserve that tee time. Try another slot."
	case containsFold(r, "20017", "select your flight again"):
		return "The club dropped the selected flight. Tap Book once more on that slot."
	case r != "":
		return r
	}
	if fb := strings.TrimSpace(fallback); fb != "" {
		return fb
	}
	return "Booking failed."
}

func containsFold(s string, phrases ...string) bool {
	low := strings.ToLower(s)
	for _, p := range phrases {
		if strings.Contains(low, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
