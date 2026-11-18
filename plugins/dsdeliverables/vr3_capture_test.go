//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：交付文件（侧栏条目 + 预览浮层）。
//
// 前两轮看的是「浮层里有没有真的 hunk」（render_test.go，裸 canvas + 假 changes 源）。
// 这一轮换真 headless + 真指针/键盘，专挑前两轮没看过的三件事：
//
//	① 卡片本身的官方几何与**字号/行高/字重**：`.file{height:60px;padding:8px 10px;
//	   gap:10px;border:.5px} / .fileName{13px/500/20px} / .description{10px/16px}`
//	② **键盘焦点环**：官方 `.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px
//	   var(--dsw-focus-ring-color,...);outline:none}`——Tab 走进去到底有没有那条 2px 内描边
//	③ **空态**与**长列表**：没有交付物时是不是真的一个像素都不画；6 个文件时是否
//	   只画 4 张卡 + 「全部 6 个文件」（官方 COLLAPSED_PRESENTED_COUNT=4）
//	④ 窄宽 900×700 下省略号是否收在卡内
//
// 官方依据（唯一权威，单行压缩原件）：
//
//	OFF = dsh-ui/tmp/asar-css/048__Deliverables.module.css.mjs.css:1
//	  `.root{gap:16px;margin-top:4px;display:flex;flex-direction:column}`
//	  `.presented{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}`
//	  `@container (width<=620px){.presented{grid-template-columns:minmax(0,1fr)}}`
//	  `.file{box-sizing:border-box;border:.5px solid var(--dsw-alias-border-l1);
//	         border-radius:var(--dsw-radius-lg);background:var(--deliverable-fill);
//	         height:60px;padding:8px 10px;gap:10px;display:flex;overflow:hidden}`
//	  `.file:hover{background:var(--deliverable-hover)}`
//	  `.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px
//	         var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));outline:none}`
//	  `.fileIcon{width:40px;height:40px;border-radius:10px;border:.5px solid border-l1}`
//	  `.fileName{font-size:13px;font-weight:500;line-height:20px}`
//	  `.description{font-size:10px;line-height:16px;color:label-tertiary}`
//
// # 依据来源（rc.2 复核）
//
// `tmp/asar-css/048__Deliverables.module.css.mjs.css` 是 rc.1 抽取件。本轮把上面每一条
// 都拿 **rc.2 asar 原件**（`Desktop-0.2.2-rc.2/resources/app.asar` 里
// `@deepseek-ai/dsh-client-ui-deliverables/Deliverables.module.css` 的 `css$1` 字符串，
// 类名前缀仍是 `K5rwea_`）逐条比对：
//
//	相同：`.file{height:60px;padding:8px 10px;gap:10px;border:.5px}`、
//	      `.fileName{13px/500/20px}`、`.description{10px/400/16px}`、
//	      `.fileIcon{40x40;border-radius:10px}`、`.root{gap:16px;margin-top:4px}`、
//	      `.presented{repeat(2,minmax(0,1fr));gap:10px}` + `@container (width<=620px)` 单列、
//	      `.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px …}`（**逐字相同**）
//	rc.2 的差异（都不改本文的断言）：`--deliverable-fill` 从 `static-neutral-50` 改指
//	      `--dsw-alias-card-fill`、`--deliverable-hover` 改指 `--dsw-alias-card-fill-hover`
//	      （rc.2 asar 里 dark = neutral-850 / neutral-800，正是本仓 `Theme.CardFill` /
//	      `CardFillHover` 的取值）；`.fileIcon` 的底色改指 `--dsw-alias-card-tile-fill`
//	      （= neutral-00 5%，即本仓 `Theme.CardTileFill`）；两组 `:hover` 外层多了
//	      `@media (hover:hover)`。
//
// 落盘目录：`$VR3_CAPTURE_DIR`（未设置时用 t.TempDir()）。
package dsdeliverables

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3DelivShot 落一张 PNG 并核非空。
func vr3DelivShot(t *testing.T, session *headless.Session, dir, name string) string {
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

// vr3DelivRig 是「侧栏宽 280 的一条列 + 浮层」的一帧环境，与壳的
// `SlotSidebarFooter` / `SlotShellOverlay` 两个座位同形。
type vr3DelivRig struct {
	plugin  *Plugin
	theme   *pluginapi.Theme
	session *headless.Session
	tree    *gfx.Tree
	feat    gfx.Scope
}

// vr3WriteMsg 造一条"写了某个文件"的工具消息。
func vr3WriteMsg(path string) apiclient.Message {
	return apiclient.Message{Role: "tool", Tool: "write",
		Args: `{"filePath":` + vr3Quote(path) + `,"content":"x"}`}
}

// vr3Quote 是最小的 JSON 字符串引号（测试夹具里路径不含怪字符）。
func vr3Quote(value string) string {
	out := make([]byte, 0, len(value)+2)
	out = append(out, '"')
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '\\' || c == '"' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(append(out, '"'))
}

// vr3OpenDeliverables 起一个窗口；msgs 是这一帧消息流里的工具调用。
//
// 侧栏列固定在 (0, 150)、宽 280——真壳里它是 `sidebar.footer` 槽，
// 位置由壳给，这里只要一个确定的锚点。
func vr3OpenDeliverables(t *testing.T, width, height int, msgs []apiclient.Message, frames []apiclient.Frame) *vr3DelivRig {
	t.Helper()
	kernel := uikernel.New("vr3-deliverables")
	plugin := &Plugin{}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, plugin); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	rig := &vr3DelivRig{plugin: plugin, theme: theme, tree: &gfx.Tree{}}
	rig.feat = rig.tree.Feature("vr3-deliverables")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	invalidator := &app.Invalidator{}
	const sideTop float32 = 150
	const sideWidth float32 = 280
	config := app.Config{
		Title: "vr3-deliverables", Width: width, Height: height,
		Tree: rig.tree, WindowMode: app.WindowFrameless, Invalidator: invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			env := &pluginapi.Env{
				Theme: theme, Tree: rig.tree, Feat: rig.feat, Canvas: canvas, Bounds: bounds,
				MainWidth: size.Width - sideWidth,
				Data:      pluginapi.Snapshot{Cur: "vr3-session", Msgs: msgs, Frames: frames},
				Geometry:  pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: sideWidth},
			}
			env.Act.Invalidate = func() { invalidator.Invalidate() }
			canvas.Fill(bounds, theme.BgBase)
			view.Draw(canvas, bounds, func(f ui.C) {
				ui.Column(f).Absolute().Left(0).Top(sideTop).Width(sideWidth).Children(func() {
					plugin.draw(f, env)
				})
				plugin.drawPreview(f, env)
			})
		},
		OnScroll: func(ev gfx.ScrollEvent) gfx.EventResult { return view.Wheel(ev) },
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

