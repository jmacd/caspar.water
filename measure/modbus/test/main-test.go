package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/simonvetter/modbus"
	"go.uber.org/zap"
)

// Orenco registers scraped from web interface
// Formula: Modbus base = (PointNum * 2) + 999 for float32 (A type)
//
//	Modbus base = 40000 + PointNum for int/digital (D type)
var orencoRegisters = []struct {
	pointNum int
	name     string
	ptype    string // A=analog, D=digital, L=label, R=log record
	unit     string
}{
	// R (Log Record) - read full 20-register record
	{12001, "LogRec1", "R", ""},
	{12021, "LogRec2", "R", ""},
	{12041, "LogRec3", "R", ""},
}

func main() {
	// Command line flags
	url := flag.String("url", "tcp://192.168.70.237:502", "Modbus URL (tcp://host:port or rtu:///dev/ttyUSB0)")
	timeout := flag.Duration("timeout", 2*time.Second, "Request timeout")
	gap := flag.Duration("gap", 15*time.Second, "Gap between register reads (manufacturer requirement)")
	loop := flag.Bool("loop", false, "Loop continuously")
	interval := flag.Duration("interval", 60*time.Second, "Loop interval between full measurement cycles")
	verbose := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	// Setup logger
	var logger *zap.Logger
	var err error
	if *verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	fmt.Printf("Orenco Modbus Test\n")
	fmt.Printf("==================\n")
	fmt.Printf("URL:      %s\n", *url)
	fmt.Printf("Timeout:  %s\n", *timeout)
	fmt.Printf("Gap:      %s (between register reads)\n", *gap)

	// Count A and D registers
	var aCount, dCount int
	for _, r := range orencoRegisters {
		switch r.ptype {
		case "A":
			aCount++
		case "D":
			dCount++
		}
	}
	fmt.Printf("Registers: %d total (%d Analog, %d Digital)\n", len(orencoRegisters), aCount, dCount)
	fmt.Println()

	// Create client config (we'll connect/disconnect for each read)
	clientConfig := &modbus.ClientConfiguration{
		URL:     *url,
		Timeout: *timeout,
	}

	readAllRegisters := func() error {
		fmt.Printf("[%s] Starting measurement cycle...\n", time.Now().Format("15:04:05"))

		for i, reg := range orencoRegisters {
			if i > 0 {
				fmt.Printf("  Waiting %s before next read...\n", *gap)
				time.Sleep(*gap)
			}

			// Connect fresh for each read (Orenco closes connection after each request)
			client, err := modbus.NewClient(clientConfig)
			if err != nil {
				fmt.Printf("  ERROR creating client: %v\n", err)
				continue
			}
			if err := client.Open(); err != nil {
				fmt.Printf("  ERROR connecting: %v\n", err)
				continue
			}

			switch reg.ptype {
			case "L": // Label - try reading as float32 at base (pointNum*2+999)
				base := uint16((reg.pointNum * 2) + 999)
				fmt.Printf("  [L] Reading %s (Pt%d):\n", reg.name, reg.pointNum)

				// Try as float32 at float base
				fmt.Printf("       float32 @ %d: ", base)
				valF, errF := client.ReadFloat32(base, modbus.HOLDING_REGISTER)
				if errF != nil {
					fmt.Printf("ERROR: %v\n", errF)
				} else {
					fmt.Printf("%.4f\n", valF)
				}

				// Try as 2x uint16 at float base (to see raw bytes)
				fmt.Printf("       uint16s @ %d,%d: ", base, base+1)
				valU1, errU1 := client.ReadRegister(base, modbus.HOLDING_REGISTER)
				valU2, errU2 := client.ReadRegister(base+1, modbus.HOLDING_REGISTER)
				if errU1 != nil || errU2 != nil {
					fmt.Printf("ERROR: %v / %v\n", errU1, errU2)
				} else {
					fmt.Printf("%d, %d (0x%04x, 0x%04x)\n", valU1, valU2, valU1, valU2)
				}

				// Try as uint16 at int base (40000+pointNum)
				baseInt := uint16(40000 + reg.pointNum)
				fmt.Printf("       uint16 @ %d: ", baseInt)
				valI, errI := client.ReadRegister(baseInt, modbus.HOLDING_REGISTER)
				if errI != nil {
					fmt.Printf("ERROR: %v\n", errI)
				} else {
					fmt.Printf("%d (0x%04x)\n", valI, valI)
				}

				client.Close()
				logger.Info("L type read", zap.String("register", reg.name))

			case "R": // Log Record - read 20 registers
				base := uint16(reg.pointNum)
				fmt.Printf("  [R] Reading %s (base: %d):\n", reg.name, base)

				// Read timestamp: Month, Day, Year, Hour, Minute, Second
				fmt.Printf("       Timestamp: ")
				month, _ := client.ReadRegister(base, modbus.HOLDING_REGISTER)
				day, _ := client.ReadRegister(base+1, modbus.HOLDING_REGISTER)
				year, _ := client.ReadRegister(base+2, modbus.HOLDING_REGISTER)
				hour, _ := client.ReadRegister(base+3, modbus.HOLDING_REGISTER)
				minute, _ := client.ReadRegister(base+4, modbus.HOLDING_REGISTER)
				second, _ := client.ReadRegister(base+5, modbus.HOLDING_REGISTER)
				fmt.Printf("%d/%d/%d %02d:%02d:%02d\n", month, day, year, hour, minute, second)

				// Read 4 integers at +6, +7, +8, +9
				fmt.Printf("       Integers: ")
				for i := uint16(6); i <= 9; i++ {
					val, _ := client.ReadRegister(base+i, modbus.HOLDING_REGISTER)
					fmt.Printf("[%d]=%d ", i, val)
				}
				fmt.Println()

				// Read 4 floats at +10, +12, +14, +16
				fmt.Printf("       Floats: ")
				for i := uint16(10); i <= 16; i += 2 {
					val, _ := client.ReadFloat32(base+i, modbus.HOLDING_REGISTER)
					fmt.Printf("[%d]=%.4f ", i, val)
				}
				fmt.Println()

				// Read AM/PM and Hour12
				ampm, _ := client.ReadRegister(base+18, modbus.HOLDING_REGISTER)
				hour12, _ := client.ReadRegister(base+19, modbus.HOLDING_REGISTER)
				ampmStr := "AM"
				if ampm == 1 {
					ampmStr = "PM"
				}
				fmt.Printf("       12hr: %d:%02d:%02d %s\n", hour12, minute, second, ampmStr)

				client.Close()
				logger.Info("Log record read", zap.String("record", reg.name))

			case "A": // Analog - read as float32
				base := uint16((reg.pointNum * 2) + 999)
				fmt.Printf("  [A] Reading %s (Pt%d, base: %d)... ", reg.name, reg.pointNum, base)
				val, err := client.ReadFloat32(base, modbus.HOLDING_REGISTER)
				client.Close()
				if err != nil {
					fmt.Printf("ERROR: %v\n", err)
					logger.Error("read failed", zap.String("register", reg.name), zap.Error(err))
					continue
				}
				if reg.unit != "" {
					fmt.Printf("%.4f %s\n", val, reg.unit)
				} else {
					fmt.Printf("%.4f\n", val)
				}
				logger.Info("read success", zap.String("register", reg.name), zap.Float32("value", val))

			case "D": // Digital - read as uint16
				base := uint16(40000 + reg.pointNum)
				fmt.Printf("  [D] Reading %s (Pt%d, base: %d)... ", reg.name, reg.pointNum, base)
				val, err := client.ReadRegister(base, modbus.HOLDING_REGISTER)
				client.Close()
				if err != nil {
					fmt.Printf("ERROR: %v\n", err)
					logger.Error("read failed", zap.String("register", reg.name), zap.Error(err))
					continue
				}
				if reg.unit == "O/F" {
					if val == 0 {
						fmt.Printf("OFF (0)\n")
					} else {
						fmt.Printf("ON (%d)\n", val)
					}
				} else {
					fmt.Printf("%d\n", val)
				}
				logger.Info("read success", zap.String("register", reg.name), zap.Uint16("value", val))
			}
		}

		fmt.Printf("[%s] Measurement cycle complete.\n\n", time.Now().Format("15:04:05"))
		return nil
	}

	// First read
	if err := readAllRegisters(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	// Loop if requested
	if *loop {
		fmt.Printf("Looping every %s (Ctrl+C to stop)...\n", *interval)
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := readAllRegisters(); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
		}
	}
}
