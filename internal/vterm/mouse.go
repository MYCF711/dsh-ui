package vterm

// 这个文件是**鼠标上报**（xterm 的 mouse tracking）：程序用 DECSET 打开上报之后，
// 指针手势要变成 xterm 的鼠标序列**写回 PTY**。这里只有两件事：
//
//  1. 记住「当前是哪种鼠标协议 + 哪种坐标编码」——由 DECSET/DECRST 驱动；
//  2. 把一次已归一化的 MouseInput 换成要写进 PTY 的字节（含官方的拒绝规则）。
//
// 指针怎么变成 MouseInput 在 `plugins/dsterminal/mouse.go`（那层知道格宽格高与
// 修饰键），写回 PTY 走既有的 terminal input 动作。屏幕状态、渲染都不在这里。
//
// # 官方规格（xterm 打进官方包里的 `client.terminal.js`，509421 字节）
//
// 模式（DECSET 私有模式 → 协议 / 编码，`setModePrivate` byte 180614-181187）：
//
//	9     → activeProtocol = X10        （只报按下）
//	1000  → activeProtocol = VT200      （按下/抬起/滚轮）
//	1002  → activeProtocol = DRAG       （+ 按住拖动）
//	1003  → activeProtocol = ANY        （+ 无键移动）
//	1004  → sendFocus（焦点上报，不是鼠标）
//	1005  → **不支持**，只记一句日志 `DECSET 1005 not supported (see #2507)`（byte 180952）
//	1006  → activeEncoding = SGR（byte 181031）
//	1015  → **不支持**，同样只记日志（byte 181092）
//	1016  → activeEncoding = SGR_PIXELS（byte 181171）
//	1001  → **没有 case**，官方直接忽略（highlight tracking 是 xterm 有、xterm.js 没有）
//
// DECRST（byte 182572 / 182802）：
//
//	9 / 1000 / 1002 / 1003 任意一个 → activeProtocol = NONE
//	1006 / 1016 任意一个           → activeEncoding = DEFAULT
//
// 也就是**后收到的 DECSET 说了算**（不是"号码大的赢"），DECRST 一批共用同一个效果。
//
// 协议能报哪些事件（`MOUSE_PROTOCOLS`，byte 265711-265902 的 events 位掩码，
// 与官方 `explainEvents`: down=1 / up=2 / drag=4 / move=8 / wheel=16 一致）：
//
//	X10   events 1  restrict: 只收 action==1（按下）、不收滚轮，且**清掉修饰键**
//	VT200 events 19 restrict: 不收 action==32（移动）
//	DRAG  events 23 restrict: action==32 只允许按住（button!=3）
//	ANY   events 31 restrict: 全收
//
// 一个事件从指针到字节（`triggerMouseEvent`，byte 268302-268640）：
//
//	1. 格号必须在屏幕内（byte 268323：`col<0||col>=cols||row<0||row>=rows` 丢弃）
//	2. 滚轮的"移动"不存在（byte 268421：`4===button&&32===action` 丢弃）
//	3. "无键"事件只能是移动（byte 268461：`3===button&&32!==action` 丢弃）
//	4. 非滚轮不接受 action 2/3
//	5. **格号加一**（byte 268556：`e.col++,e.row++`）——所以 SGR 序列是 1-based
//	6. 移动去重（byte 268572：同点同键同修饰的连着两帧只发一次）
//	7. 过协议 restrict，再过编码器
//
// 编码器（byte 266170 / 266297 / 266393）：
//
//	DEFAULT（X10 三字节）  ESC [ M  Cb+32  Cx+32  Cy+32
//	SGR（1006）            ESC [ <  Cb ; Cx ; Cy  M   （抬起是小写 m）
//	SGR_PIXELS（1016）     ESC [ <  Cb ; Px ; Py  M   （像素坐标，**不加一**）
//
// Cb 的位（`h(e,t)`，byte 265934）：bit0-1 键号、bit2 Shift、bit3 Alt、bit4 Ctrl、
// bit5 移动、bit6 滚轮、bit7 附加键；抬起事件在 DEFAULT 里编成 3，在 SGR 里保住
// 真实键号并用后缀 `m` 表示（`0===action&&4!==button` 才是 `m`，byte 266313）。
//
// 坐标基：格坐标 0-based 进来、加一之后进序列（1-based，byte 268556）；像素坐标
// 从屏幕左上角算 0-based、原样进序列（`getMouseReportCoords`，byte 107127 返回的
// `x/y` 就是像素偏移，SGR_PIXELS 直接用）。
//
// # 上游 `x/vt@v0.1.0` 到哪一步（判据，不猜）
//
// **状态有、派发有、编码有，但都不能直接拿来用**：
//
//   - 解析/状态：`mode.go:12-22` 有 9/1000/1001/1002/1003/1006 的默认值，
//     `csi_mode.go:9-35` 的 `handleMode` 从 DECSET/DECRST 写状态，`handlers.go:765-785`
//     把 `CSI ? h` / `CSI ? l` 接到它上面。**没有公开的 getter**——`isModeSet`
//     （`csi_mode.go:109-113`）是未导出的。
//   - 派发：`setMode` 末尾按开/关分别调 `Callbacks.EnableMode` / `DisableMode`
//     （`csi_mode.go:98-106`；`callbacks.go:56-64`）。**这就是公开挂载点**——
//     与 OSC 4/104 那次同一个形状（见 osc.go 的文件头）。
//   - 编码：`Emulator.SendMouse`（`mouse.go:62-115`）确实把事件编成 X10 / SGR 并
//     `io.WriteString(e.pw, …)`。但它的输出去处是**内部 pipe**（`emulator.go:101`
//     `io.Pipe()` 无缓冲、`emulator.go:264-271` 只能靠 `Read` 取走），本仓的
//     vterm 没有读者——探针实测 `SendMouse` **直接阻塞**（`tmp/GO8-RC5/_mouse/probe`）。
//   - 它的模式选择是「按它自己表里的顺序取最后一个被设的」（`mouse.go:70-80`），
//     不是官方的"后设的赢"：探针实测 `DECSET 1002` 之后 `DECSET 1000`，上游照发
//     拖动上报（官方这时是 VT200、不该发移动）。它也把 1001 当协议（官方忽略），
//     不实现 1016 的像素编码，没有官方的 restrict / 边界 / 去重。
//
// 所以这里**不自造模式编码，也不重写解析**：状态从上游的派发回调里接（`registerMouse`），
// 编码用上游同一套 `ansi` 函数（`ansi.EncodeMouseButton` / `MouseX10` / `MouseSgr`，
// 正是 `mouse.go:101-113` 用的那几个）。唯一自己拼的是 SGR_PIXELS 那一行格式化
// （上方 byte 266393 的官方原文），因为 `ansi` 没有像素编码器。
//
// # 什么条件下可以删掉自己接的那一半
//
// 上游给出公开的 `MouseState` getter、`SendMouse` 改用可注入的 writer、并实现
// 1016 与官方 restrict 之后，删掉 `registerMouse` + `MouseSequence`，
// 直接调上游即可。删之前先看 `charmbracelet/x` 的 PR（本仓没有提这个 issue：
// 008 那次 OSC 4/104 已提 `charmbracelet/x#1049`，同一个包不必再开一条）。

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// MouseProtocol 是当前生效的鼠标协议（官方 `activeProtocol`）。
type MouseProtocol uint8

