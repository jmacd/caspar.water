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

	gomock.InOrder(
		tdev.EXPECT().Write([]byte("Cal,?")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Cal,0")
			return nil
		}),

		inter.EXPECT().ReadLine().Return([]byte(""), false, nil),

		tdev.EXPECT().Write([]byte("Cal,?")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Cal,0")
			return nil
		}),

		inter.EXPECT().ReadLine().Return([]byte("7.10"), false, nil),
	)

	ch1 := make(chan struct{})
	tdev.EXPECT().Write([]byte("RT,15.00"))
	tdev.EXPECT().Sleep(device.Long)
	tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
		set(d, 1, "7.10")
		close(ch1)
		return nil
	})

	inter.EXPECT().ReadRune().DoAndReturn(func() (rune, int, error) {
		<-ch1
		return '\n', 1, nil
	})

	require.NoError(t, cc.Calibrate())
}
