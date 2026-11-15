package dsplan

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// planFrames 造一份"两轮、三份计划"的会话：第 1 轮一份、第 3 轮两份。
func planFrames(t *testing.T) []apiclient.Frame {
	t.Helper()
	return []apiclient.Frame{
		callFrame(t, 1, 1, "call-old", "exit_plan_mode", planArgs(t, "# 旧方案\n\n旧的正文。")),
		callFrame(t, 2, 3, "call-a", "exit_plan_mode", planArgs(t, "# 新方案甲\n\n甲的正文。")),
		callFrame(t, 3, 3, "call-b", "exit_plan_mode", planArgs(t, "# 新方案乙\n\n乙的正文。")),
	}
}

// TestCardsOnlyNewestTurn 钉住「卡片属于提交它的那一轮」：第 1 轮那份不出现。
func TestCardsOnlyNewestTurn(t *testing.T) {
	p, env, _, canvas := fixture(t, planFrames(t), 360)
	drawCardsOnce(t, p, env, canvas, 520, 240)

	if !hasText(canvas, "新方案甲") || !hasText(canvas, "新方案乙") {
		t.Fatalf("最新一轮两份计划都该画出来；实际文字=%v", textValues(canvas))
	}
	if hasText(canvas, "旧方案") {
		t.Errorf("上一轮的计划不该出现在这一轮尾卡里；实际文字=%v", textValues(canvas))
	}
	// 官方文案三处（OFF:locales.ts:8-10）：标题 / `计划 · Markdown` / `打开`。
	if !hasText(canvas, zhDocument) {
		t.Errorf("缺说明行 %q；实际文字=%v", zhDocument, textValues(canvas))
	}
	if !hasText(canvas, zhAction) {
		t.Errorf("缺动作文字 %q；实际文字=%v", zhAction, textValues(canvas))
	}
}

// TestCardsHiddenWithoutFrames 钉住"空槽不占位"：没有帧基线 / 没有计划时一个字都不画。
func TestCardsHiddenWithoutFrames(t *testing.T) {
	for name, frames := range map[string][]apiclient.Frame{
		"没有帧":    nil,
		"只有别的工具": {callFrame(t, 1, 1, "c1", "read", `{"path":"a.go"}`)},
	} {
		p, env, _, canvas := fixture(t, frames, 360)
		drawCardsOnce(t, p, env, canvas, 520, 240)
		if texts := textValues(canvas); len(texts) != 0 {
			t.Errorf("%s：不该画任何东西，却画了 %v", name, texts)
		}
	}
}

// TestCardClickOpensPreviewTab 钉住点卡片的三件事：切 tab、展开右栏、记住地址。
func TestCardClickOpensPreviewTab(t *testing.T) {
	p, env, act, canvas := fixture(t, planFrames(t), 360)
	id := env.Tree.ID(env.Feat, "plan-card-call-a")
	click(t, p, env, canvas, 520, 240, id, func() {
		drawCardsOnce(t, p, env, canvas, 520, 240)
	})

	if len(act.tabs) != 1 || act.tabs[0] != RightTabPlan {
		t.Fatalf("点击卡片应切到 %q，实际 %v", RightTabPlan, act.tabs)
	}
	if act.toggled != 1 {
		t.Errorf("右栏收起时点卡片应展开它，实际 toggle 次数=%d", act.toggled)
	}
	if want := planAddress("session-1", "call-a"); p.openAddress != want {
		t.Errorf("openAddress = %q，期望 %q", p.openAddress, want)
	}
}

// TestCardClickKeepsOpenRightbar 钉住"右栏已经开着就不再 toggle"。
func TestCardClickKeepsOpenRightbar(t *testing.T) {
	p, env, act, canvas := fixture(t, planFrames(t), 360)
	act.showRightbar = true
	act.install(env)
	id := env.Tree.ID(env.Feat, "plan-card-call-b")
	click(t, p, env, canvas, 520, 240, id, func() {
		drawCardsOnce(t, p, env, canvas, 520, 240)
	})
	if act.toggled != 0 {
		t.Errorf("右栏已展开时不该再 toggle，实际 %d 次", act.toggled)
	}
	if want := planAddress("session-1", "call-b"); p.openAddress != want {
		t.Errorf("openAddress = %q，期望 %q", p.openAddress, want)
	}
}

// TestCardWithoutSelectActionIsInert 钉住"壳没接动作时不编动作"：
// 老壳没有 `SelectRightTab` 时，点卡片只记地址、不报错也不算点了。
func TestCardWithoutSelectActionIsInert(t *testing.T) {
	p, env, _, canvas := fixture(t, planFrames(t), 360)
	env.Act.SelectRightTab = nil
	env.Act.ToggleRightbar = nil
	id := env.Tree.ID(env.Feat, "plan-card-call-a")
	click(t, p, env, canvas, 520, 240, id, func() {
		drawCardsOnce(t, p, env, canvas, 520, 240)
	})
	if want := planAddress("session-1", "call-a"); p.openAddress != want {
		t.Errorf("地址仍要记住（右栏 tab 自己点开时能读到），得到 %q", p.openAddress)
	}
}

// TestPreviewRendersMarkdown 钉住右栏全文真的是 Markdown（标题文字出现在画面上）。
func TestPreviewRendersMarkdown(t *testing.T) {
	p, env, _, canvas := fixture(t, planFrames(t), 360)
	drawPreviewOnce(t, p, env, canvas, 360, 600)

	body := joinedTexts(canvas)
	if !contains(body, "新方案乙") {
		t.Fatalf("右栏应显示本会话最新一份计划的标题；实际文字=%q", body)
	}
	if !contains(body, "乙的正文") {
		t.Errorf("右栏应显示计划正文；实际文字=%q", body)
	}
}

