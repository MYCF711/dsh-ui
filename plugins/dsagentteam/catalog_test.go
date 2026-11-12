// catalog_test.go —— 父容器 B（子代理目录 + footer 槽）的结构、接线与**四态**证据。
//
// 判据：
//
//	① 层级    conversation.session.header.actions（官方父槽）
//	          → SubagentCatalogAction（触发器）
//	          → 面板（官方 createPortal 的等价层 = shell.overlay）
//	          → conversation.session.header.actions.subagent.footer（**真 drawSlot**）
//	② 真画    真内核装配后从官方父槽 dispatch：触发器必须画出来；
//	          面板展开后 `footer` 槽上的探针 painter 必须被调用——证明是"真画到"，
//	          不是"只注册"（GB 报告 §7.3 的空白形态）。
//	③ 四态    未授权 / 未启用 / 数据缺失 / 正常，逐态在画布上取证据。
//	④ 不编 0  目录未知时不许出现「0 个子智能体」这种编出来的计数。
package dsagentteam

import (
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// footerProbe 在 footer 槽上画一块并计数（证明父容器真的 dispatch 了它）。
type footerProbe struct {
	mu    sync.Mutex
	drawn int
}

func (p *footerProbe) ID() string                       { return "probe-footer" }
func (p *footerProbe) Provides() []string               { return nil }
func (p *footerProbe) Requires() []string               { return nil }
func (p *footerProbe) Register(pluginapi.Context) error { return nil }

func (p *footerProbe) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:   "probe-footer",
		Slot: SlotSubagentFooter,
		Draw: func(f *flex.Frame, env *pluginapi.Env) {
			p.mu.Lock()
			p.drawn++
			p.mu.Unlock()
			// 真画一块（模拟 TeamBoard 的 section）：有像素、有命中区。
			f.Column().FillWidth().Height(20).Hit(
				env.Tree.ID(env.Feat, "probe-footer-body"), "共享任务")
		},
	}}
}

func (p *footerProbe) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.drawn
}

// catalogChain 装配真内核：ds-core + 本插件 + 附加插件；档位对象在 Load 前 Provide。
func catalogChain(t *testing.T, p *Plugin, extra ...pluginapi.Plugin) (*uikernel.Kernel, *uikernel.ShellTier) {
	t.Helper()
	k := uikernel.New("catalog-chain")
	tier := uikernel.NewShellTier()
	k.Root().Provide(uikernel.KeyShellTier, tier)
	plugins := []pluginapi.Plugin{&dscore.Plugin{Dark: true}, p}
	plugins = append(plugins, extra...)
	if err := k.Load(plugins...); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	// 壳在 Load 之后的动作：把档位告诉本容器（cmd/app 的 wave3_wiring 同形）。
	p.AttachShellTier(tier)
	return k, tier
}

// renderFromSlot 从某个槽位 dispatch 一帧。
func renderFromSlot(t *testing.T, k *uikernel.Kernel, slot pluginapi.Slot) *ui.Canvas {
	t.Helper()
	theme := dscore.Dark()
	tree := &ui.Tree{}
	feat := tree.Feature("catalog-chain")
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	bounds := ui.Rect{Width: 1180, Height: 800}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: 900,
		Has:      k.Has,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280},
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
		k.DrawSlot(f, slot, env)
	})
	return canvas
}

// TestCatalogPanelsRegistration 三条 painter 各就各位，且**每个都有真实 Draw**
// （`uikernel.Load` 对"没有 Draw 的画"是 fail-loud——空 Draw 就是骗校验）。
func TestCatalogPanelsRegistration(t *testing.T) {
	// 官方槽表登记发生在 Register（真装配时由内核调用）；这里手工补一次。
	if err := RegisterOfficialSlots(); err != nil {
		t.Fatalf("登记官方槽失败：%v", err)
	}
	p := newTestPlugin(nil)
	byID := map[string]pluginapi.Painter{}
	for _, painter := range p.catalogPanels() {
		byID[painter.ID] = painter
	}
	for _, want := range []struct {
		id     string
		slot   pluginapi.Slot
		childr bool
	}{
		// 官方父槽：`conversation.session.header.actions`（ui-subagent 注册处）
		{"agent-team.catalog-trigger", SlotSessionHeaderActions, false},
		// 旧三栏过渡桥（official 档自动让位）
		{"agent-team.catalog-trigger-legacy", pluginapi.SlotMainHeader, false},
		// 面板：Draw = 遮罩层，Children = 面板本体 + **footer 槽 dispatch**
		{"agent-team.catalog-panel", pluginapi.SlotShellOverlay, true},
	} {
		painter, ok := byID[want.id]
		if !ok {
			t.Fatalf("缺少 painter %q", want.id)
		}
		if painter.Slot != want.slot {
			t.Fatalf("%q 应挂在 %q，实际 %q", want.id, want.slot, painter.Slot)
		}
		if painter.Draw == nil && painter.AfterDraw == nil {
			t.Fatalf("%q 没有 Draw/AfterDraw：内核装配会 fail-loud", want.id)
		}
		if want.childr && painter.Children == nil {
			t.Fatalf("%q 必须有 Children（footer 槽靠它 dispatch）", want.id)
		}
	}
	// footer 槽的父级仍是官方那个父槽（GB 登记没被改坏）。
	spec, ok := uikernel.LookupOfficialSlot(SlotSubagentFooter)
	if !ok {
		t.Fatal("官方槽表里没有 subagent.footer")
	}
	if spec.Parent != uikernel.OffSessionHeaderActions {
		t.Fatalf("subagent.footer 的父槽应是 %q，得到 %q", uikernel.OffSessionHeaderActions, spec.Parent)
	}
	if SlotSessionHeaderActions != uikernel.OffSessionHeaderActions {
		t.Fatalf("本包的父槽常量必须与 uikernel 的官方槽一致：%q ≠ %q",
			SlotSessionHeaderActions, uikernel.OffSessionHeaderActions)
	}
}

