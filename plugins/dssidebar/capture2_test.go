//go:build darwin || windows || linux

// capture2_test.go —— 第二轮真实渲染审计：侧栏的**三种行状态**像素证据。
//
// 第一轮只看过"选中行"（frame_visual_test.go 的默认态），没看过 hover 态，
// 也没看过行尾的「取消固定」按钮在指针进入前是什么样。官方的规则很明确：
//
//	SidebarRoot.module.css `.panelRow:hover{background:interactive-bg-hover}`
//	SidebarRoot.module.css `.panelUnpin{opacity:0}` +
//	                        `@media (hover:hover){.panelEntry:hover .panelUnpin{opacity:1}}`
//	SidebarRoot.module.css `.panelEntryActive{background:interactive-bg-hover}`
//
// 截图落 `$FG2_CAPTURE_DIR/round2`（未设置时用临时目录）。
package dssidebar

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func sideCaptureDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("FG2_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	dir := filepath.Join(base, "round2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestSidebarRowStatesCapture 在同一窗口里用**真指针**切四种状态，各留一张图。
//
// 状态切换全部走 session.Move / Click（真 hover 判定 + 真命中测试），
// 不是改内部字段摆拍：hover 态只有真走指针才作数。
func TestSidebarRowStatesCapture(t *testing.T) {
	dir := sideCaptureDir(t)
	kernel := uikernel.New("sidebar-round2")
	sidebar := &Plugin{}
	// 固定态服务由壳提供（cmd/app/wave3_wiring.go:157 同一条接线）。不给它时
	// 面板行按"全部固定"处理、行尾「取消固定」按钮根本不画——那样就看不到
	// 官方 `.panelUnpin{opacity:0}` / `:hover{opacity:1}` 的显隐。
	kernel.Root().Provide(pluginapi.KeyEntryPins, pluginapi.EntryPins(entrypins.New()))
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, sidebar); err != nil {
		t.Fatal(err)
	}
	defer kernel.Dispose()
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)

	cur := "s-002"
	opened := ""
	tree := &gfx.Tree{}
	feat := tree.Feature("sidebar-round2")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	invalidator := &app.Invalidator{}
	config := app.Config{Title: "sidebar-round2", Width: 1182, Height: 832, Tree: tree, WindowMode: app.WindowFrameless}
	config.Invalidator = invalidator
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		mainWidth := size.Width - 280
		env := &pluginapi.Env{
			Theme: theme,
			Data:  pluginapi.Snapshot{Sessions: mockdSessions(), Cur: cur, Busy: false},
			Act: pluginapi.Actions{
				NewSession: func() {},
				OpenSession: func(id string) {
					opened = id
					cur = id
					invalidator.Invalidate()
				},
			},
			Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: mainWidth,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: 280, MainWidth: mainWidth,
			},
			// 座位判据同壳：`sidebar.panellist.actions` 有占用方时，
			// 面板行才带行尾「取消固定」（entry_menu.go:77-79）。
			Has: func(slot pluginapi.Slot) bool {
				return slot == uikernel.OffSidebarPanelActions || kernel.Has(slot)
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for range 3 {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	findHit := func(name string) (gfx.Hit, bool) {
		id := tree.ID(feat, name)
		for _, hit := range session.Snapshot().Hits {
			if hit.ID == id {
				return hit, true
			}
		}
		return gfx.Hit{}, false
	}
	save := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("FG2_CAPTURE2 path=%s", path)
	}
	moveTo := func(hit gfx.Hit) {
		t.Helper()
		if err := session.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	// ① 默认态：指针不在任何行上（先把它挪到窗口右下角）。
	if err := session.Move(1170, 820); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	save("sidebar-default.png")

	// ② 面板行 hover：官方 `.panelEntry:hover` → 行背景 interactive-bg-hover，
	//    且行尾「取消固定」`.panelUnpin` 从 opacity:0 淡入到 1。
	panel, ok := findHit("panel-plugins")
	if !ok {
		t.Fatalf("没有插件面板行的命中区；hits=%v", session.Snapshot().Hits)
	}
	moveTo(panel)
	// 行尾「取消固定」的命中区必须真的存在（否则这张图证明不了显隐）。
	unpins := 0
	for _, hit := range session.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, "从左侧栏取消固定") {
			unpins++
			t.Logf("UNPIN hit label=%q bounds=%+v", hit.Label, hit.Bounds)
		}
	}
	if unpins == 0 {
		t.Fatalf("面板行尾没有「取消固定」命中区（pins 没接上）")
	}
	save("sidebar-panel-hover.png")

	// ③ 会话行 hover：官方会话行 hover 与选中同色（Rows 的 hover 档），
	//    行尾的删除/菜单控件的显隐也在这张图里。
	s001, ok := findHit("s-s-001")
	if !ok {
		t.Fatalf("没有会话行 s-001 的命中区；hits=%v", session.Snapshot().Hits)
	}
	moveTo(s001)
	save("sidebar-session-hover.png")

	// ④ 点 s-001 选中：真点击 → OpenSession 回传 → Cur 换人 → 下一帧重画。
	if err := session.Click(s001.Bounds.X+s001.Bounds.Width/2, s001.Bounds.Y+s001.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if opened != "s-001" {
		t.Fatalf("点会话行回传 %q，期望 s-001", opened)
	}
	if cur != "s-001" {
		t.Fatalf("点击后 Cur=%q，期望 s-001", cur)
	}
	save("sidebar-session-selected.png")

	// ⑤ 悬停「自动化任务」行：官方 `.panelUnpin{opacity:0}` 只在
	//    `.panelEntry:hover` 时淡入。默认态那张图里这块必须干净。
	if err := session.Move(1170, 820); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	unpinID := tree.ID(feat, "panel-unpin-schedules")
	unpinHit, ok := findHit("panel-unpin-schedules")
	if !ok {
		t.Fatalf("没有「取消固定自动化任务」命中区；hits=%v", session.Snapshot().Hits)
	}
	before, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if ink := iconInk(before, unpinHit.Bounds); ink > 4 {
		t.Fatalf("默认态 pin 图标已经显形（ink=%d px）——官方 .panelUnpin{opacity:0}", ink)
	}
	sched, ok := findHit("panel-schedules")
	if !ok {
		t.Fatalf("没有「自动化任务」行命中区")
	}
	moveTo(sched)
	after, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if ink := iconInk(after, unpinHit.Bounds); ink < 20 {
		t.Fatalf("悬停下该行后 pin 图标仍没显形（ink=%d px）", ink)
	}
	_ = unpinID
	save("sidebar-unpin-hover.png")
}

// iconInk 数一块矩形里"与同一行的底色不同"的像素，用来判 14×14 的 pin 图标
// 到底有没有画上去。底色取矩形左侧 3px 处（同一行、同一背景）。
func iconInk(img *image.RGBA, r gfx.Rect) int {
	bg := img.RGBAAt(int(r.X)-3, int(r.Y+r.Height/2))
	ink := 0
	for y := int(r.Y); y < int(r.Y+r.Height); y++ {
		for x := int(r.X); x < int(r.X+r.Width); x++ {
			c := img.RGBAAt(x, y)
			d := absInt(int(c.R)-int(bg.R)) + absInt(int(c.G)-int(bg.G)) + absInt(int(c.B)-int(bg.B))
			if d > 24 {
				ink++
			}
		}
	}
	return ink
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
