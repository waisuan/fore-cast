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

// Config holds input parameters for a booking run.
type Config struct {
	UserName      string
	Token         string
	TxnDate       string
	CourseID      string
	CutoffTeeTime string
	// RetryInterval is the pause after a full pass through all cutoff slots before starting the next pass (0 = no extra delay).
	RetryInterval time.Duration
	Debug         bool
	// Timeout is the maximum duration for the whole run when > 0 (repeated passes until success, all-reserved exit, or deadline).
	// When 0, exactly one full pass is attempted.
	Timeout time.Duration
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

// Run fetches slots before the cutoff, then repeatedly walks them in order: CheckTeeTimeStatus,
// then at most one BookTeeTime per slot (skipped when the check Reason indicates the flight is
// already reserved). Sleeps RetryInterval only between full passes. Invalid token aborts immediately.
// The caller should supply ctx with an appropriate deadline (or cancel) for repeat mode; cancellation
// yields StatusCancelled.
func Run(ctx context.Context, cfg Config, client booker.ClientInterface) (Result, error) {
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

	repeatPasses := cfg.Timeout > 0

	for {
		if repeatPasses {
			select {
			case <-ctx.Done():
				return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
			default:
			}
		}

		success, res, allReserved, passErr := runOnePass(ctx, client, &cfg, slotsBeforeCutoff)
		if passErr != nil {
			if errors.Is(passErr, booker.ErrInvalidToken) {
				msg := "session expired — please log in again"
				return Result{Status: StatusFailed, Message: msg},
					fmt.Errorf("%s: %w", msg, passErr)
			}
			if errors.Is(passErr, context.Canceled) || errors.Is(passErr, context.DeadlineExceeded) {
				return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
			}
			return Result{Status: StatusFailed, Message: passErr.Error()},
				fmt.Errorf("booking pass: %w", passErr)
		}
		if success {
			return res, nil
		}
		if allReserved {
			msg := fmt.Sprintf("all tee times before %s cutoff already reserved", slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime))
			return Result{Status: StatusFailed, Message: msg}, fmt.Errorf("%s", msg)
		}
		if !repeatPasses {
			return noBookingResult(cfg, slotsBeforeCutoff)
		}
		if cfg.RetryInterval > 0 {
			select {
			case <-ctx.Done():
				return resultForDeadline(cfg, slotsBeforeCutoff, ctx)
			case <-time.After(cfg.RetryInterval):
			}
		}
	}
}

func resultForDeadline(cfg Config, slots []booker.TeeTimeSlot, ctx context.Context) (Result, error) {
	_ = slots
	if ctx.Err() == context.DeadlineExceeded {
		r := Result{Status: StatusFailed, Message: "no slot booked"}
		return r, fmt.Errorf("timeout after %s with no booking", cfg.Timeout)
	}
	r := Result{Status: StatusCancelled, Message: "Run cancelled"}
	return r, fmt.Errorf("booking cancelled: %w", ctx.Err())
}

func noBookingResult(cfg Config, slots []booker.TeeTimeSlot) (Result, error) {
	msg := fmt.Sprintf("no slots booked before %s cutoff (tried %d slot(s), earliest was %s)",
		slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime), len(slots), slots[0].TeeTime)
	r := Result{Status: StatusFailed, Message: msg}
	return r, fmt.Errorf("%s", msg)
}

func runOnePass(ctx context.Context, client booker.ClientInterface, cfg *Config, slots []booker.TeeTimeSlot) (success bool, result Result, allReserved bool, err error) {
	allSeenReserved := true
	for i := range slots {
		select {
		case <-ctx.Done():
			return false, Result{}, false, ctx.Err()
		default:
		}

		slot := &slots[i]
		tag := slotTag(slot)
		logger.Debug(tag+" slot",
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
			logger.Error(tag+" failed to check tee time status", logger.Err(checkErr))
			allSeenReserved = false
			booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
			if booked {
				return true, successResult(cfg, slot, bookingID), false, nil
			}
			if bookErr != nil && errors.Is(bookErr, booker.ErrInvalidToken) {
				return false, Result{}, false, bookErr
			}
			if bookErr != nil {
				logger.Error(tag+" failed to book slot", logger.Err(bookErr))
			}
			continue
		}

		reason := resp.Reason
		if reason == "" && !resp.Status {
			reason = "slot not available"
		}
		logger.Info(tag+" tee time status checked", logger.Bool("status", resp.Status), logger.String("reason", reason))

		if !resp.Status && booker.IsInvalidToken(resp.Reason) {
			return false, Result{}, false, fmt.Errorf("tee time status: %w", booker.ErrInvalidToken)
		}
		if !resp.Status && reasonFlightAlreadyReserved(reason) {
			logger.Debug(tag + " flight already reserved per status, skipping book")
			continue
		}

		if resp.Status {
			allSeenReserved = false
		} else {
			allSeenReserved = false
		}

		booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
		if booked {
			return true, successResult(cfg, slot, bookingID), false, nil
		}
		if bookErr != nil && errors.Is(bookErr, booker.ErrInvalidToken) {
			return false, Result{}, false, bookErr
		}
		if bookErr != nil {
			logger.Error(tag+" failed to book slot", logger.Err(bookErr))
		}
	}
	return false, Result{}, allSeenReserved, nil
}

func reasonFlightAlreadyReserved(reason string) bool {
	return strings.Contains(strings.ToLower(reason), strings.ToLower(flightAlreadyReservedPhrase))
}

func slotTag(slot *booker.TeeTimeSlot) string {
	t := slotutil.FormatCutoffDisplay(slot.TeeTime)
	return fmt.Sprintf("[%s S%s T%s]", t, slot.Session, slot.TeeBox.String())
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
