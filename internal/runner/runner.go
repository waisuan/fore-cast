package runner

import (
	"context"
	"errors"
	"fmt"
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
// When Retry is true, each parallel worker continually retries until it gets a slot
// or another worker succeeds. When Retry is false, each worker tries once.
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
			runWorker(ctx, client, cfg, slot, &once, outcomeCh, cancel)
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

// slotTag returns a compact identifier for log lines, e.g. "[7:00 AM S1 T1]".
func slotTag(slot *booker.TeeTimeSlot) string {
	t := slotutil.FormatCutoffDisplay(slot.TeeTime)
	return fmt.Sprintf("[%s S%s T%s]", t, slot.Session, slot.TeeBox.String())
}

// runWorker runs the booking loop for one slot. When cfg.Retry is true, it continually
// retries (check, book) until it succeeds or ctx is cancelled (neighbour won or timeout).
func runWorker(ctx context.Context, client booker.ClientInterface, cfg Config, target booker.TeeTimeSlot,
	once *sync.Once, outcomeCh chan<- Result, cancel context.CancelFunc) {
	slot := &target
	tag := slotTag(slot)
	timer := time.NewTimer(cfg.RetryInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		logger.Info(tag+" slot",
			logger.String("tee_time", slot.TeeTime),
			logger.String("session", slot.Session),
			logger.String("tee_box", slot.TeeBox.String()))
		if err := checkTeeTimeStatus(client, cfg.Token, slot, tag, cfg.TxnDate, cfg.UserName); err != nil {
			logger.Error(tag+" failed to check tee time status", logger.Err(err))
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

		select {
		case <-ctx.Done():
			return
		default:
		}

		booked, bookingID, bookErr := tryBookSlot(client, cfg, slot, tag)
		if bookErr != nil {
			logger.Error(tag+" failed to book slot", logger.Err(bookErr))
		}
		if booked {
			msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s",
				slot.TeeTime, slot.Session, slot.TeeBox.String(), cfg.TxnDate, cfg.CourseID, bookingID)
			logger.Info(tag + " " + msg)
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

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func tryBookSlot(client booker.ClientInterface, cfg Config, slot *booker.TeeTimeSlot, tag string) (bool, string, error) {
	logger.Info(tag+" attempting to book", logger.Time("at", time.Now()))
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
		reason := resp.Reason
		if reason == "" {
			reason = "booking failed"
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
		reason = "(empty)"
	}
	logger.Info(tag+" tee time status checked", logger.Bool("status", resp.Status), logger.String("reason", reason))
	return nil
}
