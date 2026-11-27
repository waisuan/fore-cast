package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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

// errRunCancelled is returned by processPreset when the user cancels from the app.
// It is not a failure for logging purposes but must not be counted as a booking success.
var errRunCancelled = errors.New("run cancelled by user")

func presetBookingOpen(p preset.Preset) string {
	if strings.TrimSpace(p.BookingOpen) == "" {
		return preset.DefaultBookingOpen
	}
	return p.BookingOpen
}

func sleepUntilBookingOpen(cfg *deps.Config, bookingOpen string) {
	h, mi, sec, err := slotutil.ParseClockHM(bookingOpen)
	if err != nil {
		logger.Warn("invalid booking_open, using default",
			logger.String("booking_open", bookingOpen), logger.Err(err))
		h, mi, sec, _ = slotutil.ParseClockHM(preset.DefaultBookingOpen)
	}
	minH := cfg.SchedulerBookingWaitMinHourMy
	if h < 0 || h > 23 || mi < 0 || mi > 59 || sec < 0 || sec > 59 || minH < 0 || minH > 23 {
		logger.Warn("invalid scheduler booking wait times, skipping wait",
			logger.Int("hour", h), logger.Int("minute", mi), logger.Int("second", sec), logger.Int("min_hour", minH))
		return
	}
	tz := cfg.SchedulerTimezone
	if tz == "" {
		tz = "Asia/Kuala_Lumpur"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		logger.Warn("skipping booking wait (timezone)", logger.String("zone", tz), logger.Err(err))
		return
	}
	now := time.Now().In(loc)
	if now.Hour() < minH {
		return
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), h, mi, sec, 0, loc)
	if !now.Before(target) {
		return
	}
	d := target.Sub(now)
	logger.Info("waiting until local time before booking attempts",
		logger.String("timezone", tz),
		logger.String("until_local", target.Format("2006-01-02 15:04:05 MST")),
		logger.Duration("sleep", d.Round(time.Second)))
	time.Sleep(d)
}

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
	var succeeded, failed, cancelled atomic.Int32

	for _, p := range presets {
		wg.Add(1)
		go func(p preset.Preset) {
			defer wg.Done()
			if !d.Config.BookerDryRun {
				sleepUntilBookingOpen(d.Config, presetBookingOpen(p))
			}
			sem <- struct{}{}
			defer func() { <-sem }()

			logger.Info("processing preset",
				logger.String("user", p.UserName),
				logger.String("course", p.Course.String),
				logger.String("cutoff", p.Cutoff),
				logger.String("booking_open", p.BookingOpen),
				logger.String("retry_interval", p.RetryInterval),
				logger.String("timeout", p.Timeout))
			switch err := processPreset(d, p); {
			case errors.Is(err, errRunCancelled):
				cancelled.Add(1)
				logger.Info("preset run cancelled", logger.String("user", p.UserName))
			case err != nil:
				failed.Add(1)
				logger.Error("preset failed", logger.String("user", p.UserName), logger.Err(err))
			default:
				succeeded.Add(1)
			}
		}(p)
	}

	wg.Wait()
	logger.Info("finished",
		logger.Int("total", len(presets)),
		logger.Int("succeeded", int(succeeded.Load())),
		logger.Int("failed", int(failed.Load())),
		logger.Int("cancelled", int(cancelled.Load())),
		logger.Duration("took", time.Since(start).Round(time.Millisecond)))
	return nil
}

