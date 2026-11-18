package dsterminal

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
// # 这条路上还缺一段（如实登记，不是实现）
//
// 官方那一下 `model.resize` 走的是 attach 流的 **uplink**（帧 `{type:'resize',
// sequence, cols, rows}`，服务端 `serve_terminal_api.go:650` 的 `case "resize"`），
// 而这条 uplink 在本宿主只存在于 `/api/remote.mux`（officialwire）上，
// HTTP 面没有 `terminal.resize` 一元方法——详见
// `docs/gui-terminal-resize-gap.md`。所以本文件只提供**测得准**的那一半：
// 面板 → cols/rows 的换算规则与上限，等上行通道接上后直接复用。

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

// # 「该不该发」这一半等上行通道接上时再落地
//
// 官方的去重判据（OFF:dsh-api-terminal-controller/lib/types/client/model.js:212-223）
// 只有两条，实现时必须原样照抄、不许自造：
//
//	if (state.info.cols === cols && state.info.rows === rows) return   // 尺寸没变 → 不发
//	if (cols < 2 || rows < 1) return                                   // 算不出来 → 不发
//
// 本仓的「当前尺寸」取壳缓存里的 `TerminalInfo.cols/rows`（与官方 `state.info`
// 同一份），目标尺寸取 terminalFitSize 的返回值。