// catalogReadyView 造一份"目录已知 + 两条子代理"的基线视图。
func catalogReadyView() View {
	v := teamView()
	v.CatalogKnown = true
	v.Catalog = []CatalogEntry{
		{ID: "child-1", Label: "收集素材", Mode: "continuable"},
		{ID: "child-2", Label: "", Mode: "continuable"}, // label 空：官方兜底显示 id
	}
	v.Members = len(v.Catalog)
	return v
}

// TestCatalogChainDispatchesFooterSlot 用真内核证明 footer 槽真的被 drawSlot。
func TestCatalogChainDispatchesFooterSlot(t *testing.T) {
	p := newTestPlugin(nil)
	probe := &footerProbe{}
	k, _ := catalogChain(t, p, probe)
	p.team.SetView(catalogReadyView())
	p.OpenCatalog()

	// ① 官方父槽：触发器必须画出来（否则用户根本没有入口）。
	triggerCanvas := renderFromSlot(t, k, SlotSessionHeaderActions)
	if _, ok := hitByLabelPrefix(triggerCanvas, "子智能体"); !ok {
		t.Fatal("官方父槽 dispatch 后没有画触发器（找不到「…个子智能体」命中区）")
	}

	// ② 面板层：footer 槽被真 dispatch（官方 renderFooter 的位置）。
	renderFromSlot(t, k, pluginapi.SlotShellOverlay)
	if probe.count() != 1 {
		t.Fatalf("展开后 footer 槽应被画 1 次，实际 %d 次（0 = 父容器没 dispatch）", probe.count())
	}

	// ③ 收起后不再画（面板关掉就什么都不 dispach）。
	p.CloseCatalog()
	renderFromSlot(t, k, pluginapi.SlotShellOverlay)
	if probe.count() != 1 {
		t.Fatalf("收起后 footer 槽不该再被画，实际累计 %d", probe.count())
	}
}

// hasPanelRect 判断面板框画了没有：官方 `.menu` 的圆角是 radius md（12），
// 触发器是 radius sm（8），所以"半径 12 且宽 ≈360"的矩形只可能是面板。
func hasPanelRect(canvas *ui.Canvas) bool {
	for _, r := range canvas.Rectangles() {
		if r.Radius == catalogMenuRadius && r.Bounds.Width > catalogMenuWidth/2 {
			return true
		}
	}
	return false
}

