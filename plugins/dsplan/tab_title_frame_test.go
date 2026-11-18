package dsplan

// tab_title_frame_test.go —— 右栏芯片标题的「一帧延迟」回归（#28 ⑤）。
//
// # 延迟是怎么来的
//
// 官方标题座位是**渲染期同步读资源**：`PlanTitle` 里 `useResource<'plan'>(address)`
// 从**已经加载好的**会话历史取 `plan.title`，然后 `plan?.title ?? tab.tab.title`
// （OFF:PlanPreview.tsx:47-53）。React 同一次渲染就拿到值，所以第一帧就是对的。
//
// 本仓的顺序不一样：壳在 `m.view.Draw` **之前**投影右栏名录（`shellOptionsFor` →
// `rightTabsFor`，`cmd/app/shell.go`），而 `planTitles` 是这个包**画的时候**
// （`drawCards` / `drawPreview` 里的 `rememberPlanTitles`）才填的。于是刚提交计划
// 的那一帧芯片只能退到类型标题「计划」，下一帧才变成计划标题。
//
// # 修法（与官方同一条语义，不是新语义）
//
// 壳把**本帧该会话的原始帧**一并递给标题 provider
// （`pluginapi.RightTabAddressTitleProvider`），于是这里能像 `useResource` 一样
// 「在问的那一刻」算出标题。缓存命中是常态；只有新计划出现的那一帧才扫一遍帧。
//
// 下面这条测试**一帧都不画**：任何依赖「先画过一帧」的实现都会红。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestRightTabTitleIsCorrectOnThePlanFrame 钉住「计划帧到的同一帧，芯片标题就对」。
func TestRightTabTitleIsCorrectOnThePlanFrame(t *testing.T) {
	plugin := &Plugin{}
	frames := []apiclient.Frame{
		callFrame(t, 1, 1, "call-a", "exit_plan_mode", planArgs(t, "# 刚提交的方案\n\n正文。")),
	}
	address := planAddress("session-1", "call-a")

	// 关键断言：**没有**画过任何一帧（没调 `rememberPlanTitles`），标题就该在。
	// 变异（去掉 frames 现算、只读 planTitles）→ 这里得到空串，红。
	if got := plugin.RightTabTitleFor("session-1", "tab1", address, frames); got != "刚提交的方案" {
		t.Fatalf("刚提交那一帧的芯片标题 = %q，want「刚提交的方案」", got)
	}

	// 现算的结果要留在缓存里：下一帧即使壳给不出帧（切会话中/老壳）也还是对的。
	if got := plugin.RightTabTitleFor("session-1", "tab1", address, nil); got != "刚提交的方案" {
		t.Fatalf("现算出来的标题没有进缓存，第二帧又丢了：%q", got)
	}

	// 帧是**本会话**的：别的会话的地址仍然不许借标题（官方读的是地址里那个
	// 会话的 history，本仓只有当前会话的帧）。
	if got := plugin.RightTabTitleFor("session-2", "tab1", address, frames); got != "" {
		t.Fatalf("跨会话地址又借到了标题 %q", got)
	}

	// 帧里没有这个 callId：不编标题，壳保留类型标题（官方 `?? tab.tab.title`）。
	if got := plugin.RightTabTitleFor("session-1", "tab9", planAddress("session-1", "call-zzz"), frames); got != "" {
		t.Fatalf("帧里没有的 callId 拿到了标题 %q", got)
	}

	// 非计划地址（官方那条 `file` 协议地址）不归本包，任何情况下都不编。
	if got := plugin.RightTabTitleFor("session-1", "tab1", "dsh-resource://file/session-1/a.md", frames); got != "" {
		t.Fatalf("非计划地址拿到了标题 %q", got)
	}
}

// TestRightTabTitlePrefersCacheOverFrames 钉住「缓存是快路径」：标题一旦记下，
// 就不该再因为帧里找不到而丢掉它（同一 callId 的正文是不可变调用参数，
// 标题不会变——`rememberPlanTitles` 的约定）。
func TestRightTabTitlePrefersCacheOverFrames(t *testing.T) {
	plugin := &Plugin{}
	address := planAddress("session-1", "call-a")
	plugin.rememberPlanTitles([]Document{{CallID: "call-a", Title: "缓存里的标题"}})

	// 帧里完全没有这份计划（切会话、只加载了一页历史）：缓存仍然作数。
	frames := []apiclient.Frame{
		callFrame(t, 1, 1, "call-other", "exit_plan_mode", planArgs(t, "# 别的方案\n")),
	}
	if got := plugin.RightTabTitleFor("session-1", "tab1", address, frames); got != "缓存里的标题" {
		t.Fatalf("缓存被帧覆盖了：%q", got)
	}
}
