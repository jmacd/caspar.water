package main

import (
	"log"

	"github.com/jmacd/caspar.water/measure/modbus"
)

func main() {
	dev, err := modbus.New()

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read()
	if err != nil {
		log.Fatal("read: %w", err)
	}

	log.Println("Temp=", comp.P)
}
