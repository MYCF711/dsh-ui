package vterm

import (
	"runtime"
	"strings"
	"testing"
)

// 选区模型的测试：全部用**真**模拟器造屏幕（`internal/vterm` 的 Emulator），
// 不手搓 Grid——「选中的文本与屏幕一致」只能靠真屏幕来钉。
//
// 每个数都来自官方 xterm 的口径（见 selection.go 顶部逐条字节偏移），
// 断言里写的是那个口径的结果，不是实现里的常量。

// selectionScreen 造一块屏幕并把 writes 按顺序写进去。
func selectionScreen(t *testing.T, w, h int, writes ...string) *Grid {
	t.Helper()
	e := New(w, h)
	for _, s := range writes {
		if _, err := e.Write([]byte(s)); err != nil {
			t.Fatalf("写入模拟器失败：%v", err)
		}
	}
	return e.Grid()
}

// wantSeparator 是官方 `isWindows ? "\r\n" : "\n"`（client.terminal.js byte 117677）。
func wantSeparator() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

// TestSelectionTextLinearSingleLine 钉住「同一行内按列取字」+ 行尾空白不进剪贴板。
func TestSelectionTextLinearSingleLine(t *testing.T) {
	grid := selectionScreen(t, 20, 3, "hello world   \r\n")
	sel := Selection{Start: SelPos{X: 0, Line: 0}, End: SelPos{X: 5, Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, sel); got != "hello" {
		t.Fatalf("0..5 该是 %q，实际 %q", "hello", got)
	}
	// 整行（到行尾）：行尾的 3 个空格被 trimRight 掉（官方 translateToString(true)）。
	whole := Selection{Start: SelPos{X: 0, Line: 0}, End: SelPos{X: 20, Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, whole); got != "hello world" {
		t.Fatalf("整行该是 %q，实际 %q", "hello world", got)
	}
}

// TestSelectionTextCrossLine 钉住跨行：首行从起点到最后、中间整行、末行到终点，
// 行间用官方平台连接符。
func TestSelectionTextCrossLine(t *testing.T) {
	grid := selectionScreen(t, 10, 4, "abc\r\ndef\r\nghi\r\n")
	sel := Selection{Start: SelPos{X: 1, Line: 0}, End: SelPos{X: 2, Line: 2}, HasEnd: true}
	want := strings.Join([]string{"bc", "def", "gh"}, wantSeparator())
	if got := SelectionText(grid, 0, grid.Width, sel); got != want {
		t.Fatalf("跨行选取该是 %q，实际 %q", want, got)
	}
}

// TestSelectionTextReversedDrag 钉住反向拖选：起点在右下、终点在左上时，
// 文本按阅读顺序给出（官方 finalSelectionStart/End 互换两端）。
func TestSelectionTextReversedDrag(t *testing.T) {
	grid := selectionScreen(t, 10, 4, "abc\r\ndef\r\n")
	forward := Selection{Start: SelPos{X: 1, Line: 0}, End: SelPos{X: 2, Line: 1}, HasEnd: true}
	reversed := Selection{Start: SelPos{X: 2, Line: 1}, End: SelPos{X: 1, Line: 0}, HasEnd: true}
	want := strings.Join([]string{"bc", "de"}, wantSeparator())
	if got := SelectionText(grid, 0, grid.Width, forward); got != want {
		t.Fatalf("正向该是 %q，实际 %q", want, got)
	}
	if got := SelectionText(grid, 0, grid.Width, reversed); got != want {
		t.Fatalf("反向拖选该给出同一段文本 %q，实际 %q", want, got)
	}
	if !reversed.Reversed() {
		t.Fatal("起点在终点右下方却没被判成反向")
	}
}

// TestSelectionTextColumnMode 钉住 Alt 拖的矩形选区：每行取同一段列。
func TestSelectionTextColumnMode(t *testing.T) {
	grid := selectionScreen(t, 12, 4, "abcdef\r\nghijkl\r\nmnopqr\r\n")
	sel := Selection{Start: SelPos{X: 1, Line: 0}, End: SelPos{X: 4, Line: 2}, HasEnd: true, Mode: SelColumn}
	lines := []string{grid.LineText(0, 1, 4), grid.LineText(1, 1, 4), grid.LineText(2, 1, 4)}
	want := strings.Join(lines, wantSeparator())
	if got := SelectionText(grid, 0, grid.Width, sel); got != want {
		t.Fatalf("矩形选区该是 %q，实际 %q", want, got)
	}
	if got := SelectionText(grid, 0, grid.Width, sel); got == "bcdef" {
		t.Fatal("矩形选区退化成线性了：第一行取到了列 4 之后的内容")
	}
}

// TestSelectionWideCharsAreNotCut 钉住 CJK 双宽：宽字占两列，
// 选区跨过它时整字给出；起点落在**尾巴**格上要被挪开（官方同上）。
func TestSelectionWideCharsAreNotCut(t *testing.T) {
	grid := selectionScreen(t, 12, 3, "中文abc\r\n")
	// 中 = 列 0（宽 2），文 = 列 2（宽 2），a=4 b=5 c=6。
	if cell, ok := grid.Cell(0, 0); !ok || cell.Width != 2 || cell.Content != "中" {
		t.Fatalf("列 0 该是全角「中」（宽 2），实际 %+v ok=%v", cell, ok)
	}
	if !grid.WideTail(0, 1) {
		t.Fatal("列 1 该是「中」的尾巴格（Width==0）")
	}
	// 覆盖两个全角字（列 0..4）→ 「中文」，不能出现空格或半个字。
	sel := Selection{Start: SelPos{X: 0, Line: 0}, End: SelPos{X: 4, Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, sel); got != "中文" {
		t.Fatalf("列 0..4 该是 %q，实际 %q", "中文", got)
	}
	// 从尾巴格起选：官方把起点挪到下一格，所以选到的是「文」而不是半个字。
	if got := grid.SnapColumn(0, 1, grid.Width); got != 2 {
		t.Fatalf("尾巴格该挪到列 2，实际列 %d", got)
	}
	tail := Selection{Start: SelPos{X: grid.SnapColumn(0, 1, grid.Width), Line: 0}, End: SelPos{X: 4, Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, tail); got != "文" {
		t.Fatalf("从尾巴格起的选区该是 %q，实际 %q", "文", got)
	}
	// 终点落在尾巴格上同样要躲开：列 0..1 不该切半个「中」。
	half := Selection{Start: SelPos{X: 0, Line: 0}, End: SelPos{X: grid.SnapColumn(0, 1, grid.Width), Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, half); got != "中" {
		t.Fatalf("终点落在尾巴格上该选出整字 %q，实际 %q", "中", got)
	}
}

// TestSelectionTextReplacesNBSP 钉住官方收尾那一步：NBSP 换成普通空格
// （byte 115700 的 `new RegExp(String.fromCharCode(160),'g')`）。
func TestSelectionTextReplacesNBSP(t *testing.T) {
	grid := selectionScreen(t, 8, 2, "a\u00a0b\r\n")
	sel := Selection{Start: SelPos{X: 0, Line: 0}, End: SelPos{X: 3, Line: 0}, HasEnd: true}
	if got := SelectionText(grid, 0, grid.Width, sel); got != "a b" {
		t.Fatalf("NBSP 该换成普通空格，实际 %q", got)
	}
}

// TestSelectionWordAt 钉住双击词选：词分隔符取官方默认值
// `" ()[]{}',\"`"`，落在空格上时吃掉整串空格。
func TestSelectionWordAt(t *testing.T) {
	grid := selectionScreen(t, 20, 3, "foo bar  baz\r\n")
	if start, length, ok := grid.WordAt(0, 5, grid.Width); !ok || start != 4 || length != 3 {
		t.Fatalf("列 5 该命中 bar（起点 4、长 3），实际 start=%d len=%d ok=%v", start, length, ok)
	}
	sel, ok := grid.WordSelection(0, 0, 5, grid.Width)
	if !ok {
		t.Fatal("双击 bar 没算出选区")
	}
	if got := SelectionText(grid, 0, grid.Width, sel); got != "bar" {
		t.Fatalf("双击 bar 该选中 %q，实际 %q", "bar", got)
	}
	// 列 8/9 是空格串（"bar" 与 "baz" 之间两格）：整串给出。
	spaceSel, ok := grid.WordSelection(0, 0, 8, grid.Width)
	if !ok {
		t.Fatal("双击空格没算出选区")
	}
	if got := SelectionText(grid, 0, grid.Width, spaceSel); got != "  " {
		t.Fatalf("双击空格串该选中两格空格，实际 %q", got)
	}
}

// TestSelectionLineSelection 钉住三击整行：起点列 0、长度 = 列数、没有终点
// （官方 `_selectLineAt`），文本仍按 trimRight 收尾。
func TestSelectionLineSelection(t *testing.T) {
	grid := selectionScreen(t, 16, 3, "  padded line   \r\nnext\r\n")
	sel := LineSelection(0, grid.Width)
	if sel.HasEnd {
		t.Fatal("行选该没有终点（官方 selectionEnd=undefined）")
	}
	if got := SelectionText(grid, 0, grid.Width, sel); got != "  padded line" {
		t.Fatalf("三击该选中整行（行首空格是内容，行尾空白按 trimRight 去掉）%q，实际 %q", "  padded line", got)
	}
}

// TestSelectionNormalizedWrapsStartLength 钉住 `finalSelectionEnd` 的回卷分支：
// Start+StartLength 超过一行时，终点折到下一行（官方 byte 95563 的两条分支）。
func TestSelectionNormalizedWrapsStartLength(t *testing.T) {
	cols := 4
	cases := []struct {
		name  string
		sel   Selection
		wantX int
		wantY int
	}{
		{name: "整行长度 4 从列 0 起 = 同一行末尾", sel: Selection{Start: SelPos{0, 2}, StartLength: 4}, wantX: 4, wantY: 2},
		{name: "长度 5 从列 0 起 = 下一行列 1", sel: Selection{Start: SelPos{0, 2}, StartLength: 5}, wantX: 1, wantY: 3},
		{name: "长度 8 从列 0 起 = 下下行末列（x%cols==0 分支）", sel: Selection{Start: SelPos{0, 2}, StartLength: 8}, wantX: 4, wantY: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, end := tc.sel.Normalized(cols)
			if end.X != tc.wantX || end.Line != tc.wantY {
				t.Fatalf("终点该是 (%d,%d)，实际 (%d,%d)", tc.wantX, tc.wantY, end.X, end.Line)
			}
		})
	}
}

// TestSelectionContains 钉住「哪些格子被画成选中态」：线性与矩形两种模式的边界。
func TestSelectionContains(t *testing.T) {
	cols := 6
	linear := Selection{Start: SelPos{X: 2, Line: 1}, End: SelPos{X: 3, Line: 3}, HasEnd: true}
	cases := []struct {
		x, line int
		want    bool
	}{
		{0, 1, false}, // 首行起点之前
		{2, 1, true},
		{5, 1, true}, // 首行从起点到行尾都算
		{0, 2, true}, // 中间整行
		{5, 2, true},
		{2, 3, true},
		{3, 3, false}, // 末行到终点**不含**终点
		{0, 0, false},
		{0, 4, false},
	}
	for _, tc := range cases {
		if got := linear.Contains(cols, tc.x, tc.line); got != tc.want {
			t.Fatalf("线性选区 (%d,%d) 该是 %v，实际 %v", tc.x, tc.line, tc.want, got)
		}
	}
	column := Selection{Start: SelPos{X: 1, Line: 1}, End: SelPos{X: 4, Line: 3}, HasEnd: true, Mode: SelColumn}
	for _, tc := range []struct {
		x, line int
		want    bool
	}{
		{0, 1, false}, {1, 1, true}, {3, 2, true}, {4, 2, false}, {4, 3, false}, {1, 3, true},
	} {
		if got := column.Contains(cols, tc.x, tc.line); got != tc.want {
			t.Fatalf("矩形选区 (%d,%d) 该是 %v，实际 %v", tc.x, tc.line, tc.want, got)
		}
	}
}

// TestSelectionTextOverScrollbackWindow 钉住「滚回里的行也能选中」：
// 传入的是 Window() 切出来的窗口 + 它的首行号。
func TestSelectionTextOverScrollbackWindow(t *testing.T) {
	e := New(10, 2)
	if _, err := e.Write([]byte("one\r\ntwo\r\nthree\r\nfour\r\nfive\r\n")); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if n := e.ScrollbackLen(); n < 2 {
		t.Fatalf("滚回该至少 2 行，实际 %d", n)
	}
	first := e.ScrollbackLen() - 1 // 滚回里最后一行 + 屏幕两行
	window := e.Window(first, 3)
	if window == nil || window.Height < 2 {
		t.Fatalf("取不到窗口：%+v", window)
	}
	sel := Selection{Start: SelPos{X: 0, Line: first}, End: SelPos{X: 10, Line: first + 1}, HasEnd: true}
	want := strings.Join([]string{window.LineText(0, 0, -1), window.LineText(1, 0, -1)}, wantSeparator())
	if got := SelectionText(window, first, 10, sel); got != want {
		t.Fatalf("跨滚回与屏幕的选区该是 %q，实际 %q", want, got)
	}
}
