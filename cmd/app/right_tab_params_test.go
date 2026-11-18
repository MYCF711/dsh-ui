package main

// right_tab_params_test.go —— 官方 `openTab(kind, { params })` 的壳侧断言。
//
// 官方两个入口都用它：
//
//	浏览器 window.open 新标签  OFF:…/view/BrowserBody.tsx:62-70
//	终端 {terminalId}|{shellPath}  OFF:…/sidebar-terminal/src/client/face.ts:20-24
//	参数表                    OFF:…/sidebar-right/src/client/contract/params.ts:31-46

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestOpenRightTabParamsAttachesToOccurrence(t *testing.T) {
	cfg := browserConfigCapture(t)
	m := browserOccurrenceModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightTabParams("browser", pluginapi.RightTabParams{URL: "https://example.com/a"})
	tabID := m.activeRightTabKey(m.rightTabsFor(nil))
	if tabID == "" {
		t.Fatal("openTab 之后没有选中的 occurrence")
	}
	if got := m.rightTabParamsFor(m.cur, tabID); got.URL != "https://example.com/a" {
		t.Fatalf("occurrence %q 的 navigation.params = %+v", tabID, got)
	}

	// 多实例类型再开一次 = 独立 occurrence，各自的参数互不覆盖。
	m.openRightTabParams("browser", pluginapi.RightTabParams{URL: "https://example.com/b"})
	second := m.activeRightTabKey(m.rightTabsFor(nil))
	if second == tabID {
		t.Fatal("multiple 类型第二次 openTab 没有新建 occurrence")
	}
	if got := m.rightTabParamsFor(m.cur, second); got.URL != "https://example.com/b" {
		t.Fatalf("第二个 occurrence 的参数 = %+v", got)
	}
	if got := m.rightTabParamsFor(m.cur, tabID); got.URL != "https://example.com/a" {
		t.Fatalf("第一个 occurrence 的参数被覆盖了：%+v", got)
	}

	// 关闭 occurrence = 导航记录消失（官方 navigation 属于这一次打开，不跨关闭保留）。
	m.closeRightTab(second)
	if got := m.rightTabParamsFor(m.cur, second); got != (pluginapi.RightTabParams{}) {
		t.Fatalf("关闭后还留着参数：%+v", got)
	}
}

func TestOpenRightTabWithoutParamsClearsStaleParams(t *testing.T) {
	cfg := browserConfigCapture(t)
	m := browserOccurrenceModel(t)
	wireBrowserConfigCapture(m, cfg)

	// 单实例类型（timeline）复用同一个 occurrence：新的 open 不带参数时，
	// 官方 `navigation.params` 是 `undefined`，不能读到上一次的值。
	m.openRightTabParams("timeline", pluginapi.RightTabParams{URL: "https://example.com/old"})
	tabID := m.activeRightTabKey(m.rightTabsFor(nil))
	if got := m.rightTabParamsFor(m.cur, tabID); got.URL == "" {
		t.Fatal("带参数那次 open 没写进去")
	}
	m.openRightTab("timeline")
	if got := m.rightTabParamsFor(m.cur, tabID); got != (pluginapi.RightTabParams{}) {
		t.Fatalf("复用的 occurrence 还留着上一次的参数：%+v", got)
	}
}
