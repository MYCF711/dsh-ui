package uikernel

// official_containers_test.go —— 两个父容器的机器断言（本单的验收核心）。
//
// 每个容器四态（本单任务书的四态）各一条：
//
//	容器 C 模型菜单   无 provider（空目录） / 未配置（unknown） / 数据缺失（取失败） / 正常
//	容器 D 命令节点   无命令 / 未认领（兜底卡） / 数据缺失（无名字节点） / 正常（认领→dispatch）
//
// 三条硬断言：
//
//	① 该隐藏时**一个像素都不画**（不是画个空壳）；
//	② 该画时容器里**真的调了 dispatch**（这就是"父容器"与"只注册槽位"的区别）；
//	③ 几何照官方（min240/max420/pad4/行高 34/trigger 28/命令卡 separator 2×2）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// renderContainer 在一个真实画布上跑一次 draw，把 frame 交给 body。
//
// 用 flex.View.Draw 而不是手搓 Frame：popup/absolute 这些位置规则只有在真正的
// 布局+绘制里才成立，手搓的 Frame 量不到"菜单贴在 trigger 上方 8px"。
func renderContainer(t *testing.T, width, height float32, env *pluginapi.Env, body func(f ui.C)) *gfx.Canvas {
	t.Helper()
	if env == nil {
		env = containerEnv(width, height)
	}
	canvas := env.Canvas
	pageTheme := dscore.VivaTheme(dscore.Dark())
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, body)
	return canvas
}

// containerEnv 造一个够用的 Env（真实 Canvas + 测量函数）。
func containerEnv(width, height float32) *pluginapi.Env {
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	return &pluginapi.Env{
		Tree:   tree,
		Feat:   tree.Feature("official-containers-test"),
		Canvas: canvas,
		Bounds: ui.Rect{Width: width, Height: height},
		Has:    func(pluginapi.Slot) bool { return false },
	}
}

// menuEnv 把 Env 补上这一帧的主题（容器从 opts.Theme 读，测试统一给）。
func menuTheme() *pluginapi.Theme { return dscore.Dark() }

// mirrorRect 把容器算出来的 trigger 矩形镜像成"菜单应该出现的矩形"（上方 8px、右缘对齐）。
func menuRectFor(anchor ui.Rect, bounds ui.Rect) ui.Rect {
	width := ModelMenuWidth(bounds.Width)
	height := ModelMenuMaxHeightFor(bounds.Height)
	// 菜单紧贴 anchor 上方 8px，右缘对齐（官方 bottom:calc(anchor(top)+8px); right:anchor(right)）。
	return ui.Rect{X: anchor.X + anchor.Width - width, Y: anchor.Y - ModelMenuAnchorGap - height, Width: width, Height: height}
}

// ── 容器 C：模型菜单 ────────────────────────────────────────────────

// TestModelMenuFourStates 是容器 C 的四态表。
func TestModelMenuFourStates(t *testing.T) {
	bounds := ui.Rect{Width: 1182, Height: 832}
	anchor := ui.Rect{X: 600, Y: 700, Width: 200, Height: ModelTriggerHeight}

	cases := []struct {
		name     string
		opts     ModelMenuOptions
		draws    bool
		dispatch int
	}{
		{
			// 未配置：宿主没有这个能力 / 没注入 ui.apiclient / 501。
			name:     "未配置：provider 目录未知（Known=false）→ 整段隐藏",
			opts:     ModelMenuOptions{Open: true, Known: false},
			draws:    false,
			dispatch: 0,
		},
		{
			// 无 provider：宿主真的没有可配置 provider（known + 空列表）。
			name:     "无 provider：空目录 → 整段隐藏（不画空菜单）",
			opts:     ModelMenuOptions{Open: true, Known: true, Providers: nil},
			draws:    false,
			dispatch: 0,
		},
		{
			// 菜单没展开：连 trigger 都不该"伪装"成有菜单。
			name:     "未展开：Open=false → 不画",
			opts:     ModelMenuOptions{Open: false, Known: true, Providers: []ModelProviderOption{{Provider: "deepseek", DisplayName: "DeepSeek"}}},
			draws:    false,
			dispatch: 0,
		},
		{
			// 数据缺失：取目录失败 → 画官方 .error 行（不编 provider），子槽仍然 dispatch。
			name: "数据缺失：取目录失败 → error 行 + 子槽照 dispatch",
			opts: ModelMenuOptions{
				Open: true, Known: true, FetchFailure: "llm.listConfigurableProviders 未接线（501）",
			},
			draws:    true,
			dispatch: 1,
		},
		{
			// 正常：真目录 → provider 行 + 子槽 dispatch。
			name: "正常：两个 provider → 行 + 子槽 dispatch",
			opts: ModelMenuOptions{
				Open: true, Known: true,
				Providers: []ModelProviderOption{
					{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
					{Provider: "openai", DisplayName: "OpenAI-compatible", Failure: "没有可用端点"},
				},
			},
			draws:    true,
			dispatch: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var dispatched []pluginapi.Slot
			opts := tc.opts
			opts.Theme = menuTheme()
			opts.Anchor = anchor
			opts.Bounds = bounds
			opts.Dispatch = func(slot pluginapi.Slot) {
				calls++
				dispatched = append(dispatched, slot)
			}

			env := containerEnv(bounds.Width, bounds.Height)
			canvas := renderContainer(t, bounds.Width, bounds.Height, env, func(f ui.C) {
				DrawModelMenu(f, env, opts)
			})

			if !tc.draws {
				if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
					t.Fatalf("该整段隐藏，却画了 %d 矩形 / %d 文本",
						len(canvas.Rectangles()), len(canvas.Texts()))
				}
				if calls != 0 {
					t.Fatalf("隐藏态不该 dispatch 子槽，却调了 %d 次", calls)
				}
				return
			}
			if len(canvas.Rectangles()) == 0 {
				t.Fatal("该画菜单面，却一个矩形都没有")
			}
			if calls != tc.dispatch {
				t.Fatalf("父容器必须 dispatch 子槽：期望 %d 次，实际 %d 次", tc.dispatch, calls)
			}
			for _, slot := range dispatched {
				if slot != OffInputModelOptions {
					t.Fatalf("dispatch 的槽位应是 %q，得到 %q", OffInputModelOptions, slot)
				}
			}
		})
	}
}

