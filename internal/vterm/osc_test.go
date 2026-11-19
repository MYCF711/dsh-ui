package vterm

import "testing"

// 这一组钉住「程序改色」：OSC 4 / 104（调色板）与 OSC 10 / 11 / 12 / 110 / 111 / 112
// （默认前景 / 背景 / 光标色）。
//
// 判据分两层：
//   - 上游这一层：OSC 4 / 104 在 `x/vt@v0.1.0` 里**没有派发器**
//     （`registerDefaultOscHandlers` handlers.go:304-337 里没有 4 和 104），
//     所以这里必须真的改到上游那张表——断言走 `inner.IndexedColor`；
//   - 我们的这一层：渲染投影（`Grid`）必须看得见改色，断言走 `Cell.Style`。

// oscEmit 把一段 OSC 序列包成 ST 结尾（`ESC \`）。
func oscEmit(body string) string { return "\x1b]" + body + "\x1b\\" }

// oscColorOf 造一块屏幕，喂 writes，返回 (x,y) 这一格的投影。
func oscCellAt(t *testing.T, x, y int, writes ...string) Cell {
	t.Helper()
	e := New(20, 3)
	for _, w := range writes {
		if _, err := e.Write([]byte(w)); err != nil {
			t.Fatalf("写入模拟器失败：%v", err)
		}
	}
	grid := e.Grid()
	cell, ok := grid.Cell(x, y)
	if !ok {
		t.Fatalf("格 (%d,%d) 不存在", x, y)
	}
	return cell
}

// TestOSC4SetsIndexedColor 钉住 OSC 4 真的改到上游调色板，
// 并且 SGR 31 画出来就是程序给的颜色（官方 terminal-theme.ts:23-30 的 4 号观察者）。
func TestOSC4SetsIndexedColor(t *testing.T) {
	e := New(20, 3)
	if _, err := e.Write([]byte(oscEmit("4;1;#ff0000"))); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	// 上游那一层：调色板里 1 号现在是 #ff0000，不再是「没设置过」。
	if got, want := plainColor(e.inner.IndexedColor(1)), RGB(0xff, 0x00, 0x00); got != want {
		t.Fatalf("OSC 4 之后上游调色板 1 号应是 %#v，实际 %#v", want, got)
	}
	// 我们的这一层：SGR 31 的格子投影成 #ff0000。
	cell := oscCellAt(t, 0, 0, oscEmit("4;1;#ff0000"), "\x1b[31mR")
	if want := RGB(0xff, 0x00, 0x00); cell.Style.Fg != want {
		t.Fatalf("OSC 4 改 1 号色后 SGR 31 应画 %#v，实际 %#v", want, cell.Style.Fg)
	}
}

// TestOSC4MultiplePairs 钉住 OSC 4 一次写多对（官方那层是 `i += 2` 的成对循环）。
func TestOSC4MultiplePairs(t *testing.T) {
	seq := oscEmit("4;1;#ff0000;2;#00ff00")
	red := oscCellAt(t, 0, 0, seq, "\x1b[31mR")
	if want := RGB(0xff, 0x00, 0x00); red.Style.Fg != want {
		t.Fatalf("1 号色应改成 #ff0000，实际 %#v", red.Style.Fg)
	}
	green := oscCellAt(t, 0, 0, seq, "\x1b[32mG")
	if want := RGB(0x00, 0xff, 0x00); green.Style.Fg != want {
		t.Fatalf("2 号色应改成 #00ff00，实际 %#v", green.Style.Fg)
	}
}

// TestOSC4AffectsTruecolorCube 钉住 256 色里非基本色那一段也走同一张表：
// SGR 38;5;196 取的是 196 号，OSC 4 改它就要变。
func TestOSC4AffectsTruecolorCube(t *testing.T) {
	cell := oscCellAt(t, 0, 0, oscEmit("4;196;#123456"), "\x1b[38;5;196mX")
	if want := RGB(0x12, 0x34, 0x56); cell.Style.Fg != want {
		t.Fatalf("256 色 196 号应画 #123456，实际 %#v", cell.Style.Fg)
	}
}

// TestOSC4IgnoresQueryAndBadColor 钉住两种「不该动调色板」的输入：
// `?` 是查询（本包不答，见 osc.go 文件头），解析不出来的颜色整对跳过。
func TestOSC4IgnoresQueryAndBadColor(t *testing.T) {
	defaultRed := RGB(0xcc, 0x00, 0x00) // 官方 xterm DEFAULT_ANSI_COLORS 的 1 号
	for _, seq := range []string{oscEmit("4;1;?"), oscEmit("4;1;notacolor"), oscEmit("4;999;#ff0000")} {
		cell := oscCellAt(t, 0, 0, seq, "\x1b[31mR")
		if cell.Style.Fg != defaultRed {
			t.Fatalf("%q 不该改调色板，1 号应是默认 %#v，实际 %#v", seq, defaultRed, cell.Style.Fg)
		}
	}
}

