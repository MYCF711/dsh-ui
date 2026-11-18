package dsterminal

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个文件钉住「面板尺寸 → PTY 列行数」这条换算的三个面：
//
//  1. 纯计算：官方 FitAddon 的取整与 environment 夹取（terminalFitSize）；
//  2. 落地：环境上限真的会传到绘制面，不会画出宿主不接受的列数；
//  3. 上行：算出来的尺寸真的发给宿主，且**同尺寸不发**（官方 model.js:212-223）。

// TestTerminalFitSizeFollowsPanel 钉住「面板变宽/变窄，列数跟着变」。
//
// 数字来源是官方 FitAddon 的算法（可用区域 ÷ 实测格宽，向下取整），
// 不是我们自己拍的常量：格宽格高由调用方给，这里只是除法与取整。
func TestTerminalFitSizeFollowsPanel(t *testing.T) {
	for _, tc := range []struct {
		name             string
		rectW, rectH     float32
		cellW, cellH     float32
		maxCols, maxRows int
		wantCols         int
		wantRows         int
	}{
		{name: "344px 面板 × 10px 格 = 34 列", rectW: 344, rectH: 400, cellW: 10, cellH: 16, wantCols: 34, wantRows: 25},
		{name: "180px 面板 × 10px 格 = 18 列", rectW: 180, rectH: 400, cellW: 10, cellH: 16, wantCols: 18, wantRows: 25},
		{name: "半格向下取整，不多算一列", rectW: 349, rectH: 400, cellW: 10, cellH: 16, wantCols: 34, wantRows: 25},
		{name: "environment.maxCols 夹住上限", rectW: 344, rectH: 400, cellW: 10, cellH: 16, maxCols: 20, wantCols: 20, wantRows: 25},
		{name: "environment.maxRows 夹住上限", rectW: 344, rectH: 400, cellW: 10, cellH: 16, maxRows: 10, wantCols: 34, wantRows: 10},
		{name: "面板放不下一行 → 不发", rectW: 344, rectH: 10, cellW: 10, cellH: 16, wantCols: 0, wantRows: 0},
		{name: "只剩 1 列 → 官方同样不发", rectW: 15, rectH: 400, cellW: 10, cellH: 16, wantCols: 0, wantRows: 0},
		{name: "格子还没量到 → 不发", rectW: 344, rectH: 400, cellW: 0, cellH: 0, wantCols: 0, wantRows: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols, rows := terminalFitSize(tc.rectW, tc.rectH, tc.cellW, tc.cellH, tc.maxCols, tc.maxRows)
			if cols != tc.wantCols || rows != tc.wantRows {
				t.Fatalf("版式 %v：得到 %d×%d，期望 %d×%d", tc.name, cols, rows, tc.wantCols, tc.wantRows)
			}
		})
	}
}

// TestGridHonorsEnvironmentMaxCols 钉住上限**真的到了绘制面**。
//
// 宿主 `terminal.environment.maxCols` 是官方 fitScreen 的第二道夹取
// （terminal.tsx:195-196）。它必须一路传到 paintGrid：否则界面会按面板宽度
// 画出一片宿主根本不接受的列（真发出去就是 bad-param）。
func TestGridHonorsEnvironmentMaxCols(t *testing.T) {
	grid := emulatorGrid(t, 80, 3, strings.Repeat("x", 80))
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	state := pluginapi.TerminalViewState{
		Phase: "connected", Writable: true,
		Grid: grid,
		Info: &apiclient.TerminalInfo{Title: "终端 1", State: "running", Cols: grid.Width, Rows: grid.Height},
		// 面板本身放得下 34 列（360-2×8 内距 ÷ 10），但宿主只接受 20 列。
		Environment: &apiclient.TerminalEnvironment{MaxCols: 20, MaxRows: 24, Cwd: "/w"},
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-maxcols"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: true},
		Act: pluginapi.Actions{
			TerminalEnsure:        func(string) {},
			TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return state },
			TerminalRetry:         func(string) {},
			OpenRightTab:          func(string) {},
			CloseRightTab:         func(string) {},
			TerminalWriteOfficial: func(string, string, bool) {},
		},
	}
	p := &Plugin{theme: theme}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	page.Draw(canvas, gfx.Rect{Width: gridTestPageW, Height: gridTestPageH}, func(f ui.C) {
		p.drawPanelBody(f, env)
	})

	label, ok := gridRowLabel(canvas, 0)
	if !ok {
		t.Fatal("找不到整行 label")
	}
	if visible := int(label.Bounds.Width / gridTestCellW); visible != 20 {
		t.Fatalf("environment.maxCols=20 时只该画 20 列，实际 %d 列", visible)
	}
	for _, run := range gridRuns(canvas, 0) {
		if end := run.column + run.cells; end > 20 {
			t.Fatalf("run %+v 越过 environment.maxCols=20", run)
		}
	}
}