// TestModelMenuDispatchesOfficialOptionsSlot 钉住子槽名与官方逐字一致。
//
// 官方 `ui-model-selection/src/client/index.ts:177` 声明的 children 就是这三个字面量；
// 写错一个字符，GB 注册的槽位就永远收不到 dispatch。
func TestModelMenuDispatchesOfficialOptionsSlot(t *testing.T) {
	if string(OffInputModelOptions) != "conversation.input.model.options" {
		t.Fatalf("子槽名应为官方字面量，得到 %q", OffInputModelOptions)
	}
	if string(OffInputModelLabel) != "conversation.input.model.label" {
		t.Fatalf("label 槽名应为官方字面量，得到 %q", OffInputModelLabel)
	}
	// 父槽必须是既有 `conversation.input.model`（槽表里 OffInputModel 是同名字面量）。
	if string(OffInputModel) != "conversation.input.model" {
		t.Fatalf("父槽名应为官方字面量，得到 %q", OffInputModel)
	}
	spec, ok := LookupOfficialSlot(OffInputModelOptions)
	if !ok {
		// 槽表由插件在 Register 时登记；这里没登记不算失败，但要确认父槽对得上。
		t.Logf("槽表里还没有 %q（由 dsagentteam.RegisterOfficialSlots 登记）", OffInputModelOptions)
		return
	}
	if spec.Parent != OffInputModel {
		t.Fatalf("槽表里 %q 的父槽应是 %q，实际 %q", OffInputModelOptions, OffInputModel, spec.Parent)
	}
}

// TestModelMenuGeometryMatchesOfficial 钉住官方 `.menu` 的三条几何。
//
// 官方 `ModelSelect.module.css:107-135`：
// min-width:min(240px,100vw-24px) / max-width:min(420px,100vw-24px) /
// max-height:min(360px,100dvh-96px) / padding:4px。
func TestModelMenuGeometryMatchesOfficial(t *testing.T) {
	cases := []struct {
		name                 string
		boundsW, boundsH     float32
		wantW, wantMaxHeight float32
	}{
		{"宽屏 1920：夹到官方 420 上限", 1920, 1032, 420, 360},
		{"窄屏 400：被视口减 24 夹住", 400, 800, 376, 360},
		{"极窄 200：可用宽度不足 240，取可用", 200, 800, 176, 360},
		{"矮窗 300：max-height 被视口减 96 夹住", 1200, 300, 420, 204},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ModelMenuWidth(tc.boundsW); got != tc.wantW {
				t.Fatalf("菜单宽度应为 %v，得到 %v", tc.wantW, got)
			}
			if got := ModelMenuMaxHeightFor(tc.boundsH); got != tc.wantMaxHeight {
				t.Fatalf("菜单最大高度应为 %v，得到 %v", tc.wantMaxHeight, got)
			}
		})
	}
	if ModelMenuPad != 4 {
		t.Fatalf("官方 `.menu{padding:4px}`，得到 %v", ModelMenuPad)
	}
	if ModelTriggerHeight != 28 {
		t.Fatalf("官方 `.trigger{height:28px}`，得到 %v", ModelTriggerHeight)
	}
	if ModelMenuAnchorGap != 8 {
		t.Fatalf("官方 `bottom:calc(anchor(top)+8px)`，得到 %v", ModelMenuAnchorGap)
	}
}

