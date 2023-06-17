package main

import (
	"log"

	"github.com/jmacd/caspar.water/display/openlcd"
)

func main() {
	lcd, err := openlcd.New("/dev/i2c-5", 0x72)

	if err != nil {
		log.Fatal("open: %w", err)
	}

	err = lcd.Update("hello world")
	if err != nil {
		log.Fatal("update: %w", err)
	}
}
