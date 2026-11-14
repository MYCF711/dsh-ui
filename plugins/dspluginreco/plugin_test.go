// plugin_test.go —— 无头测试：真画一帧（ui.NewPage + gfx.Canvas），点真的 Hit id，
// 断言画面文字与真实动作调用。
//
// 官方证据全部在 plugin.go 的常量注释里（OFF: 行号 + 对应哪个 CSS 值/哪一条文案）。
//
// 两条本仓的测试规矩：
//
//	① `ui.Box` 只在本次 paint 内有效 → 断言只对**画布上的文字**与**记录下来的调用**做；
//	② 异步结果**不许立刻读**：查询/动作都要 pump 到条件成立（这里因为 `run` 换成同步
//	   执行，条件在同一帧里就成立了；换成产品路径的 goroutine 时同一套 pump 写法仍然成立）。
package dspluginreco

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsactioncards"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 假数据源：记录每一次真实查询 ──────────────────────────────────────────

type describeCall struct {
	sessionID string
	callID    string
}

type fakeSource struct {
	details *apiclient.PluginRecommendationDetails
	err     error
	calls   []describeCall
}

func (f *fakeSource) Describe(_ context.Context, sessionID, callID string) (*apiclient.PluginRecommendationDetails, error) {
	f.calls = append(f.calls, describeCall{sessionID: sessionID, callID: callID})
	if f.err != nil {
		return nil, f.err
	}
	return f.details, nil
}

func availableDetails(title, description string, enabled bool) *apiclient.PluginRecommendationDetails {
	return &apiclient.PluginRecommendationDetails{
		Status:      apiclient.PluginRecommendationAvailable,
		Title:       title,
		Description: description,
		Enabled:     enabled,
	}
}

func refusedDetails(reason string) *apiclient.PluginRecommendationDetails {
	return &apiclient.PluginRecommendationDetails{Status: apiclient.PluginRecommendationRefused, Reason: reason}
}

// ── 夹具 ─────────────────────────────────────────────────────────────────

func fixture(t *testing.T, src RecoSource) (*Plugin, *pluginapi.Env, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("pluginreco-test")
	// `run` 换成同步执行：产品路径把宿主调用丢给 goroutine，测试必须在同一线程看到
	// 结算结果（同 dsactioncards / dsdirpicker 的思路）。
	p := &Plugin{theme: theme, run: func(fn func()) { fn() }}
	env := &pluginapi.Env{
		Theme: theme,
		Tree:  tree,
		Feat:  feat,
		Data:  pluginapi.Snapshot{Cur: "s1"},
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	SetSource(src)
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })
	return p, env, canvas
}

// propsFor 造一张卡的正文参数（官方 proposal/decision 的投影）。
func propsFor(phase string, data map[string]json.RawMessage) pluginapi.ActionCardBodyProps {
	props := pluginapi.ActionCardBodyProps{
		SessionID: "s1",
		CallID:    "call-1",
		Action:    Action,
		Title:     "审计插件",
		Brief:     "按这份说明做安全审计。",
		Details:   map[string]json.RawMessage{"name": json.RawMessage(`"acme.audit"`)},
	}
	if phase != "" {
		props.HasDecision = true
		props.Phase = phase
		props.Data = data
	}
	return props
}

// drawFrame 画一帧并返回画布。clickID 非空时把这一帧当成"点了那个 id"。
func drawFrame(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, props pluginapi.ActionCardBodyProps, clickID string) {
	t.Helper()
	canvas.Reset()
	if clickID != "" {
		canvas.BeginEventPass(env.Tree.ID(env.Feat, clickID), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	pageTheme := dscore.VivaTheme(env.Theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 640, Height: 520}, func(f ui.C) { p.Draw(f, env, props) })
}

// pumpUntil 反复画帧直到条件成立（异步结果落地后必须 pump，不许立刻读）。
//
// 上限 8 帧：本夹具的动作/查询都是同步执行的，1 帧就该成立；跑到 8 帧说明
// "要等下一帧才收敛"这条链断了（那时失败信息要能说明它等过）。
func pumpUntil(t *testing.T, done func() bool, draw func()) {
	t.Helper()
	for i := 0; i < 8; i++ {
		if done() {
			return
		}
		draw()
	}
	if !done() {
		t.Fatal("pump 8 帧后条件仍不成立")
	}
}

func texts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

// ── 1. 可启用：header + 目录文案 + 两个按钮 ─────────────────────────────

func TestAvailableCardDrawsCatalogTextAndTwoActions(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixture(t, src)
	src.calls = nil
	props := propsFor("", nil)

	pumpUntil(t, func() bool {
		drawFrame(t, p, env, canvas, props, "")
		return !hasText(canvas, zhEnable) || len(src.calls) > 0 && src.calls[0].sessionID == "s1"
	}, func() {})

	// 查询必须带**这张会话 + 这张卡**的作用域，且只发一次。
	if len(src.calls) != 1 {
		t.Fatalf("describe 调用 %d 次，应为 1：%+v", len(src.calls), src.calls)
	}
	if src.calls[0].sessionID != "s1" || src.calls[0].callID != "call-1" {
		t.Fatalf("describe 作用域错了：%+v", src.calls[0])
	}
	for _, want := range []string{zhHeading, zhOfficialBadge, "安全审计", "扫描依赖并报告风险。", zhScope, zhDismiss} {
		if !hasText(canvas, want) {
			t.Fatalf("画面缺少 %q；实际=%v", want, texts(canvas))
		}
	}
	// 标题来自目录（不是 proposal 的标题）：官方 `title = available ? resolveText(available.title) : proposal.title`。
	if hasText(canvas, "审计插件") {
		t.Fatal("拿到目录文案后仍画了 proposal 的标题")
	}
}

// ── 2. 已启用：只画状态行，不给启用入口 ─────────────────────────────────

func TestAlreadyEnabledCardShowsStatusWithoutEnable(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", true)}
	p, env, canvas := fixture(t, src)
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "")

	if !hasText(canvas, zhAlreadyOn) {
		t.Fatalf("缺少「已在插件页开启」状态行；实际=%v", texts(canvas))
	}
	if hasText(canvas, zhEnable) {
		t.Fatal("已经开着的插件不该再给「启用」")
	}
}

