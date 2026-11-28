package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// Waits are the pauses between club calls. The scheduler uses DefaultWaits;
// tests shrink them. A zero field means "RetryInterval only".
type Waits struct {
	// CheckToBook follows a successful check (Action=0 selects the flight).
	// The club returns 20017 if we book sooner.
	CheckToBook time.Duration
	// WindowClosed follows "open after 10pm" style replies. Flat, so a 1s
	// RetryInterval does not poll a shut window into a rapid lock.
	WindowClosed time.Duration
	// RapidInitial follows the first "Rapid attempts" / "temporarily locked"
	// reply and doubles on each one after, up to RapidMax (0 = uncapped).
	RapidInitial time.Duration
	RapidMax     time.Duration
}

// DefaultWaits: 3s check-to-book, 5s window-closed, rapid 3s → 6s → 12s → 24s.
var DefaultWaits = Waits{
	CheckToBook:  booker.DefaultCheckToBookDelay,
	WindowClosed: 5 * time.Second,
	RapidInitial: 3 * time.Second,
	RapidMax:     24 * time.Second,
}

// Config holds input parameters for a booking run. The run repeats passes
// until a booking, all slots reserved, or ctx is done (deadline or cancel).
type Config struct {
	UserName      string
	Token         string
	TxnDate       string
	CourseID      string
	CutoffTeeTime string
	// RetryInterval is the pause after a normal unsuccessful pass. Longer
	// window-closed and rapid waits replace it.
	RetryInterval time.Duration
	Waits         Waits
	// RefreshToken is called after CODE103 so the run can continue with a new
	// session. Nil means fail on the first invalid token.
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
	// Steps is the human-readable run log shown to the user in History.
	Steps []Step
}

// Step is one line of the user-facing run log.
type Step struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// maxSteps bounds the stored log: a 10m run at a 1s retry would otherwise log
// thousands of identical passes.
const maxSteps = 150

// stepLog collects Steps and remembers the club's last reply for the summary.
type stepLog struct {
	steps     []Step
	dropped   int
	lastReply string
}

// add keeps the first step (slot count) and the latest ones: the end of a run
// is what explains its outcome.
func (l *stepLog) add(format string, args ...any) {
	l.steps = append(l.steps, Step{At: time.Now(), Text: fmt.Sprintf(format, args...)})
	if len(l.steps) > maxSteps {
		l.steps = slices.Delete(l.steps, 1, 2)
		l.dropped++
	}
}

// finish appends the closing line even when the log is full.
func (l *stepLog) finish(text string) []Step {
	if l.dropped > 0 {
		l.steps = slices.Insert(l.steps, 1, Step{At: l.steps[1].At, Text: fmt.Sprintf("… %d earlier steps not shown", l.dropped)})
	}
	return append(l.steps, Step{At: time.Now(), Text: text})
}

// reply is what a club Reason means for the run.
type reply int

const (
	replyOther        reply = iota // includes Status=true; try to book
	replyReserved                  // slot taken; skip it
	replyWindowClosed              // booking window not open; end the pass
	replyRapid                     // account rate-limited; end the pass, back off
)

func classify(reason string) reply {
	switch {
	case booker.IsReserved(reason):
		return replyReserved
	case booker.IsRapidLock(reason):
		return replyRapid
	case booker.IsWindowClosed(reason):
		return replyWindowClosed
	}
	return replyOther
}

func resultWithCourse(cfg Config, status Status, msg string) Result {
	if cfg.CourseID != "" && msg != "" {
		msg = fmt.Sprintf("[%s] %s", cfg.CourseID, msg)
	}
	return Result{Status: status, Message: msg, CourseID: cfg.CourseID}
}

// Run fetches slots before the cutoff, then repeatedly walks them in order:
// check each slot, wait Waits.CheckToBook, then book. A window-closed or rapid
// reply ends the pass early. CODE103 calls RefreshToken and retries.
func Run(ctx context.Context, cfg Config, client booker.ClientInterface) (Result, error) {
	log := &stepLog{}
	r, err := run(ctx, cfg, client, log)
	r.Steps = log.finish(stepSummary(r))
	return r, err
}

func stepSummary(r Result) string {
	switch r.Status {
	case StatusSuccess:
		return "Done: booked."
	case StatusCancelled:
		return "Stopped: you cancelled the run."
	}
	return "Done: nothing booked. " + r.Message
}

