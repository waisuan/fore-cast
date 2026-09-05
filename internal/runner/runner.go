package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/slotutil"
)

// Status describes the outcome of a booking run.
type Status string

const (
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusNoSlots   Status = "no_slots"
	StatusCancelled Status = "cancelled"
)

const flightAlreadyReservedPhrase = "The flight has already been reserved"

// Phrases the club uses when booking is blocked account-wide (window still
// closed, or rate-limit / lock). One check is enough; do not book.
var bookingNotOpenPhrases = []string{
	"will be open after",
	"not allowed during this time range",
	"rapid attempts",
	"temporarily locked",
}

var rapidLockPhrases = []string{
	"rapid attempts",
	"temporarily locked",
}

const (
	// DefaultAccountWideCooldown is the scheduler pause after a window-closed
	// check/book ("open after 10pm", "not allowed during this time range").
	// Flat, not exponential. Long enough that a 1s RetryInterval does not
	// poll a shut window into Rapid; short enough to still be near 22:00
	// when the club opens. Not used after a CODE103 re-login — a new token
	// is usable immediately.
	DefaultAccountWideCooldown = 5 * time.Second
	// DefaultRapidBackoffInitial is the first Rapid / "temporarily locked"
	// wait. Same 3s as CheckToBookDelay: do not hammer a locked account.
	DefaultRapidBackoffInitial = booker.DefaultCheckToBookDelay
	// DefaultRapidBackoffMax is the Rapid cap: 3s → 6s → 12s → 24s.
	DefaultRapidBackoffMax = 24 * time.Second
)

type passWait int

const (
	waitRetryOnly passWait = iota
	waitAccountWide
	waitRapid
)

// Config holds input parameters for a booking run.
type Config struct {
	UserName      string
	Token         string
	TxnDate       string
	CourseID      string
	CutoffTeeTime string
	// RetryInterval is the pause after a normal unsuccessful pass (slots
	// checked, none booked, window not closed, not Rapid). Preset default is
	// 1s. Zero means no extra delay. Window-closed and Rapid waits replace
	// this when they are longer.
	RetryInterval time.Duration
	Debug         bool
	// Timeout is the maximum duration for the whole run when > 0 (repeated passes until success, all-reserved exit, or deadline).
	// When 0, exactly one full pass is attempted.
	Timeout time.Duration
	// CheckToBookDelay waits after a successful check (Action=0 selects the
	// flight) before book. Club returns 20017 if we book sooner. Scheduler
	// uses 3s (booker.DefaultCheckToBookDelay). Zero means no wait (tests).
	CheckToBookDelay time.Duration
	// Wait clocks (scheduler values; tests leave them 0 → RetryInterval only):
	//
	//   CheckToBookDelay      3s   after a *successful* check, before book
	//   AccountWideCooldown   5s   after window-closed only
	//   RapidBackoffInitial   3s   first Rapid / temporarily-locked wait
	//   RapidBackoffMax      24s   Rapid cap (3, 6, 12, 24)
	//
	// AccountWideCooldown is the minimum pause after a window-closed pass.
	// Zero means only RetryInterval.
	AccountWideCooldown time.Duration
	// RapidBackoffInitial is the first wait after Rapid / temporarily locked.
	// Each consecutive Rapid doubles it until RapidBackoffMax. A later
	// window-closed or normal pass resets to this. Zero means only RetryInterval.
	RapidBackoffInitial time.Duration
	// RapidBackoffMax caps the Rapid exponential wait. Zero means uncapped.
	RapidBackoffMax time.Duration
	// RefreshToken, when set, is called after CODE103 so the run can continue
	// until ctx deadline instead of failing fast. Nil keeps fail-fast (tests, UI).
	RefreshToken func(ctx context.Context) (string, error)
}

// Result describes the outcome of a booking run.
type Result struct {
	Status    Status
	Message   string
	TeeTime   string
	TeeBox    string
	CourseID  string
	BookingID string
}

func resultWithCourse(cfg Config, status Status, msg string) Result {
	if cfg.CourseID != "" && msg != "" {
		msg = fmt.Sprintf("[%s] %s", cfg.CourseID, msg)
	}
	return Result{
		Status:   status,
		Message:  msg,
		CourseID: cfg.CourseID,
	}
}

