package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/slotutil"
)

const defaultRetryIntervalSec = 5


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
	retryInterval := flag.Int("retry-interval", defaultRetryIntervalSec, "Seconds between rounds when using -retry")
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
	msg, err := runRetryLoop(client, token, txnDate, *user, courseID, cutoffTeeTime, *retryInterval, *retry, *debug, *timeout)
	if err != nil {
		notify(*ntfy, "FAILED: "+err.Error())
	} else if msg != "" {
		notify(*ntfy, msg)
	}
	return err
}

// tryBookSlot attempts to book one slot. Returns (true, bookingID, nil) on success, (false, "", nil) on failure.
func tryBookSlot(client *booker.Client, token string, slot *booker.TeeTimeSlot, txnDate, userName string, debug bool) (booked bool, bookingID string, err error) {
	fmt.Printf("Attempting to book at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	input := booker.GolfNewBooking2Input{
		CourseID:   slot.CourseID,
		TxnDate:    txnDate,
		Session:    slot.Session,
		TeeBox:     slot.TeeBox.String(),
		TeeTime:    slot.TeeTime,
		AccountID:  userName,
		TotalGuest: 4,
		IPaddress:  userName,
		Holes:      18,
	}
	resp, err := client.BookTeeTime(token, input, debug)
	if err != nil {
		return false, "", err
	}
	if !resp.Status || len(resp.Result) == 0 || !resp.Result[0].Status {
		return false, "", nil
	}
	return true, resp.Result[0].BookingID, nil
}

// printTeeTimeStatus calls GolfCheckTeeTimeStatus for the slot and prints the response.
func printTeeTimeStatus(client *booker.Client, token string, slot *booker.TeeTimeSlot, txnDate, userName string) {
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

// runRetryLoop fetches slots and attempts to book them.
// When retry is true, loops until a slot is booked, none remain before cutoff, or timeout is reached.
func runRetryLoop(client *booker.Client, token, txnDate, userName, courseID, cutoffTeeTime string, intervalSec int, retry, debug bool, timeout time.Duration) (string, error) {
	start := time.Now()

	for round := 1; ; round++ {
		if !retry && round > 1 {
			break
		}
		if retry && timeout > 0 && time.Since(start) >= timeout {
			return "", fmt.Errorf("timeout after %s with no booking", timeout)
		}
		fmt.Printf("Target date: %s | Course: %s", txnDate, courseID)
		if retry {
			fmt.Printf(" (round %d)", round)
		}
		fmt.Println()
		slots, err := client.GetTeeTimeSlots(token, courseID, txnDate)
		if err != nil {
			if !retry {
				return "", fmt.Errorf("get tee times: %w", err)
			}
			fmt.Fprintf(os.Stderr, "[round %d] get tee times: %v\n", round, err)
			time.Sleep(time.Duration(intervalSec) * time.Second)
			continue
		}
		slotsBeforeCutoff := slotutil.SlotsBeforeCutoff(slots, cutoffTeeTime)
		if len(slotsBeforeCutoff) == 0 {
			return "", fmt.Errorf("earliest available slot is at or after %s (cutoff). No booking made", slotutil.FormatCutoffDisplay(cutoffTeeTime))
		}
		for si := range slotsBeforeCutoff {
			slot := &slotsBeforeCutoff[si]
			fmt.Printf("Slot: %s %s (TeeBox %s)\n", slot.TeeTime, slot.Session, slot.TeeBox.String())
			printTeeTimeStatus(client, token, slot, txnDate, userName)
			booked, bookingID, _ := tryBookSlot(client, token, slot, txnDate, userName, debug)
			if booked {
				msg := fmt.Sprintf("Booked %s %s (TeeBox %s) on %s [%s]. BookingID: %s", slot.TeeTime, slot.Session, slot.TeeBox.String(), txnDate, courseID, bookingID)
				fmt.Println(msg)
				return msg, nil
			}
		}
		if !retry {
			return "", fmt.Errorf("could not book any slot before %s (tried %d)", slotutil.FormatCutoffDisplay(cutoffTeeTime), len(slotsBeforeCutoff))
		}
		fmt.Printf("No slot booked this round. Retrying in %d seconds...\n", intervalSec)
		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
	return "", nil
}

// printSlots fetches and prints available tee time slots for the date (no booking).
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

// printBookingStatus fetches and prints current booking(s) for the account.
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

func notify(topic, msg string) {
	if topic == "" {
		return
	}
	url := "https://ntfy.sh/" + topic
	resp, err := http.Post(url, "text/plain", strings.NewReader(msg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ntfy: %v\n", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("ntfy: notified topic %s\n", topic)
}
