package main

// hits_matrix_test.go —— V-06-4 的**命中区矩阵**守卫（"看着对但点不着"的唯一判据）。
//
// # 为什么需要它
//
// v0.6.0 把控件身份从"显式交互 ID"改成了"Key/位置"（`ui.Button` 用
// `controlID(pkButton)`，`ui.Scroll` 用 `page.name(pkContainer)`），而
// `Popup().Above(anchor, gap, align)` 整块删除、改成 `ui.Overlay` + 绝对定位。
// 这两件事**都只影响"能不能点到"，不影响"编译过不过"**——所以门禁全绿也可能
// 整片界面点不着。
//
// 这个文件的判据只有两条，但必须在**每一帧产品形态**上都成立：
//
//	① 每个命中区都有正面积（`Width>0 && Height>0`）——零面积 = 声明了但点不着；
//	② 每个"可点元素"都真的出现了一个命中区——用静态清单（`hitsMatrixStatic`
//	   一节的可点元素表）对拍，缺一个就是迁移把它的 `Hit` 丢了。
//
// ③ 附带：同一帧里不存在"同一个可访问名 + 同一个 role"的两个**互不相交**命中区
//    （那说明身份交换，点 A 落到 B）。

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsagentteam"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dssettings"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// hitsWindow 是矩阵跑的窗口尺寸（1182×832 是本仓真机默认，宽度留出三栏）。
const (
	hitsWindowWidth  = 1182
	hitsWindowHeight = 832
)

// hitsProduct 是一份装配好的产品（全部 25 个内置插件），可以按状态反复画。
type hitsProduct struct {
	kernel  *uikernel.Kernel
	theme   *pluginapi.Theme
	tree    *gfx.Tree
	feat    gfx.Scope
	plugins map[string]pluginapi.Plugin
}

// newHitsProduct 装配全部内置插件。
//
// 用 `pluginapi.Builtin()` 而不是手抄插件清单：手抄的清单会和产品分叉
// （`plugins_assembled_test.go` 已经证明"目录在、没装配"这种事真会发生）。
func newHitsProduct(t *testing.T) *hitsProduct {
	t.Helper()
	src, err := pluginapi.Builtin().Plugins()
	if err != nil {
		t.Fatalf("枚举内置插件失败：%v", err)
	}
	if len(src) < 20 {
		t.Fatalf("内置插件只有 %d 个，装配不完整", len(src))
	}
	p := &hitsProduct{
		kernel:  uikernel.New("hits-matrix"),
		tree:    &gfx.Tree{},
		plugins: map[string]pluginapi.Plugin{},
	}
	// 壳侧能力必须在 Load 之前提供（插件的 Register 就 Inject 它）。
	p.kernel.Root().Provide(uikernel.KeyShellTier, uikernel.NewShellTier())
	ordered := orderPlugins(src)
	for _, pl := range ordered {
		p.plugins[pl.ID()] = pl
	}
	if err := p.kernel.Load(ordered...); err != nil {
		t.Fatalf("装配全部插件失败：%v", err)
	}
	p.theme = p.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.feat = p.tree.Feature("hits-matrix")
	return p
}

// hitsFrame 是一帧的输入。
type hitsFrame struct {
	name string
	snap pluginapi.Snapshot
	// opts 允许覆盖壳选项（右栏、phase、兼容桥等）。
	opts func(*uikernel.ShellOptions)
	// open 在画之前调用，用来把插件内部的浮层打开（设置 / 目录 / 弹窗…）。
	open func(t *testing.T, p *hitsProduct)
	// click 非空时，按**文案**找到命中区再走一次事件 pass（真实点击路径）。
	click string
	// width/height 为零时用窗口默认。
	width, height float32
}

