//go:build darwin || windows || linux

package dsrightbar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// toolMsg 造一条工具消息；host 非空时当作宿主给的 turnId。
func toolMsg(name, summary, hostTurn string) apiclient.Message {
	m := apiclient.Message{Role: "tool", Tool: name, Summary: summary}
	if hostTurn != "" {
		m.TurnID = hostTurn
	}
	return m
}

// TestGroupTurnsByUserMessages 钉住 load 投影这条主路径：
// session.load 的 Message 里**没有** turnId/turn（宿主 transport.Message 就没这两个字段），
// 所以首屏的轮次边界只能靠用户消息切。切错了会让一次 turn 的工具散成两组。
func TestGroupTurnsByUserMessages(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user", Text: "第一轮"},
		toolMsg("bash", "echo one", ""),
		toolMsg("read", "file.go", ""),
		{Role: "assistant", Text: "好了"},
		{Role: "user", Text: "第二轮"},
		toolMsg("grep", "match", ""),
	}
	groups := groupTurns(msgs)
	if len(groups) != 2 {
		t.Fatalf("用户消息切出 %d 组，期望 2：%+v", len(groups), groups)
	}
	if got := len(groups[0].calls); got != 2 {
		t.Fatalf("第 1 轮工具数 = %d，期望 2", got)
	}
	if got := len(groups[1].calls); got != 1 {
		t.Fatalf("第 2 轮工具数 = %d，期望 1", got)
	}
	if groups[0].turn != 1 || groups[1].turn != 2 {
		t.Fatalf("轮次序号 = %d/%d，期望 1/2", groups[0].turn, groups[1].turn)
	}
	if groups[0].key == groups[1].key {
		t.Fatalf("两组身份相同 %q：折叠状态会互相串", groups[0].key)
	}
}

// TestGroupTurnsSplitsOnHostTurn 覆盖流式帧这条路径：没有新的用户消息，
// 但宿主给了不同的 turn 身份时仍然要切开——否则两轮的工具会被并成一组，
// 组头的「K 次工具」就把两轮算成一轮了。
func TestGroupTurnsSplitsOnHostTurn(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user", Text: "第一轮"},
		toolMsg("bash", "a", "turn-a"),
		toolMsg("bash", "b", "turn-a"),
		toolMsg("bash", "c", "turn-b"),
	}
	groups := groupTurns(msgs)
	if len(groups) != 2 {
		t.Fatalf("按宿主 turn 切出 %d 组，期望 2：%+v", len(groups), groups)
	}
	if len(groups[0].calls) != 2 || len(groups[1].calls) != 1 {
		t.Fatalf("分组条数 = %d/%d，期望 2/1", len(groups[0].calls), len(groups[1].calls))
	}
	if groups[1].hostKey != "id:turn-b" {
		t.Fatalf("第二组身份 = %q，期望 id:turn-b", groups[1].hostKey)
	}
}

// TestGroupTurnsAdoptsLaterHostTurn：load 进来的工具（没有 turn 身份）后面跟着
// 同一轮的流式工具（有身份）时，**不能**切出一组新的——那会在一次 turn 里
// 画出两个组头。身份只能被这一组收下。
func TestGroupTurnsAdoptsLaterHostTurn(t *testing.T) {
	msgs := []apiclient.Message{
		{Role: "user", Text: "同一条用户消息"},
		toolMsg("bash", "from load", ""),
		toolMsg("bash", "from stream", "turn-x"),
		toolMsg("bash", "still turn-x", "turn-x"),
	}
	groups := groupTurns(msgs)
	if len(groups) != 1 {
		t.Fatalf("同一轮被切成 %d 组：%+v", len(groups), groups)
	}
	if len(groups[0].calls) != 3 {
		t.Fatalf("组内条数 = %d，期望 3", len(groups[0].calls))
	}
}

// TestGroupTurnsBeforeFirstUserMessage：load 窗口从某一轮中间开始（第一条就是工具）时，
// 这些工具得有地方去，不能丢；后面的用户消息照常开新组。
func TestGroupTurnsBeforeFirstUserMessage(t *testing.T) {
	msgs := []apiclient.Message{
		toolMsg("bash", "orphan", ""),
		{Role: "user", Text: "下一轮"},
		toolMsg("read", "after", ""),
	}
	groups := groupTurns(msgs)
	if len(groups) != 2 || len(groups[0].calls) != 1 || len(groups[1].calls) != 1 {
		t.Fatalf("孤儿工具没有单独成组：%+v", groups)
	}
}

// TestClipGroupsDropsWholeGroups 钉住截断语义：条数上限是全局的（防几千个节点），
// 但**整组丢**——组头的条数是这一轮的真实条数，切一半就会报小。
func TestClipGroupsDropsWholeGroups(t *testing.T) {
	groups := []turnGroup{
		{turn: 1, key: "a", calls: make([]apiclient.Message, 30)},
		{turn: 2, key: "b", calls: make([]apiclient.Message, 30)},
		{turn: 3, key: "c", calls: make([]apiclient.Message, 30)},
	}
	kept, hidden := clipGroups(groups, 80)
	if hidden != 30 {
		t.Fatalf("隐藏条数 = %d，期望 30", hidden)
	}
	if len(kept) != 2 {
		t.Fatalf("留下 %d 组，期望 2", len(kept))
	}
	// 序号不重排：被丢掉的是第 1 轮，留下来的还叫第 2、3 轮。
	if kept[0].turn != 2 || kept[1].turn != 3 {
		t.Fatalf("截断后序号 = %d/%d，期望 2/3", kept[0].turn, kept[1].turn)
	}
	if kept[0].skip != 0 || kept[1].skip != 0 {
		t.Fatalf("整组丢时不该有组内截断：%d/%d", kept[0].skip, kept[1].skip)
	}
}

// TestClipGroupsCutsInsideHugeGroup：一组自己就超过上限（一轮里几百次工具）时，
// 只能组内切；切掉的条数要记下来，展开时写明「前 N 次未显示」。
func TestClipGroupsCutsInsideHugeGroup(t *testing.T) {
	groups := []turnGroup{{turn: 1, key: "a", calls: make([]apiclient.Message, 100)}}
	kept, hidden := clipGroups(groups, 80)
	if len(kept) != 1 {
		t.Fatalf("留下 %d 组，期望 1", len(kept))
	}
	if kept[0].skip != 20 {
		t.Fatalf("组内跳过 = %d，期望 20", kept[0].skip)
	}
	if hidden != 20 {
		t.Fatalf("隐藏条数 = %d，期望 20", hidden)
	}
	// 组头的条数说的是真实值（100），不是画出来的 80。
	if got := len(kept[0].calls); got != 100 {
		t.Fatalf("组头会写成 %d 次，期望 100", got)
	}
}

// TestClipGroupsNoCutWhenUnderLimit：正常长度下不许动任何组、不许报隐藏。
func TestClipGroupsNoCutWhenUnderLimit(t *testing.T) {
	groups := []turnGroup{
		{turn: 1, key: "a", calls: make([]apiclient.Message, 3)},
		{turn: 2, key: "b", calls: make([]apiclient.Message, 4)},
	}
	kept, hidden := clipGroups(groups, 80)
	if hidden != 0 || len(kept) != 2 {
		t.Fatalf("未超上限却截断：hidden=%d kept=%d", hidden, len(kept))
	}
}