func processPreset(d *deps.Dependencies, p preset.Preset) error {
	// Consume skip before marking running so last_run/history stay clean; the UPDATE is conditional (no double-skip on crash).
	if skip, err := d.Preset.ConsumeSkipNextRun(p.UserName); err != nil {
		logger.Warn("failed to consume skip flag", logger.String("user", p.UserName), logger.Err(err))
	} else if skip {
		logger.Info("skipping run by user request", logger.String("user", p.UserName))
		if err := d.Preset.ClearCancelRequested(p.UserName); err != nil {
			logger.Warn("failed to clear cancel requested", logger.String("user", p.UserName), logger.Err(err))
		}
		return nil
	}

	if err := d.Preset.UpdatePresetRunStatus(p.UserName, preset.RunStatusRunning, ""); err != nil {
		logger.Warn("failed to set running status", logger.String("user", p.UserName), logger.Err(err))
	}
	defer func() { _ = d.Preset.ClearCancelRequested(p.UserName) }()

	txnDate := txnDateFromConfig(d.Config)
	primaryCourse, clearOverrideAfterRun, overrideActive := resolveCourseForRun(d.Preset, p, txnDate, time.Now())
	if clearOverrideAfterRun {
		defer func() {
			if err := d.Preset.ClearCourseOverride(p.UserName); err != nil {
				logger.Warn("failed to clear course override", logger.String("user", p.UserName), logger.Err(err))
			}
		}()
	}
	courses := resolveCoursesForRun(primaryCourse, txnDate, p.AltCourseDays, overrideActive)

	cred, err := d.Credentials.Get(p.UserName)
	if err != nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "credentials: "+err.Error())
		return fmt.Errorf("credentials: %w", err)
	}
	if cred == nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "credentials not found")
		return fmt.Errorf("credentials not found for %s", p.UserName)
	}
	password, err := crypto.Decrypt(cred.PasswordEnc, d.Config.EncryptionKey)
	if err != nil {
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "decrypt password: "+err.Error())
		return fmt.Errorf("decrypt password: %w", err)
	}

	token, err := d.Booker.Login(p.UserName, password)
	if err != nil {
		logAttempt(d.History, p, txnDate, runner.Result{Status: runner.StatusFailed, Message: "login: " + err.Error()})
		notifyUser(d.Notify, p, "FAILED: login: "+err.Error())
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, "login: "+err.Error())
		return fmt.Errorf("login: %w", err)
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
	if timeout < preset.MinTimeoutDuration {
		// Presets saved before the minimum existed (e.g. "3s") could never send a book.
		logger.Warn("timeout below minimum, using minimum",
			logger.String("user", p.UserName),
			logger.Duration("timeout", timeout),
			logger.Duration("min", preset.MinTimeoutDuration))
		timeout = preset.MinTimeoutDuration
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

	var loginMu sync.Mutex
	baseCfg := runner.Config{
		UserName:      p.UserName,
		Token:         token,
		TxnDate:       txnDate,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: retryInterval,
		Waits:         runner.DefaultWaits,
		RefreshToken: func(ctx context.Context) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			loginMu.Lock()
			defer loginMu.Unlock()
			tok, err := d.Booker.Login(p.UserName, password)
			if err != nil {
				return "", err
			}
			return tok, nil
		},
	}

	runCtx, cancelRun := context.WithTimeout(context.Background(), timeout)
	defer cancelRun()

	stopPoll := make(chan struct{})
	defer close(stopPoll)
	pollEvery := d.Config.SchedulerCancelPollInterval
	if pollEvery <= 0 {
		pollEvery = 2 * time.Second
	}
	go func() {
		ticker := time.NewTicker(pollEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stopPoll:
				return
			case <-runCtx.Done():
				return
			case <-ticker.C:
				cur, e := d.Preset.GetPreset(p.UserName)
				if e != nil || cur == nil {
					continue
				}
				if cur.CancelRequested {
					cancelRun()
					return
				}
			}
		}
	}()

	logger.Info("starting run",
		logger.String("user", p.UserName),
		logger.String("courses", strings.Join(courses, ",")),
		logger.String("txn_date", txnDate))

	// Each course gets an independent worker that runs against runCtx (so
	// user-cancel and timeout still apply). We wait for ALL of them; the API
	// permits one booking per course per day, so successes are independent
	// and the user can cancel any extras from history.
	resultsCh := make(chan runner.Result, len(courses))
	for _, c := range courses {
		go func() {
			// A panic must not kill the other presets' runs or leave this one stuck "running".
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("course worker panicked", logger.String("user", p.UserName), logger.String("course", c))
					resultsCh <- runner.Result{Status: runner.StatusFailed, CourseID: c, Message: fmt.Sprintf("[%s] internal error: %v", c, rec)}
				}
			}()
			cfg := baseCfg
			cfg.CourseID = c
			// runner.Run always populates Result.Status + Message even on
			// error (via resultWithCourse), so we don't need the err here.
			r, _ := runner.Run(runCtx, cfg, d.Booker)
			resultsCh <- r
		}()
	}

	results := make([]runner.Result, 0, len(courses))
	for range courses {
		r := <-resultsCh
		results = append(results, r)
		logAttempt(d.History, p, txnDate, r)
		logger.Info("course attempt finished",
			logger.String("user", p.UserName),
			logger.String("course", r.CourseID),
			logger.String("status", string(r.Status)))
	}

	msg, booked := outcomeMessage(results)
	switch {
	case booked > 0:
		logger.Info("run finished",
			logger.String("user", p.UserName),
			logger.Int("bookings", booked),
			logger.String("message", msg),
			logger.String("txn_date", txnDate))
		notifyUser(d.Notify, p, msg)
		updateRunDone(d.Preset, p.UserName, preset.RunStatusSuccess, msg)
		return nil
	case errors.Is(runCtx.Err(), context.Canceled):
		// Timeouts surface as DeadlineExceeded and fall through to failure.
		notifyUser(d.Notify, p, "CANCELLED: "+msg)
		updateRunDone(d.Preset, p.UserName, preset.RunStatusCancelled, msg)
		return errRunCancelled
	default:
		notifyUser(d.Notify, p, "FAILED: "+msg+". See History for the full run log.")
		updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, msg)
		return errors.New(msg)
	}
}

