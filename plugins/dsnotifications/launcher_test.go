// launcher_test.go —— 父容器 A（设置启动器通知入口）的结构、接线与**四态**证据。
//
// 判据不是"函数被调了"，而是：
//
//	① 层级      sidebar.settings → settings.launcher → settings.launcher.notifications
//	            （官方 uikernel 槽表里父槽逐级对得上）
//	② 真画      用**真内核**装配后从 `sidebar.settings` 开始 dispatch，
//	            叶槽上的探针 painter 必须被调用——这证明 `Children` 接线真的跑到了，
//	            而不是"只注册了一个 painter、壳从不调用"（GB 报告 §7.3 那一类空白）
//	③ 四态      未授权 / 未启用（收起轨）/ 数据缺失（null 批次）/ 正常
//	            每种都在画布上取证据：卡片矩形（半径 16）在不在、文案对不对
//	④ 几何      `.notifications{bottom:calc(100% + 4px)}`：卡片下沿 = 行顶 - 4
package dsnotifications

import (
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// probePlugin 在指定槽位画一块并计数：证明父级**真的** dispatch 了这个子槽。
type probePlugin struct {
	id   string
	slot pluginapi.Slot

	mu    sync.Mutex
	drawn int
}

func (p *probePlugin) ID() string                       { return p.id }
func (p *probePlugin) Provides() []string               { return nil }
func (p *probePlugin) Requires() []string               { return nil }
func (p *probePlugin) Register(pluginapi.Context) error { return nil }

func (p *probePlugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   p.id,
		Slot: p.slot,
		Draw: func(f ui.C, env *pluginapi.Env) {
			p.mu.Lock()
			p.drawn++
			p.mu.Unlock()
			// 真画一个盒子：避免"只计数、什么都没有"看起来像没画。
			ui.Row(f).Width(4).Height(4)
		},
	}}
}

func (p *probePlugin) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.drawn
}

// launcherChain 装配真内核：ds-core（主题）+ 本插件 + 探针。
//
// 壳档位对象与生产路径一致：**Load 之前** Provide（插件 Register 里 ctx.Inject 到），
// Load 之后由测试决定写哪一档。
func launcherChain(t *testing.T, p *Plugin, extra ...pluginapi.Plugin) (*uikernel.Kernel, *uikernel.ShellTier) {
	t.Helper()
	k := uikernel.New("launcher-chain")
	tier := uikernel.NewShellTier()
	k.Root().Provide(uikernel.KeyShellTier, tier)
	plugins := []pluginapi.Plugin{&dscore.Plugin{Dark: true}, p}
	plugins = append(plugins, extra...)
	if err := k.Load(plugins...); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	return k, tier
}

// renderFromSlot 从某个槽位开始 dispatch 一帧，返回画布。
//
// 侧栏宽 280（官方 SIDEBAR_DEFAULT）——`wide` 判据与座位几何都依赖这个宽度。
func renderFromSlot(t *testing.T, k *uikernel.Kernel, slot pluginapi.Slot) *gfx.Canvas {
	t.Helper()
	const sidebarWidth float32 = uikernel.SidebarDefault // 280
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("launcher-chain")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	bounds := gfx.Rect{Width: 1180, Height: 800}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: 900,
		Has:      k.Has,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, SidebarWidth: sidebarWidth},
	}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, bounds, func(f ui.C) {
		// 把链放进一条 280 宽的侧栏列：座位与行宽都按它算。
		ui.Column(f).Width(sidebarWidth).Children(func() {
			k.DrawSlot(f, slot, env)
		})
	})
	return canvas
}

