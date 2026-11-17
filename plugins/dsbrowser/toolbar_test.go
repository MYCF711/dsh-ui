package dsbrowser

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// TestToolbarBoxModelMatchesTheOfficialBorderBox 是本轮修掉的一处几何：
// 官方 `.toolbar{height:38px; box-sizing:border-box; border-bottom:.5px …}`
// （Browser.module.css:11-20）——那条 0.5px 线**算在 38 之内**，内容盒是 37.5。
// 本仓用"一行 37.5 + 一条 0.5 线"表达同一条边，合计必须正好 38；
// 之前的"38 + 0.5"多出 0.5px，整块内容区被推下去半像素。
func TestToolbarBoxModelMatchesTheOfficialBorderBox(t *testing.T) {
	if toolbarHeight != 38 || hairline != 0.5 {
		t.Fatalf("官方 .toolbar 高 %v、描边 %v", toolbarHeight, hairline)
	}
	host := &fakeHost{}
	env := &pluginapi.Env{Web: host}
	p := &Plugin{}
	p.theme = dscore.Dark()
	p.address.SetText("example.com")
	p.submit(env)
	canvas := renderBrowser(t, p, env)
	p.afterDraw(canvas, env)
	views := canvas.NativeViews()
	if len(views) != 1 {
		t.Fatalf("应当摆 1 个 native view，得到 %d", len(views))
	}
	if got := views[0].Bounds.Y; got != toolbarHeight {
		t.Fatalf("内容区 y=%v，应当是工具条总高 %v（border-box）", got, toolbarHeight)
	}
}

// TestGoButtonOnlyExistsWhileTheAddressHasFocus 是官方 `.addressGo` 的两态：
//
//	.addressGo{visibility:hidden;opacity:0}            （Browser.module.css:72-78）
//	.addressBox:focus-within .addressGo{visible}       （:80-83）
//
// 本仓判据 = 画布焦点 id 等于地址框的 id（与 focus-within 同义）。
func TestGoButtonOnlyExistsWhileTheAddressHasFocus(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-go-test")
	goID := tree.ID(feat, "browser-go")
	addressID := tree.ID(feat, "browser-address")

	renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, gfx.ID{})
	blurred := renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, gfx.ID{})
	if _, ok := blurred.HitBounds(goID); ok {
		t.Fatal("地址框没焦点时「前往」仍然在（官方 :focus-within 才显示）")
	}

	focused := renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, addressID)
	if _, ok := focused.HitBounds(goID); !ok {
		t.Fatal("地址框拿到焦点后「前往」没有出现")
	}
}

// TestUnknownStateShowsTheChangedBadge 是官方 `.addressChanged`（:98-113）：
// 页面自己跳走、地址读不回来时，地址框右上角挂一枚「URL 已变化」，聚焦时隐去。
func TestUnknownStateShowsTheChangedBadge(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-changed-test")
	addressID := tree.ID(feat, "browser-address")
	// 先画一帧把 occurrence 激活（激活时 p.nav 会换成该实例的历史），
	// 之后再摆导航状态，否则会被激活覆盖。
	renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, gfx.ID{})
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://example.com/x", Title: "example.com"})
	request := p.nav.value.Request
	if request == nil {
		t.Fatal("navigate 没有产生请求")
	}
	// loading → known → unknown（官方 frameLoaded 的第二跳，navigation.ts:120-128）。
	p.nav.frameLoaded(request.Revision)
	p.nav.frameLoaded(request.Revision)
	if !p.unknown() {
		t.Fatalf("历史状态应当是 unknown，得到 %v", p.nav.value.Navigation.State)
	}

	canvas := renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, gfx.ID{})
	if !containsText(canvas, copyAddressChanged) {
		t.Fatalf("unknown 态没有画官方 address.changed「%s」", copyAddressChanged)
	}
	if copyAddressChanged != "URL 已变化" {
		t.Fatalf("address.changed 文案 = %q，官方 locales.ts:8 是「URL 已变化」", copyAddressChanged)
	}

	focused := renderBrowserInteractionWith(t, p, &pluginapi.Env{Web: &fakeHost{}}, tree, feat, addressID)
	if containsText(focused, copyAddressChanged) {
		t.Fatal("地址框聚焦时角标应当隐去（官方 .addressBox:focus-within .addressChanged{visibility:hidden}）")
	}
}

