package main

// gy_slots_test.go —— 三个部件（目标条 / 提问卡 / 状态栏）在 `-shell official`
// 的产品形状里**真的被 dispatch**，而且同帧只有一份。
//
// 背景：GR 报告 `tmp/GO8-RC2/reports/GR-official-composer.md` §8 第 2 条——
// 这三个部件还挂在 `main.composer`，官方档位下那个槽不再 dispatch，于是整块
// 消失（不报错、只是空白）。本文件把「迁到官方座位之后确实画出来」变成机器断言：
//
//	① `conversation.composer` 每帧 1 次（三个部件的官方座位都在这个 chain 上；
//	   官方子槽 conversation.input.dock / conversation.composer.dock 今天由
//	   composer 占用方转发，本仓占用方还没转发，见 GR 报告 §8 第 1 条）
//	② `main.composer` 每帧 0 次（同帧只有一个活，不双画）
//	③ 三部件各自的文本在画布上**只出现一次**（真画了，且没画两遍）
//	④ legacy 档位反过来：`main.composer` 1 次，官方座位 0 次

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscomposer"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsgoal"
	"github.com/MYCF711/dsh-ui/plugins/dsquestions"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dssidebar"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// gyQuestionArgs 是一条**待答**的 `ask_user_question` 工具消息参数（宿主 schema）。
const gyQuestionArgs = `{"questions":[{"id":"q1","question":"先做哪个？","header":"确认",` +
	`"options":[{"label":"目标条","description":"dsgoal"},{"label":"提问卡","description":"dsquestions"}]}]}`

// gyAskMsgs 造「一条提问之后没人说话」的消息流（= 正在等回答）。
func gyAskMsgs() []apiclient.Message {
	return []apiclient.Message{
		{Role: "user", Text: "先问我一个问题"},
		{Role: "assistant", Text: "好"},
		{Role: "tool", Tool: "ask_user_question", Args: gyQuestionArgs},
	}
}

// gyFrame 画一帧产品形状，返回画布文本与每个槽的派发次数。
//
// official 为真走官方骨架（`-shell official` 的产品形状：旧 sidebar/正文 +
// 官方 composer 座位）；否则走 dsshell 三栏（`-shell legacy`）。
func gyFrame(t *testing.T, official bool) ([]string, map[pluginapi.Slot]int) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(official)
	kernel := uikernel.New("gy-slots")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)

	goal := &dsgoal.Plugin{}
	questions := &dsquestions.Plugin{}
	status := &dsstatusbar.Plugin{}
	composer := &dscomposer.Plugin{}
	sidebar := &dssidebar.Plugin{}
	conversation := &dsconversation.Plugin{}
	shell := &dsshell.Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, shell, sidebar, conversation, composer, goal, questions, status); err != nil {
		t.Fatalf("装配产品插件集失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)

	data := pluginapi.Snapshot{
		Cur: "s-gy", Status: "已连接",
		Msgs: gyAskMsgs(),
		UsageTotal: pluginapi.Usage{
			InputTokens: 12000, OutputTokens: 345, TotalTokens: 12345,
		},
		ContextTokens: 2000, ContextWindow: 10000,
		Model: "group/deepseek-v4.1-flash-1017", Permission: "allow",
	}
	act := pluginapi.Actions{
		Goal: func() string { return "把三个部件迁到官方槽位" },
		Send: func(string) {}, Cancel: func() {},
	}
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Data: data, Act: act, Tree: tree, Feat: tree.Feature("gy-slots"), Canvas: canvas,
		Bounds:    ui.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Has:       kernel.Has,
	}
	counts := map[pluginapi.Slot]int{}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		if !official {
			gyDrawSlot(f, kernel, pluginapi.SlotShellRoot, env, counts)
			return
		}
		m := &model{kernel: kernel, cur: "s-gy"}
		opts := shellOptionsFor(m, kernel, 1182, 832)
		if !opts.BridgeLegacy {
			t.Fatal("旧 sidebar/正文还占着槽，整列兼容桥必须开着（-shell official 的产品形状）")
		}
		if opts.BridgeLegacyComposer {
			t.Fatal("官方 composer 座位有占用方，composer 不该走兼容桥")
		}
		uikernel.FirstScreen(f, env, opts, func(slot pluginapi.Slot) {
			gyDrawSlot(f, kernel, slot, env, counts)
		})
	})
	texts := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		texts = append(texts, text.Value)
	}
	return texts, counts
}

