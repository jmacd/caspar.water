package main

import (
	"log"

	"github.com/jmacd/caspar.water/measure/ph/atlas"
)

func main() {
	dev, err := atlas.New("/dev/i2c-2", 0x63)

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read()
	if err != nil {
		log.Fatal("read: %w", err)
	}

	log.Println("Ph=", comp.Ph)
}