// 官方 `MOUSE_PROTOCOLS` 的四档 + 关闭（`reset()` 里是 `NONE`，byte 267553）。
const (
	MouseProtocolNone MouseProtocol = iota
	MouseProtocolX10
	MouseProtocolVT200
	MouseProtocolDrag
	MouseProtocolAny
)

// String 只用于日志与测试失败信息。
func (p MouseProtocol) String() string {
	switch p {
	case MouseProtocolX10:
		return "X10"
	case MouseProtocolVT200:
		return "VT200"
	case MouseProtocolDrag:
		return "DRAG"
	case MouseProtocolAny:
		return "ANY"
	}
	return "NONE"
}

// events 是官方的协议位掩码（down=1 / up=2 / drag=4 / move=8 / wheel=16，
// byte 265711-265902）。`0 !== events` 就是官方的 `areMouseEventsActive`。
func (p MouseProtocol) events() uint8 {
	switch p {
	case MouseProtocolX10:
		return 1
	case MouseProtocolVT200:
		return 19
	case MouseProtocolDrag:
		return 23
	case MouseProtocolAny:
		return 31
	}
	return 0
}

// Reporting 报告这个协议会不会往 PTY 发东西（官方 `areMouseEventsActive`，
// byte 267046：`0!==this._protocols[this._activeProtocol].events`）。
func (p MouseProtocol) Reporting() bool { return p.events() != 0 }