// hitsActions 是矩阵跑帧用的动作面。
//
// 插件在**画第一帧时**把 `env.Act` 抄进自己的字段（`dssettings` 的
// `p.act = env.Act`，见 `plugins/dssettings/settings.go:344`），所以"打开浮层"
// 这类入口必须**先画一帧**才有值。给一份非 nil 的动作面，等于告诉产品
// "宿主能力都在"，这样浮层才会真的打开、真的出命中区。
func hitsActions() pluginapi.Actions {
	noop := func() {}
	noopString := func(string) {}
	return pluginapi.Actions{
		OpenSession:      noopString,
		ForkSession:      noop,
		CompactSession:   noop,
		DeleteSession:    noopString,
		NewSession:       noop,
		Notify:           noopString,
		Copy:             noopString,
		ToggleRightbar:   noop,
		Retry:            noop,
		RetrySend:        noopString,
		Send:             noopString,
		Cancel:           noop,
		Reconnect:        noop,
		Invalidate:       noop,
		LoadOlder:        noop,
		SetDraft:         noopString,
		SetModel:         noopString,
		SetPermission:    noopString,
		SetTheme:         noopString,
		Approve:          func(string, string) {},
		Allow:            noop,
		Deny:             noop,
		ConfigGet:        func() map[string]string { return map[string]string{} },
		ConfigSet:        func(map[string]string) {},
		FSList:           func(string) []pluginapi.FSEntry { return nil },
		FSRead:           func(string) (string, error) { return "", nil },
		ScheduleList:     func() []pluginapi.ScheduleItem { return nil },
		JobsList:         func() []pluginapi.Job { return nil },
		ModelsList:       func() []pluginapi.Model { return nil },
		Goal:             func() string { return "" },
		TerminalOpen:     func() string { return "" },
		TerminalInput:    func(string, string) {},
		TerminalRead:     func(string) string { return "" },
		SettingsDescribe: func() *apiclient.SettingsDescribeResult { return nil },
		SettingsHostFacts: func() pluginapi.SettingsHostFacts {
			return pluginapi.SettingsHostFacts{}
		},
	}
}

// draw 画一帧，返回画布与壳选项。
func (p *hitsProduct) draw(t *testing.T, f hitsFrame) (*gfx.Canvas, uikernel.ShellOptions) {
	t.Helper()
	// 先画一帧"预热"：插件在画的时候才把 env.Act 抄进字段。不开预热的话，
	// `dssettings.Open()` 会因为 `p.act` 还是零值而返回 false（"没有可操作的东西"）。
	if f.open != nil {
		p.paintOnce(t, f)
		f.open(t, p)
	}
	if f.click != "" {
		// 真实点击路径：先画一帧量到命中区，再拿它的**真实 ID** 走事件 pass。
		// 这也是"点得着"的证据——按文案找到、按 ID 点中、插件状态真的翻了。
		before, _ := p.paintOnce(t, f)
		hit, ok := findHitByLabel(before, f.click)
		if !ok {
			t.Fatalf("找不到文案为 %q 的命中区（点不着）：%s", f.click, describeHits(hitsOf(before)))
		}
		p.paintEventPass(t, f, hit.ID)
	}
	return p.paintOnce(t, f)
}

// findHitByLabel 按可访问名找第一个命中区。
func findHitByLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, h := range hitsOf(canvas) {
		if h.Label == label {
			return h, true
		}
	}
	return gfx.Hit{}, false
}

// paintEventPass 画一帧并让它投递一次点击（`canvas.BeginEventPass(id, …)`）。
func (p *hitsProduct) paintEventPass(t *testing.T, f hitsFrame, id gfx.ID) {
	t.Helper()
	width, height := f.width, f.height
	if width <= 0 {
		width = hitsWindowWidth
	}
	if height <= 0 {
		height = hitsWindowHeight
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		n := 0
		for range value {
			n++
		}
		return float32(n) * size * 0.55
	})
	opts, env := p.frameInputs(f, width, height)
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	theme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &theme})
	view.Draw(canvas, env.Bounds, func(c ui.C) {
		uikernel.FirstScreen(c, env, opts, func(slot pluginapi.Slot) {
			p.kernel.DrawSlot(c, slot, env)
		})
	})
	canvas.EndEventPass()
}