// ── 3. 被拒：画专属文案，不给启用入口 ───────────────────────────────────

func TestRefusedCardShowsReasonAndHidesEnable(t *testing.T) {
	for reason, want := range reasonText {
		reason, want := reason, want
		t.Run(reason, func(t *testing.T) {
			src := &fakeSource{details: refusedDetails(reason)}
			p, env, canvas := fixture(t, src)
			props := propsFor("", nil)
			drawFrame(t, p, env, canvas, props, "")
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, want) {
				t.Fatalf("缺少拒绝文案 %q；实际=%v", want, texts(canvas))
			}
			if hasText(canvas, zhEnable) {
				t.Fatal("被拒的插件不该给「启用」")
			}
			// 官方在被拒那一档也给「重试」（`lookupProblem` 存在时才画）。
			if !hasText(canvas, zhRetryLookup) {
				t.Fatal("被拒时应当给「重试」")
			}
		})
	}
}

// ── 4. 查询失败：画 lookup-failed，并允许重试 ───────────────────────────

func TestLookupFailureDrawsRetryAndRetryRefetches(t *testing.T) {
	src := &fakeSource{err: errors.New("连接断了")}
	p, env, canvas := fixture(t, src)
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, zhLookupFailed) {
		t.Fatalf("缺少「无法读取插件信息」；实际=%v", texts(canvas))
	}
	before := len(src.calls)
	// 点「重试」→ 下一帧必须**重新查一次**。
	drawFrame(t, p, env, canvas, props, "reco-retry-lookup-call-1")
	drawFrame(t, p, env, canvas, props, "")
	if len(src.calls) != before+1 {
		t.Fatalf("重试后 describe 调用 %d 次，应为 %d", len(src.calls), before+1)
	}
	// 宿主原文**不进卡片**：这张卡只有官方那 27 条文案。
	if hasText(canvas, "连接断了") {
		t.Fatal("宿主的错误原文被画进卡片了")
	}
}

// ── 5. 启用/暂不需要：动词落在自己的 callId 上 ─────────────────────────

func TestCardActionsSendKindVerbs(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixture(t, src)
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "")

	var verbs []string
	var callIDs []string
	props.Act = func(verb string, _ map[string]json.RawMessage) {
		verbs = append(verbs, verb)
		callIDs = append(callIDs, props.CallID)
	}
	drawFrame(t, p, env, canvas, props, "reco-enable-call-1")
	drawFrame(t, p, env, canvas, props, "reco-dismiss-call-1")

	if len(verbs) != 2 || verbs[0] != "enable" || verbs[1] != "dismiss" {
		t.Fatalf("动词 = %v，应为 [enable dismiss]", verbs)
	}
	for _, id := range callIDs {
		if id != "call-1" {
			t.Fatalf("动词落到别的卡上了：%q", id)
		}
	}
}

// ── 6. 决定 phase 的状态行（官方 statusKey）─────────────────────────────

