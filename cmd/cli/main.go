package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/runner"
	"github.com/waisuan/alfred/internal/slotutil"
)

func main() {
	logger.Init()
	defer logger.Sync()

	if err := run(); err != nil {
		logger.Error("cli failed", logger.Err(err))
		os.Exit(1)
	}
}

func run() error {
	user := flag.String("user", "", "Club member ID / username")
	pass := flag.String("password", "", "Password")
	date := flag.String("date", "", "Target date YYYY/MM/DD (default: 1 week from today)")
	cutoff := flag.String("cutoff", "", "Only book slots before this time, e.g. 8:15 or 7:30 (default: 8:15)")
	course := flag.String("course", "", "Override course selection (BRC or PLC; default: auto by day of week)")
	statusOnly := flag.Bool("status", false, "Only show current booking status (no booking)")
	showSlots := flag.Bool("slots", false, "Show available tee time slots for the given date (no booking)")
	retry := flag.Bool("retry", false, "Retry loop: keep trying to book until a slot is booked or none available before cutoff")
	retryInterval := flag.Duration("retry-interval", time.Second, "Pause between rounds when using -retry (e.g. 1s, 500ms)")
	timeout := flag.Duration("timeout", 10*time.Minute, "Max time to spend retrying (0 = no limit)")
	runAt := flag.String("at", "", "Run at this time today (24h, e.g. 22:00 for 10 PM); waits then runs with all other flags")
	debug := flag.Bool("debug", false, "Print booking request/response body for troubleshooting")
	ntfy := flag.String("ntfy", "", "ntfy.sh topic for push notifications on success/failure")
	flag.Usage = usage
	flag.Parse()

	logger.Info("cli config",
		logger.String("user", *user),
		logger.String("date", valueOrDefault(*date, "1 week ahead")),
		logger.String("cutoff", valueOrDefault(*cutoff, "8:15")),
		logger.String("course", valueOrDefault(*course, "")),
		logger.Bool("status", *statusOnly),
		logger.Bool("slots", *showSlots),
		logger.Bool("retry", *retry),
		logger.Duration("retry_interval", *retryInterval),
		logger.Duration("timeout", *timeout),
		logger.String("at", *runAt),
		logger.Bool("debug", *debug),
		logger.String("ntfy", *ntfy))

	if *runAt != "" {
		if err := waitUntil(*runAt); err != nil {
			return err
		}
	}

	if *user == "" || *pass == "" {
		return fmt.Errorf("-user and -password are required")
	}

	client := booker.NewClient()
	token, err := client.Login(*user, *pass)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if *statusOnly {
		printBookingStatus(client, token, *user)
		return nil
	}

	txnDate := strings.TrimSpace(*date)
	if txnDate == "" {
		txnDate = slotutil.DateOneWeekAhead()
	}
	if err := slotutil.ValidateDate(txnDate); err != nil {
		return err
	}

	cutoffTeeTime, err := slotutil.ParseCutoff(*cutoff)
	if err != nil {
		return err
	}

	courseID := strings.TrimSpace(strings.ToUpper(*course))
	if courseID == "" {
		courseID = slotutil.CourseForDate(txnDate)
	}

	if *showSlots {
		return printSlots(client, token, txnDate, courseID, cutoffTeeTime)
	}

	defer printBookingStatus(client, token, *user)

	cfg := runner.Config{
		UserName:      *user,
		Token:         token,
		TxnDate:       txnDate,
		CourseID:      courseID,
		CutoffTeeTime: cutoffTeeTime,
		RetryInterval: *retryInterval,
		Retry:         *retry,
		Debug:         *debug,
		Timeout:       *timeout,
	}

	ntfySvc := notify.NewService("https://ntfy.sh", 10*time.Second)
	result, err := runner.Run(cfg, client)
	if err != nil {
		_ = ntfySvc.Send(*ntfy, "FAILED: "+err.Error())
	} else if result.Message != "" {
		_ = ntfySvc.Send(*ntfy, result.Message)
	}
	return err
}

func printSlots(client *booker.Client, token, txnDate, courseID, cutoffTeeTime string) error {
	logger.Info("available slots", logger.String("txn_date", txnDate), logger.String("course", courseID))
	slots, err := client.GetTeeTimeSlots(token, courseID, txnDate)
	if err != nil {
		return fmt.Errorf("get tee times: %w", err)
	}
	if len(slots) == 0 {
		logger.Info("no slots available")
		return nil
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].TeeTime < slots[j].TeeTime })
	for _, s := range slots {
		eligible := s.TeeTime < cutoffTeeTime
		logger.Info("slot",
			logger.Bool("eligible", eligible),
			logger.String("tee_time", s.TeeTime),
			logger.String("session", s.Session),
			logger.String("tee_box", s.TeeBox.String()),
			logger.String("course_name", s.CourseName))
	}
	logger.Info("cutoff info", logger.String("cutoff", slotutil.FormatCutoffDisplay(cutoffTeeTime)))
	return nil
}

func printBookingStatus(client *booker.Client, token, accountID string) {
	resp, err := client.GetBooking(token, accountID, "", "")
	if err != nil {
		logger.Error("failed to fetch booking status", logger.Err(err))
		return
	}
	if !resp.Status {
		if resp.Reason == "" && len(resp.Result) == 0 {
			logger.Info("no bookings found")
			return
		}
		reason := resp.Reason
		if reason == "" {
			reason = "(no reason from API)"
		}
		logger.Warn("api status false", logger.String("reason", reason))
		return
	}
	if len(resp.Result) == 0 {
		logger.Info("no bookings found")
		return
	}
	for _, b := range resp.Result {
		logger.Info("booking",
			logger.String("booking_id", b.BookingID),
			logger.String("date", b.TxnDate),
			logger.String("course_name", b.CourseName),
			logger.String("course_id", b.CourseID),
			logger.String("tee_time", b.TeeTime),
			logger.String("session", b.Session),
			logger.String("tee_box", b.TeeBox),
			logger.Int("pax", b.Pax),
			logger.Int("holes", b.Hole),
			logger.String("name", b.Name))
	}
}

func waitUntil(at string) error {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return fmt.Errorf("invalid -at %q: use 24h (e.g. 22:00)", at)
	}
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if target.Before(now) || target.Equal(now) {
		return nil
	}
	d := time.Until(target)
	logger.Info("waiting until run time", logger.Time("target", target), logger.Duration("duration", d.Round(time.Second)))
	time.Sleep(d)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [options]

Books the earliest available tee time (before cutoff, default 8:15 AM; use -cutoff to override).
Target date defaults to 1 week from today. Course is chosen by day:
  Mon/Tue/Sun → BRC, Wed–Sat → PLC.

Options:
`, os.Args[0])
	flag.PrintDefaults()
}

func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