// resizeHarness 是一块能重复绘制的终端面板：draw(w) 用给定页宽画一帧，
// 并把插件真的发出去的 resize 收进 calls。
//
// 用真绘制路径（drawPanelBody → paintGrid → fit 回调）而不是直接调
// requestTerminalResize：这条测试的价值就在于「量出来的矩形 → 发出去的尺寸」
// 这一整段是通的，绕过绘制层就只剩算术。
type resizeHarness struct {
	plugin *Plugin
	canvas *gfx.Canvas
	page   *ui.Page
	env    *pluginapi.Env
	state  pluginapi.TerminalViewState
	calls  []terminalSize
}

func newResizeHarness(t *testing.T, state pluginapi.TerminalViewState) *resizeHarness {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) {
		return gridTestCellW, gridTestCellH
	})
	h := &resizeHarness{canvas: canvas, state: state}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("terminal-resize"), Canvas: canvas,
		Data: pluginapi.Snapshot{ShowRightbar: true},
	}
	env.Act = pluginapi.Actions{
		TerminalEnsure:        func(string) {},
		TerminalSnapshot:      func(string) pluginapi.TerminalViewState { return h.state },
		TerminalRetry:         func(string) {},
		OpenRightTab:          func(string) {},
		CloseRightTab:         func(string) {},
		TerminalWriteOfficial: func(string, string, bool) {},
		TerminalResizeOfficial: func(tabID string, cols, rows int) {
			h.calls = append(h.calls, terminalSize{cols: cols, rows: rows})
		},
	}
	h.env = env
	h.plugin = &Plugin{theme: theme}
	pageTheme := dscore.VivaTheme(theme)
	h.page = ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	return h
}

func (h *resizeHarness) draw(width float32) {
	h.page.Draw(h.canvas, gfx.Rect{Width: width, Height: gridTestPageH}, func(f ui.C) {
		h.plugin.drawPanelBody(f, h.env)
	})
}

// TestTerminalResizeWantedIsOfficialDedupe 钉住官方 model.resize 的两条判据
// （OFF:dsh-api-terminal-controller/lib/types/client/model.js:212-223）：
//
//	if (state.info.cols === cols && state.info.rows === rows) return
//	if (cols < 2 || rows < 1) return
//
// 这两条是**唯一的**节奏来源：官方全文没有 setTimeout/debounce，多一条判据
// （哪怕只是"隔 N 毫秒再发"）都是自造行为。
func TestTerminalResizeWantedIsOfficialDedupe(t *testing.T) {
	for _, tc := range []struct {
		name             string
		curCols, curRows int
		cols, rows       int
		want             bool
	}{
		{name: "同尺寸 → 不发", curCols: 80, curRows: 24, cols: 80, rows: 24, want: false},
		{name: "列变宽 → 发", curCols: 80, curRows: 24, cols: 120, rows: 24, want: true},
		{name: "行变高 → 发", curCols: 80, curRows: 24, cols: 80, rows: 40, want: true},
		{name: "1 列 → 官方 cols<2 直接 return", curCols: 80, curRows: 24, cols: 1, rows: 24, want: false},
		{name: "0 行 → 官方 rows<1 直接 return", curCols: 80, curRows: 24, cols: 80, rows: 0, want: false},
		{name: "连当前尺寸都不知道 → 不发", curCols: 0, curRows: 0, cols: 80, rows: 24, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalResizeWanted(tc.curCols, tc.curRows, tc.cols, tc.rows); got != tc.want {
				t.Fatalf("terminalResizeWanted(%d,%d,%d,%d)=%v，期望 %v",
					tc.curCols, tc.curRows, tc.cols, tc.rows, got, tc.want)
			}
		})
	}
}

