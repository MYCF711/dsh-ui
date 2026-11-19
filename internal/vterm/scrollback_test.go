package vterm

import (
	"strings"
	"testing"
)

// 这一组钉住**滚回**这一层：容量（官方 xterm 的 scrollback 选项）、窗口快照的
// 坐标与夹取、以及「关掉滚回」时的语义。它们都不碰渲染器——渲染器只看
// Grid / Window 的投影。

// manyLines 造 n 行文本，每行带自己的行号，方便按内容断言窗口落到了哪几行。
func manyLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("line ")
		b.WriteString(itoa(i))
		b.WriteString("\r\n")
	}
	return b.String()
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

// scrollbackText 把窗口的一行读成去掉尾部空白的文本。
func scrollbackText(g *Grid, y int) string {
	var b strings.Builder
	for x := 0; x < g.Width; x++ {
		c, ok := g.Cell(x, y)
		if !ok {
			break
		}
		if c.Width == 0 {
			continue
		}
		if c.Content == "" {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(c.Content)
	}
	return strings.TrimRight(b.String(), " ")
}

// TestSetScrollbackSizeFollowsOfficialOption 钉住容量来自调用方：
// 官方 `terminal.tsx:104` 把 xterm 的 `scrollback` 设成
// `environment?.scrollback ?? 0`，所以「宿主说几行就是几行」。
//
// 上游默认 10000 行（`x/vt@v0.1.0/screen.go:26`），不设就会滚出 xterm 里根本
// 看不到的历史——这一条是那个差异的守卫。
func TestSetScrollbackSizeFollowsOfficialOption(t *testing.T) {
	const rows = 5
	e := New(20, rows)
	e.SetScrollbackSize(3)
	write(t, e, manyLines(20))
	if got := e.ScrollbackLen(); got != 3 {
		t.Fatalf("容量设成 3 行时滚回最多 3 行，实际 %d 行", got)
	}
	// 淘汰的是最老的：留下的应是最后被推进去的 3 行。
	g := e.Scrollback()
	if g.Height != 3 {
		t.Fatalf("滚回快照高度该是 3，实际 %d", g.Height)
	}
	for y := 0; y < g.Height; y++ {
		text := scrollbackText(g, y)
		if !strings.HasPrefix(text, "line ") {
			t.Fatalf("第 %d 行不是正文：%q", y, text)
		}
	}
}

// TestScrollbackZeroDisablesHistory 钉住官方 `scrollback: 0` 的语义：
// **没有滚回**。上游 `SetMaxLines(<=0)` 是空操作（默认 10000 会留着），
// 所以这条路必须由本包的投影报 0 —— 否则界面会给一个「滚不动的历史」留 14px 槽。
func TestScrollbackZeroDisablesHistory(t *testing.T) {
	e := New(20, 3)
	e.SetScrollbackSize(0)
	write(t, e, manyLines(20))
	if got := e.ScrollbackLen(); got != 0 {
		t.Fatalf("scrollback=0 时不该有历史，实际 %d 行", got)
	}
	if g := e.Scrollback(); g.Height != 0 {
		t.Fatalf("scrollback=0 时整块快照也不该有行，实际 %d", g.Height)
	}
	if g := e.Window(0, 3); g.Height != 3 || scrollbackText(g, 0) == "" {
		t.Fatalf("窗口仍该能读到当前屏（3 行），实际 %+v", g)
	}
	// 再打开：容量是真的生效（不是永久关掉）。
	e.SetScrollbackSize(4)
	write(t, e, manyLines(20))
	if got := e.ScrollbackLen(); got == 0 || got > 4 {
		t.Fatalf("重新设 4 行后滚回应在 1..4 之间，实际 %d", got)
	}
}

// TestWindowSpansScrollbackThenScreen 钉住窗口的坐标口径：
// 0 是滚回里最老的一行，`ScrollbackLen()` 是第一行**屏幕**行，
// 总行数是 `ScrollbackLen()+屏幕高`。
func TestWindowSpansScrollbackThenScreen(t *testing.T) {
	const rows = 4
	e := New(20, rows)
	e.SetScrollbackSize(100)
	write(t, e, manyLines(30)) // 30 行写进 4 行屏 → 滚回 26 行，屏幕是最后 4 行

	sb := e.ScrollbackLen()
	if sb == 0 {
		t.Fatal("写了 30 行到 4 行屏里却没有滚回")
	}
	total := sb + rows
	window := e.Window(0, rows)
	if window.Height != rows {
		t.Fatalf("窗口高度该是请求的 %d，实际 %d", rows, window.Height)
	}
	if window.Width != 20 {
		t.Fatalf("窗口宽度该是屏幕宽 20，实际 %d", window.Width)
	}
	// 最老的一行必须是第一行写进去的（"line 0"）。
	if got := scrollbackText(window, 0); got != "line 0" {
		t.Fatalf("窗口第一行该是最老的一行 \"line 0\"，实际 %q", got)
	}
	// 窗口跨过屏幕顶：最后一行应该是滚回的最后一行（屏幕顶上一行），不是屏幕行。
	last := e.Window(total-rows, rows) // = Window(sb, rows) 即整屏
	if got := scrollbackText(last, 0); got != scrollbackText(e.Grid(), 0) {
		t.Fatalf("从 ScrollbackLen 起的窗口第一行该是屏幕第一行：窗口 %q，屏幕 %q",
			got, scrollbackText(e.Grid(), 0))
	}
	// 窗口钻进屏幕里：sb+1 起的第一行该是屏幕的第二行。
	inside := e.Window(sb+1, 1)
	if got, want := scrollbackText(inside, 0), scrollbackText(e.Grid(), 1); got != want {
		t.Fatalf("虚拟行 %d 该是屏幕第 2 行 %q，实际 %q", sb+1, want, got)
	}
}

// TestWindowClampsOutOfRange 钉住越界夹住而不是 panic 或返回垃圾：
// 渲染器算错一步就可能请求负起点或越过底边的窗口，它必须能安全画。
func TestWindowClampsOutOfRange(t *testing.T) {
	e := New(10, 3)
	e.SetScrollbackSize(50)
	write(t, e, manyLines(10))
	sb := e.ScrollbackLen()
	if sb == 0 {
		t.Fatal("没有滚回，测不了夹取")
	}
	if g := e.Window(-5, 2); g.Height != 2 || scrollbackText(g, 0) != "line 0" {
		t.Fatalf("负起点该夹到 0，实际 %+v 首行 %q", g, scrollbackText(g, 0))
	}
	if g := e.Window(0, 0); g.Height != 0 || len(g.Cells) != 0 {
		t.Fatalf("lines<=0 该是空窗口，实际 %+v", g)
	}
	far := e.Window(sb+100, 5)
	if far.Height != 0 {
		t.Fatalf("起点越过总行数时该是空窗口，实际 %d 行", far.Height)
	}
	tail := e.Window(sb, 100)
	if tail.Height != 3 {
		t.Fatalf("越过底边的窗口该截短到屏幕高 3，实际 %d", tail.Height)
	}
}

// TestWindowHidesCursor 钉住「滚回里没有光标」：官方只在 ydisp==ybase（贴底）
// 时画光标，滚上去那一帧用 Window，光标必须不可见，否则会有一条假光标停在历史行里。
func TestWindowHidesCursor(t *testing.T) {
	e := New(20, 3)
	e.SetScrollbackSize(50)
	write(t, e, manyLines(10))
	if !e.Grid().Cursor.Visible {
		t.Fatal("当前屏的光标该是可见的（对照组）")
	}
	if g := e.Window(0, 3); g.Cursor.Visible {
		t.Fatalf("滚回窗口不该带光标，实际 %+v", g.Cursor)
	}
}

// TestWindowSurvivesResize 钉住「resize 之后历史不丢」：这是验收里的一条
// （窗口变宽/变高不该把已经滚出去的行清掉）。上游在 Resize 里不碰
// scrollback（`screen.go` 只在 ScrollUp/DeleteLine 时 PushN），这条是那个事实的守卫。
func TestWindowSurvivesResize(t *testing.T) {
	e := New(20, 4)
	e.SetScrollbackSize(100)
	write(t, e, manyLines(26))
	before := e.ScrollbackLen()
	if before == 0 {
		t.Fatal("没有滚回，测不了 resize 前后的对比")
	}
	oldest := scrollbackText(e.Window(0, 1), 0)
	e.Resize(30, 6) // 变宽 + 变高
	after := e.ScrollbackLen()
	if after == 0 {
		t.Fatalf("resize 之后历史被清空了（之前 %d 行）", before)
	}
	if got := scrollbackText(e.Window(0, 1), 0); got != oldest {
		t.Fatalf("resize 之后最老的一行变了：之前 %q，现在 %q", oldest, got)
	}
	// 窗口宽度跟着屏幕走：resize 后是 30 列。
	if g := e.Window(0, 2); g.Width != 30 {
		t.Fatalf("resize 后窗口宽度该是 30，实际 %d", g.Width)
	}
}
