//go:build darwin || windows || linux

package dsrightbar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// TestTurnGroupsCollapseAndExpand 走真 Viva 无头驱动：真 Draw + 真 flex 布局 + 真点击。
//
// 验的不是"结构里有个 open 字段"，而是**条目到底有没有画出来**：
// 收起时 tool 名/摘要必须从画面上消失，组头必须还在；点开又回来。
func TestTurnGroupsCollapseAndExpand(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user", Text: "第一轮"},
		{Role: "tool", Tool: "bash", Summary: "echo turn-one"},
		{Role: "tool", Tool: "read", Summary: "turn-one-file"},
		{Role: "assistant", Text: "好了"},
		{Role: "user", Text: "第二轮"},
		{Role: "tool", Tool: "grep", Summary: "turn-two-match"},
	}
	session, _ := openRightbar(t, 500, 600, msgs)

	const head1 = "第 1 轮 · 2 次工具"
	const head2 = "第 2 轮 · 1 次工具"
	for _, want := range []string{head1, head2, "3 次"} {
		if !session.HasText(want) {
			t.Fatalf("缺少文本 %q；texts=%q", want, session.Texts())
		}
	}

	// 默认：只展开最后一组（正在发生的那一轮），更早的收成一行组头。
	if session.HasText("echo turn-one") || session.HasText("turn-one-file") {
		t.Fatalf("非当前轮默认不该展开：%q", session.Texts())
	}
	if !session.HasText("grep") {
		t.Fatalf("当前轮没有展开：%q", session.Texts())
	}

	// 点第 1 轮组头：它自己展开，第 2 轮不受影响。
	if err := session.ClickLabel(head1); err != nil {
		t.Fatalf("点第 1 轮组头：%v", err)
	}
	if !session.HasText("echo turn-one") || !session.HasText("turn-one-file") {
		t.Fatalf("点开后第 1 轮条目没画出来：%q", session.Texts())
	}
	if !session.HasText("grep") {
		t.Fatalf("展开第 1 轮把第 2 轮带没了：%q", session.Texts())
	}

	// 新的一帧（缩放）之后折叠状态还在——状态在插件里，不在这一帧的局部变量里。
	if err := session.Resize(420, 560); err != nil {
		t.Fatal(err)
	}
	if !session.HasText("echo turn-one") {
		t.Fatalf("换帧后折叠状态丢了：%q", session.Texts())
	}

	// 再点一次：收回。
	if err := session.ClickLabel(head1); err != nil {
		t.Fatalf("再点第 1 轮组头：%v", err)
	}
	if session.HasText("echo turn-one") {
		t.Fatalf("再点没有收起：%q", session.Texts())
	}
	// 组头始终在，收起也不让人以为"这一轮只有下面那些"。
	if !session.HasText("第 1 轮 · 2 次工具") {
		t.Fatalf("收起后组头没画：%q", session.Texts())
	}

	// 点当前轮也能收——用户想只看组头时不用先造一条新消息。
	if err := session.ClickLabel(head2); err != nil {
		t.Fatalf("点第 2 轮组头：%v", err)
	}
	if session.HasText("grep") {
		t.Fatalf("点第 2 轮没有收起：%q", session.Texts())
	}
}

// TestCollapseStateIsPerSession：两个会话各自的折叠状态不许互相影响——
// 切来切去时"我收起过哪一轮"是随会话走的，不是全局开关。
func TestCollapseStateIsPerSession(t *testing.T) {
	p := &Plugin{}
	if p.groupOpen("s1", "u1:0", false) {
		t.Fatal("非最后一组默认不该展开（否则分组等于没分组）")
	}
	if !p.groupOpen("s1", "u1:0", true) {
		t.Fatal("最后一组默认该展开")
	}
	p.toggleGroup("s1", "u1:0", true) // 手动收起最后一组
	if p.groupOpen("s1", "u1:0", true) {
		t.Fatal("手动收起没生效")
	}
	if !p.groupOpen("s2", "u1:0", true) {
		t.Fatal("另一个会话被同一个键带走了")
	}
}