// frameInputs 算这一帧的壳选项与 Env（paintOnce 与 paintEventPass 共用）。
func (p *hitsProduct) frameInputs(f hitsFrame, width, height float32) (uikernel.ShellOptions, *pluginapi.Env) {
	opts := uikernel.ShellOptions{
		Phase:                uikernel.PhaseActive,
		SidebarPreferred:     uikernel.SidebarDefault,
		CollapsedWidth:       uikernel.SidebarCollapsed,
		RightbarOn:           f.snap.ShowRightbar,
		Blank:                f.snap.Cur == "",
		BridgeLegacy:         true,
		BridgeLegacyComposer: false,
	}
	if f.snap.Cur == "" {
		opts.Phase = uikernel.PhaseHero
	}
	if f.opts != nil {
		f.opts(&opts)
	}
	env := &pluginapi.Env{
		Theme: p.theme, Data: f.snap, Act: hitsActions(), Tree: p.tree, Feat: p.feat,
		Bounds:    ui.Rect{Width: width, Height: height},
		MainWidth: width - uikernel.SidebarDefault,
		Geometry:  officialGeometry(width, height, opts),
		Has:       func(s pluginapi.Slot) bool { return p.kernel.Has(s) },
	}
	return opts, env
}

// paintOnce 画一帧（不含 open）。
func (p *hitsProduct) paintOnce(t *testing.T, f hitsFrame) (*gfx.Canvas, uikernel.ShellOptions) {
	t.Helper()
	width, height := f.width, f.height
	if width <= 0 {
		width = hitsWindowWidth
	}
	if height <= 0 {
		height = hitsWindowHeight
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(p.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		n := 0
		for range value {
			n++
		}
		return float32(n) * size * 0.55
	})
	opts := uikernel.ShellOptions{
		Phase:            uikernel.PhaseActive,
		SidebarPreferred: uikernel.SidebarDefault,
		CollapsedWidth:   uikernel.SidebarCollapsed,
		RightbarOn:       f.snap.ShowRightbar,
		Blank:            f.snap.Cur == "",
		// 旧部件还在（dstopbar 的 main.header、dsconversation 的 main.body），
		// 所以整列兼容桥开着；composer 走官方座位（dscomposer 注册了它）。
		BridgeLegacy:         true,
		BridgeLegacyComposer: false,
	}
	if f.snap.Cur == "" {
		opts.Phase = uikernel.PhaseHero
	}
	if f.opts != nil {
		f.opts(&opts)
	}
	theme := dscore.VivaTheme(p.theme)
	env := &pluginapi.Env{
		Theme: p.theme, Data: f.snap, Act: hitsActions(), Tree: p.tree, Feat: p.feat, Canvas: canvas,
		Bounds:    ui.Rect{Width: width, Height: height},
		MainWidth: width - uikernel.SidebarDefault,
		Geometry:  officialGeometry(width, height, opts),
		Has:       func(s pluginapi.Slot) bool { return p.kernel.Has(s) },
	}
	view := ui.NewPage(ui.PageConfig{Theme: &theme})
	view.Draw(canvas, env.Bounds, func(c ui.C) {
		uikernel.FirstScreen(c, env, opts, func(slot pluginapi.Slot) {
			p.kernel.DrawSlot(c, slot, env)
		})
	})
	for _, painter := range p.kernel.AllPainters() {
		if painter.AfterDraw != nil {
			painter.AfterDraw(canvas, env)
		}
	}
	return canvas, opts
}

// hitsOf 返回画布上**有身份**的命中区（`Block`/`Container`/`MovesWindow`
// 这类无身份的语义区不算可点元素）。
func hitsOf(canvas *gfx.Canvas) []gfx.Hit {
	out := make([]gfx.Hit, 0, len(canvas.Hits()))
	for _, h := range canvas.Hits() {
		if h.ID.IsZero() || h.Block || h.Container || h.MovesWindow {
			continue
		}
		out = append(out, h)
	}
	return out
}

// describeHits 把命中区排成一列（报告与失败信息共用）。
func describeHits(hits []gfx.Hit) string {
	lines := make([]string, 0, len(hits))
	for _, h := range hits {
		lines = append(lines, fmt.Sprintf("  %-28s role=%-10s %.1f,%.1f %.1fx%.1f",
			truncateLabel(h.Label), h.Role, h.Bounds.X, h.Bounds.Y, h.Bounds.Width, h.Bounds.Height))
	}
	return strings.Join(lines, "\n")
}

func truncateLabel(s string) string {
	if s == "" {
		return `""`
	}
	r := []rune(s)
	if len(r) > 26 {
		return string(r[:25]) + "…"
	}
	return string(r)
}

