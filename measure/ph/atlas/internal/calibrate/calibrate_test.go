package calibrate

import (
	"testing"

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
	tdev := mock.NewMockI2C(ctrl)

	ph := ezo.New(tdev)
	require.NotNil(t, ph)

	inter := mymock.NewMockInteractive(ctrl)
	cc := NewCalibration(inter, ph)
	ch1 := make(chan struct{})

	gomock.InOrder(
		tdev.EXPECT().Write("Cal,?").Times(1),
		tdev.EXPECT().Sleep(device.Short).Times(1),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Cal,0")
			return nil
		}).Times(1),

		inter.EXPECT().ReadLine().Return([]byte(""), false, nil).Times(1),

		tdev.EXPECT().Write("Cal,?").Times(1),
		tdev.EXPECT().Sleep(device.Short).Times(1),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Cal,0")
			return nil
		}).Times(1),

		inter.EXPECT().ReadLine().Return([]byte("7.99"), false, nil).Times(1),

		inter.EXPECT().ReadRune().DoAndReturn(func() (rune, int, error) {
			<-ch1
			return '\n', 1, nil
		}).Times(1),

		tdev.EXPECT().Write("RT,15.00").Times(1),
		tdev.EXPECT().Sleep(device.Long).Times(1),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "7.05")
			close(ch1)
			return nil
		}).Times(1),

		tdev.EXPECT().Write("Cal,mid,7.05").Times(1),
		tdev.EXPECT().Sleep(device.Long).Times(1),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}).Times(1),

		// tdev.EXPECT().Write("RT,15.00").Times(1),
		// tdev.EXPECT().Sleep(device.Long).Times(1),
		// tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
		// 	set(d, 1, "7.05")
		// 	return nil
		// }),
	)

	require.NoError(t, cc.Calibrate())
}
