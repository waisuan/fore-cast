package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/db"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/runner"
	"github.com/waisuan/alfred/internal/slotutil"
	"github.com/waisuan/alfred/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("scheduler: %v", err)
	}
}

func run() error {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		return fmt.Errorf("init deps: %w", err)
	}
	defer d.Shutdown()

	if d.Config.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required")
	}

	presets, err := d.Service.GetEnabledPresets()
	if err != nil {
		return fmt.Errorf("get presets: %w", err)
	}
	if len(presets) == 0 {
		log.Println("no enabled presets found, nothing to do")
		return nil
	}

	client := booker.NewClient()

	for _, preset := range presets {
		log.Printf("processing preset for user %s", preset.UserName)
		if err := processPreset(client, d.Service, preset, d.Config.EncryptionKey); err != nil {
			log.Printf("user %s: %v", preset.UserName, err)
		}
	}

	return nil
}

func processPreset(client *booker.Client, svc db.ServiceInterface, preset db.Preset, encryptionKey string) error {
	password, err := crypto.Decrypt(preset.PasswordEnc, encryptionKey)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	token, err := client.Login(preset.UserName, password)
	if err != nil {
		logAttempt(svc, preset, runner.Result{Status: runner.StatusFailed, Message: "login: " + err.Error()})
		notifyUser(preset, "FAILED: login: "+err.Error())
		return fmt.Errorf("login: %w", err)
	}

	txnDate := slotutil.DateOneWeekAhead()
	courseID := strings.TrimSpace(strings.ToUpper(preset.Course.String))
	if courseID == "" {
		courseID = slotutil.CourseForDate(txnDate)
	}

	cutoffTeeTime, err := slotutil.ParseCutoff(preset.Cutoff)
	if err != nil {
		return fmt.Errorf("parse cutoff: %w", err)
	}

	timeout, err := time.ParseDuration(preset.Timeout)
	if err != nil {
		timeout = 10 * time.Minute
	}

	cfg := runner.Config{
		UserName:      preset.UserName,
		Token:         token,
		TxnDate:       txnDate,
		CourseID:      courseID,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: preset.RetryInterval,
		Retry:         true,
		Debug:         false,
		Timeout:       timeout,
	}

	result, err := runner.Run(cfg, client)
	logAttempt(svc, preset, result)

	if err != nil {
		notifyUser(preset, "FAILED: "+err.Error())
		return err
	}
	if result.Message != "" {
		notifyUser(preset, result.Message)
	}
	return nil
}

func logAttempt(svc db.ServiceInterface, preset db.Preset, result runner.Result) {
	attempt := db.Attempt{
		UserName:  preset.UserName,
		CourseID:  result.CourseID,
		TxnDate:   slotutil.DateOneWeekAhead(),
		TeeTime:   sql.NullString{String: result.TeeTime, Valid: result.TeeTime != ""},
		TeeBox:    sql.NullString{String: result.TeeBox, Valid: result.TeeBox != ""},
		BookingID: sql.NullString{String: result.BookingID, Valid: result.BookingID != ""},
		Status:    string(result.Status),
		Message:   result.Message,
	}
	if err := svc.LogAttempt(attempt); err != nil {
		log.Printf("failed to log attempt for %s: %v", preset.UserName, err)
	}
}

func notifyUser(preset db.Preset, msg string) {
	topic := preset.NtfyTopic.String
	if !preset.NtfyTopic.Valid || topic == "" {
		return
	}
	if err := notify.Send(topic, msg); err != nil {
		log.Printf("ntfy error for %s: %v", preset.UserName, err)
	} else {
		log.Printf("ntfy: notified topic %s for user %s", topic, preset.UserName)
	}
}
