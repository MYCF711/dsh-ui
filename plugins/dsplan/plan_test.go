package dsplan

import (
	"encoding/json"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 纯逻辑：帧解析 / 标题 / 地址 / 取轮。全部对着官方 plan.ts 的同一批规则。

func frame(t *testing.T, seq int, eventType string, data any) apiclient.Frame {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("构造帧失败：%v", err)
	}
	return apiclient.Frame{Seq: seq, Type: eventType, Data: raw}
}

func callFrame(t *testing.T, seq int, turn int64, callID, tool, args string) apiclient.Frame {
	t.Helper()
	return frame(t, seq, "tool/call", map[string]any{
		"turn": turn, "step": 1, "callId": callID, "name": tool, "arguments": args,
	})
}

// planArgs 造一份计划的 arguments 字符串（宿主 `tool/call` 的 `arguments` 是**文本**）。
func planArgs(t *testing.T, markdown string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"plan": markdown})
	if err != nil {
		t.Fatalf("构造参数失败：%v", err)
	}
	return string(raw)
}

// TestSubmittedPlansReadsExitPlanMode 钉住官方读取路径：只有 `exit_plan_mode`
// 调用的 `plan` 参数算计划（OFF:plan.ts:33-48）。
func TestSubmittedPlansReadsExitPlanMode(t *testing.T) {
	frames := []apiclient.Frame{
		frame(t, 1, "user/message", map[string]any{"text": "帮我出个方案"}),
		callFrame(t, 2, 1, "call-read", "read", `{"path":"a.go"}`),
		callFrame(t, 3, 1, "call-plan", "exit_plan_mode", planArgs(t, "# 重构方案\n\n第一步。\n")),
		frame(t, 4, "turn/end", map[string]any{"turn": 1}),
	}
	plans := SubmittedPlans(frames)
	if len(plans) != 1 {
		t.Fatalf("应当只解析出 1 份计划，得到 %d：%+v", len(plans), plans)
	}
	got := plans[0]
	if got.CallID != "call-plan" {
		t.Errorf("CallID = %q，期望 call-plan", got.CallID)
	}
	if got.Turn != 1 || got.Seq != 3 {
		t.Errorf("Turn/Seq = %d/%d，期望 1/3", got.Turn, got.Seq)
	}
	if got.Title != "重构方案" {
		t.Errorf("Title = %q，期望「重构方案」", got.Title)
	}
	if got.Markdown != "# 重构方案\n\n第一步。\n" {
		t.Errorf("Markdown 必须逐字节是原文，得到 %q", got.Markdown)
	}
}

// TestSubmittedPlansRejectsMalformed 钉住官方的四条"不算计划"：
// 没标题 / 参数不是 JSON / plan 不是字符串 / 没有 callId。
func TestSubmittedPlansRejectsMalformed(t *testing.T) {
	cases := map[string][]apiclient.Frame{
		"没有一级标题": {
			callFrame(t, 1, 1, "c1", "exit_plan_mode", planArgs(t, "第一步。\n第二步。")),
		},
		"二级标题不算": {
			callFrame(t, 1, 1, "c1", "exit_plan_mode", planArgs(t, "## 子标题\n正文")),
		},
		"井号后没有空白": {
			callFrame(t, 1, 1, "c1", "exit_plan_mode", planArgs(t, "#标题\n正文")),
		},
		"参数不是 JSON": {
			callFrame(t, 1, 1, "c1", "exit_plan_mode", "{不是 JSON"),
		},
		"plan 不是字符串": {
			frame(t, 1, "tool/call", map[string]any{
				"callId": "c1", "name": "exit_plan_mode", "arguments": `{"plan":42}`,
			}),
		},
		"没有 callId": {
			frame(t, 1, "tool/call", map[string]any{
				"name": "exit_plan_mode", "arguments": planArgs(t, "# 方案"),
			}),
		},
		"别的工具": {
			callFrame(t, 1, 1, "c1", "propose_action", planArgs(t, "# 方案")),
		},
	}
	for name, frames := range cases {
		if plans := SubmittedPlans(frames); len(plans) != 0 {
			t.Errorf("%s：不该算计划，却得到 %+v", name, plans)
		}
	}
}

