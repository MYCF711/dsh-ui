package dsactioncards

// bodies_test.go —— 官方「按 kind 注册卡片正文」这条通道（OFF:registry.ts）的落地测试。
//
// 三条钉子：
//
//	① 注册语义：同一 action 重复登记**报错不覆盖**（OFF:registry.ts:104-106）；
//	② 派发语义：注册了正文的 kind，**正文整段接管**卡片内部（官方没有 FallbackBody
//	   的「暂不需要」footer，OFF:Card.tsx:99-118）；
//	③ 动词归属：body 点出来的动词必须落在**这张卡**的 callId 上（官方 `send` 闭包
//	   绑的是 `proposal.callId`，OFF:Card.tsx:103）。
//
// 一个 viva 的硬约束（与本包其他测试同）：`ui.Box` 只在本次 paint 内有效，
// 所以断言全部对"画出来的文字 / 记录下来的调用"做，不跨帧读矩形。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// probeBody 是一个"会被派发到"的假正文：记下它收到的 props，并画一行可断言的文字。
type probeBody struct {
	action string
	draws  int
	last   pluginapi.ActionCardBodyProps
	// actOnDraw 非 nil 时在 Draw 里点一次（验动词归属）。
	actOnDraw func(props pluginapi.ActionCardBodyProps)
}

func (b *probeBody) Action() string { return b.action }

func (b *probeBody) Draw(f ui.C, _ *pluginapi.Env, props pluginapi.ActionCardBodyProps) {
	b.draws++
	b.last = props
	ui.Text(f, "PROBE-BODY")
	if b.actOnDraw != nil {
		b.actOnDraw(props)
	}
}

func TestRegisterBodyRejectsDuplicateAndEmptyAction(t *testing.T) {
	p := &Plugin{}
	if err := p.RegisterBody(nil); err == nil {
		t.Fatal("nil 正文必须报错")
	}
	if err := p.RegisterBody(&probeBody{action: ""}); err == nil {
		t.Fatal("空 action 必须报错")
	}
	if err := p.RegisterBody(&probeBody{action: "kind-a"}); err != nil {
		t.Fatalf("首次登记应当成功：%v", err)
	}
	// 重复登记 = 两个插件抢一种卡片的绘制权：必须报错，不许静默覆盖。
	if err := p.RegisterBody(&probeBody{action: "kind-a"}); err == nil {
		t.Fatal("重复登记必须报错")
	}
	if got := p.BodyFor("kind-a"); got == nil || got.Action() != "kind-a" {
		t.Fatalf("首次登记的那份必须还在：%+v", got)
	}
	if p.BodyFor("kind-b") != nil {
		t.Fatal("没登记过的 action 必须返回 nil")
	}
}

// cardViewFor 造一份只有一个提案（可选带决定）的投影。
func cardViewFor(action, callID string, decision *apiclient.ActionCardDecision) *apiclient.ActionCardView {
	view := &apiclient.ActionCardView{
		Proposals: map[string]apiclient.ActionCardProposal{
			callID: {
				CallID: callID, Turn: 1, Action: action,
				Title: "标题" + callID, Brief: "简介" + callID,
				Details: map[string]json.RawMessage{"name": json.RawMessage(`"acme.audit"`)},
			},
		},
		Decisions: map[string]apiclient.ActionCardDecision{},
	}
	if decision != nil {
		view.Decisions[decision.CallID] = *decision
	}
	return view
}

func TestRegisteredBodyTakesOverTheCard(t *testing.T) {
	src := &fakeSource{view: cardViewFor("kind-a", "call-a", nil)}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	body := &probeBody{action: "kind-a"}
	if err := p.RegisterBody(body); err != nil {
		t.Fatalf("登记失败：%v", err)
	}

	drawCardOnce(t, p, env, canvas, 640, 480)

	if body.draws != 1 {
		t.Fatalf("正文被画 %d 次，应为 1", body.draws)
	}
	if !hasText(canvas, "PROBE-BODY") {
		t.Fatal("正文画的节点没出现在画布上——说明派发没生效")
	}
	// FallbackBody 的「暂不需要」是它独有的：注册了正文就**不该**再有它。
	if hasText(canvas, zhDismiss) {
		t.Fatal("注册了正文却还画了官方 fallback 的 footer")
	}
	// props 必须带齐官方 ActionCardBodyProps 的那几格。
	if body.last.SessionID != "s1" || body.last.CallID != "call-a" || body.last.Action != "kind-a" {
		t.Fatalf("props 身份字段不对：%+v", body.last)
	}
	if body.last.Title != "标题call-a" || body.last.Brief != "简介call-a" {
		t.Fatalf("props 标题/简介没透传：%+v", body.last)
	}
	if name, ok := body.last.Details["name"]; !ok || string(name) != `"acme.audit"` {
		t.Fatalf("props details 没透传：%+v", body.last.Details)
	}
	if body.last.HasDecision {
		t.Fatal("这份投影没有决定，HasDecision 应为 false")
	}
	if body.last.Act == nil {
		t.Fatal("props.Act 不能为 nil：正文的按钮全靠它")
	}
}

