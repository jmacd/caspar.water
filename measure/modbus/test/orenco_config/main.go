package main

import "fmt"

func main() {
	fmt.Println(`  modbus:
    url: tcp://192.168.70.237:502
    interval: 5m
    timeout: 2s
    reconnect: true       # Orenco closes connection after each request
    read_delay: 15s       # Orenco requires 15s between register reads
    prefix: "orenco"
    metrics:`)

	// Track seen names to skip duplicates
	seen := make(map[string]bool)

	for _, r := range orencoRegisters {
		// Skip duplicates
		if seen[r.name] {
			continue
		}
		seen[r.name] = true

		// A and L types: float32 at base = (pointNum * 2) + 999
		// D type: uint16 at base = 40000 + pointNum
		var base uint16
		var dtype string

		switch r.ptype {
		case "A", "L":
			base = r.baseF32
			dtype = "float32"
		case "D":
			base = r.baseInt
			dtype = "uint16"
		default:
			continue // skip unknown types
		}

		unit := r.unit
		if unit == "" {
			unit = "1"
		}

		fmt.Printf("    - name: %s\n", r.name)
		fmt.Printf("      base: %d             # Pt%d\n", base, r.pointNum)
		fmt.Printf("      type: %s\n", dtype)
		fmt.Printf("      range: holding\n")
		fmt.Printf("      unit: %q\n", unit)
		fmt.Printf("      kind: gauge\n")
	}
}
