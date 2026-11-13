// approval_test.go —— 审批条的"先算后画"部分。
//
// 这个文件只测**纯函数**：不碰 flex / canvas / 窗口，裸跑就能过。
// 绘制本身（按钮长什么样）靠无头截图验收（tmp/approval-*.png）。
//
// 这里钉的四件事，任何一件破了都意味着"审批是假的"：
//  1. Approval == nil → 不画（不能留一个空框）；
//  2. 工具名 / 参数摘要 / 按钮文案来自快照，不是界面自己编的；
//  3. 回传给壳的 requestId 与 optionId **原样**转发（一个字符都不改）；
//  4. 长参数截断不破坏 UTF-8（中文不能被切成半个字）。
package dscomposer

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// hostFrame 造一份**照宿主 H13 帧形状**填出来的 Approval。
//
// 键名对应关系（见 ../DSH-GO/docs/h13-approval.md）：
//
//	data.requestId        → RequestID
//	data.toolCall.title   → Title（"bash: …"）
//	data.toolCall.kind    → Kind
//	data.toolCall.rawInput→ Args
//	data.options[].optionId/name/kind → Options
func hostFrame() pluginapi.Approval {
	return pluginapi.Approval{
		RequestID: "perm-8f3c-2b",
		Tool:      "bash",
		Title:     "bash: go test ./...",
		Kind:      "execute",
		Args:      `{"command":"go test ./..."}`,
		Detail:    "模型自述（仅供参考）：跑一遍测试确认没有回归",
		Options: []pluginapi.ApprovalOption{
			{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
			{OptionID: "allow-always", Name: "这个会话里以后都允许", Kind: "allow_always"},
			{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
		},
	}
}

func TestApprovalInvisibleWithoutApproval(t *testing.T) {
	// 核心断言：没有待审批就整条不画。留一个空框比不画更糟——
	// 用户会以为"有个东西没加载出来"。
	snap := pluginapi.Snapshot{}
	if approvalVisible(snap) {
		t.Fatal("Approval == nil 时 approvalVisible 必须为 false（整条不画）")
	}
	if plan := planApproval(snap, true); plan.Visible {
		t.Fatalf("Approval == nil 时 plan.Visible 必须为 false，得到 %+v", plan)
	}
	// 有审批时才可见。
	snap.Approval = &pluginapi.Approval{Tool: "bash"}
	if !approvalVisible(snap) {
		t.Fatal("Approval != nil 时 approvalVisible 必须为 true")
	}
}

func TestApprovalBarShowsHostValues(t *testing.T) {
	ap := hostFrame()
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
	if !plan.Visible {
		t.Fatal("有审批时审批条必须可见")
	}
	if plan.Tool != "bash" {
		t.Fatalf("工具名应原样显示 host 的 bash，得到 %q", plan.Tool)
	}
	if plan.Args != `{"command":"go test ./..."}` {
		t.Fatalf("参数摘要应原样显示，得到 %q", plan.Args)
	}
	// 按钮文案用宿主给的 name（"允许这一次" 与 "拒绝" 是两件事，
	// 界面写死一个"允许"就等于替用户把"这一次"和"以后都允许"混了）。
	if plan.Allow.Label != "允许这一次" {
		t.Fatalf("允许按钮文案应取 host 的 name，得到 %q", plan.Allow.Label)
	}
	if plan.Deny.Label != "拒绝" {
		t.Fatalf("拒绝按钮文案应取 host 的 name，得到 %q", plan.Deny.Label)
	}
	if plan.Allow.Option != "allow-once" || plan.Deny.Option != "reject-once" {
		t.Fatalf("按钮应指向 host 给的 optionId，得到 allow=%q deny=%q",
			plan.Allow.Option, plan.Deny.Option)
	}
	if !plan.Allow.Usable || !plan.Deny.Usable {
		t.Fatalf("id 齐全时两个按钮都必须可点，得到 %+v", plan)
	}
	if plan.Note != "" {
		t.Fatalf("正常状态不该有说明文字，得到 %q", plan.Note)
	}
}

func TestApprovalToolFromTitleWhenToolMissing(t *testing.T) {
	// 宿主没给单独的 tool 键时，从 title 的 "bash: " 前缀里取；
	// 取不出来（"写入 <路径>" 这种）就不显示工具名，**不猜**。
	ap := pluginapi.Approval{Title: "bash: ls -la"}
	if got := approvalTool(&ap); got != "bash" {
		t.Fatalf("应从 title 前缀取到 bash，得到 %q", got)
	}
	ap = pluginapi.Approval{Title: "写入 /tmp/a.txt"}
	if got := approvalTool(&ap); got != "" {
		t.Fatalf("取不出工具名时必须返回空串（不猜），得到 %q", got)
	}
}

func TestApproveForwardsOriginalIDs(t *testing.T) {
	ap := hostFrame()
	var gotReq, gotOpt string
	calls := 0
	act := pluginapi.Actions{Approve: func(requestID, optionID string) {
		calls++
		gotReq, gotOpt = requestID, optionID
	}}
	if !decide(&ap, act, true) {
		t.Fatal("点允许应该回传出去")
	}
	if calls != 1 {
		t.Fatalf("回调应被调用 1 次，得到 %d 次", calls)
	}
	if gotReq != "perm-8f3c-2b" {
		t.Fatalf("requestId 必须原样回传，得到 %q", gotReq)
	}
	if gotOpt != "allow-once" {
		t.Fatalf("optionId 必须是宿主给的 allow 项，得到 %q", gotOpt)
	}
}

func TestRejectForwardsOriginalIDs(t *testing.T) {
	ap := hostFrame()
	var gotReq, gotOpt string
	calls := 0
	act := pluginapi.Actions{Approve: func(requestID, optionID string) {
		calls++
		gotReq, gotOpt = requestID, optionID
	}}
	if !decide(&ap, act, false) {
		t.Fatal("点拒绝应该回传出去")
	}
	if calls != 1 {
		t.Fatalf("回调应被调用 1 次，得到 %d 次", calls)
	}
	if gotReq != "perm-8f3c-2b" || gotOpt != "reject-once" {
		t.Fatalf("拒绝必须原样回传 (%q,%q)，得到 (%q,%q)",
			"perm-8f3c-2b", "reject-once", gotReq, gotOpt)
	}
}

func TestWeirdIDsAreNotRewritten(t *testing.T) {
	// 防"界面自己造 id"：宿主给什么就回什么，哪怕它长得不像我们的约定。
	ap := pluginapi.Approval{
		RequestID: "REQ/42 带空格",
		Options: []pluginapi.ApprovalOption{
			{OptionID: "YES", Kind: "allow_once"},
			{OptionID: "NO", Kind: "reject_once"},
		},
	}
	var gotReq, gotOpt string
	act := pluginapi.Actions{Approve: func(requestID, optionID string) { gotReq, gotOpt = requestID, optionID }}
	decide(&ap, act, true)
	if gotReq != "REQ/42 带空格" || gotOpt != "YES" {
		t.Fatalf("id 被改写了：得到 (%q,%q)", gotReq, gotOpt)
	}
	decide(&ap, act, false)
	if gotOpt != "NO" {
		t.Fatalf("拒绝的 optionId 被改写了：得到 %q", gotOpt)
	}
}

func TestNoOptionMeansNoCallback(t *testing.T) {
	// 宿主没给 options（或给了没有 kind 的）：按钮画成禁用，点了什么也不发。
	// 发一个自己编的 optionId 会让宿主按 400 拒掉，用户以为批了其实没有。
	ap := pluginapi.Approval{RequestID: "perm-1"}
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
	if plan.Allow.Usable || plan.Deny.Usable {
		t.Fatalf("没有可选项时按钮必须禁用，得到 %+v", plan)
	}
	if plan.Note == "" {
		t.Fatal("按钮点不动时必须说清原因（Note 不能为空）")
	}
	called := false
	act := pluginapi.Actions{Approve: func(string, string) { called = true }}
	if decide(&ap, act, true) || decide(&ap, act, false) {
		t.Fatal("没有 optionId 时 decide 必须返回 false")
	}
	if called {
		t.Fatal("没有 optionId 时不许调用 Approve")
	}

	// 缺 requestId 也一样：这回传不出去，按钮禁用。
	ap2 := hostFrame()
	ap2.RequestID = ""
	plan2 := planApproval(pluginapi.Snapshot{Approval: &ap2}, true)
	if plan2.Allow.Usable || plan2.Deny.Usable {
		t.Fatalf("缺 requestId 时按钮必须禁用，得到 %+v", plan2)
	}
	// 但旧壳（没接 Approve）不需要 id：退回 Allow/Deny，仍然点得动。
	plan3 := planApproval(pluginapi.Snapshot{Approval: &ap2}, false)
	if !plan3.Allow.Usable || !plan3.Deny.Usable {
		t.Fatal("壳没接 Approve 时应退回旧的 Allow/Deny，按钮可点")
	}
}

func TestLegacyFallbackCallsAllowDeny(t *testing.T) {
	// 壳还没接 Approve（mockd 那条老路径）时，按钮仍然要能清掉气泡。
	ap := pluginapi.Approval{Tool: "bash"}
	allowCalled, denyCalled := false, false
	act := pluginapi.Actions{
		Allow: func() { allowCalled = true },
		Deny:  func() { denyCalled = true },
	}
	if !decide(&ap, act, true) || !allowCalled {
		t.Fatal("没有 Approve 时点允许应退回 Act.Allow")
	}
	if !decide(&ap, act, false) || !denyCalled {
		t.Fatal("没有 Approve 时点拒绝应退回 Act.Deny")
	}
}

func TestClipRunesKeepsUTF8(t *testing.T) {
	// 按字节截断会把"审批"切成 E5 AE，屏幕上出现 �。这里断言：
	// 截断结果仍是合法 UTF-8、没有替换符、长度正确。
	in := "这是一串中文参数用来验证截断不会切坏多字节字符"
	got := clipRunes(in, 5)
	if !utf8.ValidString(got) {
		t.Fatalf("截断后不是合法 UTF-8: %q", got)
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("截断后出现替换符（切坏了字）: %q", got)
	}
	if want := "这是一串中…"; got != want {
		t.Fatalf("截断结果应为 %q，得到 %q", want, got)
	}
	if n := utf8.RuneCountInString(got); n != 6 { // 5 个字符 + 省略号
		t.Fatalf("截断后应是 6 个字符，得到 %d 个: %q", n, got)
	}
	// 不超长时原样返回（不加省略号）。
	if got := clipRunes("短参数", 10); got != "短参数" {
		t.Fatalf("没超长不该改，得到 %q", got)
	}
}

func TestLongArgsStayOneLine(t *testing.T) {
	// 长参数：压成一行 + 省略号，且不破坏 UTF-8。
	var b strings.Builder
	b.WriteString("{\n  \"command\": \"")
	for i := 0; i < 200; i++ {
		b.WriteString("中文")
	}
	b.WriteString("\"\n}")
	ap := pluginapi.Approval{Args: b.String()}
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
	if strings.ContainsAny(plan.Args, "\n\r\t") {
		t.Fatalf("参数摘要必须是一行，得到 %q", plan.Args)
	}
	if !strings.HasSuffix(plan.Args, "…") {
		t.Fatalf("超长参数必须以省略号结尾，得到 %q", plan.Args)
	}
	if !utf8.ValidString(plan.Args) || strings.ContainsRune(plan.Args, utf8.RuneError) {
		t.Fatalf("截断破坏了 UTF-8: %q", plan.Args)
	}
	if n := utf8.RuneCountInString(plan.Args); n != approvalArgsMax+1 {
		t.Fatalf("应保留 %d 个字符 + 省略号，得到 %d 个", approvalArgsMax, n)
	}
}

func TestLegacyCmdStillShown(t *testing.T) {
	// 旧路径只填 Cmd（mockd / 本地假审批）：参数行回退到它，不能空着。
	ap := pluginapi.Approval{Tool: "bash", Cmd: "git status --short"}
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
	if plan.Args != "git status --short" {
		t.Fatalf("Args 为空时应回退到 Cmd，得到 %q", plan.Args)
	}
}

func TestHandleKeyTakesEnterOnlyWhenVisible(t *testing.T) {
	// viva 的分发是"先 OnKey，没被处理才交给聚焦控件"——
	// 这里返回 true 就等于从输入框手里抢走 Enter。
	// 所以：没有审批时必须返回 false，输入框的 Enter 发送照旧。
	p := &Plugin{}

	entered := 0
	p.lastAct = pluginapi.Actions{Approve: func(string, string) { entered++ }}
	p.lastData = pluginapi.Snapshot{}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter}) {
		t.Fatal("没有审批时 Enter 必须交给输入框（不能抢）")
	}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("没有审批时 Esc 不该被吞")
	}
	if entered != 0 {
		t.Fatal("没有审批时不该有任何回传")
	}

	ap := hostFrame()
	p.lastData = pluginapi.Snapshot{Approval: &ap}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter}) {
		t.Fatal("审批条可见时 Enter 应被接管（= 允许）")
	}
	if entered != 1 {
		t.Fatalf("Enter 应回传一次，得到 %d 次", entered)
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("审批条可见时 Esc 应被接管（= 拒绝）")
	}
	if entered != 2 {
		t.Fatalf("Esc 应回传一次，共 2 次，得到 %d 次", entered)
	}
	// 带修饰键不抢：Ctrl+Enter 这类留给别的插件/输入框。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter, Modifiers: gfx.ModifierControl}) {
		t.Fatal("带修饰键的 Enter 不该被审批条吃掉")
	}
}

