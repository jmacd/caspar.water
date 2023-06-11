package main

import (
	"log"

	"github.com/jmacd/caspar.water/measure/bme280"
)

func main() {
	dev, err := bme280.New("/dev/i2c-5", 0x77, bme280.StandardAccuracy)

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read()
	if err != nil {
		log.Fatal("read: %w", err)
	}

	log.Println("Temp=", comp.T)
}
