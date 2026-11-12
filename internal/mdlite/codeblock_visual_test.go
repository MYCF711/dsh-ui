//go:build darwin || windows || linux

package mdlite

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

type codeBlockVisualFixture struct {
	block    CodeBlock
	ids      CodeBlockIDs
	style    CodeBlockStyle
	geometry CodeBlockGeometry
	copied   []string
	copyErr  error
}

func TestCodeBlockHeadlessCopyFeedbackWrapScrollAndScreenshots(t *testing.T) {
	tree := &gfx.Tree{}
	feature := tree.Feature("codeblock-actions")
	fixture := &codeBlockVisualFixture{
		ids: CodeBlockIDs{
			Copy: tree.ID(feature, "copy"), Wrap: tree.ID(feature, "wrap"), Scroll: tree.ID(feature, "scroll"),
		},
		style: DefaultCodeBlockStyle(ui.DarkTheme()),
	}
	original := "package main\r\n\r\nfunc main() {\r\n\tprintln(\"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-你好🙂\")\r\n}\r\n"
	fixture.block = CodeBlock{Language: "go", Title: "演示.go", Code: original}
	fixture.block.Copy = func(text string) error {
		fixture.copied = append(fixture.copied, text)
		return fixture.copyErr
	}

	config := app.Config{Title: "codeblock-actions", Width: 760, Height: 560, Tree: tree, WindowMode: app.WindowFrameless}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.DarkTheme().Background)
		height := fixture.block.Height(canvas, 560, fixture.style)
		bounds := ui.Rect{X: 20, Y: 20, Width: 560, Height: height}
		fixture.geometry = fixture.block.Draw(canvas, bounds, fixture.ids, fixture.style)
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult { return fixture.block.HandleScroll(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if !session.HasText("go") || !session.HasText("演示.go") {
		t.Fatalf("language/title banner not painted: %q", session.Texts())
	}
	if fixture.geometry.MaxOffsetX <= 0 {
		t.Fatalf("headless long line has no horizontal overflow: %+v", fixture.geometry)
	}
	if err := session.ClickLabel("复制代码"); err != nil {
		t.Fatal(err)
	}
	if len(fixture.copied) != 1 || fixture.copied[0] != original {
		t.Fatalf("copy callback bytes changed: count=%d got=%q want=%q", len(fixture.copied), fixture.copied, original)
	}
	if !session.HasText("已复制") || fixture.block.CopyStatus() != CopySucceeded {
		t.Fatalf("success feedback missing: texts=%q status=%v", session.Texts(), fixture.block.CopyStatus())
	}
	captureCodeBlock(t, session, "codeblock-copy-success.png")

	fixture.copyErr = errors.New("injected clipboard failure")
	if err := session.ClickLabel("复制代码"); err != nil {
		t.Fatal(err)
	}
	if len(fixture.copied) != 2 || fixture.copied[1] != original {
		t.Fatalf("failure path did not copy exact original: %q", fixture.copied)
	}
	if !session.HasText("复制失败") || fixture.block.CopyStatus() != CopyFailed {
		t.Fatalf("failure feedback missing: texts=%q status=%v", session.Texts(), fixture.block.CopyStatus())
	}
	captureCodeBlock(t, session, "codeblock-copy-failure.png")

	firstLine := firstMonoText(t, session.Snapshot().Texts)
	viewport := fixture.geometry.Content
	if err := session.Scroll(gfx.ScrollEvent{X: viewport.X + viewport.Width/2, Y: viewport.Y + viewport.Height/2, DeltaX: 96, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if fixture.block.State().OffsetX <= 0 {
		t.Fatalf("real scroll event did not change OffsetX: target=%+v state=%+v", viewport, fixture.block.State())
	}
	if after := firstMonoText(t, session.Snapshot().Texts); after.Bounds.X >= firstLine.Bounds.X {
		t.Fatalf("horizontal scroll did not move painted code: before=%+v after=%+v", firstLine, after)
	}

	beforeWrapLines := fixture.geometry.VisualLines
	savedOffset := fixture.block.State().OffsetX
	if err := session.ClickLabel("切换折行"); err != nil {
		t.Fatal(err)
	}
	if !fixture.block.State().Wrap || fixture.geometry.MaxOffsetX != 0 || fixture.geometry.VisualLines <= beforeWrapLines {
		t.Fatalf("wrap toggle not effective: state=%+v geometry=%+v beforeLines=%d", fixture.block.State(), fixture.geometry, beforeWrapLines)
	}
	if fixture.block.State().OffsetX != savedOffset {
		t.Fatalf("wrap changed saved horizontal offset: got=%v want=%v", fixture.block.State().OffsetX, savedOffset)
	}
	var restored CodeBlock
	restored.Restore(fixture.block.State())
	if restored.State() != fixture.block.State() {
		t.Fatalf("state did not survive recreation: got=%+v want=%+v", restored.State(), fixture.block.State())
	}
	captureCodeBlock(t, session, "codeblock-wrapped.png")

	if err := session.ClickLabel("切换折行"); err != nil {
		t.Fatal(err)
	}
	if fixture.block.State().Wrap || fixture.block.State().OffsetX != savedOffset {
		t.Fatalf("unwrapped state was not restored: %+v", fixture.block.State())
	}
}

func firstMonoText(t *testing.T, texts []gfx.Text) gfx.Text {
	t.Helper()
	for _, text := range texts {
		if text.Style.Mono && text.Value != "" {
			return text
		}
	}
	t.Fatal("no painted mono code text")
	return gfx.Text{}
}

func captureCodeBlock(t *testing.T, session *headless.Session, name string) {
	t.Helper()
	dir := os.Getenv("MDLITE_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if picture.Bounds().Dx() != 760 || picture.Bounds().Dy() != 560 {
		t.Fatalf("screenshot dimensions = %v", picture.Bounds())
	}
	colors := make(map[color.Color]struct{})
	for y := 0; y < picture.Bounds().Dy(); y += 7 {
		for x := 0; x < picture.Bounds().Dx(); x += 7 {
			colors[picture.At(x, y)] = struct{}{}
		}
	}
	if len(colors) < 4 {
		t.Fatalf("screenshot appears blank: only %d sampled colors", len(colors))
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("empty screenshot %s: info=%v err=%v", path, info, err)
	}
	t.Logf("MDLITE_CAPTURE path=%s size=760x560 bytes=%d sampled_colors=%d", path, info.Size(), len(colors))
}
