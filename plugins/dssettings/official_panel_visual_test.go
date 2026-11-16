//go:build darwin || windows || linux

// official_panel_visual_test.go —— 官方四区设置面板的**真实绘制**证据。
//
// 走真实 Viva：真 Paint（经真 dsshell 的 shell.overlay 槽）→ 真 HitTest →
// 真 Click → 再 Paint。判据不是「图里有东西」：
//
//   - 5 个 section 的导航 cell 都有**真实命中区**（不是只画了文字）；
//   - 卡片几何来自 official_grid 的求解结果（800×800、导航列 188），
//     且与命中区矩形逐像素一致；
//   - 点导航 cell 真的切页（右页内容跟着换）；
//   - 写失败态（409 revision_conflict）真的**画在右页上**（不是只存了个字段）；
//   - Escape 真的关得掉（模态层不是装饰）。
//
// 截图只在断言通过之后落盘（SETTINGS_CAPTURE_DIR 非空时保留）。
package dssettings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// raceSafeWriter 是线程安全的写入替身（点击走 goroutine，必须加锁）。
type raceSafeWriter struct {
	mu        sync.Mutex
	calls     []writeCall
	mutates   []mutateCall
	setErr    error
	mutateErr error
	section   apiclient.SettingsSectionView
}

func (w *raceSafeWriter) SettingsFormSet(_ context.Context, ns string, path []string, value any, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, writeCall{ns: ns, path: path, value: value, expectedRev: expectedRevision})
	if w.setErr != nil {
		return apiclient.SettingsSectionView{}, w.setErr
	}
	return w.section, nil
}

func (w *raceSafeWriter) SettingsFormReset(_ context.Context, ns string, path []string, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, writeCall{ns: ns, path: path, expectedRev: expectedRevision})
	return w.section, nil
}

func (w *raceSafeWriter) SettingsMutate(_ context.Context, ns string, ops []apiclient.SettingsOp, expectedRevision *int) (apiclient.SettingsNamespaceView, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mutates = append(w.mutates, mutateCall{ns: ns, ops: ops, expectedRev: expectedRevision})
	if w.mutateErr != nil {
		return apiclient.SettingsNamespaceView{}, w.mutateErr
	}
	return apiclient.SettingsNamespaceView{NS: ns, Revision: 1}, nil
}

func (w *raceSafeWriter) callCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.calls) + len(w.mutates)
}

func (w *raceSafeWriter) lastCall() (writeCall, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.calls) == 0 {
		return writeCall{}, false
	}
	return w.calls[len(w.calls)-1], true
}

type officialHarness struct {
	t           *testing.T
	p           *Plugin
	kernel      *uikernel.Kernel
	theme       *pluginapi.Theme
	tree        *gfx.Tree
	feat        gfx.Scope
	session     *headless.Session
	invalidator *app.Invalidator
	writer      *raceSafeWriter
	describe    *apiclient.SettingsDescribeResult
	status      string
	mu          sync.Mutex
	// inventory 是「内置插件」页的数据面（nil = 宿主没接这条能力）。
	inventory *apiclient.PluginInventoryResult
	// inventoryRefresh 记录「重试」被点了几次（插件页的刷新动作）。
	inventoryRefresh int
	// roster 是「Agent 预设」页的数据面（nil = 宿主没接 agentPresets.list）。
	roster *apiclient.AgentPresetRoster
	// agentPresetRefresh 记录名册被重读了几次（写成功后壳会重拉一次）。
	agentPresetRefresh int
	// modelsSnapshot 是 Models 独立页的三路宿主快照；nil 走真实 llm 设置回退。
	modelsSnapshot *apiclient.ModelsPageSnapshot
}

