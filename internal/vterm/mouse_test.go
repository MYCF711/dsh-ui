package vterm

import "testing"

// 这一组钉住鼠标上报的两半：**状态**（DECSET/DECRST 落成协议+编码）与
// **序列**（一次手势变成哪些字节、哪些按官方不该发）。
//
// 官方的每条判据都在 mouse.go 的文件头列了 byte 偏移；这里断言的是同一批行为，
// 断言里写的是**字面量字节**（不引用常量），这样常量被改坏时测试照样红。

// mouseMode 把若干 DECSET 私有模式喂进屏幕，返回喂完后的状态。
func mouseMode(t *testing.T, seq string) MouseState {
	t.Helper()
	e := New(80, 24)
	if _, err := e.Write([]byte(seq)); err != nil {
		t.Fatalf("写入 DECSET 失败：%v", err)
	}
	return e.MouseState()
}

// TestMouseStateTracksDECSET 钉住「DECSET 号码 → 协议/编码」这张表
// （官方 setModePrivate，byte 180614-181187）。
func TestMouseStateTracksDECSET(t *testing.T) {
	cases := []struct {
		name string
		seq  string
		want MouseState
	}{
		{"没有 DECSET", "", MouseState{}},
		{"9 = X10", "\x1b[?9h", MouseState{Protocol: MouseProtocolX10}},
		{"1000 = VT200", "\x1b[?1000h", MouseState{Protocol: MouseProtocolVT200}},
		{"1002 = DRAG", "\x1b[?1002h", MouseState{Protocol: MouseProtocolDrag}},
		{"1003 = ANY", "\x1b[?1003h", MouseState{Protocol: MouseProtocolAny}},
		{"1006 = SGR", "\x1b[?1006h", MouseState{Encoding: MouseEncodingSGR}},
		{"1016 = SGR_PIXELS", "\x1b[?1016h", MouseState{Encoding: MouseEncodingSGRPixels}},
		{"协议+编码", "\x1b[?1002h\x1b[?1006h", MouseState{Protocol: MouseProtocolDrag, Encoding: MouseEncodingSGR}},
		// 官方没有 1001 的 case（highlight tracking 在 xterm.js 里不存在），
		// 1005/1015 只记日志且**不改编码**：这三条都不能改变状态。
		{"1001 不认", "\x1b[?1001h", MouseState{}},
		{"1005 不支持", "\x1b[?1005h", MouseState{}},
		{"1015 不支持", "\x1b[?1015h", MouseState{}},
		{"1005 不会把 1006 顶掉", "\x1b[?1006h\x1b[?1005h", MouseState{Encoding: MouseEncodingSGR}},
		// 后设的协议赢（不是"号码大的赢"）：1002 之后 1000 → VT200。
		{"后设的赢", "\x1b[?1002h\x1b[?1000h", MouseState{Protocol: MouseProtocolVT200}},
	}
	for _, c := range cases {
		if got := mouseMode(t, c.seq); got != c.want {
			t.Errorf("%s：状态该是 %+v，实际 %+v", c.name, c.want, got)
		}
	}
}

// TestMouseStateTracksDECRST 钉住 DECRST：9/1000/1002/1003 任何一个复位都回 NONE，
// 1006/1016 任何一个复位都回 DEFAULT（官方 byte 182572 / 182802）。
func TestMouseStateTracksDECRST(t *testing.T) {
	for _, seq := range []string{"\x1b[?9l", "\x1b[?1000l", "\x1b[?1002l", "\x1b[?1003l"} {
		got := mouseMode(t, "\x1b[?1003h\x1b[?1006h"+seq)
		if got != (MouseState{Encoding: MouseEncodingSGR}) {
			t.Errorf("%q 之后该只剩编码 SGR，实际 %+v", seq, got)
		}
	}
	for _, seq := range []string{"\x1b[?1006l", "\x1b[?1016l"} {
		got := mouseMode(t, "\x1b[?1002h\x1b[?1016h"+seq)
		if got != (MouseState{Protocol: MouseProtocolDrag}) {
			t.Errorf("%q 之后该只剩协议 DRAG，实际 %+v", seq, got)
		}
	}
	// 1001/1005/1015 的复位也不能动状态（官方没这些 case）。
	if got := mouseMode(t, "\x1b[?1002h\x1b[?1006h\x1b[?1001l\x1b[?1005l"); got !=
		(MouseState{Protocol: MouseProtocolDrag, Encoding: MouseEncodingSGR}) {
		t.Fatalf("1001/1005 的 DECRST 不该动状态，实际 %+v", got)
	}
}