// TestLauncherSlotHierarchy 官方两级父容器逐级对表（层级写错就没有"父容器"）。
func TestLauncherSlotHierarchy(t *testing.T) {
	if got := string(SlotSidebarSettings); got != "sidebar.settings" {
		t.Fatalf("一级父槽必须是官方 sidebar.settings，得到 %q", got)
	}
	if got := string(SlotSettingsLauncher); got != "settings.launcher" {
		t.Fatalf("二级父槽必须是官方 settings.launcher，得到 %q", got)
	}
	if got := string(SlotSettingsLauncherNotifications); got != "settings.launcher.notifications" {
		t.Fatalf("叶槽必须是官方 settings.launcher.notifications，得到 %q", got)
	}
	if err := RegisterOfficialSlot(); err != nil {
		t.Fatalf("登记官方槽失败：%v", err)
	}
	// sidebar.settings 本身是 uikernel 内置槽（GA 的首屏槽表）。
	parentSpec, ok := uikernel.LookupOfficialSlot(SlotSidebarSettings)
	if !ok {
		t.Fatal("官方槽表里没有 sidebar.settings（一级父槽）")
	}
	launcherSpec, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncher)
	if !ok {
		t.Fatal("官方槽表里没有 settings.launcher（二级父槽）")
	}
	if launcherSpec.Parent != SlotSidebarSettings {
		t.Fatalf("settings.launcher 的父槽应是 sidebar.settings，得到 %q", launcherSpec.Parent)
	}
	if launcherSpec.Host != "SettingsRoot.triggerRow" {
		t.Fatalf("settings.launcher 的实现节点应是 SettingsRoot.triggerRow，得到 %q", launcherSpec.Host)
	}
	// 官方锚点必须是 rc.2 src 原件（不是 rc.1 的压缩产物）。
	for _, spec := range []uikernel.SlotSpec{parentSpec, launcherSpec} {
		if !strings.Contains(spec.Anchor, "src/client/") && !strings.Contains(spec.Anchor, "src/") {
			t.Fatalf("%q 的锚点必须是 rc.2 src 行号，得到 %q", spec.Name, spec.Anchor)
		}
	}
	if !strings.Contains(launcherSpec.Anchor, "ui-settings-general/src/client/SettingsRoot.tsx:226") {
		t.Fatalf("settings.launcher 的锚点应指向 SettingsRoot.tsx:226，得到 %q", launcherSpec.Anchor)
	}
	leafSpec, ok := uikernel.LookupOfficialSlot(SlotSettingsLauncherNotifications)
	if !ok {
		t.Fatal("官方槽表里没有 settings.launcher.notifications")
	}
	if leafSpec.Parent != SlotSettingsLauncher {
		t.Fatalf("叶槽的父槽必须是 settings.launcher（真实层级），得到 %q", leafSpec.Parent)
	}
}

// TestLauncherPanelsRegistration 三条父级 painter + 一条叶 painter 都在位。
func TestLauncherPanelsRegistration(t *testing.T) {
	p := newTestPlugin(nil)
	byID := map[string]pluginapi.Painter{}
	for _, painter := range p.Panels() {
		byID[painter.ID] = painter
	}
	for _, want := range []struct {
		id     string
		slot   pluginapi.Slot
		draw   bool
		childr bool
	}{
		{"account-launcher.row", SlotSidebarSettings, false, true},
		{"account-launcher.menu", SlotSettingsLauncher, true, true},
		{"account-notices", SlotSettingsLauncherNotifications, true, false},
	} {
		painter, ok := byID[want.id]
		if !ok {
			t.Fatalf("缺少 painter %q", want.id)
		}
		if painter.Slot != want.slot {
			t.Fatalf("%q 应挂在 %q，实际 %q", want.id, want.slot, painter.Slot)
		}
		if want.draw && painter.Draw == nil {
			t.Fatalf("%q 必须有 Draw", want.id)
		}
		if want.childr && painter.Children == nil {
			t.Fatalf("%q 必须有 Children（父容器靠它 dispatch 子槽）", want.id)
		}
	}
}

// TestLauncherChainDispatchesNotificationsSlot 用真内核证明"叶槽真的被画到"。
func TestLauncherChainDispatchesNotificationsSlot(t *testing.T) {
	p := newTestPlugin(nil)
	leaf := &probePlugin{id: "probe-leaf", slot: SlotSettingsLauncherNotifications}
	k, _ := launcherChain(t, p, leaf)
	// wide 态（测试 renderFromSlot 的侧栏 280）——座位只在宽态建。
	p.ctrl.Update(AccountState{Active: true, SignedIn: true, Wide: true})

	renderFromSlot(t, k, SlotSidebarSettings)
	if leaf.count() != 1 {
		t.Fatalf("从 sidebar.settings dispatch 一帧，叶槽应被画 1 次，实际 %d 次"+
			"（0 = 父容器没 dispatch；>1 = 画了两遍）", leaf.count())
	}

	// 二级父槽单独 dispatch 也要落到叶槽（官方 renderSlot 是无条件的）。
	renderFromSlot(t, k, SlotSettingsLauncher)
	if leaf.count() != 2 {
		t.Fatalf("从 settings.launcher dispatch 后叶槽累计应为 2，实际 %d", leaf.count())
	}
}

