package invoice

import (
	"strings"

	"github.com/jmacd/caspar.water/cmd/internal/billing/business"
	"github.com/jmacd/caspar.water/cmd/internal/billing/user"
	"github.com/jmacd/maroto/pkg/color"
	"github.com/jmacd/maroto/pkg/consts"
	"github.com/jmacd/maroto/pkg/pdf"
	"github.com/jmacd/maroto/pkg/props"
)

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

type Document interface {
	FullDate() string
	InvoiceName() string
	BodyText() (string, error)
}

var TableStyle = props.TableList{
	Align: consts.Right,
	ContentProp: props.TableListContent{
		Family: consts.Helvetica,
		Size:   9,
	},
}

func MakeInvoice(
	bus business.Business,
	user user.User,
	doc Document,
	mainContent func(pdf.Maroto),
) (pdf.Maroto, error) {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(30, 25, 30)

	const bigLine = 5
	const smallLine = 4
	const sepLine = 2

	toStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   4,
		txt:   consts.Bold,
		align: consts.Left,
		color: color.NewBlack(),
	}

	paymentStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   4,
		align: consts.Left,
		color: color.NewBlack(),
	}

	boldText := props.Text{
		Top:    3,
		Style:  consts.Bold,
		Align:  consts.Left,
		Family: consts.Helvetica,
		Size:   10,
	}

	normText := props.Text{
		Align:           consts.Left,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
	}

	centerText := props.Text{
		Align:           consts.Center,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
	}

	rightText := props.Text{
		Align:           consts.Right,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
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
	})

	m.RegisterFooter(func() {
		m.Row(3, func() {
			m.Col(0, func() {
				m.Text(bus.Contact, centerText)

			})
		})
	})

	m.Row(4, func() {})
	m.Row(6, func() {
		m.Col(4, func() {
			m.Text("To:", normText)
		})
		m.ColSpace(4)
		m.Col(4, func() {
			m.Text(doc.FullDate(), rightText)
		})
	})

	toStyle.multiLine(m, append([]string{user.UserName}, user.BillingAddress.Split()...))
	m.Row(4, func() {})

	m.Row(8, func() {
		m.Col(8, func() {
			m.Text("Invoice: "+doc.InvoiceName(), boldText)
		})
		m.Row(4, func() {})
	})

	m.Row(8, func() {
		m.Col(12, func() {
			m.Text("Service address: "+user.ServiceAddress.OneLine(), boldText)
		})
	})
	m.Row(4, func() {})

	body, err := doc.BodyText()
	if err != nil {
		return nil, err
	}

	for _, para := range strings.Split(body, "\n\n") {
		para = strings.TrimSpace(para)
		para = strings.ReplaceAll(para, "\n", " ")

		plines := m.GetLinesHeight(para, normText, 115)
		m.Row(float64(plines), func() {
			m.Col(0, func() {
				m.Text(para, normText)
			})
		})
	}
	m.Row(1, func() {})

	mainContent(m)

	m.Row(2, func() {})
	m.Row(4, func() {
		m.Col(4, func() {
			m.Text("Please send payment to:", normText)
		})
	})

	paymentStyle.multiLine(m, append([]string{bus.Name}, bus.Address.Split()...))

	m.Row(10, func() {})
	m.Row(4, func() {
		m.Col(4, func() {
			m.Text("Thank you!", normText)
		})
	})
	return m, nil
}