// vr3DelivHit 按 id 后缀找命中区。
func (r *vr3DelivRig) vr3DelivHit(name string) (gfx.Hit, bool) {
	id := r.tree.ID(r.feat, name)
	for _, hit := range r.session.Snapshot().Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// vr3DelivRects 返回某种颜色的全部矩形图元。
func (r *vr3DelivRig) vr3DelivRects(color ui.Color) []app.Primitive {
	var out []app.Primitive
	for _, primitive := range r.session.Snapshot().Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == color {
			out = append(out, primitive)
		}
	}
	return out
}

// vr3DelivText 按值找这一帧排版的文字。
func (r *vr3DelivRig) vr3DelivText(value string) (gfx.Text, bool) {
	for _, text := range r.session.Snapshot().Texts {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// vr3DelivClose 是两个浮点是否在 1px 以内。
func vr3DelivClose(got, want float32) bool {
	d := got - want
	return d <= 1 && d >= -1
}

// TestVR3DeliverablesCardGeometryAndTypography 逐条对照官方 `.file` / `.fileName` /
// `.description` 的数值，并真指针悬停验证 `.file:hover` 的换色。
func TestVR3DeliverablesCardGeometryAndTypography(t *testing.T) {
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	msgs := []apiclient.Message{
		vr3WriteMsg("src/a.go"),
		vr3WriteMsg("web/index.html"),
		vr3WriteMsg("README.md"),
		vr3WriteMsg("go.mod"),
		vr3WriteMsg("docs/spec.md"),
		vr3WriteMsg("pkg/this-is-a-very-long-file-name-that-must-be-ellipsized.go"),
	}
	rig := vr3OpenDeliverables(t, 1180, 700, msgs, nil)

	// ① 标题在 `.root{margin-top:4px}` 之后。
	title, ok := rig.vr3DelivText(msgRowTitle)
	if !ok {
		t.Fatalf("侧栏没有画「%s」：texts=%v", msgRowTitle, rig.session.Texts())
	}
	if wantY := float32(150) + cardRootMarginTop; !vr3DelivClose(title.Bounds.Y, wantY) {
		t.Errorf("标题 y=%v，want %v（`.root{margin-top:4px}`，048:1）", title.Bounds.Y, wantY)
	}

	// ② 只画 4 张卡（官方 COLLAPSED_PRESENTED_COUNT=4）。
	//
	// 几何用**命中区**量：viva 把圆角矩形的**填充**内缩一圈画（外框半径 16、
	// 填充外扩 1px），所以 `.file` 的底色块会比盒子少 2px（见
	// plugins/dsskill/skillrow_visual_test.go:95-98 的同一条事实）。
	fills := rig.vr3DelivRects(rig.theme.CardFill)
	if len(fills) != maxShown {
		t.Fatalf("卡片底色块 %d 块，want %d（官方折叠态只列 4 条）", len(fills), maxShown)
	}
	var cards []gfx.Hit
	for i := 0; i < maxShown; i++ {
		hit, ok := rig.vr3DelivHit("deliv-" + vr3DelivItoa(i))
		if !ok {
			t.Fatalf("没有 deliv-%d 的命中区：hits=%+v", i, rig.session.Snapshot().Hits)
		}
		cards = append(cards, hit)
	}
	for i, card := range cards {
		if !vr3DelivClose(card.Bounds.Height, cardHeight) {
			t.Errorf("第 %d 张卡高 %v，官方 `.file{height:60px}`（048:1）= %v",
				i, card.Bounds.Height, cardHeight)
		}
		if !vr3DelivClose(card.Bounds.Width, 280) {
			t.Errorf("第 %d 张卡宽 %v，want 280（侧栏列宽，`.presented` 在 ≤620px 容器里单列）",
				i, card.Bounds.Width)
		}
		if i > 0 {
			// `.root{gap:16px}` 是这一列唯一给出的间距（`.presented{gap:10px}` 是网格的列/gap，
			// 单列折叠后每张卡独占一行，行间距由 root 的 16 决定）。
			gap := card.Bounds.Y - (cards[i-1].Bounds.Y + cards[i-1].Bounds.Height)
			if !vr3DelivClose(gap, cardRootGap) {
				t.Errorf("相邻卡片间距 %v，want %v（官方 `.root{gap:16px}`，048:1）", gap, cardRootGap)
			}
		}
	}

	// ③ `.fileIcon{width:40px;height:40px;border-radius:10px}`。
	icons := rig.vr3DelivRects(rig.theme.CardTileFill)
	if len(icons) != maxShown {
		t.Fatalf("文件图标 %d 枚，want %d", len(icons), maxShown)
	}
	for _, icon := range icons {
		// 填充内缩 1px（0.5px 边框在 1x 下量成 1 个物理像素），所以 40 的盒子
		// 底色块是 38；圆角同理 10→9。两者都要在 2px 以内。
		if icon.Bounds.Width < cardIconSize-2 || icon.Bounds.Width > cardIconSize {
			t.Errorf("图标底色块宽 %v，官方 `.fileIcon{width:40px}`（填充内缩 1px 后应为 38..40）",
				icon.Bounds.Width)
		}
		if icon.Bounds.Height < cardIconSize-2 || icon.Bounds.Height > cardIconSize {
			t.Errorf("图标底色块高 %v，官方 `.fileIcon{height:40px}`（填充内缩 1px 后应为 38..40）",
				icon.Bounds.Height)
		}
		if icon.Radius > cardIconRadius || icon.Radius < cardIconRadius-2 {
			t.Errorf("图标圆角 %v，官方 `.fileIcon{border-radius:10px}` = %v（填充内缩后可为 9）",
				icon.Radius, cardIconRadius)
		}
	}

	// ④ `.fileName{font-size:13px;font-weight:500;line-height:20px}`。
	name, ok := rig.vr3DelivText("a.go")
	if !ok {
		t.Fatalf("画面里没有文件名 a.go：texts=%v", rig.session.Texts())
	}
	if name.Style.Size != cardTitleFont {
		t.Errorf("文件名字号 %v，官方 `.fileName{font-size:13px}` = %v", name.Style.Size, cardTitleFont)
	}
	if name.Style.Weight != cardTitleWght {
		t.Errorf("文件名字重 %v，官方 `.fileName{font-weight:500}` = %v", name.Style.Weight, cardTitleWght)
	}
	if !vr3DelivClose(name.Bounds.Height, cardTitleLine) {
		t.Errorf("文件名单行高 %v，官方 `.fileName{line-height:20px}` = %v", name.Bounds.Height, cardTitleLine)
	}
	// ⑤ `.description{font-size:10px;line-height:16px}`：扩展名大写那一行。
	ext, ok := rig.vr3DelivText("GO")
	if !ok {
		t.Fatalf("画面里没有扩展名行 GO（官方 `fileExtension(name).toUpperCase()`）：texts=%v", rig.session.Texts())
	}
	if ext.Style.Size != cardDescFont {
		t.Errorf("描述行字号 %v，官方 `.description{font-size:10px}` = %v", ext.Style.Size, cardDescFont)
	}
	if !vr3DelivClose(ext.Bounds.Height, cardDescLine) {
		t.Errorf("描述行行高 %v，官方 `.description{line-height:16px}` = %v", ext.Bounds.Height, cardDescLine)
	}

	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-cards.png")

	// ⑥ `.file:hover{background:var(--deliverable-hover)}`（= static-neutral-800）。
	card0, ok := rig.vr3DelivHit("deliv-0")
	if !ok {
		t.Fatalf("没有 deliv-0 的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	if hovered := rig.vr3DelivRects(rig.theme.CardFillHover); len(hovered) != 0 {
		t.Fatalf("指针还没进卡片，就已经有 %d 块 hover 底色了", len(hovered))
	}
	if err := rig.session.Move(card0.Bounds.X+card0.Bounds.Width-8, card0.Bounds.Y+card0.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hovered := rig.vr3DelivRects(rig.theme.CardFillHover)
	if len(hovered) != 1 {
		t.Fatalf("悬停后 hover 底色块 %d 块，want 1（`.file:hover`）", len(hovered))
	}
	if !vr3DelivClose(hovered[0].Bounds.X, card0.Bounds.X+1) || !vr3DelivClose(hovered[0].Bounds.Y, card0.Bounds.Y+1) {
		t.Errorf("hover 底色块 %+v 与被悬停的卡片 %+v（+1 的边框内缩）不是同一块",
			hovered[0].Bounds, card0.Bounds)
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-hover.png")
}

// TestVR3DeliverablesFocusRing 走**真键盘** Tab，核官方那条焦点内描边。
//
// 官方 `.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px
// var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));outline:none}`
// （048__Deliverables.module.css.mjs.css:1）——2px、**内**描边（inset）。
func TestVR3DeliverablesFocusRing(t *testing.T) {
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rig := vr3OpenDeliverables(t, 1180, 700, []apiclient.Message{
		vr3WriteMsg("src/a.go"), vr3WriteMsg("web/index.html"),
	}, nil)
	card0, ok := rig.vr3DelivHit("deliv-0")
	if !ok {
		t.Fatalf("没有 deliv-0 的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	// 指针先离开所有卡片：焦点态必须自己就能看见，不靠 hover。
	if err := rig.session.Move(1000, 690); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	before, err := rig.session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}

	// Tab 到第一张卡（本帧唯一的 Tab 停靠点）。
	id := rig.tree.ID(rig.feat, "deliv-0")
	focused := false
	for step := 0; step < 4; step++ {
		if err := rig.session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if err := rig.session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		if rig.session.Snapshot().Focused == id {
			focused = true
			break
		}
	}
	if !focused {
		t.Fatalf("Tab 走不到交付卡片（focus=%v card=%v）", rig.session.Snapshot().Focused, id)
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-card-focused.png")

	after, err := rig.session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	changed := vr3DelivPixelDiff(before, after, card0.Bounds)
	t.Logf("VR3 焦点落在卡片上时，卡片矩形内像素变化 = %d", changed)
	if changed == 0 {
		t.Errorf("键盘焦点落在交付卡片上，卡片区域一个像素都没变——缺官方那条 2px **内**描边。" +
			"官方：`.cardPreview:focus-visible{box-shadow:inset 0 0 0 2px " +
			"var(--dsw-focus-ring-color,var(--dsw-alias-state-business-primary));outline:none}`" +
			"（048__Deliverables.module.css.mjs.css:1）。" +
			"本仓 deliverables.go:209-214 的卡片只有 `Hover(...)`，没有 `Focus(...)`。" +
			"修法：给这张卡加 `Focus(ui.Look{}.Ring(2, t.Info))` 或（要严格 inset）" +
			"`DrawOver` + `canvas.RoundedOutline(rect.Inset(1), radius-1, 2, color)`" +
			"（同 dsh 里 plugins/dsplan/focus.go:16-25 的做法）")
	}
}

// TestVR3DeliverablesEmptyState 是**空态**：一条写类工具消息都没有时，
// 官方整块 `Deliverables` 不渲染（`presented.length === 0` 时组件返回 null）。
func TestVR3DeliverablesEmptyState(t *testing.T) {
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 只有读类工具与普通消息：都不该算交付物。
	rig := vr3OpenDeliverables(t, 900, 700, []apiclient.Message{
		{Role: "tool", Tool: "read", Args: `{"filePath":"src/a.go"}`},
		{Role: "user", Text: "把 a.go 改一下"},
		{Role: "tool", Tool: "bash", Args: `{"command":"rm -rf build"}`},
	}, nil)
	if texts := rig.session.Texts(); len(texts) != 0 {
		t.Errorf("没有交付物却画了文字：%q", texts)
	}
	if cards := rig.vr3DelivRects(rig.theme.CardFill); len(cards) != 0 {
		t.Errorf("没有交付物却画了 %d 张卡", len(cards))
	}
	// 区域里一个非背景像素都不许有（残留边框/阴影都算）。
	img, err := rig.session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	bounds := img.Bounds()
	bg := img.RGBAAt(bounds.Max.X-2, bounds.Max.Y-2)
	ink := 0
	first := image.Point{-1, -1}
	for y := 140; y < 520; y++ {
		for x := 0; x < 300; x++ {
			if vr3DelivSameColor(img.RGBAAt(x, y), bg, 6) {
				continue
			}
			ink++
			if first.X < 0 {
				first = image.Point{x, y}
			}
		}
	}
	if ink != 0 {
		t.Errorf("没有交付物时侧栏区域仍有 %d 个非背景像素（首处 %v）——空态应当整块不画", ink, first)
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-empty.png")
}

// vr3DelivSameColor 是像素比较（容忍 8 位量化误差）。
func vr3DelivSameColor(a, b interface {
	RGBA() (uint32, uint32, uint32, uint32)
}, tolerance int) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	d := vr3DelivAbs(int(ar>>8)-int(br>>8)) + vr3DelivAbs(int(ag>>8)-int(bg>>8)) +
		vr3DelivAbs(int(ab>>8)-int(bb>>8)) + vr3DelivAbs(int(aa>>8)-int(ba>>8))
	return d <= tolerance
}

func vr3DelivAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// vr3DelivPixelDiff 数两块图在矩形内不同的像素数。
func vr3DelivPixelDiff(before, after *image.RGBA, r gfx.Rect) int {
	if before.Bounds() != after.Bounds() {
		return -1
	}
	count := 0
	for y := int(r.Y); y < int(r.Y+r.Height); y++ {
		for x := int(r.X); x < int(r.X+r.Width); x++ {
			if x < 0 || y < 0 || x >= before.Bounds().Max.X || y >= before.Bounds().Max.Y {
				continue
			}
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				count++
			}
		}
	}
	return count
}

// TestVR3DeliverablesLongDiffScrolls 是**长列表滚动**：预览浮层的 diff 正文
// 必须滚在自己的视口里（官方 `.body{overflow:auto}`，OFF:FileDiff.module.css:5），
// 滚到底之后最后一行要真的进到视口内。
func TestVR3DeliverablesLongDiffScrolls(t *testing.T) {
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	rig := vr3OpenDeliverables(t, 1180, 700, []apiclient.Message{vr3WriteMsg("src/a.go")},
		[]apiclient.Frame{{Seq: 1, Type: "user/message"}, {Seq: 9, Type: ChangesFrameType}})

	lines := make([]string, 0, 160)
	for i := 0; i < 80; i++ {
		lines = append(lines, " ctx-line-"+vr3DelivItoa(i))
	}
	lines[1] = "-ctx-line-1"
	lines[2] = "+ctx-line-2"
	SetChangesSource(&changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Cwd: "/w", Files: []apiclient.ChangesFile{
			{Path: "src/a.go", Display: "src/a.go", Added: 1, Deleted: 1}}},
		diff: &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go",
			Before: true, After: true,
			Hunks: []apiclient.ChangesHunk{{OldStart: 1, OldLines: 80, NewStart: 1, NewLines: 80, Lines: lines}}},
	})
	t.Cleanup(func() { SetChangesSource(nil) })

	// 真点卡片打开浮层。
	card0, ok := rig.vr3DelivHit("deliv-0")
	if !ok {
		t.Fatalf("没有 deliv-0 的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	if err := rig.session.Click(card0.Bounds.X+card0.Bounds.Width/2, card0.Bounds.Y+card0.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if !rig.session.HasText("@@ -1,80 +1,80 @@") {
		t.Fatalf("浮层里没有 hunk 头：texts=%v", rig.session.Texts())
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-diff-top.png")

	// 视口：`overlayTop + 头部` 之下、`overlayBodyHeight` 之内。
	bodyTop := float32(overlayTop) + 60
	bodyHeight := overlayBodyHeight(&pluginapi.Env{Bounds: gfx.Rect{Width: 1180, Height: 700}})
	bodyBottom := bodyTop + bodyHeight
	last, ok := rig.vr3DelivText("ctx-line-79")
	if !ok {
		t.Fatalf("最后一行根本没排上版：texts 共 %d 条", len(rig.session.Texts()))
	}
	if last.Bounds.Y < bodyBottom {
		t.Fatalf("80 行 diff 没有溢出正文视口（最后一行 y=%v < 视口底 %v），滚动断言不成立",
			last.Bounds.Y, bodyBottom)
	}

	// 滚到底。
	if err := rig.session.Move(overlayLeft+overlayWidth/2, bodyTop+bodyHeight/2); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Scroll(gfx.ScrollEvent{
		X: overlayLeft + overlayWidth/2, Y: bodyTop + bodyHeight/2, DeltaY: 4000, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if err := rig.session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	after, ok := rig.vr3DelivText("ctx-line-79")
	if !ok {
		t.Fatalf("滚到底之后最后一行消失了：texts 共 %d 条", len(rig.session.Texts()))
	}
	if after.Bounds.Y < bodyTop || after.Bounds.Y+after.Bounds.Height > bodyBottom+1 {
		t.Errorf("滚到底之后最后一行仍在视口外：y=%v..%v，视口 %v..%v",
			after.Bounds.Y, after.Bounds.Y+after.Bounds.Height, bodyTop, bodyBottom)
	}
	// 浮层头部不跟着滚（它不在滚动区里）。
	if !rig.session.HasText("src/a.go") {
		t.Errorf("滚动之后浮层头部（路径）不见了：texts=%v", rig.session.Texts())
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-diff-bottom.png")
}

// vr3DelivItoa 是整数转串（本包产品代码里的 itoa 不导出，测试自己带一份）。
func vr3DelivItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestVR3DeliverablesLongNameEllipsis 是**窄宽 900×700**：超长文件名必须收在卡内。
//
// 官方 `.fileName{text-overflow:ellipsis;white-space:nowrap;overflow:hidden}`，
// 卡本身 `.file{overflow:hidden}`。
func TestVR3DeliverablesLongNameEllipsis(t *testing.T) {
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	longBase := "this-is-a-very-long-file-name-that-must-be-ellipsized-inside-the-card"
	rig := vr3OpenDeliverables(t, 900, 700, []apiclient.Message{
		vr3WriteMsg("pkg/" + longBase + ".go"),
	}, nil)
	card, ok := rig.vr3DelivHit("deliv-0")
	if !ok {
		t.Fatalf("没有 deliv-0 的命中区：hits=%+v", rig.session.Snapshot().Hits)
	}
	var text gfx.Text
	found := false
	for _, candidate := range rig.session.Snapshot().Texts {
		if strings.HasPrefix(candidate.Value, "this-is-a-very-long") {
			text, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatalf("超长文件名没排上版：texts=%v", rig.session.Texts())
	}
	// 官方 `.fileName{text-overflow:ellipsis}`：被截断时必须以省略号收尾。
	if !strings.HasSuffix(text.Value, "…") {
		t.Errorf("超长文件名被截成 %q，结尾不是省略号——官方 `.fileName{text-overflow:ellipsis}`",
			text.Value)
	}
	// 文字盒右缘不许越过卡片内容框（卡宽 − 左内边距 10 − 图标 40 − gap 10 − 右内边距 10）。
	limit := card.Bounds.X + card.Bounds.Width - cardPadX
	if right := text.Bounds.X + text.Bounds.Width; right > limit+1 {
		t.Errorf("900 宽窗口里超长文件名的文字盒右缘 %v 越过了卡片内容右缘 %v——"+
			"官方 `.fileName{text-overflow:ellipsis;white-space:nowrap;overflow:hidden}` + `.file{overflow:hidden}`",
			right, limit)
	}
	if text.Bounds.X < card.Bounds.X+cardPadX {
		t.Errorf("文件名左缘 %v 落在卡片左内边距 %v 之外", text.Bounds.X, card.Bounds.X+cardPadX)
	}
	vr3DelivShot(t, rig.session, dir, "vr3-deliverables-900x700-long-name.png")
}