// TestModelMenuIsDrawnAboveAnchor 断言菜单真的画在 trigger **上方**（官方开法）。
//
// 判据：菜单所有矩形的底边 <= trigger 顶边（允许 0px：菜单紧贴）。
func TestModelMenuIsDrawnAboveAnchor(t *testing.T) {
	bounds := ui.Rect{Width: 1182, Height: 832}
	anchor := ui.Rect{X: 560, Y: 640, Width: 240, Height: ModelTriggerHeight}
	env := containerEnv(bounds.Width, bounds.Height)
	opts := ModelMenuOptions{
		Theme: menuTheme(), Anchor: anchor, Bounds: bounds, Open: true, Known: true,
		Providers: []ModelProviderOption{{Provider: "deepseek", DisplayName: "DeepSeek"}},
		Dispatch:  func(pluginapi.Slot) {},
	}
	canvas := renderContainer(t, bounds.Width, bounds.Height, env, func(f ui.C) {
		DrawModelMenu(f, env, opts)
	})
	rects := canvas.Rectangles()
	if len(rects) == 0 {
		t.Fatal("没画出菜单")
	}
	for _, rect := range rects {
		if rect.Bounds.Y+rect.Bounds.Height > anchor.Y+0.5 {
			t.Fatalf("菜单矩形 %+v 压到了 trigger（顶边 %v）：官方是开在 trigger 上方", rect.Bounds, anchor.Y)
		}
	}
}

// ── 容器 D：chat 命令节点 ───────────────────────────────────────────

func commandNodesEnv() *pluginapi.Env { return containerEnv(1182, 832) }

// TestCommandNodeFourStates 是容器 D 的四态表。
func TestCommandNodeFourStates(t *testing.T) {
	cases := []struct {
		name     string
		nodes    []CommandNode
		claims   map[string]bool
		wantDraw int
		// wantDispatch 是子槽被 dispatch 的次数（认领了才有）。
		wantDispatch int
		// wantText 是应当出现在画布上的文本（兜底卡/无）。
		wantText string
		// wantPixels 是"容器自己画了像素"。认领状态下容器只画一个空的 .callRow
		// 外壳（内容由子槽的注册方画），所以像素在 dispatch 那边，不在这里断言。
		wantPixels bool
	}{
		{
			name:     "无命令：没有命令节点 → 一个像素都不画",
			nodes:    nil,
			wantDraw: 0,
		},
		{
			name:         "未认领：team 没有注册者 → 画官方兜底卡，不 dispatch",
			nodes:        []CommandNode{{Command: "team"}},
			claims:       map[string]bool{},
			wantDraw:     1,
			wantDispatch: 0,
			wantText:     zhChatCommandRunning,
			wantPixels:   true,
		},
		{
			// 数据缺失：没有命令名的节点（官方 entryKey 是 name ?? ''，key 为空认领不到）。
			name:     "数据缺失：命令名为空 → 跳过（不画错行）",
			nodes:    []CommandNode{{Command: ""}},
			claims:   map[string]bool{"team": true},
			wantDraw: 0,
		},
		{
			name:         "正常：team 有认领 → 只画 callRow + dispatch 子槽",
			nodes:        []CommandNode{{Command: "team", Outcome: &CommandOutcome{Kind: "success", Text: "已开启"}}},
			claims:       map[string]bool{"team": true},
			wantDraw:     1,
			wantDispatch: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatched := 0
			var slots []pluginapi.Slot
			env := commandNodesEnv()
			opts := CommandNodeOptions{
				Theme: menuTheme(), Width: 748, Bounds: env.Bounds, Nodes: tc.nodes,
				Claims: func(name string) bool { return tc.claims[name] },
				Toggle: func(string) {},
				Dispatch: func(slot pluginapi.Slot) {
					dispatched++
					slots = append(slots, slot)
				},
			}
			drawn := 0
			canvas := renderContainer(t, 1182, 832, env, func(f ui.C) {
				drawn = DrawCommandNodes(f, env, opts)
			})
			if drawn != tc.wantDraw {
				t.Fatalf("应画 %d 个命令节点，实际 %d 个", tc.wantDraw, drawn)
			}
			if dispatched != tc.wantDispatch {
				t.Fatalf("子槽 dispatch 次数应为 %d，实际 %d", tc.wantDispatch, dispatched)
			}
			for _, slot := range slots {
				if slot != OffChatCommandView {
					t.Fatalf("dispatch 的槽位应是 %q，得到 %q", OffChatCommandView, slot)
				}
			}
			if tc.wantDraw == 0 {
				if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
					t.Fatalf("该不画，却画了 %d 矩形 / %d 文本",
						len(canvas.Rectangles()), len(canvas.Texts()))
				}
				return
			}
			if tc.wantPixels && len(canvas.Rectangles()) == 0 && len(canvas.Texts()) == 0 {
				t.Fatal("该画命令节点，却什么都没有")
			}
			if tc.wantText != "" && !containerHasText(canvas, tc.wantText) {
				t.Fatalf("兜底卡缺文案 %q；文本=%v", tc.wantText, containerTexts(canvas))
			}
		})
	}
}

