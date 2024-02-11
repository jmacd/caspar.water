package ezo

import (
	"testing"

	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func set(d []byte, n byte, s string) {
	b := []byte(s)
	d[0] = n
	copy(d[1:1+len(b)], b)
}

func TestPhInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2C(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().Write([]byte("i")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?i,pH,1.23")
			return nil
		}))

	info, err := ph.Info()
	require.NoError(t, err)
	require.Equal(t, "1.23", info.Version)
}

func TestPhStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2C(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().Write([]byte("Status")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Status,P,3.83")
			return nil
		}))

	stat, err := ph.Status()
	require.NoError(t, err)
	require.Equal(t, stat.Vcc, 3.83)
	require.Equal(t, stat.Restart, "Powered off")
}

func TestPhName(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2C(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().Write([]byte("Name,probe")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}),
		tdev.EXPECT().Write([]byte("Name,?")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Name,probe")
			return nil
		}),
	)

	err := ph.SetName("probe")
	require.NoError(t, err)

	name, err := ph.Name()
	require.NoError(t, err)
	require.Equal(t, "probe", name)
}

func TestCalibration(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2C(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().Write([]byte("Cal,?")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "?Cal,0")
			return nil
		}),

		tdev.EXPECT().Write([]byte("Cal,mid,7.01")),
		tdev.EXPECT().Sleep(device.Long),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}),

		tdev.EXPECT().Write([]byte("Cal,low,4.02")),
		tdev.EXPECT().Sleep(device.Long),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}),

		tdev.EXPECT().Write([]byte("Cal,high,10.03")),
		tdev.EXPECT().Sleep(device.Long),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}),

		tdev.EXPECT().Write([]byte("Cal,clear")),
		tdev.EXPECT().Sleep(device.Short),
		tdev.EXPECT().Read(gomock.Any()).DoAndReturn(func(d []byte) error {
			set(d, 1, "")
			return nil
		}),
	)

	pts, err := ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 0, pts)

	err = ph.CalibrateMidpoint(7.01)
	require.NoError(t, err)

	err = ph.CalibrateLowpoint(4.02)
	require.NoError(t, err)

	err = ph.CalibrateHighpoint(10.03)
	require.NoError(t, err)

	err = ph.ClearCalibration()
	require.NoError(t, err)
}
