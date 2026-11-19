package dsterminal

// 这个文件是**鼠标上报**：程序在 PTY 那头用 DECSET 打开鼠标模式之后，用户在终端
// 上的按下/抬起/拖动/滚轮要变成 xterm 的鼠标序列写回 PTY。
//
// 两层分工：
//   - **模式与字节**在 `internal/vterm/mouse.go`（DECSET/DECRST 状态 + 官方编码，
//     每条都有官方 byte 偏移）；
//   - **这一个文件**是"指针事件 → 一次 MouseInput"：Viva 的窗口坐标换成格坐标与
//     像素坐标、修饰键、按住的键、以及**和本地选区的冲突**。
//
// # 官方怎么处理"鼠标模式 vs 本地选区"（这是本文件最要紧的一段）
//
//  1. 协议一开，本地选区就**整个关掉**：`onProtocolChange` 里
//     `enable-mouse-events` 时 `this._selectionService.disable()`，关掉时
//     `enable()`（官方 bundle byte 26666 / 26809；构造函数里也有一次同样的判断，
//     byte 24018 的 `areMouseEventsActive?(…disable()):(…enable())`）。
//  2. 按下的分派（`bindMouse` 的 mousedown，byte 27559）：
//
//	`if(e.preventDefault(),this.focus(),
//	    this.coreMouseService.areMouseEventsActive && !this._selectionService.shouldForceSelection(e))`
//	→ 上报给 PTY 并 `cancel(e)`；**否则**落到选区的 `handleMouseDown`。
//
//  3. `shouldForceSelection`（byte 120764）：
//
//	`isMac ? e.altKey && rawOptions.macOptionClickForcesSelection : e.shiftKey`
//
//	**非 mac 就是 Shift**。本产品只做 Windows（见 AGENTS.md 的支持范围），所以
//	这里取 Shift 那一支；这也是"Shift 强制本地选择"那句话的原文出处。
//
//  4. 选区服务自己的 `_enabled` 是 false（鼠标模式开着）时，`handleMouseDown`
//     只接受被强制的那一下（byte 120800 那段 `if(!this._enabled){if(!this.shouldForceSelection(e))return;…}`）。
//     顺带一个官方细节：`_enabled` 为假时 Shift 那一下走的是 `_handleSingleClick`
//     而不是增量点选（`_enabled&&e.shiftKey?…_handleIncrementalClick`），也就是说
//     鼠标模式下的 Shift+点击**重新起一段选区**，不是延长旧选区。本仓照这个来的
//     代价太大（`beginTerminalSelection` 已经按 `_enabled` 真分支写好了），所以
//     保留既有增量语义——登记为差异，因为它不影响"这一下归谁"。
//  5. **移动上报不看 Shift**：DRAG/ANY 的 mousemove 监听器只问 `buttons`
//     （byte 27240 `mousedrag:e=>{e.buttons&&i(e)}`、byte 26842 的 move 那支），
//     所以 Shift 强制选区拖起来时，motion 照发。这里照抄：这条**不是**自造。
//  6. 右键：官方 `contextmenu` 那条（`_initGlobal`，byte 17027）**不看鼠标模式**，
//     它只是把选区文本塞进 textarea 等原生菜单来复制；上报则照发（bindMouse 不
//     看键号）。本仓把"右键→原生菜单→复制"折叠成"右键有选区即复制"（见
//     selection.go 的文件头），所以这里两件事都做：先上报，再复制。
//
// 一处**登记差异**：官方 xterm 自己会把"用户输入"当成清选区的信号
// （`_coreService.onUserInput(() => hasSelection && clearSelection())`，byte 116812），
// 而鼠标上报是 `triggerDataEvent(t,!0)`（wasUserInput=true）发出去的——
// 照搬的话 Shift 强制选区会被随后第一帧 motion 上报清掉，与第 3 条自相矛盾。
// 本仓的"用户输入清选区"只挂在真键盘输入上（terminal.go 的 F6，照官方
// `onUserInput` 的语义），鼠标上报不清选区。
//
// # Viva 这边能给什么（`gfx.PointerEvent` / `gfx.ScrollEvent`）
//
//   - `Kind`：Down / Move / Up / Exit（`viva@v0.6.4/gfx/input.go:75-80`）；
//   - `Button`：Primary / Secondary / Middle / Back / Forward（`input.go:81-92`）——
//     前三个对应 DOM 的 0/1/2，侧键官方不报（bindMouse 把 `button>=3` 折成 3，
//     而 3 的按下会被丢掉，byte 25458 与 byte 268461）；
//   - `Modifiers`：Shift / Control / Alt / Meta（`input.go:61-67`）；
//   - `X/Y`：窗口逻辑像素；格坐标要自己除格宽格高（这一帧的几何在 rect 里）；
//   - **滚轮**在 `gfx.ScrollEvent`（`input.go:194-202`）：`DeltaY` 正数 = 往下滚
//     （与 DOM 同号，见 scrollback.go 的换算），`Modifiers` 一并带着。
//
// **一个 Viva 侧的能力缺口（登记，不做私有兜底）**：1003（ANY）要求"没按键也报
// 移动"，而 Viva 只把**按住之后的** Move 投递给画布：
// `gfx/canvas.go:2197`「Pointer moves that no press captured are not delivered this
// way: hover is Hovered」、`app/painter.go:769-777` 的 `pointerIsDelivered` 只在
// `captured` 时投递 Move，而 `Canvas.Hovered()` 只回一个 ID、没有坐标
// （`gfx/canvas.go:1962`）。所以 1003 的悬停移动报不出来；按住拖动是照报的。
// 这一条已写进 docs/official-parity.md 的 #54 行；最小复现与结论在
// `tmp/GO8-RC5/_mouse/`。
//
// 补它的路（不是本单的范围）：壳级的 `app.Config.OnPointer`（`viva@v0.6.4/app/app.go:120-126`）
// **每一帧都有坐标与 target**，缺的是"插件面"这一跳——pluginapi 现在只有
// HandleScroll / HandleKey 这种整手势入口，没有指针入口。要给 1003 补悬停，
// 得在契约里加一条指针回调并接进壳的 event pass，那会动契约与 uikernel，
// 不属于"终端鼠标上报"这一项。

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 按住的键位。官方的等价物是 DOM 的 `event.buttons` 位掩码
// （`1&buttons?0:4&buttons?1:2&buttons?2:3`，byte 25458）。
const (
	terminalMouseHeldLeft uint8 = 1 << iota
	terminalMouseHeldMiddle
	terminalMouseHeldRight
)