// TestGenericCommandCardSummaryRules 钉住官方兜底卡的三条 summary 分支与展开条件。
//
// 官方 `GenericCommandCard.tsx:20-46`：
//
//	outcome===null → command.running；有 text → text；
//	否则 error→command.failed / success→command.done
//	body 只在 text 含换行时可展开
func TestGenericCommandCardSummaryRules(t *testing.T) {
	cases := []struct {
		name string
		node CommandNode
		want string
	}{
		{"无 outcome：执行中…", CommandNode{Command: "team"}, zhChatCommandRunning},
		{"success + text：照服务端原文", CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success", Text: "已开启团队"}}, "已开启团队"},
		{"success 无 text：已完成", CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "success"}}, zhChatCommandDone},
		{"error 无 text：指令失败", CommandNode{Command: "team", Outcome: &CommandOutcome{Kind: "error"}}, zhChatCommandFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := containerEnv(800, 600)
			canvas := renderContainer(t, 800, 600, env, func(f ui.C) {
				DrawCommandNodes(f, env, CommandNodeOptions{
					Theme: menuTheme(), Width: 700, Bounds: env.Bounds,
					Nodes:    []CommandNode{tc.node},
					Claims:   func(string) bool { return false },
					Dispatch: func(pluginapi.Slot) {},
				})
			})
			if !containerHasText(canvas, tc.want) {
				t.Fatalf("summary 应为 %q；文本=%v", tc.want, containerTexts(canvas))
			}
		})
	}
}

// TestGenericCommandCardExpandableOnlyWithMultilineBody 钉住"只有多行才可展开"。
func TestGenericCommandCardExpandableOnlyWithMultilineBody(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantBody bool
	}{
		{"单行不算 body", "一行就完了", false},
		{"多行才是 body", "第一行\n第二行", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := CommandNode{
				Command: "team",
				Outcome: &CommandOutcome{Kind: "error", Text: tc.text},
				// 展开态由调用方给：这里直接给 true，看容器会不会因为"不可展开"而忽略它。
				Expanded: true,
			}
			env := containerEnv(800, 600)
			canvas := renderContainer(t, 800, 600, env, func(f ui.C) {
				DrawCommandNodes(f, env, CommandNodeOptions{
					Theme: menuTheme(), Width: 700, Bounds: env.Bounds,
					Nodes:    []CommandNode{node},
					Claims:   func(string) bool { return false },
					Toggle:   func(string) {},
					Dispatch: func(pluginapi.Slot) {},
				})
			})
			// 展开态画出来时 `<pre>` 里第二行文本才会出现在画布上。
			hasSecond := containerHasText(canvas, "第二行")
			if hasSecond != tc.wantBody {
				t.Fatalf("多行 body 判据错了：期望画正文=%v，实际=%v；文本=%v",
					tc.wantBody, hasSecond, containerTexts(canvas))
			}
		})
	}
}

// TestCommandNodeSeparatorMatchesOfficial 钉住 separator 2×2（官方
// `GenericCommandCard.module.css:14-20`）。
func TestCommandNodeSeparatorMatchesOfficial(t *testing.T) {
	if CommandNodeSeparatorSize != 2 {
		t.Fatalf("官方 separator 是 2×2，得到 %v", CommandNodeSeparatorSize)
	}
	if CommandNodeSeparatorGap != 8 {
		t.Fatalf("官方 separator margin 是 0 8px，得到 %v", CommandNodeSeparatorGap)
	}
	if CommandNodeSummaryFont != 13 || CommandNodeSummaryLine != 24 {
		t.Fatalf("官方 summary 是 13/24，得到 %v/%v", CommandNodeSummaryFont, CommandNodeSummaryLine)
	}
	if CommandNodeBodyMaxHeight != 260 || CommandNodeBodyPadV != 12 || CommandNodeBodyPadH != 16 {
		t.Fatalf("官方 body 是 max-height 260 / padding 12·16，得到 %v / %v·%v",
			CommandNodeBodyMaxHeight, CommandNodeBodyPadV, CommandNodeBodyPadH)
	}
}

// ── 小工具 ─────────────────────────────────────────────────────────

func containerTexts(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func containerHasText(canvas *gfx.Canvas, want string) bool {
	for _, text := range containerTexts(canvas) {
		if text == want || contains(text, want) {
			return true
		}
	}
	return false
}

// contains 是 strings.Contains 的本地别名（本文件不引 strings 只为一处用）。
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
