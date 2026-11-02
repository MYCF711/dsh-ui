package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	cm   = tree.Feature("charmap")
	scr  = tree.ID(cm, "scr")
)

type model struct{ view flex.View }

func row(label, s string) string { return fmt.Sprintf("%-8s %s", label, s) }

func ascii() string {
	var out []byte
	for c := byte(32); c < 127; c++ {
		out = append(out, c)
	}
	return string(out)
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	cfg := app.Config{Tree: &tree, Title: "字形覆盖矩阵", Width: 1080, Height: 820}
	cfg.Paint = func(canvas *ui.Canvas, size ui.Size) {
		b := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(b, theme.Background)
		m.view.Draw(canvas, theme, b, func(f *flex.Frame) {
			f.Column().Fill().Pad(14).Gap(6).Children(func() {
				f.Text("字形覆盖矩阵（等宽）— 红字位置若出现方框/空白/错字即为缺字形").FontSize(14).Bold()
				f.Divider()
				f.ScrollArea(scr).Grow(1).Children(func() {
					f.Column().Gap(2).Children(func() {
						f.Text(row("ASCII", ascii())).Mono().FontSize(13)
						f.Text(row("CJK标点", "，。、；：？！「」『』（）【】《》——……·～")).Mono().FontSize(14)
						f.Text(row("全角", "ＡＢＣ１２３　全角空格")).Mono().FontSize(14)
						f.Text(row("中文", "你好世界 繁體字 日本語 ひらがな カタカナ 한국어")).Mono().FontSize(14)
						f.Text(row("数字", "0123456789 ½ ¼ ⅓ ① ② ⑩")).Mono().FontSize(14)
						f.Text(row("箭头", "← → ↑ ↓ ↔ ⇒ ⇐ ⬆ ⬇ ➜")).Mono().FontSize(14)
						f.Text(row("框线", "┌ ─ ┐ │ └ ┘ ├ ┤ ┬ ┴ ┼ ═ ║")).Mono().FontSize(14)
						f.Text(row("emoji", "😀 🚀 ✅ ❌ 🔧 📦 🧪 ⚡ 🎯 🐛 👨‍💻 🇨🇳")).Mono().FontSize(14)
						f.Text(row("Markdown", "**粗** *斜* `code` # 标题 > 引用 - 列表 [链接](x) ```")).Mono().FontSize(14)
					})
				})
			})
		})
	}
	return cfg
}

func main() {
	var c ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &c); err != nil {
		log.Fatal(err)
	}
}
