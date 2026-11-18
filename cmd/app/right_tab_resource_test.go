package main

// right_tab_resource_test.go —— 官方 `openResource(address)` 的壳侧断言（#28 ④）。
//
// 官方契约（唯一权威）：
//
//	openResource(address, options?)  OFF:…/sidebar-right/src/client/contract/slots.ts:165-177
//	tab-registry.ts:140-160          `contentId` = 地址本身
//	                                 「Two opens of the same address are the same tab,
//	                                  which is what makes opening idempotent.」
//	tab-registry.ts:110-121          `navigation{address, params, revision}`
//
// 所以本文件钉三件事：
//
//	① 同一 (kind, address) 只该有一枚 occurrence（幂等）
//	② 不同 address 各自一枚——**不靠 Multiple**，靠地址
//	③ 地址进 `Tab.Address`（= 官方 contentId）与 `Env.RightTabAddress`，落盘能还原

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	testPlanAddressA = "dsh-resource://plan/s1/call-a"
	testPlanAddressB = "dsh-resource://plan/s1/call-b"
)

// resourceTitlePlugin 是一个资源型 tab 类型 + 官方标题座位的替身：
// 画挂 `sidebar.right.pane.tab`，并且按**地址**给实时标题
// （`RightTabAddressTitleProvider`，官方 `PlanTitle` 的文案那一半）。
type resourceTitlePlugin struct {
	viewRosterPlugin
	titles map[string]string
}

func (p *resourceTitlePlugin) RightTabTitleFor(sessionID, tabID, address string, frames []apiclient.Frame) string {
	return p.titles[address]
}

func resourceTabModel(t *testing.T) (*model, *resourceTitlePlugin) {
	t.Helper()
	m := viewWiringModel(t)
	plugin := &resourceTitlePlugin{
		viewRosterPlugin: viewRosterPlugin{id: "resource-roster", panels: []pluginapi.Painter{
			// 非 Multiple：页型语义下同 kind 只能有一枚。
			// 资源型靠地址并存——这正是「不靠 Multiple」要证明的那一点。
			{ID: "plan", Slot: pluginapi.SlotRightPaneTab, Key: "plan", Label: "计划",
				Order: 30, Draw: func(ui.C, *pluginapi.Env) {}},
		}},
		titles: map[string]string{},
	}
	if err := m.kernel.Load(plugin); err != nil {
		t.Fatal(err)
	}
	return m, plugin
}

func rightTabsOfKind(tabs []pluginapi.Tab, kind string) []pluginapi.Tab {
	out := make([]pluginapi.Tab, 0, len(tabs))
	for _, tab := range tabs {
		if tab.Kind == kind {
			out = append(out, tab)
		}
	}
	return out
}

// TestOpenRightResourceIsIdempotentPerAddress 钉住官方的幂等：
// 同一地址打开两次只有一枚 tab，且 tabs 数不涨、ID 不变。
func TestOpenRightResourceIsIdempotentPerAddress(t *testing.T) {
	cfg := browserConfigCapture(t)
	m, _ := resourceTabModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightResource("plan", testPlanAddressA, pluginapi.RightTabParams{})
	first := m.activeRightTabKey(m.rightTabsFor(nil))
	if first == "" {
		t.Fatal("openResource 之后没有选中的 occurrence")
	}
	if got := m.rightTabAddressFor(m.cur, first); got != testPlanAddressA {
		t.Fatalf("occurrence %q 的 navigation.address = %q，want %q", first, got, testPlanAddressA)
	}

	// 同一地址第二次：官方「same tab」——不新建、不换 ID。
	m.openRightResource("plan", testPlanAddressA, pluginapi.RightTabParams{})
	second := m.activeRightTabKey(m.rightTabsFor(nil))
	if second != first {
		t.Fatalf("同一地址第二次 openResource 换了 occurrence：%q → %q（官方是同一枚 tab）", first, second)
	}
	if got := len(rightTabsOfKind(m.rightTabsFor(nil), "plan")); got != 1 {
		t.Fatalf("同一地址开了 %d 枚 plan tab，want 1", got)
	}
}

