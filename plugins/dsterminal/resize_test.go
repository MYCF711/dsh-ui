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

// 这个文件钉住「面板尺寸 → PTY 列行数」这条换算的两个面：
//
//  1. 纯计算：官方 FitAddon 的取整与 environment 夹取（terminalFitSize）；
//  2. 落地：环境上限真的会传到绘制面，不会画出宿主不接受的列数。

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