// hitByLabelPrefix 按文案找命中区（计数文案带数字，用子串匹配）。
func hitByLabelPrefix(canvas *ui.Canvas, prefix string) (ui.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if strings.HasPrefix(hit.Label, prefix) || strings.Contains(hit.Label, prefix) {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

// TestCatalogFourStates 四态：未授权 / 未启用 / 数据缺失 / 正常。
func TestCatalogFourStates(t *testing.T) {
	cases := []struct {
		name       string
		view       View
		wantTrig   bool
		wantPanel  bool
		wantFooter int
		wantText   string
		why        string
	}{
		{
			name:     "未授权",
			view:     View{SessionID: "s1"}, // 投影/目录都未知：没有任何数据面
			wantTrig: false, wantPanel: false, wantFooter: 0,
			why: "目录未知（presentedCatalog === undefined）= 不渲染",
		},
		{
			name: "未启用",
			view: View{SessionID: "s1", CatalogKnown: true,
				ProjectionKnown: true, Projection: TeamProjection{Enabled: false}},
			wantTrig: false, wantPanel: false, wantFooter: 0,
			why: "目录已知但空、Team 关闭、footer 也没有内容 → visible=false",
		},
		{
			name: "数据缺失",
			view: View{SessionID: "s1", CatalogKnown: false,
				CatalogError: "session.projections 失败：连接被重置"},
			wantTrig: true, wantPanel: true, wantFooter: 1,
			wantText: zhCatalogLoadError,
			why:      "官方错误态（state==='error'）要能看见：容器渲染 + 错误行 + footer 仍 dispatch",
		},
		{
			name:     "正常",
			view:     catalogReadyView(),
			wantTrig: true, wantPanel: true, wantFooter: 1,
			wantText: zhCatalogSectionTitle,
			why:      "目录有两条：触发器显示计数，面板画行 + footer",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			probe := &footerProbe{}
			k, _ := catalogChain(t, p, probe)
			p.team.SetView(tc.view)
			p.OpenCatalog() // 哪怕强制展开，也不该比 visible 多画

			triggerCanvas := renderFromSlot(t, k, SlotSessionHeaderActions)
			// 触发器命中区的 aria 名：目录已知 = "N 个子智能体"；未知 = "子智能体会话"。
			_, hasTrigger := hitByLabelPrefix(triggerCanvas, "子智能体")
			if hasTrigger != tc.wantTrig {
				t.Fatalf("%s：触发器应存在=%v，实际=%v", tc.why, tc.wantTrig, hasTrigger)
			}

			panelCanvas := renderFromSlot(t, k, pluginapi.SlotShellOverlay)
			gotFooter := probe.count()
			if gotFooter != tc.wantFooter {
				t.Fatalf("%s：footer 槽 dispatch 次数应为 %d，实际 %d", tc.why, tc.wantFooter, gotFooter)
			}
			// 面板本体：用面板框的矩形判据（半径 = catalogMenuRadius 的盒子）。
			hasPanel := hasPanelRect(panelCanvas)
			if hasPanel != tc.wantPanel {
				t.Fatalf("%s：面板应存在=%v，实际=%v", tc.why, tc.wantPanel, hasPanel)
			}

			if tc.wantText != "" {
				found := false
				for _, text := range append(append([]ui.Text{}, triggerCanvas.Texts()...), panelCanvas.Texts()...) {
					if strings.Contains(text.Value, tc.wantText) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s：找不到文案 %q", tc.why, tc.wantText)
				}
			}

			// 不编 0：目录未知时不许出现"0 个子智能体"这类编出来的计数。
			if !tc.view.CatalogKnown {
				for _, text := range triggerCanvas.Texts() {
					if strings.Contains(text.Value, "0 个子智能体") {
						t.Fatalf("%s：目录未知却画出了编造的计数 %q", tc.why, text.Value)
					}
				}
			}
		})
	}
}

// TestCatalogMountPointFollowsDispatchedSlots 挂载点跟着**这一帧被 dispatch 的槽**走。
//
// 三种情形（见 uikernel.ShellTier.OfficialSlotsLive）：官方无桥 → header.actions；
// official+旧部件桥 / legacy → main.header。另一边必须一次都不画。
func TestCatalogMountPointFollowsDispatchedSlots(t *testing.T) {
	cases := []struct {
		name     string
		official bool
		bridge   bool
		wantFrom pluginapi.Slot
		other    pluginapi.Slot
	}{
		{"official 无桥", true, false, SlotSessionHeaderActions, pluginapi.SlotMainHeader},
		{"official + 旧部件桥", true, true, pluginapi.SlotMainHeader, SlotSessionHeaderActions},
		{"legacy", false, false, pluginapi.SlotMainHeader, SlotSessionHeaderActions},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(nil)
			k, tier := catalogChain(t, p)
			p.team.SetView(catalogReadyView())
			tier.Set(tc.official)
			tier.SetLegacyBridge(tc.bridge)

			canvas := renderFromSlot(t, k, tc.wantFrom)
			if _, ok := hitByLabelPrefix(canvas, "子智能体"); !ok {
				t.Fatalf("%q 这一帧应画出触发器（没画说明挂错槽）", tc.wantFrom)
			}
			other := renderFromSlot(t, k, tc.other)
			if _, ok := hitByLabelPrefix(other, "子智能体"); ok {
				t.Fatalf("%q 这一帧不该画触发器（会双画）", tc.other)
			}
		})
	}
}

// TestCatalogCountLabelIsOfficial 计数文案逐字对官方（`count.total.other`）。
func TestCatalogCountLabelIsOfficial(t *testing.T) {
	if got := catalogCountLabel(3); got != "3 个子智能体" {
		t.Fatalf("计数文案应为官方的「3 个子智能体」，得到 %q", got)
	}
	if !strings.Contains(zhCatalogCountTemplate, "个子智能体") {
		t.Fatalf("官方模板必须含「个子智能体」，得到 %q", zhCatalogCountTemplate)
	}
	if formatCount("{count} 个子智能体", 7) != "7 个子智能体" {
		t.Fatalf("formatCount 没把 {count} 换掉")
	}
}
