//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：快捷键面板（F1 键位表）。
//
// 前两轮看过的是「面板到底画没画出来」（overlay_test.go 的槽位证明）与
// 「阴影是不是 lv3」（sheet_elevation_test.go）。这一轮看**没被看过的四件事**：
//
//	① 官方 `.dialog` 的三条几何（宽 min(480,100%) / 高 min(600,dvh-108) / translateY 30）
//	   落到真帧上的矩形是多少（1180×700 与 900×700 两档）
//	② 列表真的滚在自己的视口里：15 条键位(42/行) → 内容高 660 > 视口 546，
//	   滚到底之前**最后一行画出去了但被裁掉了**（官方 `.list{overflow-y:auto}`）
//	③ 键盘焦点：本面板唯一的可交互元素是 `.close`。官方 Reference.module.css 给它
//	   写了 `:hover` 但**没有** `:focus-visible`（浏览器走 UA 默认描边），本仓用 viva
//	   的 Hit，默认不带任何焦点外观——这条要实测到底有没有可见反馈。
//	④ 窄窗口：官方 `.dialog{width:min(480px,100%)}` 没有下限，460 宽的窗口里
//	   面板必须正好等于窗口宽，不许溢出、不许把关闭按钮挤到面板外。
//
// 官方依据（唯一权威）：
//
//	OFF = dsh-ui/tmp/asar-css/104__Reference.module.css.mjs.css（单行压缩原件）
//	  :1  `.dialog{border-radius:24px;width:min(480px,100%);height:600px;
//	      max-height:calc(100dvh - 108px);transform:translateY(30px)}`
//	      `.header{flex:0 0 46px;padding:22px 54px 0 24px}`
//	      `.close{width:28px;height:28px;border-radius:28px;padding:6px;
//	              position:absolute;top:20px;right:14px}`
//	      `.list{flex:1;min-height:0;padding:0 16px 18px;overflow-y:auto}`
//	      `.row{min-height:42px;padding:8px;border-radius:12px;font-size:13px;line-height:18px}`
//	      `.keyBadge{height:26px;border-radius:10px;padding:0 12px;
//	                 background:var(--dsw-alias-bg-module-platform)}`
//	      `.binding .keyBadge{color:var(--dsw-alias-label-secondary)}`
//	      `.button:disabled{opacity:.45}`
//
// # 依据来源（rc.2 复核）
//
// `tmp/asar-css/104__Reference.module.css.mjs.css` 是 rc.1 抽取件；本轮把上面每一条
// 都拿 **rc.2 asar 原件**（`Desktop-0.2.2-rc.2/resources/app.asar` 里
// `@deepseek-ai/dsh-client-ui-shortcuts/Reference.module.css` 的 `css` 字符串、
// 类名前缀 `St8tea_`）逐条比对，结论是**逐字相同**：`.dialog` 的 480/600/108/24/
// translateY(30)、`.header` 46 与 `padding:22px 54px 0 24px`、`.close` 28×28/radius 28/
// top 20/right 14、`.list{padding:0 16px 18px}`、`.row{min-height:42px;padding:8px;
// border-radius:var(--dsw-radius-md);font-size:13px;line-height:18px}`、
// `.keyBadge{height:26px;border-radius:10px;padding:0 12px}`。
// rc.2 的全局焦点环出处也在 asar 里：`focus_css_default` 的
// `:root{--dsw-focus-ring-width:2px}` 与 `:focus-visible{outline-color:…;outline-width:…}`。
//
// 落盘目录：`$VR3_CAPTURE_DIR`（未设置时用 t.TempDir()）。
package dsshortcuts

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3ShotDir 是本次审计的落盘目录。
func vr3ShotDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// vr3Shot 用 session.Screenshot 落一张 PNG，并核它非空。
func vr3Shot(t *testing.T, session *headless.Session, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatalf("%s：截图失败：%v", name, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s：截图没落盘：%v", name, err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s：截图为空", name)
	}
	t.Logf("VR3_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

// vr3ShortcutRig 是「真 kernel + 真 dsshell + 本插件」的一帧环境，
// 与 overlay_test.go 的接线同源（槽位、真按键、真重绘）。
type vr3ShortcutRig struct {
	plugin      *Plugin
	theme       *pluginapi.Theme
	tree        *gfx.Tree
	feat        gfx.Scope
	blocked     string
	session     *headless.Session
	invalidator *app.Invalidator
}

// vr3OpenShortcuts 起一个窗口并返回 rig（面板此时还没打开）。
func vr3OpenShortcuts(t *testing.T, width, height int) *vr3ShortcutRig {
	t.Helper()
	kernel := uikernel.New("vr3-shortcuts")
	// 插件实例自己留着：本包没有对外服务键，`Load` 之后拿不到同一个指针。
	shortcutPlugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, shortcutPlugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	rig := &vr3ShortcutRig{
		plugin:      shortcutPlugin,
		theme:       theme,
		tree:        &gfx.Tree{},
		invalidator: &app.Invalidator{},
	}
	rig.feat = rig.tree.Feature("vr3-shortcuts")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "vr3-shortcuts", Width: width, Height: height,
		Tree: rig.tree, WindowMode: app.WindowFrameless, Invalidator: rig.invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			env := &pluginapi.Env{
				Theme: theme, Tree: rig.tree, Feat: rig.feat, Canvas: canvas, Bounds: bounds,
				Data: pluginapi.Snapshot{
					Cur:                       "vr3",
					DirectoryPickerAddBlocked: rig.blocked,
				},
				MainWidth: size.Width - 280,
				Geometry:  pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280},
			}
			env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
			env.Act.Notify = func(string) { rig.invalidator.Invalidate() }
			canvas.Fill(bounds, theme.BgBase)
			view.Draw(canvas, bounds, func(f ui.C) {
				kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
			})
		},
		// 滚轮必须接线：不设 OnScroll 时驱动会把 wheel 吞掉（cmd/app/main.go:2226-2234
		// 就是同一个接线），滚动区永远不动。
		OnScroll: func(ev gfx.ScrollEvent) gfx.EventResult { return view.Wheel(ev) },
		OnKey: func(ev gfx.KeyEvent) gfx.EventResult {
			if kernel.HandleKey(ev) {
				rig.invalidator.Invalidate()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	t.Cleanup(session.Close)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	rig.session = session
	return rig
}

// vr3OpenSheet 用**真按键** F1 打开面板，并等它画出来。
func (r *vr3ShortcutRig) vr3OpenSheet(t *testing.T) {
	t.Helper()
	if !r.plugin.shown {
		if err := r.session.Keys("f1"); err != nil {
			t.Fatalf("Keys(f1)：%v", err)
		}
	}
	if err := r.session.WaitFor(func(s app.Snapshot) bool {
		for _, text := range s.Texts {
			if text.Value == shortcutsTitle {
				return true
			}
		}
		return false
	}, 3*time.Second); err != nil {
		t.Fatalf("F1 之后面板标题没出现：texts=%q", r.session.Texts())
	}
}

// vr3RectOf 找第一个「颜色 + 圆角」都对得上的矩形图元。
func vr3RectOf(session *headless.Session, color gfx.Color, radius float32) (app.Primitive, bool) {
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		if radius >= 0 && primitive.Radius != radius {
			continue
		}
		return primitive, true
	}
	return app.Primitive{}, false
}

