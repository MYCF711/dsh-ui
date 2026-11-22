//go:build darwin || windows || linux

// menu_run_test.go —— 菜单多段行画笔的读数判据（Viva headless）。
//
// 只钉两件在真画布上才成立的事：
//
//  1. **等宽面的裁切按等宽面量**。`gfx.FitLine` 是按比例面量的
//     （`canvas.TextWidthWeight`），拿它裁一段等宽文本会得到一段**仍然超宽**的字符串；
//     本仓 `fitRun` 对等宽段改用 `TextWidthStyle` 二分裁。
//  2. 分组标题的字号/墨色/内边距是官方原语 `._label_70kcw_214` 那一档，且**不吃**
//     行自己给的 `Foreground`。
package dscore

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// measureSnapshot 开一帧 headless，让 draw 拿到真画布，返回这一帧的绘制快照。
func measureSnapshot(t *testing.T, draw func(canvas *gfx.Canvas)) app.Snapshot {
	t.Helper()
	theme := Dark()
	pageTheme := VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var canvas *gfx.Canvas
	config := app.Config{
		Title: "dscore-menu", Width: 480, Height: 120,
		WindowMode: app.WindowFrameless, Invalidator: &app.Invalidator{},
		Paint: func(c *gfx.Canvas, size ui.Size) {
			canvas = c
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			c.Fill(bounds, theme.BgBase)
			page.Draw(c, bounds, func(uiC ui.C) {
				ui.Column(uiC).Width(bounds.Width).Children(func() {
					draw(c)
				})
			})
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if canvas == nil {
		t.Fatal("没拿到画布")
	}
	return session.Snapshot()
}

// TestFitRunCutsMonoTextByTheMonoFace 钉住 `fitRun` 的等宽分支。
func TestFitRunCutsMonoTextByTheMonoFace(t *testing.T) {
	const line = "feature/some-rather-long-branch-name"
	mono := gfx.TextStyle{Size: 12, Mono: true}
	const room float32 = 80
	var got string
	var proportionalOver float32
	measureSnapshot(t, func(canvas *gfx.Canvas) {
		got = fitRun(canvas, line, mono, room)
		// 反向对照：Viva 的 `FitLine` 按**比例面**裁同一条，拿等宽面量回去会超宽。
		proportionalOver = canvas.TextWidthStyle(canvas.FitLine(line, 12, gfx.WeightRegular, room), mono)
	})
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("放不下时必须以省略号收尾，实得 %q", got)
	}
	if cells := len([]rune(strings.TrimSuffix(got, "…"))); cells == 0 {
		t.Fatalf("一个字符都放不下（上限 %v）：%q", room, got)
	}
	if proportionalOver <= room {
		t.Fatalf("比例面裁出来的结果拿等宽面量只有 %.2f（上限 %.2f）：这条判据区分不了两条裁切路径", proportionalOver, room)
	}
}

// TestFitRunCropFitsTheMonoFace 在真帧上复验裁切结果：把裁好的字符串按同一尺寸画出来，
// 快照里它的绘制宽度必须 <= 上限，且比它多一个字符就超。
func TestFitRunCropFitsTheMonoFace(t *testing.T) {
	const line = "feature/some-rather-long-branch-name"
	const room float32 = 80
	mono := gfx.TextStyle{Size: 12, Mono: true}
	var cropped, longer string
	snapshot := measureSnapshot(t, func(canvas *gfx.Canvas) {
		cropped = fitRun(canvas, line, mono, room)
		full := []rune(strings.TrimSuffix(cropped, "…"))
		// 「多要一个字符」= 前缀 + 1 个字符**再补省略号**：省略号自己占一格，
		// 所以只有带省略号的那一段才算"放不下"（不带省略号时它会和裁好的那段同宽）。
		longer = string([]rune(line)[:len(full)+1]) + "…"
		canvas.Label(cropped, gfx.Rect{X: 20, Y: 20, Width: room, Height: 20}, mono)
		canvas.Label(longer, gfx.Rect{X: 20, Y: 60, Width: room, Height: 20}, mono)
	})
	if cropped == "" || longer == "" {
		t.Fatal("裁切没产出文本")
	}
	croppedFound, longerFound := false, false
	found := 0
	for _, text := range snapshot.Texts {
		switch text.Value {
		case cropped:
			croppedFound, found = true, found+1
		case longer:
			longerFound, found = true, found+1
		}
	}
	if found != 2 {
		t.Fatalf("快照里没找到两段复验文本（找到 %d 段）", found)
	}
	// 两段都画在同一个宽度上限里：裁切那段按等宽面量必须放得下（单元格数 × 单元宽），
	// 多一个字符的那段则放不下——这正是裁切"取最长可放前缀"的定义性质。
	cell := monoCellWidth(snapshot)
	if cell <= 0 {
		t.Fatal("量不到等宽单元格宽")
	}
	if width := float32(len([]rune(cropped))) * cell; width > room {
		t.Fatalf("裁好的那段占 %.2f（上限 %.2f）", width, room)
	}
	if width := float32(len([]rune(longer))) * cell; width <= room {
		t.Fatalf("多一个字符那段才占 %.2f（上限 %.2f）：裁得太短", width, room)
	}
	_ = croppedFound
	_ = longerFound
}

// monoCellWidth 从这一帧的等宽文本里取单元格宽。
func monoCellWidth(snapshot app.Snapshot) float32 {
	for _, text := range snapshot.Texts {
		if text.Style.Mono && text.CellWidth > 0 {
			return text.CellWidth
		}
	}
	return 0
}

// TestPaintMenuLabelUsesTheOfficialTypeScale 钉住分组标题的字号/墨色/内边距读数
// （官方原语 `._label_70kcw_214{padding:6px 8px;font-size:11px;line-height:15px;
// color:label-tertiary}`）。
func TestPaintMenuLabelUsesTheOfficialTypeScale(t *testing.T) {
	theme := Dark()
	row := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 28}
	snapshot := measureSnapshot(t, func(canvas *gfx.Canvas) {
		PaintMenuLabel(ui.MenuItemParts{
			Canvas: canvas, Rect: row, Label: "选择基准分支", GroupLabel: true,
			// 行给的 Foreground 是 label-primary：分组标题**不该**用它。
			Foreground: theme.LabelPrimary,
		}, theme)
	})
	var title gfx.Text
	found := false
	for _, text := range snapshot.Texts {
		if text.Value == "选择基准分支" {
			title, found = text, true
		}
	}
	if !found {
		t.Fatal("分组标题没画出来")
	}
	if title.Style.Size != MenuLabelFont {
		t.Fatalf("分组标题字号 = %v，官方 `._label` 是 %v", title.Style.Size, MenuLabelFont)
	}
	if title.Style.Color.Red != theme.LabelTertiary.Red || title.Style.Color.Green != theme.LabelTertiary.Green ||
		title.Style.Color.Blue != theme.LabelTertiary.Blue {
		t.Fatalf("分组标题墨色 = %v，官方是 label-tertiary（不看行的 Foreground）", title.Style.Color)
	}
	if title.Bounds.X != row.X+MenuLabelPadX || title.Bounds.Y != row.Y+MenuLabelPadY {
		t.Fatalf("分组标题落在 (%v,%v)，官方 `padding:6px 8px` 应是 (%v,%v)",
			title.Bounds.X, title.Bounds.Y, row.X+MenuLabelPadX, row.Y+MenuLabelPadY)
	}
}
