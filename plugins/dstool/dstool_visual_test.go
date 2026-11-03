//go:build darwin || windows || linux

package dstool

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// 这个文件是「按工具类型定制渲染」的**像素证据**：
// 同一个渲染器、同一帧里并排画 bash / read / glob / grep / 未知工具，
// 每种展开态各是什么样。截图见 tmp/tool-glob.png（TOOL_CARD_CAPTURE_DIR）。
//
// 为什么不在这里画消息流：那属于 ds-conversation，它自己有前后对比
// （plugins/dsconversation/toolcard_capture_test.go）。这里只管卡本身。

// visualCards 是截图里的六张卡，覆盖两种定制渲染 + 默认回落。
func visualCards() []Card {
	return []Card{
		{Seq: 1, Tool: "bash", Args: `{"command":"CGO_ENABLED=0 go test ./..."}`,
			Summary: "exit 0 · 12 passed · 用了 2.3 s"},
		{Seq: 2, Tool: "bash", Args: `{"command":"gofmt -l plugins/dstool"}`,
			Summary: "exit 1 · 1 file needs formatting"},
		{Seq: 3, Tool: "read", Args: `{"filePath":"plugins/dstool/dstool.go","offset":1,"limit":40}`},
		{Seq: 4, Tool: "glob", Args: `{"pattern":"**/*.go","path":"plugins"}`},
		{Seq: 5, Tool: "grep", Args: `{"pattern":"renderers","path":"plugins"}`, Summary: "3 处命中"},
		{Seq: 6, Tool: "webfetch", Args: `{"url":"https://example.com"}`},
	}
}

// TestToolCardCustomRenderings 在无头窗口里真画一遍，断言"每条走的是它的定制画法"，
// 并把整帧存成 PNG。TOOL_CARD_CAPTURE_DIR 非空时存到那里。
func TestToolCardCustomRenderings(t *testing.T) {
	dir := os.Getenv("TOOL_CARD_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	theme := dscore.Dark()
	cards := New(&dscore.Primitives{Theme: theme})
	data := visualCards()

	tree := &ui.Tree{}
	feat := tree.Feature("dstool-visual")
	var view flex.View
	config := app.Config{
		Title: "dstool-visual", Width: 900, Height: 720, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			f.Column().WidthPercent(100).Pad(16).Gap(6).Children(func() {
				for _, card := range data {
					cards.Render(f, env, card)
				}
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	// 每条都点开：定制画法在展开体里（收起时六条长得一样，那是消息流的密度考虑）。
	for _, card := range data {
		id := tree.ID(feat, "tool-"+strconv.Itoa(card.Seq))
		hit, ok := hitByID(session.Snapshot(), id)
		if !ok {
			t.Fatalf("seq=%d 的标题行没有命中区；hits=%+v", card.Seq, session.Snapshot().Hits)
		}
		x := hit.Bounds.X + hit.Bounds.Width/2
		y := hit.Bounds.Y + hit.Bounds.Height/2
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
			t.Fatalf("点了 seq=%d 之后没有展开", card.Seq)
		}
	}

	snapshot := session.Snapshot()
	texts := map[string]ui.Text{}
	for _, text := range snapshot.Texts {
		texts[text.Value] = text
	}
	require := func(value string) ui.Text {
		t.Helper()
		text, ok := texts[value]
		if !ok {
			t.Fatalf("画面上没有文本 %q；实际文本：%q", value, session.Texts())
		}
		return text
	}

	// bash：命令 + 退出码 + 输出预览。退出码 0 绿、非 0 红——这是"看得见"的一半。
	require("CGO_ENABLED=0 go test ./...")
	if got := require("exit 0").Style.Color; got != theme.Success {
		t.Fatalf("exit 0 的徽标颜色 = %v，期望 Success %v", got, theme.Success)
	}
	require("12 passed · 用了 2.3 s")
	if got := require("exit 1").Style.Color; got != theme.Error {
		t.Fatalf("exit 1 的徽标颜色 = %v，期望 Error %v", got, theme.Error)
	}

	// read：路径 + 行范围（宿主没给 Summary，标题行也要能显示路径）。
	if got := require("路径").Style.Color; got != theme.LabelCaption {
		t.Fatalf("字段标签颜色 = %v，期望 LabelCaption %v", got, theme.LabelCaption)
	}
	require("plugins/dstool/dstool.go")
	require("起始行")
	require("行数")

	// glob：模式 + 范围。
	require("模式")
	require("**/*.go")
	// grep：模式 + 路径 + 宿主给的摘要。
	require("3 处命中")

	// 未知工具：走默认画法（参数原文 + 摘要），不因为它不认识就少画。
	require(`{"url":"https://example.com"}`)

	path := filepath.Join(dir, "tool-glob.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图为空 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("TOOL_CARD_CAPTURE custom=%s size=%dx%d bytes=%d", path, snapshot.Width, snapshot.Height, info.Size())
}

// hitByID 按交互 id 找命中区（工具标题行的标签是空串，唯一身份是 id）。
func hitByID(snapshot app.Snapshot, id ui.ID) (ui.Hit, bool) {
	for _, hit := range snapshot.Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return ui.Hit{}, false
}
