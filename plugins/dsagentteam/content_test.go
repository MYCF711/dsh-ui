// content_test.go —— 四块内容的**行为语义**（不是"能画出来"就行）。
//
// 覆盖官方写死的那些规则：
//
//	进度文案三段 + en-space·en-space 分隔 + 0 段省略
//	圆点四值映射（warning 只给"pending 且 !ready"）
//	开关**不做乐观更新**；关闭且已有模型请求 → 弹确认而不是直接关
//	确认框 saving 期间双按钮禁用；失败错误**一直可见**到重试/取消
//	展开互斥（单值 expandedTaskId）
//	回执 summary 三条分支 + 可展开条件
//	远端写只有 `agentTeams.setEnabled`（经 internal/apiclient）
package dsagentteam

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestProgressLabelRules 钉住官方 progressLabel（TeamBoard.tsx:29-38）。
func TestProgressLabelRules(t *testing.T) {
	cases := []struct {
		name  string
		tasks []TeamTask
		want  string
	}{
		{"三段齐全", []TeamTask{
			{Status: StatusCompleted}, {Status: StatusInProgress},
			{Status: StatusPending}, {Status: StatusPending},
		}, "1 已完成" + progressSeparator + "1 进行中" + progressSeparator + "2 待处理"},
		{"0 段省略（只有完成）", []TeamTask{{Status: StatusCompleted}}, "1 已完成"},
		{"0 段省略（只有待处理）", []TeamTask{{Status: StatusPending}}, "1 待处理"},
		{"两段组合", []TeamTask{
			{Status: StatusInProgress}, {Status: StatusPending},
		}, "1 进行中" + progressSeparator + "1 待处理"},
		{"空列表", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := progressLabel(tc.tasks); got != tc.want {
				t.Fatalf("progressLabel = %q，应为 %q", got, tc.want)
			}
		})
	}
	// 分隔符必须是 en-space + · + en-space（U+2002），不是普通空格或中点。
	if !strings.Contains(progressSeparator, "\u2002·\u2002") {
		t.Fatalf("分隔符应为 en-space·en-space，得到 %q", progressSeparator)
	}
}

// TestOwnerPriority：`ownerName ?? ownerId ?? t('unowned')`（TeamBoard.tsx:46）。
func TestOwnerPriority(t *testing.T) {
	if got := (TeamTask{OwnerName: "阿青", OwnerID: "id-1"}).ownerLabel(); got != "阿青" {
		t.Fatalf("有 ownerName 时优先用它，得到 %q", got)
	}
	if got := (TeamTask{OwnerID: "id-1"}).ownerLabel(); got != "id-1" {
		t.Fatalf("没有 ownerName 时用 ownerId，得到 %q", got)
	}
	if got := (TeamTask{}).ownerLabel(); got != zhUnowned {
		t.Fatalf("都没有时用 unowned，得到 %q", got)
	}
}

// TestStatusAndDotMapping：状态映射与圆点四值（warning 只给"pending 且 !ready"）。
func TestStatusAndDotMapping(t *testing.T) {
	p := newTestPlugin(nil)
	theme := p.theme
	cases := []struct {
		name string
		task TeamTask
		want interface{}
	}{
		{"pending 未阻塞 → idle（第三级文字色）", TeamTask{Status: StatusPending, Ready: true}, theme.LabelTertiary},
		{"pending 被阻塞 → warning", TeamTask{Status: StatusPending, Ready: false}, theme.Warn},
		{"in_progress → ongoing（品牌色）", TeamTask{Status: StatusInProgress, Ready: true}, theme.BrandPrimary},
		{"completed → done（成功色）", TeamTask{Status: StatusCompleted, Ready: true}, theme.Success},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dotColor(theme, tc.task); got != tc.want {
				t.Fatalf("dotColor = %+v，应为 %+v", got, tc.want)
			}
		})
	}
	if statusLabel(StatusPending) != zhStatusPending ||
		statusLabel(StatusInProgress) != zhStatusInProgress ||
		statusLabel(StatusCompleted) != zhStatusCompleted {
		t.Fatal("状态文案对不上官方 zh 字典")
	}
	if !(TeamTask{Status: StatusPending, Ready: false}).blocked() {
		t.Fatal("pending 且 !ready 必须算被依赖阻塞")
	}
	if (TeamTask{Status: StatusCompleted, Ready: false}).blocked() {
		t.Fatal("只有 pending 才可能被阻塞")
	}
}

