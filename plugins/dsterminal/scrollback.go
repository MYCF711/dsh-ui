package dsterminal

import (
	"math"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个文件是终端的**滚回（scrollback）**：滚到哪、滚轮走几行、14px 槽怎么和
// fit 一起算、滚动条怎么画。屏幕本身怎么逐格画在 grid.go。
//
// 全部按官方 xterm 的口径（证据都是官方包里内联的 `client.terminal.js`，
// `@deepseek-ai/dsh-client-ui-sidebar-terminal/lib/client.terminal.js`，509421 字节）：
//
//   - 滚回交给 xterm：`scrollback: environment?.scrollback ?? 0`（terminal.tsx:104）。
//     本仓的滚回在壳里（internal/vterm），容量由 `SetScrollbackSize` 对齐同一个数。
//   - 滚动条槽：`proposeDimensions` 在 `scrollback !== 0` 时从可用宽度里扣
//     `overviewRuler?.width || 14`，也就是
//     `const t=0===this._terminal.options.scrollback?0:this._terminal.options.overviewRuler?.width||14`
//     （从 `app.asar` 重抽的 bundle 里：`proposeDimensions(){` 在 byte 489625、
//     这个表达式在 489853；#174 记的 489675 指的是同一段），
//     `_getChangeOptions` 的 `verticalScrollbarSize` 是同一个 14（byte 52197）。
//     **扣掉的这 14px 就是滚动条自己占的宽度**，所以「fit 里扣」和「画滚动条」
//     必须是同一个数：只扣不画会留一条空槽，只画不扣会盖住最后一列。
//   - 滚轮：DEFAULT_OPTIONS 里 `scrollSensitivity:1`、`fastScrollModifier:'alt'`、
//     `fastScrollSensitivity:5`（byte 278089），换算见 terminalWheelLines。
//   - 滑块颜色：`scrollbarSliderBackground=opacity(foreground,.2)`、
//     hover `.4`、active `.5`（byte 132863 起的 `_updateTheme`）。
//   - 滑块几何：`ScrollbarState._computeValues`（byte 363xxx），
//     `Math.round(Math.max(20, Math.floor(visible*o/scrollSize)))` —— 最小 20px。

const (
	// terminalWheelLabel 是滚轮命中区的无障碍名（读屏软件读的是这个，
	// 不是角色名）。
	terminalWheelLabel = "终端内容"

	// terminalScrollbarWidth 是官方滚动条槽宽：`overviewRuler?.width || 14`。
	// 官方没有设 overviewRuler.width（DEFAULT_OPTIONS 是 `overviewRuler:{}`），
	// 所以真值就是 14。
	terminalScrollbarWidth float32 = 14

	// terminalMinSliderSize 是官方滑块的最小长度（`Math.max(20, …)`）。
	terminalMinSliderSize float32 = 20

	// terminalScrollSensitivity / terminalFastScrollSensitivity 是 xterm 的
	// 两个滚轮系数（官方没有覆盖 DEFAULT_OPTIONS，所以是 1 和 5）。
	terminalScrollSensitivity     float32 = 1
	terminalFastScrollSensitivity float32 = 5

	// terminalSliderAlpha 是官方滑块底色：前景色 alpha 0.2（非 hover 态）。
	terminalSliderAlpha float32 = 0.2

	// terminalSmallWheelPixels / terminalSmallWheelFactor 是 xterm 对「像素级
	// 小增量」的阻尼（`Math.abs(deltaY) < 50 → *= .3`，byte 268232）：触控板
	// 一次滑动可能只有十几像素，不阻尼就会窜得比经典滚轮还快。
	terminalSmallWheelPixels float32 = 50
	terminalSmallWheelFactor float32 = 0.3
)

// terminalScrollbackCapacity 是这条终端的滚回容量，取自宿主 environment。
//
// 官方的判据是 `options.scrollback === 0`（不是「现在攒了几行」）：容量为 0 时
// 连 14px 槽都不留。environment 还没到时按 0 处理——那时也没有屏幕可画。
func terminalScrollbackCapacity(state pluginapi.TerminalViewState) int {
	if state.Environment == nil {
		return 0
	}
	return state.Environment.Scrollback
}

// terminalScrollbarGutter 是这一帧要留给滚动条的宽度（官方 0 或 14）。
func terminalScrollbarGutter(scrollback int) float32 {
	if scrollback <= 0 {
		return 0
	}
	return terminalScrollbarWidth
}

// terminalWheelLines 把一次滚轮事件换成「走几行」，逐条照抄 xterm 的
// `_getLinesFromWheelEvent` + `_applyScrollModifier`（byte 268232）：
//
//	r = deltaY * scrollSensitivity                       // 1
//	if alt|ctrl|shift: r *= fastScrollSensitivity * scrollSensitivity   // 5
//	// DOM_DELTA_PIXEL（本仓把精确增量归到这一档）：
//	r /= cellHeight
//	if |deltaY| < 50: r *= .3
//	partial += r
//	r = floor(|partial|) * sign(partial);  partial %= 1
//
// 两个偏离说明（都是为了把**本仓的坐标**对到官方**同一套语义**上）：
//
//  1. 平台给的是逻辑像素（Viva `platform.WheelPixels`：一档 = 系统设置的
//     N 行 × 16px，默认 3 行 = 48px），官方 xterm 在 Chromium 里收到的也是一个
//     档 = 100px 级的像素增量——两边都是像素，所以直接沿用 `r /= cellHeight`。
//  2. 50px 阻尼只在**精确**增量（`Precise`：触控板、自由滚轮）上开。经典滚轮在
//     本仓被量化成整档（48px）落进了 50px 阈值，不区分的话每档会被无故砍到三成。
//
// partial 是调用方按 tab 存的小数余量（xterm 的 `_wheelPartialScroll`），
// 触控板滑动时它让不足一行的位移攒着，而不是每帧丢掉。
func terminalWheelLines(ev gfx.ScrollEvent, cellH float32, partial float64) (int, float64) {
	if cellH <= 0 {
		return 0, partial
	}
	amount := ev.DeltaY * terminalScrollSensitivity
	if ev.Modifiers&(gfx.ModifierAlt|gfx.ModifierControl|gfx.ModifierShift) != 0 {
		amount *= terminalFastScrollSensitivity * terminalScrollSensitivity
	}
	amount /= cellH
	if ev.Precise && absFloat(ev.DeltaY) < terminalSmallWheelPixels {
		amount *= terminalSmallWheelFactor
	}
	partial += float64(amount)
	lines := int(math.Floor(math.Abs(partial)))
	if partial < 0 {
		lines = -lines
	}
	partial = math.Mod(partial, 1)
	return lines, partial
}

// terminalScrollbarThumb 是官方 `ScrollbarState._computeValues`（byte 363xxx）的
// 纵向那一路：
//
//	available = visible - oppositeScrollbarSize        // 本仓没有横向滚动条，= visible
//	o         = available - 2*arrowSize                // xterm v5 没有箭头，= available
//	slider    = round(max(20, floor(visible * o / scrollSize)))
//	ratio     = (o - slider) / (scrollSize - visible)
//	position  = round(scrollPosition * ratio)
//
// 三个入参都是像素：visiblePx = 可见区高度，totalPx = 内容总高度，
// scrollPx = 已滚过的高度。不可滚时返回 (0,0)（官方 `isNeeded()` 为假，
// 滑块不出现、槽仍然占位——那正是 FitAddon 一直扣 14px 的原因）。
func terminalScrollbarThumb(visiblePx, totalPx, scrollPx float32) (offset, size float32) {
	if visiblePx <= 0 || totalPx <= visiblePx {
		return 0, 0
	}
	size = float32(math.Round(math.Max(
		float64(terminalMinSliderSize),
		math.Floor(float64(visiblePx*visiblePx/totalPx)),
	)))
	ratio := (visiblePx - size) / (totalPx - visiblePx)
	offset = float32(math.Round(float64(scrollPx * ratio)))
	return offset, size
}

// terminalScrollView 是一条 occurrence 的滚动位置。
//
// 用的是 xterm 的 `ydisp` 语义：**虚拟缓冲第一行的绝对行号**（0 = 滚回里最老的
// 一行，== ScrollbackLen 表示贴底）。绝对行号而不是「离底几行」：用户停在中途时
// 新输出只会长在下面，绝对行号不动，画面就不会跟着抖（xterm 同）。
type terminalScrollView struct {
	// ydisp 是第一行虚拟行号；follow 表示「贴底跟随」（新输出一来就跟到底）。
	ydisp  int
	follow bool
	// partial 是 xterm 的滚轮小数余量（`_wheelPartialScroll`）。
	partial float64
}

// resolve 给出这一帧该用的 ydisp，并把超出容量的位置夹回来。
//
// follow 的传播是「贴底」的唯一来源：一旦用户滚上去就停下，一旦滚回底部
// （ydisp == ScrollbackLen）就重新跟随。容量变小（resize 后滚回被截断）时
// 也在这里夹回，否则会请求一个不存在的窗口。
func (v *terminalScrollView) resolve(scrollbackLen int) int {
	if v == nil {
		return max(scrollbackLen, 0)
	}
	if scrollbackLen <= 0 {
		v.ydisp, v.follow = 0, true
		return 0
	}
	if v.follow {
		v.ydisp = scrollbackLen
	}
	v.ydisp = min(max(v.ydisp, 0), scrollbackLen)
	v.follow = v.ydisp >= scrollbackLen
	return v.ydisp
}

// scrollViewFor 取（必要时建）某个 occurrence 的滚动位置。
func (p *Plugin) scrollViewFor(tabID string) *terminalScrollView {
	if p.scroll == nil {
		p.scroll = map[string]*terminalScrollView{}
	}
	view := p.scroll[tabID]
	if view == nil {
		view = &terminalScrollView{follow: true}
		p.scroll[tabID] = view
	}
	return view
}

// forgetScroll 丢掉某个 occurrence 的滚动位置（终端重建/关闭时）。
func (p *Plugin) forgetScroll(tabID string) {
	delete(p.scroll, tabID)
}

// scrollTerminal 处理一次落在终端上的滚轮。
//
// 返回 moved 表示画面要重画。**无论有没有动都算 Handled**：官方那边滚轮落在
// xterm 里就被它吃掉（`ev.preventDefault()`），外壳与页面不该跟着滚——滚到头
// 也一样，那是浏览器的行为。
func (p *Plugin) scrollTerminal(ev gfx.ScrollEvent) bool {
	if p.wheelTab == "" || p.wheelCellH <= 0 {
		return false
	}
	if ev.DeltaY == 0 {
		return false
	}
	view := p.scrollViewFor(p.wheelTab)
	lines, partial := terminalWheelLines(ev, p.wheelCellH, view.partial)
	view.partial = partial
	if lines == 0 {
		return false
	}
	before := p.wheelYdisp
	ydisp := min(max(before+lines, 0), p.wheelScrollback)
	p.wheelYdisp = ydisp
	view.ydisp = ydisp
	view.follow = ydisp >= p.wheelScrollback
	return ydisp != before
}

// paintScrollbar 画官方那条纵向滚动条：14px 槽里的滑块，颜色是前景 20%。
//
// 槽宽、滑块几何、颜色都来自本文件顶部那几条官方证据；**槽是预留的**
// （fit 已经扣掉了这 14px），所以这里只画滑块，不画轨道底色——官方也没有
// 轨道底色（`.xterm-scrollable-element > .scrollbar > .slider` 只设 background）。
func (p *Plugin) paintScrollbar(canvas *gfx.Canvas, rect ui.Rect, gutter float32, visibleLines, totalLines, ydisp int, cellH float32) {
	if canvas == nil || gutter <= 0 || cellH <= 0 {
		return
	}
	visiblePx := float32(visibleLines) * cellH
	totalPx := float32(totalLines) * cellH
	offset, size := terminalScrollbarThumb(visiblePx, totalPx, float32(ydisp)*cellH)
	if size <= 0 {
		return
	}
	canvas.Fill(gfx.Rect{
		X:      rect.X + rect.Width - gutter,
		Y:      rect.Y + offset,
		Width:  gutter,
		Height: size,
	}, p.theme.LabelPrimary.WithAlpha(terminalSliderAlpha))
}

func absFloat(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
