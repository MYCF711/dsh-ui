package dsconversation

// official_transcript_nodes_test.go —— 20 种节点类型表的机器断言。
//
// 这一组测试回答的是「官方主区的 20 种节点，我们到底覆盖了几种」，
// 而不是「代码看起来对不对」：
//
//	① 数量：必须是 20 种（少一种 = 主区漏画一种节点）；
//	② 顺序与结构树 §5 总表逐行一致（换个顺序就说明有人重排过表）；
//	③ 每种必须有数据锚点，且除了官方真值 `submitted-plan` 之外都必须有渲染器；
//	④ `submitted-plan` 恒 hidden、无渲染器、无 DOM——官方真值，不是漏实现。

import (
	"strings"
	"testing"
)

// officialKindOrder 是结构树 §5 总表的 20 行（**逐字**照抄，含连字符）。
var officialKindOrder = []string{
	"user",
	"steering",
	"context",
	"turn-trigger",
	"command-input",
	"assistant-step",
	"tool-call",
	"command",
	"manual-compaction",
	"compaction",
	"model-retry",
	"turn-error",
	"turn-max-tokens",
	"turn-process",
	"turn-tail",
	"system-prompt",
	"unknown",
	"question-reply",
	"submitted-plan",
	"workflow-run",
}

func TestOfficialChatNodesAreExactlyTwentyInOrder(t *testing.T) {
	nodes := OfficialChatNodes()
	if len(nodes) != 20 {
		t.Fatalf("官方节点类型必须是 20 种，实际 %d 种", len(nodes))
	}
	for i, want := range officialKindOrder {
		if got := string(nodes[i].Kind); got != want {
			t.Errorf("第 %d 种 kind = %q，官方是 %q（顺序必须与结构树 §5 总表一致）", i+1, got, want)
		}
	}
}

func TestOfficialChatNodesHaveUniqueKinds(t *testing.T) {
	seen := map[ChatNodeKind]bool{}
	for _, spec := range OfficialChatNodes() {
		if seen[spec.Kind] {
			t.Errorf("kind %q 重复出现（ChatNodeDataMap 的键必须唯一）", spec.Kind)
		}
		seen[spec.Kind] = true
	}
	if len(seen) != 20 {
		t.Errorf("去重后 %d 种，应为 20", len(seen))
	}
}

func TestOfficialChatNodesAnchors(t *testing.T) {
	for _, spec := range OfficialChatNodes() {
		if spec.DataAnchor == "" {
			t.Errorf("%s 缺少数据声明锚点（ChatNodeDataMap 成员位置）", spec.Kind)
		}
		if !strings.Contains(spec.DataAnchor, ":") {
			t.Errorf("%s 的数据锚点 %q 没有行号（必须是 rc.2 src:line）", spec.Kind, spec.DataAnchor)
		}
		if spec.Anchor == "" {
			t.Errorf("%s 缺少结构树条目锚点", spec.Kind)
		}
		if !strings.HasPrefix(spec.Anchor, "§5.") {
			t.Errorf("%s 的锚点 %q 应指向结构树 §5.N 小节", spec.Kind, spec.Anchor)
		}
	}
}

func TestOfficialChatNodeVisibility(t *testing.T) {
	for _, spec := range OfficialChatNodes() {
		switch spec.Visibility {
		case ChatVisibilityVisible, ChatVisibilityHidden, ChatVisibilityConditional:
			// ok
		default:
			t.Errorf("%s 的可见性 %q 不在三态里（visible/hidden/conditional）", spec.Kind, spec.Visibility)
		}
		if spec.Visibility == ChatVisibilityConditional && spec.Condition == "" {
			t.Errorf("%s 是条件可见，必须写清 Condition 判据", spec.Kind)
		}
	}
}