// TestSelectIsNotOptimistic：点开关后 enabled 不变、pending=true；远端成功才变。
func TestSelectIsNotOptimistic(t *testing.T) {
	returned := make(chan struct{})
	block := make(chan struct{})
	remote := &fakeRemote{block: block, returned: returned}
	p := newTestPlugin(remote)
	view := teamView()
	view.Projection.Enabled = false
	p.team.SetView(view)

	p.team.Select(true)
	if !p.team.Pending() {
		t.Fatal("点了之后 pending 必须为真")
	}
	if p.team.View().Projection.Enabled {
		t.Fatal("**不许乐观更新**：远端还没成功，enabled 不能变")
	}
	close(block)
	<-returned
	waitUntil(t, "远端成功后 pending 清掉", func() bool { return !p.team.Pending() })
	if !p.team.View().Projection.Enabled {
		t.Fatal("远端成功后 enabled 应变 true")
	}
	calls := remote.callList()
	if len(calls) != 1 || calls[0].sessionID != "sess-root" || !calls[0].enabled {
		t.Fatalf("应当只调一次 setEnabled(root,true)，得到 %+v", calls)
	}
}

// TestSelectFailureKeepsValueAndReports：失败保留旧值，并按官方 mode.failed 给人话。
func TestSelectFailureKeepsValueAndReports(t *testing.T) {
	remote := &fakeRemote{err: errors.New("gateway timeout")}
	p := newTestPlugin(remote)
	view := teamView()
	view.Projection.Enabled = false
	p.team.SetView(view)

	p.team.Select(true)
	waitUntil(t, "失败已落", func() bool { return !p.team.Pending() })
	if p.team.View().Projection.Enabled {
		t.Fatal("失败后 enabled 必须保持旧值")
	}
	failure := p.team.Failure()
	if !strings.Contains(failure, "无法确认") || !strings.Contains(failure, "gateway timeout") {
		t.Fatalf("失败文案应为 mode.failed 模板 + 原因，得到 %q", failure)
	}
}

// TestSelectOffWithModelLastUsedAsksConfirmation：关闭且会话已有模型请求 → 弹确认，不发请求。
func TestSelectOffWithModelLastUsedAsksConfirmation(t *testing.T) {
	remote := &fakeRemote{}
	p := newTestPlugin(remote)
	view := teamView()
	view.ModelLastUsed = true
	p.team.SetView(view)

	p.team.Select(false)
	if req := p.team.DisableRequest(); req == nil || req.SessionID != "sess-root" {
		t.Fatalf("应弹停用确认（会话 sess-root），得到 %+v", req)
	}
	if calls := remote.callList(); len(calls) != 0 {
		t.Fatalf("请求确认期间不该直接发 setEnabled，实际 %+v", calls)
	}
}

// TestSelectOffWithoutModelLastUsedCallsRemote：没有模型请求时直接关。
func TestSelectOffWithoutModelLastUsedCallsRemote(t *testing.T) {
	remote := &fakeRemote{}
	p := newTestPlugin(remote)
	view := teamView()
	view.ModelLastUsed = false
	p.team.SetView(view)

	p.team.Select(false)
	waitUntil(t, "远端被调用", func() bool { return len(remote.callList()) == 1 })
	if remote.callList()[0].enabled {
		t.Fatal("关闭应发 enabled=false")
	}
	if p.team.DisableRequest() != nil {
		t.Fatal("没有模型请求时不该弹确认")
	}
}