// terminalMouseState 是一条 occurrence 的手势簿记（官方 `CoreMouseService`
// 里那几个字段的等价物：`_lastEvent` 与 DOM 的 buttons）。
type terminalMouseState struct {
	// held 是当前按住的键（按下置位、抬起复位）——移动上报用它选键号。
	held uint8
	// reported 是**已经上报过按下**的键：官方只在报过按下的那一路挂 mouseup
	// 监听器（byte 27559 的分支里 `s.mouseup&&…addEventListener`），所以没报过
	// 的按下（比如 Shift 强制选区那一下）也不能报抬起。
	reported uint8
	// last 是上一次**成功发出去**的事件，用于官方那条移动去重
	// （byte 268572：`32===action&&this._lastEvent&&_equalEvents(…)` 就不发）。
	last    vterm.MouseInput
	lastSet bool
}

// mouseFor 取（必要时建）某个 occurrence 的手势簿记。
func (p *Plugin) mouseFor(tabID string) *terminalMouseState {
	if p.mouse == nil {
		p.mouse = map[string]*terminalMouseState{}
	}
	state := p.mouse[tabID]
	if state == nil {
		state = &terminalMouseState{}
		p.mouse[tabID] = state
	}
	return state
}

// forgetMouse 丢掉某个 occurrence 的簿记（终端重建/关闭时）。
func (p *Plugin) forgetMouse(tabID string) { delete(p.mouse, tabID) }

// holdTerminalMouse 记下「这个键现在按着」。
//
// 官方的 `event.buttons` 是浏览器给的，与"有没有把这一下上报出去"无关：
// Shift 强制选区的那一下归本地、不上报，但它照样按着一个键，随后拖动上报的
// Cb 必须是左键——DRAG 的 restrict 会把「无键移动」丢掉（byte 265861），
// 少了这一步，Shift 拖选时一帧移动都报不出去。
func holdTerminalMouse(m *terminalMouseState, b gfx.PointerButton, down bool) {
	_, bit, ok := terminalMouseButton(b)
	if !ok {
		return
	}
	if down {
		m.held |= bit
		return
	}
	m.held &^= bit
}

// motionButton 是移动事件该用哪个键号：照官方 `event.buttons` 的优先级
// 左→中→右，一个都没按就是"无键"（3）。
func (m *terminalMouseState) motionButton() vterm.MouseButton {
	switch {
	case m.held&terminalMouseHeldLeft != 0:
		return vterm.MouseLeft
	case m.held&terminalMouseHeldMiddle != 0:
		return vterm.MouseMiddle
	case m.held&terminalMouseHeldRight != 0:
		return vterm.MouseRight
	}
	return vterm.MouseNoButton
}

