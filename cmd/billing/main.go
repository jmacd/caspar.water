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
	usersFile = flag.String("users", "", "users file csv AcctName,User Name,Address,...")

	cwcColor = getBlueColor()

	// darkGrayColor := getDarkGrayColor()
	// grayColor := getGrayColor()
	// whiteColor := color.NewWhite()
	// redColor := getRedColor()
	// header := getHeader()
	// contents := getContents()
)

type (
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

func Main() error {
	flag.Parse()

	f, err := os.Open(*usersFile)
	if err != nil {
		return fmt.Errorf("open %s: %w", *usersFile, err)
	}
	users, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return fmt.Errorf("read csv %s: %w", *usersFile, err)
	}

	for _, row := range users[1:] {
		xing := map[string]string{}
		for i, v := range row {
			xing[fmt.Sprint("field", i)] = v
		}
		var user User
		data, err := json.Marshal(xing)
		if err != nil {
			return fmt.Errorf("to json %w", err)

		}
		if err := json.Unmarshal(data, &user); err != nil {
			return fmt.Errorf("from json %w", err)
		}

		fmt.Println("Account", user.AccountName)

		if err := writePDF(user); err != nil {
			return fmt.Errorf("write pdf %w", err)
		}
	}
	return nil
}

func writePDF(user User) error {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(40, 50, 40)

	const smallLine = 4
	const sepLine = 2

	m.RegisterHeader(func() {
		m.Row(30, func() {
			m.Col(0, func() {
				_ = m.FileImage("assets/img/logo.jpg", props.Rect{
					Percent: 100,
				})
			})
		})
		m.Row(smallLine, func() {
			m.Col(0, func() {
				m.Text("45100 Caspar Frontage Road West #145", props.Text{
					Size:  8,
					Align: consts.Right,
					Color: cwcColor,
				})
			})
		})
		m.Row(smallLine, func() {
			m.Col(0, func() {
				m.Text("Caspar, CA 95420", props.Text{
					Size:  8,
					Align: consts.Right,
					Color: cwcColor,
				})
			})
		})
		// m.Row(smallLine, func() {
		// 	m.Col(0, func() {
		// 		m.Text("Tel: (415) 309-196", props.Text{
		// 			Size:  8,
		// 			Align: consts.Left,
		// 			Color: cwcColor,
		// 		})
		// 	})
		// })
		m.Row(smallLine, func() {
			m.Col(0, func() {
				m.Text("E-mail: caspar.water@gmail.com", props.Text{
					Size:  8,
					Align: consts.Right,
					Color: cwcColor,
				})
			})
		})
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

	for _, aline := range append([]string{"To:", user.UserName}, parseAddress(user.BillingAddress)...) {
		m.Row(5, func() {
			m.Col(10, func() {
				m.Text(aline, props.Text{
					Top:   3,
					Style: consts.Bold,
					Align: consts.Left,
				})
			})
		})
	}

	m.Row(10, func() {})

	m.Row(7, func() {
		m.Col(3, func() {
			m.Text("Pay all the money", props.Text{
				Top:   1.5,
				Size:  9,
				Style: consts.Bold,
				Align: consts.Left,
				// Color: color.NewWhite(),
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
				// Color: color.NewWhite(),
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
