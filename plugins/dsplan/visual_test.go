package dsplan

// 无头测试：真的画一帧（`ui.NewPage` + 真 `gfx.Canvas` + 真主题），点真的 Hit id，
// 断言画面上的文字与真实动作。不 mock 绘制层、不绕过按钮直接调方法——
// 卡片点击走的是产品同一条路径（`Hit` + `Clicked`）。
//
// 一个 viva 的硬约束：`ui.Box` 只在本次 paint 内有效，跨帧再读会 panic。
// 所以每次画完立刻把矩形抄成纯数值（snap），断言只对数值做。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// actions 记录壳接的动作，全是真实现路径上的那几个。
type actions struct {
	tabs         []string
	toggled      int
	external     []string
	showRightbar bool
}

func (a *actions) install(env *pluginapi.Env) {
	env.Data.ShowRightbar = a.showRightbar
	env.Act.SelectRightTab = func(key string) { a.tabs = append(a.tabs, key) }
	env.Act.ToggleRightbar = func() { a.toggled++ }
	env.Act.OpenExternal = func(destination string) error {
		a.external = append(a.external, destination)
		return nil
	}
}

// fixture 是一帧的设备齐全的无头环境：真主题、真 Tree、真 Canvas。
func fixture(t *testing.T, frames []apiclient.Frame, width float32) (*Plugin, *pluginapi.Env, *actions, *gfx.Canvas) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("dsplan-test")
	p := &Plugin{theme: theme}
	act := &actions{}
	env := &pluginapi.Env{
		Theme: theme,
		Tree:  tree,
		Feat:  feat,
		Data: pluginapi.Snapshot{
			Cur:    "session-1",
			Frames: frames,
		},
		Geometry: pluginapi.Geometry{RightWidth: width},
	}
	act.install(env)
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	// 壳在装配 Env 时会把本帧画布挂上去（`MarkdownView.Layout` 要它量文字）。
	env.Canvas = canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	return p, env, act, canvas
}

// drawCardsOnce 画一帧 Turn 尾卡。
func drawCardsOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.drawCards(f, env) })
}

// drawPreviewOnce 画一帧右栏全文（Draw + AfterDraw 两段，与产品路径一致）。
func drawPreviewOnce(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32) {
	t.Helper()
	canvas.Reset()
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) { p.drawPreview(f, env) })
	p.afterDrawPreview(canvas, env)
}

// click 走一次真实的点击事件通道：BeginEventPass → 画一帧 → EndEventPass。
func click(t *testing.T, p *Plugin, env *pluginapi.Env, canvas *gfx.Canvas, w, h float32, id gfx.ID, draw func()) {
	t.Helper()
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw()
	canvas.EndEventPass()
}

// textValues 抄下画面上所有文字（Markdown 覆画也在里面）。
func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

// joinedTexts 把所有文字拼起来，用于"某段正文出现过"这类断言
// （Markdown 会按行/按 span 拆成多条 Text，单条比较不成立）。
func joinedTexts(canvas *gfx.Canvas) string {
	var out string
	for _, text := range canvas.Texts() {
		out += text.Value
	}
	return out
}
