package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

var port = flag.Int("port", 8888, "metrics port")
var probe = flag.String("probe", "192.168.0.40", "monitor ipv4")

func main() {
	flag.Parse()

	startTime := time.Now()

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {

		pinger, err := probing.NewPinger("www.google.com")
		pinger.SetPrivileged(true)
		if err != nil {
			log.Fatal(err)
		}
		pinger.Count = 3
		err = pinger.Run()
		if err != nil {
			log.Fatal(err)
		}
		stats := pinger.Statistics()

		_, _ = w.Write(
			[]byte(fmt.Sprintf(
				`# HELP uptime 
# TYPE uptime gauge 
uptime %f

# HELP pingtime
# TYPE pingtime gauge
pingtime %f
`,
				time.Since(startTime).Seconds(),
				stats.AvgRtt.Seconds(),
			)))
	})
	if err := http.ListenAndServe(fmt.Sprint(":", *port), nil); err != nil {
		log.Fatal("error serving http: %v", err)
	}
}
