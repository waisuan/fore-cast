// Command createuser seeds or updates a local test account in
// user_credentials. It exists because /api/v1/admin/register requires an
// existing admin session (chicken-and-egg on a fresh DB) and because passwords
// are AES-256-GCM encrypted — a plain SQL INSERT won't work.
//
// Example:
//
//	APP_ENV=development go run ./cmd/createuser -user alice -pass secret -role ADMIN
package main

import (
	"flag"
	"fmt"
	"os"

	appcontext "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/migrations"
)

func main() {
	user := flag.String("user", "", "username (required)")
	pass := flag.String("pass", "", "plaintext password (required)")
	role := flag.String("role", appcontext.RoleNonAdmin, "ADMIN or NON_ADMIN")
	flag.Parse()

	if *user == "" || *pass == "" {
		fmt.Fprintln(os.Stderr, "usage: createuser -user NAME -pass SECRET [-role ADMIN]")
		os.Exit(2)
	}
	if *role != appcontext.RoleAdmin && *role != appcontext.RoleNonAdmin {
		fmt.Fprintf(os.Stderr, "invalid role %q: want %s or %s\n", *role, appcontext.RoleAdmin, appcontext.RoleNonAdmin)
		os.Exit(2)
	}

	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		logger.Fatal("init deps", logger.Err(err))
	}
	defer d.Shutdown()

	enc, err := crypto.Encrypt(*pass, d.Config.EncryptionKey)
	if err != nil {
		logger.Fatal("encrypt password", logger.Err(err))
	}
	if err := d.Credentials.Upsert(*user, enc, *role); err != nil {
		logger.Fatal("upsert credentials", logger.Err(err))
	}
	logger.Info("upserted user", logger.String("user", *user), logger.String("role", *role))
}
