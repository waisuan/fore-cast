package runner

import (
	"errors"
	"fmt"
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
)

// Config holds input parameters for a booking run.
type Config struct {
	UserName      string
	Token         string
	TxnDate       string
	CourseID      string
	CutoffTeeTime string
	RetryInterval time.Duration
	Retry         bool
	Debug         bool
	Timeout       time.Duration
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
// When Retry is true, it loops until a slot is booked, no slots remain, or timeout.
func Run(cfg Config, client booker.ClientInterface) (Result, error) {
	start := time.Now()

	for round := 1; ; round++ {
		if !cfg.Retry && round > 1 {
			break
		}
		if cfg.Retry && cfg.Timeout > 0 && time.Since(start) >= cfg.Timeout {
			return Result{Status: StatusFailed, Message: fmt.Sprintf("timeout after %s with no booking", cfg.Timeout)},
				fmt.Errorf("timeout after %s with no booking", cfg.Timeout)
		}

		logger.Info("round",
			logger.String("txn_date", cfg.TxnDate),
			logger.String("course", cfg.CourseID),
			logger.Int("round", round))

		slots, err := client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
		if err != nil {
			if errors.Is(err, booker.ErrInvalidToken) {
				msg := "session expired — please log in again"
				return Result{Status: StatusFailed, Message: msg},
					fmt.Errorf("get tee times: %w", err)
			}
			if !cfg.Retry {
				return Result{Status: StatusFailed, Message: err.Error()},
					fmt.Errorf("get tee times: %w", err)
			}
			logger.Warn("get tee times failed", logger.Int("round", round), logger.Err(err))
			time.Sleep(cfg.RetryInterval)
			continue
		}

		slotsBeforeCutoff := slotutil.SlotsBeforeCutoff(slots, cfg.CutoffTeeTime)
		if len(slotsBeforeCutoff) == 0 {
			msg := fmt.Sprintf("no slots available before %s cutoff", slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime))
			return Result{Status: StatusNoSlots, Message: msg}, fmt.Errorf("%s", msg)
		}

		for si := range slotsBeforeCutoff {
			slot := &slotsBeforeCutoff[si]
			logger.Info("slot",
				logger.String("tee_time", slot.TeeTime),
				logger.String("session", slot.Session),
				logger.String("tee_box", slot.TeeBox.String()))

			printTeeTimeStatus(client, cfg.Token, slot, cfg.TxnDate, cfg.UserName)

			booked, bookingID, bookErr := tryBookSlot(client, cfg, slot)
			if bookErr != nil {
				logger.Error("failed to book slot", logger.String("tee_time", slot.TeeTime), logger.Err(bookErr))
			}
			if booked {
				msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s",
					slot.TeeTime, slot.Session, slot.TeeBox.String(), cfg.TxnDate, cfg.CourseID, bookingID)
				logger.Info("booked", logger.String("message", msg))
				return Result{
					Status:    StatusSuccess,
					Message:   msg,
					TeeTime:   slot.TeeTime,
					TeeBox:    slot.TeeBox.String(),
					CourseID:  cfg.CourseID,
					BookingID: bookingID,
				}, nil
			}
		}

		if !cfg.Retry {
			msg := fmt.Sprintf("no slots booked before %s cutoff (tried %d, earliest was %s)",
				slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime), len(slotsBeforeCutoff), slotsBeforeCutoff[0].TeeTime)
			return Result{Status: StatusFailed, Message: msg}, fmt.Errorf("%s", msg)
		}

		logger.Info("no slot booked this round, retrying", logger.Duration("interval", cfg.RetryInterval))
		time.Sleep(cfg.RetryInterval)
	}
	return Result{Status: StatusFailed, Message: "no booking made"}, nil
}

func tryBookSlot(client booker.ClientInterface, cfg Config, slot *booker.TeeTimeSlot) (bool, string, error) {
	logger.Info("attempting to book", logger.Time("at", time.Now()))
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
	resp, err := client.BookTeeTime(cfg.Token, input, cfg.Debug)
	if err != nil {
		return false, "", err
	}
	if !resp.Status || len(resp.Result) == 0 || !resp.Result[0].Status {
		return false, "", nil
	}
	return true, resp.Result[0].BookingID, nil
}

func printTeeTimeStatus(client booker.ClientInterface, token string, slot *booker.TeeTimeSlot, txnDate, userName string) {
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
		logger.Error("failed to check tee time status", logger.Err(err))
		return
	}
	reason := resp.Reason
	if reason == "" {
		reason = "(empty)"
	}
	logger.Info("tee time status checked", logger.Bool("status", resp.Status), logger.String("reason", reason))
}
