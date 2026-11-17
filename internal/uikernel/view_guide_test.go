//go:build darwin || windows || linux

package uikernel

// view_guide_test.go —— 右栏向导页（guide）入口卡的机器断言。
//
// 断言分四类：
//
//	A. 名录投影：只有声明了 `Painter.Guide` 的类型上向导页；按官方向导卡 order 排；
//	   provider 身份 = 类型的 kind（keyed 座位的 key）
//	B. keyed 派发：每张卡只派发**它自己那个 provider** 的注册，且带官方
//	   `SidebarRightGuideEntryOwnerProps`（`Env.GuideEntry`），派发完恢复原值
//	C. fallback：没有占用方时画官方胶囊（`EntryBox`），点它打开那张卡声明的 kind
//	D. 真画一遍：卡片文案真的上屏

import (
	"fmt"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// openGuideFrame 开一帧 headless 画面，把 draw 交给骨架绘制。
//
// draw 在每次 paint 时拿一个**新的** env（壳的真实做法：每帧一个 Env）；
// 测试里改的 HasKey / DrawKeyed / Act 都在这个 env 上，随帧生效。
func openGuideFrame(t *testing.T, width, height int, draw func(env *pluginapi.Env) func(ui.C)) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("guide-frame")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "guide-frame", Width: width, Height: height, Tree: tree, WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Has: func(pluginapi.Slot) bool { return false },
		}
		canvas.Fill(bounds, theme.BgBase)
		body := draw(env)
		if body == nil {
			return
		}
		view.Draw(canvas, bounds, body)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	session.Resize(width, height)
	for i := 0; i < 8; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	return session
}

// ── A. 名录投影 ─────────────────────────────────────────────────────

func TestGuideEntriesProjectFromTypeRegistration(t *testing.T) {
	k := New("guide-entries")
	panels := []pluginapi.Painter{
		// 终端：官方 index.ts:72-75 的 guide 声明（id 'new'、order 20）。
		{ID: "terminal-tab", Slot: pluginapi.SlotRightPaneTab, Key: "terminal", Order: 20,
			Guide: []pluginapi.TabGuide{{ID: "new", Order: 20, Title: "新建终端", Description: "在会话工作区运行命令",
				IconImage: terminalIconForTest(), IconSize: 26}},
			Draw: noopDraw},
		// 浏览器：官方 definition.tsx:22-25（id 'new'、commandId 'browser.new'、order 30）。
		{ID: "browser-tab", Slot: pluginapi.SlotRightPaneTab, Key: "browser", Order: 0,
			Guide: []pluginapi.TabGuide{{ID: "new", Order: 30, Title: "浏览器", Description: "浏览网页",
				CommandID: "browser.new"}},
			Draw: noopDraw},
		// 没有 guide 声明的类型不上向导页（官方向导页列的是声明过的入口）。
		{ID: "trajectory", Slot: pluginapi.SlotRightPaneTab, Key: "trajectory", Order: 10, Draw: noopDraw},
	}
	for i, panel := range panels {
		one := &tabPlugin{id: fmt.Sprintf("guide-%d", i), panels: []pluginapi.Painter{panel}}
		if err := k.Load(one); err != nil {
			t.Fatalf("装第 %d 块画失败：%v", i, err)
		}
	}
	entries := GuideEntries(k)
	if len(entries) != 2 {
		t.Fatalf("向导卡条目 = %d，want 2（只有声明 guide 的类型）：%+v", len(entries), entries)
	}
	// order 20 在 30 之前（官方 guide[].order 升序）。
	if entries[0].Kind != "terminal" || entries[1].Kind != "browser" {
		t.Fatalf("向导卡顺序 = %q,%q，want terminal,browser", entries[0].Kind, entries[1].Kind)
	}
	if entries[0].ProviderID != "terminal" || entries[1].ProviderID != "browser" {
		t.Fatalf("provider 身份 = %q,%q，want kind", entries[0].ProviderID, entries[1].ProviderID)
	}
	if entries[0].Title != "新建终端" || entries[0].Description != "在会话工作区运行命令" {
		t.Fatalf("终端卡片文案 = %q / %q", entries[0].Title, entries[0].Description)
	}
	if entries[1].CommandID != "browser.new" {
		t.Fatalf("浏览器卡片 commandId = %q，want browser.new", entries[1].CommandID)
	}
}

// ── B. keyed 派发 + owner props ─────────────────────────────────────