// terminalMouseButton 把 Viva 的键换成 xterm 的键号。
//
// `PointerButtonNone` 归到左键：官方那条链上"按下"只有一个来源 DOM button 0，
// 而 Viva 对没有按键信息的按下（触控/笔）给的是 None——既有选区代码也是这么
// 处理的（selection.go 的 `case gfx.PointerButtonPrimary, gfx.PointerButtonNone`）。
func terminalMouseButton(b gfx.PointerButton) (vterm.MouseButton, uint8, bool) {
	switch b {
	case gfx.PointerButtonPrimary, gfx.PointerButtonNone:
		return vterm.MouseLeft, terminalMouseHeldLeft, true
	case gfx.PointerButtonMiddle:
		return vterm.MouseMiddle, terminalMouseHeldMiddle, true
	case gfx.PointerButtonSecondary:
		return vterm.MouseRight, terminalMouseHeldRight, true
	}
	// 侧键（后退/前进）：官方折成 3 之后被丢掉（byte 25458 / 268461），不报。
	return vterm.MouseNoButton, 0, false
}

// terminalMouseCell 把窗口坐标换成格坐标（**0-based**，夹进屏幕内）。
//
// 官方的 `getMouseReportCoords`（byte 107127）先按 canvas 宽高夹住像素，再
// `Math.floor(x/cell.width)`；这里同一件事，只是上限用这一帧的 cols/rows。
func terminalMouseCell(rect ui.Rect, cellW, cellH float32, cols, rows int, x, y float32) (int, int) {
	px, py := terminalMousePixel(rect, cellW, cellH, cols, rows, x, y)
	col, row := 0, 0
	if cellW > 0 {
		col = int(float32(px) / cellW)
	}
	if cellH > 0 {
		row = int(float32(py) / cellH)
	}
	return min(max(col, 0), max(cols-1, 0)), min(max(row, 0), max(rows-1, 0))
}

// terminalMousePixel 把窗口坐标换成**屏幕内**的像素坐标（0-based）。
//
// 官方的 `x/y` 就是 `getCoordsRelativeToElement(window, e, screenElement)` 夹到
// canvas 宽高之后的值（byte 107127），SGR_PIXELS 原样用它（不加一）。
func terminalMousePixel(rect ui.Rect, cellW, cellH float32, cols, rows int, x, y float32) (int, int) {
	px := int(math.Floor(float64(x - rect.X)))
	py := int(math.Floor(float64(y - rect.Y)))
	maxX, maxY := 0, 0
	if cellW > 0 && cols > 0 {
		maxX = int(float32(cols)*cellW) - 1
	}
	if cellH > 0 && rows > 0 {
		maxY = int(float32(rows)*cellH) - 1
	}
	return min(max(px, 0), max(maxX, 0)), min(max(py, 0), max(maxY, 0))
}

// mouseInput 组一次上报用的 MouseInput（两个坐标系 + 修饰键）。
func (g terminalGridGeometry) mouseInput(ev gfx.PointerEvent, action vterm.MouseAction, button vterm.MouseButton) vterm.MouseInput {
	col, row := terminalMouseCell(g.rect, g.cellW, g.cellH, g.cols, g.rows, ev.X, ev.Y)
	px, py := terminalMousePixel(g.rect, g.cellW, g.cellH, g.cols, g.rows, ev.X, ev.Y)
	return vterm.MouseInput{
		Action: action, Button: button,
		Col: col, Row: row, PixelX: px, PixelY: py,
		Shift: ev.Modifiers&gfx.ModifierShift != 0,
		Alt:   ev.Modifiers&gfx.ModifierAlt != 0,
		Ctrl:  ev.Modifiers&gfx.ModifierControl != 0,
	}
}

// reportTerminalMousePress 上报一次按下；返回 true 表示**已经写进 PTY**
// （调用方据此决定这一下还归不归本地选区）。
func (p *Plugin) reportTerminalMousePress(geom terminalGridGeometry, m *terminalMouseState, ev gfx.PointerEvent) bool {
	button, bit, ok := terminalMouseButton(ev.Button)
	if !ok {
		return false
	}
	if !p.writeTerminalMouse(geom, m, geom.mouseInput(ev, vterm.MousePress, button)) {
		return false
	}
	m.reported |= bit
	return true
}

// reportTerminalMouseMotion 上报一次移动（DRAG/ANY 才有意义）。
func (p *Plugin) reportTerminalMouseMotion(geom terminalGridGeometry, m *terminalMouseState, ev gfx.PointerEvent) bool {
	return p.writeTerminalMouse(geom, m, geom.mouseInput(ev, vterm.MouseMotion, m.motionButton()))
}

