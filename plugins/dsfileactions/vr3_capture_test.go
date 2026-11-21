//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：file-actions 的两段菜单在**真装配**下
// 画出来是什么样（本包自己不画常驻面，它的"渲染"就是别人菜单里的那几行）。
//
// 官方依据全部取自 rc.2 asar：
//
//	file 菜单  `ctx.contextMenus.register({id:'file-actions.file',target:'file',order:200,
//	             resolve: async ({sessionId,path},signal)=>{ const result =
//	             await ctx.remote.workspaceFiles.stat(sessionId,path,signal); …}})`
//	           —— **先 stat，再按 available 给每一项挂 `disabled:!available`**（偏移 33363246）
//	image 菜单 `{id:'file-actions.image',target:'image',order:100,resolve:target=>{…}}`
//	           —— `[copy, download, …canSaveAs?saveAs]`，这一族**没有 disabled**（偏移 33364653）
//	三种动作的文案与判据（偏移 33365633）：
//	           `...mediaType.startsWith('image/')?[copyImage(separatorBefore)]:[],`
//	           `...mediaType.startsWith('image/')||!actions.canSaveAs?[download]:[],`
//	           `...actions.canSaveAs?[saveAs]:[]`
//	zh 字典：menu.saveAs='另存为…'、menu.download='下载'、menu.copyImage='复制图片'（同偏移）
//
// 截图落 `$VR3_CAPTURE_DIR`（未设置时用临时目录）。
package dsfileactions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscontextmenu"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func vr3CaptureDir(t *testing.T) string {
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

// vr3Frame 是一台真装配：dscore + ds-contextmenu + 本包，页面上放一行可右键的目标。
type vr3Frame struct {
	t        *testing.T
	session  *headless.Session
	plugin   *Plugin
	dir      string
	saved    []string
	copied   []string
	statCall int
}

// vr3Open 起窗口。save 为 nil 时不接 SaveWorkspaceFile（"壳没接动作"的形状）；
// statOK 决定 workspaceFiles.stat 的探测结果。
func vr3Open(t *testing.T, dir string, width, height int, target pluginapi.ContextMenuTarget,
	save func(string, string, string, string), stat func(string, string) bool) *vr3Frame {
	t.Helper()
	frame := &vr3Frame{t: t, dir: dir}
	kernel := uikernel.New("file-actions-vr3")
	p := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dscontextmenu.Plugin{}, p); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	frame.plugin = p
	menus := kernel.MustGet(pluginapi.KeyContextMenus).(pluginapi.ContextMenus)
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("file-actions-vr3")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	act := pluginapi.Actions{
		SaveWorkspaceFile: func(sessionID, path, name, mode string) {
			frame.saved = append(frame.saved, sessionID+"|"+path+"|"+name+"|"+mode)
			if save != nil {
				save(sessionID, path, name, mode)
			}
		},
		WorkspaceFileStat: func(sessionID, path string) bool {
			frame.statCall++
			if stat == nil {
				return true
			}
			return stat(sessionID, path)
		},
		// 「复制图片」的壳动作（`#18` ①）。真壳在这里会去读字节、转码、写剪贴板；
		// 真装配测试只验"菜单把**捕获的** (sessionId, path) 交回来了"，
		// 写剪贴板那一段在 cmd/app/copy_image_test.go（那里按替换点，不碰真剪贴板）。
		CopyWorkspaceImage: func(sessionID, path string) {
			frame.copied = append(frame.copied, sessionID+"|"+path)
		},
	}
	if save == nil {
		act.SaveWorkspaceFile = nil
		act.CopyWorkspaceImage = nil
	}
	rowID := tree.ID(feat, "vr3-file-row")
	config := app.Config{
		Title: "file-actions-vr3", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, Act: act,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height},
			Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			// 本包的 painter 挂在 `shell.overlay`（Order 900）：它只做一件事——
			// 把这一帧的 `env.Act` 记下来，好让菜单 Resolve 能拿到壳动作。
			kernel.DrawSlot(f, pluginapi.SlotShellOverlay, env)
			row := ui.Row(f).FillWidth().FillHeight().Hit(rowID, "文件")
			menus.Attach(f, rowID, row, target)
		})
		for _, painter := range kernel.AllPainters() {
			if painter.AfterDraw != nil {
				painter.AfterDraw(canvas, env)
			}
		}
	}
	config.OnScroll = view.Wheel
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless.Open 失败：%v", err)
	}
	t.Cleanup(session.Close)
	frame.session = session
	frame.pump()
	return frame
}

