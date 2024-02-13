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
	tdev := mock.NewMockI2CStringer(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	tdev.EXPECT().WriteSleepRead("i", device.Short).Return(device.StatusOK, "?i,pH,1.23", nil)

	info, err := ph.Info()
	require.NoError(t, err)
	require.Equal(t, "1.23", info.Version)
}

func TestPhStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2CStringer(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	tdev.EXPECT().WriteSleepRead("Status", device.Short).Return(device.StatusOK, "?Status,P,3.83", nil)

	stat, err := ph.Status()
	require.NoError(t, err)
	require.Equal(t, stat.Vcc, 3.83)
	require.Equal(t, stat.Restart, "Powered off")
}

func TestPhName(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2CStringer(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().WriteSleepRead("Name,probe", device.Short).Return(device.StatusOK, "", nil),
		tdev.EXPECT().WriteSleepRead("Name,?", device.Short).Return(device.StatusOK, "?Name,probe", nil),
	)

	err := ph.SetName("probe")
	require.NoError(t, err)

	name, err := ph.Name()
	require.NoError(t, err)
	require.Equal(t, "probe", name)
}

func TestCalibration(t *testing.T) {
	ctrl := gomock.NewController(t)
	tdev := mock.NewMockI2CStringer(ctrl)

	ph := New(tdev)
	require.NotNil(t, ph)

	gomock.InOrder(
		tdev.EXPECT().WriteSleepRead("Cal,?", device.Short).Return(device.StatusOK, "?Cal,0", nil),
		tdev.EXPECT().WriteSleepRead("Cal,mid,7.01", device.Long).Return(device.StatusOK, "", nil),
		tdev.EXPECT().WriteSleepRead("Cal,low,4.02", device.Long).Return(device.StatusOK, "", nil),
		tdev.EXPECT().WriteSleepRead("Cal,high,10.03", device.Long).Return(device.StatusOK, "", nil),
		tdev.EXPECT().WriteSleepRead("Cal,clear", device.Short).Return(device.StatusOK, "", nil),
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
