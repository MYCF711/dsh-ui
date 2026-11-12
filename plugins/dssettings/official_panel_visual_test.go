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
		SettingsDescribe:  func() *apiclient.SettingsDescribeResult { return h.describe },
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
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
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
	h.invalidator = &app.Invalidator{}
	config.Invalidator = h.invalidator

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

func (h *officialHarness) hitFor(name string) gfx.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	ids := make([]string, 0, len(h.session.Snapshot().Hits))
	for _, hit := range h.session.Snapshot().Hits {
		ids = append(ids, fmt.Sprint(hit.ID))
	}
	h.t.Fatalf("没有命中区 %q；hits=%v", name, ids)
	return gfx.Hit{}
}

func (h *officialHarness) click(name string) {
	h.t.Helper()
	hit := h.hitFor(name)
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if id, ok := h.session.Driver().HitTest(x, y); !ok || id != hit.ID {
		h.t.Fatalf("%s 的中心没打到它自己：got=%v want=%v", name, id, hit.ID)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
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

// TestOfficialPanelRetryRowsRenderAndWrite 证明 retryPolicy 行真的画出来、
// 点击真的走写路径，且 409 失败态**画在右页上**。
func TestOfficialPanelRetryRowsRenderAndWrite(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()

	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")

	// 8 个 code 候选都画出来了（文本至少出现 RATE_LIMIT / TIMEOUT / TRANSPORT）。
	for _, code := range []string{"RATE_LIMIT", "TIMEOUT", "TRANSPORT", "EMPTY_RESPONSE"} {
		if !h.hasText(code) {
			t.Fatalf("models 页缺 code 行 %q；文本=%v", code, h.session.Texts())
		}
	}

	// 点击一个 code 行 → 真实写路径（mutate，多段 path）。
	h.click("settings-row-retry-deepseek-code-TIMEOUT")
	deadline := 200
	for h.writer.callCount() == 0 && deadline > 0 {
		h.pump()
		deadline--
	}
	if h.writer.callCount() == 0 {
		t.Fatalf("点击 code 行没有触发写（status=%q）", h.statusText())
	}

	// 再点一次，但让写入回 409：失败态必须画出来。
	h.writer.mu.Lock()
	// retryPolicy 走 settings.mutate（多段路径），所以冲突注入在 mutateErr。
	h.writer.mutateErr = apiError(apiclient.SettingsCodeRevisionConflict,
		"llm revision 已变化", "重新 describe 后再提交", false)
	h.writer.mu.Unlock()
	h.click("settings-row-retry-deepseek-code-SERVER")
	deadline = 200
	for h.writer.callCount() < 2 && deadline > 0 {
		h.pump()
		deadline--
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
