package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

func browserOccurrenceModel(t *testing.T) *model {
	t.Helper()
	m := viewWiringModel(t)
	if err := m.kernel.Load(&viewRosterPlugin{id: "browser-roster", panels: []pluginapi.Painter{
		{ID: "browser", Slot: pluginapi.SlotRightPaneTab, Key: "browser", Label: "浏览器",
			Order: 0, Multiple: true, Draw: func(ui.C, *pluginapi.Env) {}},
	}}); err != nil {
		t.Fatal(err)
	}
	return m
}

func browserConfigCapture(t *testing.T) map[string]string {
	t.Helper()
	previous := uiConfig
	cfg := map[string]string{}
	uiConfig = cfg
	t.Cleanup(func() { uiConfig = previous })
	return cfg
}

func wireBrowserConfigCapture(m *model, cfg map[string]string) {
	m.actions.ConfigGet = func() map[string]string { return cfg }
	m.actions.ConfigSet = func(kv map[string]string) {
		for key, value := range kv {
			cfg[key] = value
		}
	}
}

func TestBrowserOpenCreatesIndependentOccurrencesAndRestoresLayout(t *testing.T) {
	cfg := browserConfigCapture(t)
	m := browserOccurrenceModel(t)
	wireBrowserConfigCapture(m, cfg)
	tabs := m.rightTabsFor(nil)
	if len(tabs) != 3 {
		t.Fatalf("默认 occurrence 数 = %d，want browser/timeline/terminal", len(tabs))
	}
	m.openRightTab("browser")
	tabs = m.rightTabsFor(nil)
	if len(tabs) != 4 {
		t.Fatalf("多开后 occurrence 数 = %d，want 4", len(tabs))
	}
	second := m.activeRightTabKey(tabs)
	if second == "" || second == "browser" {
		t.Fatalf("第二次 openTab(browser) 没有新 occurrence key：%q", second)
	}
	m.openRightTab("timeline")
	if got := len(m.rightTabsFor(nil)); got != 4 {
		t.Fatalf("单实例 timeline 被重复打开：%d", got)
	}
	m.selectRightTabKey(second)
	if got := m.activeRightTabKey(m.rightTabsFor(nil)); got != second {
		t.Fatalf("occurrence 选择失败：%q，want %q", got, second)
	}
	m.closeRightTab(second)
	if got := len(m.rightTabsFor(nil)); got != 3 {
		t.Fatalf("关闭后还剩 %d 个 occurrence，want 3", got)
	}
	m.openRightTab("browser")
	third := m.activeRightTabKey(m.rightTabsFor(nil))
	if third == "" || third == second || third == "browser" {
		t.Fatalf("重开没有分配新 tabId：%q", third)
	}

	// 新 model 没有内存 surface，必须从 dsh.sidebar-right.v1 恢复两个浏览器。
	restarted := browserOccurrenceModel(t)
	wireBrowserConfigCapture(restarted, cfg)
	restored := restarted.rightTabsFor(nil)
	browsers := 0
	for _, tab := range restored {
		if tab.Kind == "browser" {
			browsers++
		}
	}
	if len(restored) != 4 || browsers != 2 {
		t.Fatalf("重启恢复布局错误：len=%d browsers=%d tabs=%+v", len(restored), browsers, restored)
	}
	if got := restarted.activeRightTabKey(restored); got != third {
		t.Fatalf("重启后的 activeTabId = %q，want %q", got, third)
	}

	// 全部关闭是合法的持久化空态，不能在下次启动重新播种。
	for _, tab := range restored {
		restarted.closeRightTab(tab.Key)
	}
	empty := browserOccurrenceModel(t)
	wireBrowserConfigCapture(empty, cfg)
	if got := len(empty.rightTabsFor(nil)); got != 0 {
		t.Fatalf("全关后重启又播种了 %d 个 tab", got)
	}
	empty.openRightTab("browser")
	if got := len(empty.rightTabsFor(nil)); got != 1 {
		t.Fatalf("空态重新打开浏览器失败：%d", got)
	}
}

func TestRightTabMutationsWaitForConfigInsteadOfMintingCollisions(t *testing.T) {
	cfg := browserConfigCapture(t)
	seed := browserOccurrenceModel(t)
	wireBrowserConfigCapture(seed, cfg)
	seed.openRightTab("browser")
	savedRaw := cfg[rightSidebarConfigKey("s1")]
	if savedRaw == "" {
		t.Fatal("前置布局没有落盘")
	}

	// 模拟首帧 config.get 尚未返回：新建/关闭都必须明确等待，不能拿默认
	// minted 去覆盖旧布局，也不能把旧 tab 当成已关闭。
	uiConfig = nil
	m := browserOccurrenceModel(t)
	m.openRightTab("browser")
	if m.status == "" || len(m.rightTabsFor(nil)) != 3 {
		t.Fatal("配置未就绪时新建没有明确等待")
	}
	m.closeRightTab("browser")
	if m.status == "" || len(m.rightTabsFor(nil)) != 3 {
		t.Fatal("配置未就绪时关闭没有明确等待")
	}
	cfg = map[string]string{rightSidebarConfigKey("s1"): savedRaw}
	uiConfig = cfg
	wireBrowserConfigCapture(m, cfg)
	if got := len(m.rightTabsFor(nil)); got != 4 {
		t.Fatal("配置到达后没有恢复两个浏览器 occurrence：", got)
	}
	m.openRightTab("browser")
	if got := len(m.rightTabsFor(nil)); got != 5 {
		t.Fatal("配置到达后新建仍然失败：", got)
	}
}
