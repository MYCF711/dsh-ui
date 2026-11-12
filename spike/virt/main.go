// virt —— 视口虚拟化原型。
//
// 背景（docs/u1-memory.md 实测）：Viva 每条真正画出来的消息约 0.46 MB（GPU 文本纹理），
// 200 条全渲染 = 172.9 MB。所以视口外的消息**连 f.Slot 都不该进 flex 树**。
//
// 本原型用三种模式对比：
//
//	virt -mode all       全部渲染（基线）
//	virt -mode viewport  只渲染视口内的（外面对 Slot 占位）
//	virt -mode overscan  视口 ±1 屏（默认）
//
// 关键设计：**每条消息放在固定高度的盒子里**（估算高度），
// 这样累计位置永不漂移、滚动不跳；估算偏差只影响"文字被裁多少"，不影响滚动。
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	tree   = gfx.Tree{}
	home   = tree.Feature("virt")
	listID = tree.ID(home, "list")
)

type model struct {
	view ui.Page
}

var (
	modeFlag  string
	countFlag int
	listMode  string
	msgs      []string
	heights   []float32
	lastStat  string
)

const (
	fontSize = 14.0
	lineH    = 22.0
	padY     = 10.0
)

// estHeight 估算一条消息的高度：每行放多少字 → 折算行数。
// 中文按 1 rune ≈ fontSize 宽估，所以是**上界**；真实文字只会比盒子矮。
func estHeight(text string, width float32) float32 {
	usable := width - 40
	if usable < 80 {
		usable = 80
	}
	// 0.62 是实测反推的：第一版按 1.0 倍算，500 条里每条高估约 50%，
	// 结果是每条消息下面拖一条大空白（见 docs/virt-overscan-before.png）。
	perLine := usable / (fontSize * 0.62)
	if perLine < 8 {
		perLine = 8
	}
	n := float32(len([]rune(text))) / perLine
	lines := float32(int(n))
	if n > lines {
		lines++
	}
	if lines < 1 {
		lines = 1
	}
	return lines*lineH + padY*2
}

func seed(n int) []string {
	out := make([]string, 0, n)
	base := "这是一段用来测虚拟化的中文与 English 混排文本，" +
		"它足够长，能覆盖到多行的排版，用来验证估算高度与真实高度差多少。" +
		"第 %d 段：the quick brown fox jumps over the lazy dog，0123456789。"
	for i := 0; i < n; i++ {
		reps := 2 + i%3
		s := strings.Repeat(fmt.Sprintf(base+" ", i+1), reps)
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func newConfig(m *model) app.Config {
	cfg := app.Config{Tree: &tree, Title: "视口虚拟化原型", Width: 1000, Height: 720}
	cfg.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		theme := ui.DarkTheme()
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		// 每次重新取，不能长期持有 *ScrollState（viva flex/scroll.go:15-18）
		var offset float32
		if st := m.view.ScrollState(listID); st != nil {
			offset = st.Offset
		}

		headerH := float32(52)
		viewportH := size.Height - headerH - 12
		overscan := float32(0)
		if listMode == "overscan" {
			overscan = viewportH
		}

		rendered, slotted := 0, 0
		m.view.Draw(canvas, theme, bounds, func(f ui.C) {
			ui.Column(f).Fill().Pad(12).Gap(6).Children(func() {
				ui.Text(f, fmt.Sprintf("模式 %s · %d 条 · 视口 %.0fpx · offset %.0f", listMode, len(msgs), viewportH, offset)).
					FontSize(13).Bold()
				ui.Divider(f)
				ui.Scroll(f).Grow(1).Children(func() {
					ui.Column(f).Gap(0).Children(func() {
						y := float32(0)
						for i := range msgs {
							h := heights[i]
							inView := y+h > offset-overscan && y < offset+viewportH+overscan
							if listMode == "all" || inView {
								ui.Column(f).Height(h).Pad(padY).Children(func() {
									ui.Text(f, msgs[i]).FontSize(fontSize).MaxLines(int(h / lineH))
								})
								rendered++
							} else {
								ui.Slot(f).Height(h)
								slotted++
							}
							y += h
						}
					})
				})
			})
		})
		lastStat = fmt.Sprintf("rendered=%d slotted=%d", rendered, slotted)
	}
	return cfg
}

func main() {
	flag.StringVar(&modeFlag, "mode", "overscan", "all | viewport | overscan")
	flag.IntVar(&countFlag, "n", 500, "消息条数")
	flag.Parse()

	listMode = modeFlag
	msgs = seed(countFlag)
	heights = make([]float32, len(msgs))
	for i := range msgs {
		heights[i] = estHeight(msgs[i], 1000)
	}

	log.Printf("模式=%s 条数=%d 估算总高=%.0fpx", listMode, len(msgs), totalHeight())

	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}

func totalHeight() float32 {
	var h float32
	for _, v := range heights {
		h += v
	}
	return h
}
