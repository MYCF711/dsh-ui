//go:build darwin || windows || linux

package mdlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

type linkVisualFixture struct {
	http, file Link
	markdown   markdown.Document
	view       components.MarkdownView
	actions    MarkdownLinkActions
	look       components.MarkdownLook
	opened     []LinkTarget
}

func TestLinksHeadlessRealRegionsCallbacksAndBoundaries(t *testing.T) {
	theme := ui.DarkTheme()
	tree := &gfx.Tree{}
	feature := tree.Feature("link-actions")
	fixture := &linkVisualFixture{look: components.MarkdownLookFor(theme)}
	open := func(target LinkTarget) { fixture.opened = append(fixture.opened, target) }
	fixture.http = Link{
		Text:   "打开 HTTP 文档",
		Target: LinkTarget{Kind: LinkKindHTTPS, Destination: "https://example.com/docs?q=你好"},
		Open:   open,
	}
	fixture.file = Link{
		Text:   "internal/mdlite/highlight.go#L14-L75",
		Target: LinkTarget{Kind: LinkKindFile, Destination: "internal/mdlite/highlight.go#L14-L75"},
		Open:   open,
	}
	fixture.markdown.Parse("正文 [Viva HTTP](https://example.com/markdown?a=1&b=2) 结束。")
	fixture.actions = MarkdownLinkActions{Scope: tree.Sub(feature, tree.Name("markdown-links")), Open: open}

	config := app.Config{Title: "link-actions", Width: 760, Height: 320, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, theme.Background)
		style := gfx.TextStyle{Size: 14, Color: fixture.look.Link}
		fixture.http.Draw(canvas, gfx.Rect{X: 20, Y: 20, Width: 600, Height: 28}, tree.ID(feature, "http"), style)
		fixture.file.Draw(canvas, gfx.Rect{X: 20, Y: 60, Width: 600, Height: 28}, tree.ID(feature, "file"), style)
		fixture.view.Layout(canvas, components.MarkdownStyle{Size: 14}, &fixture.markdown, 600)
		fixture.actions.Paint(canvas, &fixture.view, components.MarkdownPaint{
			Look: fixture.look, X: 20, Y: 110, Clip: gfx.Rect{X: 20, Y: 100, Width: 600, Height: 100},
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for _, label := range []string{"打开 HTTP 文档", fixture.file.Text, "Viva HTTP"} {
		hit, ok := session.Hit(label)
		if !ok {
			t.Fatalf("link %q has no reachable hit region; hits=%+v", label, session.Snapshot().Hits)
		}
		if hit.Role != gfx.RoleLink || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
			t.Errorf("link %q is only painted text, hit=%+v", label, hit)
		}
		assertLinkBoundary(t, session, hit)
	}
	if len(fixture.opened) != 0 {
		t.Fatalf("boundary clicks opened links: %+v", fixture.opened)
	}

	if err := session.ClickLabel("打开 HTTP 文档"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel(fixture.file.Text); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Viva HTTP"); err != nil {
		t.Fatal(err)
	}
	want := []LinkTarget{fixture.http.Target, fixture.file.Target, {Kind: LinkKindHTTPS, Destination: "https://example.com/markdown?a=1&b=2"}}
	if len(fixture.opened) != len(want) {
		t.Fatalf("opened = %+v, want %+v", fixture.opened, want)
	}
	for i := range want {
		if fixture.opened[i] != want[i] {
			t.Errorf("opened[%d] = %+v, want %+v", i, fixture.opened[i], want[i])
		}
	}

	dir := os.Getenv("MDLITE_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "links-hit-regions.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("empty link screenshot: info=%v err=%v", info, err)
	}
	t.Logf("MDLITE_CAPTURE path=%s size=760x320 bytes=%d http_file_markdown_callbacks=3 boundary_checks=3", path, info.Size())
}

func assertLinkBoundary(t *testing.T, session *headless.Session, hit gfx.Hit) {
	t.Helper()
	centerY := hit.Bounds.Y + hit.Bounds.Height/2
	if id, ok := session.Driver().HitTest(hit.Bounds.X+0.5, centerY); !ok || id != hit.ID {
		t.Errorf("inside edge does not hit %q: id=%v ok=%v bounds=%+v", hit.Label, id, ok, hit.Bounds)
	}
	points := []ui.Point{
		{X: hit.Bounds.X - 1, Y: centerY},
		{X: hit.Bounds.X + hit.Bounds.Width + 1, Y: centerY},
		{X: hit.Bounds.X + hit.Bounds.Width/2, Y: hit.Bounds.Y - 1},
		{X: hit.Bounds.X + hit.Bounds.Width/2, Y: hit.Bounds.Y + hit.Bounds.Height + 1},
	}
	for _, point := range points {
		if id, ok := session.Driver().HitTest(point.X, point.Y); ok && id == hit.ID {
			t.Errorf("outside point %+v still hits %q bounds=%+v", point, hit.Label, hit.Bounds)
		}
		if err := session.Click(point.X, point.Y); err != nil {
			t.Fatal(err)
		}
	}
}