func (f *vr3Frame) pump() {
	f.t.Helper()
	if err := f.session.Driver().Pump(); err != nil {
		f.t.Fatal(err)
	}
}

func (f *vr3Frame) shot(name string) {
	f.t.Helper()
	path := filepath.Join(f.dir, name)
	if err := f.session.Screenshot(path); err != nil {
		f.t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		f.t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	f.t.Logf("VR3_CAPTURE path=%s", path)
}

// vr3Items 是这一帧的菜单行（Role=menuitem），按画面顺序。
func (f *vr3Frame) items() []gfx.Hit {
	out := make([]gfx.Hit, 0, 4)
	for _, hit := range f.session.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem {
			out = append(out, hit)
		}
	}
	return out
}

func (f *vr3Frame) labels() []string {
	out := make([]string, 0, 4)
	for _, item := range f.items() {
		out = append(out, item.Label)
	}
	return out
}

func vr3Contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// ── ① file target：非图片只给「另存为…」─────────────────────────────────────

// TestVR3FileMenuOnlySaveAsForNonImage 真右键一个 `src/main.go` 目标：
// 官方 file 菜单（order 200，偏移 33363246）在 `mediaType` 不是 image、`canSaveAs=true`
// 时只剩「另存为…」这一项——「下载」与「复制图片」都不该出现。
func TestVR3FileMenuOnlySaveAsForNonImage(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, SessionID: "s-vr3", Path: "src/main.go",
	}, func(string, string, string, string) {}, nil)
	if err := frame.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	frame.shot("fileactions-file-menu.png")

	got := frame.labels()
	if !vr3Contains(got, "另存为…") {
		t.Fatalf("file 菜单里没有「另存为…」：%v（官方 label:t('menu.saveAs')）", got)
	}
	if vr3Contains(got, "下载") {
		t.Errorf("非图片的 file 菜单出现了「下载」：%v（官方判据 mediaType.startsWith('image/')||!canSaveAs）", got)
	}
	if vr3Contains(got, "复制图片") {
		t.Errorf("file 菜单出现了「复制图片」：%v（官方只在 image/* 下给 copyImage）", got)
	}
	if frame.statCall == 0 {
		t.Errorf("打开 file 菜单没有先探测一次 stat（官方 resolve 里先 await workspaceFiles.stat）")
	}
	// 点这一项必须把 (sessionId, path, name, mode) 交回壳。
	item := frame.items()[0]
	if err := frame.session.Click(item.Bounds.X+item.Bounds.Width/2, item.Bounds.Y+item.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	if len(frame.saved) != 1 {
		t.Fatalf("点「另存为…」后壳动作调用=%d 次，want 1", len(frame.saved))
	}
	if want := "s-vr3|src/main.go|main.go|" + string(pluginapi.FileSaveAs); frame.saved[0] != want {
		t.Errorf("壳动作参数=%q，want %q（mode 必须是 saveAs）", frame.saved[0], want)
	}
}

// ── ② image target：copy + 下载 + 另存为（官方三项，逐条对齐）─────────────────

