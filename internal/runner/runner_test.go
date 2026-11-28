package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/waisuan/alfred/internal/booker"
)

const defaultTestTimeout = 5 * time.Second

func testCtx(t testing.TB, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

func baseCfg(token string) Config {
	return Config{
		UserName:      "user",
		Token:         token,
		TxnDate:       "2026/03/04",
		CourseID:      "PLC",
		CutoffTeeTime: "1899-12-30T08:15:00",
		RetryInterval: time.Millisecond,
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusNoSlots, result.Status)
	assert.Equal(t, cfg.CourseID, result.CourseID)
	assert.Contains(t, err.Error(), "no slots available")
}

func TestRun_BookingFails_UntilTimeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	timeout := 50 * time.Millisecond
	cfg.Waits = Waits{WindowClosed: time.Second, RapidInitial: time.Second}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).MinTimes(1)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil).MinTimes(1)

	result, err := Run(testCtx(t, timeout), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "timeout")
}

func TestRun_Timeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	timeout := 1 * time.Nanosecond
	cfg.RetryInterval = 0

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil).AnyTimes()
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).AnyTimes()
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil).AnyTimes()

	result, err := Run(testCtx(t, timeout), cfg, mock)
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B2", result.BookingID)
	assert.GreaterOrEqual(t, checkCalls, 3)
	assert.GreaterOrEqual(t, bookAttempts, 3)
}

func TestRun_CheckTeeTimeStatusError_SkipsBookThenBooksLaterPass(t *testing.T) {
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B3", result.BookingID)
	assert.Equal(t, checkCalls-2, bookCalls, "no book after a failed check")
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
		Reason: "The flight has already been reserved; extra text",
	}, nil)

	start := time.Now()
	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, cfg.CourseID, result.CourseID)
	assert.Contains(t, result.Message, cfg.CourseID)
	assert.Contains(t, err.Error(), "already reserved")
	assert.Less(t, elapsed, 500*time.Millisecond, "should not sleep between passes when all slots are already reserved")
}

func TestRun_BookingNotOpen_EndsPassWithoutWalkingRestOrBooking(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	timeout := 50 * time.Millisecond
	cfg.Waits = Waits{WindowClosed: time.Second, RapidInitial: time.Second}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:08:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{
		Status: false,
		Reason: "Flight time will be open after 10pm",
	}, nil).Times(1)

	result, err := Run(testCtx(t, timeout), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "timeout")
}

func TestRun_BookingNotOpen_RetriesNextPassThenBooks(t *testing.T) {
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

	checkCalls := 0
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(_ string, in booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			checkCalls++
			if checkCalls == 1 {
				assert.Contains(t, in.TeeTime, "07:00:00")
				return &booker.CheckTeeTimeStatusResponse{
					Status: false,
					Reason: "Flight time will be open after 10pm",
				}, nil
			}
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).Times(2)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-open"}},
	}, nil).Times(1)

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B-open", result.BookingID)
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
				return &booker.CheckTeeTimeStatusResponse{Status: false, Reason: "The flight has already been reserved"}, nil
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B-flight", result.BookingID)
}

func TestRun_RapidAttempts_EndsPassWithoutBooking(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	timeout := 50 * time.Millisecond
	cfg.Waits = Waits{WindowClosed: time.Second, RapidInitial: time.Second}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:08:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{
		Status: false,
		Reason: "Rapid attempts. Your account is temporarily locked.",
	}, nil).Times(1)

	result, err := Run(testCtx(t, timeout), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, err.Error(), "timeout")
}

func TestRun_InvalidToken_RefreshThenBooks(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond
	cfg.RefreshToken = func(context.Context) (string, error) {
		return "tok2", nil
	}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{
		Status: false,
		Reason: "CODE103 - Invalid Token",
	}, nil).Times(1)
	mock.EXPECT().CheckTeeTimeStatus("tok2", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil).Times(1)
	mock.EXPECT().BookTeeTime("tok2", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-refresh"}},
	}, nil).Times(1)

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B-refresh", result.BookingID)
}

func TestRun_InvalidToken_GetSlots_RefreshThenBooks(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RefreshToken = func(context.Context) (string, error) {
		return "tok2", nil
	}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	errInvalid := fmt.Errorf("get tee time: %w", booker.ErrInvalidToken)
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(nil, errInvalid)
	mock.EXPECT().GetTeeTimeSlots("tok2", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok2", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	mock.EXPECT().BookTeeTime("tok2", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-get"}},
	}, nil)

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "B-get", result.BookingID)
}

func TestRun_WindowClosedWait_UsedAfterWindowClosed(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond
	cfg.Waits.WindowClosed = 40 * time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)

	checkCalls := 0
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(string, booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			checkCalls++
			if checkCalls == 1 {
				return &booker.CheckTeeTimeStatusResponse{
					Status: false,
					Reason: "Flight time will be open after 10pm",
				}, nil
			}
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).Times(2)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-cool"}},
	}, nil)

	start := time.Now()
	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
}

func TestRun_RapidBackoff_DoublesThenBooks(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond
	cfg.Waits.RapidInitial = 20 * time.Millisecond
	cfg.Waits.RapidMax = 80 * time.Millisecond

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)

	checkCalls := 0
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(string, booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			checkCalls++
			if checkCalls <= 2 {
				return &booker.CheckTeeTimeStatusResponse{
					Status: false,
					Reason: "Rapid attempts. Your account is temporarily locked.",
				}, nil
			}
			return &booker.CheckTeeTimeStatusResponse{Status: true}, nil
		},
	).Times(3)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-rapid"}},
	}, nil)

	start := time.Now()
	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.GreaterOrEqual(t, elapsed, 60*time.Millisecond, "20ms then 40ms Rapid backoff")
}

