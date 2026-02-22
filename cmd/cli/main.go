package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/saujana"
	"github.com/waisuan/alfred/internal/slotutil"
)

// Default interval between retries when -retry is used.
const defaultRetryIntervalSec = 5

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	user := flag.String("user", "", "Club member ID / username (e.g. M8816-0)")
	pass := flag.String("password", "", "Password")
	date := flag.String("date", "", "Target date YYYY/MM/DD (default: 1 week from today)")
	cutoff := flag.String("cutoff", "", "Only book slots before this time, e.g. 8:15 or 7:30 (default: 8:15)")
	statusOnly := flag.Bool("status", false, "Only show current booking status (no booking)")
	showSlots := flag.Bool("slots", false, "Show available tee time slots for the given date (no booking)")
	testSlot := flag.String("test-slot", "", "Preferred tee time to try first each round (e.g. 1899-12-30T07:37:00); also used with -test-only to show API response only")
	testTeeBox := flag.String("test-teebox", "1", "TeeBox to use with -test-slot (e.g. 1 or 10)")
	testOnly := flag.Bool("test-only", false, "With -test-slot: only run one test booking and show API response (no retry loop)")
	retry := flag.Bool("retry", false, "Retry loop: keep trying to book until a slot is booked or none available before cutoff (e.g. for 9:50–10 PM window)")
	retryInterval := flag.Int("retry-interval", defaultRetryIntervalSec, "Seconds between rounds when using -retry")
	runAt := flag.String("at", "", "Run at this time today (24h, e.g. 22:00 for 10 PM); waits then runs with all other flags")
	debug := flag.Bool("debug", false, "Print booking request/response body for troubleshooting")
	flag.Usage = usage
	flag.Parse()

	if *runAt != "" {
		if err := waitUntil(*runAt); err != nil {
			return err
		}
	}

	userName := or(*user, os.Getenv("SAUJANA_USER"))
	password := or(*pass, os.Getenv("SAUJANA_PASSWORD"))
	if userName == "" || password == "" {
		return fmt.Errorf("credentials required: set -user and -password, or SAUJANA_USER and SAUJANA_PASSWORD")
	}

	client := saujana.NewClient()
	token, err := client.Login(userName, password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if *statusOnly {
		printBookingStatus(client, token, userName)
		return nil
	}

	if *testSlot != "" && *testOnly {
		return runTestSlot(client, token, userName, *testSlot, *testTeeBox, *date, *debug)
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

	if *showSlots {
		return printSlots(client, token, txnDate, cutoffTeeTime)
	}

	defer printBookingStatus(client, token, userName)
	// When -retry is false, single round only (one attempt per slot then exit). When true, loop until booked or no slots.
	// If -test-slot is set (and not -test-only), try that slot first each round before API slots.
	return runRetryLoop(client, token, txnDate, userName, cutoffTeeTime, *retryInterval, !*retry, *debug, *testSlot, *testTeeBox)
}

// tryBookSlot attempts to book one slot. Returns (true, bookingID, nil) on success, (false, "", _) on failure.
func tryBookSlot(client *saujana.Client, token string, slot *saujana.TeeTimeSlot, txnDate, userName string, debug bool) (booked bool, bookingID string, err error) {
	fmt.Printf("Attempting to book at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	input := saujana.GolfNewBooking2Input{
		CourseID:        slot.CourseID,
		TxnDate:         txnDate,
		Session:         slot.Session,
		TeeBox:          slot.TeeBox.String(),
		TeeTime:         slot.TeeTime,
		AccountID:       userName,
		TotalGuest:      4,
		Golfer2MemberID: "",
		Golfer3MemberID: "",
		Golfer4MemberID: "",
		Golfer1Caddy:    "",
		Golfer2Caddy:    "",
		Golfer3Caddy:    "",
		Golfer4Caddy:    "",
		RequireBuggy:    false,
		IPaddress:       userName,
		Holes:           18,
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
func printTeeTimeStatus(client *saujana.Client, token string, slot *saujana.TeeTimeSlot, txnDate, userName string) {
	checkInput := saujana.GolfCheckTeeTimeStatusInput{
		CourseID:  slot.CourseID,
		TxnDate:   txnDate,
		Session:   slot.Session,
		TeeBox:    slot.TeeBox.String(),
		TeeTime:   slot.TeeTime,
		UserName:  userName,
		IPAddress: userName,
		Action:    0,
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

// runRetryLoop fetches slots; for each slot checks tee time status then tries to book once.
// If singleRound is true, runs one round then returns (with error if nothing booked). Otherwise loops until booked or no slots before cutoff.
// When preferredTeeTime is non-empty, only that slot is retried until booked or the user cancels (no API slot fallback).
func runRetryLoop(client *saujana.Client, token, txnDate, userName, cutoffTeeTime string, intervalSec int, singleRound, debug bool, preferredTeeTime, preferredTeeBox string) error {
	courseID := slotutil.CourseForDate(txnDate)
	preferredTeeTime = strings.TrimSpace(preferredTeeTime)
	preferredTeeBox = strings.TrimSpace(preferredTeeBox)
	if preferredTeeBox == "" {
		preferredTeeBox = "1"
	}
	// Preferred slot only: check status then try to book; repeat until booked or user cancels (Ctrl+C).
	if preferredTeeTime != "" {
		prefSlot := &saujana.TeeTimeSlot{
			CourseID:   courseID,
			CourseName: "",
			Session:    "Morning",
			TeeBox:     saujana.StringOrNumber(preferredTeeBox),
			TeeTime:    preferredTeeTime,
		}
		round := 0
		for {
			round++
			fmt.Printf("Target date: %s | Course: %s (round %d)\n", txnDate, courseID, round)
			fmt.Printf("Preferred slot: %s Morning (TeeBox %s)\n", preferredTeeTime, preferredTeeBox)
			printTeeTimeStatus(client, token, prefSlot, txnDate, userName)
			booked, bookingID, _ := tryBookSlot(client, token, prefSlot, txnDate, userName, debug)
			if booked {
				fmt.Printf("Booked successfully. BookingID: %s\n", bookingID)
				return nil
			}
			fmt.Printf("Retrying in %d seconds... (Ctrl+C to cancel)\n", intervalSec)
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}
	// No preferred slot: fetch API slots and try each before cutoff.
	round := 0
	for {
		round++
		if singleRound && round > 1 {
			break
		}
		fmt.Printf("Target date: %s | Course: %s", txnDate, courseID)
		if !singleRound {
			fmt.Printf(" (round %d)", round)
		}
		fmt.Println()
		slots, err := client.GetTeeTimeSlots(token, courseID, txnDate)
		if err != nil {
			if singleRound {
				return fmt.Errorf("get tee times: %w", err)
			}
			fmt.Fprintf(os.Stderr, "[round %d] get tee times: %v\n", round, err)
			time.Sleep(time.Duration(intervalSec) * time.Second)
			continue
		}
		slotsBeforeCutoff := slotutil.SlotsBeforeCutoff(slots, cutoffTeeTime)
		if len(slotsBeforeCutoff) == 0 {
			return fmt.Errorf("earliest available slot is at or after %s (cutoff). No booking made", slotutil.FormatCutoffDisplay(cutoffTeeTime))
		}
		for si := range slotsBeforeCutoff {
			slot := &slotsBeforeCutoff[si]
			fmt.Printf("Slot: %s %s (TeeBox %s)\n", slot.TeeTime, slot.Session, slot.TeeBox.String())
			printTeeTimeStatus(client, token, slot, txnDate, userName)
			booked, bookingID, _ := tryBookSlot(client, token, slot, txnDate, userName, debug)
			if booked {
				fmt.Printf("Booked successfully. BookingID: %s\n", bookingID)
				return nil
			}
		}
		if singleRound {
			return fmt.Errorf("could not book any slot before %s (tried %d)", slotutil.FormatCutoffDisplay(cutoffTeeTime), len(slotsBeforeCutoff))
		}
		fmt.Printf("No slot booked this round. Retrying in %d seconds...\n", intervalSec)
		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
	// Unreachable when singleRound is false (loop never exits without return).
	return nil
}

// printSlots fetches and prints available tee time slots for the date (no booking).
func printSlots(client *saujana.Client, token, txnDate, cutoffTeeTime string) error {
	courseID := slotutil.CourseForDate(txnDate)
	fmt.Printf("Available slots for %s | Course: %s\n", txnDate, courseID)
	slots, err := client.GetTeeTimeSlots(token, courseID, txnDate)
	if err != nil {
		return fmt.Errorf("get tee times: %w", err)
	}
	if len(slots) == 0 {
		fmt.Println("No slots available.")
		return nil
	}
	sorted := make([]saujana.TeeTimeSlot, len(slots))
	copy(sorted, slots)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].TeeTime < sorted[j].TeeTime })
	for _, s := range sorted {
		beforeCutoff := s.TeeTime < cutoffTeeTime
		mark := " "
		if beforeCutoff {
			mark = "*"
		}
		fmt.Printf("  %s %s %s (TeeBox %s) %s\n", mark, s.TeeTime, s.Session, s.TeeBox.String(), s.CourseName)
	}
	fmt.Printf("\n* = before %s cutoff (eligible for booking)\n", slotutil.FormatCutoffDisplay(cutoffTeeTime))
	return nil
}

// runTestSlot attempts one booking with the given TeeTime (e.g. non-existent slot) and prints the API response.
func runTestSlot(client *saujana.Client, token, userName, teeTime, teeBox, dateFlag string, debug bool) error {
	txnDate := strings.TrimSpace(dateFlag)
	if txnDate == "" {
		txnDate = slotutil.DateOneWeekAhead()
	}
	if err := slotutil.ValidateDate(txnDate); err != nil {
		return err
	}
	courseID := slotutil.CourseForDate(txnDate)
	teeBox = strings.TrimSpace(teeBox)
	if teeBox == "" {
		teeBox = "1"
	}

	input := saujana.GolfNewBooking2Input{
		CourseID:        courseID,
		TxnDate:         txnDate,
		Session:         "Morning",
		TeeBox:          teeBox,
		TeeTime:         teeTime,
		AccountID:       userName,
		TotalGuest:      4,
		Golfer2MemberID: "",
		Golfer3MemberID: "",
		Golfer4MemberID: "",
		Golfer1Caddy:    "",
		Golfer2Caddy:    "",
		Golfer3Caddy:    "",
		Golfer4Caddy:    "",
		RequireBuggy:    false,
		IPaddress:       userName,
		Holes:           18,
	}
	fmt.Printf("Test booking: %s on %s @ %s (TeeBox %s, Morning)\n", courseID, txnDate, teeTime, teeBox)
	fmt.Printf("Attempting to book at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	resp, err := client.BookTeeTime(token, input, debug)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	out := map[string]interface{}{
		"Status": resp.Status,
		"Reason": resp.Reason,
		"Result": resp.Result,
	}
	jsonOut, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println("Booking response:")
	fmt.Println(string(jsonOut))

	checkInput := saujana.GolfCheckTeeTimeStatusInput{
		CourseID:  courseID,
		TxnDate:   txnDate,
		Session:   "Morning",
		TeeBox:    teeBox,
		TeeTime:   teeTime,
		UserName:  userName,
		IPAddress: userName,
		Action:    0,
	}
	statusResp, err := client.CheckTeeTimeStatus(token, checkInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tee time status check failed: %v\n", err)
		return nil
	}
	statusOut := map[string]interface{}{
		"Status": statusResp.Status,
		"Reason": statusResp.Reason,
		"Result": statusResp.Result,
	}
	statusJSON, _ := json.MarshalIndent(statusOut, "", "  ")
	fmt.Println("\nTee time status response:")
	fmt.Println(string(statusJSON))
	return nil
}

// printBookingStatus fetches and prints current booking(s) for the account.
func printBookingStatus(client *saujana.Client, token, accountID string) {
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

// waitUntil blocks until the given time today (24h, e.g. "22:00"). If the time has passed, returns immediately.
func waitUntil(at string) error {
	at = strings.TrimSpace(at)
	if at == "" {
		return nil
	}
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

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [options]

Books the earliest available tee time (before cutoff, default 8:15 AM; use -cutoff to override) at Saujana Club.
Target date defaults to 1 week from today. Course is chosen by day:
  Mon/Tue/Sun → BRC (Bunga Raya), Wed–Sat → PLC.

Use -status to view current booking status. Use -slots to list available tee
times for the date (no booking). Use -test-slot <TeeTime> to attempt one
booking and print the API response. Use -retry to loop until a slot is booked or none are available before cutoff.
For each slot, tee time status is checked first, then one booking attempt is made.
Use -at 22:00 to wait until that time today (24h) then run with all other flags.

Credentials: -user and -password, or SAUJANA_USER and SAUJANA_PASSWORD.

Options:
`, os.Args[0])
	flag.PrintDefaults()
}
