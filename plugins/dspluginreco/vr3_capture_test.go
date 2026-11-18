//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：官方插件推荐卡（enable_official_plugin 正文）。
//
// 前两轮验的是**状态机与几何常量表**（plugin_test.go / geometry_test.go 都不真开窗口）。
// 这一轮在真 headless 帧里看四件前两轮没看过的事：
//
//	① `.placeholder{width:50%;height:0.8em;border-radius:var(--dsw-radius-xs)}`
//	   那根骨架条在**加载态**里的真实矩形（宽是不是描述区的一半、高是不是 10.4）
//	② `.details:focus-visible` 的**键盘焦点**：官方是「底色换成 hover + 一圈 2px **内**描边」
//	③ 窄宽下 `.title{overflow-wrap:anywhere}` / `.description{overflow-wrap:anywhere}`
//	   ——长文案必须换行收在卡内，不是横着冲出去
//	④ 没有目录数据源（`RecoSource == nil`）时的退化成"只画标题 + 简介"
//
// 官方依据（**rc.2 asar 原件**，`@deepseek-ai/dsh-client-ui-plugin-recommendations/Card.module.css`）：
//
//	:1  `.header{display:flex;align-items:center;justify-content:space-between;gap:12px;
//	     padding:10px 20px;background:var(--dsw-alias-bg-module-platform);font-size:12px;
//	     line-height:1.5;color:var(--dsw-alias-label-secondary)}`
//	:3  `.body{padding:20px}`
//	:4  `.details{...width:calc(100% + 16px);margin:-8px;padding:8px;border:0;
//	     border-radius:var(--dsw-radius-sm);...}`
//	:5  `@media (hover:hover){.details:hover{background:var(--dsw-alias-interactive-bg-hover)}}`
//	:6  `.details:focus-visible{outline:none;background:var(--dsw-alias-interactive-bg-hover);
//	     box-shadow:inset 0 0 0 2px var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary))}`
//	:7  `.title{display:block;font-size:16px;font-weight:500;line-height:1.5;overflow-wrap:anywhere}`
//	:8  `.description{display:block;margin:8px 0 0;font-size:13px;line-height:1.7;
//	     color:var(--dsw-alias-label-secondary);overflow-wrap:anywhere}`
//	:9  `.placeholder{display:inline-block;width:50%;height:0.8em;vertical-align:middle;
//	     border-radius:var(--dsw-radius-xs);background:var(--dsw-alias-bg-skeleton)}`
//
// 焦点环宽度的全局出处：rc.2 asar 的 `focus.css` → `:root{--dsw-focus-ring-width:2px}`、
// `:focus-visible{outline-color:var(--dsw-focus-ring-color,…);outline-width:var(--dsw-focus-ring-width)}`。
//
// 上面 13 行已用 **rc.2 asar 原件**（`Desktop-0.2.2-rc.2/resources/app.asar` 里
// `plugin-recommendations/Card.module.css` 的未压缩文本）逐行复核：`:4/.details`、`:6`
// 的 `:focus-visible`（`outline:none` + hover 底色 + `box-shadow:inset 0 0 0 2px`）、
// `:7/.title{overflow-wrap:anywhere}`、`:8/.description`、`:9/.placeholder` 全部逐字一致。
// 环色 fallback `--dsw-alias-state-business-primary` 在 rc.2 dark 下 = `static-deepseek-400`
// （= 本仓 `Theme.Info`），light 下 = `static-deepseek-500`（同样 = 本仓 `Theme.Info`）。
//
// 落盘目录：`$VR3_CAPTURE_DIR`（未设置时用 t.TempDir()）。
package dspluginreco

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3RecoDir 是本次审计的落盘目录。
func vr3RecoDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// vr3RecoShot 用 session.Screenshot 落一张 PNG，并核它非空。
func vr3RecoShot(t *testing.T, session *headless.Session, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatalf("%s：截图失败：%v", name, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("%s：截图为空：path=%s info=%v err=%v", name, path, info, err)
	}
	t.Logf("VR3_CAPTURE path=%s bytes=%d", path, info.Size())
	return path
}

// vr3RecoRig 是「一张卡宽 620 的会话列」里画推荐卡正文的一帧环境。
type vr3RecoRig struct {
	plugin  *Plugin
	theme   *pluginapi.Theme
	tree    *gfx.Tree
	feat    gfx.Scope
	session *headless.Session
}

