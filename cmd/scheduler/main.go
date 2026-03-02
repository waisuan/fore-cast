package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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

	if d.Config.BookerDryRun {
		log.Printf("DRY-RUN: Booker API mocked (scenario=%s)", d.Config.BookerDryRunScenario)
	}
	log.Printf("found %d enabled preset(s), concurrency limit %d", len(presets), d.Config.MaxConcurrentPresets)
	start := time.Now()

	sem := make(chan struct{}, d.Config.MaxConcurrentPresets)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var succeeded, failed int

	for _, p := range presets {
		wg.Add(1)
		go func(p preset.Preset) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			log.Printf("processing preset for user %s (course=%s cutoff=%s retry=%s timeout=%s)",
				p.UserName, p.Course.String, p.Cutoff, p.RetryInterval, p.Timeout)
			if err := processPreset(d, p); err != nil {
				log.Printf("user %s: %v", p.UserName, err)
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()
	log.Printf("finished: %d preset(s), %d succeeded, %d failed, took %s",
		len(presets), succeeded, failed, time.Since(start).Round(time.Millisecond))
	return nil
}

func processPreset(d *deps.Dependencies, p preset.Preset) error {
	if err := d.Preset.UpdatePresetRunStatus(p.UserName, preset.RunStatusRunning, ""); err != nil {
		log.Printf("user %s: failed to set running status: %v", p.UserName, err)
	}

	password, err := crypto.Decrypt(p.PasswordEnc, d.Config.EncryptionKey)
	if err != nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "decrypt password: "+err.Error())
		return fmt.Errorf("decrypt password: %w", err)
	}

	token, err := d.Booker.Login(p.UserName, password)
	txnDate := slotutil.DateOneWeekAhead()
	if err != nil {
		logAttempt(d.History, p, txnDate, runner.Result{Status: runner.StatusFailed, Message: "login: " + err.Error()})
		notifyUser(d.Notify, p, "FAILED: login: "+err.Error())
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "login: "+err.Error())
		return fmt.Errorf("login: %w", err)
	}
	courseID := strings.TrimSpace(strings.ToUpper(p.Course.String))
	if courseID == "" {
		courseID = slotutil.CourseForDate(txnDate)
	}

	cutoffTeeTime, err := slotutil.ParseCutoff(p.Cutoff)
	if err != nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "parse cutoff: "+err.Error())
		return fmt.Errorf("parse cutoff: %w", err)
	}

	timeout, err := time.ParseDuration(p.Timeout)
	if err != nil {
		log.Printf("user %s: invalid timeout %q, falling back to 10m", p.UserName, p.Timeout)
		timeout = 10 * time.Minute
	}
	if d.Config.BookerDryRun && d.Config.BookerDryRunTimeout > 0 && timeout > d.Config.BookerDryRunTimeout {
		timeout = d.Config.BookerDryRunTimeout
	}

	retryInterval, err := time.ParseDuration(p.RetryInterval)
	if err != nil {
		log.Printf("user %s: invalid retry_interval %q, falling back to 1s", p.UserName, p.RetryInterval)
		retryInterval = time.Second
	}
	if retryInterval < preset.MinRetryIntervalDuration {
		log.Printf("user %s: retry_interval %s below minimum %s, using minimum", p.UserName, retryInterval, preset.MinRetryIntervalDuration)
		retryInterval = preset.MinRetryIntervalDuration
	}

	cfg := runner.Config{
		UserName:      p.UserName,
		Token:         token,
		TxnDate:       txnDate,
		CourseID:      courseID,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: retryInterval,
		Retry:         true,
		Debug:         false,
		Timeout:       timeout,
	}

	result, err := runner.Run(cfg, d.Booker)
	logAttempt(d.History, p, txnDate, result)

	if err != nil {
		notifyUser(d.Notify, p, "FAILED: "+err.Error())
		updateRunDone(d.Preset, p.UserName, runStatusFromResult(result.Status), err.Error())
		return err
	}
	if result.Message != "" {
		notifyUser(d.Notify, p, result.Message)
	}
	updateRunDone(d.Preset, p.UserName, runStatusFromResult(result.Status), result.Message)
	return nil
}

func logAttempt(svc history.Service, p preset.Preset, txnDate string, result runner.Result) {
	attempt := history.Attempt{
		UserName:  p.UserName,
		CourseID:  result.CourseID,
		TxnDate:   txnDate,
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

func notifyUser(svc notify.Service, p preset.Preset, msg string) {
	topic := p.NtfyTopic.String
	if !p.NtfyTopic.Valid || topic == "" {
		return
	}
	if err := svc.Send(topic, msg); err != nil {
		log.Printf("ntfy error for %s: %v", p.UserName, err)
	} else {
		log.Printf("ntfy: notified topic %s for user %s", topic, p.UserName)
	}
}