// TestOSC104ResetsPalette 钉住两条复位：列出下标复位那一条，不带参数复位整张表。
func TestOSC104ResetsPalette(t *testing.T) {
	defaultRed := RGB(0xcc, 0x00, 0x00)
	one := oscCellAt(t, 0, 0, oscEmit("4;1;#ff0000"), oscEmit("104;1"), "\x1b[31mR")
	if one.Style.Fg != defaultRed {
		t.Fatalf("OSC 104;1 之后 1 号应回默认 %#v，实际 %#v", defaultRed, one.Style.Fg)
	}
	// 只复位列出的那一条，没列的不动。
	partial := oscCellAt(t, 0, 0, oscEmit("4;1;#ff0000;2;#00ff00"), oscEmit("104;1"), "\x1b[32mG")
	if want := RGB(0x00, 0xff, 0x00); partial.Style.Fg != want {
		t.Fatalf("OSC 104;1 不该动 2 号色（%#v），实际 %#v", want, partial.Style.Fg)
	}
	all := oscCellAt(t, 0, 0, oscEmit("4;1;#ff0000;2;#00ff00"), oscEmit("104"), "\x1b[32mG")
	if want := RGB(0x4e, 0x9a, 0x06); all.Style.Fg != want {
		t.Fatalf("OSC 104 之后 2 号应回默认 %#v，实际 %#v", want, all.Style.Fg)
	}
}

// TestOSC4BrightUsedByBold 钉住「粗体走 8–15 档」这条也吃到 OSC 4：
// 官方 xterm 默认 drawBoldTextInBrightColors，SGR 1;31 画的是 9 号色。
func TestOSC4BrightUsedByBold(t *testing.T) {
	cell := oscCellAt(t, 0, 0, oscEmit("4;9;#00ff00"), "\x1b[1;31mR")
	if want := RGB(0x00, 0xff, 0x00); cell.Style.Fg != want {
		t.Fatalf("9 号色改成 #00ff00 后 SGR 1;31 应画它，实际 %#v", cell.Style.Fg)
	}
}

// TestOSCDefaultColors 钉住 OSC 10 / 11 / 12 进快照的 Defaults
// （官方 terminal-theme.ts:39-48 把它们落到 theme 的 foreground / background / cursor）。
func TestOSCDefaultColors(t *testing.T) {
	e := New(20, 3)
	for _, w := range []string{oscEmit("10;#ff0000"), oscEmit("11;#112233"), oscEmit("12;#00ff00")} {
		if _, err := e.Write([]byte(w)); err != nil {
			t.Fatalf("写入失败：%v", err)
		}
	}
	def := e.Grid().Default
	if want := RGB(0xff, 0x00, 0x00); def.Fg != want {
		t.Fatalf("OSC 10 之后默认前景应是 %#v，实际 %#v", want, def.Fg)
	}
	if want := RGB(0x11, 0x22, 0x33); def.Bg != want {
		t.Fatalf("OSC 11 之后默认背景应是 %#v，实际 %#v", want, def.Bg)
	}
	if want := RGB(0x00, 0xff, 0x00); def.Cursor != want {
		t.Fatalf("OSC 12 之后光标色应是 %#v，实际 %#v", want, def.Cursor)
	}
}

// TestOSCDefaultColorResets 钉住 OSC 110 / 111 / 112 清掉对应那一档，
// 其余两档不受影响（官方是 `special.delete(key)` 单个 key）。
func TestOSCDefaultColorResets(t *testing.T) {
	writes := []string{
		oscEmit("10;#ff0000"), oscEmit("11;#112233"), oscEmit("12;#00ff00"),
		oscEmit("110"),
	}
	e := New(20, 3)
	for _, w := range writes {
		if _, err := e.Write([]byte(w)); err != nil {
			t.Fatalf("写入失败：%v", err)
		}
	}
	def := e.Grid().Default
	if def.Fg.Set {
		t.Fatalf("OSC 110 之后默认前景应回到「没改过」，实际 %#v", def.Fg)
	}
	if want := RGB(0x11, 0x22, 0x33); def.Bg != want {
		t.Fatalf("OSC 110 不该动默认背景（%#v），实际 %#v", want, def.Bg)
	}
	if want := RGB(0x00, 0xff, 0x00); def.Cursor != want {
		t.Fatalf("OSC 110 不该动光标色（%#v），实际 %#v", want, def.Cursor)
	}

	// 复位之后，上游 getter 回的就是 defaultFg——这正是我们另记一份的原因。
	if got := plainColor(e.inner.ForegroundColor()); got == RGB(0xff, 0x00, 0x00) {
		t.Fatalf("上游复位后不该还留着 #ff0000，实际 %#v", got)
	}
}

// TestOSC10WritesAllThreeDefaults 钉住官方那条 X11 语义：OSC 10 后面可以跟多个
// 颜色，依次写前景 / 背景 / 光标（terminal-theme.ts:41-45 的 `specialKeys[offset+i]`）。
func TestOSC10WritesAllThreeDefaults(t *testing.T) {
	e := New(20, 3)
	if _, err := e.Write([]byte(oscEmit("10;#010203;#040506;#070809"))); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	def := e.Grid().Default
	for _, c := range []struct {
		name string
		got  Color
		want Color
	}{
		{"前景", def.Fg, RGB(0x01, 0x02, 0x03)},
		{"背景", def.Bg, RGB(0x04, 0x05, 0x06)},
		{"光标", def.Cursor, RGB(0x07, 0x08, 0x09)},
	} {
		if c.got != c.want {
			t.Fatalf("OSC 10 三连应把%s写成 %#v，实际 %#v", c.name, c.want, c.got)
		}
	}
}

// TestOSCDefaultsAbsentByDefault 钉住全新屏幕三档都是「没改过」——
// 渲染器据此落回主题色，官方也是 `special` 空表时用 defaults。
func TestOSCDefaultsAbsentByDefault(t *testing.T) {
	def := New(20, 3).Grid().Default
	if def.Fg.Set || def.Bg.Set || def.Cursor.Set {
		t.Fatalf("没写过 OSC 时三档都该是 Set=false，实际 %#v", def)
	}
}
