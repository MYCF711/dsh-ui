package main

// right_tab_badge_test.go —— 壳把插件的芯片徽标投影进 `pluginapi.Tab.Badge` 的断言。
//
// 官方：`sidebar.right.pane.tab.title` 的占用方在标题前画一枚 `Tag`
// （OFF:…/sidebar-terminal/src/client/TerminalTitle.tsx:44-50）。本仓的壳在投影名录时
// 问一遍 `RightTabBadgeProvider`，与「实时标题」是同一个模式。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// badgeRosterPlugin 是一个既挂类型画、又提供芯片徽标的插件替身。
type badgeRosterPlugin struct {
	viewRosterPlugin
	badge pluginapi.TabBadge
}

func (p *badgeRosterPlugin) RightTabBadge(sessionID, tabID string) (pluginapi.TabBadge, bool) {
	if sessionID == "" {
		return pluginapi.TabBadge{}, false
	}
	return p.badge, true
}

func TestRightTabsCarryPluginBadge(t *testing.T) {
	m := viewWiringModel(t)
	want := pluginapi.TabBadge{Text: "Agent", Tone: pluginapi.TabBadgeInfo, Hint: "由 Agent 打开"}
	if err := m.kernel.Load(&badgeRosterPlugin{
		viewRosterPlugin: viewRosterPlugin{id: "badge-roster", panels: []pluginapi.Painter{
			{ID: "terminal", Slot: pluginapi.SlotRightPaneTab, Key: "terminal", Label: "终端",
				Order: 0, Multiple: true, Draw: func(ui.C, *pluginapi.Env) {}},
		}},
		badge: want,
	}); err != nil {
		t.Fatal(err)
	}
	// 徽标跟着会话走（官方标题座位的占用方也要 sessionId）：这里造一个当前会话，
	// 好断言"壳真的逐条问了插件"。
	m.mu.Lock()
	m.cur = "s1"
	m.mu.Unlock()
	tabs := m.rightTabsFor(nil)
	if len(tabs) == 0 {
		t.Fatal("没有右栏 tab 名录")
	}
	found := false
	for _, tab := range tabs {
		if tab.Key != "terminal" {
			continue
		}
		found = true
		if tab.Badge == nil || *tab.Badge != want {
			t.Fatalf("tab %q 的 Badge = %+v，want %+v", tab.Key, tab.Badge, want)
		}
	}
	if !found {
		t.Fatalf("名录里没有 terminal：%+v", tabs)
	}
}