// reportTerminalMouseRelease 上报一次抬起。
//
// 只有**报过按下**的键才报抬起（官方只在报过按下那一路挂 mouseup，byte 27559）。
func (p *Plugin) reportTerminalMouseRelease(geom terminalGridGeometry, m *terminalMouseState, ev gfx.PointerEvent) bool {
	button, bit, ok := terminalMouseButton(ev.Button)
	if !ok {
		return false
	}
	if m.reported&bit == 0 {
		return false
	}
	m.reported &^= bit
	return p.writeTerminalMouse(geom, m, geom.mouseInput(ev, vterm.MouseRelease, button))
}

// writeTerminalMouse 把一次事件交给 internal/vterm 编码并写进 PTY。
//
// `typed=false`：鼠标上报是**协议回复**，不是人员敲的字（与 terminal.go 里 F6
// 那条 `typed=true` 的分工一致）。
func (p *Plugin) writeTerminalMouse(geom terminalGridGeometry, m *terminalMouseState, in vterm.MouseInput) bool {
	// 官方那条移动去重：同点、同键、同修饰的连续移动只发一次
	// （byte 268572；像素编码下比像素，否则比格号，见 byte 268640 的 `_equalEvents`）。
	if in.Action == vterm.MouseMotion && m.lastSet && terminalSameMouseEvent(m.last, in, geom.mouse.Encoding) {
		return false
	}
	seq, ok := vterm.MouseSequence(geom.mouse, in, geom.cols, geom.rows)
	if !ok {
		return false
	}
	if p.act.TerminalWriteOfficial == nil {
		return false
	}
	p.act.TerminalWriteOfficial(geom.tabID, seq, false)
	m.last, m.lastSet = in, true
	return true
}

// terminalSameMouseEvent 是官方 `_equalEvents` 的等价物。
func terminalSameMouseEvent(a, b vterm.MouseInput, encoding vterm.MouseEncoding) bool {
	if a.Action != b.Action || a.Button != b.Button ||
		a.Shift != b.Shift || a.Alt != b.Alt || a.Ctrl != b.Ctrl {
		return false
	}
	if encoding == vterm.MouseEncodingSGRPixels {
		return a.PixelX == b.PixelX && a.PixelY == b.PixelY
	}
	return a.Col == b.Col && a.Row == b.Row
}

// reportTerminalMouseWheel 上报一次滚轮；返回 true 表示"这一下归程序"。
//
// 官方的滚轮上报只在协议收滚轮时挂（`16&events`，byte 26995 的
// `16&e?s.wheel`），所以 X10 那档**不上报**、照旧滚本地滚回。
//
// 键号永远是 4（滚轮），方向在位里（`s=4`、`r=i<0?0:1`，byte 25864 →
// `i|=64;i|=action`）：DOM 的 deltaY<0 是往上滚 → Cb 64，>0 → Cb 65。
//
// 返回 true 但没编出序列（比如坐标越界）时也**不滚本地**：官方那条
// `wheel:e=>(i(e),this.cancel(e,!0))`（byte 26995 之后那支）是无条件吃掉滚轮的。
func (p *Plugin) reportTerminalMouseWheel(ev gfx.ScrollEvent) bool {
	state := p.wheelMouse
	if !state.Reporting() || !state.Protocol.ReportsWheel() {
		return false
	}
	button := vterm.MouseWheelDown
	if ev.DeltaY < 0 {
		button = vterm.MouseWheelUp
	}
	col, row := terminalMouseCell(p.wheelRect, p.wheelCellW, p.wheelCellH, p.wheelCols, p.wheelRows, ev.X, ev.Y)
	px, py := terminalMousePixel(p.wheelRect, p.wheelCellW, p.wheelCellH, p.wheelCols, p.wheelRows, ev.X, ev.Y)
	in := vterm.MouseInput{
		Action: vterm.MousePress, Button: button,
		Col: col, Row: row, PixelX: px, PixelY: py,
		Shift: ev.Modifiers&gfx.ModifierShift != 0,
		Alt:   ev.Modifiers&gfx.ModifierAlt != 0,
		Ctrl:  ev.Modifiers&gfx.ModifierControl != 0,
	}
	if seq, ok := vterm.MouseSequence(state, in, p.wheelCols, p.wheelRows); ok &&
		p.act.TerminalWriteOfficial != nil && p.wheelTab != "" {
		p.act.TerminalWriteOfficial(p.wheelTab, seq, false)
	}
	return true
}
