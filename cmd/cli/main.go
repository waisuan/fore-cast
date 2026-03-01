package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/runner"
	"github.com/waisuan/alfred/internal/slotutil"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	retryInterval := flag.Int("retry-interval", 1, "Seconds between rounds when using -retry")
	timeout := flag.Duration("timeout", 10*time.Minute, "Max time to spend retrying (0 = no limit)")
	runAt := flag.String("at", "", "Run at this time today (24h, e.g. 22:00 for 10 PM); waits then runs with all other flags")
	debug := flag.Bool("debug", false, "Print booking request/response body for troubleshooting")
	ntfy := flag.String("ntfy", "", "ntfy.sh topic for push notifications on success/failure")
	flag.Usage = usage
	flag.Parse()

	fmt.Println("--- fore-cast ---")
	fmt.Printf("  user:           %s\n", *user)
	fmt.Printf("  date:           %s\n", valueOrDefault(*date, "1 week ahead"))
	fmt.Printf("  cutoff:         %s\n", valueOrDefault(*cutoff, "8:15"))
	if *course != "" {
		fmt.Printf("  course:         %s\n", *course)
	}
	fmt.Printf("  status:         %v\n", *statusOnly)
	fmt.Printf("  slots:          %v\n", *showSlots)
	fmt.Printf("  retry:          %v\n", *retry)
	if *retry {
		fmt.Printf("  retry-interval: %ds\n", *retryInterval)
		fmt.Printf("  timeout:        %s\n", *timeout)
	}
	if *runAt != "" {
		fmt.Printf("  at:             %s\n", *runAt)
	}
	fmt.Printf("  debug:          %v\n", *debug)
	if *ntfy != "" {
		fmt.Printf("  ntfy:           %s\n", *ntfy)
	}
	fmt.Println()

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

	result, err := runner.Run(cfg, client)
	if err != nil {
		_ = notify.Send(*ntfy, "FAILED: "+err.Error())
	} else if result.Message != "" {
		_ = notify.Send(*ntfy, result.Message)
	}
	return err
}

func printSlots(client *booker.Client, token, txnDate, courseID, cutoffTeeTime string) error {
	fmt.Printf("Available slots for %s | Course: %s\n", txnDate, courseID)
	slots, err := client.GetTeeTimeSlots(token, courseID, txnDate)
	if err != nil {
		return fmt.Errorf("get tee times: %w", err)
	}
	if len(slots) == 0 {
		fmt.Println("No slots available.")
		return nil
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].TeeTime < slots[j].TeeTime })
	for _, s := range slots {
		mark := " "
		if s.TeeTime < cutoffTeeTime {
			mark = "*"
		}
		fmt.Printf("  %s %s %s (TeeBox %s) %s\n", mark, s.TeeTime, s.Session, s.TeeBox.String(), s.CourseName)
	}
	fmt.Printf("\n* = before %s cutoff (eligible for booking)\n", slotutil.FormatCutoffDisplay(cutoffTeeTime))
	return nil
}

func printBookingStatus(client *booker.Client, token, accountID string) {
	resp, err := client.GetBooking(token, accountID, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not fetch booking status: %v\n", err)
		return
	}
	fmt.Println("\n--- Current booking status ---")
	if !resp.Status {
		if resp.Reason == "" && len(resp.Result) == 0 {
			fmt.Println("No bookings found.")
			return
		}
		reason := resp.Reason
		if reason == "" {
			reason = "(no reason from API)"
		}
		fmt.Printf("API Status: false (Reason: %s)\n", reason)
		return
	}
	if len(resp.Result) == 0 {
		fmt.Println("No bookings found.")
		return
	}
	for i, b := range resp.Result {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("BookingID: %s\n", b.BookingID)
		fmt.Printf("  Date:    %s\n", b.TxnDate)
		fmt.Printf("  Course:  %s (%s)\n", b.CourseName, b.CourseID)
		fmt.Printf("  Time:    %s  Session: %s  TeeBox: %s\n", b.TeeTime, b.Session, b.TeeBox)
		fmt.Printf("  Pax:     %d  Holes:   %d\n", b.Pax, b.Hole)
		fmt.Printf("  Name:    %s\n", b.Name)
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
	fmt.Printf("Waiting until %s (%s from now)...\n", target.Format("15:04"), d.Round(time.Second))
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
