package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	otlpsvc "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
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
			var msg otlpsvc.ExportLogsServiceRequest
			text := scanner.Text()
			err := protojson.Unmarshal([]byte(text), &msg)
			if err != nil {
				if len(text) > 0 && text[0] == 0 {
					log.Printf("Skipping corrupt line: %q\n", text)
					continue
				}

				log.Fatalf("error in unmarshal: %q: %v", text, err)
			}
			for _, rm := range msg.ResourceLogs {
				for _, sm := range rm.ScopeLogs {
					for _, m := range sm.LogRecords {
						fmt.Println(m.Body.Value.(*common.AnyValue_StringValue).StringValue)
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal("reading standard input:", err)
		}
	}
}
