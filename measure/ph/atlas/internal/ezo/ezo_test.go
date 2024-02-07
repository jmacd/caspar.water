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
