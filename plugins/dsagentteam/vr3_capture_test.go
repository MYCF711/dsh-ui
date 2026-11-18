//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：AgentTeams（官方 ui-subagent 的目录
// 父容器 + ui-agent-team 的四块槽位贡献）的**空态 / loading / hover / 长名单 / 禁用**。
//
// 依据一律取 **rc.2 asar 里的原件**（`Desktop-0.2.2-rc.2/resources/app.asar`，
// 解出来的源文件在 `$HOME/.rc2src/`，本文件按它的行号引用）：
//
//	子代理目录（ui-subagent / SubagentHeaderLineage.module.css）
//	  `.menu{max-width:calc(100vw - 32px);max-height:min(560px,calc(100dvh - 140px));
//	    padding:6px;overflow:hidden;--dsh-subagent-menu-width:min(360px,calc(100vw - 32px));
//	    width:var(--dsh-subagent-menu-width);height:fit-content}` …:154-179
//	  `.menuBody{flex:1 1 auto;min-height:0;overflow:auto}` …:181-188
//	  `.sectionEmpty{padding:0 7px 8px 28px;font-size:12px}` …:234-239
//	  `.row{min-height:40px;padding:4px 6px 4px 9px;gap:7px;font-size:12px;line-height:17px}` …:249-263
//	  `.label{font-size:13px}` …:360-362
//	  `.metrics{font-size:10px;line-height:15px;grid-template-rows:17px 15px}` …:374-378
//	  `.notice,.error{padding:8px 10px;font-size:11px;line-height:16px}` …:480-485
//	任务板（ui-agent-team / TeamBoard.module.css）
//	  `.board{padding:8px 7px}`:5；`.taskRow{gap:7px;width:calc(100% + 12px);margin:0 -6px;
//	    padding:7px 6px;border-radius:md;font-size:13px;line-height:17px}`:60-75
//	  `@media(hover:hover){.taskRow:hover{background:var(--dsw-alias-interactive-bg-hover)}}`:85-88
//	  `.glyph{width:14px;height:17px}`:91-93；`.chevron{width:28px;height:17px}`:118-120
//	  `.details{padding:0 0 8px 21px;font-size:12px;line-height:18px}`:123-127
//	  TeamBoard.tsx：`if (!team?.enabled) return null`、`if (tasks.length === 0)
//	    return failure === null ? null : …`
//	开关（TeamModeToggle.module.css）`.row{gap:16px;min-height:34px;padding:0 8px}`:4-15、
//	  `.infoButton{width:20px;height:20px}`:25-32
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsagentteam

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// ── 落图与像素小工具（vr3 前缀防撞名）───────────────────────────────

func vr3Dir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("VR3_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

func vr3Sample(img *image.RGBA, x, y int) color.RGBA {
	if img == nil {
		return color.RGBA{}
	}
	return img.RGBAAt(x, y)
}

func vr3AbsDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}

// vr3Ink 数 rect 里与 bg 不同的像素并给出墨迹包围盒。
func vr3Ink(img *image.RGBA, rect gfx.Rect, bg color.RGBA) (box gfx.Rect, count int) {
	if img == nil || rect.Width <= 0 || rect.Height <= 0 {
		return gfx.Rect{}, 0
	}
	minX, minY := float32(1e9), float32(1e9)
	maxX, maxY := float32(-1e9), float32(-1e9)
	bounds := img.Bounds()
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
			if x < bounds.Min.X || y < bounds.Min.Y || x >= bounds.Max.X || y >= bounds.Max.Y {
				continue
			}
			p := img.RGBAAt(x, y)
			if vr3AbsDiff(p.R, bg.R) <= 6 && vr3AbsDiff(p.G, bg.G) <= 6 && vr3AbsDiff(p.B, bg.B) <= 6 {
				continue
			}
			count++
			fx, fy := float32(x), float32(y)
			if fx < minX {
				minX = fx
			}
			if fx > maxX {
				maxX = fx
			}
			if fy < minY {
				minY = fy
			}
			if fy > maxY {
				maxY = fy
			}
		}
	}
	if count == 0 {
		return gfx.Rect{}, 0
	}
	return gfx.Rect{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1}, count
}

