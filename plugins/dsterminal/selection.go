package dsterminal

import (
	"math"
	"time"

	"github.com/MYCF711/dsh-ui/internal/vterm"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个文件是终端的**选区与复制**：指针怎么变成选区、选中态怎么画、复制怎么落到
// 系统剪贴板。几何与文本在 `internal/vterm/selection.go`（照 xterm 的
// `SelectionService` / `SelectionModel`），这里只做 xterm 在**指针那一层**做的事。
//
// 官方证据（全是官方包里内联的 xterm，`dsh-client-ui-sidebar-terminal/lib/client.terminal.js`，
// 509421 字节，`grep -b` 可复现）：
//
//   - 单选/双击/三击分派：`handleMouseDown`（byte 121098）
//     `1===e.detail → _handleSingleClick`、`2===e.detail → _handleDoubleClick`、
//     `3===e.detail → _handleTripleClick`；按住 Shift 是 `_handleIncrementalClick`
//     （只挪终点、留着起点，byte 122249）。**`e.detail` 是浏览器给的连击次数**，
//     Viva 里对应的是平台的 `gfx.PointerEvent.ClickCount`（Windows 由系统双击
//     时间/距离算出：`viva@v0.6.4/internal/platform/windows/window_windows.go:1137`
//     的 `configureClicks`），所以这里不自己数秒表。
//   - 矩形选区：`shouldColumnSelect` = Alt（byte 122685）。
//   - 拖动时贴边滚动：`_getMouseEventScrollAmount`（byte 120966）
//     `y 超出画布 → 夹到 ±50px，然后 sign(y) + round(14*y/50)` 行/次，
//     定时器 `setInterval(_dragScroll, 50)`（byte 121584）。
//   - 拖动时终点躲开宽字尾巴：byte 123245。
//   - 用户一输入就清空选区：`_coreService.onUserInput(() => hasSelection && clearSelection())`
//     （byte 116835）。
//   - 选中态颜色：官方 `terminal-theme.ts:61` 把
//     `selectionBackground` 设成 foreground、`selectionForeground` 设成 background、
//     `selectionInactiveBackground` 也设成 foreground。
//   - 复制：xterm 只注册 `copy` 事件（`_initGlobal`，byte 16481 →
//     `copyHandler`，byte 10034：`clipboardData.setData('text/plain', selectionText)`），
//     **自己不绑定任何快捷键**。官方桌面端真正的触发是右键弹出的原生编辑菜单
//     （`app.asar` byte 207130469：`editItem(copy,"C",["control"],"Ctrl+C")` →
//     `shortcuts.sendEditingKey` → `webContents.sendInputEvent`，让 Chromium 对
//     焦点元素执行 Copy）；xterm 的右键处理（`rightClickHandler`，byte 10372）
//     就是在为那一次 Copy 准备内容：`textarea.value = selectionText; textarea.select()`。
//     本仓没有原生菜单（Viva 不向 `ui.C` 导出 Popup，见 `$DSH_HOME/outbox/` 的
//     issue 草稿），所以这里把「右键 → 编辑菜单 → 复制」这一条折叠成
//     「右键有选区就复制」，写剪贴板走 Viva 自己的
//     `app.SetClipboardText`（`viva@v0.6.4/app/system.go:139`）。

// terminalSelectionAlpha 是 xterm 对不透明 selectionBackground 的强制透明度。
//
// 官方把 `selectionBackground` 设成前景色（**不透明**），xterm 的 `_setTheme`
// 会把它降到 α 0.3：`if (isOpaque(selectionBackgroundTransparent))
// selectionBackgroundTransparent = opacity(…, .3)`（byte 132577），再用
// `selectionBackgroundOpaque = blend(background, selectionBackgroundTransparent)`
// （byte 132041）合成出**不透明**的格子底色。所以选中态是
// 「bg-base 上盖一层 30% 的 label-primary」，不是 50% 也不是原色。
const terminalSelectionAlpha float32 = 0.3

// terminalDragScrollInterval 是官方贴边滚动的节拍（`setInterval(…, 50)`，
// byte 121584 的 50 就是毫秒）。
const terminalDragScrollInterval = 50 * time.Millisecond

// terminalSelection 是一条 occurrence 的选区状态。
type terminalSelection struct {
	// sel 是 xterm 的 SelectionModel：坐标是**虚拟缓冲的绝对行**，
	// 所以滚回里的行和屏幕行是同一套坐标，选到屏幕外不需要换算。
	sel vterm.Selection
	// pressing 表示指针还按在终端上（拖动中）。
	pressing bool
	// dragScroll 是 xterm 的 `_dragScrollAmount`：指针拖出画布上下边时每次
	// 滚几行，0 = 不滚。
	dragScroll int
	// moved 表示这次按下之后指针动过（决定是「点击」还是「拖动」）。
	moved bool
}

// terminalGridGeometry 是最近一帧的网格几何：指针事件只带窗口坐标，
// 要换成「第几列、第几行」得由绘制时记下来（与滚轮那条路同一个理由，
// 见 grid.go 的 rememberWheelTarget）。
type terminalGridGeometry struct {
	hit               gfx.ID
	tabID             string
	rect              ui.Rect
	cellW, cellH      float32
	cols, rows, first int
	scrollback, ydisp int
	// mouse 是这一帧终端的鼠标上报状态（协议 + 编码）。指针手势归谁由它决定：
	// 上报开着且没按 Shift 时按下归 PTY，否则归本地选区（见 mouse.go 的文件头）。
	mouse vterm.MouseState
}

// posAt 把窗口坐标换成虚拟缓冲坐标；x 夹进 [0,cols) 并躲开宽字尾巴，
// y 夹进这一帧看得见的行。
func (g terminalGridGeometry) posAt(x, y float32) vterm.SelPos {
	col := 0
	if g.cellW > 0 {
		col = int(math.Floor(float64(x-g.rect.X) / float64(g.cellW)))
	}
	row := 0
	if g.cellH > 0 {
		row = int(math.Floor(float64(y-g.rect.Y) / float64(g.cellH)))
	}
	if row < 0 {
		row = 0
	}
	if g.rows > 0 && row > g.rows-1 {
		row = g.rows - 1
	}
	return vterm.SelPos{X: col, Line: g.first + row}
}

// selectionFor 取（必要时建）某个 occurrence 的选区状态。
func (p *Plugin) selectionFor(tabID string) *terminalSelection {
	if p.selection == nil {
		p.selection = map[string]*terminalSelection{}
	}
	state := p.selection[tabID]
	if state == nil {
		state = &terminalSelection{}
		p.selection[tabID] = state
	}
	return state
}

// forgetSelection 丢掉某个 occurrence 的选区（终端重建/关闭时）。
func (p *Plugin) forgetSelection(tabID string) {
	delete(p.selection, tabID)
}

// selectionView 返回这一帧要画的选区（空选区返回零值，`Empty()` 为真）。
func (p *Plugin) selectionView(tabID string) vterm.Selection {
	if state := p.selection[tabID]; state != nil {
		return state.sel
	}
	return vterm.Selection{}
}

// clearTerminalSelection 清掉选区。返回是否真的清掉了东西（要不要重绘）。
func (p *Plugin) clearTerminalSelection(tabID string) bool {
	state := p.selection[tabID]
	if state == nil || state.sel.Empty() {
		return false
	}
	state.sel = vterm.Selection{}
	state.dragScroll = 0
	return true
}

// terminalSelectionColors 是选中态的两支颜色。
//
// 官方 `terminal-theme.ts:61`：
//
//	selectionBackground: foreground, selectionForeground: background,
//	selectionInactiveBackground: foreground
//
// 这里的 `foreground` / `background` **不是主题常量**，而是 xterm 那两个
// 会变的默认色：官方先 `update(background, foreground)`（terminal-theme.ts:57-64，
// 调用点 terminal.tsx:124-127 用 `getComputedStyle` 取 `.screen` 的那两个值），
// 之后程序可以用 OSC 10 / 11 把它们换掉（同一文件 39-53 行那两段观察者）。
// 所以入参必须是**已经按 OSC 覆盖算过的**那份（`p.defaultColors(def)`），不能直接
// 传 `theme.BgBase` / `theme.LabelPrimary`——否则程序换过底色之后，选区矩形与
// 选中文字色会和周围格子对不上。
//
//   - 底色 = blend(background, foreground @ 0.3)（见 terminalSelectionAlpha）；
//   - 字色 = background；
//   - **失焦不换色**：官方把 selectionInactiveBackground 也设成 foreground，
//     而 selectionForeground 在一帧里对焦点/失焦都生效（xterm byte 88032）。
func terminalSelectionColors(foreground, background ui.Color) (fill, text ui.Color) {
	return terminalBlend(background, foreground, terminalSelectionAlpha), background
}

// terminalBlend 是 xterm 的 `color.blend(background, foreground)`（byte 138965）：
// `out = background + round((foreground-background)*alpha)`，逐通道。
//
// xterm 的通道是 8 位整数，所以那句 `Math.round` 是把结果落回整数；
// Viva 的 `gfx.Color` 是 0..1 的浮点（`viva@v0.6.4/gfx/canvas.go:10-19`），
// 取整没有意义，这里直接用同一条线性插值（与官方差值 ≤ 1/255）。
func terminalBlend(background, foreground ui.Color, alpha float32) ui.Color {
	mix := func(b, f float32) float32 { return b + (f-b)*alpha }
	return ui.Color{
		Red:   mix(background.Red, foreground.Red),
		Green: mix(background.Green, foreground.Green),
		Blue:  mix(background.Blue, foreground.Blue),
		Alpha: 1,
	}
}

// handleTerminalSelectionPointer 处理落在终端上的指针事件。
//
// 形状照 `HandleScroll` / `plugins/dssidebar/drag.go` 那条路：绘制时用
// `canvas.PointerOn(hit)` 问「这一帧有没有给我的指针事件」，按下之后运行时
// 会把 Move/Up 一直送到按下的那个 id（viva 的指针捕获）。
func (p *Plugin) handleTerminalSelectionPointer(env *pluginapi.Env, canvas *gfx.Canvas, geom terminalGridGeometry) {
	if canvas == nil || geom.hit.IsZero() || geom.cols <= 0 || geom.rows <= 0 {
		return
	}
	event, ok := canvas.PointerOn(geom.hit)
	if !ok {
		return
	}
	state := p.selectionFor(geom.tabID)
	mouse := p.mouseFor(geom.tabID)
	// 官方 `shouldForceSelection`（byte 120764）：非 mac 上就是 Shift。
	// 本产品只做 Windows，所以取这一支（见 mouse.go 的文件头第 3 条）。
	forced := event.Modifiers&gfx.ModifierShift != 0
	// 官方 bindMouse 的 mousedown（byte 27559）：鼠标上报开着、且这一下没被
	// Shift 强制扣下时，按下归 PTY，本地选区不参与。
	sending := geom.mouse.Reporting() && !forced
	switch event.Kind {
	case gfx.PointerDown:
		// 按住的键先记上：官方的 `event.buttons` 与"有没有上报"无关
		// （见 mouse.go 的 holdTerminalMouse）。
		holdTerminalMouse(mouse, event.Button, true)
		if sending {
			// 官方那一支是 `i(e); …; this.cancel(e)`——**不看上报的返回值**：
			// 只要鼠标模式开着且没被 Shift 强制，这一下就归 PTY（哪怕某个序列
			// 因为装不下而被编码器丢掉），本地选区不参与。
			p.reportTerminalMousePress(geom, mouse, event)
			canvas.Consume()
		}
		switch event.Button {
		case gfx.PointerButtonPrimary, gfx.PointerButtonNone:
			if sending {
				return
			}
			p.beginTerminalSelection(env, canvas, geom, state, event)
		case gfx.PointerButtonSecondary:
			// 官方 Windows 的右键 = 弹出原生编辑菜单（app.asar byte 207130469），
			// 那个菜单里的「复制」才是剪贴板的入口。本仓没有原生菜单，所以
			// 右键有选区就直接复制（见文件头）。
			//
			// 鼠标上报开着时**两件事都做**：官方 `_initGlobal` 挂的 contextmenu
			// 处理器不看鼠标模式（byte 17027），而 bindMouse 那边右键照样上报。
			if p.copyTerminalSelection(env, geom, state) {
				canvas.Consume()
			}
		}
	case gfx.PointerMove:
		// 官方的移动上报**不看 Shift**：DRAG/ANY 的 mousemove 监听器只看
		// buttons（byte 27240 / 26842），所以 Shift 强制选区拖起来时 motion
		// 照发。这里照抄，不是自造。
		if geom.mouse.Reporting() {
			p.reportTerminalMouseMotion(geom, mouse, event)
		}
		if !state.pressing {
			return
		}
		p.dragTerminalSelection(env, canvas, geom, state, event)
	case gfx.PointerUp:
		holdTerminalMouse(mouse, event.Button, false)
		if geom.mouse.Reporting() {
			p.reportTerminalMouseRelease(geom, mouse, event)
		}
		if !state.pressing {
			return
		}
		state.pressing, state.dragScroll = false, 0
		if state.moved {
			canvas.Consume() // 拖动结束：这一下不是点击
		}
		state.moved = false
	case gfx.PointerExit:
		// 指针离开窗口：xterm 靠 ownerDocument 上的 mousemove/mouseup 继续跟
		//（`_addMouseDownListeners`，byte 121584），Viva 的指针捕获同样会把按住
		// 之后的事件一直送到按下的那个 id（`gfx.PointerEvent.Captured`），
		// 所以离窗这一帧不做处理。
	}
}

// beginTerminalSelection 是 `handleMouseDown`（byte 121098）的分派。
func (p *Plugin) beginTerminalSelection(env *pluginapi.Env, canvas *gfx.Canvas, geom terminalGridGeometry, state *terminalSelection, event gfx.PointerEvent) {
	pos := geom.posAt(event.X, event.Y)
	pos.X = p.snapSelectionColumn(env, geom, pos)
	state.pressing, state.moved, state.dragScroll = true, false, 0
	// 官方 `e.preventDefault()`：按下这一下属于终端，不是页面滚动/点击。
	canvas.Consume()
	switch {
	case event.ClickCount >= 3:
		// 三击 = 整行（`_handleTripleClick` → `_selectLineAt`）。
		state.sel = vterm.LineSelection(pos.Line, geom.cols)
	case event.ClickCount == 2:
		// 双击 = 词（`_handleDoubleClick` → `_selectWordAtCursor`）。
		state.sel = p.wordSelectionAt(env, geom, pos)
	case event.Modifiers&gfx.ModifierShift != 0 && !state.sel.Empty():
		// Shift+点击 = 只挪终点，起点留着（`_handleIncrementalClick`，byte 122249）。
		state.sel.End, state.sel.HasEnd = pos, true
	default:
		// 普通按下：起点与终点同一格（`hasSelection` 为假 = 清空），
		// Alt 按住则是矩形选区（`shouldColumnSelect`，Windows 上就是 Alt）。
		mode := vterm.SelLinear
		if event.Modifiers&gfx.ModifierAlt != 0 {
			mode = vterm.SelColumn
		}
		state.sel = vterm.Selection{Start: pos, End: pos, HasEnd: true, Mode: mode}
	}
}

// dragTerminalSelection 是 `_handleMouseMove`（byte 123245）。
func (p *Plugin) dragTerminalSelection(env *pluginapi.Env, canvas *gfx.Canvas, geom terminalGridGeometry, state *terminalSelection, event gfx.PointerEvent) {
	pos := geom.posAt(event.X, event.Y)
	pos.X = p.snapSelectionColumn(env, geom, pos)
	state.moved = true
	end := pos
	switch state.sel.Mode {
	case vterm.SelLine:
		// 行选拖动：终点列由方向决定（官方 `2===mode` 那一条：往回拖是 0，往前是 cols）。
		if pos.Line < state.sel.Start.Line {
			end.X = 0
		} else {
			end.X = geom.cols
		}
	case vterm.SelWord:
		// 词选拖动：终点吸到词边界（`_selectToWordAt`）。
		reversed := pos.Line < state.sel.Start.Line ||
			(pos.Line == state.sel.Start.Line && pos.X < state.sel.Start.X)
		end = p.wordEndAt(env, geom, pos, reversed)
	}
	state.sel.End, state.sel.HasEnd = end, true

	state.dragScroll = terminalDragScrollAmount(event.Y, geom.rect.Y, geom.rect.Height)
	if state.dragScroll != 0 && p.scrollSelectionView(geom.tabID, state, geom) {
		// 官方每 50ms 滚一次（byte 121584），这里按同一节拍预约下一帧：
		// 指针不动的贴边也要继续滚，靠 RequestFrameIn 续上。滚不动了（已经到
		// 缓冲头/尾）就不再排帧，否则会变成一个空转的重绘循环。
		canvas.RequestFrameIn(terminalDragScrollInterval)
	}
	canvas.Consume() // 选区跟着指针走：运行时立刻重绘
}

// advanceTerminalDragScroll 在每个重绘帧里推一次贴边滚动（相当于 xterm 的
// `_dragScroll` 定时器），没有在拖就什么都不做。
//
// 返回 true 表示滚动位置变了、需要重画。
func (p *Plugin) advanceTerminalDragScroll(tabID string, geom terminalGridGeometry, canvas *gfx.Canvas) bool {
	state := p.selection[tabID]
	if state == nil || !state.pressing || state.dragScroll == 0 || canvas == nil {
		return false
	}
	moved := p.scrollSelectionView(tabID, state, geom)
	if moved {
		canvas.RequestFrameIn(terminalDragScrollInterval)
	}
	return moved
}

// scrollSelectionView 按 `_dragScroll` 的规则滚一屏：滚完之后终点贴住新的
// 视口边界（官方 `_dragScroll`：往下滚贴到 ydisp+rows-1，往上滚贴到 ydisp，
// 列在非矩形模式下分别是 cols / 0）。
func (p *Plugin) scrollSelectionView(tabID string, state *terminalSelection, geom terminalGridGeometry) bool {
	view := p.scrollViewFor(tabID)
	if view == nil || geom.scrollback <= 0 {
		return false
	}
	before := view.ydisp
	ydisp := min(max(before+state.dragScroll, 0), geom.scrollback)
	if ydisp == before {
		return false
	}
	view.ydisp = ydisp
	view.follow = ydisp >= geom.scrollback
	end := vterm.SelPos{Line: ydisp}
	if state.dragScroll > 0 {
		// 往下滚：终点贴到新的视口最后一行（官方 `ydisp + rows` 再夹到缓冲末尾，
		// 这里的缓冲就是滚回 + 屏幕，夹住由 sel 的 Normalized 与取窗口那一层做）。
		end.Line = ydisp + max(geom.rows-1, 0)
	}
	if state.sel.Mode != vterm.SelColumn {
		if state.dragScroll > 0 {
			end.X = geom.cols
		} else {
			end.X = 0
		}
	} else {
		end.X = state.sel.End.X
	}
	state.sel.End, state.sel.HasEnd = end, true
	return true
}

// snapSelectionColumn 把列夹进 [0,cols) 并躲开宽字尾巴。
//
// 官方在鼠标那一层就躲（`_handleSingleClick` / `_handleMouseMove`：
// `0===line.hasWidth(col) && col<cols → col++`），这里用同一份判据：
// 尾巴格 = `Cell.Width == 0`（见 internal/vterm 的 Cell）。
func (p *Plugin) snapSelectionColumn(env *pluginapi.Env, geom terminalGridGeometry, pos vterm.SelPos) int {
	pos.X = min(max(pos.X, 0), max(geom.cols-1, 0))
	grid, first := p.selectionGrid(env, geom.tabID, pos.Line, 1)
	if grid == nil || first != pos.Line || grid.Height < 1 {
		return pos.X
	}
	local := 0
	return grid.SnapColumn(local, pos.X, geom.cols)
}

// wordSelectionAt / wordEndAt 是词选的两种落点，取的是同一份屏幕。
func (p *Plugin) wordSelectionAt(env *pluginapi.Env, geom terminalGridGeometry, pos vterm.SelPos) vterm.Selection {
	grid, first := p.selectionGrid(env, geom.tabID, pos.Line, 1)
	if grid == nil || first != pos.Line {
		return vterm.Selection{}
	}
	if sel, ok := grid.WordSelection(0, pos.Line, pos.X, geom.cols); ok {
		return sel
	}
	return vterm.Selection{Start: pos, End: pos, HasEnd: true}
}

func (p *Plugin) wordEndAt(env *pluginapi.Env, geom terminalGridGeometry, pos vterm.SelPos, reversed bool) vterm.SelPos {
	grid, first := p.selectionGrid(env, geom.tabID, pos.Line, 1)
	if grid == nil || first != pos.Line {
		return pos
	}
	return grid.WordEndAt(0, pos.Line, pos.X, geom.cols, reversed)
}

// selectionGrid 取 [line, line+lines) 这一段缓冲的单元格快照，返回窗口与它的首行号。
//
// 一律走壳的 `TerminalScrollbackWindow`：它按坐标切「滚回 + 当前屏」这条虚拟缓冲，
// 与绘制滚回那一帧用的是同一个动作（见 pluginapi.Actions 的说明）。
// 老壳没有这个动作时，只有落在当前屏里的行还能拿到——用 `state.Grid` 顶。
func (p *Plugin) selectionGrid(env *pluginapi.Env, tabID string, line, lines int) (*vterm.Grid, int) {
	if lines <= 0 {
		return nil, 0
	}
	if env != nil && env.Act.TerminalScrollbackWindow != nil {
		if window := env.Act.TerminalScrollbackWindow(tabID, line, lines); window != nil && window.Height > 0 {
			return window, line
		}
		return nil, 0
	}
	if p.act.TerminalSnapshot == nil {
		return nil, 0
	}
	state := p.act.TerminalSnapshot(tabID)
	if state.Grid == nil {
		return nil, 0
	}
	scrollback := max(state.ScrollbackLen, 0)
	screenLine := line - scrollback
	if screenLine < 0 || screenLine >= state.Grid.Height {
		return nil, 0
	}
	return state.Grid, scrollback
}

// copyTerminalSelection 把选区文本写进系统剪贴板。
//
// 返回是否真的复制了东西（决定要不要 Consume 这次右键）。
func (p *Plugin) copyTerminalSelection(env *pluginapi.Env, geom terminalGridGeometry, state *terminalSelection) bool {
	if state == nil || state.sel.Empty() || p.clipboard == nil {
		return false
	}
	text := p.terminalSelectionText(env, geom.tabID, geom.cols, state.sel)
	if text == "" {
		return false
	}
	if err := p.clipboard(text); err != nil {
		if env != nil && env.Act.Notify != nil {
			env.Act.Notify("复制失败: " + err.Error())
		}
		return false
	}
	return true
}

// terminalSelectionText 取出选区文本：一次取整段缓冲，再交给 internal/vterm 拼串。
func (p *Plugin) terminalSelectionText(env *pluginapi.Env, tabID string, cols int, sel vterm.Selection) string {
	if cols <= 0 || sel.Empty() {
		return ""
	}
	first, last := sel.Lines()
	grid, start := p.selectionGrid(env, tabID, first, last-first+1)
	if grid == nil {
		return ""
	}
	return vterm.SelectionText(grid, start, cols, sel)
}

// paintTerminalSelection 把选中态画在**格子底色之上、字形之下**：官方 xterm 的
// 选区是一层独立的绝对定位 div（`handleSelectionChanged`，byte 80805 的
// `_selectionContainer` + byte 78634 的 CSS：`position:absolute;
// background-color: selectionBackgroundOpaque`），它盖住格子自己的底色，
// 字形再画在上面（字色换成 selectionForeground，byte 88032）。
//
// 两支颜色都跟着 `def` 走（程序 OSC 10/11 换过的默认色），见
// terminalSelectionColors。
func (p *Plugin) paintTerminalSelection(canvas *gfx.Canvas, grid *vterm.Grid, def vterm.Defaults, line int, cols int, x0, top, cellW, cellH float32, sel vterm.Selection) {
	if canvas == nil || sel.Empty() {
		return
	}
	fill, _ := terminalSelectionColors(p.defaultColors(def))
	start := -1
	for x := 0; x < cols; x++ {
		if !sel.Contains(cols, x, line) {
			if start >= 0 {
				p.fillSelectionSpan(canvas, start, x, x0, top, cellW, cellH, fill)
				start = -1
			}
			continue
		}
		if start < 0 {
			start = x
		}
	}
	if start >= 0 {
		p.fillSelectionSpan(canvas, start, cols, x0, top, cellW, cellH, fill)
	}
}

func (p *Plugin) fillSelectionSpan(canvas *gfx.Canvas, start, end int, x0, top, cellW, cellH float32, fill ui.Color) {
	if end <= start {
		return
	}
	canvas.Fill(gfx.Rect{
		X:      x0 + float32(start)*cellW,
		Y:      top,
		Width:  float32(end-start) * cellW,
		Height: cellH,
	}, fill)
}

// terminalDragScrollAmount 是官方 `_getMouseEventScrollAmount`（byte 120966）：
//
//	y 在画布内 → 0；出去了先把 y 夹到 ±50px，然后
//	amount = sign(y) + round(14 * (y / 50))
//
// 所以贴边 1 格（≈16px）→ sign 1 + round(4.48) = 1+4 = 5 行/次；
// 贴到底（≥50px 外）→ 1+14 = 15 行/次。14 和 50 都是官方的数。
func terminalDragScrollAmount(y, top, height float32) int {
	offset := y - top
	if offset >= 0 && offset <= height {
		return 0
	}
	if offset > height {
		offset -= height
	}
	offset = min(max(offset, -terminalDragScrollClampPixels), terminalDragScrollClampPixels)
	scaled := offset / terminalDragScrollClampPixels
	sign := float32(0)
	if scaled > 0 {
		sign = 1
	} else if scaled < 0 {
		sign = -1
	}
	return int(sign + float32(math.Round(float64(terminalDragScrollStepLines*scaled))))
}

const (
	// terminalDragScrollClampPixels 是官方夹住贴边距离的 50px（byte 120966）。
	terminalDragScrollClampPixels float32 = 50
	// terminalDragScrollStepLines 是官方那一档的系数 14（同一句）。
	terminalDragScrollStepLines float32 = 14
)
