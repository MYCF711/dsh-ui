package dsterminal

import "github.com/MYCF711/dsh-ui/pluginapi"

// 这个文件回答一个问题：**面板有多大，PTY 就该有多少列/行**。
//
// 官方口径（OFF:dsh-client-ui-sidebar-terminal/src/client/terminal.tsx:191-200
// 的 `fitScreen`）：
//
//	const dimensions = fit.proposeDimensions()   // xterm FitAddon 按实测格宽格高反算
//	const cols = Math.min(dimensions.cols, environment.maxCols)
//	const rows = Math.min(dimensions.rows, environment.maxRows)
//	if (cols < 2 || rows < 1) return              // 太小就**不发**
//	xterm.resize(cols, rows)
//	model.resize(cols, rows)                       // 真正的上行在哪条通道见下
//
// 触发时机是 `new ResizeObserver(measure)` 观察终端节点（:126-127），外加三处重排：
// 可见性/可写性变化（:169-175）、字号与字体族变化（:145-155）、挂载时
// （`useLayoutEffect` 里的初测，:100-138）。
//
// **官方没有定时节流**：terminal.tsx 全文没有 setTimeout / setInterval /
// requestAnimationFrame / debounce / throttle（已 grep 该文件，零命中）。去重靠
// 两件事：① 浏览器 ResizeObserver 按帧合并通知；② 官方 `model.resize`
// （OFF:dsh-api-terminal-controller/lib/types/client/model.js:212-223）在
// `state.info.cols === cols && state.info.rows === rows` 时直接 return。
// 因此本仓**不自造节流参数**：同尺寸不重复发，尺寸真变了就发。
//
// # 上行通道：宿主 `terminal.resize`（HTTP 回落投影）
//
// 官方那一下 `model.resize` 走的是 attach 流的 **uplink**（帧 `{type:'resize',
// sequence, cols, rows}`）。那条 uplink 只存在于 `/api/remote.mux`
// （officialwire）上，本仓 GUI 没有 mux 客户端，所以宿主补了一条与
// `terminal.input` 同型的加法方法 `POST /api/terminal.resize`
// （`DSH-GO/internal/transport/handlers_terminal.go` 的 handleTerminalResize）。
//
// 本文件因此是**完整的一段**：换算（terminalFitSize）→ 去重（terminalResizeWanted）
// → 发出（requestTerminalResize 调 `Actions.TerminalResizeOfficial`）。
// 上限之外的官方第二道校验在宿主（越界报错不截断），壳只负责把错误讲清楚。
// 差异登记见 `docs/gui-terminal-resize-gap.md`。

// terminalFitSize 是官方 `fitScreen` 的纯计算部分：从绘制面反算 cols/rows。
//
//	rectW/rectH —— 终端面板可画区域（列 × 格宽、行 × 格高都是从这里除出来的）
//	cellW/cellH —— 画布对等宽字体的**实测**格子尺寸（`Canvas.MonoCellSize`），
//	               与官方 xterm 量的是同一件事；<=0 表示还没量到
//	maxCols/maxRows —— 官方 `environment.maxCols/maxRows`；<=0 表示宿主没给上限
//
// 返回 (0,0) 表示**官方口径下不该发 resize**：格子还没量到、面板放不下一行，
// 或者算出来 `cols < 2 || rows < 1`（官方同样直接 return，不把 1 列的终端发给宿主）。
//
// 取整用截断而不是四舍五入：半格放不下一个字，官方 FitAddon 也是向下取整
// （`Math.floor`）。多留半格会画出一条永远被裁的残列。
func terminalFitSize(rectW, rectH, cellW, cellH float32, maxCols, maxRows int) (cols, rows int) {
	if cellW <= 0 || cellH <= 0 || rectW <= 0 || rectH <= 0 {
		return 0, 0
	}
	cols = int(rectW / cellW)
	rows = int(rectH / cellH)
	if maxCols > 0 && cols > maxCols {
		cols = maxCols
	}
	if maxRows > 0 && rows > maxRows {
		rows = maxRows
	}
	if cols < 2 || rows < 1 {
		return 0, 0
	}
	return cols, rows
}

// terminalSize 是一次 resize 的目标尺寸。
type terminalSize struct{ cols, rows int }

// terminalResizeWanted 是官方 `model.resize` 的去重判据，逐条照抄
// （OFF:dsh-api-terminal-controller/lib/types/client/model.js:212-223）：
//
//	if (state.info.cols === cols && state.info.rows === rows) return   // 尺寸没变 → 不发
//	if (cols < 2 || rows < 1) return                                   // 算不出来 → 不发
//
// curCols/curRows 是「PTY 现在的尺寸」，取壳缓存里的 `TerminalInfo.cols/rows`
// （与官方 `state.info` 同一份；没有时退到上一次已请求的尺寸，见
// requestTerminalResize）；cols/rows 是 terminalFitSize 的返回值。
//
// **不要在这里加时间窗**：官方 terminal.tsx 全文没有 setTimeout / setInterval /
// requestAnimationFrame / debounce / throttle（grep 零命中），节奏由两条判据
// 加宿主 state 回填决定。加一个毫秒值就是自造官方没有的行为。
func terminalResizeWanted(curCols, curRows, cols, rows int) bool {
	if cols < 2 || rows < 1 {
		return false
	}
	if curCols <= 0 || curRows <= 0 {
		// 连"现在是多大"都不知道：官方这时也没有可比较的 state.info，
		// 凭空发一个尺寸只会让 PTY 与屏幕错位。
		return false
	}
	return curCols != cols || curRows != rows
}

// requestTerminalResize 决定这一帧要不要把面板量出的尺寸发给宿主。
//
// 为什么还要记一份 `resizeRequested`：本仓的触发是**绘制帧**（Viva 没有
// ResizeObserver 那一层"按帧合并的通知"），而宿主的 `state` 广播要等 resize
// 请求返回才回填 `Info`。这中间隔着的几帧里 `Info` 还是旧尺寸，只读它就会
// 对同一个尺寸连发几次——官方没有这个问题，因为它的触发是事件而不是帧。
//
// 所以这份记录是 `state.info` 的**乐观副本**：发出即为它的新值，宿主 state 帧
// 回来后会与它一致。判据一个字没改（还是 terminalResizeWanted），
// **没有任何时间节流**。
//
// 宿主拒绝时（越界/终端没了）`Info` 不会变，这里也不会重试——与官方一样：
// 没有新的触发就没有新的请求，错误由壳在状态栏讲清楚。
func (p *Plugin) requestTerminalResize(env *pluginapi.Env, tabID string, state pluginapi.TerminalViewState, cols, rows int) {
	act := env.Act.TerminalResizeOfficial
	if act == nil {
		return // 老壳：不发，界面继续按整格裁掉右侧。
	}
	if state.Info == nil || state.Phase != "connected" {
		// 还没接上（connecting）或这一轮已经结束：上一轮记的尺寸对新 PTY 不成立。
		// 删掉而不是留着，否则重连后"同尺寸不发"会把新 PTY 的默认 80×24
		// 当成已经对齐过。
		delete(p.resizeRequested, tabID)
		return
	}
	curCols, curRows := state.Info.Cols, state.Info.Rows
	if sent, ok := p.resizeRequested[tabID]; ok {
		curCols, curRows = sent.cols, sent.rows
	}
	if !terminalResizeWanted(curCols, curRows, cols, rows) {
		return
	}
	if p.resizeRequested == nil {
		p.resizeRequested = map[string]terminalSize{}
	}
	p.resizeRequested[tabID] = terminalSize{cols: cols, rows: rows}
	act(tabID, cols, rows)
}