// TestVR3ImageMenuShowsCopyDownloadSaveAs image 菜单（order 100，偏移 33364653）：
// 官方给 `[copy, download, saveAs]`（:98-105），本仓 `#18` ① 闭合后**同样三项**。
// 这里钉住三件事：行数与顺序、三行都可点（这一族没有 disabled）、点「复制图片」
// 交回的是**打开菜单时捕获的** (sessionId, path)。
func TestVR3ImageMenuShowsCopyDownloadSaveAs(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetImage, SessionID: "s-vr3", Path: "assets/logo.png",
		Name: "logo.png", MediaType: "image/png",
	}, func(string, string, string, string) {}, nil)
	if err := frame.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	frame.shot("fileactions-image-menu.png")

	got := frame.labels()
	if len(got) != 3 || got[0] != "复制图片" || got[1] != "下载" || got[2] != "另存为…" {
		t.Fatalf("image 菜单=%v，want [复制图片 下载 另存为…]（官方 :98-105 的顺序）", got)
	}
	// image 族官方**没有 disabled**（偏移 33364653 全文无 disabled），所以三行都可点。
	for _, item := range frame.items() {
		if item.Disabled {
			t.Errorf("image 菜单行 %q 被置灰，官方 image 族没有 disabled 判据", item.Label)
		}
	}
	// stat 不该被 image 族调用（官方 image 的 resolve 里没有 stat）。
	if frame.statCall != 0 {
		t.Errorf("image 菜单调了 %d 次 stat，官方 image 族不 stat", frame.statCall)
	}
	// 点第一行「复制图片」：必须把捕获的 (sessionId, path) 交回壳。
	item := frame.items()[0]
	if err := frame.session.Click(item.Bounds.X+item.Bounds.Width/2, item.Bounds.Y+item.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	if len(frame.copied) != 1 {
		t.Fatalf("点「复制图片」后壳动作调用=%d 次，want 1（%v）", len(frame.copied), frame.copied)
	}
	if want := "s-vr3|assets/logo.png"; frame.copied[0] != want {
		t.Errorf("复制图片交回的是 %q，want 捕获的 %q", frame.copied[0], want)
	}
	if len(frame.saved) != 0 {
		t.Errorf("点「复制图片」不该触发保存动作：%v", frame.saved)
	}
}

// TestVR3FileMenuImageOffersCopyImage file 族的图片目标：官方三段展开条件在
// canSaveAs=true 下给 `[copyImage(separatorBefore), download, saveAs]`（:69-80）。
//
// 这一条与本包单测的区别是**真渲染**：行真的画进了菜单（不是 Resolve 的返回值）。
func TestVR3FileMenuImageOffersCopyImage(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, SessionID: "s-vr3", Path: "assets/logo.png",
	}, func(string, string, string, string) {}, nil)
	if err := frame.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	frame.shot("fileactions-file-menu-image.png")

	got := frame.labels()
	if len(got) != 3 || got[0] != "复制图片" || got[1] != "下载" || got[2] != "另存为…" {
		t.Fatalf("图片 file 菜单=%v，want [复制图片 下载 另存为…]", got)
	}
	if frame.statCall == 0 {
		t.Error("图片 file 菜单没有先探测 stat（官方 resolve 里先 await workspaceFiles.stat）")
	}
	item := frame.items()[0]
	if err := frame.session.Click(item.Bounds.X+item.Bounds.Width/2, item.Bounds.Y+item.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	if len(frame.copied) != 1 || frame.copied[0] != "s-vr3|assets/logo.png" {
		t.Fatalf("点「复制图片」后壳收到 %v，want [s-vr3|assets/logo.png]", frame.copied)
	}
}

// ── ③ stat 失败 → 两行都灰（官方 disabled:!available）──────────────────────