// vr3Rects 返回所有「颜色 + 圆角」都对得上的矩形图元（按绘制顺序）。
func vr3Rects(session *headless.Session, color gfx.Color, radius float32) []app.Primitive {
	var out []app.Primitive
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Color != color {
			continue
		}
		if radius >= 0 && primitive.Radius != radius {
			continue
		}
		out = append(out, primitive)
	}
	return out
}

// vr3Hit 按 label 找命中区。
func vr3Hit(session *headless.Session, label string) (gfx.Hit, bool) {
	for _, hit := range session.Snapshot().Hits {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// vr3TextBounds 返回值为 value 的文字这一帧的矩形（裁剪前的排版位置）。
func vr3TextBounds(session *headless.Session, value string) (gfx.Rect, bool) {
	for _, text := range session.Snapshot().Texts {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// vr3LabelDrawn 报告某个文字是否真的**落进了可见区域**（Snapshot.Labels 是裁剪后的）。
func vr3LabelDrawn(session *headless.Session, value string) bool {
	for _, label := range session.Snapshot().Labels {
		if label.Value == value {
			return true
		}
	}
	return false
}

// vr3Near 是 1 像素以内的浮点比较（布局会做分数像素）。
func vr3Near(got, want float32) bool {
	d := got - want
	return d <= 1 && d >= -1
}

// TestVR3ShortcutsDialogGeometry 用真帧核官方 `.dialog` 的三条几何（1180×700）。
//
// 官方式子：宽 min(480,100%)=480，高 min(600, dvh-108)=592，纵向居中后下移 30
// → top=(700-592)/2+30=84，left=(1180-480)/2=350。
func TestVR3ShortcutsDialogGeometry(t *testing.T) {
	dir := vr3ShotDir(t)
	rig := vr3OpenShortcuts(t, 1180, 700)
	rig.vr3OpenSheet(t)
	vr3Shot(t, rig.session, dir, "vr3-shortcuts-sheet.png")

	panel, ok := vr3RectOf(rig.session, rig.theme.InputMajor, sheetRadius)
	if !ok {
		t.Fatalf("真帧里没有圆角 %v 的面板底（颜色 input-major）：%+v",
			sheetRadius, rig.session.Snapshot().Primitives)
	}
	want := gfx.Rect{X: 350, Y: 84, Width: 480, Height: 592}
	if !vr3Near(panel.Bounds.X, want.X) || !vr3Near(panel.Bounds.Y, want.Y) ||
		!vr3Near(panel.Bounds.Width, want.Width) || !vr3Near(panel.Bounds.Height, want.Height) {
		t.Errorf("面板矩形 = %+v，官方 `width:min(480px,100%%);height:600px;max-height:calc(100dvh-108px);transform:translateY(30px)` "+
			"（104__Reference.module.css.mjs.css:1）算出来是 %+v", panel.Bounds, want)
	}

	// `.header{flex:0 0 46px}`：标题左缘 = 面板左 + 24（padding-left）。
	title, ok := vr3TextBounds(rig.session, shortcutsTitle)
	if !ok {
		t.Fatalf("面板标题没画出来：texts=%q", rig.session.Texts())
	}
	if wantX := panel.Bounds.X + sheetHeaderPadLeft; !vr3Near(title.X, wantX) {
		t.Errorf("标题左缘 x=%v，want %v（`.header{padding:22px 54px 0 24px}`）", title.X, wantX)
	}
	if limit := panel.Bounds.Y + sheetHeaderH; title.Y >= limit {
		t.Errorf("标题 y=%v 越过了 46 高的头部（面板顶 %v）——`.header{flex:0 0 46px}`", title.Y, panel.Bounds.Y)
	}

	// `.close{width:28px;height:28px;top:20px;right:14px}`：绝对定位在面板右上角。
	closeHit, ok := vr3Hit(rig.session, shortcutsCloseLabel)
	if !ok {
		t.Fatalf("没有关闭按钮的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	wantClose := gfx.Rect{
		X:     panel.Bounds.X + panel.Bounds.Width - sheetCloseRight - sheetCloseSize,
		Y:     panel.Bounds.Y + sheetCloseTop,
		Width: sheetCloseSize, Height: sheetCloseSize,
	}
	if !vr3Near(closeHit.Bounds.X, wantClose.X) || !vr3Near(closeHit.Bounds.Y, wantClose.Y) ||
		!vr3Near(closeHit.Bounds.Width, wantClose.Width) || !vr3Near(closeHit.Bounds.Height, wantClose.Height) {
		t.Errorf("关闭按钮 = %+v，want %+v（`.close{width:28px;height:28px;top:20px;right:14px}`）",
			closeHit.Bounds, wantClose)
	}

	// `.keyBadge{height:26px;border-radius:10px}`：每一条键位一枚，高度与圆角是硬值。
	badges := vr3Rects(rig.session, rig.theme.BgModulePlatform, sheetKeyRadius)
	if len(badges) == 0 {
		t.Fatal("真帧里一枚键位徽标都没有")
	}
	// 15 条键位 × 42 = 660 > 视口 546，所以**一定**有行被裁掉：可见徽标数必须少于行数。
	if len(badges) >= len(bindings) {
		t.Errorf("可见徽标 %d 枚 ≥ 键位 %d 条：内容没有被视口裁掉——"+
			"官方 `.list{flex:1;min-height:0;overflow-y:auto}`（104__Reference.module.css.mjs.css:1）",
			len(badges), len(bindings))
	}
	// 每一枚都必须**起于**面板内（`overflow-y:auto` 允许最后一行被下沿切一半，
	// 那是滚动区的正常外观；真正"画到面板外"由下面的像素断言卡）。
	for _, badge := range badges {
		if badge.Bounds.Y >= panel.Bounds.Y+panel.Bounds.Height {
			t.Errorf("徽标 %+v 整枚都在面板下沿 %v 之外", badge.Bounds, panel.Bounds.Y+panel.Bounds.Height)
		}
	}
	if len(badges) > 0 {
		if !vr3Near(badges[0].Bounds.Height, sheetKeyH) {
			t.Errorf("徽标高 %v，官方 `.keyBadge{height:26px}` = %v", badges[0].Bounds.Height, sheetKeyH)
		}
		// `.list{padding:0 16px 18px}` + `.row{padding:8px}` → 徽标右缘 = 面板右 - 24。
		if wantRight := panel.Bounds.X + panel.Bounds.Width - sheetListPadX - sheetRowPad; !vr3Near(badges[0].Bounds.X+badges[0].Bounds.Width, wantRight) {
			t.Errorf("徽标右缘 = %v，want %v（list padding 16 + row padding 8）",
				badges[0].Bounds.X+badges[0].Bounds.Width, wantRight)
		}
		// `.row{min-height:42px}`：相邻两枚徽标的纵向步长必须是 42。
		if len(badges) > 1 {
			if step := badges[1].Bounds.Y - badges[0].Bounds.Y; !vr3Near(step, sheetRowMinH) {
				t.Errorf("相邻行步长 %v，官方 `.row{min-height:42px}` = %v", step, sheetRowMinH)
			}
		}
	}
}

// TestVR3ShortcutsListScrollsInsideDialog 钉住官方的两段夹住：
// `.list{flex:1;min-height:0;overflow-y:auto}` 自己滚，`.dialog` 之外一个像素都不画。
//
// 判据三条：
//
//	① 第一屏：最后一条键位**排到了视口下方**（排版位置 > 面板底），且**没被画出来**
//	   （Snapshot.Labels 里没有）——这就是 overflow 裁剪真的生效；
//	② 滚到底之后：同一条文字落进面板内；
//	③ 面板底边以下整条带子里没有非背景像素（否则说明内容漏出了圆角外）。
func TestVR3ShortcutsListScrollsInsideDialog(t *testing.T) {
	dir := vr3ShotDir(t)
	rig := vr3OpenShortcuts(t, 1180, 700)
	rig.vr3OpenSheet(t)
	session := rig.session

	panel, ok := vr3RectOf(session, rig.theme.InputMajor, sheetRadius)
	if !ok {
		t.Fatal("真帧里找不到面板底")
	}
	last := bindings[len(bindings)-1]
	// ① 第一屏：最后一条在视口外。
	bounds, ok := vr3TextBounds(session, last.Key)
	if !ok {
		t.Fatalf("最后一条键位 %q 连排版都没排上：texts=%q", last.Key, session.Texts())
	}
	if bounds.Y < panel.Bounds.Y+panel.Bounds.Height {
		t.Logf("VR3 最后一条键位 y=%v 已在面板内（内容没有溢出，本用例的滚动判据不成立）", bounds.Y)
	} else if vr3LabelDrawn(session, last.Key) {
		t.Errorf("最后一条键位 %q 排在面板下沿之外（y=%v > %v）却被画进了可见区域——"+
			"官方 `.list{overflow-y:auto}` 应把它裁掉", last.Key, bounds.Y, panel.Bounds.Y+panel.Bounds.Height)
	}
	vr3Shot(t, session, dir, "vr3-shortcuts-scroll-top.png")

	// ② 滚到底：指针放在列表中部，向下滚一大段。
	listTop := panel.Bounds.Y + sheetHeaderH
	if err := session.Move(panel.Bounds.X+panel.Bounds.Width/2, listTop+40); err != nil {
		t.Fatal(err)
	}
	if err := session.Scroll(gfx.ScrollEvent{
		X: panel.Bounds.X + panel.Bounds.Width/2, Y: listTop + 40,
		DeltaY: 2000, Precise: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	after, ok := vr3TextBounds(session, last.Key)
	if !ok {
		t.Fatalf("滚动后最后一条键位消失了：texts=%q", session.Texts())
	}
	panelBottom := panel.Bounds.Y + panel.Bounds.Height
	if after.Y < listTop || after.Y+after.Height > panelBottom+1 {
		t.Errorf("滚到底之后最后一条键位仍在视口外：y=%v..%v，视口 %v..%v",
			after.Y, after.Y+after.Height, listTop, panelBottom)
	}
	if !vr3LabelDrawn(session, last.Key) {
		t.Errorf("滚到底之后最后一条键位 %q 仍没被画出来（Labels 里没有）", last.Key)
	}
	if after.Y >= bounds.Y {
		t.Errorf("滚动没有生效：滚动前后 y 都是 %v", bounds.Y)
	}
	vr3Shot(t, session, dir, "vr3-shortcuts-scroll-bottom.png")

	// ③ 面板底边以下不许有东西。背景色取窗口右下角（面板到不了那儿）。
	img, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	sample := img.Bounds()
	bg := img.RGBAAt(sample.Max.X-2, sample.Max.Y-2)
	for y := int(panelBottom) + 2; y < sample.Max.Y; y++ {
		for x := int(panel.Bounds.X); x < int(panel.Bounds.X+panel.Bounds.Width); x++ {
			if c := img.RGBAAt(x, y); !vr3SameColor(c, bg, 6) {
				t.Fatalf("面板下沿（y=%v）之外画出了东西：首处 (%d,%d)=%v；"+
					"官方 `.dialog` 的圆角 + `.list{overflow-y:auto}` 应该把内容夹在面板内",
					panelBottom, x, y, c)
			}
		}
	}
}

// vr3SameColor 是像素比较（含 8 位量化误差）。
func vr3SameColor(a, b color.RGBA, tolerance int) bool {
	d := vr3Abs(int(a.R) - int(b.R))
	d += vr3Abs(int(a.G) - int(b.G))
	d += vr3Abs(int(a.B) - int(b.B))
	d += vr3Abs(int(a.A) - int(b.A))
	return d <= tolerance
}

func vr3Abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestVR3ShortcutsNarrowWindowAndFocus 在 900×700 与 460×700 两档窗口里各画一帧：
//
//	① 900 宽：面板仍是官方设计宽 480（`width:min(480px,100%)`），水平居中；
//	② 460 宽：`min(480,100%)` 退到 100%，面板正好等于窗口宽、左缘 0，关闭按钮
//	   必须仍落在面板内部（不许被挤出去）；
//	③ 键盘焦点：唯一可交互的是关闭按钮。官方 css 只写 `.close:hover`，没有
//	   `:focus-visible`（浏览器用 UA 默认描边）。这里实测 Tab 之后**有没有任何**
//	   可见反馈——没有就是「键盘用户看不见焦点在哪」。
func TestVR3ShortcutsNarrowWindowAndFocus(t *testing.T) {
	dir := vr3ShotDir(t)

	// ① 900×700。
	nine := vr3OpenShortcuts(t, 900, 700)
	nine.vr3OpenSheet(t)
	panel, ok := vr3RectOf(nine.session, nine.theme.InputMajor, sheetRadius)
	if !ok {
		t.Fatal("900×700 里找不到面板底")
	}
	if !vr3Near(panel.Bounds.Width, sheetWidth) {
		t.Errorf("900 宽窗口里面板宽 %v，官方 `.dialog{width:min(480px,100%%)}` 应为 %v",
			panel.Bounds.Width, sheetWidth)
	}
	if wantLeft := float32((900 - sheetWidth) / 2); !vr3Near(panel.Bounds.X, wantLeft) {
		t.Errorf("900 宽窗口里面板左缘 %v，want %v（水平居中）", panel.Bounds.X, wantLeft)
	}
	vr3Shot(t, nine.session, dir, "vr3-shortcuts-900x700.png")

	// ② 460×700：min(480,100%) → 460。
	narrow := vr3OpenShortcuts(t, 460, 700)
	narrow.vr3OpenSheet(t)
	small, ok := vr3RectOf(narrow.session, narrow.theme.InputMajor, sheetRadius)
	if !ok {
		t.Fatal("460×700 里找不到面板底")
	}
	if !vr3Near(small.Bounds.Width, 460) {
		t.Errorf("460 宽窗口里面板宽 %v，官方 `min(480px,100%%)` 应为窗口宽 460", small.Bounds.Width)
	}
	if !vr3Near(small.Bounds.X, 0) {
		t.Errorf("460 宽窗口里面板左缘 %v，want 0", small.Bounds.X)
	}
	closeHit, ok := vr3Hit(narrow.session, shortcutsCloseLabel)
	if !ok {
		t.Fatalf("460 宽窗口里没有关闭按钮：hits=%+v", narrow.session.Snapshot().Hits)
	}
	if closeHit.Bounds.X+closeHit.Bounds.Width > small.Bounds.X+small.Bounds.Width+1 {
		t.Errorf("460 宽窗口里关闭按钮 %+v 被挤出了面板（面板右缘 %v）",
			closeHit.Bounds, small.Bounds.X+small.Bounds.Width)
	}
	// 窄窗里超长的「命令名」必须省略号收在自己的格子里，不许压到键位徽标上：
	// 官方 `.commandLabel{text-overflow:ellipsis;white-space:nowrap;min-width:0}
	// overflow:hidden`（104__Reference.module.css.mjs.css:1）
	if badges := vr3Rects(narrow.session, narrow.theme.BgModulePlatform, sheetKeyRadius); len(badges) > 0 {
		rowTop := badges[0].Bounds.Y
		badgeLeft := badges[0].Bounds.X
		for _, text := range narrow.session.Snapshot().Texts {
			if text.Value == shortcutsTitle || text.Value == shortcutsHint || text.Value == "" {
				continue
			}
			if text.Bounds.Y > rowTop+sheetRowMinH || text.Bounds.Y+text.Bounds.Height < rowTop {
				continue // 不是这一行的文字
			}
			if text.Bounds.X >= badgeLeft-1 {
				continue // 徽标自己的键名
			}
			if right := text.Bounds.X + text.Bounds.Width; right > badgeLeft-sheetRowGap+1 {
				t.Errorf("窄窗里 %q 的右缘 %v 压到了键位徽标左缘 %v——"+
					"官方 `.commandLabel{overflow:hidden;text-overflow:ellipsis}` + `.row{gap:8px}`",
					text.Value, right, badgeLeft)
			}
		}
	}
	vr3Shot(t, narrow.session, dir, "vr3-shortcuts-460x700.png")

	// ③ 键盘焦点：Tab 到关闭按钮，看画面上有没有可见变化。
	before, err := narrow.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	closeID := narrow.tree.ID(narrow.feat, "shortcuts-close")
	panelID := narrow.tree.ID(narrow.feat, "shortcuts-sheet")
	steps := 0
	for ; steps < 4; steps++ {
		if err := narrow.session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if err := narrow.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if narrow.session.Snapshot().Focused == closeID {
			break
		}
	}
	focused := narrow.session.Snapshot().Focused
	if focused != closeID {
		t.Fatalf("Tab 了 %d 次也没把焦点送到关闭按钮（focus=%v close=%v panel=%v）",
			steps, focused, closeID, panelID)
	}
	t.Logf("VR3 第 %d 次 Tab 把焦点送到关闭按钮（id=%v）", steps+1, focused)
	after, err := narrow.session.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 采样区 = 按钮矩形 **+ 外面 3px**：官方焦点态走全局 `outline`（画在元素外侧、
	// 宽 2px，focus.css），只量按钮内部会漏掉整圈环带。判据不变：环必须真的出现。
	changed := vr3PixelDiff(before, after, vr3OutsetRect(closeHit.Bounds, 3))
	t.Logf("VR3 焦点落在关闭按钮上时，按钮矩形（外扩 3px）内像素变化 = %d", changed)
	vr3Shot(t, narrow.session, dir, "vr3-shortcuts-close-focused.png")
	if changed == 0 {
		t.Errorf("键盘焦点落在关闭按钮上，按钮区域一个像素都没变——" +
			"键盘用户看不出焦点在哪。官方 `.close` 只有 `:hover`（104__Reference.module.css.mjs.css:1），" +
			"焦点态走浏览器 UA 默认描边；本仓用 viva 的 `HitWith`，不带任何焦点外观。" +
			"修法：`closeBtn.Focus(ui.Look{}.Ring(1, t.Info))`（viva 的 Ring 画在元素外侧，" +
			"正是 CSS `outline` 的语义；ui/style.go:369-377 的 paintRing）")
	}
}

// vr3OutsetRect 把矩形四边各外扩 d（官方 `outline` 落在元素外侧，采样要用它）。
func vr3OutsetRect(r gfx.Rect, d float32) gfx.Rect {
	return gfx.Rect{X: r.X - d, Y: r.Y - d, Width: r.Width + 2*d, Height: r.Height + 2*d}
}

// vr3PixelDiff 数两块同尺寸图在矩形内**不同**的像素个数。
func vr3PixelDiff(before, after *image.RGBA, r gfx.Rect) int {
	if before.Bounds() != after.Bounds() {
		return -1
	}
	count := 0
	for y := int(r.Y); y < int(r.Y+r.Height); y++ {
		for x := int(r.X); x < int(r.X+r.Width); x++ {
			if x < 0 || y < 0 || x >= before.Bounds().Max.X || y >= before.Bounds().Max.Y {
				continue
			}
			b, a := before.RGBAAt(x, y), after.RGBAAt(x, y)
			if b != a {
				count++
			}
		}
	}
	return count
}

// TestVR3ShortcutsBlockedAddLine 是「禁用 / 降级」这一档：
// 官方 `shortcut.noPicker` / `shortcut.directoryBusy` 的理由由壳算好交过来，
// 空串时**不许**多画一行（多画一行这份表就像在报错）。
func TestVR3ShortcutsBlockedAddLine(t *testing.T) {
	dir := vr3ShotDir(t)

	clean := vr3OpenShortcuts(t, 1180, 700)
	clean.vr3OpenSheet(t)
	for _, text := range clean.session.Texts() {
		if strings.HasPrefix(text, "Ctrl+O 当前不可用") {
			t.Fatalf("没有禁用理由时画出了 %q", text)
		}
	}
	vr3Shot(t, clean.session, dir, "vr3-shortcuts-no-blocked-reason.png")

	blocked := vr3OpenShortcuts(t, 1180, 700)
	blocked.blocked = "没有可选的目录"
	blocked.invalidator.Invalidate()
	blocked.vr3OpenSheet(t)
	found := false
	for _, text := range blocked.session.Texts() {
		if strings.HasPrefix(text, "Ctrl+O 当前不可用：没有可选的目录") {
			found = true
		}
	}
	if !found {
		t.Errorf("壳给了禁用理由却没画出来：texts=%q", blocked.session.Texts())
	}
	vr3Shot(t, blocked.session, dir, "vr3-shortcuts-blocked-reason.png")
}
