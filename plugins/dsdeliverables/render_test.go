package dsdeliverables

// 无头渲染测试：真的开一个 `ui.NewPage` 画一帧，点真的 Hit id，断言**画面上出现的文字**。
//
// 为什么不能只测 `planOverlay`：决策对了不等于画得出来——hunk 行在滚动区里
// （`ui.Scroll`）、正负号与行号是分开的 Text、兜底路径要真的把全文画在说明行下面。
// 这些只有真画一帧才看得见（与 `plugins/dsattachment/rail_test.go` 同一套做法）。
//
// 一个 viva 的硬约束：`ui.Box` 只在**本次 paint** 内有效（box.go:9-12），
// 所以断言全部只读 `canvas.Texts()` 里的纯值。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// previewText 是假 FSRead 返回的全文（兜底路径要画的就是它）。
const previewText = "one\ntwo\nthree\n"

type renderFixture struct {
	plugin *Plugin
	env    *pluginapi.Env
	tree   *gfx.Tree
	feat   gfx.Scope
	canvas *gfx.Canvas
	reads  []string
}

// newRenderFixture 造一个"侧栏里写过 src/a.go、当前会话有一条 workspace/changes 帧"的环境。
func newRenderFixture(t *testing.T) *renderFixture {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	feat := tree.Feature("deliverables-test")
	fx := &renderFixture{
		plugin: &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}},
		tree:   tree,
		feat:   feat,
		canvas: &gfx.Canvas{},
	}
	fx.env = &pluginapi.Env{
		Theme:  theme,
		Tree:   tree,
		Feat:   feat,
		Bounds: ui.Rect{Width: 1180, Height: 700},
		Data: pluginapi.Snapshot{
			Cur: "s1",
			Msgs: []apiclient.Message{
				{Role: "tool", Tool: "write", Args: `{"filePath":"src/a.go","content":"x"}`},
			},
			Frames: []apiclient.Frame{
				{Seq: 1, Type: "user/message"},
				{Seq: 9, Type: ChangesFrameType},
			},
		},
		Act: pluginapi.Actions{FSRead: func(path string) (string, error) {
			fx.reads = append(fx.reads, path)
			return previewText, nil
		}},
	}
	fx.canvas.SetTree(tree)
	fx.canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size / 2
	})
	t.Cleanup(func() { SetChangesSource(nil) })
	return fx
}

