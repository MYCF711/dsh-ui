//go:build darwin || windows || linux

// capture2_test.go —— 第二轮真实渲染审计：右栏 tab 芯片（含 Agent / 沙箱徽标位）
// 与向导入口卡的像素证据。
//
// 这两块的断言在 view_badge_test.go / view_guide_test.go 里已有（徽标在标题
// **之前**、Tag 高 19、芯片 28…），但那些用例没有落图。图上要看的是另一类：
// 徽标与标题基线是否对齐、芯片内容会不会顶到 170 的上限、向导卡在窄列里是否被切。
//
// 截图落 `$RIGHT_TAB_CAPTURE_DIR`（未设置时用临时目录）。
package uikernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func rightTabCaptureDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("RIGHT_TAB_CAPTURE_DIR")
	if dir == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRightTabsWithBadgesCapture 画一排芯片：Info 徽标（Agent）、Neutral 徽标
// （沙箱）、以及没有徽标的普通芯片，三者在同一行里比较。
func TestRightTabsWithBadgesCapture(t *testing.T) {
	dir := rightTabCaptureDir(t)
	tabs := []pluginapi.Tab{
		{Key: "terminal", Kind: "terminal", Label: "终端",
			Badge: &pluginapi.TabBadge{Text: "Agent", Tone: pluginapi.TabBadgeInfo, Hint: "由 Agent 打开"}},
		{Key: "terminal-sandbox", Kind: "terminal", Label: "终端 2",
			Badge: &pluginapi.TabBadge{Text: "沙箱", Tone: pluginapi.TabBadgeNeutral, Hint: "只读沙箱"}},
		{Key: "browser", Kind: "browser", Label: "浏览器"},
	}
	session := openGuideFrame(t, 420, 120, func(env *pluginapi.Env) func(ui.C) {
		return func(f ui.C) { DrawRightTabs(f, env, tabs, "terminal", func(string) {}) }
	})

	// 官方 TerminalTitle.tsx:44-50 的 DOM 顺序 `<TerminalIcon/>{badge}{title}`：
	// 徽标必须在标题之前，且与标题同一条基线（图上一眼能看出歪没歪）。
	boxes := map[string]gfx.Text{}
	for _, one := range session.Snapshot().Texts {
		if _, ok := boxes[one.Value]; !ok {
			boxes[one.Value] = one
		}
	}
	for _, label := range []string{"Agent", "沙箱", "终端", "浏览器"} {
		if _, ok := boxes[label]; !ok {
			t.Fatalf("芯片上没画出 %q：%v", label, session.Texts())
		}
	}
	agent := boxes["Agent"]
	if agent.Style.Size != RightTabBadgeFontSize {
		t.Fatalf("徽标字号=%v，官方 11", agent.Style.Size)
	}
	// `padding:1px 8px` + `line-height:17px` → 19 高（官方 Tag 的实高）。
	if agent.Bounds.Height != RightTabBadgeLineHeight {
		t.Fatalf("徽标行盒高=%v，官方 17（Tag line-height）", agent.Bounds.Height)
	}
	if agent.Bounds.X >= boxes["终端"].Bounds.X {
		t.Fatalf("徽标 x=%v 不在标题 x=%v 之前", agent.Bounds.X, boxes["终端"].Bounds.X)
	}
	path := filepath.Join(dir, "right-tabs-badges.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("RIGHT_TAB_CAPTURE path=%s", path)
}

// TestRightGuideCapture 画向导页的入口卡列（没有占用方时走官方胶囊 fallback，
// GuideBody.tsx:150-157）。窄的 300px 列是右栏最常见宽度，正好看会不会被切。
func TestRightGuideCapture(t *testing.T) {
	dir := rightTabCaptureDir(t)
	entries := []GuideEntry{
		{
			TabGuide: pluginapi.TabGuide{ID: "new", Order: 20, Title: "新建终端",
				Description: "在会话工作区运行命令", IconSize: 26},
			ProviderID: "terminal", Kind: "terminal",
		},
		{
			TabGuide: pluginapi.TabGuide{ID: "browser", Order: 30, Title: "浏览器",
				Description: "预览网页与本地产物", IconSize: 26},
			ProviderID: "browser", Kind: "browser",
		},
	}
	session := openGuideFrame(t, 300, 320, func(env *pluginapi.Env) func(ui.C) {
		return func(f ui.C) { DrawRightGuide(f, env, entries) }
	})
	for _, want := range []string{"新建终端", "浏览器", "在会话工作区运行命令"} {
		if !session.HasText(want) {
			t.Fatalf("向导页没画出 %q：%v", want, session.Texts())
		}
	}
	path := filepath.Join(dir, "right-guide.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("RIGHT_TAB_CAPTURE path=%s", path)
}