// TestMouseSequenceSGR 钉住 1006 的序列形状（byte 266297）与坐标基：
// 格坐标 0-based 进来、**序列里是 1-based**（byte 268556 的 `e.col++,e.row++`）。
func TestMouseSequenceSGR(t *testing.T) {
	st := MouseState{Protocol: MouseProtocolVT200, Encoding: MouseEncodingSGR}
	cases := []struct {
		name string
		in   MouseInput
		want string
	}{
		{"左键按下 (0,0)", MouseInput{Action: MousePress, Button: MouseLeft, Col: 0, Row: 0}, "\x1b[<0;1;1M"},
		{"左键按下 (11,6)", MouseInput{Action: MousePress, Button: MouseLeft, Col: 11, Row: 6}, "\x1b[<0;12;7M"},
		{"左键抬起", MouseInput{Action: MouseRelease, Button: MouseLeft, Col: 11, Row: 6}, "\x1b[<0;12;7m"},
		{"中键按下", MouseInput{Action: MousePress, Button: MouseMiddle, Col: 2, Row: 3}, "\x1b[<1;3;4M"},
		{"右键按下", MouseInput{Action: MousePress, Button: MouseRight, Col: 2, Row: 3}, "\x1b[<2;3;4M"},
		{"右键抬起", MouseInput{Action: MouseRelease, Button: MouseRight, Col: 2, Row: 3}, "\x1b[<2;3;4m"},
		// 修饰键位：Shift=4、Alt=8、Ctrl=16（byte 265934 的 h()）。
		{"Shift", MouseInput{Action: MousePress, Button: MouseLeft, Col: 1, Row: 1, Shift: true}, "\x1b[<4;2;2M"},
		{"Alt", MouseInput{Action: MousePress, Button: MouseLeft, Col: 1, Row: 1, Alt: true}, "\x1b[<8;2;2M"},
		{"Ctrl", MouseInput{Action: MousePress, Button: MouseLeft, Col: 1, Row: 1, Ctrl: true}, "\x1b[<16;2;2M"},
		// 滚轮：键号 4 + bit6(64) + 方向位（官方 `i|=64;i|=action`，byte 265934）。
		{"滚上", MouseInput{Action: MousePress, Button: MouseWheelUp, Col: 4, Row: 4}, "\x1b[<64;5;5M"},
		{"滚下", MouseInput{Action: MousePress, Button: MouseWheelDown, Col: 4, Row: 4}, "\x1b[<65;5;5M"},
	}
	for _, c := range cases {
		got, ok := MouseSequence(st, c.in, 80, 24)
		if !ok || got != c.want {
			t.Errorf("%s：该是 %q（ok=true），实际 %q（ok=%v）", c.name, c.want, got, ok)
		}
	}
}