// TestAllowAlwaysGetsItsOwnButton 钉住「宿主给 3 个选项时界面不能只画 2 个」。
//
// 真宿主 policy=ask 推的是 allow-once / allow-always / reject-once 三项
// （见 2026-11-03 17:47 的真机日志 options=3）。如果只画一个允许按钮，
// 用户就永远点不到"以后都允许"——连续几次工具调用要手点好几次。
func TestAllowAlwaysGetsItsOwnButton(t *testing.T) {
	ap := hostFrame()
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)

	if plan.Allow.Option != "allow-once" {
		t.Fatalf("主允许按钮应是 allow_once 那一项，得到 %q", plan.Allow.Option)
	}
	if plan.AllowAlways.Option != "allow-always" {
		t.Fatalf("第二个允许项（以后都允许）必须有独立按钮，得到 %q", plan.AllowAlways.Option)
	}
	var gotOpt string
	act := pluginapi.Actions{Approve: func(requestID, optionID string) { gotOpt = optionID }}
	if !decideOption(&ap, act, plan.AllowAlways.Option) {
		t.Fatal("点 allow_always 应该能回传")
	}
	if gotOpt != "allow-always" {
		t.Fatalf("点的是以后都允许，回传的必须是它自己，得到 %q", gotOpt)
	}
}

// TestNoSecondAllowWhenHostGivesOne 反向：宿主只给一个 allow 时**不许**多画一个按钮。
func TestNoSecondAllowWhenHostGivesOne(t *testing.T) {
	ap := hostFrame()
	ap.Options = []pluginapi.ApprovalOption{
		{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
		{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
	}
	plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
	if plan.AllowAlways.Option != "" {
		t.Fatalf("宿主只给一个允许项时不该有第二个按钮，得到 %q", plan.AllowAlways.Option)
	}
}
