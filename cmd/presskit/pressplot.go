package main

import (
	"bufio"
	"log"
	"math/rand"
	"os"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"

	otlpsvc "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	otlp "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	rand.Seed(int64(0))

	p := plot.New()

	p.Title.Text = "Water pressure"
	p.X.Label.Text = "time"
	p.Y.Label.Text = "psi"

	err := plotutil.AddLinePoints(
		p,
		"pressure", readPoints(),
	)
	if err != nil {
		panic(err)
	}

	// Save the plot to a PNG file.
	if err := p.Save(10*vg.Inch, 10*vg.Inch, "pressure.png"); err != nil {
		panic(err)
	}
}

func readPoints() plotter.XYs {
	var pts plotter.XYs

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg otlpsvc.ExportMetricsServiceRequest
		text := scanner.Text()
		err := protojson.Unmarshal([]byte(text), &msg)
		if err != nil {
			if len(text) > 0 && text[0] == 0 {
				log.Printf("Skipping corrupt line: %q\n", text)
				continue
			}

			log.Fatalf("error in unmarshal: %q: %v", text, err)
		}
		for _, rm := range msg.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name != "water_pressure" {
						continue
					}

					switch t := m.Data.(type) {
					case *otlp.Metric_Gauge:
						for _, p := range t.Gauge.DataPoints {
							pts = append(pts, plotter.XY{
								X: float64(p.GetTimeUnixNano()),
								Y: p.GetAsDouble(),
							})
						}
					default:
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("reading standard input:", err)
	}
	return pts
}
