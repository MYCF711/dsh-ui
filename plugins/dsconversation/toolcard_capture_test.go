//go:build darwin || windows || linux

package dsconversation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// toolCardData 是工具卡抽取前后**同一份**输入：一条用户消息、一条 bash、
// 一条 read、一条助手回复。bash/read 两条都带宿主给的 Summary，
// 这正是 `-demo` 造出来的样子（cmd/app/main.go 的 demoTool 分支）。
//
// 为什么这里不掺 glob / 空 Summary 的用例：那两条要验的是**新的**
// 按工具类型定制的头部派生，抽取前根本不存在，掺进来就不是"前后对比"了。
// 它们是 plugins/dstool 自己的截图（见 dstool/toolcard_visual_test.go）。
func toolCardData() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur: "s-tool",
		Msgs: []apiclient.Message{
			{Seq: 1, Role: "user", Text: "先看下工作区状态，再把 mux.go 读一遍。"},
			{Seq: 2, Role: "tool", Tool: "bash",
				Args:    "git status --short",
				Summary: "exit 0 · 3 files changed · 用了 12 ms"},
			{Seq: 3, Role: "tool", Tool: "read",
				Args:    "internal/apiclient/mux.go",
				Summary: "1.2 KB · 78 行 · 读完了"},
			{Seq: 4, Role: "assistant", Text: "工作区是干净的，`mux.go` 一共 78 行。"},
		},
	}
}

// captureToolCards 用真实 flex 布局把消息流画进无头窗口并存 PNG。
//
// 走的是 p.draw + p.afterDraw 两个真阶段（Markdown 覆画也在里面），
// 所以像素是"真画出来的"，不是把数据拼进模板。
//
// expand 里的每条工具卡都是**点出来的**（真的 Press/Release 在标题行上），
// 不是直接改内部状态：展开/收起属于工具卡自己的交互，抽取前后都得能点开。
func captureToolCards(t *testing.T, dir, name string, expand []int) string {
	t.Helper()
	theme := dscore.Dark()
	prims := &dscore.Primitives{Theme: theme}
	p := &Plugin{theme: theme, prims: prims}
	// 工具卡由 ds-tool 提供（内核装配时走 ctx.Inject(dstool.ServiceKey)）。
	// 这里直接用同一个渲染器，和内核给的是同一份代码。
	p.tools = dstool.New(prims)

	tree := &gfx.Tree{}
	feat := tree.Feature("tool-card")
	data := toolCardData()

	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "tool-card", Width: 900, Height: 700, Tree: tree,
		WindowMode: app.WindowFrameless,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, bounds, func(f ui.C) {
			p.draw(f, env)
		})
		p.afterDraw(canvas, env)
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	for _, seq := range expand {
		// dstool 的 int 只是内部局部键；测试仍按输入消息的 v4 seq 选人，
		// 再走与产品相同的 MessageKey 适配，不能假定交互 id 仍是裸 seq。
		renderID := 0
		for i, msg := range data.Msgs {
			if msg.Role == "tool" && msg.Seq == seq {
				renderID = p.toolRenderID(messageRenderKey(&pluginapi.Env{Data: data}, msg, i))
				break
			}
		}
		if renderID == 0 {
			t.Fatalf("测试输入没有工具消息 seq=%d", seq)
		}
		hit, ok := findToolHit(session.Snapshot(), tree.ID(feat, "tool-"+itoa(renderID)))
		if !ok {
			t.Fatalf("没有工具消息 seq=%d 的标题行命中区；hits=%+v", seq, session.Snapshot().Hits)
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
		if !p.toolCards().Open(renderID) {
			t.Fatalf("点了 seq=%d 的标题行，工具卡没有展开", seq)
		}
	}
	path := filepath.Join(dir, name)
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("截图为空 path=%s info=%v err=%v", path, info, err)
	}
	return path
}

// findToolHit 按交互 id 找命中区。
//
// 为什么不按标签找：工具标题行的标签是空串（`Hit(id, "")`），它没有可见文字，
// 唯一的身份就是那个 id。ui.Tree 的名字是内部化（intern）的，所以在这里
// 用同样的 (scope, word) 算出来的 ID 与绘制期那个**逐位相同**。
func findToolHit(snapshot app.Snapshot, id gfx.ID) (gfx.Hit, bool) {
	for _, hit := range snapshot.Hits {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// TestToolCardCapture 留 before/after 像素证据。
// TOOL_CARD_CAPTURE_DIR 非空时存到那里，否则仍在临时目录实画。
func TestToolCardCapture(t *testing.T) {
	dir := os.Getenv("TOOL_CARD_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := os.Getenv("TOOL_CARD_CAPTURE_NAME")
	collapsedName, expandedName := "tool-before.png", "tool-before-expanded.png"
	if name != "" {
		collapsedName, expandedName = name+".png", name+"-expanded.png"
	}
	collapsed := captureToolCards(t, dir, collapsedName, nil)
	expanded := captureToolCards(t, dir, expandedName, []int{2, 3})
	t.Logf("TOOL_CARD_CAPTURE collapsed=%s expanded=%s", collapsed, expanded)
}