// TestTerminalResizeSendsOncePerSizeChange 是本单的主单测：**面板量出来的尺寸
// 真的发出去，且同尺寸不发**（官方 model.js:212-223 的 return）。
//
// 尺寸由真实绘制路径算出：页宽 360 → screen 内宽 344（减两侧 8px 内距）→
// 344 / 10px 格宽 = 34 列；页宽 460 → 444 / 10 = 44 列。
// 断言的是「发出去的 cols/rows」，不是内部字段，所以绘制层与上行层任何一侧
// 接错都会红。
func TestTerminalResizeSendsOncePerSizeChange(t *testing.T) {
	grid := emulatorGrid(t, 80, 24, "ready")
	h := newResizeHarness(t, pluginapi.TerminalViewState{
		Phase: "connected", Writable: true, Grid: grid,
		Info:        &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "running", Cols: 80, Rows: 24},
		Environment: &apiclient.TerminalEnvironment{MaxCols: 500, MaxRows: 200},
	})
	h.draw(360)
	if len(h.calls) != 1 {
		t.Fatalf("尺寸与 PTY 不同时必须发一次，实际 %d 次：%v", len(h.calls), h.calls)
	}
	first := h.calls[0]
	if first.cols != 34 {
		t.Fatalf("360px 面板（344px 内宽 / 10px 格）该发 34 列，实际 %d 列", first.cols)
	}
	if first.rows < 1 {
		t.Fatalf("行数必须是正数，实际 %d", first.rows)
	}
	// 同一尺寸多画几帧：官方那里是 `state.info.cols === cols && ...` 直接 return。
	h.draw(360)
	h.draw(360)
	if len(h.calls) != 1 {
		t.Fatalf("同尺寸必须一帧都不再发，实际累计 %d 次：%v", len(h.calls), h.calls)
	}
	// 面板拉宽：444 / 10 = 44 列，行数不变（高度没动）。
	h.draw(460)
	if len(h.calls) != 2 || h.calls[1].cols != 44 || h.calls[1].rows != first.rows {
		t.Fatalf("拉宽后要发 44×%d，实际 %v", first.rows, h.calls)
	}
	// 宿主 state 回填成新尺寸之后再画：仍然不该发（同尺寸 return）。
	h.state.Info = &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "running", Cols: 44, Rows: first.rows}
	h.draw(460)
	if len(h.calls) != 2 {
		t.Fatalf("宿主回填后再画不该重发，实际累计 %d 次：%v", len(h.calls), h.calls)
	}
	// 再变窄：回到 34 列，这一下要发。
	h.draw(360)
	if len(h.calls) != 3 || h.calls[2].cols != 34 {
		t.Fatalf("变窄后要再发 34 列，实际 %v", h.calls)
	}
}

// TestTerminalResizeSendsClampedSize 钉住「夹取后与实际发送一致」：
// 面板量出 34 列，但宿主 environment.maxCols=20 / maxRows=10 —— 发出去的必须
// 是夹取后的 20×10，不是量出来的 34×36。发原始值会被宿主按官方档位拒绝
// （越界报错不截断），界面就永远对不上。
func TestTerminalResizeSendsClampedSize(t *testing.T) {
	grid := emulatorGrid(t, 80, 24, "ready")
	h := newResizeHarness(t, pluginapi.TerminalViewState{
		Phase: "connected", Writable: true, Grid: grid,
		Info:        &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "running", Cols: 80, Rows: 24},
		Environment: &apiclient.TerminalEnvironment{MaxCols: 20, MaxRows: 10},
	})
	h.draw(360)
	if len(h.calls) != 1 {
		t.Fatalf("夹取后仍与 PTY 不同，必须发一次，实际 %d 次：%v", len(h.calls), h.calls)
	}
	if got := h.calls[0]; got.cols != 20 || got.rows != 10 {
		t.Fatalf("发出去的必须是夹取后的 20×10，实际 %d×%d", got.cols, got.rows)
	}
}

// TestTerminalResizeSilentWithoutAction 钉住「壳没接就什么都不发」：
// nil = 老壳，界面继续按整格裁掉右侧，不得崩、不得伪造成功。
func TestTerminalResizeSilentWithoutAction(t *testing.T) {
	grid := emulatorGrid(t, 80, 24, "ready")
	h := newResizeHarness(t, pluginapi.TerminalViewState{
		Phase: "connected", Writable: true, Grid: grid,
		Info:        &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "running", Cols: 80, Rows: 24},
		Environment: &apiclient.TerminalEnvironment{MaxCols: 500, MaxRows: 200},
	})
	h.env.Act.TerminalResizeOfficial = nil
	h.draw(360) // 不 panic、不发送即可
	if len(h.calls) != 0 {
		t.Fatalf("壳没接线时不该有发送：%v", h.calls)
	}
}

// TestTerminalResizeWaitsForConnected 钉住「没接上就不发」：connecting/failed
// 阶段没有可比的 PTY 尺寸，发出去只会打到一个未知的终端上。
// 接上之后（Info 回来）才按真实尺寸发一次。
func TestTerminalResizeWaitsForConnected(t *testing.T) {
	grid := emulatorGrid(t, 80, 24, "ready")
	h := newResizeHarness(t, pluginapi.TerminalViewState{
		Phase: "connecting", Writable: false, Grid: grid,
		Info:        &apiclient.TerminalInfo{ID: "tab1", Title: "终端 1", State: "starting", Cols: 80, Rows: 24},
		Environment: &apiclient.TerminalEnvironment{MaxCols: 500, MaxRows: 200},
	})
	h.draw(360)
	if len(h.calls) != 0 {
		t.Fatalf("还没 connected 就不该发：%v", h.calls)
	}
	h.state.Phase = "connected"
	h.state.Writable = true
	h.draw(360)
	if len(h.calls) != 1 {
		t.Fatalf("接上后要按真实尺寸发一次，实际 %d 次：%v", len(h.calls), h.calls)
	}
}