// TestMouseSequenceDefaultsToX10Encoding 钉住「没开 1006」时的三字节编码
// （byte 266170）：Cb/Cx/Cy 各 +32，坐标在加一之后再 +32，
// 抬起编成键号 3（`h(e,!1)` 里 `0!==action||t||(i|=3)`）。
func TestMouseSequenceDefaultsToX10Encoding(t *testing.T) {
	st := MouseState{Protocol: MouseProtocolVT200}
	cases := []struct {
		name string
		in   MouseInput
		want string
	}{
		{"左键按下 (0,0)", MouseInput{Action: MousePress, Button: MouseLeft, Col: 0, Row: 0}, "\x1b[M !!"},
		{"左键抬起", MouseInput{Action: MouseRelease, Button: MouseLeft, Col: 0, Row: 0}, "\x1b[M#!!"},
		{"右键按下 (3,1)", MouseInput{Action: MousePress, Button: MouseRight, Col: 3, Row: 1}, "\x1b[M\"$\""},
		{"滚上", MouseInput{Action: MousePress, Button: MouseWheelUp, Col: 0, Row: 0}, "\x1b[M`!!"},
		{"滚下", MouseInput{Action: MousePress, Button: MouseWheelDown, Col: 0, Row: 0}, "\x1b[Ma!!"},
	}
	for _, c := range cases {
		got, ok := MouseSequence(st, c.in, 80, 24)
		if !ok || got != c.want {
			t.Errorf("%s：该是 %q（ok=true），实际 %q（ok=%v）", c.name, c.want, got, ok)
		}
	}
	// 官方在装不下一个字节时**不发**（byte 266170 的 `t[0]>255||…?''`）：
	// 列 223 之后（223+1+32 = 256）就没有这个事件的表示。
	if got, ok := MouseSequence(st, MouseInput{Action: MousePress, Button: MouseLeft, Col: 223, Row: 0}, 400, 24); ok || got != "" {
		t.Fatalf("列 223 在 DEFAULT 编码里装不下，该不发，实际 %q（ok=%v）", got, ok)
	}
	if _, ok := MouseSequence(st, MouseInput{Action: MousePress, Button: MouseLeft, Col: 222, Row: 0}, 400, 24); !ok {
		t.Fatal("列 222 刚好装得下（222+1+32=255），该发")
	}
}

// TestMouseSequenceSGRPixels 钉住 1016：像素坐标**不加一**
// （官方 SGR_PIXELS 直接用 getMouseReportCoords 的 x/y，byte 266393）。
func TestMouseSequenceSGRPixels(t *testing.T) {
	st := MouseState{Protocol: MouseProtocolVT200, Encoding: MouseEncodingSGRPixels}
	cases := []struct {
		name string
		in   MouseInput
		want string
	}{
		{"像素 (0,0)", MouseInput{Action: MousePress, Button: MouseLeft, Col: 0, Row: 0}, "\x1b[<0;0;0M"},
		{"像素 (12,34)", MouseInput{Action: MousePress, Button: MouseLeft, Col: 1, Row: 2, PixelX: 12, PixelY: 34}, "\x1b[<0;12;34M"},
		{"像素抬起", MouseInput{Action: MouseRelease, Button: MouseLeft, Col: 1, Row: 2, PixelX: 12, PixelY: 34}, "\x1b[<0;12;34m"},
	}
	for _, c := range cases {
		got, ok := MouseSequence(st, c.in, 80, 24)
		if !ok || got != c.want {
			t.Errorf("%s：该是 %q（ok=true），实际 %q（ok=%v）", c.name, c.want, got, ok)
		}
	}
}

