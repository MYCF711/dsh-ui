package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	tree  = gfx.Tree{}
	e2    = tree.Feature("e2")
	draft = tree.ID(e2, "draft")
	outer = tree.ID(e2, "outer")
	codeS = tree.ID(e2, "code")
	listS = tree.ID(e2, "list")
)

type model struct {
	draft  ui.Field
	view   ui.Page
	codeOv mdlite.Overflow // 代码块的横向滚动状态
}

const cn = "这是一段中文与 English 混排的段落，用来验证断行、标点与字重。包含「引号」、（括号）、—— 破折号、……省略号，以及 100% 的数字与 v0.2.3 的版本号。换行应发生在 CJK 字符之间（kinsoku 规则），而不是把整个英文单词撑到下一行。"

var emojis = "😀 🚀 ✅ ❌ 🔧 📦 🧪 ⚡ 🎯 🐛 — emoji 必须是彩色字形，不能是方框；全角标点：，。；：！？"

func codeLines() []string {
	out := []string{"package main", "", "import ("}
	for i := 0; i < 54; i++ {
		// 每行刻意超过 110 个字符，用来触发横向溢出
		out = append(out, fmt.Sprintf("\t// 第 %02d 行：` func 很长的标识符名 %d(x int) string { return fmt.Sprint(x * %d) } // 尾巴故意很长，验证横向平移 —— END_OF_LINE_%02d", i+1, i, i+1, i+1))
	}
	out = append(out, ")", "", "func main() {", "\tok := true", "}", "// ⬆ 共 60 行，验证等宽对齐与横向滚动")
	return out
}

var msgs []string

func init() {
	for i := 0; i < 1000; i++ {
		msgs = append(msgs, fmt.Sprintf("%04d · 消息行 %d — 用户输入的一段中文，用来测量长列表滚动的成本", i+1, i+1))
	}
}

// monoCells 是一个字符串在等宽网格里占的格数（宽字符算 2）。
func monoCells(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case r < 0x1100:
			n++
		case r >= 0x1100 && r <= 0x115F,
			r >= 0x2E80 && r <= 0xA4CF,
			r >= 0xAC00 && r <= 0xD7A3,
			r >= 0xF900 && r <= 0xFAFF,
			r >= 0xFE30 && r <= 0xFE6F,
			r >= 0xFF00 && r <= 0xFF60,
			r >= 0xFFE0 && r <= 0xFFE6,
			r >= 0x1F300 && r <= 0x1FAFF,
			r >= 0x20000 && r <= 0x3FFFD:
			n += 2
		default:
			n++
		}
	}
	return n
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "E2 渲染矩阵", Width: 1100, Height: 860}
	config.TextInput = ui.Fields(&m.draft)
	lines := codeLines()

	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)
		var codeSlot ui.Box
		m.view.Draw(canvas, theme, bounds, func(f ui.C) {
			ui.Column(f).Fill().Pad(16).Gap(8).Children(func() {
				ui.Text(f, "E2 · 渲染矩阵（1 中文 · 2 代码 · 3 卡片 · 4 emoji · 5 IME · 6 长列表 · 7 缩放）").FontSize(16).Bold()
				ui.Divider(f)

				// 1 中文/英文混排
				ui.Text(f, "① 中文与英文混排").FontSize(13).Bold()
				ui.Wrap(f, cn).FontSize(15)

				// 4 emoji + 全角标点
				ui.Text(f, "④ emoji 与全角标点").FontSize(13).Bold()
				ui.Wrap(f, emojis).FontSize(15)

				// 5 IME 输入框（真机人工项）
				ui.Text(f, "⑤ 中文输入法：请在这个框里用系统 IME 打「你好世界」").FontSize(13).Bold()
				ui.Row(f).Gap(8).Children(func() {
					ui.TextFieldID(f, draft, &m.draft, "Message", "点这里，然后用 Shift 切中文输入").Grow(1)
				})

				// 2 代码块：flex 只有纵向滚动，这里用 mdlite.Overflow 自己接管横向
				ui.Text(f, "② Go 代码块 60 行（等宽，可横向滚动）").FontSize(13).Bold()
				codeSlot = ui.Slot(f).Height(150)

				// 3 工具卡
				ui.Text(f, "③ 工具调用卡片").FontSize(13).Bold()
				ui.Column(f).Pad(10).Radius(8).Children(func() {
					ui.Row(f).Gap(8).Children(func() {
						ui.Text(f, "bash").Mono().Bold().FontSize(13)
						ui.Text(f, "git status --short").Mono().Muted().FontSize(12)
					})
					ui.Text(f, "exit 0 · 3 files changed · 用了 12 ms").Muted().FontSize(12)
				})

				// 6 长列表
				ui.Text(f, "⑥ 1000 条长列表滚动").FontSize(13).Bold()
				ui.Scroll(f).Grow(1).Children(func() {
					ui.Column(f).Gap(2).Children(func() {
						for _, s := range msgs {
							ui.Text(f, s).FontSize(12)
						}
					})
				})
			})
		})

		// Draw 之后布局已定，Slot 的 Rect 才是最终位置
		if r := codeSlot.Rect(); r.Width > 1 && r.Height > 1 {
			cell, lineH := canvas.MonoCellSize(12, gfx.MonoSystem)
			if cell <= 0 {
				cell = 7.2
			}
			if lineH <= 0 {
				lineH = 16
			}
			maxCells := 0
			for _, ln := range lines {
				if c := monoCells(ln); c > maxCells {
					maxCells = c
				}
			}
			contentW := float32(maxCells)*cell + 20
			contentH := float32(len(lines))*lineH + 10
			m.codeOv.Draw(canvas, theme, r, contentW, contentH, codeS, "代码块", func(c ui.Rect) {
				for i, ln := range lines {
					y := c.Y + 5 + float32(i)*lineH
					if y > r.Y+r.Height {
						break
					}
					if y+lineH < r.Y {
						continue
					}
					canvas.Label(ln, ui.Rect{X: c.X + 10, Y: y, Width: contentW, Height: lineH},
						gfx.TextStyle{Size: 12, Mono: true, Color: theme.Foreground})
				}
			})
		}
	}
	_ = outer
	_ = strings.TrimSpace
	return config
}

func main() {
	var canvas gfx.Canvas
	// 演示用：起始就横向偏 220px，一眼能看出内容确实被平移了
	if err := app.RunCanvas(newConfig(&model{codeOv: mdlite.Overflow{OffsetX: 0}}), &canvas); err != nil {
		log.Fatal(err)
	}
}
