package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
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
	h, mi, err := slotutil.ParseClockHM(bookingOpen)
	if err != nil {
		h, mi = cfg.SchedulerBookingWaitHourMy, cfg.SchedulerBookingWaitMinuteMy
		if strings.TrimSpace(bookingOpen) != "" {
			logger.Warn("invalid booking_open, using env wait time",
				logger.String("booking_open", bookingOpen), logger.Err(err))
		}
	}
	minH := cfg.SchedulerBookingWaitMinHourMy
	if h < 0 || h > 23 || mi < 0 || mi > 59 || minH < 0 || minH > 23 {
		logger.Warn("invalid scheduler booking wait times, skipping wait",
			logger.Int("hour", h), logger.Int("minute", mi), logger.Int("min_hour", minH))
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
	target := time.Date(now.Year(), now.Month(), now.Day(), h, mi, 0, 0, loc)
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
	var mu sync.Mutex
	var succeeded, failed, cancelled int

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
			if err := processPreset(d, p); err != nil {
				if errors.Is(err, errRunCancelled) {
					mu.Lock()
					cancelled++
					mu.Unlock()
					logger.Info("preset run cancelled", logger.String("user", p.UserName))
				} else {
					logger.Error("preset failed", logger.String("user", p.UserName), logger.Err(err))
					mu.Lock()
					failed++
					mu.Unlock()
				}
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
		logger.Int("cancelled", cancelled),
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

	baseCfg := runner.Config{
		UserName:      p.UserName,
		Token:         token,
		TxnDate:       txnDate,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: retryInterval,
		Debug:         false,
		Timeout:       timeout,
	}

	var runCtx context.Context
	var cancelRun context.CancelFunc
	if timeout > 0 {
		runCtx, cancelRun = context.WithTimeout(context.Background(), timeout)
	} else {
		runCtx, cancelRun = context.WithCancel(context.Background())
	}
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
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// Send the synthetic failure result FIRST so the parent never
				// blocks waiting for our slot, even if subsequent logging or
				// formatting panics. The send is to a buffered channel sized
				// to len(courses), so it never blocks.
				panicMsg := fmt.Sprintf("[%s] panic: %v", c, rec)
				resultsCh <- runner.Result{
					Status:   runner.StatusFailed,
					Message:  panicMsg,
					CourseID: c,
				}
				logger.Error("course goroutine panicked",
					logger.String("user", p.UserName),
					logger.String("course", c),
					logger.String("panic", panicMsg))
			}()
			cfg := baseCfg
			cfg.CourseID = c
			// runner.Run always populates Result.Status + Message even on
			// error (via resultWithCourse), so we don't need the err here.
			r, _ := runner.Run(runCtx, cfg, d.Booker)
			resultsCh <- r
		}()
	}

	allResults := make([]runner.Result, 0, len(courses))
	var successes []runner.Result
	for i := 0; i < len(courses); i++ {
		r := <-resultsCh
		allResults = append(allResults, r)
		logAttempt(d.History, p, txnDate, r)
		// Per-attempt log is intentionally compact; the full message + booking
		// metadata is logged once in the run summary below.
		logger.Info("course attempt finished",
			logger.String("user", p.UserName),
			logger.String("course", r.CourseID),
			logger.String("status", string(r.Status)))
		if r.Status == runner.StatusSuccess {
			successes = append(successes, r)
		}
	}

	if len(successes) > 0 {
		msg := buildOutcomeMessage(successes, allResults)
		logRunFinished(p, txnDate, successes, msg)
		if msg != "" {
			notifyUser(d.Notify, p, msg)
		}
		updateRunDone(d.Preset, p.UserName, preset.RunStatusSuccess, msg)
		return nil
	}

	// No bookings. runCtx.Canceled means the user cancelled (timeouts surface
	// as DeadlineExceeded and fall through to the failure branch).
	aggregatedMsg := aggregateFailureMessages(allResults)
	if errors.Is(runCtx.Err(), context.Canceled) {
		notifyUser(d.Notify, p, "CANCELLED: "+aggregatedMsg)
		updateRunDone(d.Preset, p.UserName, preset.RunStatusCancelled, aggregatedMsg)
		return errRunCancelled
	}

	notifyUser(d.Notify, p, "FAILED: "+aggregatedMsg)
	updateRunDone(d.Preset, p.UserName, preset.RunStatusFailed, aggregatedMsg)
	return errors.New(aggregatedMsg)
}

