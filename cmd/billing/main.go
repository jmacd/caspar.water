package main

import (
	"fmt"
	"os"
	"time"

	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

func main() {
	begin := time.Now()

	cwcColor := getBlueColor()
	// darkGrayColor := getDarkGrayColor()
	// grayColor := getGrayColor()
	// whiteColor := color.NewWhite()
	// redColor := getRedColor()
	// header := getHeader()
	// contents := getContents()

	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(10, 15, 10)

	m.RegisterHeader(func() {
		m.Row(30, func() {
			m.Col(0, func() {
				_ = m.FileImage("assets/img/logo.jpg", props.Rect{
					Percent: 100,
				})
			})
		})
		m.Row(5, func() {
			m.Col(0, func() {
				m.Text("00000 Caspar Frontage Road West #000", props.Text{
					Size:  8,
					Align: consts.Left,
					Color: cwcColor,
				})
			})
		})
		m.Row(5, func() {
			m.Col(0, func() {
				m.Text("Caspar, CA 95420", props.Text{
					Size:  8,
					Align: consts.Left,
					Color: cwcColor,
				})
			})
		})
		m.Row(5, func() {
			m.Col(0, func() {
				m.Text("Tel: (xxx) xxx-xxxx", props.Text{
					Size:  8,
					Align: consts.Left,
					Color: cwcColor,
				})
			})
		})
		m.Row(5, func() {
			m.Col(0, func() {
				m.Text("E-mail: caspar.xxxxx@gmail.com", props.Text{
					Size:  8,
					Align: consts.Left,
					Color: cwcColor,
				})
			})
		})
	})

	m.RegisterFooter(func() {
		m.Row(10, func() {
			m.Col(12, func() {
				m.Text("Tel: (xxx) xxx-xxxx", props.Text{
					Top:   13,
					Style: consts.BoldItalic,
					Size:  8,
					Align: consts.Left,
					Color: cwcColor,
				})
			})
		})
	})

	m.Row(10, func() {})

	m.Row(5, func() {
		m.Col(10, func() {
			m.Text("To: Water User", props.Text{
				Top:   3,
				Style: consts.Bold,
				Align: consts.Left,
			})
		})
	})

	m.Row(5, func() {
		m.Col(10, func() {
			m.Text("00000 Caspar Frontage Road West", props.Text{
				Top:   3,
				Style: consts.Bold,
				Align: consts.Left,
			})
		})
	})
	m.Row(5, func() {
		m.Col(10, func() {
			m.Text("Caspar CA, 95420", props.Text{
				Top:   3,
				Style: consts.Bold,
				Align: consts.Left,
			})
		})
	})

	m.Row(10, func() {})

	m.Row(10, func() {
		m.Col(12, func() {
			m.Text("Invoice Oct-2022", props.Text{
				Top:   3,
				Style: consts.Bold,
				Align: consts.Center,
			})
		})
	})

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

	err := m.OutputFileAndClose("billing.pdf")
	if err != nil {
		fmt.Println("Could not save PDF:", err)
		os.Exit(1)
	}

	end := time.Now()
	fmt.Println(end.Sub(begin))
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