// TestOpenRightResourceKeepsOneTabPerAddress 钉住官方的并存：
// 不同地址 = 不同 tab，且**不需要** Multiple。
func TestOpenRightResourceKeepsOneTabPerAddress(t *testing.T) {
	cfg := browserConfigCapture(t)
	m, _ := resourceTabModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightResource("plan", testPlanAddressA, pluginapi.RightTabParams{})
	aID := m.activeRightTabKey(m.rightTabsFor(nil))
	m.openRightResource("plan", testPlanAddressB, pluginapi.RightTabParams{})
	bID := m.activeRightTabKey(m.rightTabsFor(nil))

	if aID == "" || bID == "" || aID == bID {
		t.Fatalf("两个地址没有各自一枚 tab：a=%q b=%q", aID, bID)
	}
	plans := rightTabsOfKind(m.rightTabsFor(nil), "plan")
	if len(plans) != 2 {
		t.Fatalf("plan tab 数 = %d，want 2（地址就是身份，Multiple 与它无关）", len(plans))
	}
	addresses := map[string]string{}
	for _, tab := range plans {
		addresses[tab.Key] = tab.Address
	}
	if addresses[aID] != testPlanAddressA || addresses[bID] != testPlanAddressB {
		t.Fatalf("Tab.Address 投影错位：%+v", addresses)
	}
}

// TestOpenRightResourceEmptyAddressFallsBackToKind 钉住退化：
// 空地址不是"造一枚无身份 tab"，而是按类型打开（页型语义）。
func TestOpenRightResourceEmptyAddressFallsBackToKind(t *testing.T) {
	cfg := browserConfigCapture(t)
	m, _ := resourceTabModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightResource("plan", "", pluginapi.RightTabParams{})
	tabID := m.activeRightTabKey(m.rightTabsFor(nil))
	if tabID == "" {
		t.Fatal("空地址时没有退化出可用的 occurrence")
	}
	if got := m.rightTabAddressFor(m.cur, tabID); got != "" {
		t.Fatalf("空地址退化出来的 tab 带了地址 %q", got)
	}
}

// TestRightTabAddressSurvivesPersistence 钉住地址进落盘、能还原，且**违反唯一性
// 的盘面被拒**（同一 (kind, address) 两枚 = 写侧出过问题）。
func TestRightTabAddressSurvivesPersistence(t *testing.T) {
	cfg := browserConfigCapture(t)
	m, _ := resourceTabModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightResource("plan", testPlanAddressA, pluginapi.RightTabParams{})
	m.openRightResource("plan", testPlanAddressB, pluginapi.RightTabParams{})
	raw := cfg[rightSidebarConfigKey(m.cur)]
	if raw == "" {
		t.Fatal("openResource 之后没有落盘")
	}
	restored, err := decodeRightTabSurface(m.cur, raw)
	if err != nil {
		t.Fatalf("还原失败：%v", err)
	}
	seen := map[string]bool{}
	for _, record := range restored.Tabs {
		if record.Kind == "plan" {
			if record.Address == "" {
				t.Fatalf("落盘丢了地址：%+v", record)
			}
			seen[record.Address] = true
		}
	}
	if !seen[testPlanAddressA] || !seen[testPlanAddressB] {
		t.Fatalf("两个地址没有都还原：%+v", seen)
	}

	duplicate := `{"bySession":{"s1":{"tabs":[` +
		`{"id":"tab1","kind":"plan","address":"` + testPlanAddressA + `"},` +
		`{"id":"tab2","kind":"plan","address":"` + testPlanAddressA + `"}],` +
		`"activeTabId":"tab1","minted":2}}}`
	if _, err := decodeRightTabSurface("s1", duplicate); err == nil {
		t.Fatal("同一 (kind,address) 出现两枚时应判盘面无效")
	}
}