// TestUnregisteredKindKeepsFallback 钉住"没注册就画官方 FallbackBody"。
func TestUnregisteredKindKeepsFallback(t *testing.T) {
	src := &fakeSource{view: cardViewFor("kind-b", "call-b", nil)}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	if err := p.RegisterBody(&probeBody{action: "kind-a"}); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 640, 480)
	if hasText(canvas, "PROBE-BODY") {
		t.Fatal("别的 kind 的正文被误派发了")
	}
	if !hasText(canvas, zhDismiss) {
		t.Fatal("没有正文时应当画官方 fallback 的「暂不需要」")
	}
}

// TestBodyVerbBindsToItsOwnCard 钉住动词归属：body 的 enable 落在自己的 callId 上。
func TestBodyVerbBindsToItsOwnCard(t *testing.T) {
	// 投影里放两张卡：LatestProposal 会选"最新一轮 + 最小 callId"，
	// 而 body 的动词必须落在**它自己**那张卡上。
	view := &apiclient.ActionCardView{
		Proposals: map[string]apiclient.ActionCardProposal{
			"call-old": {CallID: "call-old", Turn: 1, Action: "kind-a", Title: "旧"},
			"call-new": {CallID: "call-new", Turn: 2, Action: "kind-a", Title: "新"},
		},
		Decisions: map[string]apiclient.ActionCardDecision{},
	}
	src := &fakeSource{view: view, raw: json.RawMessage(`{"status":"settled","decision":{"callId":"call-new","action":"kind-a","phase":"enabled"}}`)}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	body := &probeBody{action: "kind-a"}
	body.actOnDraw = func(props pluginapi.ActionCardBodyProps) { props.Act("enable", nil) }
	if err := p.RegisterBody(body); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 640, 480)

	acted := src.acted()
	if len(acted) != 1 {
		t.Fatalf("应当刚好发一次动作，实际 %d 次：%+v", len(acted), acted)
	}
	if acted[0].callID != body.last.CallID || acted[0].session != "s1" {
		t.Fatalf("动词落错了卡：动作=%+v，正文那张卡=%q", acted[0], body.last.CallID)
	}
	if acted[0].verb != "enable" {
		t.Fatalf("动词被改写成了 %q", acted[0].verb)
	}
}

// TestBodySeesDecisionAndFeedback 钉住决定/反馈回灌给正文。
func TestBodySeesDecisionAndFeedback(t *testing.T) {
	decision := &apiclient.ActionCardDecision{
		CallID: "call-a", Action: "kind-a", Phase: "failed",
		Data: map[string]json.RawMessage{"errorCode": json.RawMessage(`"incompatible-version"`)},
	}
	src := &fakeSource{
		view: cardViewFor("kind-a", "call-a", decision),
		raw:  json.RawMessage(`{"status":"refused","reason":"access-denied"}`),
	}
	p, env, _, canvas := fixture(t, src)
	if err := p.Refresh(context.Background(), "s1"); err != nil {
		t.Fatalf("Refresh 失败：%v", err)
	}
	body := &probeBody{action: "kind-a"}
	if err := p.RegisterBody(body); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 640, 480)

	if !body.last.HasDecision || body.last.Phase != "failed" {
		t.Fatalf("决定没回灌：%+v", body.last)
	}
	if code := string(body.last.Data["errorCode"]); code != `"incompatible-version"` {
		t.Fatalf("决定的 data 没回灌：%v", body.last.Data)
	}

	// 再点一次让动作结算成 refused；**结算发生在那一帧里**，所以正文要在下一帧
	// 才看得到四变体（官方那边是 `await act()` 之后 setState，同一件事）。
	body.actOnDraw = func(props pluginapi.ActionCardBodyProps) { props.Act("enable", nil) }
	drawCardOnce(t, p, env, canvas, 640, 480)
	body.actOnDraw = nil
	drawCardOnce(t, p, env, canvas, 640, 480)
	if body.last.LastResult == nil {
		t.Fatal("动作结果没回灌给正文")
	}
	if body.last.LastResult.Status != apiclient.ActionCardRefused || body.last.LastResult.Reason != "access-denied" {
		t.Fatalf("四变体解错了：%+v", body.last.LastResult)
	}
}

// TestBodyDrawsNothingWithoutSource 钉住"没有数据源时整块不画"（正文也不该被叫到）。
func TestBodyDrawsNothingWithoutSource(t *testing.T) {
	p, env, _, canvas := fixture(t, nil)
	body := &probeBody{action: "kind-a"}
	if err := p.RegisterBody(body); err != nil {
		t.Fatalf("登记失败：%v", err)
	}
	drawCardOnce(t, p, env, canvas, 640, 480)
	if body.draws != 0 {
		t.Fatalf("没有数据源时正文被画了 %d 次", body.draws)
	}
	if len(textValues(canvas)) != 0 {
		t.Fatalf("没有数据源时画了内容：%v", textValues(canvas))
	}
}