// TestConfirmSavingAndFailureSticky：saving 期间不能重复提交；失败保留错误并可重试。
func TestConfirmSavingAndFailureSticky(t *testing.T) {
	returned := make(chan struct{})
	block := make(chan struct{})
	remote := &fakeRemote{block: block, returned: returned, err: errors.New("persist failed")}
	p := newTestPlugin(remote)
	p.team.SetView(teamView())
	p.team.RequestDisable("sess-root")

	p.team.ConfirmDisable()
	if got := p.team.Confirm(); got.Kind != ConfirmSaving {
		t.Fatalf("提交后应进入 saving，得到 %+v", got)
	}
	waitUntil(t, "第一次请求已发出", func() bool { return len(remote.callList()) == 1 })
	p.team.ConfirmDisable() // saving 期间重复点：必须被忽略
	p.team.ConfirmDisable()
	if calls := remote.callList(); len(calls) != 1 {
		t.Fatalf("saving 期间不该重复发请求，实际 %d 次", len(calls))
	}
	close(block)
	<-returned
	waitUntil(t, "失败落定", func() bool { return p.team.Confirm().Kind == ConfirmFailed })
	got := p.team.Confirm()
	if !strings.Contains(got.Message, "persist failed") {
		t.Fatalf("失败文案应带原因，得到 %+v", got)
	}
	if p.team.DisableRequest() == nil {
		t.Fatal("失败后确认框必须还在（一直可见到重试或取消）")
	}

	// 取消必须在 session 匹配时才清空（官方 mount.ts:51-53）。
	p.team.CancelDisable("another-session")
	if p.team.DisableRequest() == nil {
		t.Fatal("session 不匹配时不许清空别人的确认框")
	}
	p.team.CancelDisable("sess-root")
	if p.team.DisableRequest() != nil {
		t.Fatal("session 匹配时应关掉确认框")
	}
}

// TestConfirmSuccessCloses：成功关框并把 enabled 改 false。
func TestConfirmSuccessCloses(t *testing.T) {
	remote := &fakeRemote{}
	p := newTestPlugin(remote)
	p.team.SetView(teamView())
	p.team.RequestDisable("sess-root")

	p.team.ConfirmDisable()
	waitUntil(t, "确认完成", func() bool { return p.team.DisableRequest() == nil })
	if p.team.View().Projection.Enabled {
		t.Fatal("停用成功后 enabled 应为 false")
	}
}

// TestBoardExpansionIsExclusive：展开互斥（单值 expandedTaskId）。
func TestBoardExpansionIsExclusive(t *testing.T) {
	p := newTestPlugin(nil)
	p.team.SetView(teamView())

	p.team.ToggleExpanded("t1")
	if got := p.team.ExpandedTaskID(); got != "t1" {
		t.Fatalf("应展开 t1，得到 %q", got)
	}
	p.team.ToggleExpanded("t2")
	if got := p.team.ExpandedTaskID(); got != "t2" {
		t.Fatalf("开 t2 必须关 t1，得到 %q", got)
	}
	p.team.ToggleExpanded("t2")
	if got := p.team.ExpandedTaskID(); got != "" {
		t.Fatalf("再点一次应收起，得到 %q", got)
	}
}

// TestBoardRendersExpandedDetails：展开后画描述与负责人（官方 details 区）。
func TestBoardRendersExpandedDetails(t *testing.T) {
	p := newTestPlugin(nil)
	view := teamView()
	view.Projection.Tasks = []TeamTask{{
		ID: "t1", Subject: "写实现", Description: "把槽位接上", Status: StatusInProgress,
		OwnerID: "dev", Ready: true,
	}}
	p.team.SetView(view)
	p.team.ToggleExpanded("t1")

	canvas := renderSlot(t, p, SlotSubagentFooter)
	for _, want := range []string{"写实现", "把槽位接上", zhOwner + "：", "dev"} {
		if !hasText(canvas, want) {
			t.Fatalf("展开后缺 %q；文本=%v", want, textValues(canvas))
		}
	}
}

// TestBoardFailureRow：投影带 failure 时画错误行（任务为空也画）。
func TestBoardFailureRow(t *testing.T) {
	p := newTestPlugin(nil)
	view := teamView()
	view.Projection.Tasks = nil
	view.Projection.Failure = "任务记录损坏"
	p.team.SetView(view)

	canvas := renderSlot(t, p, SlotSubagentFooter)
	if !anyTextContains(canvas, "任务记录损坏") {
		t.Fatalf("应画出投影 failure；文本=%v", textValues(canvas))
	}
}

