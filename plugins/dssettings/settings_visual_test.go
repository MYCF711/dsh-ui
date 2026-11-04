//go:build darwin || windows || linux

// settings_visual_test.go —— 设置浮层的**真实绘制/命中/点击**证据。
//
// 走的是真实 Viva：真 Paint（经真 dsshell 的 shell.overlay 槽）→ 真 HitTest →
// 真 Click → 再 Paint。两个尺寸各一遍（1182×832 / 1920×1032）。
//
// 判据不是"图里有东西"：
//   - 行是**按真实命中区 id** 找到并点的，不是按坐标猜的
//   - 点「浅色」之后，**主题对象就地换了**（和 cmd/app 的 applyPendingTheme 同一条路），
//     同一张截图的平均亮度必须从深色档跳到浅色档——这一步证明"运行时立刻生效"
//   - 未知的当前值不许有 ✓：初始 cfg 为空 → 外观一行都不选中
//
// 截图只在断言通过之后落盘（SETTINGS_CAPTURE_DIR 非空时保留，否则也留在临时目录）。
package dssettings

import (
	"fmt"
	"image/png"
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
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	settingsSessionID = "sess-settings-evidence"
	settingsTitle     = "设置浮层证据"
	settingsCWD       = `C:\work\demo`
)

type settingsHarness struct {
	t           *testing.T
	p           *Plugin
	kernel      *uikernel.Kernel
	theme       *pluginapi.Theme
	data        pluginapi.Snapshot
	tree        *ui.Tree
	feat        ui.Scope
	session     *headless.Session
	invalidator *app.Invalidator

	cfg    map[string]string
	act    pluginapi.Actions
	mu     sync.Mutex
	got    [3]string // theme / model / permission 各自最后收到的值
	called [3]int    // 各自被调用了几次
}

// actions 造壳在这一帧给的动作。SetTheme 的实现**照抄 cmd/app**：
// 就地改同一个 *Theme 对象（换对象会让所有持有者的指针失效）。
func (h *settingsHarness) buildActions() pluginapi.Actions {
	models := []pluginapi.Model{
		{ID: "group/deepseek-v4.1", OwnedBy: "group"},
		{ID: "b", OwnedBy: "local"},
	}
	return pluginapi.Actions{
		ConfigGet: func() map[string]string { return h.cfg },
		SetTheme: func(mode string) {
			h.mu.Lock()
			h.got[0], h.called[0] = mode, h.called[0]+1
			h.mu.Unlock()
			switch mode {
			case "dark":
				*h.theme = *dscore.Dark()
			case "light":
				*h.theme = *dscore.Light()
			}
			h.invalidator.Invalidate()
		},
		ModelsList: func() []pluginapi.Model { return models },
		SetModel: func(id string) {
			h.mu.Lock()
			h.got[1], h.called[1] = id, h.called[1]+1
			h.mu.Unlock()
			h.invalidator.Invalidate()
		},
		SetPermission: func(perm string) {
			h.mu.Lock()
			h.got[2], h.called[2] = perm, h.called[2]+1
			h.mu.Unlock()
			h.invalidator.Invalidate()
		},
	}
}