// TestSubmittedPlansAcceptsPTCDispatch 钉住另外两种事件名（OFF:plan.ts:35-38）。
//
// 本仓宿主不发这两种（没有 PTC 运行时），但解析器与官方同一形状：
// PTC 形态的 `arguments` 已经是对象，callId 在 `subCallId`。
func TestSubmittedPlansAcceptsPTCDispatch(t *testing.T) {
	frames := []apiclient.Frame{
		frame(t, 7, "tool/ptc-dispatch", map[string]any{
			"subCallId": "sub-1", "name": "exit_plan_mode",
			"arguments": map[string]any{"plan": "# PTC 方案\n正文"},
		}),
	}
	plans := SubmittedPlans(frames)
	if len(plans) != 1 {
		t.Fatalf("PTC 派发帧应当解析出计划，得到 %d", len(plans))
	}
	if plans[0].CallID != "sub-1" || plans[0].Title != "PTC 方案" {
		t.Errorf("PTC 解析结果 = %+v，期望 sub-1 / PTC 方案", plans[0])
	}
}

// TestPlanTitleMatchesOfficialRegex 逐条对官方 `/^#\s+(\S[^\r\n]*)/`（trim 之后）。
func TestPlanTitleMatchesOfficialRegex(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
		why  string
	}{
		{in: "# 方案\n正文", want: "方案", ok: true, why: "标准一级标题"},
		{in: "  \n#   带多个空格  \n正文", want: "带多个空格", ok: true, why: "trim + \\s+ + 去行尾空白"},
		{in: "#\t制表符开头", want: "制表符开头", ok: true, why: "\\s+ 含制表符"},
		{in: "# 方案\r\n正文", want: "方案", ok: true, why: "CRLF 行尾"},
		{in: "#", ok: false, why: "只有井号"},
		{in: "#    \n正文", want: "正文", ok: true, why: "空白可以跨行（JS 的 \\s 含换行）"},
		{in: "# ", ok: false, why: "标题正文为空（trim 后只剩井号）"},
		{in: "## 二级", ok: false, why: "两个井号不是一级标题"},
		{in: "正文 # 方案", ok: false, why: "^ 锚定行首"},
	}
	for _, c := range cases {
		got, ok := planTitle(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s：planTitle(%q) = (%q,%v)，期望 (%q,%v)", c.why, c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestPlanAddressRoundTrip 钉住官方 `planAddress` / `parsePlanAddress` 成对的分段与转义
// （OFF:plan.ts:56-87）。转义用 `encodeURIComponent` 的等价集，不跟 `url.PathEscape` 分叉。
func TestPlanAddressRoundTrip(t *testing.T) {
	cases := []struct{ session, callID string }{
		{"s1", "call-1"},
		{"会话/带斜杠", "call:1"},
		{"a b", "x?y#z"},
		{"$&+,;=:@", "!'()*"},
	}
	for _, c := range cases {
		address := planAddress(c.session, c.callID)
		if address == "" {
			t.Fatalf("地址为空：%+v", c)
		}
		gotSession, gotCall, ok := parsePlanAddress(address)
		if !ok || gotSession != c.session || gotCall != c.callID {
			t.Errorf("往返失败：%q → (%q,%q,%v)", address, gotSession, gotCall, ok)
		}
	}
	if address := planAddress("", "c"); address != "" {
		t.Errorf("缺会话时不该造地址，得到 %q", address)
	}
}

// TestParsePlanAddressRejects 钉住不合约的地址（官方同样返回 undefined）。
func TestParsePlanAddressRejects(t *testing.T) {
	bad := []string{
		"",
		"dsh-resource://plan/",
		"dsh-resource://plan/s1",
		"dsh-resource://plan/s1/c1/extra",
		"dsh-resource://plan-review/s1/c1",
		"https://plan/s1/c1",
		"dsh-resource://plan/s1/c1?x=1",
		"dsh-resource://plan//c1",
		"dsh-resource://plan/s1/%zz",
	}
	for _, address := range bad {
		if s, c, ok := parsePlanAddress(address); ok {
			t.Errorf("%q 不该解析成功，得到 (%q,%q)", address, s, c)
		}
	}
}

// TestLatestTurnKeepsOnlyNewest 钉住「Turn 尾卡属于提交它的那一轮」。
func TestLatestTurnKeepsOnlyNewest(t *testing.T) {
	plans := []Document{
		{CallID: "a", Turn: 1, Seq: 3, Title: "旧方案"},
		{CallID: "b", Turn: 3, Seq: 9, Title: "新方案 1"},
		{CallID: "c", Turn: 3, Seq: 12, Title: "新方案 2"},
	}
	got := latestTurn(plans)
	if len(got) != 2 || got[0].CallID != "b" || got[1].CallID != "c" {
		t.Fatalf("取最新一轮失败：%+v", got)
	}
	if latestTurn(nil) != nil {
		t.Error("没有计划时应当返回 nil")
	}
}