// TestLauncherMountPointFollowsDispatchedSlots 挂载点必须跟着**这一帧真正被 dispatch 的槽**走。
//
// 三档（真机 `-shell official` 踩到第三档）都从同一个官方槽进：
//
//	official（无旧部件）  sidebar.settings 被 dispatch（官方骨架自己画那一行）
//	official + 旧部件桥   换旧三栏排版：壳仍画设置行（drawLegacySidebar）
//	legacy                dsshell 只 dispatch 旧槽 —— 它现在也画设置行
//
// 判据不只是"画了"，还有"**另一边一次都没画**"（双画会让同一行出现两次）。
func TestLauncherMountPointFollowsDispatchedSlots(t *testing.T) {
	cases := []struct {
		name     string
		official bool
		bridge   bool
		wantFrom pluginapi.Slot // 从哪个槽开始 dispatch 会落到叶槽
		other    pluginapi.Slot // 这个槽必须一次都不 dispatch
	}{
		// 三档都从同一个官方槽进：壳在**所有**绘制路径下都画设置行
		// （uikernel 官方骨架 / uikernel 兼容桥 / dsshell 旧三栏），
		// 所以不再需要"过渡桥座位"（HH 单第 1 件）。
		{"official 无桥", true, false, SlotSidebarSettings, pluginapi.SlotSidebarFooter},
		{"official + 旧部件桥", true, true, SlotSidebarSettings, pluginapi.SlotSidebarFooter},
		{"legacy", false, false, SlotSidebarSettings, pluginapi.SlotSidebarFooter},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			leaf := &probePlugin{id: "probe-leaf", slot: SlotSettingsLauncherNotifications}
			k, tier := launcherChain(t, p, leaf)
			p.ctrl.Update(AccountState{Active: true, SignedIn: true, Wide: true})
			tier.Set(tc.official)
			tier.SetLegacyBridge(tc.bridge)

			renderFromSlot(t, k, tc.wantFrom)
			if leaf.count() != 1 {
				t.Fatalf("从 %q dispatch 后叶槽应被画 1 次，实际 %d", tc.wantFrom, leaf.count())
			}
			renderFromSlot(t, k, tc.other)
			if leaf.count() != 1 {
				t.Fatalf("%q 这一帧不该 dispatch 叶槽（会双画），实际累计 %d", tc.other, leaf.count())
			}
		})
	}
}

// launcherSignedIn 造一个"已登录 + 宽态"的插件（通知数据由 remote 给）。
func launcherSignedIn(t *testing.T, remote Remote) *Plugin {
	t.Helper()
	p := newTestPlugin(remote)
	p.ctrl.Update(AccountState{Active: true, SignedIn: true, Wide: true})
	return p
}

// TestLauncherFourStates 四态：未授权 / 未启用 / 数据缺失 / 正常。
//
// 每一态都断言三件事：① 父容器仍然 dispatch 叶槽（官方 renderSlot 无条件）；
// ② 卡片画不画；③ 启动器那一行说哪句官方文案。
func TestLauncherFourStates(t *testing.T) {
	fullBatch := &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{sampleNotice("n-state")},
	}
	cases := []struct {
		name     string
		remote   Remote
		state    AccountState
		wantCard bool
		wantText string
		why      string
	}{
		{
			name:   "未授权",
			remote: nil,
			// 没登录：官方 AccountMenu 画 32px「更多」行，通知座位整段不激活。
			state:    AccountState{Active: false, SignedIn: false, Wide: true},
			wantCard: false, wantText: zhAccountMore,
			why: "无 remote = 未授权：控制器不轮询、不画卡片，但启动器行仍是官方登出态",
		},
		{
			name: "未启用",
			// 侧栏收起成轨：官方 `hidden={!wide}` 整段不渲染（座位都不建）。
			remote:   &fakeRemote{batch: fullBatch},
			state:    AccountState{Active: true, SignedIn: true, Wide: false},
			wantCard: false, wantText: "", // 收起态官方不给 label（只留 icon）
			why: "wide=false = 侧栏收起轨：官方 hidden={!wide}，通知座位不建",
		},
		{
			name:   "数据缺失",
			remote: &fakeRemote{batch: nil}, // 宿主回 null = 未登录/授权变化（不是空批次）
			state:  AccountState{Active: true, SignedIn: true, Wide: true},
			// 已登录但没有任何通知可画：一帧都不该出现卡片，也不许编一条。
			wantCard: false, wantText: zhAccountSignedIn,
			why: "batch=null：没有未过期通知就不画（不是画空壳）",
		},
		{
			name:     "正常",
			remote:   &fakeRemote{batch: fullBatch},
			state:    AccountState{Active: true, SignedIn: true, Wide: true},
			wantCard: true, wantText: zhAccountSignedIn,
			why: "有未过期通知 + active：卡片画在座位里",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(tc.remote)
			leaf := &probePlugin{id: "probe-leaf", slot: SlotSettingsLauncherNotifications}
			k, _ := launcherChain(t, p, leaf)
			p.ctrl.Update(tc.state)
			if tc.wantCard {
				waitUntil(t, "卡片到达", func() bool { return len(p.ctrl.Notices()) > 0 })
			}

			canvas := renderFromSlot(t, k, SlotSidebarSettings)

			// ① 父容器**总是** dispatch 叶槽（wide 为真时）：这是"槽真被画到"的证明。
			wantLeaf := 1
			if !tc.state.Wide {
				// 收起轨：官方 `hidden={!wide}` 连 renderSlot 都不执行。
				wantLeaf = 0
			}
			if leaf.count() != wantLeaf {
				t.Fatalf("%s：叶槽 dispatch 次数应为 %d，实际 %d", tc.why, wantLeaf, leaf.count())
			}

			// ② 卡片在不在。
			_, hasCard := cardRect(canvas)
			if hasCard != tc.wantCard {
				t.Fatalf("%s：卡片应存在=%v，实际=%v", tc.why, tc.wantCard, hasCard)
			}

			// ③ 启动器那一行的官方文案。
			if tc.wantText != "" {
				if _, ok := textByValue(canvas, tc.wantText); !ok {
					t.Fatalf("%s：找不到启动器文案 %q", tc.why, tc.wantText)
				}
			}
			// "不编 0"：任何一态都不许出现造出来的通知/名字文案。
			for _, text := range canvas.Texts() {
				if strings.Contains(text.Value, "0 条通知") || strings.Contains(text.Value, "0 个") {
					t.Fatalf("%s：画出了编造的计数 %q", tc.why, text.Value)
				}
			}
		})
	}
}