// TestSubmittedPlanHasNoRenderer 钉住官方真值：`submitted-plan` **没有行渲染器**。
//
// 这不是漏实现：它只作 Turn 尾卡（`conversation.chat.turnTail`）的数据源，
// 为它补一张 transcript 卡 = 做了个官方不存在的界面。
func TestSubmittedPlanHasNoRenderer(t *testing.T) {
	spec, ok := ChatNodeSpecFor(ChatNodeSubmittedPlan)
	if !ok {
		t.Fatal("submitted-plan 必须出现在 20 种节点表里（它是 ChatNodeDataMap 的键）")
	}
	if spec.RendererAnchor != "" {
		t.Errorf("submitted-plan 官方**没有**渲染器注册，这里却写了锚点 %q", spec.RendererAnchor)
	}
	if spec.DOMRoot != "" {
		t.Errorf("submitted-plan 不产生 DOM，这里却写了 %q", spec.DOMRoot)
	}
	if spec.Visibility != ChatVisibilityHidden {
		t.Errorf("submitted-plan 恒 visibility:hidden，实际 %q", spec.Visibility)
	}
	if ChatNodeRenders(ChatNodeSubmittedPlan) {
		t.Error("ChatNodeRenders(submitted-plan) 必须是 false（官方没有行渲染器）")
	}
}

func TestOtherNineteenKindsRender(t *testing.T) {
	renderable := 0
	for _, spec := range OfficialChatNodes() {
		if spec.Kind == ChatNodeSubmittedPlan {
			continue
		}
		if !ChatNodeRenders(spec.Kind) {
			t.Errorf("%s 官方有渲染器注册，但表里 RendererAnchor 为空", spec.Kind)
		}
		if spec.DOMRoot == "" {
			t.Errorf("%s 官方有 DOM 根，但表里为空", spec.Kind)
		}
		renderable++
	}
	if renderable != 19 {
		t.Fatalf("可渲染 kind 应为 19 种，实际 %d 种", renderable)
	}
}

func TestChatNodeSpecForUnknownKind(t *testing.T) {
	if _, ok := ChatNodeSpecFor(ChatNodeKind("not-a-kind")); ok {
		t.Error("表外 kind 必须 ok=false（调用方当未知处理，不许编默认值）")
	}
	if ChatNodeRenders(ChatNodeKind("not-a-kind")) {
		t.Error("表外 kind 不得被当成可渲染")
	}
}

func TestChatNodeKindsMatchesTable(t *testing.T) {
	kinds := ChatNodeKinds()
	if len(kinds) != 20 {
		t.Fatalf("ChatNodeKinds 返回 %d 个，应为 20", len(kinds))
	}
	for i, want := range officialKindOrder {
		if string(kinds[i]) != want {
			t.Errorf("ChatNodeKinds[%d] = %q，应为 %q", i, kinds[i], want)
		}
	}
}

// TestOfficialTranscriptSlots 钉住三处官方挂载点 + flow/node 两枚槽。
func TestOfficialTranscriptSlots(t *testing.T) {
	cases := map[string]string{
		"content":   SlotConversationContent,
		"session":   SlotConversationSession,
		"view":      SlotConversationView,
		"chat.flow": SlotChatFlow,
		"chat.node": SlotChatNode,
	}
	want := map[string]string{
		"content":   "conversation.content",
		"session":   "conversation.session",
		"view":      "conversation.view",
		"chat.flow": "conversation.chat.flow",
		"chat.node": "conversation.chat.node",
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s 槽 = %q，官方字面量是 %q", name, got, want[name])
		}
	}
}

// TestChatFlowSlotIsDisplayContents 钉住 flow 槽是 `display:contents`。
//
// 给它加任何盒模型（flex/block）都会让行距选择器失效：
// 官方必须用 `:global([data-slot="conversation.chat.flow"])` 显式穿透。
func TestChatFlowSlotIsDisplayContents(t *testing.T) {
	if ChatFlowSlotDisplay != "contents" {
		t.Fatalf("flow 槽 display = %q，官方是 contents", ChatFlowSlotDisplay)
	}
}
