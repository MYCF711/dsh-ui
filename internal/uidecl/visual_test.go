//go:build darwin || windows || linux

package uidecl

// visual_test.go —— 声明部件那一行文字的**真绘制行高**证据。
//
// 判据不是"画面里有字"，而是逐条对官方**对等控件**的行盒（官方没有声明式部件
// 这种包，所以锚的是"一行只读 12px 文字"的官方写法）：
//
//	label / keyvalue 行高 22 = AgentPresetLabel.module.css:9,12-13（height:22px;line-height:22px）
//	dot 行高 20            = StatsPills.module.css:9-11,19（12px/20 行高 + padding:1px 撑到 22）
//	list 行高 18 / 行距 2   = Deliverables.module.css:6（font-size:12px;line-height:18px）
//	                          与 :17（.details{gap:2px}）
//
// 旧实现一个 LineHeight 都没给，Viva 按 `fontSize+7` 排行（viva ui/layout.go:63-67），
// 12px 字排出 19：比官方 18 溢出 1px、比官方 22 偏心 3px。把 LineHeight 去掉，
// TestCaptionLineHeightsAreOfficial 必须红——这条测试就是那处缺口的反证。

import (
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 为什么量 Hit.Bounds 而不是 Box.Rect：无头画布上 Box.Rect() 只在 Page.Draw 的
// 同步那一段有效，下一帧一来节点就重建（viva 的 Box 有代际失效）。Hits 是同一套
// 布局解算出来的、可跨帧读的测量值——plugins/dsentrycustom/entry_menu_test.go
// 量官方行高用的也是它。所以每块挂一个具名 Hit，量 Bounds.Height。

// uidraw 在真 Canvas 上画一帧（无头，不用开窗口：布局解算与命中区都在 Canvas 里）。
func uidraw(theme *pluginapi.Theme, body func(cc ui.C, tree *gfx.Tree, feat gfx.Scope)) *gfx.Canvas {
	tree := &gfx.Tree{}
	feat := tree.Feature("uidecl-visual")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	// 无头画布**没有字宽**：不装测量器，中文一律量成 0 宽、文字行量成 0 高，
	// 「量到的行高」就全是 0 —— 那是夹具缺件，不是被测代码的问题。
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 240, Height: 200}, func(cc ui.C) { body(cc, tree, feat) })
	return canvas
}

// hitHeights 取前 n 个命中区的高度；少一个就报错（行没画全，测了没意义）。
func hitHeights(t *testing.T, canvas *gfx.Canvas, n int) []float32 {
	t.Helper()
	hits := canvas.Hits()
	if len(hits) != n {
		t.Fatalf("落下 %d 个命中区，应为 %d —— 行没画全", len(hits), n)
	}
	out := make([]float32, n)
	for i := range hits {
		out[i] = hits[i].Bounds.Height
		if out[i] <= 0 {
			t.Fatalf("第 %d 个命中区高度 %g：布局没量到（测量器缺件？）", i, out[i])
		}
	}
	return out
}

// hitHeight 取第一个命中区的高度。
func hitHeight(t *testing.T, canvas *gfx.Canvas) float32 {
	t.Helper()
	return hitHeights(t, canvas, 1)[0]
}

// TestCaptionLineHeightsAreOfficial：三种官方行高画出来必须就是它们本身。
func TestCaptionLineHeightsAreOfficial(t *testing.T) {
	theme := dscore.Dark()
	// 字号阶梯先自证：官方 caption 是 12/18（Deliverables.module.css:6），
	// 主题的 Caption/CaptionLine 必须与它同值，否则下面的行高没有意义。
	if theme.Fonts.Caption != 12 || theme.Fonts.CaptionLine != 18 {
		t.Fatalf("主题字号阶梯与官方 caption 不一致: Caption=%g CaptionLine=%g，官方 12/18",
			theme.Fonts.Caption, theme.Fonts.CaptionLine)
	}

	lines := []float32{defaultHeight, dotLineHeight, theme.Fonts.CaptionLine}
	want := []float32{22, 20, 18} // 官方：标签行 22 / 药丸行 20 / caption 18
	names := []string{
		"label+keyvalue 行高（AgentPresetLabel.module.css:9,13）",
		"dot 行高（StatsPills.module.css:9-11）",
		"list 行高（Deliverables.module.css:6）",
	}

	// 命中区**挂在文字本身**上，不挂在外面那层固定高的行盒上：
	// 行盒的高度是我们自己给的，量它等于量入参、恒等于期望值（第一版就这么写，
	// 把 captionText 的 LineHeight 删掉它照样绿 —— 假绿）。文字自己的行盒高度
	// 才是 Viva 排出来的真值：给了 LineHeight 就是它，不给就是 fontSize+7 = 19。
	canvas := uidraw(theme, func(cc ui.C, tree *gfx.Tree, feat gfx.Scope) {
		ui.Column(cc).Width(240).Children(func() {
			for i, line := range lines {
				box := captionText(cc, "外部声明部件", theme, theme.LabelSecondary, line)
				box.Hit(tree.ID(feat, "line"+strconv.Itoa(i)), "")
			}
		})
	})
	got := hitHeights(t, canvas, len(lines))
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: 实际 %g，官方 %g（差值 %g）—— 少了 LineHeight 时 Viva 按 fontSize+7 排出 19",
				names[i], got[i], want[i], got[i]-want[i])
		}
	}
}

// TestDrawnBlockHeightsAreOfficial：行盒按官方值撑开（label/keyvalue 22、list 行 18+2）。
//
// 这条**不是**行高证据：外层行盒的高度就是调用方给的 d.height()/CaptionLine，
// 量它只能证明常量没写错。真正的行高证据是 TestCaptionLineHeightsAreOfficial
// （命中区挂在文字本身，删掉 LineHeight 就红）。这里留着是因为它同时守着
// 「四种 kind 至少画出一块」——空内容整块不画是设计，但非空内容必须落下来。
func TestDrawnBlockHeightsAreOfficial(t *testing.T) {
	theme := dscore.Dark()
	cases := []struct {
		name string
		json string
		want float32
	}{
		{"label", `{"id":"t.label","slot":"sidebar.footer","kind":"label","text":"一行"}`, 22},
		{"keyvalue", `{"id":"t.kv","slot":"sidebar.footer","kind":"keyvalue","label":"键","text":"值"}`, 22},
		{"list-3", `{"id":"t.list","slot":"sidebar.footer","kind":"list","items":["a","b","c"]}`, 3*18 + 2*2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := parseJSON("t.json", []byte(c.json))
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			pl := d.Plugin(theme, &dscore.Primitives{Theme: theme}).(*declPlugin)
			canvas := uidraw(theme, func(cc ui.C, tree *gfx.Tree, feat gfx.Scope) {
				block := ui.Column(cc).Width(240)
				block.Children(func() {
					pl.draw(cc, &pluginapi.Env{Theme: theme})
				})
				block.Hit(tree.ID(feat, "block"), "")
			})
			if got := hitHeight(t, canvas); got != c.want {
				t.Fatalf("%s 整块高 %g，官方行盒累计 %g", c.name, got, c.want)
			}
		})
	}
}
