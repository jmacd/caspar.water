package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

var (
	usersFile    = flag.String("users", "", "users file csv AcctName,User Name,Address,...")
	metadataFile = flag.String("metadata", "", "file csv")

	cwcColor = getBlueColor()

	// darkGrayColor := getDarkGrayColor()
	// grayColor := getGrayColor()
	// whiteColor := color.NewWhite()
	// redColor := getRedColor()
	// header := getHeader()
	// contents := getContents()
)

type (
	Metadata struct {
		Name    string `json:"field0"`
		Address string `json:"field1"`
		Contact string `json:"field2"`
	}

	User struct {
		AccountName    string `json:"field0"`
		UserName       string `json:"field1"`
		ServiceAddress string `json:"field2"`
		BillingAddress string `json:"field3"`
	}
)

func parseAddress(in string) []string {
	out := strings.Split(in, ";")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func main() {
	err := Main()
	if err != nil {
		log.Println(err)
	}
}

func csvToStruct(row []string, out interface{}) error {
	xing := map[string]string{}
	for i, v := range row {
		xing[fmt.Sprint("field", i)] = v
	}
	data, err := json.Marshal(xing)
	if err != nil {
		return fmt.Errorf("to json %w", err)

	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("from json %w", err)
	}
	return nil
}

func readAll(name string) ([][]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	ret, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %s: %w", name, err)
	}
	return ret, nil
}

func Main() error {
	flag.Parse()

	users, err := readAll(*usersFile)
	if err != nil {
		return err
	}

	rawMeta, err := readAll(*metadataFile)
	if len(rawMeta) != 2 {
		return fmt.Errorf("metadata file should have two lines: %d", len(rawMeta))
	}
	var meta Metadata
	if err := csvToStruct(rawMeta[1], &meta); err != nil {
		return err
	}

	for _, row := range users[1:] {
		var user User
		if err := csvToStruct(row, &user); err != nil {
			return err
		}

		fmt.Println("Account", user.AccountName)

		if err := writePDF(meta, user); err != nil {
			return fmt.Errorf("write pdf %w", err)
		}
	}
	return nil
}

type lineStyle struct {
	sz    float64
	ht    float64
	top   float64
	txt   consts.Style
	align consts.Align
	color color.Color
}

func (style lineStyle) multiLine(m pdf.Maroto, lines []string) {
	for _, line := range lines {
		m.Row(style.ht, func() {
			m.Col(0, func() {
				m.Text(line, props.Text{
					Size:  style.sz,
					Top:   style.top,
					Style: style.txt,
					Align: style.align,
					Color: style.color,
				})
			})
		})
	}
}

func writePDF(meta Metadata, user User) error {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(50, 50, 50)

	const bigLine = 5
	const smallLine = 4
	const sepLine = 2

	toStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   3,
		txt:   consts.Bold,
		align: consts.Left,
		color: color.NewBlack(),
	}

	fromStyle := lineStyle{
		sz:    8,
		ht:    smallLine,
		top:   3,
		txt:   consts.Normal,
		align: consts.Right,
		color: cwcColor,
	}

	m.RegisterHeader(func() {
		m.Row(30, func() {
			m.Col(0, func() {
				_ = m.FileImage("assets/img/logo.jpg", props.Rect{
					Percent: 100,
					Center:  true,
				})
			})
		})
		fromStyle.multiLine(m,
			append([]string{meta.Name},
				append(parseAddress(meta.Address),
					parseAddress(meta.Contact)...)...))
	})

	// m.RegisterFooter(func() {
	// 	m.Row(smallLine, func() {
	// 		m.Col(0, func() {
	// 			m.Text("E-mail: caspar.water@gmail.com", props.Text{
	// 				Size:  8,
	// 				Align: consts.Left,
	// 				Color: cwcColor,
	// 			})
	// 		})
	// 	})
	// })

	// m.Row(10, func() {})

	m.Row(10, func() {
		m.Col(12, func() {
			m.Text("Invoice Oct-2022", props.Text{
				Top:   3,
				Style: consts.Bold,
				Align: consts.Left,
			})
		})
	})

	m.Row(sepLine, func() {})

	toStyle.multiLine(m, append([]string{"To:", user.UserName}, parseAddress(user.BillingAddress)...))

	m.Row(10, func() {})

	m.Row(7, func() {
		m.Col(3, func() {
			m.Text("Pay all the money", props.Text{
				Top:   1.5,
				Size:  9,
				Style: consts.Bold,
				Align: consts.Left,
			})
		})
		m.ColSpace(9)
	})

	m.Row(7, func() {
		m.Col(3, func() {
			m.TableList([]string{
				"WHAT",
				"HOWMUCH",
			}, [][]string{
				{
					"Maintenance",
					"300.0",
				},
				{
					"Operations",
					"0.0",
				},
				{
					"Utilities",
					"0.0",
				},
				{
					"Insurance",
					"0.0",
				},
				{
					"Taxes",
					"0.0",
				},
			})

			m.Text("$300.00", props.Text{
				Top:   1.5,
				Size:  9,
				Style: consts.Bold,
				Align: consts.Center,
			})
		})
		m.ColSpace(9)
	})

	return m.OutputFileAndClose(user.AccountName + ".pdf")
}

func getDarkGrayColor() color.Color {
	return color.Color{
		Red:   55,
		Green: 55,
		Blue:  55,
	}
}

func getGrayColor() color.Color {
	return color.Color{
		Red:   200,
		Green: 200,
		Blue:  200,
	}
}

func getBlueColor() color.Color {
	return color.Color{
		Red:   10,
		Green: 10,
		Blue:  150,
	}
}

func getRedColor() color.Color {
	return color.Color{
		Red:   150,
		Green: 10,
		Blue:  10,
	}
}
