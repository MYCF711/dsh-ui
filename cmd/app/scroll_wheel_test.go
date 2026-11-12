package main

// scroll_wheel_test.go —— V-06-4 判据③：**滚轮接管与 OnScroll 同步**。
//
// # 这条判据在防什么
//
// v0.6.0 把滚动状态交给**声明方**（`Box.ScrollState()`），并且新增了一条
// 显式规则：`ScrollAt` 会把 `scrollManual` 置真，而 `view.Wheel` 遇到 manual
// 区**直接返回未处理**（`ui/scroll.go` 的 `view.Wheel`：
// `if f.exts[n.ext].scrollManual { return gfx.EventResult{} }`）——
// 也就是说"应用自己接管滚轮"和"应用自己恢复偏移"是**互斥**的两条路。
//
// 本仓走的是 `ScrollTo`（不是 `ScrollAt`），所以：
//
//	① 滚轮必须真的滚动消息区（`view.Wheel` Handled）；
//	② 滚轮之后 `recordScroll` 必须把新偏移抄进 `m.scrollPos`（切会话恢复的就是它）；
//	③ 恢复路径（`settleScroll` → `ScrollTo`）**不能**把滚轮弄坏——
//	   恢复之后滚轮仍要 Handled。若哪天有人把 `ScrollTo` 换成 `ScrollAt`，
//	   这条会立刻红（那是必须同时接管 `Config.OnScroll` 的信号）。
//
// 没这条断言的话，`ScrollAt` 换上去编译过、单测过、门禁全绿，只有真机上
// 鼠标滚轮滚不动——V-06-3 记的 ③ 就是这个坑。

import (
	"fmt"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// wheelFixture 装配一个"20 条消息、视口 100 高"的滚动区。
func wheelFixture(t *testing.T, follow bool) (*model, ui.Box) {
	t.Helper()
	const id = "wheel"
	pageTheme := ui.LightTheme()
	m := &model{
		cur:       id,
		view:      ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
		scrollPos: map[string]float32{},
	}
	m.msgs = make([]apiclient.Message, 20)
	for i := range m.msgs {
		m.msgs[i] = apiclient.Message{Seq: i + 1, Role: "assistant", Text: fmt.Sprintf("message-%02d", i+1)}
	}
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2
	})
	var area ui.Box
	m.view.Draw(canvas, gfx.Rect{Width: 100, Height: 100}, func(f ui.C) {
		ui.Column(f).Fill().Children(func() {
			area = ui.Scroll(f).Grow(1).Follow(follow)
			area.Children(func() {
				for i := range m.msgs {
					ui.Text(f, m.msgs[i].Text).Height(30)
				}
			})
		})
	})
	return m, area
}

// TestWheelScrollsMessageArea 是判据①：滚轮真的滚动消息区。
func TestWheelScrollsMessageArea(t *testing.T) {
	m, area := wheelFixture(t, false)
	st := area.ScrollState()
	if st.Max() <= 0 {
		t.Fatalf("夹具没有可滚动空间：Content=%v Viewport=%v", st.Content, st.Viewport)
	}
	before := st.Offset

	result := m.view.Wheel(gfx.ScrollEvent{X: 50, Y: 50, DeltaY: 100})
	if !result.Handled {
		t.Fatalf("滚轮没被接管（Handled=false）——真机上鼠标滚轮滚不动消息列表")
	}
	if st.Offset <= before {
		t.Fatalf("滚轮之后偏移没变：before=%v after=%v", before, st.Offset)
	}
	if !result.Repaint {
		t.Fatal("滚动之后必须请求重绘，否则画面停在旧位置")
	}
}

// TestWheelOffsetReachesScrollPos 是判据②：滚轮之后 recordScroll 抄的是**新**偏移。
func TestWheelOffsetReachesScrollPos(t *testing.T) {
	m, area := wheelFixture(t, false)
	st := area.ScrollState()

	if res := m.view.Wheel(gfx.ScrollEvent{X: 50, Y: 50, DeltaY: 200}); !res.Handled {
		t.Fatal("滚轮没被接管")
	}
	wheeled := st.Offset

	// 产品路径：Draw 返回后 settleScroll（先恢复、再记偏移）。这里没有待恢复值，
	// 所以只有 recordScroll 那半步生效。
	m.settleScroll(area, true)

	if got := m.scrollPos["wheel"]; got != wheeled {
		t.Fatalf("recordScroll 抄下的偏移 = %v，want %v（滚轮之后的位置）——切会话会恢复错位置",
			got, wheeled)
	}
}

// TestScrollToRestoreKeepsWheelWorking 是判据③：恢复位置不能把滚轮弄坏。
//
// 这是"应用接管滚轮"与"精确恢复"必须同步的那条：`ScrollAt` 会置 scrollManual，
// 之后 `view.Wheel` 对该区返回未处理。本仓用 `ScrollTo`，所以恢复后滚轮仍可用。
func TestScrollToRestoreKeepsWheelWorking(t *testing.T) {
	const id = "restore-wheel"
	m, area := wheelFixture(t, false)
	m.cur = id
	m.scrollPos[id] = 120
	m.pendingScroll = id

	// 恢复：产品路径就是 settleScroll（先恢复、再记）。
	if !m.settleScroll(area, true) {
		t.Fatal("有待恢复的偏移时必须请求补帧")
	}
	if got := area.ScrollState().Offset; got != 120 {
		t.Fatalf("恢复后的偏移 = %v，want 120", got)
	}

	// 恢复之后**滚轮仍然可用**——ScrollTo 不该把 scrollManual 打开。
	before := area.ScrollState().Offset
	res := m.view.Wheel(gfx.ScrollEvent{X: 50, Y: 50, DeltaY: 100})
	if !res.Handled {
		t.Fatal("恢复位置之后滚轮失效：ScrollTo 把 scrollManual 打开了？" +
			"（若改成 ScrollAt，必须同时接管 Config.OnScroll，见本文件头）")
	}
	if area.ScrollState().Offset <= before {
		t.Fatalf("恢复之后滚轮没滚动：before=%v after=%v", before, area.ScrollState().Offset)
	}
	// 并且新位置也要被抄回去（两件事同帧完成）。
	m.settleScroll(area, true)
	if got := m.scrollPos[id]; got != area.ScrollState().Offset {
		t.Fatalf("滚轮之后 m.scrollPos = %v，want %v", got, area.ScrollState().Offset)
	}
}
