package main

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/history"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/preset"
	"github.com/waisuan/alfred/internal/runner"
	"github.com/waisuan/alfred/internal/slotutil"
	"github.com/waisuan/alfred/migrations"
)

func main() {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		logger.Fatal("init deps", logger.Err(err))
	}
	defer d.Shutdown()

	if err := run(d); err != nil {
		logger.Fatal("scheduler", logger.Err(err))
	}
}

func run(d *deps.Dependencies) error {
	if d.Config.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required")
	}

	presets, err := d.Preset.GetEnabledPresets()
	if err != nil {
		return fmt.Errorf("get presets: %w", err)
	}
	if len(presets) == 0 {
		logger.Info("no enabled presets found, nothing to do")
		return nil
	}

	if d.Config.BookerDryRun {
		logger.Info("dry-run: booker api mocked", logger.String("scenario", d.Config.BookerDryRunScenario))
	}
	logger.Info("found enabled presets", logger.Int("count", len(presets)), logger.Int("concurrency", d.Config.MaxConcurrentPresets))
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

			logger.Info("processing preset",
				logger.String("user", p.UserName),
				logger.String("course", p.Course.String),
				logger.String("cutoff", p.Cutoff),
				logger.String("retry", p.RetryInterval),
				logger.String("timeout", p.Timeout))
			if err := processPreset(d, p); err != nil {
				logger.Error("preset failed", logger.String("user", p.UserName), logger.Err(err))
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
	logger.Info("finished",
		logger.Int("total", len(presets)),
		logger.Int("succeeded", succeeded),
		logger.Int("failed", failed),
		logger.Duration("took", time.Since(start).Round(time.Millisecond)))
	return nil
}

func processPreset(d *deps.Dependencies, p preset.Preset) error {
	if err := d.Preset.UpdatePresetRunStatus(p.UserName, preset.RunStatusRunning, ""); err != nil {
		logger.Warn("failed to set running status", logger.String("user", p.UserName), logger.Err(err))
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
		logger.Warn("invalid timeout, falling back to 10m", logger.String("user", p.UserName), logger.String("timeout", p.Timeout))
		timeout = 10 * time.Minute
	}
	if d.Config.BookerDryRun && d.Config.BookerDryRunTimeout > 0 && timeout > d.Config.BookerDryRunTimeout {
		timeout = d.Config.BookerDryRunTimeout
	}

	retryInterval, err := time.ParseDuration(p.RetryInterval)
	if err != nil {
		logger.Warn("invalid retry_interval, falling back to 1s", logger.String("user", p.UserName), logger.String("retry_interval", p.RetryInterval))
		retryInterval = time.Second
	}
	if retryInterval < preset.MinRetryIntervalDuration {
		logger.Warn("retry_interval below minimum, using minimum",
			logger.String("user", p.UserName),
			logger.Duration("retry_interval", retryInterval),
			logger.Duration("min", preset.MinRetryIntervalDuration))
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

	logger.Info("starting run", logger.String("user", p.UserName), logger.String("course", courseID), logger.String("txn_date", txnDate))
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
		logger.Error("failed to log attempt", logger.String("user", p.UserName), logger.Err(err))
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
		logger.Error("failed to update run status", logger.String("user", userName), logger.Err(err))
	}
}

func notifyUser(svc notify.Service, p preset.Preset, msg string) {
	topic := p.NtfyTopic.String
	if !p.NtfyTopic.Valid || topic == "" {
		return
	}
	if err := svc.Send(topic, msg); err != nil {
		logger.Error("failed to send ntfy notification", logger.String("user", p.UserName), logger.Err(err))
	} else {
		logger.Info("ntfy notification sent", logger.String("topic", topic), logger.String("user", p.UserName))
	}
}
