package device

import (
	"strings"
	"time"
)

type Fake struct {
	CloseErr    error
	Version     string
	Vcc         string
	RestartCode string

	nextRead string
	sleepFor time.Duration
}

var (
	noData          = []byte{254}
	stillProcessing = []byte{255}
	okReady         = []byte{1}
)

func (f *Fake) Close() error {
	return f.CloseErr
}

func (f *Fake) Read(dest []byte) error {
	for i := range dest {
		dest[i] = 0
	}
	if f.nextRead == "" {
		copy(dest[:1], noData)
		return nil
	}
	if f.sleepFor > 0 {
		copy(dest[:1], stillProcessing)
		return nil
	}
	copy(dest[0:len(okReady)], okReady)
	copy(dest[len(okReady):len(okReady)+len(f.nextRead)], f.nextRead)
	f.nextRead = ""
	return nil
}

func (f *Fake) Write(src []byte) error {
	cmd, _, _ := strings.Cut(string(src), ",")
	switch strings.ToLower(cmd) {
	case "i":
		f.nextRead = "?I,pH," + f.Version
		f.sleepFor = Short
	case "status":
		f.nextRead = "?Status," + f.RestartCode + "," + f.Vcc
		f.sleepFor = Short
	default:
		panic("untested")
	}
	return nil
}

func (f *Fake) Sleep(d time.Duration) {
	f.sleepFor -= d
	f.sleepFor = max(0, f.sleepFor)
}

// type Test struct {
// 	command   string
// 	Name      string
// 	Vcc       float64
// 	ResetCode string
// }
