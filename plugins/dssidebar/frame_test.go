package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// fg2Text 返回这一帧实际绘制的文字；找不到就立即失败，避免用空零值误判字号。
func fg2Text(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("画面缺少 %q；实际文字=%v", value, canvas.Texts())
	return gfx.Text{}
}

func fg2HasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func TestFG2OfficialGeometryConstants(t *testing.T) {
	cases := map[string]struct{ got, want float32 }{
		"logoRow":          {logoRowHeight, 40},
		"brandIdentity":    {brandIdentityHeight, 24},
		"brandMark":        {brandMarkSize, 24},
		"brandName":        {brandNameHeight, 24},
		"brandNameFont":    {brandNameFontSize, 18},
		"brandNameLine":    {brandNameLineHeight, 24},
		"brandGap":         {brandGap, 8},
		"newSession":       {newSessionHeight, 38},
		"newSessionTop":    {newSessionMarginTop, 8},
		"newSessionBottom": {newSessionMarginBottom, 12},
		"newSessionFont":   {newSessionFontSize, 14},
		"newSessionLine":   {newSessionLineHeight, 22},
		"panelRow":         {panelRowHeight, 36},
		"panelListGap":     {panelListGap, 2},
		"panelListBottom":  {panelListBottom, 8},
		"workspaceHeader":  {workspaceHeaderHeight, 36},
		"searchExpanded":   {searchHeight, 30},
	}
	for name, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s=%v，官方源=%v", name, tc.got, tc.want)
		}
	}
}

func TestFG2BrandNewSessionAndNavigationGeometry(t *testing.T) {
	p, _, env, _, canvas, _ := searchFixture(t)
	env.Act.NewSession = func() {}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
		p.drawHeader(f, env)
	})

	boxes := p.frameBoxes
	if got := boxes.Brand.Rect().Height; got != 40 {
		t.Errorf("logoRow 实际高=%v，want 40", got)
	}
	if got := boxes.BrandIdentity.Rect().Height; got != 24 {
		t.Errorf("brandIdentity 实际高=%v，want 24", got)
	}
	if got := boxes.BrandMark.Rect().Width; got != 24 {
		t.Errorf("鱼标宽=%v，want 24", got)
	}
	if got := boxes.BrandName.Rect().Height; got != 24 {
		t.Errorf("brandName 高=%v，want 24", got)
	}
	if got := boxes.BrandName.Rect().X - (boxes.BrandMark.Rect().X + boxes.BrandMark.Rect().Width); got != 8 {
		t.Errorf("鱼标与 wordmark 间距=%v，want 8", got)
	}
	deepseek := fg2Text(t, canvas, "deepseek")
	if deepseek.Style.Size != 18 || deepseek.Bounds.Height != 24 {
		t.Errorf("deepseek 字体/行高=%v/%v，want 18/24", deepseek.Style.Size, deepseek.Bounds.Height)
	}
	fg2Text(t, canvas, "HARNESS")
	if len(canvas.Triangles()) == 0 {
		t.Error("品牌行没有画出官方鱼标矢量")
	}

	newRect := boxes.NewSession.Rect()
	if newRect.Height != 38 {
		t.Errorf("新会话高=%v，want 38", newRect.Height)
	}
	if got := newRect.Y - (boxes.Brand.Rect().Y + boxes.Brand.Rect().Height); got != 8 {
		t.Errorf("Windows 新会话 margin-top=%v，want 8", got)
	}
	label := fg2Text(t, canvas, "新会话")
	if label.Style.Size != 14 || label.Bounds.Height != 22 || label.Style.Weight != gfx.WeightMedium {
		t.Errorf("新会话字体/行高/字重=%v/%v/%v，want 14/22/500", label.Style.Size, label.Bounds.Height, label.Style.Weight)
	}
	if len(boxes.PanelRows) != 2 {
		t.Fatalf("内置真实导航行=%d，want plugins/schedules 两条", len(boxes.PanelRows))
	}
	for i, row := range boxes.PanelRows {
		if row.Rect().Height != 36 {
			t.Errorf("导航行 %d 高=%v，want 36", i, row.Rect().Height)
		}
		if i > 0 {
			prev := boxes.PanelRows[i-1].Rect()
			if got := row.Rect().Y - (prev.Y + prev.Height); got != 2 {
				t.Errorf("panelList gap=%v，want 2", got)
			}
		}
	}
	firstPanel := boxes.PanelRows[0].Rect()
	if got := firstPanel.Y - (newRect.Y + newRect.Height); got != 12 {
		t.Errorf("新会话 margin-bottom=%v，want 12", got)
	}
	fg2Text(t, canvas, "插件")
	fg2Text(t, canvas, "自动化任务")
}