func openOfficialHarness(t *testing.T, width, height int) *officialHarness {
	t.Helper()
	h := &officialHarness{t: t, writer: &raceSafeWriter{section: apiclient.SettingsSectionView{NS: "permission", Revision: 9}}}
	h.describe = describeFixture()
	h.p = &Plugin{}
	h.kernel = uikernel.New("official-settings")
	if err := h.kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, h.p); err != nil {
		t.Fatal(err)
	}
	h.theme = h.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)

	facts := pluginapi.SettingsHostFacts{
		SidebarRightTabs:  true,
		SessionLogServed:  true,
		ClientVersion:     "0.2.2-rc.2",
		PermissionCatalog: true,
		TranscriptView:    "standard",
		CredentialStored:  true,
	}
	act := pluginapi.Actions{
		// 加锁读：用例会在会话起来之后换 describe（例如 #3 那条要真的发布
		// agent-preset-registry 命名空间），而读点在绘制线程上——不加锁就是一条
		// "本机不炸、-race 才炸"的竞态。与 PluginInventory 的读法保持一致。
		SettingsDescribe: func() *apiclient.SettingsDescribeResult {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.describe
		},
		SettingsHostFacts: func() pluginapi.SettingsHostFacts { return facts },
		SettingsFormSet: func(ns string, path []string, value any, expectedRevision int) (apiclient.SettingsSectionView, error) {
			var rev *int
			if expectedRevision > 0 {
				rev = &expectedRevision
			}
			return h.writer.SettingsFormSet(context.Background(), ns, path, value, rev)
		},
		SettingsFormReset: func(ns string, path []string, expectedRevision int) (apiclient.SettingsSectionView, error) {
			var rev *int
			if expectedRevision > 0 {
				rev = &expectedRevision
			}
			return h.writer.SettingsFormReset(context.Background(), ns, path, rev)
		},
		SettingsMutate: func(ns string, ops []apiclient.SettingsOp, expectedRevision int) (apiclient.SettingsNamespaceView, error) {
			var rev *int
			if expectedRevision > 0 {
				rev = &expectedRevision
			}
			return h.writer.SettingsMutate(context.Background(), ns, ops, rev)
		},
		Notify: func(msg string) {
			h.mu.Lock()
			h.status = msg
			h.mu.Unlock()
			h.invalidator.Invalidate()
		},
		// 「内置插件」页的数据面：同步返回这一帧的替身清单（与真壳同一个形状）。
		PluginInventory: func() *apiclient.PluginInventoryResult {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.inventory
		},
		PluginInventoryRefresh: func() {
			h.mu.Lock()
			h.inventoryRefresh++
			h.mu.Unlock()
		},
		// 「Agent 预设」页的数据面：与真壳同一个形状（同步返回这一帧的名册）。
		AgentPresetRoster: func() *apiclient.AgentPresetRoster {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.roster
		},
		AgentPresetRefresh: func() {
			h.mu.Lock()
			h.agentPresetRefresh++
			h.mu.Unlock()
			h.invalidator.Invalidate()
		},
		ModelsPageSnapshot: func() *apiclient.ModelsPageSnapshot {
			h.mu.Lock()
			defer h.mu.Unlock()
			return h.modelsSnapshot
		},
	}

	h.tree = &gfx.Tree{}
	h.feat = h.tree.Feature("official-settings")
	pageTheme := dscore.VivaTheme(h.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "official-settings", Width: width, Height: height, Tree: h.tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: h.theme, Tree: h.tree, Feat: h.feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Act: act,
			Has: h.kernel.Has,
		}
		canvas.Fill(bounds, h.theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			h.kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	config.OnKey = func(ev gfx.KeyEvent) gfx.EventResult {
		if h.kernel.HandleKey(ev) {
			h.invalidator.Invalidate()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}
	config.OnScroll = func(ev gfx.ScrollEvent) gfx.EventResult { return view.Wheel(ev) }
	h.invalidator = &app.Invalidator{}
	config.Invalidator = h.invalidator
	// 文本输入必须登记（与产品壳 `newConfig` 同一条聚合路径），否则搜索框收不到键盘。
	config.TextInput = ui.Fields(h.p.Fields()...)

	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *officialHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *officialHarness) close() { h.session.Close() }

func (h *officialHarness) findHit(name string) (gfx.Hit, bool) {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func (h *officialHarness) hitFor(name string) gfx.Hit {
	h.t.Helper()
	if hit, ok := h.findHit(name); ok {
		return hit
	}
	ids := make([]string, 0, len(h.session.Snapshot().Hits))
	for _, hit := range h.session.Snapshot().Hits {
		ids = append(ids, fmt.Sprint(hit.ID))
	}
	if section, ok := h.p.PanelModel().Section(h.p.ActiveSection()); ok {
		for _, row := range section.Rows {
			if hit, found := h.findHit("settings-row-" + row.ID); found {
				h.t.Logf("行命中 %q bounds=%+v", row.ID, hit.Bounds)
			}
		}
	}
	h.t.Fatalf("没有命中区 %q；hits=%v；texts=%v", name, ids, h.session.Texts())
	return gfx.Hit{}
}

func (h *officialHarness) click(name string) {
	h.t.Helper()
	hit := h.hitFor(name)
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	// 窗口外的点不该"点了没反应"：命中区可能因为滚动/裁剪在树里仍在、但按不到。
	// 不先判这一条的话，失败会表现成"动作没触发"，查半天查不到是坐标的锅。
	if snap := h.session.Snapshot(); x < 0 || y < 0 || x >= float32(snap.Width) || y >= float32(snap.Height) {
		h.t.Fatalf("%s 的中心 (%v,%v) 在窗口 %dx%d 之外——命中断区被裁剪，点击落不到它身上",
			name, x, y, snap.Width, snap.Height)
	}
	if id, ok := h.session.Driver().HitTest(x, y); !ok || id != hit.ID {
		h.t.Fatalf("%s 的中心没打到它自己：got=%v want=%v", name, id, hit.ID)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// clickScrolled 在真实滚动容器里把长 Models 卡的行滚到可见后再点击；
// 每轮都 pump，绝不拿未布局的旧命中区断言。
func (h *officialHarness) clickScrolled(name string) {
	h.t.Helper()
	for i := 0; i < 40; i++ {
		if hit, ok := h.findHit(name); ok {
			x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			snap := h.session.Snapshot()
			if x >= 0 && y >= 0 && x < float32(snap.Width) && y < float32(snap.Height) {
				if id, hitOK := h.session.Driver().HitTest(x, y); hitOK && id == hit.ID {
					h.click(name)
					return
				}
			}
		}
		snap := h.session.Snapshot()
		if err := h.session.Scroll(gfx.ScrollEvent{X: float32(snap.Width) * 0.75, Y: float32(snap.Height) / 2, DeltaY: 180}); err != nil {
			h.t.Fatal(err)
		}
		h.pump()
	}
	h.click(name)
}

// agentPresetRefreshCount 读"名册重读被触发了几次"（加锁：写请求在别的 goroutine 里）。
func (h *officialHarness) agentPresetRefreshCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.agentPresetRefresh
}

// waitWriterCalls 等到写请求达到 want 次；等待期间继续 pump。
//
// 为什么不能点完立刻断言：卡片点击走 `go applyAgentPresetDefault(...)`
// （产品里本来就异步——绘制线程不能等 HTTP）。断言跑在测试 goroutine 上，
// 新 goroutine 未必已经被调度：本机 150 次里有 1~2 次、CI 上更容易输掉这个
// 竞态（2026-11-15 CI run 43622431919：`点了卡片却没发出任何写请求`）。
// 这与 official_panel_visual_test.go 里 retryPolicy 那两处的既有写法同一套：
// pump 到条件成立，超时才失败——断言本身一点没放松。
func waitWriterCalls(h *officialHarness, want int) bool {
	for i := 0; i < 400 && h.writer.callCount() < want; i++ {
		h.pump()
	}
	return h.writer.callCount() >= want
}

func (h *officialHarness) hasText(want string) bool {
	return h.session.HasText(want)
}

func (h *officialHarness) statusText() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status
}

// TestOfficialPanelRendersFiveSections 是主证据：5 个 section 的导航 cell 都
// 有真实命中区，卡片几何与网格求解结果一致。
func TestOfficialPanelRendersFiveSections(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	if h.p.OfficialShown() {
		t.Fatal("装配完不该自己打开")
	}
	// Shift+F2 打开（真实按键 → kernel.HandleKey → 官方面板接管）。
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatalf("Shift+F2 没打开官方面板（status=%q）", h.statusText())
	}

	// —— 结构：5 个导航 cell 都有命中区。
	for _, id := range []string{"account", "general", "models", "plugins", "agent-presets"} {
		hit := h.hitFor("settings-nav-" + id)
		if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Fatalf("导航 cell %s 的命中区 = %+v", id, hit.Bounds)
		}
	}
	// 关闭按钮也有命中区。
	h.hitFor("settings-close")

	// —— 几何：卡片 800×800 居中，导航列 188。
	layout := h.p.PanelLayout()
	wantW, wantH := float32(800), float32(832-2*PanelOverlayTop)
	if layout.Width != wantW || layout.Height != wantH {
		t.Fatalf("卡片 = %v×%v，官方 800×min(800, 832-48) = %v×%v", layout.Width, layout.Height, wantW, wantH)
	}
	if layout.X != (1182-wantW)/2 || layout.Y != (832-wantH)/2 {
		t.Fatalf("卡片位置 = (%v,%v)，应居中", layout.X, layout.Y)
	}
	navArea, ok := layout.Area("nav")
	if !ok || navArea.Width != 188 {
		t.Fatalf("导航列 = %+v，官方 188px", navArea)
	}
	optionsArea, _ := layout.Area("options")
	if optionsArea.Width != 612 {
		t.Fatalf("内容列 = %v，官方 1fr = 612", optionsArea.Width)
	}
	// 命中区矩形必须与网格求解一致（导航 cell 落在左列）。
	hit := h.hitFor("settings-nav-general")
	if hit.Bounds.X < layout.X || hit.Bounds.X > layout.X+188 {
		t.Fatalf("导航 cell 的 X = %v 不在左列 [%v,%v]", hit.Bounds.X, layout.X, layout.X+188)
	}

	// —— 默认页是 account（官方取 ledger 第一项）。
	if got := h.p.ActiveSection(); got != "account" {
		t.Fatalf("默认 section = %q，官方取 ledger 第一项 account", got)
	}

	// —— 切到 general：官方 14 行（含「工作步骤展示」三行）。
	h.click("settings-nav-general")
	if got := h.p.ActiveSection(); got != "general" {
		t.Fatalf("点导航后 section = %q", got)
	}
	for _, want := range []string{"权限", "语言", "外观", "字号大小", "工作步骤展示", "开发者模式", "快捷键", "当前版本"} {
		if !h.hasText(want) {
			t.Fatalf("general 页缺标题 %q；文本=%v", want, h.session.Texts())
		}
	}
	general, _ := h.p.PanelModel().Section("general")
	if len(general.Rows) != 14 {
		t.Fatalf("general 可见行 = %d，官方 14", len(general.Rows))
	}

	// 截图只在断言通过之后落盘（**面板开着时**——关掉之后再截就是黑的）。
	if dir := os.Getenv("SETTINGS_CAPTURE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "official-settings-1182x832.png")
		if err := h.session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：%v", err)
		}
		t.Logf("SETTINGS_CAPTURE path=%s", path)
	}

	// —— Escape 关闭（模态层真的接管）。
	if err := h.session.Driver().Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if h.p.OfficialShown() {
		t.Fatal("Escape 没关掉官方面板")
	}

}