// ReportsWheel 报告这个协议收不收滚轮（events 的 bit 16）。X10 不收：
// 官方那档的 `s.wheel` 监听器就不会挂（byte 26995 是 `16&e?s.wheel`），
// 滚轮于是落回本地滚回。
func (p MouseProtocol) ReportsWheel() bool { return p.events()&16 != 0 }

// MouseEncoding 是坐标编码（官方 `activeEncoding`）。
type MouseEncoding uint8

const (
	// MouseEncodingDefault 是 xterm 原来的三字节 X10 编码（官方 `DEFAULT`）。
	MouseEncodingDefault MouseEncoding = iota
	// MouseEncodingSGR 是 DECSET 1006（`CSI < Cb ; Cx ; Cy M/m`）。
	MouseEncodingSGR
	// MouseEncodingSGRPixels 是 DECSET 1016（同 SGR 但坐标是像素）。
	MouseEncodingSGRPixels
)

// MouseState 是「当前鼠标上报状态」的快照值：协议 + 编码。
// 零值 = 没打开上报（协议 NONE、编码 DEFAULT），与官方 `reset()` 一致。
type MouseState struct {
	Protocol MouseProtocol
	Encoding MouseEncoding
}

// Reporting 报告要不要把指针手势交给 PTY（官方 `areMouseEventsActive`）。
func (m MouseState) Reporting() bool { return m.Protocol.Reporting() }

// MouseAction 是事件动作。官方那边是 DOM 事件的三个 action 码：
// 0 = 抬起、1 = 按下、32 = 移动（byte 268302 那段 `e.action`）。
type MouseAction uint8

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMotion
)

// MouseButton 是键号，取值照官方 `bindMouse` 的 DOM button 映射
// （byte 25458）：0 左、1 中、2 右、4 滚轮（6/7 是横向滚轮，属于"附加键"，
// 官方那支不报——见 `t.button<3?t.button:3` 的折叠）。
type MouseButton uint8

const (
	MouseNoButton MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
)

// MouseInput 是一次已归一化的指针事件：格坐标 0-based、像素坐标 0-based
// （相对屏幕左上角），修饰键是事件发生时的状态。
type MouseInput struct {
	Action MouseAction
	Button MouseButton
	// Col/Row 是**格坐标，0-based**；越界会被丢掉（官方 byte 268323）。
	Col, Row int
	// PixelX/PixelY 是**像素坐标，0-based**，只在 SGR_PIXELS 编码里用到
	// （官方 `getMouseReportCoords` 的 `x/y`，byte 107127）。
	PixelX, PixelY   int
	Shift, Alt, Ctrl bool
}

// registerMouse 把上游的模式派发接到本包的状态上。构造时调一次。
//
// 为什么挂回调而不是自己扫 DECSET：解析（含 `CSI ? 1002;1003 h` 这种一段多重、
// 私有/公有模式区分）在上游已经完整，重写只会多一份会漂的实现。上游的回调是
// **未过滤**的（任何 DECSET 号码都会派发，探针实测 1001/1005 也来了），所以
// 这里必须只认官方那六个号码。
func (e *Emulator) registerMouse() {
	e.inner.SetCallbacks(vt.Callbacks{
		EnableMode:  func(mode ansi.Mode) { e.observeMouseMode(mode, true) },
		DisableMode: func(mode ansi.Mode) { e.observeMouseMode(mode, false) },
	})
}

