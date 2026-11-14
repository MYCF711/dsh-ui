// view_tabs_wiring_test.go —— 壳侧两个选择器座位的闸门与解析。
//
// 这里守的是「闸门搬到了注册表」这一条（2026-11-15）：
//
//	名录外的 key **不许**写进选择状态（否则界面会出现"点了一个不存在的页签，
//	主区整块空白"）
//	存的偏好失效时，对外交出去的**一定**是名录里的 key（否则壳画 chat、
//	页签上一个选中态都没有——自相矛盾）
package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/ui"
)

// viewRosterPlugin 只挂画，用来喂名录。
type viewRosterPlugin struct {
	id     string
	panels []pluginapi.Painter
}

func (p *viewRosterPlugin) ID() string         { return p.id }
func (p *viewRosterPlugin) Provides() []string { return nil }
func (p *viewRosterPlugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}
func (p *viewRosterPlugin) Register(pluginapi.Context) error { return nil }
func (p *viewRosterPlugin) Panels() []pluginapi.Painter      { return p.panels }

func viewWiringModel(t *testing.T) *model {
	t.Helper()
	k := uikernel.New("view-tabs-wiring")
	// dstopbar 提供 viewapi.Store（会话视图选择的既有唯一出处）。
	plugins := []pluginapi.Plugin{
		&dscore.Plugin{Dark: true},
		&dstopbar.Plugin{},
		&viewRosterPlugin{id: "roster", panels: []pluginapi.Painter{
			{ID: "chat", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewChat,
				Label: "对话", Draw: func(ui.C, *pluginapi.Env) {}},
			{ID: "traj", Slot: pluginapi.SlotConvView, Key: pluginapi.ViewTrajectory,
				Label: "轨迹", Order: 10, Draw: func(ui.C, *pluginapi.Env) {}},
			{ID: "timeline", Slot: pluginapi.SlotRightPaneTab, Key: "timeline",
				Label: "轨迹", Order: 10, Draw: func(ui.C, *pluginapi.Env) {}},
			{ID: "terminal", Slot: pluginapi.SlotRightPaneTab, Key: "terminal",
				Label: "终端", Order: 20, Draw: func(ui.C, *pluginapi.Env) {}},
		}},
	}
	if err := k.Load(plugins...); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	return &model{cur: "s1", kernel: k}
}

func TestActiveViewKeyFallsBackToChat(t *testing.T) {
	m := viewWiringModel(t)
	if got := m.activeViewKey(); got != pluginapi.ViewChat {
		t.Fatalf("没选过时 = %q，want chat", got)
	}
	if got := m.activeRightTabKey(uikernel.RightTabs(m.kernel)); got != "timeline" {
		t.Fatalf("没选过右栏时 = %q，want 名录第一条 timeline", got)
	}
}

func TestSelectViewKeyRejectsUnregistered(t *testing.T) {
	m := viewWiringModel(t)
	m.selectViewKey(pluginapi.ViewTrajectory)
	if got := m.activeViewKey(); got != pluginapi.ViewTrajectory {
		t.Fatalf("注册过的 view 没切过去：%q", got)
	}
	// 没注册过的 key：**什么都不做**（官方 resolveActiveView 也不会凭空造视图）。
	m.selectViewKey("theme-fish")
	if got := m.activeViewKey(); got != pluginapi.ViewTrajectory {
		t.Fatalf("名录外的 key 改了状态：%q", got)
	}
	m.selectViewKey("")
	if got := m.activeViewKey(); got != pluginapi.ViewTrajectory {
		t.Fatalf("空 key 改了状态：%q", got)
	}
}

func TestStaleStoredViewResolvesBackToRoster(t *testing.T) {
	m := viewWiringModel(t)
	store := m.viewStore()
	if store == nil {
		t.Fatal("dstopbar 应当提供了 viewapi.Store")
	}
	// 模拟"选了某个扩展的 view，然后那个插件被卸载"。
	store.Select("s1", "unloaded-plugin-view")
	if got := m.activeViewKey(); got != pluginapi.ViewChat {
		t.Fatalf("失效偏好应当落回 chat，实际 %q —— 直接交原值会让壳画 chat 而页签没有选中态", got)
	}
}

func TestSelectRightTabKeyRejectsUnregistered(t *testing.T) {
	m := viewWiringModel(t)
	tabs := uikernel.RightTabs(m.kernel)
	m.selectRightTabKey("terminal")
	if got := m.activeRightTabKey(tabs); got != "terminal" {
		t.Fatalf("注册过的 tab 没切过去：%q", got)
	}
	m.selectRightTabKey("nope")
	if got := m.activeRightTabKey(tabs); got != "terminal" {
		t.Fatalf("名录外的 tab key 改了状态：%q", got)
	}
}

func TestSelectorsDoNothingWithoutSession(t *testing.T) {
	m := viewWiringModel(t)
	m.cur = ""
	m.selectViewKey(pluginapi.ViewTrajectory)
	m.selectRightTabKey("terminal")
	if m.rightTabSel != nil {
		t.Fatalf("空会话不该写右栏选择：%v", m.rightTabSel)
	}
	if got := m.activeViewKey(); got != "" {
		t.Fatalf("空会话不该有 view：%q", got)
	}
}
