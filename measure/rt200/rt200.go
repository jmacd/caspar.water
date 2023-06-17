package rt200

import (
	"fmt"
	"math"
	"time"

	"github.com/simonvetter/modbus"
)

type RT200 struct {
}

// Measurements contains compensated measurement values.
type Measurements struct {
	P float64
}

func New() (*RT200, error) {
	err := test()
	if err != nil {
		panic(err)
	}
	return &RT200{}, nil
}

func (rt *RT200) Close() error {
	return nil
}

func (rt *RT200) Read() (Measurements, error) {
	return Measurements{}, nil
}

func test() error {
	var client *modbus.ModbusClient
	var err error

	client, err = modbus.NewClient(&modbus.ClientConfiguration{
		URL:      "rtu:///dev/ttyUSB0",
		Speed:    19200,              // default
		DataBits: 8,                  // default, optional
		Parity:   modbus.PARITY_EVEN, // default, optional
		StopBits: 1,                  // default if no parity, optional
		Timeout:  300 * time.Millisecond,
	})
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	err = client.Open()
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	fmt.Println("New client on /dev/ttyUSB0")

	for {
		sn, err := client.ReadUint32(9001, modbus.HOLDING_REGISTER)
		time.Sleep(200 * time.Millisecond)
		if err == nil {
			fmt.Println("Serial number", sn)
			break
		}
		fmt.Println("Waiting..", err)
	}

	for {
		u32, err := client.ReadUint32(9099, modbus.HOLDING_REGISTER)
		if err == nil {
			fmt.Println("STATUS_REGS", u32)
			break
		}
		fmt.Println("Waiting..", err)
	}

	// devId, err := client.ReadRegister(9000, modbus.HOLDING_REGISTER)
	// if err != nil {
	// 	fmt.Println("can't read device ID")
	// } else {
	// 	switch devId {
	// 	case 16:
	// 		fmt.Println("RuggedTroll 200")
	// 	}
	// }

	// Temp registers

	for {
		regs, err := client.ReadRegisters(37, 8, modbus.HOLDING_REGISTER)

		if err != nil {
			fmt.Println("Waiting", err)
			continue
		}

		x := (uint32(regs[0]) << 16) | uint32(regs[1])
		f := math.Float32frombits(x)
		fmt.Println("PRESS", f)
		break
	}

	for {
		regs, err := client.ReadRegisters(45, 8, modbus.HOLDING_REGISTER)

		if err != nil {
			fmt.Println("Waiting", err)
			continue
		}

		x := (uint32(regs[0]) << 16) | uint32(regs[1])
		f := math.Float32frombits(x)
		fmt.Println("TEMP", f)
		break
	}

	// for {
	// 	press, err := client.ReadFloat32(37, modbus.HOLDING_REGISTER)
	// 	if err != nil {
	// 		if err != nil {
	// 			fmt.Println("can't read pressure", err)
	// 		} else {
	// 			fmt.Printf("Pressure: %v\n", press)
	// 			break
	// 		}
	// 	}
	// }

	// press, err := client.ReadFloat32(38, modbus.HOLDING_REGISTER)
	// fmt.Println("LOOK", press, err)

	// for i := uint16(0); i < 100; i++ {
	// 	press, err := client.ReadFloat32(i, modbus.HOLDING_REGISTER)
	// 	if err != nil {
	// 		fmt.Println("can't read pressure", i)
	// 	} else {
	// 		fmt.Printf("%d: Pressure: %v\n", i, press)
	// 	}
	// }

	// close the TCP connection/serial port
	return client.Close()
}
