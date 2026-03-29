package context

import (
	stdctx "context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithUser_UserFrom_RoundTrip(t *testing.T) {
	t.Parallel()
	u := &User{UserName: "alice", Role: RoleNonAdmin, APIToken: "tok"}
	ctx := WithUser(stdctx.Background(), u)
	got := UserFrom(ctx)
	assert.Same(t, u, got)
}

func TestUserFrom_NotSet(t *testing.T) {
	t.Parallel()
	assert.Nil(t, UserFrom(stdctx.Background()))
}

func TestUserFrom_WrongTypeInContext(t *testing.T) {
	t.Parallel()
	ctx := stdctx.WithValue(stdctx.Background(), userKey, "not-a-user")
	assert.Nil(t, UserFrom(ctx))
}

func TestUser_IsAdmin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		u    *User
		want bool
	}{
		{name: "nil receiver", u: nil, want: false},
		{name: "admin role", u: &User{UserName: "a", Role: RoleAdmin}, want: true},
		{name: "non-admin role", u: &User{UserName: "b", Role: RoleNonAdmin}, want: false},
		{name: "empty role", u: &User{UserName: "c", Role: ""}, want: false},
		{name: "unknown role string", u: &User{UserName: "d", Role: "GUEST"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.u.IsAdmin())
		})
	}
}
