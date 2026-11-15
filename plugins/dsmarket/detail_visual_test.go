//go:build darwin || windows || linux

// detail_visual_test.go —— 详情页的**真实像素帧**验收。
//
// 为什么必须有这一条：单测能证明"版本行算对了、状态映射对了"，证明不了它**画在
// 屏幕上**。这个仓里"算了但没画"已经踩过好几次（SlotMainHeader、open-in-app 的
// 按钮），所以每块新界面都要有一帧真实像素。
//
// 判据取自官方 `EntryDetail`（`OFF:…/DiscoverSection.tsx:265-355`）与
// `VersionRow`（:166-215）：标题 + 版本 Tag、说明、兼容性提示条、详情文档、
// 版本列表（推荐 / 预发布 / 当前已安装 / 不兼容）、来源 facts。
//
// 两条**否定**判据同样重要（画了不该画的东西也是错）：
//   - 没有返回目标时不画面包屑（`发现` 不该出现）；
//   - 安装 / 更新按钮一个都不画（宿主 501，画了就是死按钮）。
package dsmarket

import (
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

// captureEntry 是一条字段齐全的市场条目（形状照官方 MarketListingEntry 摆）。
//
// 默认版本故意选**不兼容**的那一条：这样详情页同时走到提示条与"不兼容"标签，
// 而"当前已安装"落在另一条上——三条判据一帧全测。
func captureEntry() Entry {
	return Entry{
		Name:        "demo-plugin",
		Title:       "演示插件",
		Description: "一句话说明",
		Author:      "Alice",
		Homepage:    "https://example.com/demo",
		Repository:  &Repository{URL: "https://github.com/x/y", Directory: "packages/demo"},
		Versions: []Version{
			{
				Version:          "1.9.0",
				PublishTimestamp: 1767268800,
				Source:           Source{Kind: SourceGit, URL: "https://github.com/x/y", Commit: "abc123"},
				Compatibility: Compatibility{Compatible: false, RuntimeVersion: "0.2.2",
					Requirements: map[string]string{"dsh": "^0.2.0"}},
			},
			{
				Version:          "2.0.0",
				PublishTimestamp: 1767355200,
				Source:           Source{Kind: SourceNPM, Registry: "https://registry.example"},
				Compatibility:    Compatibility{Compatible: true},
			},
			{
				Version:          "2.0.0-beta.1",
				PublishTimestamp: 1767441600,
				Source:           Source{Kind: SourceNPM},
				Compatibility:    Compatibility{Compatible: true},
			},
		},
		DefaultVersion: "1.9.0",
		Installation:   Installation{Status: InstallStatusInstalled, InstalledVersion: "2.0.0"},
	}
}

// capturePage 画一帧详情页并把这一帧的**已绘制文字**取回来。
//
// pump 到条件成立再断言：viva 的字体测量在第一帧之后才稳定，一帧就断言会得到
// 时红时绿的结果（这个仓里踩过）。
func capturePage(t *testing.T, back func()) app.Snapshot {
	t.Helper()
	theme := dscore.Dark()
	page := NewPage(theme)
	page.SetLanguage("zh-CN")
	page.Open(captureEntry(), "https://m.example/index.json")
	page.SetBack(back)

	tree := &gfx.Tree{}
	feat := tree.Feature("market-detail-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "market-detail", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	env := &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: feat,
		Bounds:    gfx.Rect{Width: 1182, Height: 832},
		MainWidth: 902,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: 902},
		Act: pluginapi.Actions{
			OpenExternal: func(string) error { return nil },
			MarketEntryDetail: func(url, name, version, language string) (apiclient.PluginMarketEntryDetail, bool, apiclient.PluginMarketDetailFailure) {
				return apiclient.PluginMarketEntryDetail{
					Status:   apiclient.PluginDetailFound,
					Origin:   apiclient.PluginDetailOriginReadme,
					Markdown: "## 用法\n\n先跑 `demo --help`。",
					Document: "demo-plugin@1.9.0",
				}, true, ""
			},
		},
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env.Canvas = canvas
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(902).FillHeight().Children(func() {
				page.Draw(f, env)
			})
		})
		page.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()

	// pump 到"标题 + 版本号 + markdown 正文"三样都在，最多 20 帧。
	for i := 0; i < 20; i++ {
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
		text := drawnText(session.Snapshot())
		if strings.Contains(text, "演示插件") && strings.Contains(text, "1.9.0") && strings.Contains(text, "用法") {
			return session.Snapshot()
		}
	}
	t.Fatalf("20 帧内没画全：\n%s", drawnText(session.Snapshot()))
	return app.Snapshot{}
}