// vr3RecoProps 造一张卡的 props（官方 proposal 的逐字段投影）。
func vr3RecoProps(title, brief, name string) pluginapi.ActionCardBodyProps {
	return pluginapi.ActionCardBodyProps{
		SessionID: "vr3-session",
		CallID:    "call-1",
		Action:    Action,
		Title:     title,
		Brief:     brief,
		Details:   map[string]json.RawMessage{"name": json.RawMessage(`"` + name + `"`)},
		Act:       func(string, map[string]json.RawMessage) {},
	}
}

// vr3OpenRecoCard 起一个窗口画这张卡。width/height 是窗口尺寸，卡宽取 620（会话列）。
func vr3OpenRecoCard(t *testing.T, width, height int, cardWidth float32,
	props pluginapi.ActionCardBodyProps, p *Plugin) *vr3RecoRig {
	t.Helper()
	if p == nil {
		p = &Plugin{}
	}
	theme := dscore.Dark()
	p.theme = theme
	rig := &vr3RecoRig{plugin: p, theme: theme, tree: &gfx.Tree{}}
	rig.feat = rig.tree.Feature("vr3-pluginreco")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	invalidator := &app.Invalidator{}
	config := app.Config{
		Title: "vr3-pluginreco", Width: width, Height: height,
		Tree: rig.tree, WindowMode: app.WindowFrameless, Invalidator: invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			env := &pluginapi.Env{
				Theme: theme, Tree: rig.tree, Feat: rig.feat, Canvas: canvas, Bounds: bounds,
				MainWidth: size.Width - 280,
				Data:      pluginapi.Snapshot{Cur: "vr3-session"},
			}
			env.Act.Invalidate = func() { invalidator.Invalidate() }
			canvas.Fill(bounds, theme.BgBase)
			view.Draw(canvas, bounds, func(f ui.C) {
				ui.Column(f).Absolute().Left(0).Top(24).Width(cardWidth).Children(func() {
					p.Draw(f, env, props)
				})
			})
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

// vr3RecoRects 返回某种颜色的全部矩形图元。
func (r *vr3RecoRig) vr3RecoRects(color ui.Color) []app.Primitive {
	var out []app.Primitive
	for _, primitive := range r.session.Snapshot().Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == color {
			out = append(out, primitive)
		}
	}
	return out
}

// vr3RecoHit 按 label 找命中区。
func (r *vr3RecoRig) vr3RecoHit(label string) (gfx.Hit, bool) {
	for _, hit := range r.session.Snapshot().Hits {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// vr3RecoPixelDiff 数两块图在矩形内不同的像素数。
func vr3RecoPixelDiff(a, b *image.RGBA, rect gfx.Rect) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	count := 0
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
			if x < 0 || y < 0 || x >= a.Bounds().Max.X || y >= a.Bounds().Max.Y {
				continue
			}
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				count++
			}
		}
	}
	return count
}

