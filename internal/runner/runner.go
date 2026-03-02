package runner

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/waisuan/alfred/internal/booker"
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

		fmt.Printf("Target date: %s | Course: %s", cfg.TxnDate, cfg.CourseID)
		if cfg.Retry {
			fmt.Printf(" (round %d)", round)
		}
		fmt.Println()

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
			fmt.Fprintf(os.Stderr, "[round %d] get tee times: %v\n", round, err)
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
			fmt.Printf("Slot: %s %s (TeeBox %s)\n", slot.TeeTime, slot.Session, slot.TeeBox.String())

			printTeeTimeStatus(client, cfg.Token, slot, cfg.TxnDate, cfg.UserName)

			booked, bookingID, bookErr := tryBookSlot(client, cfg, slot)
			if bookErr != nil {
				fmt.Fprintf(os.Stderr, "  book error for %s: %v\n", slot.TeeTime, bookErr)
			}
			if booked {
				msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s",
					slot.TeeTime, slot.Session, slot.TeeBox.String(), cfg.TxnDate, cfg.CourseID, bookingID)
				fmt.Println(msg)
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

		fmt.Printf("No slot booked this round. Retrying in %s...\n", cfg.RetryInterval)
		time.Sleep(cfg.RetryInterval)
	}
	return Result{Status: StatusFailed, Message: "no booking made"}, nil
}

func tryBookSlot(client booker.ClientInterface, cfg Config, slot *booker.TeeTimeSlot) (bool, string, error) {
	fmt.Printf("Attempting to book at %s\n", time.Now().Format("2006-01-02 15:04:05"))
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
		fmt.Fprintf(os.Stderr, "  tee time status: error %v\n", err)
		return
	}
	reason := resp.Reason
	if reason == "" {
		reason = "(empty)"
	}
	fmt.Printf("  tee time status: Status=%v Reason=%s\n", resp.Status, reason)
}