// logRunFinished emits the run summary. With one success we include the
// structured booking fields the single-course path used to log; with multiple
// successes we just report counts + aggregated message.
func logRunFinished(p preset.Preset, txnDate string, successes []runner.Result, msg string) {
	if len(successes) == 1 {
		s := successes[0]
		logger.Info("run finished",
			logger.String("user", p.UserName),
			logger.String("status", string(s.Status)),
			logger.Int("bookings", 1),
			logger.String("message", s.Message),
			logger.String("booking_id", s.BookingID),
			logger.String("tee_time", s.TeeTime),
			logger.String("tee_box", s.TeeBox),
			logger.String("course_id", s.CourseID),
			logger.String("txn_date", txnDate))
		return
	}
	logger.Info("run finished",
		logger.String("user", p.UserName),
		logger.String("status", string(runner.StatusSuccess)),
		logger.Int("bookings", len(successes)),
		logger.String("message", msg),
		logger.String("txn_date", txnDate))
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

// buildOutcomeMessage renders the user-facing summary when at least one
// per-course attempt succeeded. With pure successes it's just
// buildSuccessMessage; with a mixed outcome (some succeeded, others
// failed/timed-out/cancelled) it appends the aggregated non-success detail in
// a parenthetical so the user knows what the alt attempt did.
func buildOutcomeMessage(successes, allResults []runner.Result) string {
	msg := buildSuccessMessage(successes)
	if len(allResults) == len(successes) {
		return msg
	}
	nonSuccesses := make([]runner.Result, 0, len(allResults)-len(successes))
	for _, r := range allResults {
		if r.Status != runner.StatusSuccess {
			nonSuccesses = append(nonSuccesses, r)
		}
	}
	return msg + " (also: " + aggregateFailureMessages(nonSuccesses) + ")"
}

// buildSuccessMessage formats the one-or-more successful per-course bookings
// into a single user-facing line. With one success it's the runner's existing
// message verbatim; with multiple it concatenates them (sorted by CourseID for
// stable output) and adds a cancel hint — the API allows one booking per
// course per day, so 2 successes = 2 real bookings the user can choose between.
func buildSuccessMessage(successes []runner.Result) string {
	if len(successes) == 1 {
		return successes[0].Message
	}
	sorted := append([]runner.Result(nil), successes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CourseID < sorted[j].CourseID })
	msgs := make([]string, 0, len(sorted))
	for _, s := range sorted {
		msgs = append(msgs, s.Message)
	}
	return fmt.Sprintf("%d bookings made — cancel any you don't want: %s",
		len(sorted), strings.Join(msgs, "; "))
}

// aggregateFailureMessages joins per-course outcome messages with "; ". Order
// is stable by CourseID so the same set of failures always produces the same
// string (workers complete in non-deterministic order otherwise).
func aggregateFailureMessages(results []runner.Result) string {
	parts := make([]string, 0, len(results))
	sorted := append([]runner.Result(nil), results...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CourseID < sorted[j].CourseID })
	for _, r := range sorted {
		if r.Message != "" {
			parts = append(parts, r.Message)
		}
	}
	if len(parts) == 0 {
		// Defensive: runner.Run always sets Message, but keep a sentinel
		// so a future regression / panic-recovery edge can't surface "" to
		// notify + history.
		return "no slot booked"
	}
	return strings.Join(parts, "; ")
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