// TestVR3PluginRecoLoadingSkeleton 是**加载态**：目录文案还没到，
// 官方画 `.placeholder` 骨架条（宽 50%、高 0.8em=10.4、圆角 4、色 bg-skeleton）占位。
func TestVR3PluginRecoLoadingSkeleton(t *testing.T) {
	dir := vr3RecoDir(t)
	// 查询发出去但**不落地**：停在 loading 态（产品路径是 goroutine，这里挂起）。
	pending := &deferred{}
	source := &fakeSource{details: availableDetails("示例插件", "一段目录文案", false)}
	SetSource(source)
	// `.details` 只有在宿主给了插件页（`currentOpenDetails() != nil`）时才是命中区；
	// 这里给它，好让"骨架条宽度 = 描述区一半"这条断言有一个可比的真值。
	SetOnOpenDetails(func(string) {})
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })

	p := &Plugin{run: pending.run}
	props := vr3RecoProps("示例插件", "简介", "demo-plugin")
	rig := vr3OpenRecoCard(t, 900, 700, 620, props, p)
	vr3RecoShot(t, rig.session, dir, "vr3-pluginreco-loading.png")

	// 骨架条：颜色 bg-skeleton、圆角 --dsw-radius-xs = 4。
	placeholders := rig.vr3RecoRects(rig.theme.BgSkeleton)
	if len(placeholders) != 1 {
		t.Fatalf("加载态骨架条 %d 根，want 1（官方 `.placeholder`，Card.module.css:9）", len(placeholders))
	}
	placeholder := placeholders[0].Bounds
	// 高度 = `.description` 字号 13 × 0.8em = 10.4（像素对齐后 10 或 11）。
	if placeholder.Height < 10 || placeholder.Height > 11 {
		t.Errorf("骨架条高 %v，want ≈10.4（官方 `.placeholder{height:0.8em}`，13px 字号下 = 10.4）",
			placeholder.Height)
	}
	if placeholders[0].Radius != rig.theme.Radii.XS {
		t.Errorf("骨架条圆角 %v，官方 `.placeholder{border-radius:var(--dsw-radius-xs)}` = %v",
			placeholders[0].Radius, rig.theme.Radii.XS)
	}
	// 宽 = 描述区宽的 50%。描述区 = `.details` 内容宽（`.body{padding:20px}` 内再减
	// `.details{padding:8px}`）；用「查看详情」命中区（就是 `.details`）反算。
	details, ok := rig.vr3RecoHit(detailsLabel("示例插件"))
	if !ok {
		t.Fatalf("没有 `.details` 的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	contentWidth := details.Bounds.Width - 2*recoDetailsPad
	want := contentWidth * recoPlaceholderPercent / 100
	if d := placeholder.Width - want; d > 2 || d < -2 {
		t.Errorf("骨架条宽 %v，want %v（官方 `.placeholder{width:50%%}`，描述区宽 %v）",
			placeholder.Width, want, contentWidth)
	}
	// `.details` 必须真的外扩 8px（`margin:-8px; width:calc(100% + 16px)`，:4）：
	// 卡左 0 + `.body{padding:20px}` − 8 = 12。
	if wantX := recoBodyPad - recoDetailsBleed; !vr3RecoClose(int(details.Bounds.X), int(wantX)) {
		t.Errorf("`.details` 左缘 %v，want %v（`.body{padding:20px}` + `.details{margin:-8px}`，:3-4）",
			details.Bounds.X, wantX)
	}
	const probeCardWidth float32 = 620
	if wantW := probeCardWidth - 2*recoBodyPad + 2*recoDetailsBleed; !vr3RecoClose(int(details.Bounds.Width), int(wantW)) {
		t.Errorf("`.details` 宽 %v，want %v（`.details{width:calc(100%% + 16px)}`，:4）",
			details.Bounds.Width, wantW)
	}
	// 加载态必须画得出「推荐插件」通栏头部（`.header`，:1）与两个页面文案。
	for _, want := range []string{zhHeading, zhOfficialBadge, "示例插件"} {
		if !rig.session.HasText(want) {
			t.Errorf("加载态缺 %q：texts=%q", want, rig.session.Texts())
		}
	}
	if len(pending.fns) == 0 {
		t.Fatal("加载态没有发出 describe（挂起的执行器里一条都没有）")
	}
}

// TestVR3PluginRecoDetailsFocusRing 走**真键盘** Tab 到 `.details` 上：
// 官方要求「底色换 hover + 一圈 2px **内**描边」（Card.module.css:6）。
func TestVR3PluginRecoDetailsFocusRing(t *testing.T) {
	dir := vr3RecoDir(t)
	SetSource(&fakeSource{details: availableDetails("示例插件", "一段目录文案", false)})
	SetOnOpenDetails(func(string) {})
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })

	p := &Plugin{run: func(fn func()) { fn() }}
	props := vr3RecoProps("示例插件", "简介", "demo-plugin")
	rig := vr3OpenRecoCard(t, 900, 700, 620, props, p)

	details, ok := rig.vr3RecoHit(detailsLabel("示例插件"))
	if !ok {
		t.Fatalf("没有「查看详情」的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	// 指针先离开：焦点态必须自己可见，不靠 hover。
	if err := rig.session.Move(880, 680); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	before, err := rig.session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	vr3RecoShot(t, rig.session, dir, "vr3-pluginreco-details-rest.png")

	wantID := rig.tree.ID(rig.feat, "reco-details-call-1")
	reached := false
	for step := 0; step < 4; step++ {
		if err := rig.session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if err := rig.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if rig.session.Snapshot().Focused == wantID {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("Tab 走不到 `.details`（focus=%v want=%v）", rig.session.Snapshot().Focused, wantID)
	}
	after, err := rig.session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	vr3RecoShot(t, rig.session, dir, "vr3-pluginreco-details-focused.png")

	// ① 底色：官方 `:focus-visible{background:var(--dsw-alias-interactive-bg-hover)}`。
	if changed := vr3RecoPixelDiff(before, after, details.Bounds); changed == 0 {
		t.Errorf("焦点落在 `.details` 上，整块一个像素都没变——连官方那条 `background:" +
			"var(--dsw-alias-interactive-bg-hover)`（Card.module.css:6）都没有")
	} else {
		t.Logf("VR3 焦点前后 `.details` 内变化像素 = %d（底色已按 :6 换 hover）", changed)
	}
	// ② 2px 内描边：官方 `box-shadow:inset 0 0 0 2px var(--dsw-focus-ring-color,…)`。
	//    焦点态下，离边 1px 的那一圈必须是**焦点环色**（business-primary），
	//    而不是 hover 叠色。
	ring := rig.theme.Info // --dsw-alias-state-business-primary
	edge := after.RGBAAt(int(details.Bounds.X+2), int(details.Bounds.Y+details.Bounds.Height/2))
	inner := after.RGBAAt(int(details.Bounds.X+details.Bounds.Width/2), int(details.Bounds.Y+details.Bounds.Height/2))
	if vr3RecoNear(edge, ring) && !vr3RecoNear(inner, ring) {
		t.Logf("VR3 焦点内描边已画：edge=%v inner=%v", edge, inner)
	} else {
		t.Errorf("`.details` 的 2px 内描边没有画：左缘内 2px 的像素 = %v，内部 = %v，"+
			"焦点环色 `--dsw-alias-state-business-primary` = %v。"+
			"官方 `.details:focus-visible{outline:none;background:interactive-bg-hover;"+
			"box-shadow:inset 0 0 0 2px var(--dsw-focus-ring-color,…)}`（rc.2 asar 的 "+
			"plugin-recommendations/Card.module.css:6；宽度 2px 见 rc.2 focus.css 的 "+
			"`:root{--dsw-focus-ring-width:2px}`）。根因：plugin.go:427-432 的 `.details` 只有 "+
			"`Focus(ui.Look{}.Background(t.Hover))`，没有 ring；文件头「没搬」第 1 条把原因写成 "+
			"「viva 的 Ring 画在外侧、与 inset 不是一回事」——这条理由只对 `Look.Ring` 成立。"+
			"inset 可以用 `DrawOver` + `canvas.RoundedOutline` 精确画出：把矩形四边各内缩 1px、"+
			"半径减 1、线宽 2，描边中心正好落在 0..2px 的环带上（plugins/dsplan/focus.go:16-25 "+
			"就是同一手法），不必改 pluginapi", edge, inner, ring)
	}
}

// vr3RecoNear 是像素与主题色的近似比较（±3/通道）。
func vr3RecoNear(pixel interface {
	RGBA() (uint32, uint32, uint32, uint32)
}, want ui.Color) bool {
	r, g, b, _ := pixel.RGBA()
	wr := uint32(want.Red*255 + 0.5)
	wg := uint32(want.Green*255 + 0.5)
	wb := uint32(want.Blue*255 + 0.5)
	return vr3RecoClose(int(r>>8), int(wr)) && vr3RecoClose(int(g>>8), int(wg)) && vr3RecoClose(int(b>>8), int(wb))
}

func vr3RecoClose(a, b int) bool {
	d := a - b
	return d <= 3 && d >= -3
}

// TestVR3PluginRecoLongTextWrapsInsideCard 是**窄宽**：官方给 `.title` 与 `.description`
// 都写了 `overflow-wrap:anywhere`（Card.module.css:7-8），长文案必须换行留在卡内。
func TestVR3PluginRecoLongTextWrapsInsideCard(t *testing.T) {
	dir := vr3RecoDir(t)
	long := strings.Repeat("supercalifragilisticexpialidocious-", 8)
	SetSource(&fakeSource{details: availableDetails("示例插件", long, false)})
	SetOnOpenDetails(func(string) {})
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })

	p := &Plugin{run: func(fn func()) { fn() }}
	props := vr3RecoProps("示例插件", "简介", "demo-plugin")
	const cardWidth float32 = 520 // 900 宽窗口里一条窄会话列
	rig := vr3OpenRecoCard(t, 900, 700, cardWidth, props, p)
	vr3RecoShot(t, rig.session, dir, "vr3-pluginreco-900x700-long-description.png")

	details, ok := rig.vr3RecoHit(detailsLabel("示例插件"))
	if !ok {
		t.Fatalf("没有「查看详情」的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	// 描述文字必须留在 `.details` 的内容框里（`.details{padding:8px}`）。
	//
	// 官方 `.description` 是**块级 + overflow-wrap:anywhere**（:8）：这段 272 字的长文
	// 在 580 宽的描述区里要折成好几行（行高 22.1）。折行后 viva 每行提交一条
	// `gfx.Text`，所以判据是"落在这段描述上的行 ≥ 2 **且** 最后一行收在原文末尾"
	// ——单行 + clip 的实现只会有 1 条、且末尾不是原文的尾巴。
	limit := details.Bounds.X + details.Bounds.Width - recoDetailsPad
	lastRune := []rune(long)[len([]rune(long))-1]
	var shown []gfx.Text
	for _, text := range rig.session.Snapshot().Texts {
		if text.Value == "" || !strings.Contains(long, text.Value) {
			continue // 不是这段描述的一条
		}
		if text.Bounds.Y < details.Bounds.Y {
			continue
		}
		shown = append(shown, text)
		if right := text.Bounds.X + text.Bounds.Width; right > limit+1 {
			t.Errorf("描述文字右缘 %v 冲出了 `.details` 内容右缘 %v（宽 %v > 可用 %v）",
				right, limit, text.Bounds.Width, limit-text.Bounds.X)
		}
	}
	if len(shown) == 0 {
		t.Fatalf("长描述一个字都没画出来：texts=%q", rig.session.Texts())
	}
	if len(shown) < 2 {
		t.Errorf("长描述只画出 %d 行（单行 %v 高）——尾巴被裁掉了，用户看不到后半句。"+
			"官方 `.description{line-height:1.7;overflow-wrap:anywhere}`（rc.2 "+
			"plugin-recommendations/Card.module.css:8）是块级段落，会折成多行；"+
			"`.title{...overflow-wrap:anywhere}`（:7）同理。首个条目 = %q",
			len(shown), shown[0].Bounds.Height, shown[0].Value)
	}
	// 尾巴必须真的在：最后一行以原文最后一个字收尾。
	last := shown[len(shown)-1]
	if runes := []rune(last.Value); len(runes) == 0 || runes[len(runes)-1] != lastRune {
		t.Errorf("折行后最后一行是 %q，没有收在原文末尾（%q）——说明尾巴仍然被裁掉",
			last.Value, string(lastRune))
	}
	t.Logf("VR3 长描述折成 %d 行，末行 = %q", len(shown), last.Value)
	// 卡内不许出现被裁掉一半的文字：`.card` 的框由动作卡框架给，这里只核
	// 描述最高不越过「窗口高度」这种硬失败。
	if details.Bounds.Width > cardWidth {
		t.Errorf("`.details` 宽 %v 超过了卡宽 %v——官方 `.details{width:calc(100%% + 16px)}` 只该外扩 16",
			details.Bounds.Width, cardWidth)
	}
}

// TestVR3PluginRecoNoSourceDegrades 是没有目录数据源（`RecoSource == nil`）时的退化：
// 只画标题 + 简介，**不画**通栏头部（「推荐插件」/「官方」）与两组动作按钮。
func TestVR3PluginRecoNoSourceDegrades(t *testing.T) {
	dir := vr3RecoDir(t)
	SetSource(nil)
	SetOnOpenDetails(nil)
	t.Cleanup(func() { SetSource(nil); SetOnOpenDetails(nil) })

	p := &Plugin{}
	props := vr3RecoProps("示例插件", "一句简介", "demo-plugin")
	rig := vr3OpenRecoCard(t, 900, 700, 620, props, p)
	vr3RecoShot(t, rig.session, dir, "vr3-pluginreco-no-source.png")

	if !rig.session.HasText("示例插件") {
		t.Errorf("没有数据源时连标题都不画了：texts=%q", rig.session.Texts())
	}
	if !rig.session.HasText("一句简介") {
		t.Errorf("没有数据源时简介也消失了：texts=%q", rig.session.Texts())
	}
	for _, unwanted := range []string{zhHeading, zhOfficialBadge, zhEnable, zhDismiss, zhScope} {
		if rig.session.HasText(unwanted) {
			t.Errorf("没有数据源时画出了 %q——官方 `inject: ['remote', …]` 缺失时这张卡退化成只读标题， "+
				"不该画头部/按钮（点了没反应比不画更糟）", unwanted)
		}
	}
	// 退化态不该有「查看详情」的可点入口（宿主没有插件页）。
	if _, ok := rig.vr3RecoHit(detailsLabel("示例插件")); ok {
		t.Errorf("没有数据源时仍挂了 `.details` 的点击入口（点了没反应）")
	}
	_ = apiclient.PluginRecommendationDetails{}
}