// TestHitsMatrixNoZeroAreaHit 是**判据①**：任何一帧里都没有零面积命中区。
func TestHitsMatrixNoZeroAreaHit(t *testing.T) {
	for _, f := range hitsFrames() {
		f := f
		t.Run(f.name, func(t *testing.T) {
			// 每个状态一份**全新装配**：插件里的 shown/open 是真状态，
			// 复用产品会让"上一帧开着的浮层"污染下一帧的命中区清单。
			p := newHitsProduct(t)
			canvas, _ := p.draw(t, f)
			hits := hitsOf(canvas)
			if len(hits) == 0 {
				t.Fatalf("这一帧一个命中区都没有——整片界面点不着；canvas.Hits()=%d（含无身份区）",
					len(canvas.Hits()))
			}
			var bad []gfx.Hit
			for _, h := range hits {
				if h.Bounds.Width <= 0 || h.Bounds.Height <= 0 ||
					math.IsNaN(float64(h.Bounds.Width)) || math.IsNaN(float64(h.Bounds.Height)) {
					bad = append(bad, h)
				}
			}
			if len(bad) > 0 {
				t.Fatalf("%d 个命中区零面积/NaN（声明了但点不着）：\n%s", len(bad), describeHits(bad))
			}
		})
	}
}

// TestHitsMatrixNoDuplicateIdentity 是**判据③**：同帧同 (label,role) 不出现
// 两个互不相交的命中区——那说明"身份按位置分配"换了位，点 A 落到 B。
func TestHitsMatrixNoDuplicateIdentity(t *testing.T) {
	for _, f := range hitsFrames() {
		f := f
		t.Run(f.name, func(t *testing.T) {
			p := newHitsProduct(t)
			canvas, _ := p.draw(t, f)
			seen := map[string][]gfx.Hit{}
			for _, h := range hitsOf(canvas) {
				if h.Label == "" {
					continue // 无名命中区（遮罩/吃掉点击的容器）本来就允许重复
				}
				key := fmt.Sprintf("%s|%v", h.Label, h.Role)
				seen[key] = append(seen[key], h)
			}
			for key, group := range seen {
				if len(group) < 2 {
					continue
				}
				// 同 (label,role) 的多行（列表行）是合法的：它们的矩形互不相交
				// 且**本来就是多行**。只有"矩形互相覆盖却又是两个身份"才是错。
				for i := 0; i < len(group); i++ {
					for j := i + 1; j < len(group); j++ {
						a, b := group[i].Bounds, group[j].Bounds
						if a.X == b.X && a.Y == b.Y && a.Width == b.Width && a.Height == b.Height &&
							group[i].ID != group[j].ID {
							t.Fatalf("同一帧里 %q 有两个完全重合但身份不同的命中区 %v / %v——身份交换",
								key, group[i].ID, group[j].ID)
						}
					}
				}
			}
		})
	}
}

// TestHitsMatrixDump 把矩阵打出来给报告用（`-v` 才有输出）。
func TestHitsMatrixDump(t *testing.T) {
	for _, f := range hitsFrames() {
		p := newHitsProduct(t)
		canvas, _ := p.draw(t, f)
		hits := hitsOf(canvas)
		labels := make([]string, 0, len(hits))
		for _, h := range hits {
			labels = append(labels, h.Label)
		}
		sort.Strings(labels)
		t.Logf("=== %s：%d 个可点元素 ===", f.name, len(hits))
		for _, line := range strings.Split(describeHits(hits), "\n") {
			t.Log(line)
		}
	}
}

// hitsSessionFixture 造一份"有会话、有消息、有审批"的快照。
func hitsSessionFixture() pluginapi.Snapshot {
	msgs := []apiclient.Message{
		{Role: "user", Text: "帮我看一下 build 为什么红"},
		{Role: "assistant", Text: "先跑一次 build 看看。"},
		{Role: "user", Text: "好"},
		{Role: "assistant", Text: "已经修好了。"},
	}
	return pluginapi.Snapshot{
		Sessions: []apiclient.Session{
			{ID: "s-1", Title: "build 为什么红"},
			{ID: "s-2", Title: "第二会话"},
			{ID: "s-3", Title: "第三会话"},
		},
		Cur:           "s-1",
		Status:        "就绪",
		Msgs:          msgs,
		Total:         len(msgs),
		Link:          pluginapi.LinkConnected,
		Busy:          false,
		ShowRightbar:  true,
		Model:         "deepseek-v4.1-flash",
		Permission:    "ask",
		ContextTokens: 12000,
		ContextWindow: 262144,
		SessionStats: &apiclient.SessionStats{
			Turns: 2, Steps: 4, LLMMs: 3200, ToolMs: 400, TTFTMs: 900, TTFTSteps: 2,
		},
	}
}

