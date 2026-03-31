package booker

import "strings"

// Dry-run scenario constants (used as BOOKER_DRY_RUN_SCENARIO env values).
const (
	DryRunScenarioSuccess = "success" // Book succeeds
	DryRunScenarioTimeout = "timeout" // Slots exist but book fails, retries until timeout
	DryRunScenarioEmpty   = "empty"   // No slots, immediate exit
)

// DryRunClient implements ClientInterface for scheduler dry-run testing.
// No real HTTP calls are made. Behaviour depends on scenario.
type DryRunClient struct {
	scenario string
}

// NewDryRunClient returns a mock client for the given scenario.
// Scenarios: DryRunScenarioSuccess, DryRunScenarioTimeout, DryRunScenarioEmpty.
func NewDryRunClient(scenario string) *DryRunClient {
	s := strings.ToLower(strings.TrimSpace(scenario))
	if s == "" {
		s = DryRunScenarioTimeout
	}
	switch s {
	case DryRunScenarioSuccess, DryRunScenarioTimeout, DryRunScenarioEmpty:
	default:
		s = DryRunScenarioTimeout
	}
	return &DryRunClient{scenario: s}
}

func (c *DryRunClient) Login(userName, password string) (string, error) {
	return "dry-run-token", nil
}

func (c *DryRunClient) GetTeeTimeSlots(token, courseID, txnDate string) ([]TeeTimeSlot, error) {
	switch c.scenario {
	case DryRunScenarioEmpty:
		return []TeeTimeSlot{}, nil
	case DryRunScenarioSuccess, DryRunScenarioTimeout:
		// Slot before default cutoff 08:15
		return []TeeTimeSlot{
			{
				CourseID:   courseID,
				CourseName: courseID,
				Session:    "1",
				TeeBox:     StringOrNumber("1"),
				TeeTime:    "1899-12-30T07:30:00",
				TxnDate:    txnDate,
			},
		}, nil
	default:
		return []TeeTimeSlot{
			{CourseID: courseID, CourseName: courseID, Session: "1", TeeBox: StringOrNumber("1"),
				TeeTime: "1899-12-30T07:30:00", TxnDate: txnDate},
		}, nil
	}
}

func (c *DryRunClient) GetBooking(token, accountID, bookingID, chitID string) (*GetBookingResponse, error) {
	return &GetBookingResponse{Status: true, Result: []GetBookingResultItem{}}, nil
}

func (c *DryRunClient) CancelBooking(token, accountID, bookingID string) (*GolfCancelBookingResponse, error) {
	return &GolfCancelBookingResponse{Status: true}, nil
}

func (c *DryRunClient) CheckTeeTimeStatus(token string, input GolfCheckTeeTimeStatusInput) (*CheckTeeTimeStatusResponse, error) {
	return &CheckTeeTimeStatusResponse{Status: true}, nil
}

func (c *DryRunClient) BookTeeTime(token string, input GolfNewBooking2Input, debug bool) (*BookingResponse, error) {
	switch c.scenario {
	case DryRunScenarioSuccess:
		return &BookingResponse{
			Status: true,
			Result: []BookingResultItem{{Status: true, BookingID: "dry-run-BK001"}},
		}, nil
	case DryRunScenarioTimeout, DryRunScenarioEmpty:
		// Book fails so runner retries (timeout) or never gets here (empty)
		return &BookingResponse{Status: false, Reason: "dry-run: slot unavailable"}, nil
	default:
		return &BookingResponse{Status: false}, nil
	}
}
