package device

import (
	"fmt"
	"strings"
	"time"
)

type Fake struct {
	CloseErr    error
	Version     string
	Vcc         string
	RestartCode string
	Name        string
	Points      int

	nextRead string
	sleepFor time.Duration
}

var (
	noData          = []byte{254}
	stillProcessing = []byte{255}
	syntaxError     = []byte{2}
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
	} else if f.sleepFor > 0 {
		copy(dest[:1], stillProcessing)
	} else {
		copy(dest[:len(f.nextRead)], f.nextRead)
		f.nextRead = ""
	}
	return nil
}

func ok(txt string) string {
	return string(append(okReady, []byte(txt)...))
}

var justOk = ok("")

func panicIf(x bool) {
	if x {
		panic("unexpected fake logic")
	}
}

func (f *Fake) Write(src []byte) error {
	cmd, rest, split := strings.Cut(string(src), ",")
	switch strings.ToLower(cmd) {
	case "i":
		panicIf(split)
		f.nextRead = ok("?I,pH," + f.Version)
		f.sleepFor = Short
	case "status":
		panicIf(split)
		f.nextRead = ok("?Status," + f.RestartCode + "," + f.Vcc)
		f.sleepFor = Short
	case "name":
		panicIf(!split)
		if rest == "?" {
			f.nextRead = ok("?Name," + f.Name)
		} else if rest == "" {
			f.Name = ""
			f.nextRead = justOk
		} else {
			f.Name = rest
			f.nextRead = justOk
		}
		f.sleepFor = Short
	case "cal":
		panicIf(!split)
		switch {
		case strings.HasPrefix(rest, "mid,"):
			f.nextRead = justOk
			f.Points = 1
			f.sleepFor = Long
		case strings.HasPrefix(rest, "low,"):
			f.nextRead = justOk
			f.Points = 2
			f.sleepFor = Long
		case strings.HasPrefix(rest, "high,"):
			f.nextRead = justOk
			f.Points = 3
			f.sleepFor = Long
		case rest == "clear":
			f.nextRead = justOk
			f.Points = 0
			f.sleepFor = Short
		case rest == "?":
			f.nextRead = ok(fmt.Sprint("?Cal,", f.Points))
			f.sleepFor = Short
		default:
			panic("invalid case")
		}
	default:
		panic("untested")
	}
	return nil
}

func (f *Fake) Sleep(d time.Duration) {
	f.sleepFor -= d
	f.sleepFor = max(0, f.sleepFor)
}
