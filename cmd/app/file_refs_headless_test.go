//go:build darwin || windows || linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestFileRefsHeadlessRealClickAndTwoSizeEvidence(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{name: "1182x832", width: 1182, height: 832}, {name: "1920x1032", width: 1920, height: 1032}} {
		t.Run(size.name, func(t *testing.T) {
			runFileRefsHeadless(t, size.width, size.height)
		})
	}
}

func runFileRefsHeadless(t *testing.T, width, height int) {
	t.Helper()
	cwd := t.TempDir()
	relative := "交付 文件.txt"
	absolute := filepath.Join(t.TempDir(), "绝对 文件.txt")
	for _, file := range []string{filepath.Join(cwd, relative), absolute} {
		if err := os.WriteFile(file, []byte("file reference click target"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := func(key, path string) string {
		encoded, _ := json.Marshal(map[string]string{key: path})
		return string(encoded)
	}
	relativeLabel := relative + "#L24-L30"
	absoluteLabel := absolute + "#L7-L9"
	assistant := "## 已完成\n\n" +
		"点击 " + relativeLabel + " 或 " + absoluteLabel + "。\n\n" +
		"未知 缺失.txt 与普通文本不点击；URL https://example.com/交付%20文件.txt 不变成文件。\n\n" +
		"行内码 `" + relative + "` 不链化。\n\n```text\n" + relative + "\n```\n"
	m := &model{cur: "s-file-refs", cwd: cwd, msgs: []apiclient.Message{
		{Role: "tool", Tool: "write", Args: args("file_path", relative)},
		{Role: "tool", Tool: "edit", Args: args("filePath", absolute)},
		{Role: "assistant", TurnID: "turn-file-refs", Text: assistant},
	}}
	m.actions.OpenFile = m.openFile
	m.actions.Notify = m.setStatus

	conversation := &dsconversation.Plugin{}
	kernel := uikernel.New("file-refs-test")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, conversation); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	painter := conversation.Panels()[0]
	tree := &gfx.Tree{}
	feat := tree.Feature("file-refs-headless")
	var view flex.View
	var invalidator app.Invalidator
	data := pluginapi.Snapshot{Cur: m.cur, CWD: m.cwd, Msgs: m.msgs}
	config := app.Config{
		Title: "file-refs-headless", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Act: m.actions, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: current.Width, Scroll: view.ScrollState,
		}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			painter.Draw(f, env)
		})
		painter.AfterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for range 2 {
		invalidator.Invalidate()
		if err := session.Driver().Pump(); err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := session.Hit("缺失.txt"); ok {
		t.Fatal("未知路径被猜成可点击文件")
	}
	captureFileRefs(t, session, width, height)
	if err := session.ClickLabel(relativeLabel); err != nil {
		t.Fatalf("已知相对路径不可点击：%v；texts=%q", err, session.Texts())
	}
	opened, ok := session.Driver().LastOpenedPath()
	wantRelative := filepath.Join(cwd, relative)
	if !ok || opened != wantRelative {
		m.mu.Lock()
		status := m.status
		m.mu.Unlock()
		t.Fatalf("相对路径 OpenPath = %q, %v；期望 %q（#L 片段必须剥离）；status=%q", opened, ok, wantRelative, status)
	}
	// 长 Windows 路径会换行为多个 hit fragment；点击含文件名与行号的可见末段，
	// 仍必须打开同一完整 destination，而不是某个显示片段。
	if err := session.ClickLabel("绝对 文件.txt#L7-L9"); err != nil {
		t.Fatalf("已知绝对路径不可点击：%v；texts=%q", err, session.Texts())
	}
	opened, ok = session.Driver().LastOpenedPath()
	if !ok || opened != absolute {
		t.Fatalf("绝对路径 OpenPath = %q, %v；期望 %q（#L 片段必须剥离）", opened, ok, absolute)
	}
	t.Logf("FILEREFS_CLICK size=%dx%d relative=%q absolute=%q unknown_hit=false", width, height, wantRelative, absolute)
}

func captureFileRefs(t *testing.T, session *headless.Session, width, height int) {
	t.Helper()
	dir := os.Getenv("FILEREFS_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("conversation-file-refs-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	image, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if image.Bounds().Dx() != width || image.Bounds().Dy() != height {
		t.Fatalf("截图尺寸 = %v，期望 %dx%d", image.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：%s info=%v err=%v", path, info, err)
	}
	t.Logf("FILEREFS_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
}
