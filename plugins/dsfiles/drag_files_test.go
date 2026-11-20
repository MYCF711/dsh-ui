package dsfiles

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这一组测试钉的是 #19 的**拖出**接线：文件行能不能变成一次真的 OS 拖拽，以及
// 点一下文件行还能不能照常选中/预览（阈值那一条最主要——没有它，点开预览全失效）。

// renderDragList 画一帧文件列表，并把树/身份留给测试用来造命中 id。
func renderDragList(t *testing.T, p *Plugin, cwd string, entries []pluginapi.FSEntry) (*gfx.Tree, gfx.Scope) {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: p.theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("files-drag")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	env := &pluginapi.Env{
		Theme: p.theme,
		Data:  pluginapi.Snapshot{CWD: cwd},
		Act: pluginapi.Actions{
			FSList: func(string) []pluginapi.FSEntry { return entries },
			FSRead: func(string) (string, error) { return "", nil },
		},
		Tree: tree, Feat: feat, Canvas: canvas,
		Bounds:    gfx.Rect{Width: 320, Height: 900},
		MainWidth: 320,
		Geometry:  pluginapi.Geometry{Width: 320, Height: 900, SidebarWidth: 280},
	}
	pageTheme := dscore.VivaTheme(p.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) { p.drawList(f, env) })
	return tree, feat
}

// hitTree is the tree identities are made in, and the feature they belong to:
// two rows must come from one tree, or their identities collide, because a name
// is numbered inside the tree that made it.
func hitTree() (*gfx.Tree, gfx.Scope) {
	tree := &gfx.Tree{}
	return tree, tree.Feature("files-drag")
}

// hitID is one row's identity, from a tree of its own.
func hitID(name string) gfx.ID {
	tree, feat := hitTree()
	return tree.ID(feat, name)
}

// dragCalls 记下 startFileDrag 收到的每一次调用。
type dragCalls struct {
	paths   [][]string
	effects []app.FileDragEffect
	err     error
}

// stubDrag 把发起拖出的出口换成记录器，测试结束还原。
func stubDrag(t *testing.T, calls *dragCalls) {
	t.Helper()
	previous := startFileDrag
	startFileDrag = func(paths []string, effects app.FileDragEffect) (app.FileDragEffect, error) {
		calls.paths = append(calls.paths, append([]string(nil), paths...))
		calls.effects = append(calls.effects, effects)
		if calls.err != nil {
			return app.FileDragNone, calls.err
		}
		return app.FileDragCopy, nil
	}
	t.Cleanup(func() { startFileDrag = previous })
}

func press(x, y float32, id gfx.ID) gfx.PointerEvent {
	return gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: id, X: x, Y: y}
}

func move(x, y float32, id gfx.ID) gfx.PointerEvent {
	return gfx.PointerEvent{Kind: gfx.PointerMove, Button: gfx.PointerButtonPrimary, Target: id, X: x, Y: y}
}

func release(x, y float32, id gfx.ID) gfx.PointerEvent {
	return gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary, Target: id, X: x, Y: y}
}

// TestOnlyFilesAreDraggable：目录行不进可拖表（拖一整个目录是另一个承诺），
// 而且可拖表里的路径是**绝对**路径（CF_HDROP 只认绝对路径）。
func TestOnlyFilesAreDraggable(t *testing.T) {
	const cwd = `D:\ws`
	p := &Plugin{}
	tree, feat := renderDragList(t, p, cwd, []pluginapi.FSEntry{
		{Name: "plugins", IsDir: true},
		{Name: "report.pdf"},
	})
	if _, ok := p.draggable[tree.ID(feat, "file-plugins")]; ok {
		t.Fatal("a directory row is draggable")
	}
	path, ok := p.draggable[tree.ID(feat, "file-report.pdf")]
	if !ok {
		t.Fatal("a file row is not draggable")
	}
	if want := filepath.Join(cwd, "report.pdf"); path != want {
		t.Errorf("the row's path is %q, want %q", path, want)
	}
}

