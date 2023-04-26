package csv

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadAll converts a CSV file into a list of T structs (all defined
// above), where the first CSV row matches field names.  This is done
// via an intermediate JSON representation.
func ReadAll[T any](name string) ([]T, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	read, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %s: %w", name, err)
	}
	if len(read) < 2 {
		return nil, fmt.Errorf("not enough rows: %s", name)
	}
	legend := read[0]
	for i := range legend {
		legend[i] = strings.ReplaceAll(legend[i], " ", "")
	}
	rows := read[1:]
	var ret []T
	for _, row := range rows {
		xing := map[string]interface{}{}
		for i, v := range row {
			xing[legend[i]] = v
		}
		data, err := json.Marshal(xing)
		if err != nil {
			return nil, fmt.Errorf("to json %w", err)
		}
		var out T
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("from json %w", err)
		}
		ret = append(ret, out)

	}
	return ret, nil
}
