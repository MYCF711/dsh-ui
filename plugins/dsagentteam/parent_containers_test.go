// parent_containers_test.go —— 两个父容器的**装配级**证据（本单验收核心）。
//
// 与 uikernel 侧的纯结构测试分工：
//
//	uikernel/official_containers_test.go  画法/几何/隐藏态（不知道插件存在）
//	本文件                                **真 kernel + 真插件 + 真 drawSlot** 端到端
//
// 关键判据只有一条：容器 `Children` 里那次 `drawSlot` 真的把内核里的槽位画出来了。
// 所以这里用 `uikernel.Kernel.DrawSlot` 当 dispatch，而不是一个记账用的假函数——
// 假函数只能证明"调了"，证明不了"GB 注册的那块画真的画出来了"。
package dsagentteam

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 装配框架 ────────────────────────────────────────────────────────

// containerHarness 是"真内核 + 真插件 + 一块真画布"的测试台。
type containerHarness struct {
	kernel *uikernel.Kernel
	plugin *Plugin
	store  *Store
	env    *pluginapi.Env
	canvas *gfx.Canvas
	// rendered 记录本帧真的调过哪些槽位（drawSlot 的记账，供断言用）。
	rendered []pluginapi.Slot
}

func newContainerHarness(t *testing.T) *containerHarness {
	t.Helper()
	theme := dscore.Dark()
	kernel := uikernel.New("agent-team-containers-test")
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	store, ok := kernel.Get(ServiceKey).(*Store)
	if !ok {
		t.Fatalf("ServiceKey 不是 *Store：%T", kernel.Get(ServiceKey))
	}
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("agent-team-containers"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1180, Height: 800}, MainWidth: 900,
		Geometry: pluginapi.Geometry{Width: 1180, Height: 800, MainWidth: 900, SidebarWidth: 280},
	}
	return &containerHarness{kernel: kernel, plugin: plugin, store: store, env: env, canvas: canvas}
}