func openSettingsHarness(t *testing.T, width, height int) *settingsHarness {
	t.Helper()
	h := &settingsHarness{t: t}
	h.cfg = map[string]string{}
	h.data = pluginapi.Snapshot{
		Cur:        settingsSessionID,
		CWD:        settingsCWD,
		Model:      "b",
		Permission: "ask",
		Sessions:   []apiclient.Session{{ID: settingsSessionID, Title: settingsTitle}},
	}
	h.p = &Plugin{}
	// 真实装配：ds-core（令牌/图元）→ ds-shell（三栏 + overlay 槽）→ 本插件。
	// 插件顺序由 Requires 拓扑决定，和产品里同一个装配器。
	h.kernel = uikernel.New("settings-visual")
	if err := h.kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, h.p); err != nil {
		t.Fatal(err)
	}
	h.theme = h.kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	h.act = h.buildActions()

	h.tree = &ui.Tree{}
	h.feat = h.tree.Feature("settings-visual")
	var view flex.View
	config := app.Config{
		Title: "settings-visual", Width: width, Height: height, Tree: h.tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		sidebar := float32(280)
		env := &pluginapi.Env{
			Theme: h.theme, Data: h.data, Tree: h.tree, Feat: h.feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width - sidebar,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: sidebar,
				MainWidth: size.Width - sidebar,
			},
			Scroll: view.ScrollState,
			Act:    h.act,
			Has:    h.kernel.Has,
		}
		canvas.Fill(bounds, h.theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(h.theme), bounds, func(f *flex.Frame) {
			h.kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	config.OnKey = func(ev ui.KeyEvent) ui.EventResult {
		if h.kernel.HandleKey(ev) {
			// 和壳一样：**必须**显式 Invalidate（见 cmd/app 的 OnKey 注释）。
			h.invalidator.Invalidate()
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}
	h.invalidator = &app.Invalidator{}
	config.Invalidator = h.invalidator

	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	h.session = session
	h.pump() // 第一帧：插件记住 Actions
	h.pump()
	return h
}

func (h *settingsHarness) pump() {
	h.t.Helper()
	h.invalidator.Invalidate()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *settingsHarness) close() { h.session.Close() }

// hitFor 找一个真实命中区（Paint 之后由 HitTest 登记）。找不到直接红。
func (h *settingsHarness) hitFor(key string) ui.Hit {
	h.t.Helper()
	id := h.tree.ID(h.feat, "settings-"+key)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("没有命中区 id=%v（key=%s）；hits=%+v", id, key, h.session.Snapshot().Hits)
	return ui.Hit{}
}

// clickRow 走真实 HitTest + Click：先用点问"这里是哪个 id"，再点它。
func (h *settingsHarness) clickRow(key string) {
	h.t.Helper()
	hit := h.hitFor(key)
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if id, ok := h.session.Driver().HitTest(x, y); !ok || id != hit.ID {
		h.t.Fatalf("%s 的命中区中心没打到它自己：got=%v ok=%v want=%v", key, id, ok, hit.ID)
	}
	if err := h.session.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

func (h *settingsHarness) gotAt(i int) (string, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.got[i], h.called[i]
}

// countText 数某个**完整文本**画了几次（✓ 的个数就是"有几行是当前值"）。
func (h *settingsHarness) countText(text string) int {
	n := 0
	for _, value := range h.session.Texts() {
		if value == text {
			n++
		}
	}
	return n
}

func (h *settingsHarness) screenshot(width, height int, name, dir string) string {
	h.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%dx%d.png", name, width, height))
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：%s info=%v err=%v", path, info, err)
	}
	h.t.Logf("SETTINGS_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

// meanLuma 是一张图的平均亮度（0–255），用来证明"深浅真的换了"。
func meanLuma(t *testing.T, path string) float64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	var sum float64
	var n int
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(bl>>8)
			n++
		}
	}
	if n == 0 {
		t.Fatal("截图是空的")
	}
	return sum / float64(n)
}

// TestSettingsOverlayTwoSizePaintHitTestClick 在两个尺寸上各走一遍：
// 关闭态 → Shift+F2 打开 → 真实命中/点击主题行（深浅换色）→ 点模型/权限 →
// Esc 关闭。每一步都断言，截图只记录已验过的画面。
func TestSettingsOverlayTwoSizePaintHitTestClick(t *testing.T) {
	dir := os.Getenv("SETTINGS_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct {
		name          string
		width, height int
	}{{"1182x832", 1182, 832}, {"1920x1032", 1920, 1032}} {
		t.Run(size.name, func(t *testing.T) {
			runSettingsEvidence(t, size.width, size.height, dir)
		})
	}
}

func runSettingsEvidence(t *testing.T, width, height int, dir string) {
	h := openSettingsHarness(t, width, height)
	defer h.close()

	// —— 关闭态：不该有"设置"卡片，也不该有任何设置行命中区。
	if h.p.Shown() {
		t.Fatal("装配完不该自己打开")
	}
	if h.session.HasText("设置") {
		t.Fatalf("没打开就画了设置卡片：%q", h.session.Texts())
	}

	// —— Shift+F2 打开（真实按键，走 config.OnKey → kernel.HandleKey）。
	if err := h.session.Driver().Key(ui.KeyF2, ui.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.p.Shown() {
		t.Fatal("Shift+F2 之后浮层没打开")
	}
	for _, want := range []string{"设置", "外观", "模型", "权限", "会话", "深色", "浅色", "跟随系统",
		"恢复默认", settingsTitle, settingsCWD, "当前：b", "当前：ask", "当前：未知"} {
		if !h.session.HasText(want) {
			t.Fatalf("打开后没画 %q：%q", want, h.session.Texts())
		}
	}
	// **未知就不伪造选中**：cfg 是空的（这轮主题不是从配置恢复的），
	// 外观三行都不该有 ✓；模型 b、权限 ask 各一个 → 一共两个 ✓。
	if got := h.countText("✓"); got != 2 {
		t.Fatalf("初始 ✓ 应有 2 个（模型 b / 权限 ask），得到 %d：%q", got, h.session.Texts())
	}

	darkShot := h.screenshot(width, height, "settings-open-dark", dir)
	darkLuma := meanLuma(t, darkShot)
	if darkLuma > 90 {
		t.Fatalf("深色底亮度 %.1f 不该这么高", darkLuma)
	}

	// —— 点「浅色」：真实命中区 → 点击 → 主题对象就地换 → 下一帧全窗变浅。
	h.clickRow("theme-light")
	if mode, n := h.gotAt(0); mode != "light" || n != 1 {
		t.Fatalf("点浅色应回传一次 SetTheme(light)，得到 %q × %d", mode, n)
	}
	if h.theme.Dark {
		t.Fatal("SetTheme(light) 之后主题对象还是深色（运行时没生效）")
	}
	if !h.p.Shown() {
		t.Fatal("换主题不该把浮层关掉")
	}
	lightShot := h.screenshot(width, height, "settings-open-light", dir)
	lightLuma := meanLuma(t, lightShot)
	if lightLuma < 200 {
		t.Fatalf("浅色底亮度 %.1f 不该这么低", lightLuma)
	}
	if lightLuma <= darkLuma*1.5 {
		t.Fatalf("深浅两张图亮度差不够：dark=%.1f light=%.1f", darkLuma, lightLuma)
	}
	t.Logf("LUMA dark=%.1f light=%.1f", darkLuma, lightLuma)

	// —— 点模型（真实 id，带渠道的那一条）与权限。
	h.clickRow("model-group/deepseek-v4.1")
	if id, n := h.gotAt(1); id != "group/deepseek-v4.1" || n != 1 {
		t.Fatalf("点模型应回传原样 id，得到 %q × %d", id, n)
	}
	h.clickRow("perm-allow")
	if perm, n := h.gotAt(2); perm != "allow" || n != 1 {
		t.Fatalf("点权限应回传 allow，得到 %q × %d", perm, n)
	}

	// —— 卡片没被窗口切掉：最下面那条可点行的命中区必须在窗口内。
	last := h.hitFor("perm-default")
	if bottom := last.Bounds.Y + last.Bounds.Height; bottom > float32(height) {
		t.Fatalf("卡片底部 %.0f 超出窗口高 %d", bottom, height)
	}

	// —— Esc 关闭（真实按键）：浮层收起，设置行不再有命中区。
	if err := h.session.Driver().Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if h.p.Shown() {
		t.Fatal("Esc 之后浮层还开着")
	}
	if h.session.HasText("只列壳真接上的项") {
		t.Fatalf("Esc 之后卡片还在画：%q", h.session.Texts())
	}
	h.screenshot(width, height, "settings-closed", dir)
}

// TestSettingsOverlayDoesNotFightComposerKeys 钉住"关着时一律放行"：
// 没有设置动作时 Shift+F2 放行；开着时普通 F2/Enter/带修饰键的键都不吃。
func TestSettingsOverlayDoesNotFightComposerKeys(t *testing.T) {
	p := &Plugin{}
	if p.HandleKey(ui.KeyEvent{Key: ui.KeyF2, Modifiers: ui.ModifierShift}) {
		t.Fatal("没有设置动作时 Shift+F2 应放行")
	}
	p.act = pluginapi.Actions{SetTheme: func(string) {}}
	p.rows = []Row{{Key: "only", Run: func() {}}}
	p.shown = true
	for _, ev := range []ui.KeyEvent{
		{Key: ui.KeyF2}, // 命令面板
		{Key: ui.KeyF2, Modifiers: ui.ModifierControl},    // 别的组合
		{Key: ui.KeyEnter, Modifiers: ui.ModifierControl}, // 输入框的 Ctrl+Enter
		{Key: ui.KeyDown, Modifiers: ui.ModifierShift},    // 选择类组合
	} {
		if p.HandleKey(ev) {
			t.Fatalf("%+v 不该被设置浮层吃掉", ev)
		}
	}
	// 而浮层开着时，Esc / ↑↓ / Enter 是它自己的——必须吃。
	for _, ev := range []ui.KeyEvent{{Key: ui.KeyEscape}, {Key: ui.KeyDown}, {Key: ui.KeyUp}, {Key: ui.KeyEnter}} {
		if !p.HandleKey(ev) {
			t.Fatalf("浮层开着时 %v 应被吃掉", ev.Key)
		}
		p.shown = true // Esc 会关掉它，测试里重新打开继续验下一个键
	}
}

// TestSettingsTextHasNoFakeCurrentState 是视觉层的反证：当前值未知时，
// 画出来的文本里不许出现"当前：深色 / 当前：浅色"这种编出来的值。
func TestSettingsTextHasNoFakeCurrentState(t *testing.T) {
	data := pluginapi.Snapshot{Cur: settingsSessionID, Sessions: []apiclient.Session{{ID: settingsSessionID, Title: settingsTitle}}}
	secs := BuildSections(pluginapi.Actions{
		SetTheme: func(string) {}, SetPermission: func(string) {},
		ModelsList: func() []pluginapi.Model { return []pluginapi.Model{{ID: "b"}} },
		SetModel:   func(string) {},
	}, data, map[string]string{}, true)

	var notes []string
	for _, s := range secs {
		notes = append(notes, s.Note)
	}
	joined := strings.Join(notes, " | ")
	if strings.Contains(joined, "当前：深色") || strings.Contains(joined, "当前：浅色") ||
		strings.Contains(joined, "当前：跟随系统") {
		t.Fatalf("未知时编出了当前主题：%q", joined)
	}
	if !strings.Contains(joined, "未知") {
		t.Fatalf("未知时必须写清未知：%q", joined)
	}
}
