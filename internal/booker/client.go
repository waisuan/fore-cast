package booker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/waisuan/alfred/internal/logger"
)

// ClientInterface is the subset of the booking API used by handlers.
// It allows tests to inject a mock (e.g. via mockgen) instead of the real client.
//
//go:generate mockgen -destination=./mock_client.go -package=booker -source=client.go
type ClientInterface interface {
	Login(userName, password string) (string, error)
	GetTeeTimeSlots(token, courseID, txnDate string) ([]TeeTimeSlot, error)
	GetBooking(token, accountID, bookingID, chitID string) (*GetBookingResponse, error)
	CheckTeeTimeStatus(token string, input GolfCheckTeeTimeStatusInput) (*CheckTeeTimeStatusResponse, error)
	BookTeeTime(token string, input GolfNewBooking2Input, debug bool) (*BookingResponse, error)
}

// Client calls the club booking JSON API. Create with NewClient.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClientOptions configures the HTTP client used for Booker API calls.
type ClientOptions struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

// NewClient returns a client with the given base URL and options.
func NewClient(baseURL string, opts ClientOptions) *Client {
	transport := &http.Transport{
		MaxIdleConns:        opts.MaxIdleConns,
		MaxIdleConnsPerHost: opts.MaxIdleConnsPerHost,
		IdleConnTimeout:     opts.IdleConnTimeout,
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   opts.Timeout,
			Transport: transport,
		},
	}
}

// Login authenticates and returns the session token.
func (c *Client) Login(userName, password string) (token string, err error) {
	req := LoginRequest{
		Type: RequestTypeLogin,
		Input: LoginInput{
			UserName: userName,
			Password: password,
		},
	}
	raw, err := c.do(req, "")
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	var resp LoginResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		if len(raw) > 0 && (raw[0] == '<' || (len(raw) >= 4 && string(raw[:4]) == "<!DO")) {
			return "", fmt.Errorf("login response: server returned HTML instead of JSON (wrong URL, redirect, or error page): %w", err)
		}
		return "", fmt.Errorf("login response: %w", err)
	}
	if !resp.Status || resp.Token == "" {
		return "", fmt.Errorf("login rejected (Status=%v, Token empty)", resp.Status)
	}
	return resp.Token, nil
}

// GetTeeTime returns the raw JSON response body for the GolfGetTeeTime API.
func (c *Client) GetTeeTime(token, courseID, txnDate string) (rawJSON []byte, err error) {
	req := GetTeeTimeRequest{
		Type: RequestTypeTeeTime,
		Input: GetTeeTimeInput{
			CourseID: courseID,
			TxnDate:  txnDate,
		},
	}
	raw, err := c.do(req, token)
	if err != nil {
		return nil, fmt.Errorf("get tee time: %w", err)
	}
	return raw, nil
}

// GetTeeTimeSlots fetches available slots for the given course and date.
func (c *Client) GetTeeTimeSlots(token, courseID, txnDate string) ([]TeeTimeSlot, error) {
	raw, err := c.GetTeeTime(token, courseID, txnDate)
	if err != nil {
		return nil, err
	}
	var resp GetTeeTimeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse tee time response: %w", err)
	}
	if !resp.Status {
		if IsInvalidToken(resp.Reason) {
			return nil, fmt.Errorf("get tee time: %w", ErrInvalidToken)
		}
		return nil, fmt.Errorf("get tee time returned Status=false")
	}
	return resp.Result, nil
}

// BookTeeTime sends a GolfNewBooking2 request. When debug is true, request/response bodies are printed to stderr.
func (c *Client) BookTeeTime(token string, input GolfNewBooking2Input, debug bool) (*BookingResponse, error) {
	req := GolfNewBooking2Request{Type: RequestTypeBooking, Input: input}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal booking request: %w", err)
	}
	if debug {
		logger.Debug("booking request", logger.String("body", string(jsonBody)))
	}
	raw, err := c.doWithBody(jsonBody, token)
	if err != nil {
		return nil, fmt.Errorf("booking request: %w", err)
	}
	if debug {
		logger.Debug("booking response", logger.String("body", string(raw)))
	}
	var resp BookingResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse booking response: %w", err)
	}
	return &resp, nil
}

// GetBooking fetches current booking(s) for the account.
func (c *Client) GetBooking(token, accountID, bookingID, chitID string) (*GetBookingResponse, error) {
	req := GolfGetBookingRequest{
		Type:  RequestTypeGetBooking,
		Input: GolfGetBookingInput{AccountID: accountID, BookingID: bookingID, ChitID: chitID},
	}
	raw, err := c.do(req, token)
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}
	var resp GetBookingResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse get booking response: %w", err)
	}
	return &resp, nil
}

// CheckTeeTimeStatus checks if a slot is valid/available.
func (c *Client) CheckTeeTimeStatus(token string, input GolfCheckTeeTimeStatusInput) (*CheckTeeTimeStatusResponse, error) {
	req := GolfCheckTeeTimeStatusRequest{Type: RequestTypeCheckTeeTimeStatus, Input: input}
	raw, err := c.do(req, token)
	if err != nil {
		return nil, fmt.Errorf("check tee time status: %w", err)
	}
	var resp CheckTeeTimeStatusResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse check tee time status response: %w", err)
	}
	return &resp, nil
}

func (c *Client) doWithBody(jsonBody []byte, token string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	c.setHeaders(req)
	if token != "" {
		req.Header.Set(HeaderToken, token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateBody(raw, 200))
	}
	return raw, nil
}

func (c *Client) do(body interface{}, token string) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	c.setHeaders(req)
	if token != "" {
		req.Header.Set(HeaderToken, token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateBody(raw, 200))
	}
	return raw, nil
}

func truncateBody(b []byte, max int) string {
	s := string(b)
	for len(s) > 0 && (s[0] == '\r' || s[0] == '\n') {
		s = s[1:]
	}
	if len(s) > max {
		s = s[:max] + "..."
	}
	return strings.ReplaceAll(s, "\n", " ")
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", HeaderContentType)
	req.Header.Set("User-Agent", HeaderUserAgent)
	req.Header.Set("Accept", HeaderAccept)
	req.Header.Set("Accept-Language", HeaderAcceptLang)
	req.Header.Set("Accept-Encoding", HeaderAcceptEnc)
	req.Header.Set("version", HeaderVersion)
	req.Header.Set("type", HeaderClientType)
}