// TestOfficialModelsDirectoryAndDefaultWrite 用真实三路快照证明 provider join、
// 按 provider 分组的模型目录与默认模型选择都落到可点击页面。
func TestOfficialModelsDirectoryAndDefaultWrite(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.modelsSnapshot = modelsSnapshotFixture()
	for index := range h.describe.Sections {
		if h.describe.Sections[index].NS == apiclient.SettingsNSModel {
			h.describe.Sections[index].Fields[0].Current = "zeta-1"
		}
	}
	h.mu.Unlock()
	h.pump()
	h.pump()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	for i := 0; i < 20 && !h.hasText("Zeta One"); i++ {
		h.pump()
	}
	for _, want := range []string{"Zeta configurable", "Registered only", "模型目录 · Zeta", "Zeta One", "DeepSeek Chat"} {
		if !h.hasText(want) {
			t.Fatalf("Models 页缺真实目录文本 %q；texts=%v", want, h.session.Texts())
		}
	}

	h.click("settings-row-model-default")
	if !waitWriterCalls(h, 1) {
		t.Fatal("点击默认模型选择器没有触发 settings.form.set")
	}
	for i := 0; i < 400 && h.p.WriteState().Kind != WriteSaved; i++ {
		h.pump()
	}
	call, ok := h.writer.lastCall()
	if !ok || call.value != "zeta-2" {
		t.Fatalf("默认模型写入 = %+v, ok=%v；应从 zeta-1 移到 zeta-2", call, ok)
	}
	if strings.Join(call.path, ".") != "defaultModel" || call.expectedRev == nil || *call.expectedRev != 3 {
		t.Fatalf("默认模型写路径/revision 错：%+v", call)
	}
}

