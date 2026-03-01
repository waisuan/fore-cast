package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/history"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/preset"
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

	presets, err := d.Preset.GetEnabledPresets()
	if err != nil {
		return fmt.Errorf("get presets: %w", err)
	}
	if len(presets) == 0 {
		log.Println("no enabled presets found, nothing to do")
		return nil
	}

	client := booker.NewClient()
	sem := make(chan struct{}, d.Config.MaxConcurrentPresets)
	var wg sync.WaitGroup

	for _, p := range presets {
		wg.Add(1)
		go func(p preset.Preset) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			log.Printf("processing preset for user %s", p.UserName)
			if err := processPreset(client, d, p); err != nil {
				log.Printf("user %s: %v", p.UserName, err)
			}
		}(p)
	}

	wg.Wait()
	return nil
}

func processPreset(client *booker.Client, d *deps.Dependencies, p preset.Preset) error {
	if err := d.Preset.UpdatePresetRunStatus(p.UserName, preset.RunStatusRunning, ""); err != nil {
		log.Printf("user %s: failed to set running status: %v", p.UserName, err)
	}

	password, err := crypto.Decrypt(p.PasswordEnc, d.Config.EncryptionKey)
	if err != nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "decrypt password: "+err.Error())
		return fmt.Errorf("decrypt password: %w", err)
	}

	token, err := client.Login(p.UserName, password)
	if err != nil {
		logAttempt(d.History, p, runner.Result{Status: runner.StatusFailed, Message: "login: " + err.Error()})
		notifyUser(p, "FAILED: login: "+err.Error())
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "login: "+err.Error())
		return fmt.Errorf("login: %w", err)
	}

	txnDate := slotutil.DateOneWeekAhead()
	courseID := strings.TrimSpace(strings.ToUpper(p.Course.String))
	if courseID == "" {
		courseID = slotutil.CourseForDate(txnDate)
	}

	cutoffTeeTime, err := slotutil.ParseCutoff(p.Cutoff)
	if err != nil {
		return fmt.Errorf("parse cutoff: %w", err)
	}

	timeout, err := time.ParseDuration(p.Timeout)
	if err != nil {
		timeout = 10 * time.Minute
	}

	cfg := runner.Config{
		UserName:      p.UserName,
		Token:         token,
		TxnDate:       txnDate,
		CourseID:      courseID,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: p.RetryInterval,
		Retry:         true,
		Debug:         false,
		Timeout:       timeout,
	}

	result, err := runner.Run(cfg, client)
	logAttempt(d.History, p, result)

	if err != nil {
		notifyUser(p, "FAILED: "+err.Error())
		updateRunDone(d.Preset, p.UserName, runStatusFromResult(result.Status), err.Error())
		return err
	}
	if result.Message != "" {
		notifyUser(p, result.Message)
	}
	updateRunDone(d.Preset, p.UserName, runStatusFromResult(result.Status), result.Message)
	return nil
}

func logAttempt(svc history.Service, p preset.Preset, result runner.Result) {
	attempt := history.Attempt{
		UserName:  p.UserName,
		CourseID:  result.CourseID,
		TxnDate:   slotutil.DateOneWeekAhead(),
		TeeTime:   sql.NullString{String: result.TeeTime, Valid: result.TeeTime != ""},
		TeeBox:    sql.NullString{String: result.TeeBox, Valid: result.TeeBox != ""},
		BookingID: sql.NullString{String: result.BookingID, Valid: result.BookingID != ""},
		Status:    string(result.Status),
		Message:   result.Message,
	}
	if err := svc.LogAttempt(attempt); err != nil {
		log.Printf("failed to log attempt for %s: %v", p.UserName, err)
	}
}

func runStatusFromResult(s runner.Status) preset.RunStatus {
	switch s {
	case runner.StatusSuccess:
		return preset.RunStatusSuccess
	default:
		return preset.RunStatusFailed
	}
}

func updateRunDone(svc preset.Service, userName string, status preset.RunStatus, message string) {
	if err := svc.UpdatePresetRunStatus(userName, status, message); err != nil {
		log.Printf("user %s: failed to update run status: %v", userName, err)
	}
}

func notifyUser(p preset.Preset, msg string) {
	topic := p.NtfyTopic.String
	if !p.NtfyTopic.Valid || topic == "" {
		return
	}
	if err := notify.Send(topic, msg); err != nil {
		log.Printf("ntfy error for %s: %v", p.UserName, err)
	} else {
		log.Printf("ntfy: notified topic %s for user %s", topic, p.UserName)
	}
}
