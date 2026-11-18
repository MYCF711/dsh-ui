package dsterminal

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 网格渲染的测试：全部用**真** `internal/vterm` 模拟器造屏幕，再用无头画布
// 把页面画一遍，最后读画布上的图元（矩形 / label / run）断言。
//
// 格子尺寸由一个固定的 cell measurer 给出（10×16），这样断言里的坐标全部是
// 「第几格 × 格宽」，而不是渲染器自己算出来又自己验证自己。真实字体宽度来自
// 平台引擎，测试不猜它。

const (
	gridTestCellW float32 = 10
	gridTestCellH float32 = 16
	gridTestPageW float32 = 360
	gridTestPageH float32 = 600 // 24 行 × 16 高 + 内边距，够放下整屏
)

// gridTestOrigin 是网格左上角：root 的 padding-top（8）加 screen 的四边内距（8）。
func gridTestOrigin() (x, y float32) {
	return terminalScreenPad, terminalRootPadTop + terminalScreenPad
}

// emulatorGrid 用真模拟器造一块屏幕，返回它的网格快照。
func emulatorGrid(t *testing.T, w, h int, writes ...string) *vterm.Grid {
	t.Helper()
	e := vterm.New(w, h)
	for _, s := range writes {
		if _, err := e.Write([]byte(s)); err != nil {
			t.Fatalf("写入模拟器失败：%v", err)
		}
	}
	return e.Grid()
}

// terminalGridCanvas 把终端的右栏面板画到无头画布上。
func terminalGridCanvas(t *testing.T, grid *vterm.Grid, clock func() time.Duration) (*gfx.Canvas, *pluginapi.Theme) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	if clock != nil {
		canvas.SetClock(clock)
	}
	state := pluginapi.TerminalViewState{
		Phase: "connected", Writable: true,
		Screen: "stripped projection",
		Grid:   grid,
		Info:   &apiclient.TerminalInfo{Title: "终端 1", State: "running", Cols: grid.Width, Rows: grid.Height},
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-grid"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: true},
		Act: pluginapi.Actions{
			TerminalEnsure:        func(string) {},
			TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return state },
			TerminalRetry:         func(string) {},
			OpenRightTab:          func(string) {},
			CloseRightTab:         func(string) {},
			TerminalWriteOfficial: func(string, string, bool) {},
		},
	}
	p := &Plugin{theme: theme}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}, func(f ui.C) {
		p.drawPanelBody(f, env)
	})
	return canvas, theme
}

// gridGridRow 报告一条 label 是第几行（按行高换算）。
func gridGridRow(text gfx.Text) int {
	_, originY := gridTestOrigin()
	return int(math.Round(float64(text.Bounds.Y-originY) / float64(gridTestCellH)))
}

// gridRowTexts 把画出来的网格还原成「行号 → 该行字形（去掉行尾空白）」。
//
// 还原必须**按列**放：一条 label 是一个 Value 加若干 run，每个 run 只说自己从
// 第几列开始、跨几格。跳过的空格格不在 Value 里，所以缺列要补空格，
// 否则 "progress 100%" 会被读成 "progress100%"。
func gridRowTexts(canvas *gfx.Canvas) map[int]string {
	out := map[int]string{}
	chosen := map[int]float32{}
	for _, text := range canvas.Texts() {
		if text.RunCount == 0 || text.Bounds.Width <= gridTestCellW {
			continue // 块光标里那条单格 label 不是整行
		}
		row := gridGridRow(text)
		if width, ok := chosen[row]; ok && width >= text.Bounds.Width {
			continue
		}
		chosen[row] = text.Bounds.Width
		out[row] = gridLabelText(canvas, text)
	}
	return out
}

// gridLabelText 按列把一条网格 label 还原成字符串。
func gridLabelText(canvas *gfx.Canvas, text gfx.Text) string {
	var b strings.Builder
	column := 0
	offset := 0
	for _, run := range canvas.TextRuns(text) {
		end := offset + int(run.Bytes)
		for column < int(run.Column) {
			b.WriteByte(' ')
			column++
		}
		b.WriteString(text.Value[offset:end])
		offset = end
		column = int(run.Column) + int(run.Cells)
	}
	return strings.TrimRight(b.String(), " ")
}

// gridRun 是一条字形 run 的可读投影。
type gridRun struct {
	column    int
	cells     int
	value     string
	color     gfx.Color
	weight    float32
	underline bool
}