// TestOfficialPanelRetryRowsRenderAndWrite 证明 retryPolicy 行真的画出来；
// 字段点击只改草稿，Apply 才发一次原子 mutate，409 失败态画在右页且草稿保留。
func TestOfficialPanelRetryRowsRenderAndWrite(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	h.click("settings-row-provider-edit-deepseek")

	// 8 个 code 候选都画出来了（文本至少出现 RATE_LIMIT / TIMEOUT / TRANSPORT）。
	for _, code := range []string{"RATE_LIMIT", "TIMEOUT", "TRANSPORT", "EMPTY_RESPONSE"} {
		if !h.hasText(code) {
			t.Fatalf("models 页缺 code 行 %q；文本=%v", code, h.session.Texts())
		}
	}

	h.clickScrolled("settings-row-retry-deepseek-code-RATE_LIMIT")
	for i := 0; i < 20; i++ {
		h.pump()
	}
	if got := h.writer.callCount(); got != 0 {
		t.Fatalf("字段点击必须只改草稿，却在 Apply 前写了 %d 次", got)
	}
	models, _ := h.p.PanelModel().Section("models")
	if _, ok := models.Row("retry-deepseek-apply"); !ok {
		t.Fatal("打开 provider 卡后缺 Apply 行")
	}
	h.clickScrolled("settings-row-retry-deepseek-apply")
	if !waitWriterCalls(h, 1) {
		t.Fatalf("Apply 没有触发写（status=%q）", h.statusText())
	}
	for i := 0; i < 400 && h.p.WriteState().Kind != WriteSaved; i++ {
		h.pump()
	}
	if h.p.WriteState().Kind != WriteSaved {
		t.Fatalf("第一次 Apply 未进入已保存：%+v", h.p.WriteState())
	}

	// 再改一次并注入 409：Apply 后失败态必须画出来，编辑卡不能消失。
	models, _ = h.p.PanelModel().Section("models")
	providerRow, _ := models.Row("provider-edit-deepseek")
	h.p.activateRow(providerRow)
	h.pump()
	for i := 0; i < 20; i++ {
		models, _ = h.p.PanelModel().Section("models")
		if _, ok := models.Row("retry-deepseek-apply"); ok {
			break
		}
		h.pump()
	}
	if _, ok := models.Row("retry-deepseek-apply"); !ok {
		t.Fatalf("第二次点击 Edit 未打开编辑卡；texts=%v", h.session.Texts())
	}
	h.clickScrolled("settings-row-retry-deepseek-code-TIMEOUT")
	h.writer.mu.Lock()
	h.writer.mutateErr = apiError(apiclient.SettingsCodeRevisionConflict,
		"llm revision 已变化", "重新 describe 后再提交", false)
	h.writer.mu.Unlock()
	h.clickScrolled("settings-row-retry-deepseek-apply")
	if !waitWriterCalls(h, 2) {
		t.Fatal("第二次 Apply 没有触发写")
	}
	for i := 0; i < 100 && h.p.WriteState().Kind != WriteConflict; i++ {
		h.pump()
	}
	h.pump()
	h.pump()

	state := h.p.WriteState()
	if state.Kind != WriteConflict {
		t.Fatalf("写状态 = %q，409 应渲染成冲突", state.Kind)
	}
	found := false
	for _, text := range h.session.Texts() {
		if strings.Contains(text, "冲突") && strings.Contains(text, "revision") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("冲突文案没画到右页上；文本=%v", h.session.Texts())
	}
	models, ok := h.p.PanelModel().Section("models")
	if !ok {
		t.Fatal("冲突后 Models 页消失")
	}
	if _, ok = models.Row("retry-deepseek-apply"); !ok {
		t.Fatal("冲突后编辑卡被关闭，用户草稿丢失")
	}
}