func TestFG2NewSessionClickCallsRealActionOnce(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	calls := 0
	env.Act.NewSession = func() { calls++ }
	p.activePanel = "schedules"
	pageTheme2 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme2})
	draw := func() {
		view.Draw(canvas, gfx.Rect{Width: 256, Height: 420}, func(f ui.C) {
			p.drawHeader(f, env)
		})
	}
	draw()
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "new-session"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if calls != 1 {
		t.Fatalf("一次点击 NewSession 调用=%d，want 1", calls)
	}
	if p.activePanel != "" {
		t.Fatalf("新会话后仍停在面板 %q，want 返回会话", p.activePanel)
	}
}

func TestFG2NavigationUsesRealRoutesAndExtensionVisibility(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	env.Bounds = gfx.Rect{Width: 1182, Height: 832}
	env.Geometry = pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902}
	env.Data.Cur = "session-sentinel"
	env.Data.Sessions = mockdSessions()
	p.search.SetText("保留搜索现场")
	draftCalls := 0
	env.Act.SetDraft = func(string) { draftCalls++ }
	opened := ""
	env.Act.OpenSession = func(id string) { opened = id }

	routes := p.panelRoutes()
	if len(routes) != 2 || routes[0].ID != "plugins" || routes[1].ID != "schedules" {
		t.Fatalf("默认路由=%+v，want plugins(order0)/schedules(order10)", routes)
	}
	for _, route := range routes {
		if route.Draw == nil || route.Label == "" {
			t.Fatalf("可见入口缺真实面板或文案：%+v", route)
		}
	}

	pageTheme3 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme3})
	draw := func() {
		view.Draw(canvas, env.Bounds, func(f ui.C) {
			ui.Column(f).FillWidth().FillHeight().Children(func() {
				p.drawHeader(f, env)
				p.drawActivePanel(f, env)
			})
		})
	}
	draw()
	for _, absent := range []string{"主题鱼", "上下文压缩", "更多"} {
		if fg2HasText(canvas, absent) {
			t.Fatalf("当前没有对应扩展/动作，%q 不应出现", absent)
		}
	}

	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "panel-schedules"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if p.activePanel != "schedules" {
		t.Fatalf("点击自动化任务后 activePanel=%q", p.activePanel)
	}
	if !fg2HasText(canvas, "当前宿主未提供任务目录") {
		t.Fatal("schedules 路由没有实画真实能力状态")
	}
	if got := p.frameBoxes.ActivePanel.Rect(); got.X != 280 || got.Width != 902 {
		t.Fatalf("主区面板矩形=%+v，want x=280 width=902", got)
	}
	if env.Data.Cur != "session-sentinel" || p.search.Text() != "保留搜索现场" || draftCalls != 0 {
		t.Fatal("打开面板改动了会话 id、搜索现场或草稿")
	}

	// 会话行仍是返回会话的真实路由；不能重载、清草稿或新建会话。
	row := sess("session-sentinel", "真会话", `D:\work`, "2026-11-03T10:00:00Z")
	drawRow := func() {
		view.Draw(canvas, gfx.Rect{Width: 268, Height: 40}, func(f ui.C) {
			p.drawSession(f, env, row)
		})
	}
	drawRow()
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "s-session-sentinel"), gfx.ClickInfo{})
	drawRow()
	canvas.EndEventPass()
	if opened != "session-sentinel" || p.activePanel != "" {
		t.Fatalf("返回会话：opened=%q activePanel=%q", opened, p.activePanel)
	}
	if env.Data.Cur != "session-sentinel" || p.search.Text() != "保留搜索现场" || draftCalls != 0 {
		t.Fatal("返回会话破坏了原有现场")
	}

	// 扩展只有同时提供真实 Draw 才能进入导航；名字本身不会点亮入口。
	p.extensionRoutes = []panelRoute{
		{ID: "theme-fish", Label: "主题鱼", Order: 20},
		{ID: "compact", Label: "上下文压缩", Order: 30},
	}
	if got := p.panelRoutes(); len(got) != 2 {
		t.Fatalf("没有 Draw 的扩展竟出现在路由中：%+v", got)
	}
	p.extensionRoutes[0].Draw = func(f ui.C, env *pluginapi.Env) {
		ui.Text(f, "主题鱼真实扩展")
	}
	if got := p.panelRoutes(); len(got) != 3 || got[2].ID != "theme-fish" {
		t.Fatalf("真实扩展没有按 order 接入：%+v", got)
	}
}