// gridRuns 取第 row 行的字形 run（按顺序）。它拒绝块光标里那条单格 label。
func gridRuns(canvas *gfx.Canvas, row int) []gridRun {
	for _, text := range canvas.Texts() {
		if text.RunCount == 0 || gridGridRow(text) != row || text.Bounds.Width <= gridTestCellW {
			continue
		}
		var out []gridRun
		offset := 0
		for _, run := range canvas.TextRuns(text) {
			end := offset + int(run.Bytes)
			out = append(out, gridRun{
				column: int(run.Column), cells: int(run.Cells), value: text.Value[offset:end],
				color: run.Color, weight: run.Weight, underline: run.Underline,
			})
			offset = end
		}
		return out
	}
	return nil
}

// gridRowLabel 返回第 row 行那条整行 label。
func gridRowLabel(canvas *gfx.Canvas, row int) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.RunCount == 0 || gridGridRow(text) != row || text.Bounds.Width <= gridTestCellW {
			continue
		}
		return text, true
	}
	return gfx.Text{}, false
}

// gridFillsAt 返回落在 rect 内的纯色块（光标 / 反显 / 背景用）。
func gridFillsAt(canvas *gfx.Canvas, want gfx.Rect) []gfx.Rectangle {
	var out []gfx.Rectangle
	for _, rect := range canvas.Rectangles() {
		if rect.Bounds == want {
			out = append(out, rect)
		}
	}
	return out
}

// TestGridRendersCursorAddressedFrame 钉住「靠光标定位绘制」这条路：
// vim / htop 那种先跳行列再写的程序，画面上必须是**覆盖后的屏幕**，
// 不是按写入顺序拼起来的一串碎片。
func TestGridRendersCursorAddressedFrame(t *testing.T) {
	grid := emulatorGrid(t, 20, 4, "aaaaaaaaaa\r\nbbbbbbbbbb\r\ncccccccccc\r\ndddddddddd\x1b[2;3Hxy")
	canvas, _ := terminalGridCanvas(t, grid, nil)

	rows := gridRowTexts(canvas)
	want := map[int]string{
		0: "aaaaaaaaaa",
		1: "bbxybbbbbb",
		2: "cccccccccc",
		3: "dddddddddd",
	}
	for row, expected := range want {
		if rows[row] != expected {
			t.Fatalf("第 %d 行应画成 %q，实际 %q（整屏 %v）", row, expected, rows[row], rows)
		}
	}
}

// TestGridRedrawOnlyShowsFinalFrame 是任务要求的那条：`\r` 原地重画的进度条，
// 画面上只能有**最后那一帧**，中间帧一个像素都不该留。
func TestGridRedrawOnlyShowsFinalFrame(t *testing.T) {
	// 换行用 `\r\n`：pty 开着 ONLCR，真实输出到我们手里就是 `\r\n`
	// （裸 `\n` 只下移不回列 0，那不是真实链路上的字节）。
	grid := emulatorGrid(t, 20, 2, "progress  0%\rprogress 50%\rprogress 100%\r\ndone")
	canvas, _ := terminalGridCanvas(t, grid, nil)

	rows := gridRowTexts(canvas)
	if rows[0] != "progress 100%" {
		t.Fatalf("第 1 行应是最后一帧 %q，实际 %q", "progress 100%", rows[0])
	}
	if rows[1] != "done" {
		t.Fatalf("第 2 行应是 %q，实际 %q", "done", rows[1])
	}
	for row, text := range rows {
		for _, stale := range []string{"progress  0%", "progress 50%"} {
			if strings.Contains(text, stale) {
				t.Fatalf("第 %d 行残留了已被重画掉的中间帧 %q：%q", row, stale, text)
			}
		}
	}
}

