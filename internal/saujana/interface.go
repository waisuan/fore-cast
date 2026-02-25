package saujana

// ClientInterface is the subset of the Saujana API used by API handlers.
// It allows tests to inject a mock (e.g. via mockgen) instead of the real client.
//
//go:generate mockgen -destination=./mock_client.go -package=saujana -source=interface.go
type ClientInterface interface {
	Login(userName, password string) (string, error)
	GetTeeTimeSlots(token, courseID, txnDate string) ([]TeeTimeSlot, error)
	GetBooking(token, accountID, bookingID, chitID string) (*GetBookingResponse, error)
	CheckTeeTimeStatus(token string, input GolfCheckTeeTimeStatusInput) (*CheckTeeTimeStatusResponse, error)
	BookTeeTime(token string, input GolfNewBooking2Input, debug bool) (*BookingResponse, error)
}
