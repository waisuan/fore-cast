package runner

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/waisuan/alfred/internal/booker"
)

func baseCfg(token string) Config {
	return Config{
		UserName:      "user",
		Token:         token,
		TxnDate:       "2026/03/04",
		CourseID:      "PLC",
		CutoffTeeTime: "1899-12-30T08:15:00",
		RetryInterval: time.Second,
		Retry:         false,
		Debug:         false,
		Timeout:       10 * time.Second,
	}
}

func TestRun_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B1"}},
	}, nil)

	result, err := Run(cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B1", result.BookingID)
	assert.Contains(t, result.Message, "Booked")
}

func TestRun_NoSlotsBeforeCutoff(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T09:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusNoSlots, result.Status)
	assert.Contains(t, err.Error(), "no slots available")
}

func TestRun_BookingFails_NoRetry(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "no slots booked")
}

func TestRun_Timeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Retry = true
	cfg.Timeout = 1 * time.Nanosecond
	cfg.RetryInterval = 0 // no sleep for fast timeout test

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil).AnyTimes()
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).AnyTimes()
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil).AnyTimes()

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "timeout")
}

func TestRun_GetSlotsError_NoRetry(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")

	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(nil, assert.AnError)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
}

func TestRun_InvalidToken_FailsFast(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Retry = true
	cfg.Timeout = 5 * time.Second
	cfg.RetryInterval = time.Millisecond

	errInvalid := fmt.Errorf("get tee time: %w", booker.ErrInvalidToken)
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(nil, errInvalid).Times(1)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "session expired")
	assert.True(t, assert.ErrorIs(t, err, booker.ErrInvalidToken))
}

func TestRun_MaxParallelSlots_RespectsConfig(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.MaxParallelSlots = 2

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:08:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:16:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).Times(2)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil).Times(2)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
}

func TestRun_Retry_SucceedsAfterRetries(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Retry = true
	cfg.Timeout = 5 * time.Second
	cfg.RetryInterval = time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil).AnyTimes()
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).AnyTimes()

	bookAttempts := 0
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).DoAndReturn(
		func(string, booker.GolfNewBooking2Input, bool) (*booker.BookingResponse, error) {
			bookAttempts++
			if bookAttempts < 3 {
				return &booker.BookingResponse{Status: false}, nil
			}
			return &booker.BookingResponse{
				Status: true,
				Result: []booker.BookingResultItem{{Status: true, BookingID: "B2"}},
			}, nil
		},
	).AnyTimes()

	result, err := Run(cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B2", result.BookingID)
	assert.GreaterOrEqual(t, bookAttempts, 3)
}

func TestRun_CheckTeeTimeStatusError_SkipsBookingAndRetries(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Retry = true
	cfg.Timeout = 100 * time.Millisecond
	cfg.RetryInterval = 10 * time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)

	// CheckTeeTimeStatus fails twice, then succeeds; BookTeeTime succeeds on first real attempt
	checkCalls := 0
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(string, booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			checkCalls++
			if checkCalls < 3 {
				return nil, assert.AnError
			}
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).AnyTimes()
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B3"}},
	}, nil).Times(1)

	result, err := Run(cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B3", result.BookingID)
	assert.GreaterOrEqual(t, checkCalls, 3, "CheckTeeTimeStatus should have been retried after errors")
}

func TestResetTimer(t *testing.T) {
	t.Parallel()
	// Verify resetTimer doesn't panic when timer has fired
	timer := time.NewTimer(time.Millisecond)
	<-timer.C
	resetTimer(timer, time.Second)
	timer.Stop()

	// Verify resetTimer works when timer was stopped
	timer2 := time.NewTimer(time.Hour)
	timer2.Stop()
	resetTimer(timer2, time.Millisecond)
	<-timer2.C
}