// drawContainer 画某个父容器 painter，dispatch 走**真内核**的 DrawSlot。
func (h *containerHarness) drawContainer(t *testing.T, painterID string) *gfx.Canvas {
	t.Helper()
	var target *pluginapi.Painter
	panels := h.panels()
	for i := range panels {
		if panels[i].ID == painterID {
			target = &panels[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("找不到父容器 painter %q", painterID)
	}
	if target.Children == nil {
		t.Fatalf("父容器 %q 没有 Children = 里面没有 drawSlot", painterID)
	}
	theme := dscore.Dark()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.env.Tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	h.canvas = canvas
	env := *h.env
	env.Canvas = canvas
	h.rendered = nil
	var view ui.Page
	view.Draw(canvas, dscore.VivaTheme(theme), env.Bounds, func(f ui.C) {
		// 与内核同一个顺序：先 Draw（命中区/兜底卡），再 Children（dispatch）。
		if target.Draw != nil {
			target.Draw(f, &env)
		}
		target.Children(f, &env, func(slot pluginapi.Slot) {
			h.rendered = append(h.rendered, slot)
			h.kernel.DrawSlot(f, slot, &env)
		})
	})
	return canvas
}

// panels 取本插件的全部 painter（Panels() 每次调用都会新建切片，所以这里只读）。
func (h *containerHarness) panels() []pluginapi.Painter { return h.plugin.Panels() }

func (h *containerHarness) renderedSlot(slot pluginapi.Slot) int {
	n := 0
	for _, got := range h.rendered {
		if got == slot {
			n++
		}
	}
	return n
}

// ── 容器 D：chat 命令节点 ────────────────────────────────────────────

// TestCommandNodeContainerReallyDrawsTeamRow 是本单最关键的一条断言。
//
// 链路：Store.SetCommand(team 回执) → 容器 D 的 Children → drawSlot(真内核)
//
//	→ dsagentteam 的 agent-team.command 画 TeamCommandRow。
//
// 判据不是"dispatch 被调用"，而是**画布上真的出现了 TeamCommandRow 的标题**。
func TestCommandNodeContainerReallyDrawsTeamRow(t *testing.T) {
	h := newContainerHarness(t)
	h.store.SetCommand(&CommandNode{
		Command: CommandKey,
		Outcome: &CommandOutcome{Kind: "success", Text: "已开启智能体团队协作"},
	})

	canvas := h.drawContainer(t, "agent-team.command-node-legacy")

	if got := h.renderedSlot(SlotChatCommandView); got != 1 {
		t.Fatalf("容器 D 必须 dispatch 一次 %q，实际 %d 次；rendered=%v",
			SlotChatCommandView, got, h.rendered)
	}
	if !hasText(canvas, zhCommandTitle) {
		t.Fatalf("team 回执没有真的画出来（缺 %q）；文本=%v", zhCommandTitle, textValues(canvas))
	}
	if !anyTextContains(canvas, "已开启智能体团队协作") {
		t.Fatalf("回执正文没画出来；文本=%v", textValues(canvas))
	}
}

// TestCommandNodeContainerFourStates 是容器 D 在**真装配**里的四态。
func TestCommandNodeContainerFourStates(t *testing.T) {
	cases := []struct {
		name string
		node *CommandNode
		// wantTeamRow 是"GB 的 TeamCommandRow 画出来了"。
		wantTeamRow bool
		// wantFallback 是容器自己画了官方兜底卡（title = 裸命令名）。
		wantFallback bool
	}{
		{
			name: "无命令：Store 里没有命令节点 → 一个像素都不画",
			node: nil,
		},
		{
			name:         "未认领：别的命令（/compact）→ 兜底卡，不 dispatch team 的行",
			node:         &CommandNode{Command: "compact", Outcome: &CommandOutcome{Kind: "success", Text: "已压缩"}},
			wantFallback: true,
		},
		{
			name:        "正常：team 命令 → 认领，TeamCommandRow 真画出来",
			node:        &CommandNode{Command: CommandKey, Outcome: &CommandOutcome{Kind: "success", Text: "已开启"}},
			wantTeamRow: true,
		},
		{
			name:        "数据缺失：没有 outcome → 仍画（官方 running 态），不是隐藏",
			node:        &CommandNode{Command: CommandKey},
			wantTeamRow: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newContainerHarness(t)
			h.store.SetCommand(tc.node)
			canvas := h.drawContainer(t, "agent-team.command-node-legacy")

			if tc.wantTeamRow {
				if h.renderedSlot(SlotChatCommandView) != 1 {
					t.Fatalf("team 命令必须 dispatch 子槽一次；rendered=%v", h.rendered)
				}
				if !hasText(canvas, zhCommandTitle) {
					t.Fatalf("TeamCommandRow 没画出来；文本=%v", textValues(canvas))
				}
				return
			}
			if h.renderedSlot(SlotChatCommandView) != 0 {
				t.Fatalf("不该 dispatch team 子槽；rendered=%v", h.rendered)
			}
			if tc.wantFallback {
				// 兜底卡的 title 是裸命令名（官方 GenericCommandCard.tsx:37）。
				if !hasText(canvas, "compact") {
					t.Fatalf("兜底卡缺裸命令名；文本=%v", textValues(canvas))
				}
				if !anyTextContains(canvas, "已压缩") {
					t.Fatalf("兜底卡缺 summary；文本=%v", textValues(canvas))
				}
				return
			}
			if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
				t.Fatalf("无命令态不该画任何东西，却画了 %d 矩形 / %d 文本",
					len(canvas.Rectangles()), len(canvas.Texts()))
			}
		})
	}
}

// TestClaimsCommandViewOnlyTeam 钉住 keyed 认领范围：只有 team。
func TestClaimsCommandViewOnlyTeam(t *testing.T) {
	h := newContainerHarness(t)
	if !h.plugin.ClaimsCommandView(CommandKey) {
		t.Fatal("team 必须被本包认领（官方 mount.ts:36-38 的 key: 'team'）")
	}
	for _, other := range []string{"compact", "model", "permission", ""} {
		if h.plugin.ClaimsCommandView(other) {
			t.Fatalf("命令 %q 不该被本包认领（其余命令用官方兜底卡）", other)
		}
	}
}