// TestGridCJKDoubleWidthAlignment 钉住全角字的等宽对齐：
// 一个全角字占**两格**，它后面那一格是占位（不画字），再后面的字从第 3 列开始。
// 这条错了的典型症状是「中文后面多出空格 / 后续字符整体偏移一格」。
func TestGridCJKDoubleWidthAlignment(t *testing.T) {
	grid := emulatorGrid(t, 10, 1, "中a中")
	canvas, _ := terminalGridCanvas(t, grid, nil)

	runs := gridRuns(canvas, 0)
	want := []gridRun{
		{column: 0, cells: 2, value: "中"},
		{column: 2, cells: 1, value: "a"},
		{column: 3, cells: 2, value: "中"},
	}
	if len(runs) != len(want) {
		t.Fatalf("应当只有 3 条 run（全角各占一条、占位格不画），实际 %d 条：%+v", len(runs), runs)
	}
	for i, w := range want {
		got := runs[i]
		if got.column != w.column || got.cells != w.cells || got.value != w.value {
			t.Fatalf("第 %d 条 run 应是 col=%d cells=%d %q，实际 col=%d cells=%d %q",
				i, w.column, w.cells, w.value, got.column, got.cells, got.value)
		}
	}
	if got := runs[0].column + runs[0].cells; got != runs[1].column {
		t.Fatalf("全角字之后的 run 必须从它的第二格之后开始：全角结束于 %d，实际下一条从 %d 开始", got, runs[1].column)
	}

	// 整行 label 的格子契约：格宽来自画布实测，基线由画布放进这一格高的行盒。
	label, ok := gridRowLabel(canvas, 0)
	if !ok {
		t.Fatal("找不到第 1 行的整行 label")
	}
	if label.CellWidth != gridTestCellW || label.Bounds.Height != gridTestCellH {
		t.Fatalf("一格应是 %v×%v，实际 CellWidth=%v Height=%v", gridTestCellW, gridTestCellH, label.CellWidth, label.Bounds.Height)
	}
	x, y := gridTestOrigin()
	if label.Bounds.X != x || label.Bounds.Y != y {
		t.Fatalf("网格左上角应在 (%v,%v)，实际 (%v,%v)", x, y, label.Bounds.X, label.Bounds.Y)
	}
}

// TestGridCursorBlockCoversCell 钉住块光标：位置 = 光标所在格，底色用光标色，
// 格里的字用对比色重画（官方 terminal.module.css:13-14）。
func TestGridCursorBlockCoversCell(t *testing.T) {
	grid := emulatorGrid(t, 10, 2, "ab\x1b[1;2H\x1b[2 q")
	canvas, theme := terminalGridCanvas(t, grid, nil)

	x, y := gridTestOrigin()
	cell := gfx.Rect{X: x + gridTestCellW, Y: y, Width: gridTestCellW, Height: gridTestCellH}
	fills := gridFillsAt(canvas, cell)
	if len(fills) != 1 {
		t.Fatalf("光标格 (1,0) 应有且只有一块填充，实际 %d：%+v", len(fills), fills)
	}
	fill, accent := terminalCursorColors(theme.LabelPrimary, theme.BgBase)
	if fills[0].Color != fill {
		t.Fatalf("块光标底色应是光标色 %+v，实际 %+v", fill, fills[0].Color)
	}

	// 光标格里的字（"b"）用对比色画在光标上。
	var glyph gfx.Text
	found := false
	for _, text := range canvas.Texts() {
		if text.RunCount == 0 || text.Bounds != cell || text.Value != "b" {
			continue
		}
		glyph, found = text, true
	}
	if !found {
		t.Fatalf("光标格里的字必须用对比色重画一遍，实际图元：%+v", canvas.Texts())
	}
	if runs := canvas.TextRuns(glyph); len(runs) != 1 || runs[0].Color != accent {
		t.Fatalf("光标上的字应是 accent %+v，实际 %+v", accent, canvas.TextRuns(glyph))
	}
}

// TestGridCursorShapes 钉住另外两种官方形状：bar 是格内左侧 1px，
// underline 是格子底边 1px（terminal.module.css:16-17）。
func TestGridCursorShapes(t *testing.T) {
	x, y := gridTestOrigin()
	cases := []struct {
		name  string
		seq   string
		want  gfx.Rect
		check string
	}{
		{"bar", "\x1b[6 q", gfx.Rect{
			X: x + 2*gridTestCellW, Y: y + gridTestCellH, Width: terminalCursorBarWidth, Height: gridTestCellH,
		}, "bar 光标是格内左侧 1px 的竖条"},
		{"underline", "\x1b[4 q", gfx.Rect{
			X:      x + 2*gridTestCellW,
			Y:      y + 2*gridTestCellH - terminalCursorUnderlineHeight,
			Width:  gridTestCellW,
			Height: terminalCursorUnderlineHeight,
		}, "underline 光标是格子底边 1px 的横线"},
	}
	for _, c := range cases {
		grid := emulatorGrid(t, 10, 3, "abc\x1b[2;3H"+c.seq)
		canvas, _ := terminalGridCanvas(t, grid, nil)
		if fills := gridFillsAt(canvas, c.want); len(fills) != 1 {
			t.Fatalf("%s：%s，实际填充 %+v", c.name, c.check, canvas.Rectangles())
		}
	}
}

