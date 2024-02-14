package calibrate

import (
	"fmt"
	"testing"
	"time"

	mymock "github.com/jmacd/caspar.water/measure/ph/atlas/internal/calibrate/mock"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device/mock"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/ezo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func set(d []byte, n byte, s string) {
	b := []byte(s)
	d[0] = n
	copy(d[1:1+len(b)], b)
}

func TestCalibrate(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2CStringer(ctrl)

	ph := ezo.New(tdev)
	require.NotNil(t, ph)

	inter := mymock.NewMockInteractive(ctrl)
	cc := NewCalibration(inter, ph)

	gomock.InOrder(
		// reads calibration temperature,
		inter.EXPECT().ReadLine().Return([]byte(""), false, nil).Times(1),

		// reads mid-point pH reference value
		inter.EXPECT().ReadLine().Return([]byte("7.01"), false, nil).Times(1),

		// reads low-point pH reference value
		inter.EXPECT().ReadLine().Return([]byte("4.02"), false, nil).Times(1),

		// reads high-point pH reference value
		inter.EXPECT().ReadLine().Return([]byte("10.03"), false, nil).Times(1),
	)

	// reads keystrokes
	inter.EXPECT().ReadRune().DoAndReturn(func() (rune, int, error) {
		return '\n', 1, nil
	}).AnyTimes()

	cnt := 0
	tdev.EXPECT().WriteSleepRead("Cal,?", device.Short).DoAndReturn(func(_ string, _ time.Duration) (byte, string, error) {
		num := max(0, cnt-1)
		cnt++
		return device.StatusOK, fmt.Sprint("?Cal,", num), nil
	}).AnyTimes()

	tdev.EXPECT().WriteSleepRead("Cal,mid,7.01", device.Long).Return(device.StatusOK, "", nil).Times(1)
	tdev.EXPECT().WriteSleepRead("Cal,low,4.02", device.Long).Return(device.StatusOK, "", nil).Times(1)
	tdev.EXPECT().WriteSleepRead("Cal,high,10.03", device.Long).Return(device.StatusOK, "", nil).Times(1)

	tdev.EXPECT().WriteSleepRead("RT,15.00", device.Long).Return(device.StatusOK, "7.25", nil).MinTimes(9)
	tdev.EXPECT().WriteSleepRead("Slope,?", device.Short).Return(device.StatusOK, "?Slope,99.9,100.1,-0.5", nil).Times(1)

	require.NoError(t, cc.Calibrate())
}
