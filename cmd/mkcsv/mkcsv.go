package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	otlpsvc "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	otlp "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// This is a one-off script to extract well depth measurements to a
// CSV file to be used for hydrological analysis.

var listMetrics = []string{
	"well_depth_value",
}

func main() {
	mapMetrics := map[string]int{}
	for i, m := range listMetrics {
		mapMetrics[m] = i
	}
	if len(os.Args) <= 1 {
		log.Fatalf(`program uses file inputs and standard output
usage: %v input.json ... > output.csv`, os.Args[0])
	}
	for _, arg := range os.Args[1:] {
		f, err := os.Open(arg)
		if err != nil {
			log.Fatalf(`open: %s: %v`, arg, err)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var msg otlpsvc.ExportMetricsServiceRequest
			text := scanner.Text()
			err := protojson.Unmarshal([]byte(text), &msg)
			if err != nil {
				if len(text) > 0 && text[0] == 0 {
					log.Printf("Skipping corrupt line: %q\n", text)
					continue
				}

				log.Printf("error in unmarshal: %q: %v", text, err)
				continue
			}
			items := make([][]*otlp.NumberDataPoint, len(mapMetrics))
			for _, rm := range msg.ResourceMetrics {
				for _, sm := range rm.ScopeMetrics {
					for _, m := range sm.Metrics {
						idx, ok := mapMetrics[m.Name]
						if !ok {
							continue
						}

						switch t := m.Data.(type) {
						case *otlp.Metric_Gauge:
							for _, p := range t.Gauge.DataPoints {
								items[idx] = append(items[idx], p)
							}
						default:
						}
					}
				}
			}
			output(items)
		}
		if err := scanner.Err(); err != nil {
			log.Fatal("reading standard input:", err)
		}
	}
}

func output(points [][]*otlp.NumberDataPoint) {
	for idx := range points[0] {
		var vals []string
		vals = append(vals, time.Unix(0, int64(points[0][idx].TimeUnixNano)).Format(time.RFC3339))

		for _, val := range points {
			if len(val) <= idx {
				continue
			}
			dp := val[idx].Value
			if _, ok := dp.(*otlp.NumberDataPoint_AsDouble); ok {
				dv := val[idx].Value.(*otlp.NumberDataPoint_AsDouble)
				vals = append(vals, fmt.Sprintf("%v", dv.AsDouble))
			} else {
				dv := val[idx].Value.(*otlp.NumberDataPoint_AsInt)
				vals = append(vals, fmt.Sprintf("%v", dv.AsInt))
			}
		}

		fmt.Println(strings.Join(vals, ","))
	}
}