func run(ctx context.Context, cfg Config, client booker.ClientInterface, log *stepLog) (Result, error) {
	slots, err := client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
	if errors.Is(err, booker.ErrInvalidToken) && tryRefreshToken(ctx, &cfg) == nil {
		slots, err = client.GetTeeTimeSlots(cfg.Token, cfg.CourseID, cfg.TxnDate)
	}
	if err != nil {
		msg := err.Error()
		if errors.Is(err, booker.ErrInvalidToken) {
			msg = "session expired — please log in again"
		}
		return resultWithCourse(cfg, StatusFailed, msg), fmt.Errorf("get tee times: %w", err)
	}

	cutoff := slotutil.FormatCutoffDisplay(cfg.CutoffTeeTime)
	slots = slotutil.SlotsBeforeCutoff(slots, cfg.CutoffTeeTime)
	day := cfg.TxnDate
	if t, err := time.Parse("2006/01/02", cfg.TxnDate); err == nil {
		day = t.Format("Mon 2 Jan")
	}
	noun := "tee times"
	if len(slots) == 1 {
		noun = "tee time"
	}
	log.add("%s on %s: %d %s before your %s cutoff.", cfg.CourseID, day, len(slots), noun, cutoff)
	if len(slots) == 0 {
		msg := fmt.Sprintf("no slots available before %s cutoff", cutoff)
		r := resultWithCourse(cfg, StatusNoSlots, msg)
		return r, errors.New(r.Message)
	}

	rapidStreak := 0
	for {
		if ctx.Err() != nil {
			return resultForDone(cfg, ctx, log)
		}

		res, outcome, passErr := runOnePass(ctx, client, &cfg, slots, log)
		switch {
		case errors.Is(passErr, booker.ErrInvalidToken):
			if tryRefreshToken(ctx, &cfg) != nil {
				r := resultWithCourse(cfg, StatusFailed, "session expired — please log in again")
				return r, fmt.Errorf("%s: %w", r.Message, passErr)
			}
			log.add("Club session expired; logged in again.")
			// A new token is usable straight away; keep the rapid streak.
			if sleepCtx(ctx, cfg.RetryInterval) != nil {
				return resultForDone(cfg, ctx, log)
			}
			continue
		case passErr != nil:
			return resultForDone(cfg, ctx, log)
		case res != nil:
			return *res, nil
		case outcome == replyReserved:
			msg := fmt.Sprintf("all tee times before %s cutoff already reserved", cutoff)
			r := resultWithCourse(cfg, StatusFailed, msg)
			return r, errors.New(r.Message)
		}

		wait := cfg.RetryInterval
		switch outcome {
		case replyRapid:
			wait = max(wait, rapidWait(cfg.Waits, rapidStreak))
			rapidStreak++
			log.add("Club rate limit hit; waiting %s before trying again.", wait)
			logger.Info("rapid lock, backing off",
				logger.String("user", cfg.UserName),
				logger.String("course", cfg.CourseID),
				logger.Duration("wait", wait))
		case replyWindowClosed:
			wait = max(wait, cfg.Waits.WindowClosed)
			rapidStreak = 0
			log.add("Booking window not open yet; waiting %s.", wait)
		default:
			rapidStreak = 0
		}
		if sleepCtx(ctx, wait) != nil {
			return resultForDone(cfg, ctx, log)
		}
	}
}

// rapidWait is RapidInitial doubled streak times, capped at RapidMax.
func rapidWait(w Waits, streak int) time.Duration {
	d := w.RapidInitial
	for i := 0; i < streak && d > 0; i++ {
		d *= 2
		if w.RapidMax > 0 && d >= w.RapidMax {
			return w.RapidMax
		}
	}
	return d
}

func tryRefreshToken(ctx context.Context, cfg *Config) error {
	if cfg.RefreshToken == nil {
		return booker.ErrInvalidToken
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tok, err := cfg.RefreshToken(ctx)
	if err == nil && tok == "" {
		err = booker.ErrInvalidToken
	}
	if err != nil {
		return err
	}
	cfg.Token = tok
	logger.Info("refreshed club session after invalid token",
		logger.String("user", cfg.UserName),
		logger.String("course", cfg.CourseID))
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func resultForDone(cfg Config, ctx context.Context, log *stepLog) (Result, error) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		msg := "no slot booked"
		if log.lastReply != "" {
			msg += " — last club reply: " + log.lastReply
		}
		return resultWithCourse(cfg, StatusFailed, msg), errors.New("timeout with no booking")
	}
	return resultWithCourse(cfg, StatusCancelled, "Run cancelled"), fmt.Errorf("booking cancelled: %w", ctx.Err())
}

