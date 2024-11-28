package main

import (
	"context"
	"log"
	"time"

	"github.com/jmacd/caspar.water/measure/modbus"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	cfg := modbus.Config{
		URL:      "rtu:///dev/ttyUSB0",
		Interval: time.Minute,
		Baud:     9600,
		DataBits: 8,
		StopBits: 1,
		Parity:   "none",
		Timeout:  time.Second * 5,
	}
	
	dev, err := modbus.New(
		&cfg,
		[]modbus.Attribute{},
		[]modbus.Metric{
			{
				Field: modbus.Field{
					Name:  "energy_usage",
					Base:  1,
					Type:  "uint32",
					Range: "holding",
				},
				Unit: "kWh",
			},
		},
		logger,
	)

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read(context.Background())
	if err != nil {
		log.Fatal("read: %w", err)
	}

	for _, a := range comp.A {
		log.Println("A=", a)
	}
	for _, m := range comp.M {
		log.Println("M=", m)
	}
}
