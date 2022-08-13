package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	otlpsvc "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	otlp "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// This is a one-off script to extract well depth measurements to a
// CSV file to be used for hydrological analysis.

const singleMetric = "well_depth_value"

func main() {
	if len(os.Args) != 1 {
		log.Fatalf(`program uses standard input and output
usage: %v < input.json > output.csv`, os.Args[0])
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg otlpsvc.ExportMetricsServiceRequest
		err := protojson.Unmarshal([]byte(scanner.Text()), &msg)
		if err != nil {
			log.Fatal("error in unmarshal:", err)
		}
		for _, rm := range msg.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name != singleMetric {
						continue
					}

					switch t := m.Data.(type) {
					case *otlp.Metric_Gauge:
						for _, p := range t.Gauge.DataPoints {
							output(p)
						}
					default:
						continue
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("reading standard input:", err)
	}
}

func output(p *otlp.NumberDataPoint) {
	dv := p.Value.(*otlp.NumberDataPoint_AsDouble)
	_, err := os.Stdout.WriteString(
		fmt.Sprintf("%.3f, %v\n", float64(p.TimeUnixNano)/1e9, dv.AsDouble),
	)
	if err != nil {
		log.Fatal("error writing output:", err)
	}
}