// runOnePass walks every slot once. It returns a non-nil result on a booking.
// outcome is replyReserved when every slot was already reserved, or the reply
// that ended the pass early (window closed / rapid). err is ErrInvalidToken or
// a ctx error.
func runOnePass(ctx context.Context, client booker.ClientInterface, cfg *Config, slots []booker.TeeTimeSlot, log *stepLog) (*Result, reply, error) {
	allReserved := true
	for i := range slots {
		if err := ctx.Err(); err != nil {
			return nil, replyOther, err
		}
		slot := &slots[i]
		tag := slotTag(slot)
		name := slotName(slot)

		resp, err := client.CheckTeeTimeStatus(cfg.Token, booker.GolfCheckTeeTimeStatusInput{
			CourseID:  slot.CourseID,
			TxnDate:   cfg.TxnDate,
			Session:   slot.Session,
			TeeBox:    slot.TeeBox.String(),
			TeeTime:   slot.TeeTime,
			UserName:  cfg.UserName,
			IPAddress: cfg.UserName,
		})
		if err != nil {
			// The check selects the flight; booking without it cannot succeed.
			logger.Error(tag+" failed to check tee time status",
				logger.String("user", cfg.UserName),
				logger.String("course", slot.CourseID),
				logger.Err(err))
			log.lastReply = err.Error()
			log.add("%s: could not check the slot (%v). Skipping it.", name, err)
			allReserved = false
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
		if resp.Status {
			log.add("%s: open.", name)
		} else {
			log.lastReply = reason
			log.add("%s: %s", name, reason)
			if booker.IsInvalidToken(resp.Reason) {
				return nil, replyOther, fmt.Errorf("tee time status: %w", booker.ErrInvalidToken)
			}
			switch r := classify(reason); r {
			case replyReserved:
				continue
			case replyWindowClosed, replyRapid:
				return nil, r, nil
			}
		}
		allReserved = false

		if resp.Status && cfg.Waits.CheckToBook > 0 {
			log.add("%s: waiting %s before booking (the club requires it).", name, cfg.Waits.CheckToBook)
			if sleepCtx(ctx, cfg.Waits.CheckToBook) != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					log.add("%s: time ran out before the booking was sent. Try a longer timeout.", name)
				}
				logger.Warn(tag+" deadline hit before book sent",
					logger.String("user", cfg.UserName),
					logger.String("course", slot.CourseID),
					logger.Duration("check_to_book_delay", cfg.Waits.CheckToBook))
				return nil, replyOther, ctx.Err()
			}
		}

		bookingID, err := bookSlot(client, cfg, slot)
		if err == nil {
			log.add("%s: booked. Booking ID %s.", name, bookingID)
			res := successResult(cfg, slot, bookingID)
			return &res, replyOther, nil
		}
		if errors.Is(err, booker.ErrInvalidToken) {
			return nil, replyOther, err
		}
		logger.Error(tag+" failed to book slot",
			logger.String("user", cfg.UserName),
			logger.String("course", slot.CourseID),
			logger.Err(err))
		log.lastReply = err.Error()
		log.add("%s: booking failed: %s", name, err)
		if r := classify(err.Error()); r == replyWindowClosed || r == replyRapid {
			return nil, r, nil
		}
	}
	if allReserved {
		return nil, replyReserved, nil
	}
	return nil, replyOther, nil
}

// slotTag builds the per-slot log prefix. CourseID comes first so BRC and PLC
// lines stay attributable when both run in parallel, e.g. "[BRC 7:46 AM SMorning T10]".
func slotTag(slot *booker.TeeTimeSlot) string {
	t := slotutil.FormatCutoffDisplay(slot.TeeTime)
	return fmt.Sprintf("[%s %s S%s T%s]", slot.CourseID, t, slot.Session, slot.TeeBox.String())
}

// slotName is the user-facing slot label, e.g. "7:37 AM tee 10".
func slotName(slot *booker.TeeTimeSlot) string {
	return fmt.Sprintf("%s tee %s", slotutil.FormatCutoffDisplay(slot.TeeTime), slot.TeeBox.String())
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

func bookSlot(client booker.ClientInterface, cfg *Config, slot *booker.TeeTimeSlot) (bookingID string, err error) {
	resp, err := client.BookTeeTime(cfg.Token, booker.GolfNewBooking2Input{
		CourseID:   slot.CourseID,
		TxnDate:    cfg.TxnDate,
		Session:    slot.Session,
		TeeBox:     slot.TeeBox.String(),
		TeeTime:    slot.TeeTime,
		AccountID:  cfg.UserName,
		TotalGuest: 4,
		IPaddress:  cfg.UserName,
		Holes:      18,
	}, false)
	if err != nil {
		return "", err
	}
	if !resp.Status || len(resp.Result) == 0 || !resp.Result[0].Status {
		reason := resp.Reason
		if reason == "" {
			reason = "booking failed"
		}
		if booker.IsInvalidToken(resp.Reason) {
			return "", fmt.Errorf("%s: %w", reason, booker.ErrInvalidToken)
		}
		return "", errors.New(reason)
	}
	return resp.Result[0].BookingID, nil
}
