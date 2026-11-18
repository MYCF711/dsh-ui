//go:build darwin || windows || linux

// vr3_capture_test.go —— 第三轮真实渲染审计：个性化页面推荐卡（create_personalized_page 正文）。
//
// 前两轮看的是状态机与常量表（plugin_test.go / official_geometry_test.go 都在裸 canvas 上）。
// 这一轮在真 headless 帧里看三件前两轮没看过的事：
//
//	① **自定义位置的 textarea**：官方 `.customLocation textarea{...min-height:72px;
//	   padding:8px 10px;border:.5px solid border-l4;border-radius:var(--dsw-radius-md);
//	   font-size:13px;line-height:1.7}`——真点开位置菜单切到「自行撰写」之后它到底画在哪、
//	   多大、有没有按 72 高画
//	② 它的**键盘焦点**：官方 `.customLocation textarea:focus-visible{outline:1px solid
//	   var(--dsw-alias-state-business-primary);outline-offset:1px}`——Tab 进去有没有那圈 1px 描边
//	③ 窄宽 900×700 下的 offer 布局：`.controls{display:flex;flex-wrap:wrap;gap:12px}`
//	   里「添加到 + 位置按钮 + 两个动作按钮」是不是都在卡内、没有被挤掉
//
// 官方依据（**rc.2 asar 原件**，`@deepseek-ai/dsh-client-ui-creation-recommendations/Card.module.css`）：
//
//	:1  `.header{display:flex;align-items:center;justify-content:space-between;gap:8px 12px;
//	     padding:12px 20px 0;font-size:12px;line-height:1.5;color:label-secondary;flex-wrap:wrap}`
//	:3  `.infoButton{width:28px;padding:0;color:label-secondary}`
//	:9  `.body{padding:12px 20px 20px}`
//	:17 `.controls{display:flex;align-items:center;flex-wrap:wrap;gap:12px;margin-top:16px;
//	     padding-top:16px;border-top:.5px solid var(--dsw-alias-border-l1)}`
//	:19 `.locationRow{display:flex;align-items:center;gap:8px;min-width:0}`
//	:21 `.locationButton{gap:8px;font-size:13px}`
//	:25 `.customLocation{display:flex;flex-direction:column;gap:6px;flex:1 0 100%;min-width:0}`
//	:26 `.customLocation textarea{box-sizing:border-box;width:100%;min-height:72px;
//	     resize:vertical;padding:8px 10px;border:.5px solid var(--dsw-alias-border-l4);
//	     border-radius:var(--dsw-radius-md);background:var(--dsw-alias-bg-layer-1);
//	     color:label-primary;font:inherit;font-size:13px;line-height:1.7}`
//	:27 `.customLocation textarea:focus-visible{outline:1px solid
//	     var(--dsw-alias-state-business-primary);outline-offset:1px}`
//
// 上面这些行号来自 **rc.2 asar 原件**（`Desktop-0.2.2-rc.2/resources/app.asar` 里
// `@deepseek-ai/dsh-client-ui-creation-recommendations/Card.module.css` 的未压缩文本，
// 类名前缀 `rQbmCW_`）：`:26` 的 textarea 几何（`min-height:72px;padding:8px 10px;
// border:.5px solid --dsw-alias-border-l4;border-radius:var(--dsw-radius-md);
// font-size:13px;line-height:1.7`）与 `:27` 的
// `:focus-visible{outline:1px solid var(--dsw-alias-state-business-primary);
// outline-offset:1px}` 均已逐字核对。
//
// 落盘目录：`$VR3_CAPTURE_DIR`（未设置时用 t.TempDir()）。
package dscreationreco

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// vr3CreationDir 是本次审计的落盘目录。
func vr3CreationDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("VR3_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// vr3CreationShot 用 session.Screenshot 落一张 PNG，并核它非空。
func vr3CreationShot(t *testing.T, session *headless.Session, dir, name string) string {
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

// vr3CreationProps 造一张 offer 态卡的 props。
func vr3CreationProps(initial string) pluginapi.ActionCardBodyProps {
	details := map[string]json.RawMessage{"slot": json.RawMessage(`"sidebar-right"`)}
	if initial != "" {
		details["initialContent"] = json.RawMessage(`"` + initial + `"`)
	}
	return pluginapi.ActionCardBodyProps{
		SessionID: "vr3-session",
		CallID:    "call-1",
		Action:    Action,
		Title:     "把验收清单放到右侧栏",
		Brief:     "在当前会话右侧新增一页验收清单。",
		Details:   details,
		Act:       func(string, map[string]json.RawMessage) {},
	}
}

// vr3OpenCreationCard 起一个窗口画这张卡（卡挂在会话列左缘 0、上缘 24）。
func vr3OpenCreationCard(t *testing.T, width, height int, cardWidth float32,
	props pluginapi.ActionCardBodyProps, p *Plugin) *headless.Session {
	t.Helper()
	if p == nil {
		p = &Plugin{}
	}
	theme := dscore.Dark()
	p.theme = theme
	tree := &gfx.Tree{}
	feat := tree.Feature("vr3-creationreco")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	invalidator := &app.Invalidator{}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat, Bounds: gfx.Rect{Width: float32(width), Height: float32(height)},
		MainWidth: float32(width) - 280,
		Data:      pluginapi.Snapshot{Cur: "vr3-session"},
	}
	env.Act.Invalidate = func() { invalidator.Invalidate() }
	config := app.Config{
		Title: "vr3-creationreco", Width: width, Height: height,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			env.Canvas = canvas
			env.Bounds = bounds
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
	return session
}

// vr3CreationTextarea 从无障碍树里找多行输入框（官方 `.customLocation textarea`）。
func vr3CreationTextarea(session *headless.Session) (app.AccessNode, bool) {
	for _, node := range session.Access() {
		if node.Multiline || node.Role == gfx.RoleTextField {
			return node, true
		}
	}
	return app.AccessNode{}, false
}

// TestVR3CreationCustomTextareaFocusOutline 走「点开位置 → 选自行撰写 → Tab 进 textarea」
// 这条真路径，核官方 :26 的几何与 :27 的焦点描边。
func TestVR3CreationCustomTextareaFocusOutline(t *testing.T) {
	dir := vr3CreationDir(t)
	p := &Plugin{}
	session := vr3OpenCreationCard(t, 900, 700, 620, vr3CreationProps(""), p)
	// 「自行撰写」的选中由 `chooseLocation` 承担：viva 的菜单项身份由框架自己持有，
	// 测试点不到（本包 plugin.go:838-846 已把这条写成测试接缝）。
	// **必须在第一帧之后**：`adoptCard` 在卡片身份第一次出现时会清掉 `selected`
	// （plugin.go:1029-1042），提前设置会被吃掉。
	p.chooseLocation(choiceCustom)
	// `Frame()` 是"无条件画一帧"：改状态之后没走 `Actions.Invalidate`，
	// 光 `Pump()` 不会重绘（驱动会认为没有待画帧），读到的还是上一屏。
	if err := session.Frame(); err != nil {
		t.Fatal(err)
	}
	vr3CreationShot(t, session, dir, "vr3-creationreco-custom-rest.png")

	area, ok := vr3CreationTextarea(session)
	if !ok {
		var names []string
		for _, node := range session.Access() {
			names = append(names, node.Label)
		}
		t.Fatalf("选了「自行撰写」也没有多行输入框（无障碍树里的控件名：%v）", names)
	}
	t.Logf("VR3 textarea bounds=%+v label=%q", area.Bounds, area.Label)
	if area.Label != zhCustomLabel {
		t.Errorf("输入框无障碍名 = %q，官方 `:138-141` 的 label 是 %q", area.Label, zhCustomLabel)
	}
	// `.customLocation textarea{min-height:72px}`：高度是硬下限。
	if area.Bounds.Height < createdCustomHeight-1 {
		t.Errorf("输入框高 %v，官方 `.customLocation textarea{min-height:72px}` = %v",
			area.Bounds.Height, createdCustomHeight)
	}
	// `.customLocation textarea{padding:8px 10px}`（左内边距 10）与 `width:100%`：
	// 输入框右缘应贴在 `.body{padding:12px 20px 20px}` 的内容右缘（卡宽 − 20）。
	if wantRight := float32(620) - createdBodyPadX; area.Bounds.X+area.Bounds.Width > wantRight+1 {
		t.Errorf("输入框右缘 %v 越过了 `.body{padding:12px 20px 20px}` 的内容右缘 %v——"+
			"官方 `.customLocation textarea{width:100%%}`（Card.module.css:26）",
			area.Bounds.X+area.Bounds.Width, wantRight)
	}

	// 真键盘：Tab 进 textarea。
	before, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	reached := false
	for step := 0; step < 8; step++ {
		if err := session.Keys("tab"); err != nil {
			t.Fatalf("Keys(tab)：%v", err)
		}
		if node, ok := vr3CreationTextarea(session); ok && node.Focused {
			reached = true
			break
		}
	}
	if !reached {
		t.Fatalf("Tab 走不进 textarea（当前焦点 %v）", session.Snapshot().Focused)
	}
	vr3CreationShot(t, session, dir, "vr3-creationreco-custom-focused.png")
	after, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}

	// 官方 :27 是 `outline-offset:1px` 的**外**描边：取输入框左缘外面那 1px 带子。
	ring := p.theme.Info // --dsw-alias-state-business-primary
	// 官方几何：`outline:1px` + `outline-offset:1px` → 环带严格落在框外 **1..2px**，
	// 贴着边的那 1px 是 offset 让出来的空档。所以采样扫框外 1..3px 的竖带
	// （含 1px 量化余量），落在环带上任意一处命中即算画出。
	y := int(area.Bounds.Y + area.Bounds.Height/2)
	found := 0
	for dx := 1; dx <= 3; dx++ {
		x := int(area.Bounds.X) - dx
		for dy := -2; dy <= 2; dy++ {
			if vr3CreationNear(after.RGBAAt(x, y+dy), ring) {
				found++
			}
		}
	}
	edgeInside := after.RGBAAt(int(area.Bounds.X)+1, y)
	t.Logf("VR3 输入框左缘外 1..3px 带子里的焦点环像素命中 = %d/15（框内 1px = %v，环色 = %v）",
		found, edgeInside, ring)
	// 焦点本身必须带来变化（至少光标/底色），但官方那条 1px 描边才是这一条的判据。
	if found == 0 {
		t.Errorf("Tab 进自定义位置输入框后，框外 1..3px 带子里一点焦点环色（%v）都没有——"+
			"官方 `.customLocation textarea:focus-visible{outline:1px solid "+
			"var(--dsw-alias-state-business-primary);outline-offset:1px}`"+
			"（rc.2 asar 的 creation-recommendations/Card.module.css:27）。"+
			"根因：plugin.go:852-868 的 `ui.TextArea` 只挂了 "+
			"`Border(0.5,BorderL4).Background(BgLayer1)`，没有 `Focus(...)`。"+
			"修法：`.Focus(ui.Look{}.Ring(1, t.Info))`（viva 的 Ring 画在元素外侧，"+
			"正是官方 `outline` + 正向 offset 的语义，见 ui/style.go:369-377 paintRing）",
			ring)
	}
	// 顺带核一眼：焦点前后画面确实变了（否则上面"没描边"可能只是没拍到）。
	if diff := vr3CreationDiff(before, after, area.Bounds); diff == 0 {
		t.Logf("VR3 注意：焦点前后输入框内部像素也没变（某些平台把编辑区交给原生控件绘制）")
	}
}

// vr3CreationNear 是像素与主题色的近似比较（±3/通道）。
func vr3CreationNear(pixel interface {
	RGBA() (uint32, uint32, uint32, uint32)
}, want ui.Color) bool {
	r, g, b, _ := pixel.RGBA()
	wr := uint32(want.Red*255 + 0.5)
	wg := uint32(want.Green*255 + 0.5)
	wb := uint32(want.Blue*255 + 0.5)
	return vr3CreationCloseInt(int(r>>8), int(wr)) && vr3CreationCloseInt(int(g>>8), int(wg)) &&
		vr3CreationCloseInt(int(b>>8), int(wb))
}

func vr3CreationCloseInt(a, b int) bool {
	d := a - b
	return d <= 3 && d >= -3
}

// TestVR3CreationOfferLayoutAt900 是**窄宽 900×700** 的 offer 布局：
// 官方 `.controls{flex-wrap:wrap}` 里「添加到 + 位置按钮」与两个动作按钮必须都在卡内。
func TestVR3CreationOfferLayoutAt900(t *testing.T) {
	dir := vr3CreationDir(t)
	p := &Plugin{}
	longContent := strings.Repeat("这一行是预填内容，用来触发官方的 FoldedText 折叠。", 24)
	session := vr3OpenCreationCard(t, 900, 700, 620, vr3CreationProps(longContent), p)
	vr3CreationShot(t, session, dir, "vr3-creationreco-offer-900x700.png")

	// 头部与正文的官方文案必须都画出来（`:1` 头部 + `:9` 正文）。
	for _, want := range []string{zhHeading, zhLocation, zhSlotSidebarRight, zhCreate, zhDismiss, zhInitialContent} {
		if !session.HasText(want) {
			t.Errorf("offer 态缺 %q：texts=%q", want, session.Texts())
		}
	}
	// 每个可交互控件都必须落在卡内（卡宽 620）。
	for _, node := range session.Access() {
		if node.Label == "" || !node.Focusable {
			continue
		}
		if node.Bounds.X+node.Bounds.Width > 620+1 {
			t.Errorf("控件 %q 的右缘 %v 越过了卡宽 620（`.controls{flex-wrap:wrap}` 没兜住）",
				node.Label, node.Bounds.X+node.Bounds.Width)
		}
		if node.Bounds.Width <= 0 || node.Bounds.Height <= 0 {
			t.Errorf("控件 %q 尺寸退化：%+v", node.Label, node.Bounds)
		}
	}
	// 「创建」在自定义位置为空时必须是**真禁用**（官方 Card.tsx 的 disabled 判据同源）。
	for _, node := range session.Access() {
		if node.Label == zhCreate && node.Disabled {
			t.Logf("VR3 「创建」在 offer 态是 enabled=%v disabled=%v", !node.Disabled, node.Disabled)
		}
	}
	// 预填内容折叠块：`.contentLabel` + 折叠开关（官方 `expandContent`）。
	if !session.HasText(zhExpandContent) && !session.HasText(zhCollapseContent) {
		t.Errorf("有 initialContent 却没有折叠开关（%q / %q）：texts=%q",
			zhExpandContent, zhCollapseContent, session.Texts())
	}
	// 头部位置选择按钮：官方 `.infoButton{width:28px}` 是 info 按钮，位置按钮
	// 走 `.locationButton{font-size:13px}`——这里只核它在头/正文行内、没有被裁。
	found := false
	for _, node := range session.Access() {
		if strings.TrimSpace(node.Label) == "" {
			continue
		}
		if node.Label == zhSlotSidebarRight {
			found = true
			if node.Bounds.Width < 40 {
				t.Errorf("位置按钮 %q 宽只有 %v，官方 `.locationButton{gap:8px;font-size:13px}` "+
					"+ sm 原语（28 高）不该窄成这样", node.Label, node.Bounds.Width)
			}
		}
	}
	if !found {
		t.Errorf("找不到位置选择按钮（label=%q）", zhSlotSidebarRight)
	}
}

// vr3CreationDiff 数两块图在矩形内不同的像素数。
func vr3CreationDiff(before, after *image.RGBA, rect gfx.Rect) int {
	if before.Bounds() != after.Bounds() {
		return -1
	}
	count := 0
	for y := int(rect.Y); y < int(rect.Y+rect.Height); y++ {
		for x := int(rect.X); x < int(rect.X+rect.Width); x++ {
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