// vr3PixelDiff 数两张图的差异像素数（hover / disabled 这类"只有看图才知道"的证据）。
func vr3PixelDiff(a, b *image.RGBA) int {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// ── 真窗口现场 ──────────────────────────────────────────────────────

// vr3Team 是一台装了真内核的真窗口：每个用例自己说画哪个槽。
type vr3Team struct {
	t       *testing.T
	p       *Plugin
	kernel  *uikernel.Kernel
	theme   *pluginapi.Theme
	tree    *gfx.Tree
	feat    gfx.Scope
	session *headless.Session
	dir     string
	// draw 是这一帧要 dispatch 的槽。
	draw func(f ui.C, env *pluginapi.Env, k *uikernel.Kernel)
}

// vr3OpenTeam 装配真内核并起窗口。draw 决定这一帧画哪一块。
func vr3OpenTeam(t *testing.T, windowW, windowH int, view View,
	draw func(f ui.C, env *pluginapi.Env, k *uikernel.Kernel)) *vr3Team {
	t.Helper()
	kernel := uikernel.New("agent-team-vr3")
	tier := uikernel.NewShellTier()
	kernel.Root().Provide(uikernel.KeyShellTier, tier)
	p := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	p.AttachShellTier(tier) // 壳在 Load 之后做的那一步（catalog_test.go 同形）
	p.team.SetView(view)
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("agent-team-vr3")
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	h := &vr3Team{t: t, p: p, kernel: kernel, theme: theme, tree: tree, feat: feat,
		dir: vr3Dir(t), draw: draw}
	config := app.Config{
		Title: "agent-team-vr3", Width: windowW, Height: windowH, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		OnScroll: page.Wheel,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280},
			Act:      pluginapi.Actions{Invalidate: invalidator.Invalidate, Notify: func(string) {}},
			// 官方 footerAvailable = "该槽有没有注册者"；这里按"有"喂，
			// 让目录在有共享任务时真的可见（catalog.go:243-247 读的就是它）。
			Has: func(slot pluginapi.Slot) bool {
				return slot == SlotSubagentFooter || kernel.Has(slot)
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		page.Draw(canvas, bounds, func(f ui.C) {
			draw(f, env, kernel)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open(%dx%d) 失败：%v", windowW, windowH, err)
	}
	t.Cleanup(session.Close)
	h.session = session
	h.pump()
	h.pump()
	return h
}

func (h *vr3Team) pump() {
	h.t.Helper()
	if err := h.session.Driver().Pump(); err != nil {
		h.t.Fatal(err)
	}
	// OpenCatalog/SetView 这类**没有输入事件**的状态变化不会自己排帧，
	// 必须显式要一帧（headless 的 Frame = "画一帧但不走时钟"）。
	if err := h.session.Frame(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *vr3Team) id(name string) gfx.ID { return h.tree.ID(h.feat, name) }

func (h *vr3Team) hit(name string) (gfx.Hit, bool) {
	want := h.id(name)
	for _, hit := range h.session.Snapshot().Hits {
		if hit.ID == want {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func (h *vr3Team) hitLabels() []string {
	out := make([]string, 0, len(h.session.Snapshot().Hits))
	for _, hit := range h.session.Snapshot().Hits {
		out = append(out, fmt.Sprintf("%q@%v", hit.Label, hit.Bounds))
	}
	return out
}

func (h *vr3Team) texts() []string {
	snapshot := h.session.Snapshot()
	out := make([]string, 0, len(snapshot.Texts))
	for _, text := range snapshot.Texts {
		out = append(out, text.Value)
	}
	return out
}

func (h *vr3Team) hasText(want string) bool {
	for _, value := range h.texts() {
		if value == want {
			return true
		}
	}
	return false
}

func (h *vr3Team) shot(name string) *image.RGBA {
	h.t.Helper()
	path := filepath.Join(h.dir, name)
	if err := h.session.Screenshot(path); err != nil {
		h.t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		h.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	img, err := h.session.Image()
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Logf("VR3_CAPTURE path=%s size=%v bytes=%d", path, img.Bounds(), info.Size())
	return img
}

func (h *vr3Team) move(x, y float32) {
	h.t.Helper()
	if err := h.session.Move(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// vr3CatalogOnly 只 dispatch 目录面板（官方 createPortal 的等价层 = shell.overlay）。
func vr3CatalogOnly(f ui.C, env *pluginapi.Env, k *uikernel.Kernel) {
	k.DrawSlot(f, pluginapi.SlotShellOverlay, env)
}

// vr3BoardOnly 只 dispatch 任务板（目录 footer 里那一段）。
func vr3BoardOnly(f ui.C, env *pluginapi.Env, k *uikernel.Kernel) {
	k.DrawSlot(f, SlotSubagentFooter, env)
}

// vr3ToggleOnly 只 dispatch 模型菜单里的 Team 开关。
func vr3ToggleOnly(f ui.C, env *pluginapi.Env, k *uikernel.Kernel) {
	k.DrawSlot(f, SlotInputModelOptions, env)
}

// vr3CatalogView 造一份"目录已知 + n 个直接子代理 + 可选共享任务"的视图。
func vr3CatalogView(n int, sharedTasks bool) View {
	v := View{SessionID: "sess-vr3", CatalogKnown: true, SubagentChild: false,
		ModelSelectionKnown: true}
	for i := 0; i < n; i++ {
		v.Catalog = append(v.Catalog, CatalogEntry{
			ID:    fmt.Sprintf("child-%02d", i),
			Label: fmt.Sprintf("子智能体 %02d", i),
			Mode:  "continuable",
		})
	}
	v.Members = len(v.Catalog)
	if sharedTasks {
		v.ProjectionKnown = true
		v.Projection = TeamProjection{
			Enabled: true,
			Tasks: []TeamTask{
				{ID: "t1", Subject: "收集素材", Status: StatusCompleted, Ready: true, OwnerName: "qa"},
				{ID: "t2", Subject: "写实现", Status: StatusInProgress, Ready: true, OwnerID: "dev"},
			},
		}
	}
	return v
}

// ── ① 目录：空态（有共享任务、没有子代理）与面板几何 ────────────────

// TestVR3CatalogEmptySectionGeometry 验两件事：
//
//	① 面板宽度 = 官方 `--dsh-subagent-menu-width = min(360px, 100vw - 32px)`
//	   （SubagentHeaderLineage.module.css:170-174），900 宽窗口 → 360。
//	② 空态文案 `.sectionEmpty` 的左内缩 = 官方 28px（同文件 :236
//	   `padding:0 7px 8px 28px`），加上 `.menu{padding:6px}`（:163）后
//	   文字左缘 = 面板左缘 + 6 + 28 = +34。
func TestVR3CatalogEmptySectionGeometry(t *testing.T) {
	p := &Plugin{}
	h := vr3OpenTeam(t, 900, 700, vr3CatalogView(0, true), vr3CatalogOnly)
	_ = p
	h.p.OpenCatalog()
	h.pump()
	h.pump()
	menu, ok := h.hit("subagent-catalog-menu")
	if !ok {
		t.Fatalf("目录面板没有命中区；hits=%v texts=%v", h.hitLabels(), h.texts())
	}
	if menu.Bounds.Width != catalogMenuWidth {
		t.Fatalf("目录面板宽 %v，官方 `--dsh-subagent-menu-width:min(360px,100vw - 32px)`"+
			"（SubagentHeaderLineage.module.css:170-174）在 900 宽窗口下应为 %v",
			menu.Bounds.Width, float32(catalogMenuWidth))
	}
	if !h.hasText(zhCatalogSectionEmpty) {
		t.Fatalf("有共享任务、没有子代理时应画 %q（官方 .sectionEmpty）；texts=%v",
			zhCatalogSectionEmpty, h.texts())
	}
	var empty gfx.Rect
	for _, text := range h.session.Snapshot().Texts {
		if text.Value == zhCatalogSectionEmpty {
			empty = text.Bounds
			break
		}
	}
	want := menu.Bounds.X + catalogMenuPad + catalogSectionEmptyLeft
	if empty.X != want {
		t.Fatalf("空态文案左缘 %v，官方 `.sectionEmpty{padding-left:28px}`（:236）"+
			"＋ `.menu{padding:6px}`（:163）应为令牌左缘+34 = %v", empty.X, want)
	}
	h.shot("agentteam-catalog-empty.png")
	t.Logf("VR3_CATALOG_EMPTY menu=%+v text=%+v", menu.Bounds, empty)
}

// ── ② 目录：行高 40 与长名单溢出 ────────────────────────────────────

// TestVR3CatalogRowHeight 验行的官方 40px 下界。
//
// 官方 `.row{min-height:40px;padding:4px 6px 4px 9px;gap:7px}`（:249-263）。
func TestVR3CatalogRowHeight(t *testing.T) {
	h := vr3OpenTeam(t, 900, 700, vr3CatalogView(3, false), vr3CatalogOnly)
	h.p.OpenCatalog()
	h.pump()
	h.pump()
	row, ok := h.hit("subagent-catalog-row-child-00")
	if !ok {
		t.Fatalf("找不到目录行命中区；hits=%v texts=%v", h.hitLabels(), h.texts())
	}
	if row.Bounds.Height < catalogRowMinHeight {
		t.Fatalf("目录行高 %v，官方 `.row{min-height:40px}`（:256）是**下界**，不能更矮", row.Bounds.Height)
	}
	// 行的宽度必须吃满 `.menu` 的内容宽（360 - 2×6 = 348）。
	if want := float32(catalogMenuWidth - 2*catalogMenuPad); row.Bounds.Width != want {
		t.Fatalf("目录行宽 %v，官方 `.row{width:100%%}` 在 360-6×2 的面板里应为 %v",
			row.Bounds.Width, want)
	}
	h.shot("agentteam-catalog-three-rows.png")
	t.Logf("VR3_CATALOG_ROW bounds=%+v", row.Bounds)
}

// TestVR3CatalogLongRosterStaysInsidePanel 验长名单不出面板。
//
// 官方 `.menu{overflow:hidden;max-height:min(560px,calc(100dvh - 140px))}`（:162-164）
// + `.menuBody{flex:1 1 auto;min-height:0;overflow:auto}`（:181-188）：
// 名单长了在面板里滚，**不许画到面板外面**。
//
// 500 高窗口 → 面板 max-height = 500-140 = 360；14 行 × 40 = 560 > 360，
// 所以这一帧必然要"裁"。
func TestVR3CatalogLongRosterStaysInsidePanel(t *testing.T) {
	h := vr3OpenTeam(t, 900, 500, vr3CatalogView(14, false), vr3CatalogOnly)
	h.p.OpenCatalog()
	h.pump()
	h.pump()
	menu, ok := h.hit("subagent-catalog-menu")
	if !ok {
		t.Fatalf("目录面板没有命中区；hits=%v", h.hitLabels())
	}
	if want := float32(500 - catalogMenuViewportInset); menu.Bounds.Height > want {
		t.Fatalf("面板高 %v 超过官方 `max-height:min(560px,100dvh - 140px)` 在 500 高窗口下的 %v",
			menu.Bounds.Height, want)
	}
	img := h.shot("agentteam-catalog-long-roster.png")
	// ① 字迹不许**画到**面板外：面板外的行必须是被面板/滚动盒裁掉的
	//    （`HasClip` 且裁剪盒落在面板内），否则就是真画出来了。
	var outside []string
	for _, text := range h.session.Snapshot().Texts {
		b := text.Bounds
		if b.X >= menu.Bounds.X-0.5 && b.Y >= menu.Bounds.Y-0.5 &&
			b.X+b.Width <= menu.Bounds.X+menu.Bounds.Width+0.5 &&
			b.Y+b.Height <= menu.Bounds.Y+menu.Bounds.Height+0.5 {
			continue // 在面板里
		}
		if text.HasClip && text.ClipOf.X >= menu.Bounds.X-0.5 && text.ClipOf.Y >= menu.Bounds.Y-0.5 &&
			text.ClipOf.X+text.ClipOf.Width <= menu.Bounds.X+menu.Bounds.Width+0.5 &&
			text.ClipOf.Y+text.ClipOf.Height <= menu.Bounds.Y+menu.Bounds.Height+0.5 {
			continue // 被面板内的裁剪盒裁掉：看不见，不算画出格
		}
		outside = append(outside, fmt.Sprintf("%q@%v", text.Value, b))
	}
	if len(outside) > 0 {
		t.Errorf("有 %d 段文字画到了面板 %+v 之外且没有被裁（官方 `.menu{overflow:hidden}` :164）：%v",
			len(outside), menu.Bounds, outside)
	}
	// ② 像素层：面板底边**下面**那一整段（阴影以里 8px 起，到原溢出位置）必须干净。
	//    溢出的第 08–13 行原本落在 +2…+239，这里把中下段落全包住。
	below := gfx.Rect{X: menu.Bounds.X, Y: menu.Bounds.Y + menu.Bounds.Height + 8,
		Width: menu.Bounds.Width, Height: 232}
	if box, count := vr3Ink(img, below, vr3Sample(img, 1, 1)); count > 0 {
		t.Errorf("面板下方（+8..+240）出现 %d 像素墨迹（bbox=%+v）：长名单溢出了面板", count, box)
	}
	// ③ 真滚：滚轮到底之后最后一条（子智能体 13）必须进入面板可见区——
	//    官方 `.menuBody{overflow:auto}`（:186）是"能滚"，不是"只裁掉"。
	h.move(menu.Bounds.X+menu.Bounds.Width/2, menu.Bounds.Y+menu.Bounds.Height/2)
	for range 12 {
		if err := h.session.Scroll(gfx.ScrollEvent{
			X: menu.Bounds.X + menu.Bounds.Width/2, Y: menu.Bounds.Y + menu.Bounds.Height/2,
			DeltaY: 300, Precise: true,
		}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	h.shot("agentteam-catalog-long-roster-bottom.png")
	var last gfx.Rect
	for _, text := range h.session.Snapshot().Texts {
		if strings.HasPrefix(text.Value, "子智能体 13") {
			last = text.Bounds
		}
	}
	if last.Width == 0 {
		t.Fatalf("滚到底之后最后一条(%q)不见了", "子智能体 13")
	}
	if last.Y < menu.Bounds.Y-0.5 || last.Y+last.Height > menu.Bounds.Y+menu.Bounds.Height+0.5 {
		t.Fatalf("滚到底之后最后一条仍在面板外：%+v（面板 %+v）", last, menu.Bounds)
	}
	t.Logf("VR3_CATALOG_LONG menu=%+v texts=%d lastVisible=%+v", menu.Bounds, len(h.texts()), last)
}

// ── ③ 任务板：hover 与空态 ──────────────────────────────────────────

// TestVR3BoardHoverAndEmpty 验任务板两件官方写死的事。
//
//	hover：`@media(hover:hover){.taskRow:hover{background:var(--dsw-alias-
//	  interactive-bg-hover)}}`（TeamBoard.module.css:85-88）——指针进到行上要有像素变化。
//	空态：`if (tasks.length === 0) return failure === null ? null : …`
//	  （TeamBoard.tsx）——没有任务、没有 failure 时**整块不画**。
func TestVR3BoardHoverAndEmpty(t *testing.T) {
	// 空态：不算 failure、没有任务 → 一个字都不画。
	empty := vr3OpenTeam(t, 900, 700, View{
		SessionID: "sess-vr3", ProjectionKnown: true,
		Projection: TeamProjection{Enabled: true},
	}, vr3BoardOnly)
	if got := empty.texts(); len(got) != 0 {
		t.Fatalf("空任务板画出了文字：%v（官方 TeamBoard.tsx: tasks.length===0 且无 failure → null）", got)
	}
	empty.shot("agentteam-board-empty.png")

	// hover：有任务时行是可点的，指针进入要有底色。
	h := vr3OpenTeam(t, 900, 700, vr3CatalogView(2, true), vr3BoardOnly)
	row, ok := h.hit("team-task-t2")
	if !ok {
		t.Fatalf("找不到任务行命中区；hits=%v texts=%v", h.hitLabels(), h.texts())
	}
	h.move(880, 680)
	rest := h.shot("agentteam-board-rest.png")
	h.move(row.Bounds.X+row.Bounds.Width/2, row.Bounds.Y+row.Bounds.Height/2)
	hover := h.shot("agentteam-board-hover.png")
	if diff := vr3PixelDiff(rest, hover); diff <= 0 {
		t.Errorf("指针进入任务行 %+v 后画面零变化：没有 hover 底色。官方 "+
			"`@media(hover:hover){.taskRow:hover{background:var(--dsw-alias-interactive-bg-hover)}}`"+
			"（TeamBoard.module.css:85-88）", row.Bounds)
	}
	t.Logf("VR3_BOARD_ROW bounds=%+v", row.Bounds)
}

// ── ④ 任务板：进行中的 StateDot 到底转不转 ──────────────────────────

// TestVR3BoardSpinnerFrozen 验 in_progress 的 14px spinner。
//
// 官方 `StateDot`：ongoing 默认 14px（primitives 的 `size ?? (ongoing ? 14 : 10)`），
// 且**真的在转**——`index-CSAdOBx6.css` 的
// `._spinnerMotion{transform-origin:center;animation:_dsh-state-dot-spin 1.5s linear infinite}`
// 与 `@keyframes …{to{transform:rotate(360deg)}}`。
//
// 判据用渲染器自己的动画时钟：`Transitioning()` 报告"上一帧有没有要下一帧"。
func TestVR3BoardSpinnerFrozen(t *testing.T) {
	h := vr3OpenTeam(t, 900, 700, vr3CatalogView(2, true), vr3BoardOnly)
	before := h.shot("agentteam-board-spinner.png")
	if err := h.session.Advance(750 * 1000 * 1000); err != nil { // 750ms：半圈
		t.Fatal(err)
	}
	h.pump()
	after := h.shot("agentteam-board-spinner-adv750.png")
	if !h.session.Transitioning() {
		t.Errorf("推 750ms 后渲染器没有要求下一帧：进行中的 StateDot 是静止的。" +
			"官方 `._spinnerMotion{animation:_dsh-state-dot-spin 1.5s linear infinite}`" +
			"（index-CSAdOBx6.css，rc.2）要求它一直转")
	}
	if diff := vr3PixelDiff(before, after); diff != 0 {
		t.Logf("VR3_BOARD_SPINNER_FRAME_DIFF=%d", diff)
	}
	// 14px 的口径：glyph 槽 14×17（TeamBoard.module.css:91-93），ongoing 的
	// spinner 是 14×14，必须落在行内第一个 14×17 的槽里。
	glyph, ok := h.hit("team-task-glyph-t2")
	if !ok {
		t.Fatalf("找不到 glyph 槽命中区；hits=%v", h.hitLabels())
	}
	if glyph.Bounds.Width != 14 || glyph.Bounds.Height != 17 {
		t.Fatalf("glyph 槽 = %v×%v，官方 `.glyph{width:14px;height:17px}`（:91-93）",
			glyph.Bounds.Width, glyph.Bounds.Height)
	}
	if box, count := vr3Ink(before, glyph.Bounds, vr3Sample(before, 1, 1)); count == 0 {
		t.Errorf("glyph 槽 %+v 里没有墨迹：进行中的状态点没画出来", glyph.Bounds)
	} else {
		t.Logf("VR3_BOARD_GLYPH_INK box=%+v count=%d", box, count)
	}
}

// ── ⑤ 开关：禁用时视觉上真的降级 ────────────────────────────────────

// TestVR3ToggleDisabledLooksDisabled 验 Team 开关的禁用态。
//
// 官方 `.row{gap:16px;min-height:34px;padding:0 8px}`（TeamModeToggle.module.css:4-15）
// 与 `.infoButton{width:20px;height:20px}`（:25-32）；
// 禁用条件 `locked || pending || modelSelection === undefined`（TeamModeToggle.tsx）。
// 判据：命中区 Disabled=true **并且**画面真的变了（不是"标了禁用但其实照旧"）。
func TestVR3ToggleDisabledLooksDisabled(t *testing.T) {
	enabled := vr3OpenTeam(t, 600, 300, View{
		SessionID: "sess-vr3", ProjectionKnown: true, ModelSelectionKnown: true,
		Projection: TeamProjection{Enabled: true},
	}, vr3ToggleOnly)
	if !enabled.hasText(zhModeLabel) {
		t.Fatalf("开关行没画文案；texts=%v", enabled.texts())
	}
	onImg := enabled.shot("agentteam-toggle-enabled.png")

	locked := vr3OpenTeam(t, 600, 300, View{
		SessionID: "sess-vr3", ProjectionKnown: true, ModelSelectionKnown: true,
		Projection: TeamProjection{Enabled: true}, Locked: true,
	}, vr3ToggleOnly)
	offImg := locked.shot("agentteam-toggle-locked.png")
	if diff := vr3PixelDiff(onImg, offImg); diff <= 0 {
		t.Errorf("把 locked 置真后画面零变化：禁用态没有任何视觉降级")
	}
	disabled := 0
	for _, hit := range locked.session.Snapshot().Hits {
		if hit.Disabled {
			disabled++
		}
	}
	if disabled == 0 {
		t.Errorf("locked 时没有任何命中区标成 Disabled（viva 的 Switch 应以 Disabled(true) 传播）；hits=%v",
			locked.hitLabels())
	}
	// 官方信息按钮 20×20。
	if info, ok := locked.hit("team-mode-info"); ok {
		if info.Bounds.Width != infoButtonSize || info.Bounds.Height != infoButtonSize {
			t.Fatalf("信息按钮 = %v×%v，官方 `.infoButton{width:20px;height:20px}`"+
				"（TeamModeToggle.module.css:25-32）", info.Bounds.Width, info.Bounds.Height)
		}
	} else {
		t.Errorf("找不到信息按钮命中区；hits=%v", locked.hitLabels())
	}
	t.Logf("VR3_TOGGLE disabledHits=%d texts=%v", disabled, locked.texts())
}

// vr3Sanitize 把字符串里的控制字符换成可见形式（断言信息可读）。
func vr3Sanitize(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.Map(func(r rune) rune {
			if r < 0x20 {
				return '·'
			}
			return r
		}, value)
		out = append(out, clean)
	}
	return out
}
