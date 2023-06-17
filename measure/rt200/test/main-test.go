package main

import (
	"log"

	"github.com/jmacd/caspar.water/measure/rt200"
)

func main() {
	dev, err := rt200.New()

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read()
	if err != nil {
		log.Fatal("read: %w", err)
	}

	log.Println("Temp=", comp.P)
}