// TestGridCursorHiddenAndBlink 钉住可见性与闪烁：
//   - DECTCEM 隐藏后整格不画光标；
//   - 闪烁相位按官方 CSS 的 1s（0–500ms 画、500–1000ms 不画），
//     并且会**预约下一帧**，否则静态画布上光标就永远停在第一帧。
func TestGridCursorHiddenAndBlink(t *testing.T) {
	x, y := gridTestOrigin()
	cell := gfx.Rect{X: x + 2*gridTestCellW, Y: y, Width: gridTestCellW, Height: gridTestCellH}

	hidden := emulatorGrid(t, 10, 2, "abc\x1b[1;3H\x1b[?25l")
	if canvas, _ := terminalGridCanvas(t, hidden, nil); len(gridFillsAt(canvas, cell)) != 0 {
		t.Fatalf("光标隐藏后不该画任何光标填充：%+v", canvas.Rectangles())
	}

	blinking := func() *vterm.Grid {
		// 1 = 闪烁块光标。
		return emulatorGrid(t, 10, 2, "abc\x1b[1;3H\x1b[1 q")
	}
	onCanvas, _ := terminalGridCanvas(t, blinking(), func() time.Duration { return 0 })
	if len(gridFillsAt(onCanvas, cell)) != 1 {
		t.Fatalf("闪烁相位 0ms 时光标应当画出来：%+v", onCanvas.Rectangles())
	}
	if _, asked := onCanvas.NextFrame(); !asked {
		t.Fatal("闪烁光标必须预约下一帧，否则画布停在第一帧不动")
	}

	offCanvas, _ := terminalGridCanvas(t, blinking(), func() time.Duration { return 600 * time.Millisecond })
	if fills := gridFillsAt(offCanvas, cell); len(fills) != 0 {
		t.Fatalf("闪烁相位 600ms 时光标应当淡出（画单元格自己的颜色）：%+v", fills)
	}

	steady := emulatorGrid(t, 10, 2, "abc\x1b[1;3H\x1b[2 q")
	steadyCanvas, _ := terminalGridCanvas(t, steady, func() time.Duration { return 600 * time.Millisecond })
	if len(gridFillsAt(steadyCanvas, cell)) != 1 {
		t.Fatal("steady 光标不随相位闪烁")
	}
}

// TestGridColorsReverseUnderlineAndIndexed 钉住颜色与属性画到画布上：
// 基本索引色、反显换色、下划线；默认前景/背景来自主题。
func TestGridColorsReverseUnderlineAndIndexed(t *testing.T) {
	grid := emulatorGrid(t, 10, 1, "\x1b[31mR\x1b[0m\x1b[7mV\x1b[0m\x1b[4mU\x1b[0mP")
	canvas, theme := terminalGridCanvas(t, grid, nil)

	runs := gridRuns(canvas, 0)
	if len(runs) < 4 {
		t.Fatalf("应有红字 / 反显 / 下划线 / 普通四段，实际 %+v", runs)
	}
	if want := gfx.RGB(0xcc, 0x00, 0x00); runs[0].color != want {
		t.Fatalf("SGR 31 应画官方调色板的 #cc0000，实际 %+v", runs[0].color)
	}
	if runs[1].color != theme.BgBase {
		t.Fatalf("反显格的字应当用背景色画，实际 %+v", runs[1].color)
	}
	if !runs[2].underline {
		t.Fatalf("SGR 4 的字形必须带下划线：%+v", runs[2])
	}
	if runs[3].color != theme.LabelPrimary {
		t.Fatalf("没有显式颜色的字必须用主题 label-primary，实际 %+v", runs[3].color)
	}

	// 反显格的背景块用前景色铺满那一格。
	x, y := gridTestOrigin()
	if fills := gridFillsAt(canvas, gfx.Rect{
		X: x + gridTestCellW, Y: y, Width: gridTestCellW, Height: gridTestCellH,
	}); len(fills) != 1 || fills[0].Color != theme.LabelPrimary {
		t.Fatalf("反显格的背景应是 label-primary，实际 %+v", fills)
	}
}

// TestGridClipsToVisibleColumns 钉住「屏幕比绘制面宽时按整格裁」：
// 只画放得下的列，不缩放、不半格偏移。
func TestGridClipsToVisibleColumns(t *testing.T) {
	grid := emulatorGrid(t, 80, 3, strings.Repeat("x", 80))
	canvas, _ := terminalGridCanvas(t, grid, nil)

	label, ok := gridRowLabel(canvas, 0)
	if !ok {
		t.Fatal("找不到整行 label")
	}
	visible := int(label.Bounds.Width / gridTestCellW)
	inner := float32(gridTestPageW - 2*terminalScreenPad)
	wantVisible := int(inner / gridTestCellW)
	if visible != wantVisible {
		t.Fatalf("放得下的列数应是 %d（360-2×8 内距 ÷ 10 格宽），实际 %d", wantVisible, visible)
	}
	for _, run := range gridRuns(canvas, 0) {
		if end := run.column + run.cells; end > visible {
			t.Fatalf("run %+v 越过可见列数 %d", run, visible)
		}
	}
}

