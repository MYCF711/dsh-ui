//go:build darwin || windows || linux

package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/headless"
)

// OPEN-IN-APP 的第 2 条验收：真实点击 → 精确 CWD → 真实 OS 打开。
//
// 这条必须走完整链路：Viva Paint → HitTest → ClickLabel → ds-open 的
// `Actions.OpenFile(file:…)` → `model.openFile` → `filePathFromLink`
// → `openLocal` → 目录交给文件管理器。中间任何一步退回裸路径/OpenExternal，
// 真机上都会变成"点一次失败一次"。

func TestOpenInAppHeadlessRealClickAndTwoSizeEvidence(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{name: "1182x832", width: 1182, height: 832}, {name: "1920x1032", width: 1920, height: 1032}} {
		t.Run(size.name, func(t *testing.T) {
			runOpenInAppHeadless(t, size.width, size.height)
		})
	}
}

func runOpenInAppHeadless(t *testing.T, width, height int) {
	t.Helper()
	cwd := t.TempDir()
	m := &model{cur: "s-open", cwd: cwd}
	rig := newOpenInAppRig(t, m, width, height)
	defer rig.close()

	// 目录打开的真实落点：测试里换成记录器，真机上这里会拉起文件管理器。
	originalOpenDirectory := openDirectory
	var directories []string
	openDirectory = func(path string) error {
		directories = append(directories, path)
		return nil
	}
	defer func() { openDirectory = originalOpenDirectory }()

	// 动作边界也记一份：它是"点击收到了什么"的最直接证据。
	var destinations []string
	rig.actions.OpenFile = func(destination string) error {
		destinations = append(destinations, destination)
		return m.openFile(destination)
	}
	rig.repaint()

	if len(directories) != 0 || len(destinations) != 0 {
		t.Fatalf("还没点就打开了东西：dirs=%v files=%v", directories, destinations)
	}
	hit, ok := rig.session.Hit("在工作目录里打开")
	if !ok {
		t.Fatalf("%dx%d 按钮不可见：texts=%q", width, height, rig.session.Texts())
	}
	if hit.Bounds.Height <= 0 {
		t.Fatalf("%dx%d 按钮高度 = %v（0 高就是看不见）", width, height, hit.Bounds.Height)
	}
	corner, _ := rig.session.Hit("打开右侧边栏")
	tabs, _ := rig.session.Hit("对话")
	t.Logf("OPEN_IN_APP_HIT size=%dx%d button=%+v corner=%+v tabs=%+v", width, height, hit.Bounds, corner.Bounds, tabs.Bounds)
	if err := rig.session.ClickLabel("在工作目录里打开"); err != nil {
		t.Fatalf("%dx%d 真实点击失败：%v；texts=%q", width, height, err, rig.session.Texts())
	}

	wantLink := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(cwd)}).String()
	if len(destinations) != 1 || destinations[0] != wantLink {
		t.Fatalf("%dx%d OpenFile 收到 %v，want [%q]（必须精确等于 CWD 的 file 链接）",
			width, height, destinations, wantLink)
	}
	if len(directories) != 1 || directories[0] != cwd {
		t.Fatalf("%dx%d 文件管理器收到 %v，want [%q]", width, height, directories, cwd)
	}
	// **没有**任何东西被当成 URL 交给系统：裸 Windows 路径进 OpenExternal，
	// app.OpenURL 只会回一句 "external URL must use http, https, or mailto"。
	if opened, ok := rig.session.Driver().LastOpenedURL(); ok {
		t.Fatalf("%dx%d 本地目录被当成外链打开了：%q", width, height, opened)
	}
	// 目录不走 app.OpenPath：Viva 明确拒绝非常规文件，所以 driver 不会记，
	// 也不许为了"看起来成功"伪造一条记录。目录的真实落点是上面的文件管理器。
	if opened, ok := rig.session.Driver().LastOpenedPath(); ok {
		t.Fatalf("%dx%d 目录被当成文件打开：%q", width, height, opened)
	}
	if status := openInAppStatus(m); status != "已打开目录" {
		t.Fatalf("%dx%d 状态栏 = %q，want「已打开目录」（失败会写「打开目录失败: …」）", width, height, status)
	}

	// 同一条 glue 对**常规文件**仍走 Viva 的 app.OpenPath：driver 记下精确路径。
	file := filepath.Join(cwd, "真实 文件.txt")
	if err := os.WriteFile(file, []byte("open me"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileLink := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(file)}).String()
	if err := m.openFile(fileLink); err != nil {
		t.Fatalf("同一条 glue 打不开常规文件：%v", err)
	}
	if opened, ok := rig.session.Driver().LastOpenedPath(); !ok || opened != file {
		t.Fatalf("%dx%d LastOpenedPath = %q, %v；want %q", width, height, opened, ok, file)
	}
	if status := openInAppStatus(m); status != "已打开文件" {
		t.Fatalf("%dx%d 状态栏 = %q，want「已打开文件」", width, height, status)
	}

	captureOpenInApp(t, rig.session, width, height)
}

func openInAppStatus(m *model) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// captureOpenInApp 落一张真实像素图，并断言它不是空文件、尺寸与窗口一致。
func captureOpenInApp(t *testing.T, session *headless.Session, width, height int) {
	t.Helper()
	dir := os.Getenv("OPEN_IN_APP_CAPTURE_DIR")
	if dir == "" {
		dir = "testdata/open-in-app"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("open-in-app-%dx%d.png", width, height))
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
		t.Fatalf("截图尺寸 = %v，want %dx%d", picture.Bounds(), width, height)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：%s info=%v err=%v", path, info, err)
	}
	t.Logf("OPEN_IN_APP_CAPTURE path=%s size=%dx%d bytes=%d status=%s",
		path, width, height, info.Size(), "已打开目录")
}