func TestFG2WorkspaceHeaderCollapseExpandAndSearch(t *testing.T) {
	p, _, env, tree, canvas, _ := searchFixture(t)
	p.known = []string{"workspace"}
	pageTheme4 := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme4})
	draw := func() {
		view.Draw(canvas, gfx.Rect{Width: 268, Height: 50}, func(f ui.C) {
			p.drawWorkspaceHeader(f, env)
		})
	}
	draw()
	if got := p.frameBoxes.WorkspaceHeader.Rect().Height; got != 36 {
		t.Fatalf("工作区 header 高=%v，want 36", got)
	}
	if got := p.frameBoxes.SearchButton.Rect(); got.Width != 28 || got.Height != 28 {
		t.Fatalf("折叠搜索按钮=%+v，want 28x28", got)
	}
	fg2Text(t, canvas, "工作区")

	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "workspace-search"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if !p.searchExpanded {
		t.Fatal("点击搜索按钮没有展开真实 Field")
	}
	canvas.Reset()
	draw() // 点击在当帧布局后生效；下一帧才是展开后的真实矩形。
	if got := p.frameBoxes.Search.Rect().Height; got != 30 {
		t.Fatalf("展开搜索高=%v，want 30", got)
	}
	if fg2HasText(canvas, "工作区") {
		t.Fatal("搜索展开时 section label 应隐藏")
	}
	p.search.SetText("Alpha")
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) || p.searchExpanded || p.search.Text() != "" {
		t.Fatalf("Esc 应清空并收起搜索：expanded=%v query=%q", p.searchExpanded, p.search.Text())
	}

	// 右侧视图按钮接既有的真实“全部收起”行为，不是无动作装饰。
	draw()
	canvas.Reset()
	canvas.BeginEventPass(tree.ID(env.Feat, "workspace-view-options"), gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
	if !p.collapsed["workspace"] {
		t.Fatal("工作区视图按钮没有触发真实分组收起")
	}
}

func TestFG2SidebarFishUsesOfficialPath(t *testing.T) {
	if len(fishLogoPath) != 3448 || fishViewBoxW != 23.16 || fishViewBoxH != 17.04 {
		t.Fatalf("鱼标字节/viewBox 偏离官方：bytes=%d viewBox=%vx%v", len(fishLogoPath), fishViewBoxW, fishViewBoxH)
	}
	icon := fishIcon()
	if len(icon.Commands) != 80 || icon.ViewBox != 23.16 || icon.ViewHeight != 17.04 {
		t.Fatalf("鱼标解析不完整：commands=%d viewBox=%vx%v", len(icon.Commands), icon.ViewBox, icon.ViewHeight)
	}
}
