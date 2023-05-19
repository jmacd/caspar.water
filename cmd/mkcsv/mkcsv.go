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

var listMetrics = []string{
	"well_depth_value",
	"tank_level_value",
}

func main() {
	mapMetrics := map[string]int{}
	for i, m := range listMetrics {
		mapMetrics[m] = i
	}
	if len(os.Args) != 1 {
		log.Fatalf(`program uses standard input and output
usage: %v < input.json > output.csv`, os.Args[0])
	}

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

func output(points [][]*otlp.NumberDataPoint) {
	for idx := range points[0] {
		var line string
		line += fmt.Sprintf("%.3f, ", float64(points[0][idx].TimeUnixNano)/1e9)

		for _, val := range points {
			if len(val) <= idx {
				continue
			}
			dp := val[idx].Value
			if _, ok := dp.(*otlp.NumberDataPoint_AsDouble); ok {
				dv := val[idx].Value.(*otlp.NumberDataPoint_AsDouble)
				line += fmt.Sprintf("%v, ", dv.AsDouble)
			} else {
				dv := val[idx].Value.(*otlp.NumberDataPoint_AsInt)
				line += fmt.Sprintf("%v, ", dv.AsInt)
			}
		}

		fmt.Println(line)
	}
}