// TestVR3StatFailureDisablesItems 官方 file 族每一项都挂 `disabled:!available`
// （偏移 33363246）：`workspaceFiles.stat` 探测失败时**菜单照弹、动作灰着**。
func TestVR3StatFailureDisablesItems(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, SessionID: "s-vr3", Path: "src/main.go",
	}, func(string, string, string, string) {}, func(string, string) bool { return false })
	if err := frame.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	frame.shot("fileactions-stat-failed.png")

	items := frame.items()
	if len(items) == 0 {
		t.Fatalf("stat 失败时菜单整块没弹（官方只置灰、不藏菜单）")
	}
	for _, item := range items {
		if !item.Disabled {
			t.Errorf("stat 失败时菜单行 %q 仍可点（官方 disabled:!available）", item.Label)
		}
	}
	// 置灰是 opacity .4（`._item_70kcw_173:disabled{opacity:.4}`，rc.2 偏移 123324111）。
	theme := dscore.Dark()
	for _, one := range frame.session.Snapshot().Texts {
		if one.Value != "另存为…" {
			continue
		}
		want := theme.LabelPrimary.Alpha * 0.4
		if d := one.Style.Color.Alpha - want; d > 0.02 || d < -0.02 {
			t.Errorf("置灰行文字 alpha=%v，官方 opacity:.4 × label-primary(%v) = %v",
				one.Style.Color.Alpha, theme.LabelPrimary.Alpha, want)
		}
	}
	// 点灰行：动作不许执行。
	item := items[0]
	if err := frame.session.Click(item.Bounds.X+item.Bounds.Width/2, item.Bounds.Y+item.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	if len(frame.saved) != 0 {
		t.Errorf("点了置灰行仍调用了壳动作：%v", frame.saved)
	}
}

// ── ④ 身份不全 → 菜单整块不弹 ───────────────────────────────────────────────

// TestVR3MissingIdentityGivesNoMenu 没有 sessionId 时官方连 stat 都做不了；
// 本仓 `resolveFile` 直接返回 nil（plugin.go:196-205 的"身份不全"判据）。
func TestVR3MissingIdentityGivesNoMenu(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, Path: "src/main.go", // 故意不给 SessionID
	}, func(string, string, string, string) {}, nil)
	if err := frame.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame.pump()
	frame.shot("fileactions-no-identity.png")
	if items := frame.items(); len(items) != 0 {
		t.Errorf("缺 sessionId 时仍弹出了 %d 行菜单：%+v", len(items), items)
	}
	if frame.statCall != 0 {
		t.Errorf("缺身份时还调了 stat %d 次", frame.statCall)
	}

	// 壳没接 SaveWorkspaceFile（actions 缺）时同样一项都不给。
	frame2 := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, SessionID: "s-vr3", Path: "src/main.go",
	}, nil, nil)
	if err := frame2.session.RightClick(400, 300); err != nil {
		t.Fatal(err)
	}
	frame2.pump()
	frame2.shot("fileactions-no-save-action.png")
	if items := frame2.items(); len(items) != 0 {
		t.Errorf("壳没接 SaveWorkspaceFile 时仍弹出了 %d 行菜单", len(items))
	}
}

// ── ⑤ 本包不画任何常驻面 ────────────────────────────────────────────────────

// TestVR3PackagePaintsNoSurface 本包唯一的绘制入口是"只记账"的 painter：
// 它必须**一个图元都不加**，但每帧都要把 `env.Act` 记下来（否则菜单点了没反应）。
func TestVR3PackagePaintsNoSurface(t *testing.T) {
	dir := vr3CaptureDir(t)
	frame := vr3Open(t, dir, 900, 700, pluginapi.ContextMenuTarget{
		Kind: pluginapi.TargetFile, SessionID: "s-vr3", Path: "src/main.go",
	}, func(string, string, string, string) {}, nil)
	// 没开菜单的这一帧：画面上除底色外不该有本包的图元。
	snapshot := frame.session.Snapshot()
	theme := dscore.Dark()
	extra := 0
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			extra++
			continue
		}
		if primitive.Bounds.Width == 900 && primitive.Bounds.Height == 700 && primitive.Color == theme.BgBase {
			continue
		}
		extra++
	}
	if extra != 0 {
		t.Errorf("没开菜单时画面上有 %d 个非底色图元（本包不画任何常驻面）", extra)
	}
	// 每帧记录 Act：这是菜单能点到壳动作的前提。
	recorded := frame.plugin.act.SaveWorkspaceFile != nil
	if !recorded {
		t.Errorf("painter 没有把这一帧的 Actions.SaveWorkspaceFile 记下来——菜单动作必然点了没反应")
	}
	frame.shot("fileactions-no-surface.png")
}
