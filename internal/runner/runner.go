package runner

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/slotutil"
)

// Status describes the outcome of a booking run.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusNoSlots Status = "no_slots"

	// MsgFlightAlreadyReserved is the API reason when a tee time’s flight is already reserved.
	MsgFlightAlreadyReserved = "The flight has already been reserved; kindly refresh the flight"
)

// Config holds input parameters for a booking run.
type Config struct {
	UserName         string
	Token            string
	TxnDate          string
	CourseID         string
	CutoffTeeTime    string
	RetryInterval    time.Duration
	Retry            bool
	Debug            bool
	Timeout          time.Duration
	MaxParallelSlots int // max slots to try in parallel (default 5 when 0)

	// RefreshToken, when set, is called when the API returns CODE103 (invalid token).
	// The runner updates Config.Token with the new token and retries. RefreshTokenMu
	// serializes refresh attempts across workers.

	// StartupJitterMax is the max random delay before each worker starts (0 = disabled).
	// Helps stagger workers to avoid thundering herd.
	StartupJitterMax time.Duration

	RefreshToken   func() (string, error)
	RefreshTokenMu *sync.Mutex
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

// Run executes the booking loop: fetch slots, filter by cutoff, attempt to book.
// When Retry is true, each worker polls until CheckTeeTimeStatus succeeds once, then
// retries booking until it wins, another worker succeeds (shared cancel), or timeout.
// When Retry is false, each worker tries one check and one book.
func Run(cfg Config, client booker.ClientInterface) (Result, error) {
	maxParallel := cfg.MaxParallelSlots
	slots, err := client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
	if err != nil {
		if errors.Is(err, booker.ErrInvalidToken) {
			msg := "session expired — please log in again"
			return Result{Status: StatusFailed, Message: msg},
				fmt.Errorf("get tee times: %w", err)
		}
		return Result{Status: StatusFailed, Message: err.Error()},
			fmt.Errorf("get tee times: %w", err)
	}

	slotsBeforeCutoff := slotutil.SlotsBeforeCutoff(slots, cfg.CutoffTeeTime)
	if len(slotsBeforeCutoff) == 0 {
		msg := fmt.Sprintf("no slots available before %s cutoff", slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime))
		return Result{Status: StatusNoSlots, Message: msg}, fmt.Errorf("%s", msg)
	}

	slotsToTry := slotsBeforeCutoff
	if len(slotsToTry) > maxParallel {
		slotsToTry = slotsToTry[:maxParallel]
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if cfg.Retry && cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.Timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	var once sync.Once
	outcomeCh := make(chan Result, 1)
	var wg sync.WaitGroup
	wg.Add(len(slotsToTry))

	go func() {
		wg.Wait()
		once.Do(func() {
			outcomeCh <- Result{Status: StatusFailed, Message: "no slot booked"}
		})
	}()

	for i := range slotsToTry {
		target := slotsToTry[i]
		go func(slot booker.TeeTimeSlot) {
			defer wg.Done()
			runWorker(ctx, client, &cfg, slot, &once, outcomeCh, cancel)
		}(target)
	}

	result := <-outcomeCh
	timedOut := ctx.Err() == context.DeadlineExceeded
	cancel()
	if result.Status == StatusSuccess {
		return result, nil
	}
	if result.Message == "session expired — please log in again" {
		return result, fmt.Errorf("%s", result.Message)
	}
	if timedOut {
		return result, fmt.Errorf("timeout after %s with no booking", cfg.Timeout)
	}
	msg := fmt.Sprintf("no slots booked before %s cutoff (tried first %d of %d, earliest was %s)",
		slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime), len(slotsToTry), len(slotsBeforeCutoff), slotsBeforeCutoff[0].TeeTime)
	result.Message = msg
	return result, fmt.Errorf("%s", msg)
}

func isFlightAlreadyReservedMessage(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "flight has already been reserved") &&
		strings.Contains(low, "refresh the flight")
}

// isExpectedPreOpenCheckError matches CheckTeeTimeStatus failures that are normal
// before the club opens booking (e.g. “open after 10pm”) or while a flight is blocked.
func isExpectedPreOpenCheckError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if !strings.Contains(s, "tee time status false:") {
		return false
	}
	return strings.Contains(s, "flight time will be open after") ||
		strings.Contains(s, "this flight is blocked")
}

// slotTag returns a compact identifier for log lines, e.g. "[7:00 AM S1 T1]".
func slotTag(slot *booker.TeeTimeSlot) string {
	t := slotutil.FormatCutoffDisplay(slot.TeeTime)
	return fmt.Sprintf("[%s S%s T%s]", t, slot.Session, slot.TeeBox.String())
}