// TestGridOverlayUsesMeasuredRowHeight 钉住 F5 浮层那条路：浮层高度由内容决定，
// 所以行高必须来自画布实测（不是 13px 字号），且行数封顶 maxLines。
func TestGridOverlayUsesMeasuredRowHeight(t *testing.T) {
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	// 30 行都有字：空白行不产生 label，也就没法用「画了几条 label」数行数。
	var screen string
	for i := 0; i < 30; i++ {
		if i > 0 {
			screen += "\r\n"
		}
		screen += fmt.Sprintf("row %02d", i)
	}
	grid := emulatorGrid(t, 40, 30, screen)
	state := pluginapi.TerminalViewState{
		Phase: "connected", Writable: true, Screen: "$ echo hi", Grid: grid,
		Info: &apiclient.TerminalInfo{Title: "终端 1", State: "running", Cols: 40, Rows: 30},
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-overlay"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: false},
		Act: pluginapi.Actions{
			TerminalEnsure:        func(string) {},
			TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return state },
			TerminalWriteOfficial: func(string, string, bool) {},
		},
	}
	p := &Plugin{theme: theme, open: true}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: 760, Height: 800}, func(f ui.C) {
		p.draw(f, env)
	})

	// 浮层的网格原点不是右栏面板那一套（它有自己的 Top/边框/内距），所以这里
	// 按「行 label 的 Y 排序」取行，不假设固定原点。
	var lines []gfx.Text
	for _, text := range canvas.Texts() {
		if text.RunCount == 0 || text.Bounds.Width <= gridTestCellW {
			continue
		}
		lines = append(lines, text)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].Bounds.Y < lines[j].Bounds.Y })
	if len(lines) == 0 {
		t.Fatal("浮层没画出网格")
	}
	// 网格 30 行、浮层封顶 maxLines 行：高度必须是 18×实测行高，才正好放下 18 行。
	// 少一行（例如退回 13px 字号算高度）就说明行高没用画布实测值。
	if len(lines) != maxLines {
		t.Fatalf("浮层应画 %d 行（30 行封顶），实际 %d 行", maxLines, len(lines))
	}
	for _, line := range lines {
		if line.Bounds.Height != gridTestCellH {
			t.Fatalf("浮层的行盒应是实测行高 %v，实际 %+v", gridTestCellH, line.Bounds)
		}
	}
	if got := gridLabelText(canvas, lines[0]); got != "row 00" {
		t.Fatalf("第 1 行应是 %q，实际 %q", "row 00", got)
	}
	if got := gridLabelText(canvas, lines[1]); got != "row 01" {
		t.Fatalf("第 2 行应是 %q，实际 %q", "row 01", got)
	}
}

// TestGridFallsBackToTextWithoutGrid 钉住退路：壳还没给网格时仍按行画文本，
// 且**不**画任何光标/底色块（没有屏幕就假装不了）。
func TestGridFallsBackToTextWithoutGrid(t *testing.T) {
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-fallback"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: true},
		Act: pluginapi.Actions{
			TerminalEnsure: func(string) {},
			TerminalSnapshot: func(string) pluginapi.TerminalViewState {
				return pluginapi.TerminalViewState{
					Phase: "connected", Writable: true, Screen: "$ echo hi\nhello",
					Info: &apiclient.TerminalInfo{Title: "终端 1", State: "running"},
				}
			},
		},
	}
	p := &Plugin{theme: theme}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}, func(f ui.C) {
		p.drawPanelBody(f, env)
	})

	var got []string
	found := false
	for _, text := range canvas.Texts() {
		got = append(got, text.Value)
		if text.Value != "$ echo hi" {
			continue
		}
		found = true
		// 退路的行盒是 13px（旧文本路径），网格路径是实测行高 16px——
		// 用行盒区分「走的是哪条路」，比看颜色更直接。
		if text.Bounds.Height != terminalFontSize {
			t.Fatalf("退路应按 13px 行盒画文本，实际 %+v", text.Bounds)
		}
	}
	joined := strings.Join(got, "|")
	if !found || !strings.Contains(joined, "hello") {
		t.Fatalf("没有网格时应按行画文本投影，实际 %q", joined)
	}
}