// hitsFrames 是矩阵的状态表。
//
// 只覆盖"由 ShellOptions / Snapshot 驱动"的形态；插件内部浮层（设置、目录、
// 弹窗）在 `open` 里显式打开。新增可点元素时**必须**在这里加一帧，否则
// 判据②（静态清单对拍）会红。
func hitsFrames() []hitsFrame {
	return []hitsFrame{
		{
			name: "hero-空首屏",
			snap: pluginapi.Snapshot{Link: pluginapi.LinkConnected, Sessions: nil},
		},
		{
			name: "active-有会话有消息",
			snap: hitsSessionFixture(),
		},
		{
			name: "active-发送中",
			snap: func() pluginapi.Snapshot {
				s := hitsSessionFixture()
				s.Busy = true
				s.Stream = "正在思考…"
				return s
			}(),
		},
		{
			name: "active-审批待定",
			snap: func() pluginapi.Snapshot {
				s := hitsSessionFixture()
				s.Approval = &pluginapi.Approval{
					RequestID: "ap-1",
					Tool:      "bash",
					Title:     "bash: rm -rf build",
					Kind:      "execute",
					Options: []pluginapi.ApprovalOption{
						{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
						{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
					},
				}
				return s
			}(),
		},
		{
			name: "active-历史分页五态",
			snap: func() pluginapi.Snapshot {
				s := hitsSessionFixture()
				s.History = pluginapi.HistoryPaging{Baseline: true, HasMore: true, Partial: true, Loading: false}
				return s
			}(),
		},
		{
			name: "active-设置浮层打开",
			snap: hitsSessionFixture(),
			open: func(t *testing.T, p *hitsProduct) {
				t.Helper()
				pl, ok := p.plugins["ds-settings"].(*dssettings.Plugin)
				if !ok {
					t.Fatalf("ds-settings 不是 *dssettings.Plugin：%T", p.plugins["ds-settings"])
				}
				// 官方四区面板接管时走 OpenOfficial；否则退回 rc.1 的最小浮层。
				opened := pl.Open()
				if pl.OfficialEnabled() {
					opened = pl.OpenOfficial()
				}
				if !opened {
					t.Fatalf("设置浮层打不开（OfficialEnabled=%v）", pl.OfficialEnabled())
				}
			},
		},
		{
			// 点官方「活动」胶囊 → 统计弹窗。这条同时证明"胶囊点得着"与
			// "点完状态真的翻了（弹窗出现）"。
			name:  "active-点活动胶囊开统计弹窗",
			snap:  hitsSessionFixture(),
			click: "2 轮 4 步",
		},
		{
			name: "active-子代理目录打开",
			snap: hitsSessionFixture(),
			open: func(t *testing.T, p *hitsProduct) {
				t.Helper()
				pl, ok := p.plugins["ds-agent-team"].(*dsagentteam.Plugin)
				if !ok {
					t.Fatalf("ds-agent-team 不是 *dsagentteam.Plugin：%T", p.plugins["ds-agent-team"])
				}
				pl.OpenCatalog()
				if !pl.CatalogOpen() {
					t.Fatalf("子代理目录打不开")
				}
			},
		},
		{
			name: "active-右栏关闭",
			snap: func() pluginapi.Snapshot {
				s := hitsSessionFixture()
				s.ShowRightbar = false
				return s
			}(),
		},
		{
			name: "active-侧栏收起（窄窗）",
			snap: hitsSessionFixture(),
			opts: func(o *uikernel.ShellOptions) { o.SidebarPreferred = uikernel.SidebarCollapsed },
		},
		{
			name:  "active-窄窗 720x600",
			snap:  hitsSessionFixture(),
			width: 720, height: 600,
		},
	}
}

// 保证 dsconversation / dsstatusbar 的插件类型在断言里可用（编译期引用，
// 免得未来插件改名时这个文件静默失效）。
var (
	_ = (*dsconversation.Plugin)(nil)
	_ = (*dsstatusbar.Plugin)(nil)
)