// TestMouseSequenceProtocolGates 钉住四档协议的 restrict（byte 265711-265902 的
// events/restrict 表）：哪些事件在哪个协议下**不该发**。
func TestMouseSequenceProtocolGates(t *testing.T) {
	press := MouseInput{Action: MousePress, Button: MouseLeft, Col: 1, Row: 1}
	release := MouseInput{Action: MouseRelease, Button: MouseLeft, Col: 1, Row: 1}
	motion := MouseInput{Action: MouseMotion, Button: MouseLeft, Col: 1, Row: 1}
	wheel := MouseInput{Action: MousePress, Button: MouseWheelUp, Col: 1, Row: 1}

	cases := []struct {
		name    string
		proto   MouseProtocol
		in      MouseInput
		want    bool
		wantSeq string
	}{
		// NONE：什么都没开。
		{"NONE 什么都不发", MouseProtocolNone, press, false, ""},
		// X10：只有按下；滚轮/抬起/移动都不发。
		{"X10 按下", MouseProtocolX10, press, true, "\x1b[M \"\""},
		{"X10 不发抬起", MouseProtocolX10, release, false, ""},
		{"X10 不发移动", MouseProtocolX10, motion, false, ""},
		{"X10 不发滚轮", MouseProtocolX10, wheel, false, ""},
		// VT200：按下/抬起/滚轮，不发移动。
		{"VT200 按下", MouseProtocolVT200, press, true, "\x1b[M \"\""},
		{"VT200 抬起", MouseProtocolVT200, release, true, "\x1b[M#\"\""},
		{"VT200 滚轮", MouseProtocolVT200, wheel, true, "\x1b[M`\"\""},
		{"VT200 不发移动", MouseProtocolVT200, motion, false, ""},
		// DRAG：移动要按着键；"无键移动"不发。
		{"DRAG 按住移动", MouseProtocolDrag, motion, true, "\x1b[M@\"\""},
		{"DRAG 不发无键移动", MouseProtocolDrag,
			MouseInput{Action: MouseMotion, Button: MouseNoButton, Col: 1, Row: 1}, false, ""},
		// ANY：连无键移动都发（键号 3 → Cb=32+3=35，即 '#'+32='C'）。
		{"ANY 无键移动", MouseProtocolAny,
			MouseInput{Action: MouseMotion, Button: MouseNoButton, Col: 1, Row: 1}, true, "\x1b[MC\"\""},
		// 滚轮的"移动"不存在（byte 268421）。
		{"滚轮移动不发", MouseProtocolAny,
			MouseInput{Action: MouseMotion, Button: MouseWheelUp, Col: 1, Row: 1}, false, ""},
		// "无键"只能是移动（byte 268461）。
		{"无键按下不发", MouseProtocolAny, MouseInput{Action: MousePress, Button: MouseNoButton, Col: 1, Row: 1}, false, ""},
	}
	for _, c := range cases {
		got, ok := MouseSequence(MouseState{Protocol: c.proto}, c.in, 80, 24)
		if ok != c.want {
			t.Errorf("%s：该发不发=%v，实际 %v（%q）", c.name, c.want, ok, got)
			continue
		}
		if c.wantSeq != "" && got != c.wantSeq {
			t.Errorf("%s：该是 %q，实际 %q", c.name, c.wantSeq, got)
		}
	}
}

// TestMouseSequenceX10ClearsModifiers 钉住 X10 的 restrict 会**清掉修饰键**
// （byte 265766：`e.ctrl=!1,e.alt=!1,e.shift=!1`）——X10 编码里没有修饰位。
func TestMouseSequenceX10ClearsModifiers(t *testing.T) {
	in := MouseInput{Action: MousePress, Button: MouseLeft, Col: 0, Row: 0, Shift: true, Alt: true, Ctrl: true}
	got, ok := MouseSequence(MouseState{Protocol: MouseProtocolX10}, in, 80, 24)
	if !ok || got != "\x1b[M !!" {
		t.Fatalf("X10 该忽略修饰键（键号 0），实际 %q（ok=%v）", got, ok)
	}
}

// TestMouseSequenceRejectsOffScreen 钉住边界检查（byte 268323）：
// 屏幕外的格号不发。
func TestMouseSequenceRejectsOffScreen(t *testing.T) {
	st := MouseState{Protocol: MouseProtocolDrag, Encoding: MouseEncodingSGR}
	for _, in := range []MouseInput{
		{Action: MousePress, Button: MouseLeft, Col: -1, Row: 0},
		{Action: MousePress, Button: MouseLeft, Col: 0, Row: -1},
		{Action: MousePress, Button: MouseLeft, Col: 10, Row: 0},
		{Action: MousePress, Button: MouseLeft, Col: 0, Row: 5},
	} {
		if got, ok := MouseSequence(st, in, 10, 5); ok || got != "" {
			t.Errorf("%+v 越界该不发，实际 %q（ok=%v）", in, got, ok)
		}
	}
	if _, ok := MouseSequence(st, MouseInput{Action: MousePress, Button: MouseLeft, Col: 9, Row: 4}, 10, 5); !ok {
		t.Fatal("最后一格（9,4）该发")
	}
}