func TestRun_RapidBackoff_KeptAcrossTokenRefresh(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.RetryInterval = time.Millisecond
	cfg.Waits.RapidInitial = 2 * time.Millisecond
	cfg.RefreshToken = func(context.Context) (string, error) { return "tok", nil }

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	rapid := &booker.CheckTeeTimeStatusResponse{Reason: "Rapid attempts. Your account is temporarily locked."}
	replies := []*booker.CheckTeeTimeStatusResponse{
		rapid, rapid, {Reason: "CODE103 - Invalid Token"}, rapid, {Status: true},
	}
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).DoAndReturn(
		func(string, booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
			r := replies[0]
			replies = replies[1:]
			return r, nil
		},
	).Times(5)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B-rapid"}},
	}, nil)

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	var waits []string
	for _, s := range result.Steps {
		if strings.HasPrefix(s.Text, "Club rate limit hit") {
			waits = append(waits, s.Text)
		}
	}
	assert.Equal(t, []string{
		"Club rate limit hit; waiting 2ms before trying again.",
		"Club rate limit hit; waiting 4ms before trying again.",
		"Club rate limit hit; waiting 8ms before trying again.",
	}, waits, "a re-login does not reset the rapid backoff")
}

func TestRapidWait_DoublesAndCaps(t *testing.T) {
	t.Parallel()
	for streak, want := range []time.Duration{3, 6, 12, 24, 24} {
		assert.Equal(t, want*time.Second, rapidWait(DefaultWaits, streak), streak)
	}
	assert.Equal(t, time.Duration(0), rapidWait(Waits{}, 3))
}

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := map[string]reply{
		"The flight has already been reserved; kindly refresh the flight":    replyReserved,
		"Rapid attempts detected. Please wait a moment before trying again.": replyRapid,
		"Your account is temporarily locked":                                 replyRapid,
		"Flight time will be open after 10pm":                                replyWindowClosed,
		"Booking is not allowed during this time range":                      replyWindowClosed,
		"This Flight Is Blocked":                                             replyOther,
		"":                                                                   replyOther,
	}
	for reason, want := range cases {
		assert.Equal(t, want, classify(reason), reason)
	}
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

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Message, "session expired")
}

func TestRun_AlreadyCancelled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)

	result, err := Run(ctx, cfg, mock)
	require.Error(t, err)
	assert.Equal(t, StatusCancelled, result.Status)
	assert.Contains(t, result.Message, "cancelled")
}

func TestRun_StepsExplainTimeout(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	cfg.Waits = Waits{CheckToBook: time.Second}

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T07:08:00", Session: "1", TeeBox: booker.StringOrNumber("10")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: false, Reason: "This Flight Is Blocked"}, nil)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Reason: "Tee Time was not reserve"}, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)

	result, err := Run(testCtx(t, 50*time.Millisecond), cfg, mock)
	require.Error(t, err)
	assert.Equal(t, "[PLC] no slot booked — last club reply: Tee Time was not reserve", result.Message)

	var texts []string
	for _, s := range result.Steps {
		texts = append(texts, s.Text)
	}
	assert.Equal(t, []string{
		"PLC on Wed 4 Mar: 2 tee times before your 8:15 AM cutoff.",
		"7:00 AM tee 1: This Flight Is Blocked",
		"7:00 AM tee 1: booking failed: Tee Time was not reserve",
		"7:08 AM tee 10: open.",
		"7:08 AM tee 10: waiting 1s before booking (the club requires it).",
		"7:08 AM tee 10: time ran out before the booking was sent. Try a longer timeout.",
		"Done: nothing booked. [PLC] no slot booked — last club reply: Tee Time was not reserve",
	}, texts)
}

func TestStepLog_CapsAndKeepsSummary(t *testing.T) {
	t.Parallel()
	l := &stepLog{}
	for i := 0; i < maxSteps+5; i++ {
		l.add("step %d", i)
	}
	steps := l.finish("Done.")
	require.Len(t, steps, maxSteps+2)
	assert.Equal(t, "step 0", steps[0].Text)
	assert.Equal(t, "… 5 earlier steps not shown", steps[1].Text)
	assert.Equal(t, "step 6", steps[2].Text)
	assert.Equal(t, fmt.Sprintf("step %d", maxSteps+4), steps[maxSteps].Text)
	assert.Equal(t, "Done.", steps[maxSteps+1].Text)
}

func TestRun_OnStepStreamsEveryStepWithCourse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := booker.NewMockClientInterface(ctrl)
	cfg := baseCfg("tok")
	var live []Step
	cfg.OnStep = func(s Step) { live = append(live, s) }

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{
		Status: true,
		Result: []booker.BookingResultItem{{Status: true, BookingID: "B1"}},
	}, nil)

	result, err := Run(testCtx(t, defaultTestTimeout), cfg, mock)
	require.NoError(t, err)
	assert.Equal(t, result.Steps, live, "live steps match the stored log")
	for _, s := range live {
		assert.Equal(t, "PLC", s.Course)
	}
	assert.Equal(t, "Done: booked.", live[len(live)-1].Text)
}
