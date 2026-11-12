package dsquestions

// official_slot_test.go —— 提问卡**官方座位**的 dispatch / 不双画 / 可见性两态。
//
// 判据：
//
//	① 注册在官方 chain 槽 `conversation.composer`（ui-user-questions index.ts:383），
//	   order 3（在目标宿主 4 与输入卡片 10 之前 = 最上面）；
//	② 有待答问题 → 官方座位画出来（eyebrow「在等你回答」+ 问题 + 选项，各一次）；
//	③ 没有待答问题 → 整块不画（不再挂着输入框上方）；
//	④ 官方档位下 legacy 座位（main.composer）一次都不画；legacy 档位反过来。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// drawSeat 模拟官方骨架 dispatch 一个槽（含 Children 转发）并计数。
// 语义与 `uikernel.Kernel.drawSlot` 一致，区别只是顺手记下每个槽派发几次。
func drawSeat(f *flex.Frame, k *uikernel.Kernel, env *pluginapi.Env, slot pluginapi.Slot, counts map[pluginapi.Slot]int) {
	counts[slot]++
	for _, painter := range k.Painters(slot) {
		if painter.Draw != nil {
			painter.Draw(f, env)
		}
		if painter.Children != nil {
			painter.Children(f, env, func(next pluginapi.Slot) {
				drawSeat(f, k, env, next, counts)
			})
		}
	}
}

// questionFrame 画一帧并返回画布、文本、每槽派发次数。
//
// tierOfficial 是壳档位；drawOfficial 是这一帧要 dispatch 哪个座位
// （两个参数分开，才能验「官方档位下 legacy 座位一声不响」）。
func questionFrame(t *testing.T, tierOfficial, drawOfficial bool, msgs []apiclient.Message) (*gfx.Canvas, []string, map[pluginapi.Slot]int) {
	t.Helper()
	tier := uikernel.NewShellTier()
	tier.Set(tierOfficial)
	kernel := uikernel.New("questions-official-seat")
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size * 0.55 })
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s", Msgs: msgs},
		Act:  pluginapi.Actions{Send: func(string) {}},
		Tree: tree, Feat: tree.Feature("questions-official"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1182, Height: 832}, MainWidth: 902,
		Geometry: pluginapi.Geometry{MainWidth: 902}, Has: kernel.Has,
	}
	counts := map[pluginapi.Slot]int{}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f *flex.Frame) {
		if drawOfficial {
			drawSeat(f, kernel, env, uikernel.OffConvComposer, counts)
			return
		}
		drawSeat(f, kernel, env, pluginapi.SlotMainComposer, counts)
	})
	texts := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		texts = append(texts, text.Value)
	}
	return canvas, texts, counts
}

func countText(texts []string, want string) int {
	n := 0
	for _, text := range texts {
		if text == want {
			n++
		}
	}
	return n
}

// ① 官方槽声明：conversation.composer（chain），order 3。
func TestQuestionsRegistersOfficialComposerChain(t *testing.T) {
	p := &Plugin{}
	byID := map[string]pluginapi.Painter{}
	for _, painter := range p.Panels() {
		byID[painter.ID] = painter
	}
	official, ok := byID["questions-bar-official"]
	if !ok {
		t.Fatal("缺官方座位 painter（questions-bar-official）")
	}
	if official.Slot != uikernel.OffConvComposer {
		t.Fatalf("官方槽 = %q，want conversation.composer（ui-user-questions index.ts:383）", official.Slot)
	}
	if official.Order != questionsOrder || questionsOrder != 3 {
		t.Fatalf("官方 order = %d，want 3（问题在最上面）", official.Order)
	}
	legacy, ok := byID["questions-bar"]
	if !ok || legacy.Slot != pluginapi.SlotMainComposer {
		t.Fatalf("legacy 座位必须还在 main.composer：%+v", legacy)
	}
}

// ② 有待答问题 → 官方座位真的画出来（且每段文本只出现一次 = 不双画）。
func TestQuestionsOfficialSeatDrawsPendingQuestionOnce(t *testing.T) {
	canvas, texts, counts := questionFrame(t, true, true, []apiclient.Message{toolMsg(twoOpts)})
	if counts[uikernel.OffConvComposer] != 1 {
		t.Fatalf("conversation.composer 派发次数 = %d，want 1", counts[uikernel.OffConvComposer])
	}
	for _, want := range []string{questionEyebrowText, "选哪个？", "A", "B"} {
		if got := countText(texts, want); got != 1 {
			t.Fatalf("文本 %q 出现 %d 次，want 1（texts=%v）", want, got, texts)
		}
	}
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("官方提问卡没画出任何矩形（卡片/选项行）")
	}
}

// ③ 没有待答问题 → 整块不画。
func TestQuestionsOfficialSeatHiddenWithoutPending(t *testing.T) {
	for name, msgs := range map[string][]apiclient.Message{
		"空消息流":     nil,
		"已经答过了":    {toolMsg(twoOpts), {Role: "user"}, {Role: "assistant"}},
		"没有提问只有工具": {{Role: "tool", Tool: "bash", Args: `{"command":"ls"}`}},
	} {
		canvas, texts, counts := questionFrame(t, true, true, msgs)
		if counts[uikernel.OffConvComposer] != 1 {
			t.Fatalf("%s：座位仍应被派发一次（拍发是壳的事）：%v", name, counts)
		}
		if len(canvas.Rectangles()) != 0 || len(texts) != 0 {
			t.Fatalf("%s：没有待答问题时不该画任何东西，rects=%d texts=%v", name, len(canvas.Rectangles()), texts)
		}
	}
}

// ④ 两个座位互斥：官方档位 legacy 不画；legacy 档位官方不画、legacy 画。
func TestQuestionsSeatsAreMutuallyExclusive(t *testing.T) {
	msgs := []apiclient.Message{toolMsg(twoOpts)}

	// 官方档位 + 画 legacy 座位 → 一片空白。
	canvas, texts, _ := questionFrame(t, true, false, msgs)
	if len(texts) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("官方档位下 legacy 座位还画了东西（双画）：%v", texts)
	}

	// legacy 档位 + 画官方座位 → 一片空白。
	canvas, texts, _ = questionFrame(t, false, true, msgs)
	if len(texts) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("legacy 档位下官方座位还画了东西：%v", texts)
	}

	// legacy 档位 + 画 legacy 座位 → 有内容（旧路径没坏）。
	canvas, texts, _ = questionFrame(t, false, false, msgs)
	if countText(texts, questionEyebrowText) != 1 {
		t.Fatalf("legacy 档位下 legacy 座位没画出问题条：%v", texts)
	}
}
