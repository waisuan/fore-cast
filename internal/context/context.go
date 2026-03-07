package context

import "context"

type contextKey string

const userKey contextKey = "user"

// User holds the authenticated user info. UserName comes from the session.
// APIToken is set by TokenRefresh when a handler needs 3rd party access.
type User struct {
	UserName string
	APIToken string
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
