package context

import "context"

type contextKey string

const userKey contextKey = "user"

// Role values in user_credentials.role.
const (
	RoleAdmin    = "ADMIN"
	RoleNonAdmin = "NON_ADMIN"
)

// User holds the authenticated user info. UserName comes from the session.
// Role is loaded from user_credentials on each request. APIToken is set by
// TokenRefresh when a handler needs 3rd party access.
type User struct {
	UserName string
	Role     string
	APIToken string
}

// IsAdmin reports whether the user has the ADMIN role.
func (u *User) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}

// WithUser returns a context with the user attached.
func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFrom returns the user from the context, or nil if not set.
func UserFrom(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}