// ── 容器 C：模型菜单 ────────────────────────────────────────────────

// TestModelMenuContainerFourStates 是容器 C 在**真装配**里的四态。
//
// 判据：菜单展开时，容器里那次 drawSlot 必须把 `conversation.input.model.options`
// 画出来（GB 的 Team 开关就在这个槽上）——这才叫"父容器接上了"。
func TestModelMenuContainerFourStates(t *testing.T) {
	cases := []struct {
		name string
		// setup 造这一态的数据面。
		setup func(t *testing.T, h *containerHarness)
		// wantMenu 是"菜单面画出来了"。
		wantMenu bool
		// wantToggle 是"GB 的 Team 开关真的出现在菜单里了"。
		wantToggle bool
	}{
		{
			name: "未配置：没有任何 provider 目录来源 → 整段隐藏",
			setup: func(t *testing.T, h *containerHarness) {
				h.store.ToggleModelMenu() // 展开也没用：known=false
			},
		},
		{
			name: "无 provider：宿主回空目录 → 整段隐藏",
			setup: func(t *testing.T, h *containerHarness) {
				h.store.SetProviders(true, "", nil)
				h.store.ToggleModelMenu()
			},
		},
		{
			name: "数据缺失：取目录失败 → 官方 error 行，不编 provider",
			setup: func(t *testing.T, h *containerHarness) {
				h.store.SetProviders(true, "llm.listConfigurableProviders 未接线（501）", nil)
				h.store.ToggleModelMenu()
			},
			wantMenu: true,
		},
		{
			name: "正常：真目录 + 投影在 → 菜单 + provider 行 + Team 开关",
			setup: func(t *testing.T, h *containerHarness) {
				h.store.SetProviders(true, "", []ModelProviderOption{
					{Provider: "deepseek", DisplayName: "DeepSeek", Seed: "llm"},
				})
				// 投影在，GB 的 Team 开关才会渲染（TeamModeToggle.tsx:43 守卫）。
				h.store.SetView(teamView())
				h.store.ToggleModelMenu()
			},
			wantMenu:   true,
			wantToggle: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newContainerHarness(t)
			tc.setup(t, h)
			canvas := h.drawContainer(t, "agent-team.model-menu")

			if !tc.wantMenu {
				// 菜单不画，但**占用方本身还在**：trigger 降级成只读 chip
				// （宿主没给模型 → 「未知」），它不是菜单、没有命中区。
				if h.renderedSlot(SlotInputModelOptions) != 0 {
					t.Fatalf("隐藏态不该 dispatch 子槽；rendered=%v", h.rendered)
				}
				if hasHitLabel(canvas, zhModelMenuAria) {
					t.Fatalf("目录不可用时不该给可点 trigger（假入口）；hits=%v", hitLabels(canvas))
				}
				if hasText(canvas, "DeepSeek") || anyTextContains(canvas, "未接线") {
					t.Fatalf("隐藏态菜单里不该出现 provider/error 行；文本=%v", textValues(canvas))
				}
				if !hasText(canvas, zhModelUnknown) {
					t.Fatalf("占用方降级态应显示 %q 的只读 chip；文本=%v", zhModelUnknown,
						textValues(canvas))
				}
				return
			}
			if h.renderedSlot(SlotInputModelOptions) != 1 {
				t.Fatalf("菜单展开必须 dispatch 一次 %q；rendered=%v",
					SlotInputModelOptions, h.rendered)
			}
			if !anyTextContains(canvas, "DeepSeek") && !anyTextContains(canvas, "未接线") {
				t.Fatalf("菜单里既没有 provider 行也没有 error 行；文本=%v", textValues(canvas))
			}
			if tc.wantToggle && !hasText(canvas, zhModeLabel) {
				t.Fatalf("Team 开关（槽 ②）没有出现在菜单里（缺 %q）；文本=%v",
					zhModeLabel, textValues(canvas))
			}
		})
	}
}