// gyDrawSlot 与 `uikernel.Kernel.DrawSlot` 同义（含 Children 转发），额外记派发次数。
// 与生产路径的 `drawSlot`（main.go）分开命名，避免遮蔽。
func gyDrawSlot(f ui.C, k *uikernel.Kernel, slot pluginapi.Slot, env *pluginapi.Env, counts map[pluginapi.Slot]int) {
	counts[slot]++
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				gyDrawSlot(f, k, next, env, counts)
			})
		}
	}
}

// countTexts 数一段文本出现几次（精确匹配）。
func countTexts(texts []string, want string) int {
	n := 0
	for _, text := range texts {
		if text == want {
			n++
		}
	}
	return n
}

// hasText 报告画布上有没有以 want 开头的文本（长文本会被省略号截断）。
func hasText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want || strings.HasPrefix(text, want) {
			return true
		}
	}
	return false
}

// TestOfficialTierDispatchesGoalQuestionsStatusbar 是这次的正面判据。
func TestOfficialTierDispatchesGoalQuestionsStatusbar(t *testing.T) {
	texts, counts := gyFrame(t, true)

	// ① 官方座位 1 次；旧槽 0 次（同帧只有一个活）。
	if got := counts[uikernel.OffConvComposer]; got != 1 {
		t.Fatalf("conversation.composer 每帧派发次数 = %d，want 1（counts=%v）", got, counts)
	}
	if got := counts[pluginapi.SlotMainComposer]; got != 0 {
		t.Fatalf("官方档位下 main.composer 被派发 %d 次（双画）：%v", got, counts)
	}

	// ⑤ 三块内容都真画出来了，而且只画一次。
	for _, want := range []string{"目标", "在等你回答", "活动 未知", "总量 12.3k"} {
		if got := countTexts(texts, want); got != 1 {
			t.Fatalf("文本 %q 出现 %d 次，want 1（texts=%v）", want, got, texts)
		}
	}
	// 目标条的目标文本、提问卡的问题文本也要在。
	if !hasText(texts, "把三个部件迁到官方槽位") {
		t.Fatalf("目标文本没画出来：%v", texts)
	}
	if !hasText(texts, "先做哪个？") {
		t.Fatalf("问题文本没画出来：%v", texts)
	}
	// 上下文读数只出现一次：归 composer 的 ContextMeter；状态栏不再画第二枚。
	if got := countTexts(texts, "上下文"); got != 1 {
		t.Fatalf("上下文标签出现 %d 次，want 1（状态栏不该重复画，那归 composer 的 ContextMeter）", got)
	}
}

// TestLegacyTierKeepsOldSlots 是兼容面：legacy 档位仍走旧路径。
func TestLegacyTierKeepsOldSlots(t *testing.T) {
	texts, counts := gyFrame(t, false)
	if got := counts[pluginapi.SlotMainComposer]; got != 1 {
		t.Fatalf("legacy 档位 main.composer 派发次数 = %d，want 1（counts=%v）", got, counts)
	}
	if got := counts[uikernel.OffConvComposer]; got != 0 {
		t.Fatalf("legacy 档位不该派发官方座位 conversation.composer：%d 次", got)
	}
	for _, want := range []string{"目标", "在等你回答", "总量 12.3k"} {
		if got := countTexts(texts, want); got != 1 {
			t.Fatalf("legacy 档位文本 %q 出现 %d 次，want 1（texts=%v）", want, got, texts)
		}
	}
}