func TestDrawRightGuideDispatchesKeyedCardWithOwnerProps(t *testing.T) {
	entries := []GuideEntry{
		{TabGuide: pluginapi.TabGuide{ID: "new", Title: "新建终端", Description: "在会话工作区运行命令"},
			ProviderID: "terminal", Kind: "terminal"},
		{TabGuide: pluginapi.TabGuide{ID: "new", Title: "浏览器"}, ProviderID: "browser", Kind: "browser"},
	}
	var dispatched []struct {
		key           string
		props         pluginapi.GuideEntryProps
		nilAtDispatch bool
	}
	sentinel := &pluginapi.GuideEntryProps{EntryID: "sentinel"}
	session := openGuideFrame(t, 420, 320, func(env *pluginapi.Env) func(ui.C) {
		env.HasKey = func(slot pluginapi.Slot, key string) bool {
			return slot == pluginapi.SlotRightGuideEntry && key == "terminal"
		}
		env.DrawKeyed = func(target ui.C, slot pluginapi.Slot, key string) {
			if slot != pluginapi.SlotRightGuideEntry {
				return
			}
			record := struct {
				key           string
				props         pluginapi.GuideEntryProps
				nilAtDispatch bool
			}{key: key}
			if env.GuideEntry == nil {
				record.nilAtDispatch = true
			} else {
				record.props = *env.GuideEntry
			}
			dispatched = append(dispatched, record)
			ui.Text(target, "card:"+key)
		}
		env.GuideEntry = sentinel
		return func(f ui.C) {
			dispatched = dispatched[:0] // 每帧重记：断言的是最后那一帧
			DrawRightGuide(f, env, entries)
			// 派发结束必须恢复写者自己的值（与 EntryMenu / ViewportOffset 同一约定）。
			if env.GuideEntry != sentinel {
				t.Errorf("派发结束后 Env.GuideEntry 没被恢复：%+v", env.GuideEntry)
			}
		}
	})
	texts := map[string]bool{}
	for _, one := range session.Snapshot().Texts {
		texts[one.Value] = true
	}
	// 终端那张卡有占用方 → 只派发它，且带上官方 owner props 四个字段。
	if len(dispatched) != 1 || dispatched[0].key != "terminal" {
		t.Fatalf("keyed 派发 = %+v，want 只有 terminal", dispatched)
	}
	got := dispatched[0].props
	if dispatched[0].nilAtDispatch || got.EntryID != "new" || got.Kind != "terminal" ||
		got.Title != "新建终端" || got.Description != "在会话工作区运行命令" {
		t.Fatalf("owner props = %+v（nilAtDispatch=%v），want 官方 SidebarRightGuideEntryOwnerProps",
			got, dispatched[0].nilAtDispatch)
	}
	if !texts["card:terminal"] {
		t.Fatalf("占用方画的东西没上屏：%v", session.Texts())
	}
	// 浏览器那张卡没占用方 → 画官方胶囊 fallback；终端那张由占用方自己画，
	// 所以它的标题**不该**由 fallback 画第二遍。
	if !texts["浏览器"] {
		t.Fatalf("fallback 胶囊没有画标题：%v", session.Texts())
	}
	if texts["新建终端"] {
		t.Fatalf("有占用方的卡不该再画一遍 fallback 标题：%v", session.Texts())
	}
}

// ── C. fallback 胶囊可点 ────────────────────────────────────────────

func TestDrawRightGuideFallbackCapsuleOpensKind(t *testing.T) {
	entries := []GuideEntry{
		{TabGuide: pluginapi.TabGuide{ID: "new", Title: "新建终端", Description: "在会话工作区运行命令"},
			ProviderID: "terminal", Kind: "terminal"},
	}
	opened := ""
	session := openGuideFrame(t, 420, 200, func(env *pluginapi.Env) func(ui.C) {
		env.HasKey = func(pluginapi.Slot, string) bool { return false }
		env.DrawKeyed = func(ui.C, pluginapi.Slot, string) {}
		env.Act.OpenRightTab = func(kind string) { opened = kind }
		return func(f ui.C) { DrawRightGuide(f, env, entries) }
	})
	if err := session.ClickLabel("新建终端"); err != nil {
		t.Fatalf("点向导胶囊失败：%v", err)
	}
	for i := 0; i < 20 && opened == ""; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if opened != "terminal" {
		t.Fatalf("点胶囊打开的类型 = %q，want terminal", opened)
	}
}

func terminalIconForTest() gfx.Icon {
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: []gfx.PathCommand{
		gfx.MoveTo(3, 4), gfx.LineTo(7, 8), gfx.LineTo(3, 12),
	}}
}