// observeMouseMode 把一次 DECSET/DECRST 落成官方语义。
//
// 注意这个函数是在上游 `Write` 持着 `SafeEmulator` 写锁时被调的：**绝不能**
// 在里面回头调 `e.inner` 的任何带锁方法（不可重入，见 osc.go 的 setIndexed 注）。
func (e *Emulator) observeMouseMode(mode ansi.Mode, set bool) {
	dec, ok := mode.(ansi.DECMode)
	if !ok {
		return
	}
	switch dec {
	case ansi.ModeMouseX10:
		e.setMouseProtocol(MouseProtocolX10, set)
	case ansi.ModeMouseNormal:
		e.setMouseProtocol(MouseProtocolVT200, set)
	case ansi.ModeMouseButtonEvent:
		e.setMouseProtocol(MouseProtocolDrag, set)
	case ansi.ModeMouseAnyEvent:
		e.setMouseProtocol(MouseProtocolAny, set)
	case ansi.ModeMouseExtSgr:
		e.setMouseEncoding(MouseEncodingSGR, set)
	case ansi.ModeMouseExtSgrPixel:
		e.setMouseEncoding(MouseEncodingSGRPixels, set)
	}
	// 其余一律忽略，与官方一致：
	//   1001（highlight）官方没有 case；
	//   1005（UTF-8 坐标）/ 1015（urxvt）官方只记 `not supported` 日志，
	//   编码**不改**（byte 180952 / 181092），所以这里也不动。
}

func (e *Emulator) setMouseProtocol(p MouseProtocol, set bool) {
	e.mouseMu.Lock()
	defer e.mouseMu.Unlock()
	if set {
		e.mouse.Protocol = p
		return
	}
	// 官方的 DECRST 是"这几个号码任一复位 → NONE"，不是"只复位自己那一档"
	// （byte 182572），所以关就是关，不看当前是哪一档。
	e.mouse.Protocol = MouseProtocolNone
}

func (e *Emulator) setMouseEncoding(enc MouseEncoding, set bool) {
	e.mouseMu.Lock()
	defer e.mouseMu.Unlock()
	if set {
		e.mouse.Encoding = enc
		return
	}
	e.mouse.Encoding = MouseEncodingDefault
}

// MouseState 返回当前鼠标上报状态（零值 = 没打开）。
//
// 读锁很轻，可以在每帧的视图快照里读（壳就是那么做的，见
// `cmd/app/terminal_wiring.go` 的 terminalSnapshot）。
func (e *Emulator) MouseState() MouseState {
	e.mouseMu.RLock()
	defer e.mouseMu.RUnlock()
	return e.mouse
}

// x10Offset 是 X10 三字节编码的偏移 32（官方 byte 266170 的 `+32`，
// 同一个数在 `ansi/mouse.go:142` 的 `x10Offset`）。
const x10Offset = 32

