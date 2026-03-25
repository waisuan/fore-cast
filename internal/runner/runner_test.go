package runner

import (
	"fmt"
	"strings"
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
		RetryInterval: time.Millisecond,
		Debug:         false,
		Timeout:       5 * time.Second,
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

func TestRun_Success_0746SlotRunsCheckThenBook(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:46:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
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

func TestRun_BookingFails_OnePass(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Timeout = 0

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
	cfg.Timeout = 1 * time.Nanosecond
	cfg.RetryInterval = 0

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
	cfg.RetryInterval = time.Millisecond

	errInvalid := fmt.Errorf("get tee time: %w", booker.ErrInvalidToken)
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(nil, errInvalid).Times(1)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "session expired")
	assert.True(t, assert.ErrorIs(t, err, booker.ErrInvalidToken))
}

func TestRun_Retry_SucceedsAfterPasses(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil).Times(1)

	checkCalls := 0
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(string, booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			checkCalls++
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).AnyTimes()

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
	assert.GreaterOrEqual(t, checkCalls, 3)
	assert.GreaterOrEqual(t, bookAttempts, 3)
}

func TestRun_CheckTeeTimeStatusError_ThenBookSucceedsAcrossPasses(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil).Times(1)

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

	bookCalls := 0
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).DoAndReturn(
		func(string, booker.GolfNewBooking2Input, bool) (*booker.BookingResponse, error) {
			bookCalls++
			if bookCalls < 3 {
				return &booker.BookingResponse{Status: false}, nil
			}
			return &booker.BookingResponse{
				Status: true,
				Result: []booker.BookingResultItem{{Status: true, BookingID: "B3"}},
			}, nil
		},
	).AnyTimes()

	result, err := Run(cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B3", result.BookingID)
	assert.GreaterOrEqual(t, checkCalls, 3)
	assert.GreaterOrEqual(t, bookCalls, 3)
}

func TestRun_AllSlotsFlightAlreadyReserved_ExitsWithoutRetrySleep(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Second

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{
		Status: false,
		Reason: flightAlreadyReservedPhrase + "; extra text",
	}, nil)

	start := time.Now()
	result, err := Run(cfg, mock)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "already reserved")
	assert.Less(t, elapsed, 500*time.Millisecond, "should not sleep between passes when all slots are already reserved")
}

func TestRun_FlightAlreadyReserved_SkipsSlot_SecondSlotBooks(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:08:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(_ string, in booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			if strings.Contains(in.TeeTime, "07:00:00") {
				return &booker.CheckTeeTimeStatusResponse{Status: false, Reason: flightAlreadyReservedPhrase}, nil
			}
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).AnyTimes()
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).DoAndReturn(
		func(_ string, in booker.GolfNewBooking2Input, _ bool) (*booker.BookingResponse, error) {
			assert.Contains(t, in.TeeTime, "07:08")
			return &booker.BookingResponse{
				Status: true,
				Result: []booker.BookingResultItem{{Status: true, BookingID: "B-flight"}},
			}, nil
		},
	).Times(1)

	result, err := Run(cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B-flight", result.BookingID)
}

func TestRun_InvalidToken_FromCheck_FailsFast(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{
		Status: false,
		Reason: "CODE103 - Invalid Token",
	}, nil)

	result, err := Run(cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "session expired")
}
