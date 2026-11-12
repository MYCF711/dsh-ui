package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

const sample = `## MarkdownView 验证

这是一段**粗体**、*斜体*（Viva 说斜体暂用加粗代替）、` + "`行内代码`" + ` 与 [链接](https://example.com)。

- 列表项一
- 列表项二，中文与 English 混排
- 列表项三

> 引用：中文排版必须正确。

` + "```go" + `
func main() {
	fmt.Println("等宽代码块")
}
` + "```" + `

| 列 A | 列 B |
|---|---|
| 中文 | English |
| 123 | 456 |

未闭合的 **粗体在这里
`

type model struct {
	view ui.Page
	doc  markdown.Document
	md   components.MarkdownView
}

var theme = ui.DarkTheme()

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &gfx.Tree{}, Title: "MarkdownView 验证", Width: 900, Height: 760}
	m.doc.Parse(sample)
	cfg.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		b := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(b, theme.Background)
		var slot ui.Box
		m.view.Draw(canvas, theme, b, func(f ui.C) {
			ui.Column(f).Fill().Pad(20).Children(func() {
				w := f.Bounds().Width - 40
				h := m.md.Layout(canvas, components.MarkdownStyle{Size: 14}, &m.doc, w)
				slot = ui.Slot(f).Height(h)
			})
		})
		r := slot.Rect()
		m.md.Paint(canvas, components.MarkdownPaint{
			Look: components.MarkdownLookFor(theme),
			X:    r.X, Y: r.Y,
		})
	}
	return cfg
}

func main() {
	var c gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &c); err != nil {
		log.Fatal(err)
	}
}