// TestModelMenuTriggerOnlyWhenProvidersKnown 钉住"没有目录就不给假入口"。
//
// 官方 `available=false` 时 trigger 不可用；本仓对应判据：目录未知时不盖命中区。
func TestModelMenuTriggerOnlyWhenProvidersKnown(t *testing.T) {
	h := newContainerHarness(t)
	// 未配置：不该有 "模型与推理等级" 这个 trigger 命中区。
	h.drawContainer(t, "agent-team.model-menu")
	if hasHitLabel(h.canvas, zhModelMenuAria) {
		t.Fatal("目录未知时不该给 trigger 命中区（点了只会打开空菜单 = 假入口）")
	}

	// 有目录：trigger 命中区出现（点了才可能开菜单）。
	h.store.SetProviders(true, "", []ModelProviderOption{{Provider: "deepseek", DisplayName: "DeepSeek"}})
	h.drawContainer(t, "agent-team.model-menu")
	if !hasHitLabel(h.canvas, zhModelMenuAria) {
		t.Fatalf("有目录时必须有 trigger 命中区；hits=%v", hitLabels(h.canvas))
	}
}

// TestModelMenuProvidersComeFromApiclient 断言 provider 目录经 internal/apiclient 进来。
//
// 用真 `*apiclient.Client` + httptest 假宿主走一遍：注入值 → Register → RefreshProviders
// → 菜单里出现服务端给的 displayName。这条链是"不直连宿主内部"的证据。
func TestModelMenuProvidersComeFromApiclient(t *testing.T) {
	srv := fakeProviderServer(t, `{"result":[{"provider":"deepseek","displayName":"服务端给的 DeepSeek","settingsNs":"llm"}]}`)

	client := apiclient.New(srv)
	var fetched bool
	h := &containerHarness{kernel: uikernel.New("providers-e2e")}
	withProviders := &Plugin{}
	h.plugin = withProviders
	store := NewStore(nil)
	withProviders.team = store
	// 直接走 SetProvidersRemote + RefreshProviders（Register 里就是这两步）。
	withProviders.SetProvidersRemote(client)
	store.RefreshProviders()
	waitProviders(t, store, &fetched)
	store.ToggleModelMenu()

	h.store = store
	h.env = containerHarnessEnv()
	canvas := h.drawContainer(t, "agent-team.model-menu")
	if !anyTextContains(canvas, "服务端给的 DeepSeek") {
		t.Fatalf("apiclient 拿到的 provider 没有出现在菜单里；文本=%v", textValues(canvas))
	}
}

// fakeProviderServer 起一个只认 `llm.listConfigurableProviders` 的假宿主。
func fakeProviderServer(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/llm.listConfigurableProviders" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// waitProviders 等目录拉完（RefreshProviders 是异步的）。
func waitProviders(t *testing.T, store *Store, _ *bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if known, failure, _ := store.Providers(); known || failure != "" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	known, failure, list := store.Providers()
	t.Fatalf("等不到 provider 目录：known=%v failure=%q list=%d", known, failure, len(list))
}

// ── 小工具 ─────────────────────────────────────────────────────────

func containerHarnessEnv() *pluginapi.Env {
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	return &pluginapi.Env{
		Theme: dscore.Dark(), Tree: tree, Feat: tree.Feature("agent-team-containers"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1180, Height: 800}, MainWidth: 900,
		Geometry: pluginapi.Geometry{Width: 1180, Height: 800, MainWidth: 900, SidebarWidth: 280},
	}
}

func hasHitLabel(canvas *gfx.Canvas, label string) bool {
	for _, got := range hitLabels(canvas) {
		if strings.Contains(got, label) {
			return true
		}
	}
	return false
}
