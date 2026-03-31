package booker

import (
	"encoding/json"
	"strconv"
)

const (
	BaseURL           = "http://mobile.saujana.com.my/ClubApp/json/Default.aspx/default.aspx"
	HeaderToken       = "token"
	HeaderContentType = "text/plain"
	HeaderUserAgent   = "saujanagolf/1 CFNetwork/3826.600.41 Darwin/24.6.0"
	HeaderAccept      = "application/json, text/plain, */*"
	HeaderAcceptLang  = "en-GB,en;q=0.9"
	HeaderAcceptEnc   = "gzip, deflate"
	HeaderVersion     = "1.4.0"
	HeaderClientType  = "IOS"
)

const (
	CourseBRC = "BRC"
	CoursePLC = "PLC"
)

const (
	RequestTypeLogin              = "OnlineValidateLogin"
	RequestTypeTeeTime            = "GolfGetTeeTime"
	RequestTypeBooking            = "GolfNewBooking2"
	RequestTypeGetBooking         = "GolfGetBooking"
	RequestTypeCheckTeeTimeStatus = "GolfCheckTeeTimeStatus"
	RequestTypeCancelBooking      = "GolfCancelBooking"
)

// --- Login ---
type LoginRequest struct {
	Type  string     `json:"type"`
	Input LoginInput `json:"Input"`
}

type LoginInput struct {
	UserName string `json:"UserName"`
	Password string `json:"Password"`
}

type LoginResponse struct {
	Status bool   `json:"Status"`
	Token  string `json:"Token"`
}

// --- Get tee time slots ---
type GetTeeTimeRequest struct {
	Type  string          `json:"type"`
	Input GetTeeTimeInput `json:"Input"`
}

type GetTeeTimeInput struct {
	CourseID string `json:"CourseID"`
	TxnDate  string `json:"TxnDate"`
}

// StringOrNumber unmarshals from either a JSON string or number (so TeeBox "10" or 10 both work).
type StringOrNumber string

func (s *StringOrNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = StringOrNumber(str)
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = StringOrNumber(strconv.FormatInt(int64(n), 10))
	return nil
}

func (s StringOrNumber) String() string { return string(s) }

type TeeTimeSlot struct {
	CourseID   string         `json:"CourseID"`
	CourseName string         `json:"CourseName"`
	Flight     int            `json:"Flight"`
	Session    string         `json:"Session"`
	Status     string         `json:"Status"`
	TeeBox     StringOrNumber `json:"TeeBox"`
	TeeTime    string         `json:"TeeTime"`
	TxnDate    string         `json:"TxnDate"`
}

type GetTeeTimeResponse struct {
	Result []TeeTimeSlot `json:"Result,omitempty"`
	Status bool          `json:"Status"`
	Reason string        `json:"Reason,omitempty"`
}

// --- Booking (GolfNewBooking2) ---
type GolfNewBooking2Request struct {
	Type  string               `json:"type"`
	Input GolfNewBooking2Input `json:"Input"`
}

type GolfNewBooking2Input struct {
	CourseID        string `json:"CourseID"`
	TxnDate         string `json:"TxnDate"`
	Session         string `json:"Session"`
	TeeBox          string `json:"TeeBox"`
	TeeTime         string `json:"TeeTime"`
	AccountID       string `json:"AccountID"`
	TotalGuest      int    `json:"TotalGuest"`
	Golfer2MemberID string `json:"Golfer2MemberID"`
	Golfer3MemberID string `json:"Golfer3MemberID"`
	Golfer4MemberID string `json:"Golfer4MemberID"`
	Golfer1Caddy    string `json:"Golfer1Caddy"`
	Golfer2Caddy    string `json:"Golfer2Caddy"`
	Golfer3Caddy    string `json:"Golfer3Caddy"`
	Golfer4Caddy    string `json:"Golfer4Caddy"`
	RequireBuggy    bool   `json:"RequireBuggy"`
	IPaddress       string `json:"IPaddress"`
	Holes           int    `json:"Holes"`
}

type BookingResultItem struct {
	Status    bool   `json:"Status"`
	BookingID string `json:"BookingID"`
}

type BookingResponse struct {
	Status bool                `json:"Status"`
	Reason string              `json:"Reason"`
	Result []BookingResultItem `json:"Result,omitempty"`
}

// --- Get booking ---
type GolfGetBookingRequest struct {
	Type  string              `json:"type"`
	Input GolfGetBookingInput `json:"Input"`
}

type GolfGetBookingInput struct {
	AccountID string `json:"AccountID"`
	BookingID string `json:"BookingID"`
	ChitID    string `json:"ChitID"`
}

type GetBookingResultItem struct {
	BookingID  string `json:"BookingID"`
	TxnDate    string `json:"TxnDate"`
	CourseID   string `json:"CourseID"`
	CourseName string `json:"CourseName"`
	TeeTime    string `json:"TeeTime"`
	Session    string `json:"Session"`
	TeeBox     string `json:"TeeBox"`
	Pax        int    `json:"Pax"`
	Hole       int    `json:"Hole"`
	Name       string `json:"Name"`
}

type GetBookingResponse struct {
	Status bool                   `json:"Status"`
	Reason string                 `json:"Reason"`
	Result []GetBookingResultItem `json:"Result,omitempty"`
}

// --- Cancel booking ---
type GolfCancelBookingRequest struct {
	Type  string                 `json:"type"`
	Input GolfCancelBookingInput `json:"Input"`
}

type GolfCancelBookingInput struct {
	BookingID string `json:"BookingID"`
	AccountID string `json:"AccountID"`
}

type GolfCancelBookingResponse struct {
	Status bool   `json:"Status"`
	Reason string `json:"Reason,omitempty"`
}

// --- Check tee time status ---
type GolfCheckTeeTimeStatusRequest struct {
	Type  string                      `json:"type"`
	Input GolfCheckTeeTimeStatusInput `json:"Input"`
}

type GolfCheckTeeTimeStatusInput struct {
	CourseID  string `json:"CourseID"`
	TxnDate   string `json:"TxnDate"`
	Session   string `json:"Session"`
	TeeBox    string `json:"TeeBox"`
	TeeTime   string `json:"TeeTime"`
	UserName  string `json:"UserName"`
	IPAddress string `json:"IPAddress"`
	Action    int    `json:"Action"`
}

type CheckTeeTimeStatusResponse struct {
	Status bool        `json:"Status"`
	Reason string      `json:"Reason"`
	Result interface{} `json:"Result,omitempty"`
}