// drawnText 把这一帧真正画出来的文字拼成一段（换行分隔），便于 Contains 断言。
func drawnText(snapshot app.Snapshot) string {
	values := make([]string, 0, len(snapshot.Labels))
	for _, label := range snapshot.Labels {
		values = append(values, label.Value)
	}
	return strings.Join(values, "\n")
}

func TestMarketDetailDrawsOfficialStructure(t *testing.T) {
	snapshot := capturePage(t, nil)
	text := drawnText(snapshot)

	// 头：标题 + 展示出来的那一个版本（默认版本 1.9.0，不是最新版 2.0.0）。
	for _, want := range []string{"演示插件", "demo-plugin", "一句话说明", "1.9.0"} {
		if !strings.Contains(text, want) {
			t.Fatalf("详情页头缺 %q：\n%s", want, text)
		}
	}
	// 兼容性提示条（官方 `.notice`）：默认版本 1.9.0 与当前 DSH 不兼容。
	if !strings.Contains(text, "此版本与当前的 DSH 0.2.2 不兼容") {
		t.Fatalf("不兼容提示条没画：\n%s", text)
	}
	// 详情文档：markdown 由 MarkdownView 真实排版并覆画（不是纯文本占位）。
	if !strings.Contains(text, "用法") || !strings.Contains(text, "demo --help") {
		t.Fatalf("详情文档正文没画出来：\n%s", text)
	}
	// 版本列表：三条版本 + 四类标签。
	for _, want := range []string{"2.0.0", "2.0.0-beta.1", "推荐", "当前已安装", "预发布", "不兼容"} {
		if !strings.Contains(text, want) {
			t.Fatalf("版本列表缺 %q：\n%s", want, text)
		}
	}
	// facts：来源分区、作者、git 来源三件事、发布时间。
	for _, want := range []string{"来源", "作者", "Alice", "提交", "abc123", "仓库", "https://github.com/x/y", "发布时间"} {
		if !strings.Contains(text, want) {
			t.Fatalf("来源 facts 缺 %q：\n%s", want, text)
		}
	}
	// 没有返回目标 → 不画面包屑（画一个点了没反应的按钮比不画更糟）。
	if strings.Contains(text, "发现") {
		t.Fatalf("没有返回目标时不该画面包屑：\n%s", text)
	}
	// 安装 / 更新按钮一个都不许画（宿主 installBundle/updateBundle 是 501）。
	//
	// 判据是**整条文字相等**，不是 Contains：`当前已安装` 这枚标签本身就带"安装"
	// 两个字，用子串判会把自己误伤成失败（第一版就踩了）。
	for _, label := range snapshot.Labels {
		if label.Value == "安装" || label.Value == "更新" {
			t.Fatalf("%q 按钮不该出现（宿主 501）：\n%s", label.Value, text)
		}
	}
}

// TestMarketDetailDrawsCrumbWhenBackExists 钉住面包屑的另一半：有返回目标时才画。
func TestMarketDetailDrawsCrumbWhenBackExists(t *testing.T) {
	clicked := 0
	text := drawnText(capturePage(t, func() { clicked++ }))
	if !strings.Contains(text, "发现") {
		t.Fatalf("有返回目标时应当画面包屑：\n%s", text)
	}
	_ = clicked
}

// TestMarketDetailDoesNotDrawWhenClosed 钉住"关掉之后一帧都不画"。
func TestMarketDetailDoesNotDrawWhenClosed(t *testing.T) {
	theme := dscore.Dark()
	page := NewPage(theme)
	page.Open(captureEntry(), "https://m.example/index.json")
	page.Close()

	tree := &gfx.Tree{}
	feat := tree.Feature("market-detail-closed")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "market-detail-closed", Width: 1182, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat,
		Bounds: gfx.Rect{Width: 1182, Height: 832}, MainWidth: 902}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env.Canvas = canvas
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(902).FillHeight().Children(func() { page.Draw(f, env) })
		})
		page.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if text := drawnText(session.Snapshot()); strings.Contains(text, "演示插件") {
		t.Fatalf("关掉之后不该再画详情：\n%s", text)
	}
}