// draw 画一帧（侧栏列表 + 浮层，与壳的 painter 顺序一致，见 Panels()）。
func (fx *renderFixture) draw(t *testing.T, w, h float32) {
	t.Helper()
	fx.canvas.Reset()
	pageTheme := dscore.VivaTheme(fx.plugin.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(fx.canvas, gfx.Rect{Width: w, Height: h}, func(f ui.C) {
		fx.plugin.draw(f, fx.env)
		fx.plugin.drawPreview(f, fx.env)
	})
}

// click 走一次真实的点击通道：BeginEventPass → 画一帧 → EndEventPass。
//
// 注意：点击的**效果**（改了插件状态）在这一帧里就生效，但"改完之后画成什么样"
// 要下一帧才看得到——壳收到点击会重绘一次，测试里那一下就是 `draw` 再来一帧。
func (fx *renderFixture) click(t *testing.T, id gfx.ID, w, h float32) {
	t.Helper()
	fx.canvas.BeginEventPass(id, gfx.ClickInfo{})
	fx.draw(t, w, h)
	fx.canvas.EndEventPass()
}

// clickThenSettle 点一下再画一帧：断言"点击之后画面变成什么样"时用它。
func (fx *renderFixture) clickThenSettle(t *testing.T, id gfx.ID, w, h float32) {
	t.Helper()
	fx.click(t, id, w, h)
	fx.draw(t, w, h)
}

func (fx *renderFixture) texts() []string {
	out := make([]string, 0, len(fx.canvas.Texts()))
	for _, text := range fx.canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func (fx *renderFixture) has(value string) bool {
	for _, text := range fx.texts() {
		if text == value {
			return true
		}
	}
	return false
}

// hasPrefix 报告画面里有没有以某段前缀开头的文字（hunk 头里带行号，值逐字比对太脆）。
func (fx *renderFixture) hasPrefix(prefix string) bool {
	for _, text := range fx.texts() {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func (fx *renderFixture) fail(t *testing.T, what string) {
	t.Helper()
	t.Fatalf("%s；画面文字 = %q", what, fx.texts())
}

// rowID / closeID / retryID 复算插件用的 Hit id（同一套 env.Tree.ID）。
func (fx *renderFixture) rowID(name string) gfx.ID { return fx.tree.ID(fx.feat, name) }

// TestOverlayDrawsHunkRows 是一条端到端判据：点侧栏那一行 → 浮层里出现
// **真的 hunk**（头、正文去前缀、增删行、两个行号列），以及头部的 +N/-M 计数。
func TestOverlayDrawsHunkRows(t *testing.T) {
	fx := newRenderFixture(t)
	SetChangesSource(&changesFake{
		summary: &apiclient.ChangesSummary{Turn: 1, Cwd: "/w", Files: []apiclient.ChangesFile{
			{Path: "src/a.go", Display: "src/a.go", Added: 2, Deleted: 1}}},
		diff: &apiclient.ChangesDiff{Kind: "text", Path: "src/a.go", Display: "src/a.go",
			Before: true, After: true,
			Hunks: []apiclient.ChangesHunk{{OldStart: 1, OldLines: 2, NewStart: 1, NewLines: 3,
				Lines: []string{" one", "-two", "+TWO", "+three"}}}},
	})
	fx.click(t, fx.rowID("deliv-0"), 1180, 700)

	for _, want := range []string{
		"写过的文件",    // 侧栏旧路径还在（不是被这一条替换掉了）
		"a.go",     // 侧栏只显示文件名
		"src/a.go", // 浮层头部用摘要的 display
		"+2", "-1", // 头部计数（OFF:locales.ts:35-36）
		"@@ -1,2 +1,3 @@",            // hunk 头（OFF:FileDiff.tsx:165-167）
		"one", "two", "TWO", "three", // 正文（前缀字符被剥掉）
		"+", "-", " ",
	} {
		if !fx.has(want) {
			fx.fail(t, "画面里缺 "+want)
		}
	}
	// 有 diff 时**不该**去读全文（多一次 fs.read 是白花的 HTTP）。
	if len(fx.reads) != 0 {
		t.Fatalf("有 diff 可画时不该读全文，实际读了 %v", fx.reads)
	}
}

// TestOverlayFallsBackToFullTextWhenHostHasNoRecord 是这一条的核心：
// 宿主回 404（非 git 会话就是这样）时，浮层必须写明「这一轮没有变更记录」
// **并退回全文预览**，一行 diff 都不画。
func TestOverlayFallsBackToFullTextWhenHostHasNoRecord(t *testing.T) {
	fx := newRenderFixture(t)
	SetChangesSource(&changesFake{summaryErr: missingErr()})
	fx.click(t, fx.rowID("deliv-0"), 1180, 700)

	if !fx.has(msgNoChangesRecord) {
		fx.fail(t, "404 时浮层没写「"+msgNoChangesRecord+"」")
	}
	for _, want := range []string{"one", "two", "three"} {
		if !fx.has(want) {
			fx.fail(t, "404 时该退回全文预览，缺 "+want)
		}
	}
	if len(fx.reads) != 1 || fx.reads[0] != "src/a.go" {
		t.Fatalf("兜底路径读了 %v，want [src/a.go]", fx.reads)
	}
	if fx.hasPrefix("@@ ") {
		fx.fail(t, "没有记录时不该画 hunk 头")
	}
}

// TestOverlayNoChangesFrameFallsBack 验"这一轮根本没有 workspace/changes 帧"
// （非 git 工作目录的会话）走的是同一条兜底路径，而且**一次请求都不发**。
func TestOverlayNoChangesFrameFallsBack(t *testing.T) {
	fx := newRenderFixture(t)
	src := &changesFake{}
	SetChangesSource(src)
	fx.env.Data.Frames = []apiclient.Frame{{Seq: 1, Type: "user/message"}}
	fx.click(t, fx.rowID("deliv-0"), 1180, 700)

	if !fx.has(msgNoChangesRecord) || !fx.has("two") {
		fx.fail(t, "没有 workspace/changes 帧时该退回全文预览")
	}
	if src.summaryCalls != 0 {
		t.Fatalf("没有 seq 就不该发请求，实际 %d 次", src.summaryCalls)
	}
}

// TestOverlayBinaryAndOversizedShowOfficialCopy 验拒绝支只显示官方那两句，
// 不画假 hunk、也不退回全文（宿主明确说了"看不到改动"，画全文是另一回事）。
func TestOverlayBinaryAndOversizedShowOfficialCopy(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{"binary", msgDiffBinary},
		{"oversized", msgDiffOversized},
	}
	for _, tc := range cases {
		fx := newRenderFixture(t)
		SetChangesSource(&changesFake{
			summary: &apiclient.ChangesSummary{Turn: 1, Files: []apiclient.ChangesFile{
				{Path: "src/a.go", Display: "src/a.go", Binary: tc.kind == "binary", Oversized: tc.kind == "oversized"}}},
			diff: &apiclient.ChangesDiff{Kind: tc.kind, Path: "src/a.go", Display: "src/a.go"},
		})
		fx.click(t, fx.rowID("deliv-0"), 1180, 700)
		if !fx.has(tc.want) {
			fx.fail(t, tc.kind+" 该显示「"+tc.want+"」")
		}
		if fx.hasPrefix("@@ ") || fx.has("+TWO") {
			fx.fail(t, tc.kind+" 不该画 hunk")
		}
		if len(fx.reads) != 0 {
			t.Fatalf("%s 不该退回全文预览，实际读了 %v", tc.kind, fx.reads)
		}
	}
}

// TestOverlayFailedReadShowsRetryAndRefetches 验读失败时画官方「无法读取改动」+「重试」，
// 而且**点真的那一行**会重新问一次宿主（不是画个样子）。
func TestOverlayFailedReadShowsRetryAndRefetches(t *testing.T) {
	fx := newRenderFixture(t)
	src := &changesFake{summaryErr: &apiclient.ChangesError{
		Status: 500, Kind: apiclient.ChangesStatusFailed, Code: "internal", Message: "炸了"}}
	SetChangesSource(src)
	fx.click(t, fx.rowID("deliv-0"), 1180, 700)

	if !fx.has(msgDiffError) || !fx.has(msgRetry) {
		fx.fail(t, "读失败该显示「"+msgDiffError+"」+「"+msgRetry+"」")
	}
	if !fx.has("two") {
		fx.fail(t, "读失败时也该让人看得到文件本身（全文预览）")
	}
	if src.summaryCalls != 1 {
		t.Fatalf("第一次应当只问一次，实际 %d", src.summaryCalls)
	}
	fx.clickThenSettle(t, fx.rowID("deliv-retry"), 1180, 700)
	if src.summaryCalls != 2 {
		t.Fatalf("点「重试」应当再问一次，实际 %d 次", src.summaryCalls)
	}
}

// TestOverlayCloseStopsDrawing 验「关闭」真的把浮层关掉（否则它会一直挡着侧栏）。
func TestOverlayCloseStopsDrawing(t *testing.T) {
	fx := newRenderFixture(t)
	SetChangesSource(&changesFake{summaryErr: missingErr()})
	fx.click(t, fx.rowID("deliv-0"), 1180, 700)
	if !fx.has(msgNoChangesRecord) {
		fx.fail(t, "浮层没打开")
	}
	fx.clickThenSettle(t, fx.rowID("deliv-close"), 1180, 700)
	if fx.has(msgNoChangesRecord) || fx.has("关闭") {
		fx.fail(t, "点了关闭之后浮层还在")
	}
}