// outcomeMessage renders the user-facing summary of a run's per-course results,
// ordered by CourseID so parallel workers always produce the same string.
// Several successes are real bookings (one per course per day), so the user
// gets a cancel hint; non-successes alongside a success go in "(also: ...)".
func outcomeMessage(results []runner.Result) (msg string, booked int) {
	sorted := append([]runner.Result(nil), results...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CourseID < sorted[j].CourseID })
	var ok, other []string
	for _, r := range sorted {
		switch {
		case r.Status == runner.StatusSuccess:
			ok = append(ok, r.Message)
		case r.Message != "":
			other = append(other, r.Message)
		}
	}
	switch len(ok) {
	case 0:
		if len(other) == 0 {
			return "no slot booked", 0
		}
		return strings.Join(other, "; "), 0
	case 1:
		msg = ok[0]
	default:
		msg = fmt.Sprintf("%d bookings made — cancel any you don't want: %s", len(ok), strings.Join(ok, "; "))
	}
	if len(other) > 0 {
		msg += " (also: " + strings.Join(other, "; ") + ")"
	}
	return msg, len(ok)
}

// resolveCoursesForRun returns the ordered list of distinct courses to try in
// parallel for a single scheduler fire. Always includes the primary; appends
// the OPPOSITE course (BRC <-> PLC) only when:
//   - no override is active (override means the user explicitly chose today's course),
//   - altCourseDays is set,
//   - today's weekday (parsed from txnDate) is in altCourseDays, and
//   - the opposite course is well-defined and differs from primary.
//
// On any parse / weekday-lookup failure we conservatively fall back to
// "primary only".
func resolveCoursesForRun(primary, txnDate string, altCourseDays sql.NullString, overrideActive bool) []string {
	primary = slotutil.NormalizeCourseCode(primary)
	courses := []string{primary}
	if overrideActive || !altCourseDays.Valid || altCourseDays.String == "" {
		return courses
	}
	days, err := slotutil.ParseWeekdayCodes(altCourseDays.String)
	if err != nil || len(days) == 0 {
		return courses
	}
	t, err := time.Parse("2006/01/02", txnDate)
	if err != nil {
		return courses
	}
	if _, ok := days[t.Weekday()]; !ok {
		return courses
	}
	// OtherCourse returns "" when primary is not a known club course; in that
	// case there's no well-defined alt and we fall back to primary-only.
	alt := slotutil.OtherCourse(primary)
	if alt == "" {
		return courses
	}
	return append(courses, alt)
}

// resolveCourseForRun applies the temporary override (if any) and returns the
// course to book against, whether the override should be cleared once the run
// completes, and whether the returned course came from an active override.
// Expired overrides are cleared immediately and the default course is used.
// The overrideActive flag is used downstream to suppress the alt-course fan-out
// (an active override means the user explicitly chose this course for today).
func resolveCourseForRun(svc preset.Service, p preset.Preset, txnDate string, now time.Time) (course string, clearAfterRun bool, overrideActive bool) {
	state, override := preset.ResolveOverride(p, now)
	switch state {
	case preset.OverrideExpired:
		if err := svc.ClearCourseOverride(p.UserName); err != nil {
			logger.Warn("failed to clear expired course override", logger.String("user", p.UserName), logger.Err(err))
		}
	case preset.OverrideActive:
		return slotutil.NormalizeCourseCode(override), false, true
	case preset.OverrideOnce:
		return slotutil.NormalizeCourseCode(override), true, true
	}
	courseID := slotutil.NormalizeCourseCode(p.Course.String)
	if courseID == "" {
		courseID = slotutil.CourseForDate(txnDate)
	}
	return courseID, false, false
}

func txnDateFromConfig(cfg *deps.Config) string {
	if cfg.SchedulerTxnDate != "" {
		if err := slotutil.ValidateDate(cfg.SchedulerTxnDate); err == nil {
			return cfg.SchedulerTxnDate
		}
		logger.Warn("invalid SCHEDULER_TXN_DATE, using 1 week ahead",
			logger.String("value", cfg.SchedulerTxnDate),
			logger.String("expected", "YYYY/MM/DD"))
	}
	return slotutil.DateOneWeekAhead()
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
	if len(result.Steps) > 0 {
		attempt.Steps, _ = json.Marshal(result.Steps) // []Step of strings and times cannot fail
	}
	if err := svc.LogAttempt(attempt); err != nil {
		logger.Error("failed to log attempt", logger.String("user", p.UserName), logger.Err(err))
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