// TestExternalToolIsDisabledWhenTheAddressIsUnknown 是官方
// `externalUrl = unknown ? undefined : target?.url`（BrowserBody.tsx:73）→
// 页面跳走之后那枚「在系统浏览器中打开」必须置灰（:99 disabled）。
// 本仓禁用 = 不登记 Hit，所以按钮也不吃点击。
func TestExternalToolIsDisabledWhenTheAddressIsUnknown(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-external-test")
	externalID := tree.ID(feat, "browser-external")
	opened := ""
	env := &pluginapi.Env{Web: &fakeHost{}}
	env.Act.OpenExternal = func(destination string) error {
		opened = destination
		return nil
	}

	renderBrowserInteractionWith(t, p, env, tree, feat, gfx.ID{})
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://example.com/x", Title: "example.com"})
	visible := renderBrowserInteractionWith(t, p, env, tree, feat, gfx.ID{})
	if _, ok := visible.HitBounds(externalID); !ok {
		t.Fatal("有可信地址时「在系统浏览器中打开」应当在")
	}

	request := p.nav.value.Request
	p.nav.frameLoaded(request.Revision)
	p.nav.frameLoaded(request.Revision)
	hidden := renderBrowserInteractionWith(t, p, env, tree, feat, gfx.ID{})
	if _, ok := hidden.HitBounds(externalID); ok {
		t.Fatal("地址 unknown 时「在系统浏览器中打开」仍然可点（官方 disabled）")
	}
	if opened != "" {
		t.Fatalf("unknown 态打开了系统浏览器：%q", opened)
	}
}

// TestFailureRowUsesTheOfficialTintAndLineHeight 钉住两条被改错的数值：
//
//	.failure{background:color-mix(in srgb, state-error-primary 8%, transparent)}（:218-224）
//	.failure / .limit 的 font 是 `xxxs-11` = 11px/**14px**（:218-224、:234-244）
func TestFailureRowUsesTheOfficialTintAndLineHeight(t *testing.T) {
	if failureTint != 0.08 {
		t.Fatalf("错误行底色 = 错误色的 %v，官方 8%%", failureTint)
	}
	if lineHeightXXS != 14 {
		t.Fatalf("xxxs-11 行高 = %v，官方 14", lineHeightXXS)
	}
	if statusRowPadY != 6 || statusRowPadX != 12 {
		t.Fatalf("错误行内边距 = %v/%v，官方 6/12", statusRowPadY, statusRowPadX)
	}
	if limitRowPadY != 4 || limitRowPadX != 10 {
		t.Fatalf(".limit 内边距 = %v/%v，官方 4/10", limitRowPadY, limitRowPadX)
	}
	// 地址角标的四边几何：top 6、right 8、padding-left 8、line-height 16。
	if addressChangedTop != 6 || addressChangedRgt != 8 || addressChangedPadL != 8 || addressChangedLine != 16 {
		t.Fatalf("addressChanged 几何 = top %v right %v pad-left %v line %v，官方 6/8/8/16",
			addressChangedTop, addressChangedRgt, addressChangedPadL, addressChangedLine)
	}
	if addressFocusRing != 1 {
		t.Fatalf("聚焦描边宽 = %v，官方 outline 1px", addressFocusRing)
	}
}

// TestAddressCommandsClearTheRefusal 是官方 `command()`（BrowserController.ts:163-168）：
// 每一次命令都清掉 addressFailure，被拒的红行不该赖在屏幕上。
func TestAddressCommandsClearTheRefusal(t *testing.T) {
	p := &Plugin{}
	p.theme = dscore.Dark()
	env := &pluginapi.Env{Web: &fakeHost{}}
	tree := &gfx.Tree{}
	feat := tree.Feature("browser-refusal-test")
	renderBrowserInteractionWith(t, p, env, tree, feat, gfx.ID{})
	p.address.SetText("file:///etc/passwd")
	p.submit(env)
	if p.failure != failProtocol {
		t.Fatalf("被拒原因 = %q，应当是 %q", p.failure, failProtocol)
	}
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://example.com/a", Title: "example.com"})
	p.nav.navigate(browserTarget{Kind: kindHTTPS, URL: "https://example.com/b", Title: "example.com"})
	p.goBack(env)
	if p.failure != "" {
		t.Fatalf("后退之后仍带被拒原因 %q", p.failure)
	}
}

// ── 夹具 ────────────────────────────────────────────────────────────────

// renderBrowserInteractionWith 用调用方给的 Tree/Feature 画一帧，并把焦点摆到
// focused 上（零值 = 没有任何焦点）。
func renderBrowserInteractionWith(t *testing.T, p *Plugin, env *pluginapi.Env,
	tree *gfx.Tree, feat gfx.Scope, focused gfx.ID) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, focused, false)
	env.Theme = p.theme
	env.Tree = tree
	env.Feat = feat
	env.Canvas = canvas
	env.Bounds = gfx.Rect{Width: 360, Height: 600}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		ui.Column(f).Width(360).Height(600).Children(func() { p.drawBody(f, env) })
	})
	return canvas
}

// containsText 报告画布上是否画过 value。
func containsText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}