// TestCommandSummaryRules：三条分支（无 outcome / 成功 / 失败取第一行非空）。
func TestCommandSummaryRules(t *testing.T) {
	cases := []struct {
		name string
		node *CommandNode
		want string
	}{
		{"无结果 = 执行中", &CommandNode{Command: "team", Outcome: nil}, zhCommandRunning},
		{"成功且有文本", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success", Text: "已开启"}}, "已开启"},
		{"成功但文本为空", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success"}}, zhCommandDone},
		{"失败取第一行非空", &CommandNode{Command: "team", Outcome: &CommandOutcome{
			Kind: "error", Text: "\n  boom: 第一行\n第二行",
		}}, "执行失败：boom: 第一行"},
		{"失败但无原因", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "error", Text: "  \n "}}, zhCommandFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commandSummary(tc.node); got != tc.want {
				t.Fatalf("summary = %q，应为 %q", got, tc.want)
			}
		})
	}
}

// TestCommandExpandableRules：可展开 = body 非空 且（失败 或 含换行）。
func TestCommandExpandableRules(t *testing.T) {
	cases := []struct {
		name string
		node *CommandNode
		want bool
	}{
		{"运行中不可展开", &CommandNode{Command: "team"}, false},
		{"成功单行不可展开", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success", Text: "ok"}}, false},
		{"成功多行可展开", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success", Text: "a\nb"}}, true},
		{"失败单行也可展开", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "error", Text: "boom"}}, true},
		{"空 body 不可展开", &CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "error", Text: ""}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := commandExpandable(tc.node); got != tc.want {
				t.Fatalf("expandable = %v，应为 %v", got, tc.want)
			}
		})
	}
}

// TestCommandRowExpandedShowsBody：展开后正文（pre）出现，且失败时用错误色。
func TestCommandRowExpandedShowsBody(t *testing.T) {
	p := newTestPlugin(nil)
	p.team.SetCommand(&CommandNode{Command: CommandKey, Outcome: &CommandOutcome{
		Kind: "error", Text: "第一行\n第二行 detail",
	}})
	// 收起：只有摘要。
	collapsed := renderSlot(t, p, SlotChatCommandView)
	if !hasText(collapsed, "执行失败：第一行") {
		t.Fatalf("收起时摘要应为 failedReason；文本=%v", textValues(collapsed))
	}
	// 展开：正文出现。
	p.team.ToggleCommandExpanded()
	expanded := renderSlot(t, p, SlotChatCommandView)
	if !hasText(expanded, "第一行\n第二行 detail") {
		t.Fatalf("展开后应画出完整 body；文本=%v", textValues(expanded))
	}
}

// TestRemoteIsApiclientMethods：远端写真的打到 `agentTeams.setEnabled`，
// 且 wire 参数是 `agentId`（官方 source=lookup 的 wire 名）——经 internal/apiclient。
func TestRemoteIsApiclientMethods(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := apiclient.New(srv.URL)
	p := newTestPlugin(client)
	view := teamView()
	view.Projection.Enabled = false
	p.team.SetView(view)

	p.team.Select(true)
	waitUntil(t, "请求完成", func() bool { return !p.team.Pending() })
	waitUntil(t, "假宿主收到请求", func() bool { return gotPath != "" })
	if gotPath != "/api/agentTeams.setEnabled" {
		t.Fatalf("路径应为 /api/agentTeams.setEnabled，得到 %q", gotPath)
	}
	if body["agentId"] != "sess-root" {
		t.Fatalf("wire 名必须 agentId，得到 %v", body)
	}
	if enabled, ok := body["enabled"].(bool); !ok || !enabled {
		t.Fatalf("enabled 必须 true，得到 %v", body["enabled"])
	}
}

// TestRemoteNilIsHonest：没有远端时点了要**报错**，不能把开关装成成功。
func TestRemoteNilIsHonest(t *testing.T) {
	p := newTestPlugin(nil)
	view := teamView()
	p.team.SetView(view)

	p.team.Select(false)
	waitUntil(t, "失败落定", func() bool { return !p.team.Pending() })
	if p.team.View().Projection.Enabled == false {
		t.Fatal("未接线时不许改本地 enabled")
	}
	if !strings.Contains(p.team.Failure(), "没有") && !strings.Contains(p.team.Failure(), "未接线") {
		t.Fatalf("未接线时失败文案应说清缺什么，得到 %q", p.team.Failure())
	}
}
