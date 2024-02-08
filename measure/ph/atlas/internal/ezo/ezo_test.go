package ezo

import (
	"fmt"
	"testing"

	"github.com/jmacd/caspar.water/measure/ph/atlas/internal/device"
	"github.com/stretchr/testify/require"
)

func TestPhInfo(t *testing.T) {
	tdev := &device.Fake{
		Version: "1.23",
	}
	ph := New(tdev)
	require.NotNil(t, ph)

	info, err := ph.Info()
	require.NoError(t, err)
	require.Equal(t, info.Version, tdev.Version)
}

func TestPhStatus(t *testing.T) {
	tdev := &device.Fake{
		Vcc:         "3.45",
		RestartCode: "P",
	}
	ph := New(tdev)
	require.NotNil(t, ph)

	stat, err := ph.Status()
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(stat.Vcc), tdev.Vcc)
	require.Equal(t, stat.Restart, "Powered off")
}

func TestPhName(t *testing.T) {
	tdev := &device.Fake{
		Name: "test",
	}
	ph := New(tdev)
	require.NotNil(t, ph)

	name, err := ph.Name()
	require.NoError(t, err)
	require.Equal(t, "test", name)

	err = ph.SetName("probe")
	require.NoError(t, err)

	name, err = ph.Name()
	require.NoError(t, err)
	require.Equal(t, "probe", name)
}

func TestCalibration(t *testing.T) {
	tdev := &device.Fake{}
	ph := New(tdev)
	require.NotNil(t, ph)

	pts, err := ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 0, pts)

	err = ph.CalibrateMidpoint(7.01)
	require.NoError(t, err)
	pts, err = ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 1, pts)

	err = ph.CalibrateLowpoint(4.02)
	require.NoError(t, err)
	pts, err = ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 2, pts)

	err = ph.CalibrateHighpoint(10.03)
	require.NoError(t, err)
	pts, err = ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 3, pts)

	err = ph.ClearCalibration()
	require.NoError(t, err)
	pts, err = ph.CalibrationPoints()
	require.NoError(t, err)
	require.Equal(t, 0, pts)
}
