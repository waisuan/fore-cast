package runner

import (
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
		RetryInterval: 1,
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
	cfg.RetryInterval = 0

	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	mock.EXPECT().GetTeeTimeSlots("tok", "PLC", "2026/03/04").Return(slots, nil)
	mock.EXPECT().CheckTeeTimeStatus("tok", gomock.Any()).Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	mock.EXPECT().BookTeeTime("tok", gomock.Any(), false).Return(&booker.BookingResponse{Status: false}, nil)

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