// runWorker runs the booking loop for one slot. When cfg.Retry is true, it polls
// CheckTeeTimeStatus until the slot reports available, then retries BookTeeTime only
// until success or ctx is cancelled (another worker booked or timeout), without
// re-checking between book attempts.
func runWorker(ctx context.Context, client booker.ClientInterface, cfg *Config, target booker.TeeTimeSlot,
	once *sync.Once, outcomeCh chan<- Result, cancel context.CancelFunc) {
	// Stagger worker startup to avoid thundering herd
	if cfg.StartupJitterMax > 0 {
		jitter := time.Duration(rand.Int63n(int64(cfg.StartupJitterMax)))
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter):
		}
	}

	slot := &target
	tag := slotTag(slot)
	timer := time.NewTimer(cfg.RetryInterval)
	defer timer.Stop()

	// After CheckTeeTimeStatus returns OK once, keep attempting BookTeeTime until this worker
	// books, the run context is cancelled (another worker succeeded), or timeout — without
	// re-checking availability each iteration.
	confirmedAvailable := false

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		logger.Debug(tag+" slot",
			logger.String("tee_time", slot.TeeTime),
			logger.String("session", slot.Session),
			logger.String("tee_box", slot.TeeBox.String()))

		if !confirmedAvailable {
			if err := checkTeeTimeStatus(client, cfg.Token, slot, tag, cfg.TxnDate, cfg.UserName); err != nil {
				if isFlightAlreadyReservedMessage(err.Error()) {
					logger.Warn(tag+" flight already reserved for this slot, worker stopping", logger.Err(err))
					return
				}
				if errors.Is(err, booker.ErrInvalidToken) && cfg.RefreshToken != nil {
					if _, refreshErr := refreshToken(cfg, cfg.Token); refreshErr != nil {
						logger.Error(tag+" token refresh failed", logger.Err(refreshErr))
					} else {
						logger.Info(tag + " token refreshed, retrying")
						continue
					}
				}
				if isExpectedPreOpenCheckError(err) {
					logger.Debug(tag+" tee time not open yet, will retry",
						logger.String("phase", "pre_open"),
						logger.Err(err))
				} else {
					logger.Error(tag+" failed to check tee time status", logger.Err(err))
				}
				if !cfg.Retry {
					return
				}
				resetTimer(timer, cfg.RetryInterval)
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
				continue
			}
			confirmedAvailable = true
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
		if bookErr != nil {
			if errors.Is(bookErr, booker.ErrInvalidToken) && cfg.RefreshToken != nil {
				if _, refreshErr := refreshToken(cfg, cfg.Token); refreshErr != nil {
					logger.Error(tag+" token refresh failed", logger.Err(refreshErr))
				} else {
					logger.Info(tag + " token refreshed, retrying")
					resetTimer(timer, cfg.RetryInterval)
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
					}
					continue
				}
			}
			if isLostRaceBookError(bookErr) {
				logger.Debug(tag+" book lost race, will retry",
					logger.String("outcome", "lost_race"),
					logger.Err(bookErr))
			} else {
				logger.Error(tag+" failed to book slot", logger.Err(bookErr))
			}
		}
		if booked {
			msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s",
				slot.TeeTime, slot.Session, slot.TeeBox.String(), cfg.TxnDate, cfg.CourseID, bookingID)
			logger.Info(tag+" "+msg,
				logger.String("event", "booking_success"),
				logger.String("booking_id", bookingID),
				logger.String("tee_time", slot.TeeTime),
				logger.String("tee_box", slot.TeeBox.String()),
				logger.String("course_id", cfg.CourseID),
				logger.String("txn_date", cfg.TxnDate))
			once.Do(func() {
				cancel()
				outcomeCh <- Result{
					Status:    StatusSuccess,
					Message:   msg,
					TeeTime:   slot.TeeTime,
					TeeBox:    slot.TeeBox.String(),
					CourseID:  cfg.CourseID,
					BookingID: bookingID,
				}
			})
			return
		}

		if !cfg.Retry {
			return
		}

		resetTimer(timer, cfg.RetryInterval)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func refreshToken(cfg *Config, oldToken string) (string, error) {
	if cfg.RefreshTokenMu != nil {
		cfg.RefreshTokenMu.Lock()
		defer cfg.RefreshTokenMu.Unlock()
	}
	// Another worker may have refreshed while we waited for the lock
	if cfg.Token != oldToken {
		return cfg.Token, nil
	}
	newToken, err := cfg.RefreshToken()
	if err != nil {
		return "", err
	}
	cfg.Token = newToken
	return newToken, nil
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

// isLostRaceBookError matches API failures where the slot looked bookable but the
// reservation step lost a race (e.g. "Tee Time was not reserve").
func isLostRaceBookError(err error) bool {
	if err == nil || errors.Is(err, booker.ErrInvalidToken) {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not reserve") ||
		strings.Contains(s, "not reserved") ||
		strings.Contains(s, "unable to reserve") ||
		strings.Contains(s, "already been taken") ||
		strings.Contains(s, "no longer available")
}

// tryBookSlot calls BookTeeTime once; runWorker retries on a schedule for lost races.
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
	logger.Debug(tag+" attempting to book", logger.Time("at", time.Now()))
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

// checkTeeTimeStatus calls the API to validate the slot. Returns an error if the check fails;
// the caller should skip booking and retry when that happens.
func checkTeeTimeStatus(client booker.ClientInterface, token string, slot *booker.TeeTimeSlot, tag, txnDate, userName string) error {
	checkInput := booker.GolfCheckTeeTimeStatusInput{
		CourseID:  slot.CourseID,
		TxnDate:   txnDate,
		Session:   slot.Session,
		TeeBox:    slot.TeeBox.String(),
		TeeTime:   slot.TeeTime,
		UserName:  userName,
		IPAddress: userName,
	}
	resp, err := client.CheckTeeTimeStatus(token, checkInput)
	if err != nil {
		return err
	}
	reason := resp.Reason
	if reason == "" {
		reason = "slot not available"
	}
	logger.Debug(tag+" tee time status checked", logger.Bool("status", resp.Status), logger.String("reason", reason))
	if !resp.Status {
		if booker.IsInvalidToken(resp.Reason) {
			return fmt.Errorf("tee time status false: %w", booker.ErrInvalidToken)
		}
		return fmt.Errorf("tee time status false: %s", reason)
	}
	return nil
}