// Run fetches slots before the cutoff, then repeatedly walks them in order: CheckTeeTimeStatus,
// then at most one BookTeeTime per slot (skipped when the check Reason indicates the flight is
// already reserved). If the check says the booking window is not open yet, the rest of that
// pass is skipped (no further slots, no book). Window-closed waits
// AccountWideCooldown (5s in cron). Rapid waits 3s, then 6s, 12s, 24s.
// Invalid token calls RefreshToken when set; the next pass uses RetryInterval
// only (no extra 5s). If the next check is still Rapid, Rapid backoff applies.
// The caller should supply ctx with an appropriate deadline (or cancel) for repeat mode; cancellation
// yields StatusCancelled.
func Run(ctx context.Context, cfg Config, client booker.ClientInterface) (Result, error) {
	slots, err := client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
	if err != nil && errors.Is(err, booker.ErrInvalidToken) && tryRefreshToken(ctx, &cfg) == nil {
		slots, err = client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
	}
	if err != nil {
		if errors.Is(err, booker.ErrInvalidToken) {
			msg := "session expired — please log in again"
			r := resultWithCourse(cfg, StatusFailed, msg)
			return r, fmt.Errorf("get tee times: %w", err)
		}
		r := resultWithCourse(cfg, StatusFailed, err.Error())
		return r, fmt.Errorf("get tee times: %w", err)
	}

	slotsBeforeCutoff := slotutil.SlotsBeforeCutoff(slots, cfg.CutoffTeeTime)
	if len(slotsBeforeCutoff) == 0 {
		msg := fmt.Sprintf("no slots available before %s cutoff", slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime))
		r := resultWithCourse(cfg, StatusNoSlots, msg)
		return r, fmt.Errorf("%s", r.Message)
	}

	repeatPasses := cfg.Timeout > 0
	rapidWait := cfg.RapidBackoffInitial

	for {
		if repeatPasses {
			select {
			case <-ctx.Done():
				return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
			default:
			}
		}

		success, res, allReserved, waitKind, passErr := runOnePass(ctx, client, &cfg, slotsBeforeCutoff)
		if passErr != nil {
			if errors.Is(passErr, booker.ErrInvalidToken) {
				if tryRefreshToken(ctx, &cfg) == nil {
					if waitErr := sleepCtx(ctx, retryWait(cfg, waitRetryOnly, 0)); waitErr != nil {
						return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
					}
					continue
				}
				msg := "session expired — please log in again"
				r := resultWithCourse(cfg, StatusFailed, msg)
				return r, fmt.Errorf("%s: %w", r.Message, passErr)
			}
			if errors.Is(passErr, context.Canceled) || errors.Is(passErr, context.DeadlineExceeded) {
				return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
			}
			r := resultWithCourse(cfg, StatusFailed, passErr.Error())
			return r, fmt.Errorf("booking pass: %w", passErr)
		}
		if success {
			return res, nil
		}
		if allReserved {
			msg := fmt.Sprintf("all tee times before %s cutoff already reserved", slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime))
			r := resultWithCourse(cfg, StatusFailed, msg)
			return r, fmt.Errorf("%s", r.Message)
		}
		if !repeatPasses {
			return noBookingResult(cfg, slotsBeforeCutoff)
		}
		d := retryWait(cfg, waitKind, rapidWait)
		if waitKind == waitRapid && cfg.RapidBackoffInitial > 0 {
			logger.Info("rapid lock, backing off",
				logger.String("user", cfg.UserName),
				logger.String("course", cfg.CourseID),
				logger.Duration("wait", d))
			rapidWait = nextRapidBackoff(rapidWait, cfg.RapidBackoffMax)
		} else {
			rapidWait = cfg.RapidBackoffInitial
		}
		if waitErr := sleepCtx(ctx, d); waitErr != nil {
			return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
		}
	}
}