// TestOfficialPanelUnknownRendersAsUnknown 证明「宿主没给字段 → 画未知」。
func TestOfficialPanelUnknownRendersAsUnknown(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	// 只留 permission，其余命名空间都缺失。
	h.describe.Namespaces = h.describe.Namespaces[:2]
	h.describe.Namespaces = h.describe.Namespaces[1:] // 只剩 permission
	for i := range h.describe.Sections {
		if h.describe.Sections[i].NS != apiclient.SettingsNSPermission {
			h.describe.Sections = h.describe.Sections[:i]
			break
		}
	}

	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-general")

	if !h.hasText("未知") {
		t.Fatalf("缺失命名空间必须画「未知」；文本=%v", h.session.Texts())
	}
	general, _ := h.p.PanelModel().Section("general")
	for _, row := range general.Rows {
		if row.ID == "permission" || row.ID == "current-version" {
			continue
		}
		if row.Known {
			t.Fatalf("%s 在宿主没给数据时不该有真值", row.ID)
		}
	}
}

// TestOpenRoutesToOfficialPanel 守住"打开设置真的会画东西"。
//
// 2026-11-13 真机复现的缺陷：`Open()` 只置旧的 `p.shown`，而
//   - 旧浮层的 `draw` 在 `OfficialEnabled()` 为真时**直接 return**；
//   - 官方面板的 `drawOfficialPanel` 又要求 `o.open`（只有 `OpenOfficial()` 会置）。
//
// 于是两边都不画 —— 日志打「设置浮层已打开」，截图是**整页空白**。
// 产品路径（Shift+F2 / 侧栏设置按钮 / `ui.settings.open` 服务）全走 `Open()`，所以这是产品缺陷，
// 不是调试开关的问题。
func TestOpenRoutesToOfficialPanel(t *testing.T) {
	h := openOfficialHarness(t, 1180, 800)
	if !h.p.OfficialEnabled() {
		t.Fatal("夹具前提：官方四区面板应已接管（有 describe 数据面）")
	}
	if !h.p.Open() {
		t.Fatal("官方接管时 Open() 要返回 true")
	}
	if !h.p.OfficialShown() {
		t.Fatal("Open() 必须打开**官方**面板（OfficialShown）；只置旧 p.shown 就是那个整页空白缺陷")
	}
	h.pump()

	// 关的时候两个状态都要回去，否则下次开还是空白。
	h.p.Close()
	if h.p.OfficialShown() {
		t.Fatal("Close() 之后官方面板必须是关的")
	}
	if h.p.Shown() {
		t.Fatal("Close() 之后旧浮层也必须是关的（避免残留态）")
	}
}
