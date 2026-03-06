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