// MouseSequence 把一次手势换成要写进 PTY 的字节。
//
// 返回 false = **这次事件按官方不该发**（协议不收这类事件、坐标越界、
// 或 DEFAULT 编码装不下）。调用方据此既不上报，也不假装报过。
//
// cols/rows 是这一帧屏幕的列行数（官方 bounds 检查用的是 `bufferService.cols/rows`，
// byte 268323）。
func MouseSequence(state MouseState, in MouseInput, cols, rows int) (string, bool) {
	if !state.Protocol.Reporting() {
		return "", false
	}
	// 1) 屏幕外的事件官方直接丢（byte 268323）。
	if in.Col < 0 || in.Col >= cols || in.Row < 0 || in.Row >= rows {
		return "", false
	}
	wheel := in.Button == MouseWheelUp || in.Button == MouseWheelDown
	// 2) 滚轮没有"移动"（byte 268421）。
	if wheel && in.Action == MouseMotion {
		return "", false
	}
	// 3) "无键"只能是移动；按下/抬起必须带键号（byte 268461）。
	if in.Button == MouseNoButton && in.Action != MouseMotion {
		return "", false
	}

	shift, alt, ctrl := in.Shift, in.Alt, in.Ctrl
	switch state.Protocol {
	case MouseProtocolX10:
		// restrict（byte 265766）：只有按下、不收滚轮，且**清掉修饰键**。
		if in.Action != MousePress || wheel {
			return "", false
		}
		shift, alt, ctrl = false, false, false
	case MouseProtocolVT200:
		// restrict（byte 265818）：不收移动。
		if in.Action == MouseMotion {
			return "", false
		}
	case MouseProtocolDrag:
		// restrict（byte 265861）：移动必须按着键（button!=3）。
		if in.Action == MouseMotion && in.Button == MouseNoButton {
			return "", false
		}
	}

	button := ansiMouseButton(in.Button)
	if in.Action == MouseRelease && state.Encoding == MouseEncodingDefault {
		// DEFAULT 编码的抬起是键号 3（官方 `h(e,!1)` 里 `0!==action||t||(i|=3)`，
		// t=false 时抬起被编成 3），SGR 才保住真实键号 + 小写后缀。
		button = ansi.MouseNone
	}
	mask := ansi.EncodeMouseButton(button, in.Action == MouseMotion, shift, alt, ctrl)
	release := in.Action == MouseRelease

	switch state.Encoding {
	case MouseEncodingSGR:
		// `ansi.MouseSgr` 收 0-based 格号、自己加一（`ansi/mouse.go:165-173`），
		// 与官方 `e.col++` 之后进编码是同一个结果。
		return ansi.MouseSgr(mask, in.Col, in.Row, release), true
	case MouseEncodingSGRPixels:
		return mouseSGRPixels(mask, in.PixelX, in.PixelY, release), true
	}

	// DEFAULT 三字节：`ansi.MouseX10` 也收 0-based 并加 32+1（`ansi/mouse.go:150-152`）。
	// 官方在装不下一个字节时**直接不发**（byte 266170：`t[0]>255||t[1]>255||t[2]>255
	// ? '' : …`），所以要在这里先挡住，否则 ansi 的 `byte(x)` 会绕回成一个假坐标。
	if int(mask)+x10Offset > 255 || in.Col+1+x10Offset > 255 || in.Row+1+x10Offset > 255 {
		return "", false
	}
	return ansi.MouseX10(mask, in.Col, in.Row), true
}

// mouseSGRPixels 是官方的 SGR_PIXELS 编码（byte 266393）：
//
//	`ESC [ < Cb ; Px ; Py` + (`m` | `M`)
//
// 与 SGR 只差坐标：**像素、且不加一**（`triggerMouseEvent` 只给 col/row 加一，
// byte 268556；x/y 原样进编码）。`ansi` 没有这个编码器，所以这里照官方原文拼。
// 抬起后缀的判据同官方：`0===action && 4!==button` 才是 `m`。
func mouseSGRPixels(mask byte, x, y int, release bool) string {
	suffix := byte('M')
	if release {
		suffix = 'm'
	}
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", mask, x, y, suffix)
}

// ansiMouseButton 把本包的中性键号换成 `ansi` 的键号。
//
// 两边的值恰好一致（ansi 的 MouseButton1..5 = 左/中/右/滚上/滚下），这里仍写出
// 显式映射：上游改顺序时这里是编译期就错，而不是悄悄发错键。
func ansiMouseButton(b MouseButton) ansi.MouseButton {
	switch b {
	case MouseLeft:
		return ansi.MouseLeft
	case MouseMiddle:
		return ansi.MouseMiddle
	case MouseRight:
		return ansi.MouseRight
	case MouseWheelUp:
		return ansi.MouseWheelUp
	case MouseWheelDown:
		return ansi.MouseWheelDown
	}
	return ansi.MouseNone
}