// TestPreviewShowsOpenedPlan 钉住"点了哪张卡就看哪份"，而不是永远看最新一份。
func TestPreviewShowsOpenedPlan(t *testing.T) {
	p, env, _, canvas := fixture(t, planFrames(t), 360)
	p.openAddress = planAddress("session-1", "call-a")
	drawPreviewOnce(t, p, env, canvas, 360, 600)

	body := joinedTexts(canvas)
	if !contains(body, "甲的正文") {
		t.Fatalf("右栏应显示被点开的那份计划（甲）；实际文字=%q", body)
	}
	if contains(body, "乙的正文") {
		t.Errorf("不该同时显示另一份计划；实际文字=%q", body)
	}
}

// TestPreviewStates 钉住三种非正文状态的**官方原文**（OFF:locales.ts:12-15）。
func TestPreviewStates(t *testing.T) {
	cases := map[string]struct {
		frames  []apiclient.Frame
		address string
		want    string
	}{
		"会话里没有计划": {frames: nil, want: zhUnavailable},
		"地址不合约": {
			frames:  planFrames(t),
			address: "dsh-resource://plan/only-one-segment",
			want:    zhInvalidAddress,
		},
		"地址里的 callId 找不到": {
			frames:  planFrames(t),
			address: planAddress("session-1", "call-gone"),
			want:    zhNotFound,
		},
		// 地址属于别的会话：右栏 tab 是全局的，切会话后按"没选过"处理，
		// 退到本会话最新一份（不是"找不到"）。
		"地址属于别的会话（退到本会话最新）": {
			frames:  planFrames(t),
			address: planAddress("session-2", "call-a"),
			want:    "",
		},
	}
	for name, c := range cases {
		p, env, _, canvas := fixture(t, c.frames, 360)
		p.openAddress = c.address
		drawPreviewOnce(t, p, env, canvas, 360, 600)
		if c.want == "" {
			if body := joinedTexts(canvas); !contains(body, "乙的正文") {
				t.Errorf("%s：应退到本会话最新一份计划；实际文字=%q", name, body)
			}
			continue
		}
		if !hasText(canvas, c.want) {
			t.Errorf("%s：应显示 %q；实际文字=%v", name, c.want, textValues(canvas))
		}
	}
}

// TestPreviewLinkOpensExternal 钉住正文里的明确 scheme 链接真的走壳的打开动作。
func TestPreviewLinkOpensExternal(t *testing.T) {
	frames := []apiclient.Frame{
		callFrame(t, 1, 1, "call-link", "exit_plan_mode",
			planArgs(t, "# 带链接的方案\n\n见 [官方文档](https://example.com/doc)。\n")),
	}
	p, env, act, canvas := fixture(t, frames, 360)
	drawPreviewOnce(t, p, env, canvas, 360, 600)
	if p.preview.view.LinkCount() == 0 {
		t.Skip("这一版 viva 没把链接记成可点区，跳过点击断言")
	}
	scope := env.Tree.Feature("plan-preview-links")
	click(t, p, env, canvas, 360, 600, scope.At(0), func() {
		drawPreviewOnce(t, p, env, canvas, 360, 600)
	})
	if len(act.external) != 1 || act.external[0] != "https://example.com/doc" {
		t.Fatalf("链接应交给壳打开，实际 %v", act.external)
	}
}

// TestPreviewTruncatedWhenTallerThanViewport 钉住"正文比视口高时只画视口里那段"：
// 高度 200 的视口里，尾部文字不该被画出来（否则会盖到 tab 条上）。
func TestPreviewTruncatedWhenTallerThanViewport(t *testing.T) {
	long := "# 长方案\n\n"
	for i := 0; i < 80; i++ {
		long += "第 " + itoa(i) + " 段正文。\n\n"
	}
	frames := []apiclient.Frame{callFrame(t, 1, 1, "call-long", "exit_plan_mode", planArgs(t, long))}
	p, env, _, canvas := fixture(t, frames, 360)
	drawPreviewOnce(t, p, env, canvas, 360, 200)
	body := joinedTexts(canvas)
	if contains(body, "第 79 段正文") {
		t.Errorf("视口外的正文不该画出来；实际文字=%q", body)
	}
	if !contains(body, "第 0 段正文") {
		t.Errorf("视口内的正文该画出来；实际文字=%q", body)
	}
}

// contains / itoa 是测试内的小工具（不引 strings 只为两行代码更清楚）。
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// TestCardsWithoutTurnKeepOnlyLast 钉住"帧里没有 turn"时的取舍：
// 只留最后提交的那一份，不把整个会话的计划都堆到当前轮尾部。
func TestCardsWithoutTurnKeepOnlyLast(t *testing.T) {
	frames := []apiclient.Frame{
		callFrame(t, 1, 0, "call-1", "exit_plan_mode", planArgs(t, "# 第一份\n正文一")),
		callFrame(t, 2, 0, "call-2", "exit_plan_mode", planArgs(t, "# 第二份\n正文二")),
	}
	p, env, _, canvas := fixture(t, frames, 360)
	drawCardsOnce(t, p, env, canvas, 520, 240)
	if !hasText(canvas, "第二份") {
		t.Fatalf("应画最后一份；实际文字=%v", textValues(canvas))
	}
	if hasText(canvas, "第一份") {
		t.Errorf("turn 缺失时不该把上一份也画出来；实际文字=%v", textValues(canvas))
	}
}

// 编译期断言：fixture 用的帧类型就是壳给的那个。
var _ = gfx.Rect{}
var _ = pluginapi.Snapshot{}