// TestResourceTitleReplacesTypeLabel 钉住官方的 `plan?.title ?? tab.tab.title`：
// 有资源标题就用它，没有就保留类型标题。
func TestResourceTitleReplacesTypeLabel(t *testing.T) {
	cfg := browserConfigCapture(t)
	m, plugin := resourceTabModel(t)
	wireBrowserConfigCapture(m, cfg)

	m.openRightResource("plan", testPlanAddressA, pluginapi.RightTabParams{})
	tabID := m.activeRightTabKey(m.rightTabsFor(nil))
	labelOf := func() string {
		for _, tab := range m.rightTabsFor(nil) {
			if tab.Key == tabID {
				return tab.Label
			}
		}
		return ""
	}
	if got := labelOf(); got != "计划" {
		t.Fatalf("没有资源标题时标签 = %q，want 类型标题「计划」", got)
	}

	plugin.titles[testPlanAddressA] = "迁移到 Go 宿主"
	if got := labelOf(); got != "迁移到 Go 宿主" {
		t.Fatalf("有资源标题时标签 = %q，want「迁移到 Go 宿主」", got)
	}

	// 地址对不上的标题不许串到别的 tab 上（同 kind 多地址时最容易串）。
	plugin.titles = map[string]string{testPlanAddressB: "另一份计划"}
	if got := labelOf(); got != "计划" {
		t.Fatalf("别的地址的标题串了进来：%q", got)
	}
}

// framesTitlePlugin 只在壳把**帧**递过来时才给标题：用来钉「壳确实把本帧帧给了
// provider」——这正是官方 `PlanTitle` 里 `useResource` 在渲染期同步读资源的等价物
// （OFF:PlanPreview.tsx:47-53）。没有帧就没有标题，所以这个替身一旦拿到标题，
// 就证明壳在 Draw 之前已经把数据递到了。
type framesTitlePlugin struct {
	viewRosterPlugin
}

func (p *framesTitlePlugin) RightTabTitleFor(sessionID, tabID, address string, frames []apiclient.Frame) string {
	if len(frames) == 0 {
		return ""
	}
	return "帧里的标题"
}

// TestRightTabTitleProviderReceivesFramesBeforeDraw 钉住 #28 ⑤ 的修法：
// 壳投影名录时（`shellOptionsFor` → `rightTabsFor`，跑在 `m.view.Draw` 之前）
// 必须把本帧该会话的原始帧交给资源型标题 provider，否则插件只能读到上一帧缓存，
// 刚提交那一帧的芯片会先显示类型标题「计划」。
//
// 变异：`rightTabsFor` 里把 `frames` 换成 `nil`（或只给 `RightTabTitleProvider`
// 不给带地址的那一支）→ 第一条断言红。
func TestRightTabTitleProviderReceivesFramesBeforeDraw(t *testing.T) {
	m := viewWiringModel(t)
	plugin := &framesTitlePlugin{viewRosterPlugin: viewRosterPlugin{id: "frames-roster", panels: []pluginapi.Painter{
		{ID: "plan", Slot: pluginapi.SlotRightPaneTab, Key: "plan", Label: "计划",
			Order: 30, Draw: func(ui.C, *pluginapi.Env) {}},
	}}}
	if err := m.kernel.Load(plugin); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.cur = "s1"
	m.rightTabSurfaces = map[string]*rightTabSurface{}
	// 配置已加载（产品里由 `uiConfig` 到位触发；测试里直接置位，否则
	// `openRightTabAt` 会以「右栏配置尚未加载」拒绝新建）。
	m.rightTabLoaded = map[string]bool{"s1": true}
	m.rightTabDirty = map[string]bool{}
	m.mu.Unlock()

	// 按地址打开一枚 plan tab（官方 openResource；地址就是 contentId）。
	m.openRightTabAt("plan", testPlanAddressA, pluginapi.RightTabParams{})

	frames := []apiclient.Frame{{Seq: 1, Type: "tool/call"}}

	// 壳画芯片用的名录就是这个函数（`shellOptionsFor` 里同一行），帧必须传下去。
	opts := shellOptionsFor(m, m.kernel, 1182, 832, frames)
	label := ""
	for _, tab := range opts.RightTabs {
		if tab.Key == "plan" && tab.Address == testPlanAddressA {
			label = tab.Label
		}
	}
	if label != "帧里的标题" {
		t.Fatalf("计划 tab 标题 = %q，want「帧里的标题」——壳没把帧交给 provider，"+
			"第一帧就会退成类型标题", label)
	}

	// 没有帧（老壳 / 还没加载）时退回类型标题，不许编。
	opts = shellOptionsFor(m, m.kernel, 1182, 832, nil)
	for _, tab := range opts.RightTabs {
		if tab.Key == "plan" && tab.Address == testPlanAddressA && tab.Label != "计划" {
			t.Fatalf("没有帧时标题 = %q，want 类型标题「计划」", tab.Label)
		}
	}
}
