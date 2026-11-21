//go:build darwin || windows || linux

// dx_product_harness_test.go —— 把 hitsProduct 接成**产品那条输入链**的无头窗口。
//
// `hits_matrix_test.go` 只画一帧就丢，拿不到"按键/指针 -> 重画 -> 再看"这条链；
// 而真实产品的次序是 `cmd/app/main.go` 里的 cfg.OnKey：**整窗页面先接**
// （`view.Key` → 快捷键/焦点控件），页面不接才轮到 `kernel.HandleKey`（插件）。
// 少了这一步，Tab 这种由渲染器 Router 处理的键在测试里根本不存在。
package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// dxFrame 是一台"产品形态"的无头窗口：真骨架 + 真插件 + 产品输入次序。
type dxFrame struct {
	t       *testing.T
	p       *hitsProduct
	session *headless.Session
}

func (f *dxFrame) snapshot() app.Snapshot { return f.session.Snapshot() }

func (f *dxFrame) pump() {
	f.t.Helper()
	if err := f.session.Frame(); err != nil {
		f.t.Fatalf("重画：%v", err)
	}
}

// openDXFrame 建一台无头窗口；width/height 是窗口尺寸。
func openDXFrame(t *testing.T, frame hitsFrame, width, height float32) *dxFrame {
	t.Helper()
	p := newHitsProduct(t)
	if frame.open != nil {
		p.paintOnce(t, frame)
		frame.open(t, p)
	}
	opts, env := p.frameInputs(frame, width, height)
	theme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &theme})
	cfg := app.Config{
		Title: "dx-product", Width: int(width), Height: int(height), Tree: p.tree,
		WindowMode:  app.WindowFrameless,
		Invalidator: &app.Invalidator{},
	}
	cfg.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		env.Canvas = canvas
		view.Draw(canvas, env.Bounds, func(c ui.C) {
			uikernel.FirstScreen(c, env, opts, func(slot pluginapi.Slot) {
				p.kernel.DrawSlot(c, slot, env)
			})
		})
		for _, painter := range p.kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	cfg.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if result := view.Key(ev); result.Handled || !result.Activate.IsZero() {
			cfg.Invalidator.Invalidate()
			return result
		}
		if p.kernel.HandleKey(ev) {
			cfg.Invalidator.Invalidate()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}
	session, err := headless.Open(cfg, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("无头窗口打不开：%v", err)
	}
	t.Cleanup(session.Close)
	return &dxFrame{t: t, p: p, session: session}
}

// hitByID 找本帧的命中区。
func (f *dxFrame) hitByID(id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range f.snapshot().Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}
