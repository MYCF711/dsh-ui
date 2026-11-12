package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree: the application declares the tree once, and every
// control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	home = tree.Feature("home")

	increment = tree.ID(home, "increment")
	reset     = tree.ID(home, "reset")
)

// model is everything the window shows. Paint only reads it; a click handled where
// its button is declared is the only place that changes it.
type model struct {
	count int
	view  flex.View
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "hello", Width: 420, Height: 260}

	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				f.Text("hello").FontSize(20).Bold()
				f.Paragraph("Edit main.go and save: viva dev reloads the window and keeps this count.", 3).Muted()
				f.Spacer()
				f.Text(fmt.Sprintf("Count: %d", m.count)).FontSize(28).Bold()
				f.Row().Gap(8).Children(func() {
					if f.Button(increment, "Add one").Clicked() {
						m.count++
					}
					if f.Button(reset, "Reset").Outline().Clicked() {
						m.count = 0
					}
				})
			})
		})
	}
	return config
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
