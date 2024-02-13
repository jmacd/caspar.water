package calibrate

import (
	"fmt"
	"testing"

	mymock "github.com/jmacd/caspar.water/measure/ph/atlas/internal/calibrate/mock"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device/mock"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/ezo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func set(d []byte, n byte, s string) {
	fmt.Println("SET RESP", s)
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
		fmt.Println("HI!!!")
		return '\n', 1, nil
	}).AnyTimes()

	// at least three times
	tdev.EXPECT().WriteSleepRead("RT,15.00", device.Long).Return(device.StatusOK, "7.25", nil).MinTimes(3)

	gomock.InOrder(
		// calibration count: twice 0, then 1, then 2
		tdev.EXPECT().WriteSleepRead("Cal,?", device.Short).Return(device.StatusOK, "?Cal,0", nil).Times(2),
		tdev.EXPECT().WriteSleepRead("Cal,?", device.Short).Return(device.StatusOK, "?Cal,1", nil).Times(1),
		tdev.EXPECT().WriteSleepRead("Cal,?", device.Short).Return(device.StatusOK, "?Cal,2", nil).Times(1),
	)

	gomock.InOrder(
		tdev.EXPECT().WriteSleepRead("Cal,mid,7.01", device.Long).Return(device.StatusOK, "", nil).Times(1),
		tdev.EXPECT().WriteSleepRead("Cal,low,4.02", device.Long).Return(device.StatusOK, "", nil).Times(1),
		tdev.EXPECT().WriteSleepRead("Cal,high,10.03", device.Long).Return(device.StatusOK, "", nil).Times(1),
	)

	require.NoError(t, cc.Calibrate())
}