// TestAPressOnAFileRowDragsItOut is the whole gesture: the press (which must not
// consume the event, so the click that selects the file still happens), a move
// that is not a drag yet, the move that crosses the threshold and starts the
// drag, and no second drag from a pointer that keeps moving.
func TestAPressOnAFileRowDragsItOut(t *testing.T) {
	calls := &dragCalls{}
	stubDrag(t, calls)
	p := &Plugin{}
	path := filepath.Join(t.TempDir(), "report.pdf")
	id := hitID("file-report.pdf")
	p.dragRows(map[gfx.ID]string{id: path})

	if p.OnPointer(press(10, 10, id)) {
		t.Error("the press was consumed, so the click that selects the file never happens")
	}
	if !p.dragging() {
		t.Fatal("the press was not remembered")
	}
	if p.OnPointer(move(12, 11, id)) {
		t.Error("a move inside the threshold started a drag")
	}
	if len(calls.paths) != 0 {
		t.Fatalf("a move inside the threshold dragged %q", calls.paths)
	}
	if !p.OnPointer(move(30, 10, id)) {
		t.Error("the move that crossed the threshold was not consumed")
	}
	if len(calls.paths) != 1 {
		t.Fatalf("the drag ran %d times, want once", len(calls.paths))
	}
	if len(calls.paths[0]) != 1 || calls.paths[0][0] != path {
		t.Errorf("the drag carried %q, want %q", calls.paths[0], path)
	}
	if want := app.FileDragCopy | app.FileDragMove; calls.effects[0] != want {
		t.Errorf("the drag offered %d, want copy and move (%d)", calls.effects[0], want)
	}
	if p.dragging() {
		t.Error("the press survived the drag that started from it")
	}
	p.OnPointer(move(60, 10, id))
	if len(calls.paths) != 1 {
		t.Errorf("a moving pointer started a second drag: %q", calls.paths)
	}
	if p.OnPointer(release(60, 10, id)) {
		t.Error("the release was consumed")
	}
}

// TestAPressElsewhereIsLeftToThePage keeps every row that is not a file — a
// session, an icon, the whole window — out of the drag.
func TestAPressElsewhereIsLeftToThePage(t *testing.T) {
	calls := &dragCalls{}
	stubDrag(t, calls)
	tree, feat := hitTree()
	p := &Plugin{}
	p.dragRows(map[gfx.ID]string{tree.ID(feat, "file-report.pdf"): filepath.Join(t.TempDir(), "report.pdf")})

	elsewhere := tree.ID(feat, "s-a-session-row")
	if p.OnPointer(press(4, 4, elsewhere)) || p.dragging() {
		t.Fatal("a press off a file row was taken as one")
	}
	p.OnPointer(move(80, 4, elsewhere))
	if len(calls.paths) != 0 {
		t.Fatalf("a drag started off a file row: %q", calls.paths)
	}
}

// TestADragThatFailsSaysWhy：平台不支持拖出（macOS/Linux/Web）时不能假装成功，
// 原因要能到用户眼前，并且要重画一次才看得见。
func TestADragThatFailsSaysWhy(t *testing.T) {
	calls := &dragCalls{err: errors.New("app: system integration is unsupported on this platform")}
	stubDrag(t, calls)
	p := &Plugin{}
	path := filepath.Join(t.TempDir(), "report.pdf")
	id := hitID("file-report.pdf")
	p.dragRows(map[gfx.ID]string{id: path})

	p.OnPointer(press(10, 10, id))
	if !p.OnPointer(move(40, 10, id)) {
		t.Error("a failed drag did not ask for the repaint that shows why")
	}
	if p.dragErr == "" {
		t.Fatal("the failure was not kept for the user")
	}
	if p.dragging() {
		t.Error("the press survived a failed drag")
	}
}