// TestLauncherSeatGeometry 官方 `.notifications{bottom:calc(100% + 4px)}` 落到矩形上。
//
// 座位是 absolute 子节点：bottom 边缘距容器**底边** h+4 → 等效于
// "卡片下沿在容器顶边上方 4px"。这里从画布上直接量卡片矩形。
func TestLauncherSeatGeometry(t *testing.T) {
	remote := &fakeRemote{batch: &apiclient.NotificationBatch{
		NextPollAfterSeconds: 600,
		Notifications:        []apiclient.Notification{geometryNotice()},
	}}
	p := launcherSignedIn(t, remote)
	waitUntil(t, "卡片到达", func() bool { return len(p.ctrl.Notices()) > 0 })
	k, _ := launcherChain(t, p)
	canvas := renderFromSlot(t, k, SlotSidebarSettings)

	card, ok := cardRect(canvas)
	if !ok {
		t.Fatal("找不到卡片（半径 16 的矩形）")
	}
	// 行高 44（已登录 + 宽态），座位下沿 = 行顶 - 4 = -4。
	wantBottom := (LauncherRowHeight - LauncherRowHeight) - SeatGap
	if !near(card.Y+card.Height, wantBottom) {
		t.Fatalf("卡片下沿应 = 行顶 - %.0fpx = %.1f，实际 %.1f（卡片 y=%.1f h=%.1f）",
			SeatGap, wantBottom, card.Y+card.Height, card.Y, card.Height)
	}
	if !near(card.Width, StackMaxWidth) {
		t.Fatalf("卡片宽应 = %.0f（.stack max-width），实际 %.1f", StackMaxWidth, card.Width)
	}

	// 启动器行的命中区存在（点它能打开设置），高度 44。
	hit, ok := hitByLabel(canvas, zhAccountMenu)
	if !ok {
		t.Fatalf("找不到启动器命中区（label=%q）", zhAccountMenu)
	}
	if hit.Bounds.Height < LauncherRowHeight-1 {
		t.Fatalf("启动器行高应 ≈ %.0f，实际 %.1f", LauncherRowHeight, hit.Bounds.Height)
	}
}

// TestLauncherOpenSettingsHonest 没装设置面板时点启动器要**如实说没有**，不假装跳转。
func TestLauncherOpenSettingsHonest(t *testing.T) {
	p := newTestPlugin(nil)
	var said string
	env := &pluginapi.Env{
		Act: pluginapi.Actions{Notify: func(msg string) { said = msg }},
	}
	if p.LauncherOpenSettings(env) {
		t.Fatal("没有设置能力时不能报告「打开成功」")
	}
	if said != zhAccountNoSettings {
		t.Fatalf("应如实提示 %q，实际 %q", zhAccountNoSettings, said)
	}

	// 接上能力之后：真的调它，并把它的返回值当结果（不伪造 true）。
	called := false
	p.settings = func() bool { called = true; return true }
	if !p.LauncherOpenSettings(env) {
		t.Fatal("能力返回 true 时应报告成功")
	}
	if !called {
		t.Fatal("没有真的调用设置面板的打开能力")
	}
}
