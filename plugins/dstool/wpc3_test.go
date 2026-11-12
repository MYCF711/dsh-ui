//go:build darwin || windows || linux

package dstool

import (
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func drawWPC3SingleCard(t *testing.T, card Card) *headless.Session {
	t.Helper()
	theme := dscore.Dark()
	cards := New(&dscore.Primitives{Theme: theme})
	tree := &gfx.Tree{}
	feat := tree.Feature("wpc3-single")
	var view flex.View
	config := app.Config{Title: "wpc3-single", Width: 900, Height: 500, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds, MainWidth: size.Width}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			f.Column().WidthPercent(100).Pad(16).Children(func() { cards.Render(f, env, card) })
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	hit, ok := hitByID(session.Snapshot(), tree.ID(feat, "tool-"+strconv.Itoa(card.Seq)))
	if !ok {
		t.Fatalf("工具卡没有标题命中区：%+v", session.Snapshot().Hits)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Press(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Release(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if !cards.Open(card.Seq) {
		t.Fatal("工具卡没有通过真实点击展开")
	}
	return session
}

func requireWPC3CardText(t *testing.T, session *headless.Session, values ...string) {
	t.Helper()
	for _, value := range values {
		if !session.HasText(value) {
			t.Fatalf("展开卡缺少 %q：%q", value, session.Texts())
		}
	}
}

func rejectWPC3Diff(t *testing.T, session *headless.Session) {
	t.Helper()
	for _, value := range []string{"旧内容", "新内容", "diff"} {
		for _, text := range session.Texts() {
			if strings.Contains(text, value) {
				t.Fatalf("缺 old/new 真值却伪造 %q：%q", value, session.Texts())
			}
		}
	}
}

func TestWPC3EditCardPathRangeAndNoInventedDiff(t *testing.T) {
	session := drawWPC3SingleCard(t, Card{Seq: 71, Tool: "edit", Args: `{"file_path":"plugins/dsconversation/wpc3.go","offset":24,"limit":6}`})
	requireWPC3CardText(t, session, "plugins/dsconversation/wpc3.go", "起始行", "24", "行数", "6")
	rejectWPC3Diff(t, session)
}

func TestWPC3EditCardDoesNotInventHalfDiff(t *testing.T) {
	for _, args := range []string{
		`{"file_path":"a.go","old_string":"before"}`,
		`{"file_path":"a.go","new_string":"after"}`,
	} {
		session := drawWPC3SingleCard(t, Card{Seq: 72, Tool: "edit", Args: args})
		requireWPC3CardText(t, session, "a.go")
		rejectWPC3Diff(t, session)
	}
}

func TestWPC3EditCardUsesRealOldNewAndViewPriority(t *testing.T) {
	t.Run("old new 同时存在才展示真实内容", func(t *testing.T) {
		session := drawWPC3SingleCard(t, Card{Seq: 73, Tool: "edit", Args: `{"file_path":"a.go","old_string":"REAL-BEFORE","new_string":"REAL-AFTER"}`})
		requireWPC3CardText(t, session, "a.go", "旧内容", "REAL-BEFORE", "新内容", "REAL-AFTER")
	})
	t.Run("View 优先于参数 diff", func(t *testing.T) {
		session := drawWPC3SingleCard(t, Card{Seq: 74, Tool: "edit", Args: `{"file_path":"a.go","old_string":"SHOULD-HIDE-BEFORE","new_string":"SHOULD-HIDE-AFTER"}`, View: &apiclient.ViewSpec{Kind: "plain", Badge: "ok", Body: "HOST-VIEW-BODY"}})
		requireWPC3CardText(t, session, "HOST-VIEW-BODY")
		for _, hidden := range []string{"旧内容", "新内容", "SHOULD-HIDE-BEFORE", "SHOULD-HIDE-AFTER"} {
			if session.HasText(hidden) {
				t.Fatalf("View 优先时仍画参数内容 %q：%q", hidden, session.Texts())
			}
		}
	})
}