func TestDecisionPhasesMapToOfficialStatus(t *testing.T) {
	cases := []struct {
		phase string
		data  map[string]json.RawMessage
		want  string
	}{
		{"dismissed", nil, zhDismissed},
		{"replaced", nil, zhReplaced},
		{"enabled", nil, zhEnabled},
		{"enabled", map[string]json.RawMessage{"application": json.RawMessage(`"restart-required"`)}, zhRestartNeeded},
		{"unconfirmed", nil, zhUnconfirmed},
		{"failed", nil, zhFailed},
	}
	for _, c := range cases {
		c := c
		t.Run(c.phase, func(t *testing.T) {
			src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
			p, env, canvas := fixture(t, src)
			props := propsFor(c.phase, c.data)
			drawFrame(t, p, env, canvas, props, "")
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, c.want) {
				t.Fatalf("phase=%s 缺状态行 %q；实际=%v", c.phase, c.want, texts(canvas))
			}
			// 定局之后不再给动作（官方 `offered = !settled && …`）。
			if c.phase != "failed" && c.phase != "unconfirmed" {
				if hasText(canvas, zhEnable) || hasText(canvas, zhDismiss) {
					t.Fatalf("phase=%s 已经定局，不该还有动作按钮", c.phase)
				}
			}
		})
	}
}

// ── 7. 启用失败的两条专属文案（不看 errorCode 就只说 failed）─────────────

func TestFailedPhaseWithErrorCodeDrawsSpecificAlert(t *testing.T) {
	cases := map[string]string{
		"incompatible-version": zhVersionError,
		"profile-write-failed": zhManageError,
	}
	for code, want := range cases {
		code, want := code, want
		t.Run(code, func(t *testing.T) {
			src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
			p, env, canvas := fixture(t, src)
			props := propsFor("failed", map[string]json.RawMessage{"errorCode": json.RawMessage(`"` + code + `"`)})
			drawFrame(t, p, env, canvas, props, "")
			drawFrame(t, p, env, canvas, props, "")
			if !hasText(canvas, want) {
				t.Fatalf("errorCode=%s 缺 %q；实际=%v", code, want, texts(canvas))
			}
		})
	}
}

// ── 8. 「查看详情」跳到插件页；没有插件页就不给可点入口 ─────────────────

func TestDetailsOpensPluginPageWhenAvailable(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixture(t, src)
	var opened []string
	SetOnOpenDetails(func(name string) { opened = append(opened, name) })
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "")
	drawFrame(t, p, env, canvas, props, "reco-details-call-1")
	if len(opened) != 1 || opened[0] != "acme.audit" {
		t.Fatalf("「查看详情」打开了 %v，应为 [acme.audit]", opened)
	}
}

// ── 9. details 里没有 name → 只画 summary，不给动作 ─────────────────────

func TestMissingNameDrawsSummaryOnly(t *testing.T) {
	src := &fakeSource{details: availableDetails("安全审计", "扫描依赖并报告风险。", false)}
	p, env, canvas := fixture(t, src)
	props := propsFor("", nil)
	props.Details = nil
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, "审计插件") || !hasText(canvas, "按这份说明做安全审计。") {
		t.Fatalf("缺 name 时应当画 proposal 的标题+简介；实际=%v", texts(canvas))
	}
	if hasText(canvas, zhEnable) || hasText(canvas, zhDismiss) {
		t.Fatal("缺 name 时不该给任何动作")
	}
	if len(src.calls) != 0 {
		t.Fatalf("缺 name 时不该去查目录，实际查了 %d 次", len(src.calls))
	}
}

// ── 10. 没有数据源 → 只画标题 + 简介（画死按钮比不画更糟）───────────────

func TestNilSourceDrawsReadOnlyTitle(t *testing.T) {
	p, env, canvas := fixture(t, nil)
	props := propsFor("", nil)
	drawFrame(t, p, env, canvas, props, "")
	if !hasText(canvas, "审计插件") || !hasText(canvas, "按这份说明做安全审计。") {
		t.Fatalf("没有数据源时应当画标题+简介；实际=%v", texts(canvas))
	}
	if hasText(canvas, zhEnable) || hasText(canvas, zhDismiss) {
		t.Fatal("没有数据源时不该给动作按钮")
	}
}

// ── 11. 注册：正文真的进了动作卡框架的注册表 ─────────────────────────────

// TestRegistersIntoActionCardRegistry 走一遍真装配（uikernel.Load），
// 断言 ds-actioncards 的注册表里确实有 `enable_official_plugin` 这一格。
//
// 这条替代"只在插件内自测"：官方的接法是 `ctx.actionCards.register(...)`，
// 装配顺序（本节 Requires）错了就永远注册不上，而单测内部调用看不出这一点。
func TestRegistersIntoActionCardRegistry(t *testing.T) {
	themePlugin := &dscore.Plugin{Dark: true}
	cards := &dsactioncards.Plugin{}
	reco := &Plugin{}
	kernel := uikernel.New("plugin-reco-registry-test")
	if err := kernel.Load(themePlugin, cards, reco); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	body := cards.BodyFor(Action)
	if body == nil {
		t.Fatal("动作卡注册表里没有 enable_official_plugin 的正文")
	}
	if body.Action() != Action {
		t.Fatalf("注册的 action = %q", body.Action())
	}
}
