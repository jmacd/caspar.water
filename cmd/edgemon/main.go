package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

var port = flag.Int("port", 8888, "metrics port")
var target = flag.String("target", "192.168.0.40", "monitor target")
var window = flag.Int("window", 600, "size of window in seconds")
var interval = flag.Int("interval", 10, "interval in seconds")

func fmtDur(x time.Duration) string {
	s := x.String()
	if strings.HasSuffix(s, "0s") {
		return s[:len(s)-2]
	}
	return s
}

func main() {
	startTime := time.Now()

	flag.Parse()

	if *window%*interval != 0 {
		log.Fatal("please let interval divide window evenly, thanks")
	}
	size := *window / *interval

	pinger, err := probing.NewPinger(*target)
	if err != nil {
		log.Fatal(fmt.Errorf("new pinger: %w", err))
	}

	var lock sync.Mutex

	sent := 0
	recv := 0
	measurements := make([]time.Duration, size)
	pinger.SetPrivileged(true)
	pinger.Interval = time.Duration(*interval) * time.Second
	pinger.OnSend = func(p *probing.Packet) {
		lock.Lock()
		defer lock.Unlock()

		for ; recv < sent; recv++ {
			measurements[recv%size] = -1
		}
		sent++
	}
	pinger.OnRecv = func(p *probing.Packet) {
		lock.Lock()
		defer lock.Unlock()
		if recv < sent {
			measurements[recv%size] = p.Rtt
		}
		recv++
	}

	go func() {
		if err := pinger.Run(); err != nil {
			log.Fatal(err)
		}
	}()

	ds := fmtDur(time.Duration(*window) * time.Second)
	pingName := fmt.Sprintf("pingtime_%s", ds)
	lossName := fmt.Sprintf("pingloss_%s", ds)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()

		var sum time.Duration
		var lost int
		var have int
		for _, val := range measurements {
			switch {
			case val < 0:
				lost++
			case val == 0:
			default:
				have++
				sum += val
			}
		}
		// Average in seconds
		avgRtt := float64(sum) / float64(have) / 1e9
		lossRatio := float64(lost) / float64(have+lost)

		_, _ = w.Write(
			[]byte(fmt.Sprintf(
				`# TYPE uptime gauge
uptime %f

# TYPE %s gauge
%s %f

# TYPE %s gauge
%s %f
`,
				time.Since(startTime).Seconds(),
				pingName, pingName,
				avgRtt,
				lossName, lossName,
				lossRatio,
			)))
	})
	if err := http.ListenAndServe(fmt.Sprint(":", *port), nil); err != nil {
		log.Fatal("error serving http: %v", err)
	}
}
