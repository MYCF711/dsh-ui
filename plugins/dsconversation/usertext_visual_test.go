//go:build windows || darwin || linux

package dsconversation

// usertext_visual_test.go —— 用户消息行内引用 chip 与凭据显示/隐藏的**真画布**验收。
//
// 断言：
//   - `@path` / `@[label](dsh-session:id)` 画成了 chip（不是纯文本）；
//   - 点文件 chip 走 Actions.PreviewFile（不是系统打开、也不是什么都没做）；
//   - 凭据默认不显示原文；点眼睛后在**同一位置**显示原文（再点隐藏）。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

type userTextHarness struct {
	p     *Plugin
	data  pluginapi.Snapshot
	act   pluginapi.Actions
	env   *pluginapi.Env
	view  *ui.Page
	tree  *gfx.Tree
	feat  gfx.Scope
	theme *pluginapi.Theme
}

func newUserTextHarness(t *testing.T, data pluginapi.Snapshot, act pluginapi.Actions) (*headless.Session, *Plugin, *[]string) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	feat := tree.Feature("usertext-headless")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	h := &userTextHarness{p: p, data: data, act: act, view: view, tree: tree, feat: feat, theme: theme}
	previews := &[]string{}
	if act.PreviewFile == nil {
		act.PreviewFile = func(path string) { *previews = append(*previews, path) }
		h.act = act
	} else {
		h.act = act
	}
	var invalidator app.Invalidator
	config := app.Config{
		Title: "usertext-headless", Width: 860, Height: 560, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: h.data, Act: h.act, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
		}
		h.env = env
		view.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).Width(size.Width - 24).Children(func() { p.draw(f, env) })
		})
		p.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session, p, previews
}

func TestUserReferenceChipsRenderAndOpenFile(t *testing.T) {
	data := pluginapi.Snapshot{Cur: "s-1", CWD: `D:\ws`, Msgs: []apiclient.Message{
		{Seq: 1, Role: "user", Text: "看下 @plugins/dsopen/open.go 和 @[上一次对话](dsh-session:s-2) 的结论"},
	}}
	session, _, previews := newUserTextHarness(t, data, pluginapi.Actions{})
	texts := session.Texts()
	for _, want := range []string{"open.go", "上一次对话"} {
		if !containsText(texts, want) {
			t.Fatalf("引用 chip 没画出来：缺 %q；文本=%q", want, texts)
		}
	}
	// chip 是 chip：整串 `@plugins/dsopen/open.go` 不应作为一段文字出现。
	if containsText(texts, "@plugins/dsopen/open.go") {
		t.Fatalf("引用还是纯文本：%q", texts)
	}
	if err := session.ClickLabel("open.go"); err != nil {
		t.Fatalf("点文件 chip 失败: %v", err)
	}
	if got := *previews; len(got) != 1 || got[0] != "plugins/dsopen/open.go" {
		t.Fatalf("PreviewFile 调用 = %v，want [plugins/dsopen/open.go]", got)
	}
}

func TestUserSecretMaskedAndRevealed(t *testing.T) {
	secret := "sk-" + strings.Repeat("a", 24) + "012345"
	data := pluginapi.Snapshot{Cur: "s-2", Msgs: []apiclient.Message{
		{Seq: 1, Role: "user", Text: "我的 key 是 " + secret + " 别外传"},
	}}
	session, _, _ := newUserTextHarness(t, data, pluginapi.Actions{})
	if containsText(session.Texts(), secret) {
		t.Fatalf("凭据原文直接画在屏幕上了：%q", session.Texts())
	}
	hit, ok := session.Hit("显示密钥")
	if !ok {
		t.Fatalf("没有眼睛按钮（命中区）；文本=%q", session.Texts())
	}
	if err := session.ClickLabel("显示密钥"); err != nil {
		t.Fatalf("点眼睛失败: %v", err)
	}
	if !containsText(session.Texts(), secret) {
		t.Fatalf("点开之后仍看不到原文（应在本位显示）；文本=%q", session.Texts())
	}
	// 再点一次回到遮罩态（此时按钮文案是官方 secret.hide「隐藏密钥」）。
	if err := session.ClickLabel("隐藏密钥"); err != nil {
		t.Fatalf("再点眼睛失败: %v", err)
	}
	if containsText(session.Texts(), secret) {
		t.Fatalf("再点之后原文还在：%q", session.Texts())
	}
	_ = hit
}