func tryRefreshToken(ctx context.Context, cfg *Config) error {
	if cfg.RefreshToken == nil {
		return booker.ErrInvalidToken
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tok, err := cfg.RefreshToken(ctx)
	if err != nil || tok == "" {
		if err == nil {
			err = booker.ErrInvalidToken
		}
		return err
	}
	cfg.Token = tok
	logger.Info("refreshed club session after invalid token",
		logger.String("user", cfg.UserName),
		logger.String("course", cfg.CourseID))
	return nil
}

func retryWait(cfg Config, kind passWait, rapidWait time.Duration) time.Duration {
	d := cfg.RetryInterval
	switch kind {
	case waitRapid:
		if rapidWait > d {
			d = rapidWait
		}
	case waitAccountWide:
		if cfg.AccountWideCooldown > d {
			d = cfg.AccountWideCooldown
		}
	}
	return d
}

func nextRapidBackoff(cur, max time.Duration) time.Duration {
	if cur <= 0 {
		return 0
	}
	n := cur * 2
	if max > 0 && n > max {
		return max
	}
	return n
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func resultForDeadline(cfg Config, slots []booker.TeeTimeSlot, ctx context.Context) (Result, error) {
	_ = slots
	if ctx.Err() == context.DeadlineExceeded {
		r := resultWithCourse(cfg, StatusFailed, "no slot booked")
		return r, fmt.Errorf("timeout after %s with no booking", cfg.Timeout)
	}
	r := resultWithCourse(cfg, StatusCancelled, "Run cancelled")
	return r, fmt.Errorf("booking cancelled: %w", ctx.Err())
}

func noBookingResult(cfg Config, slots []booker.TeeTimeSlot) (Result, error) {
	msg := fmt.Sprintf("no slots booked before %s cutoff (tried %d slot(s), earliest was %s)",
		slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime), len(slots), slots[0].TeeTime)
	r := resultWithCourse(cfg, StatusFailed, msg)
	return r, fmt.Errorf("%s", r.Message)
}

func runOnePass(ctx context.Context, client booker.ClientInterface, cfg *Config, slots []booker.TeeTimeSlot) (success bool, result Result, allReserved bool, waitKind passWait, err error) {
	allSeenReserved := true
	for i := range slots {
		select {
		case <-ctx.Done():
			return false, Result{}, false, waitRetryOnly, ctx.Err()
		default:
		}

		slot := &slots[i]
		tag := slotTag(slot)
		logger.Debug(tag+" slot",
			logger.String("user", cfg.UserName),
			logger.String("course", slot.CourseID),
			logger.String("tee_time", slot.TeeTime),
			logger.String("session", slot.Session),
			logger.String("tee_box", slot.TeeBox.String()))

		checkIn := booker.GolfCheckTeeTimeStatusInput{
			CourseID:  slot.CourseID,
			TxnDate:   cfg.TxnDate,
			Session:   slot.Session,
			TeeBox:    slot.TeeBox.String(),
			TeeTime:   slot.TeeTime,
			UserName:  cfg.UserName,
			IPAddress: cfg.UserName,
		}
		resp, checkErr := client.CheckTeeTimeStatus(cfg.Token, checkIn)
		if checkErr != nil {
			logger.Error(tag+" failed to check tee time status",
				logger.String("user", cfg.UserName),
				logger.String("course", slot.CourseID),
				logger.Err(checkErr))
			allSeenReserved = false
			booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
			if booked {
				return true, successResult(cfg, slot, bookingID), false, waitRetryOnly, nil
			}
			if bookErr != nil && errors.Is(bookErr, booker.ErrInvalidToken) {
				return false, Result{}, false, waitRetryOnly, bookErr
			}
			if bookErr != nil {
				logger.Error(tag+" failed to book slot",
					logger.String("user", cfg.UserName),
					logger.String("course", slot.CourseID),
					logger.Err(bookErr))
			}
			continue
		}

		reason := resp.Reason
		if reason == "" && !resp.Status {
			reason = "slot not available"
		}
		logger.Info(tag+" tee time status checked",
			logger.String("user", cfg.UserName),
			logger.String("course", slot.CourseID),
			logger.Bool("status", resp.Status),
			logger.String("reason", reason))

		if !resp.Status && booker.IsInvalidToken(resp.Reason) {
			return false, Result{}, false, waitRetryOnly, fmt.Errorf("tee time status: %w", booker.ErrInvalidToken)
		}
		if !resp.Status && reasonFlightAlreadyReserved(reason) {
			logger.Debug(tag+" flight already reserved per status, skipping book",
				logger.String("user", cfg.UserName),
				logger.String("course", slot.CourseID))
			continue
		}
		if !resp.Status && reasonBookingNotOpen(reason) {
			return false, Result{}, false, waitKindForReason(reason), nil
		}

		if resp.Status && cfg.CheckToBookDelay > 0 {
			select {
			case <-ctx.Done():
				return false, Result{}, false, waitRetryOnly, ctx.Err()
			case <-time.After(cfg.CheckToBookDelay):
			}
		}

		allSeenReserved = false

		booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
		if booked {
			return true, successResult(cfg, slot, bookingID), false, waitRetryOnly, nil
		}
		if bookErr != nil && errors.Is(bookErr, booker.ErrInvalidToken) {
			return false, Result{}, false, waitRetryOnly, bookErr
		}
		if bookErr != nil {
			logger.Error(tag+" failed to book slot",
				logger.String("user", cfg.UserName),
				logger.String("course", slot.CourseID),
				logger.Err(bookErr))
			if reasonBookingNotOpen(bookErr.Error()) {
				return false, Result{}, false, waitKindForReason(bookErr.Error()), nil
			}
		}
	}
	return false, Result{}, allSeenReserved, waitRetryOnly, nil
}

func reasonFlightAlreadyReserved(reason string) bool {
	return reasonContains(reason, flightAlreadyReservedPhrase)
}

func reasonBookingNotOpen(reason string) bool {
	return reasonContains(reason, bookingNotOpenPhrases...)
}

func reasonRapidLock(reason string) bool {
	return reasonContains(reason, rapidLockPhrases...)
}

func waitKindForReason(reason string) passWait {
	if reasonRapidLock(reason) {
		return waitRapid
	}
	return waitAccountWide
}

func reasonContains(reason string, phrases ...string) bool {
	low := strings.ToLower(reason)
	for _, p := range phrases {
		if strings.Contains(low, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// slotTag builds the human-readable prefix used for per-slot log lines.
// CourseID is included up front so that when two courses run in parallel for
// the same user (BRC + PLC fan-out), the interleaved log lines are still
// attributable at a glance — e.g. "[BRC 7:46 AM SMorning T10] ...".
func slotTag(slot *booker.TeeTimeSlot) string {
	t := slotutil.FormatCutoffDisplay(slot.TeeTime)
	return fmt.Sprintf("[%s %s S%s T%s]", slot.CourseID, t, slot.Session, slot.TeeBox.String())
}

func successResult(cfg *Config, slot *booker.TeeTimeSlot, bookingID string) Result {
	msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s",
		slot.TeeTime, slot.Session, slot.TeeBox.String(), cfg.TxnDate, cfg.CourseID, bookingID)
	return Result{
		Status:    StatusSuccess,
		Message:   msg,
		TeeTime:   slot.TeeTime,
		TeeBox:    slot.TeeBox.String(),
		CourseID:  cfg.CourseID,
		BookingID: bookingID,
	}
}

func tryBookSlot(client booker.ClientInterface, cfg *Config, slot *booker.TeeTimeSlot, tag string) (booked bool, bookingID string, err error) {
	input := booker.GolfNewBooking2Input{
		CourseID:   slot.CourseID,
		TxnDate:    cfg.TxnDate,
		Session:    slot.Session,
		TeeBox:     slot.TeeBox.String(),
		TeeTime:    slot.TeeTime,
		AccountID:  cfg.UserName,
		TotalGuest: 4,
		IPaddress:  cfg.UserName,
		Holes:      18,
	}
	logger.Debug(tag+" attempting to book",
		logger.String("user", cfg.UserName),
		logger.String("course", slot.CourseID),
		logger.Time("at", time.Now()))
	resp, e := client.BookTeeTime(cfg.Token, input, cfg.Debug)
	if e != nil {
		return false, "", e
	}
	if !resp.Status || len(resp.Result) == 0 || !resp.Result[0].Status {
		reason := resp.Reason
		if reason == "" {
			reason = "booking failed"
		}
		if booker.IsInvalidToken(resp.Reason) {
			return false, "", fmt.Errorf("%s: %w", reason, booker.ErrInvalidToken)
		}
		return false, "", fmt.Errorf("%s", reason)
	}
	return true, resp.Result[0].BookingID, nil
}
