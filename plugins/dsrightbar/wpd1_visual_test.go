//go:build darwin || windows || linux

package dsrightbar

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestWPD1TwoSizeRealSearchInspectSelectionAnd1000Evidence 使用完整 shell 的真实
// 右栏，在 1182×832 与 1920×1032 各走一遍真实输入、点击、选择、收起和 1000 条搜索。
// 截图只记录已由断言验证过的场景，不代替行为断言。
func TestWPD1TwoSizeRealSearchInspectSelectionAnd1000Evidence(t *testing.T) {
	dir := os.Getenv("WPD1_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	rightbar := &Plugin{}
	kernel := uikernel.New("wpd1-two-size")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dstopbar.Plugin{}, rightbar); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	msgs := wpd1SearchMessages()
	data := pluginapi.Snapshot{
		Cur: wpd1SessionID, ShowRightbar: true, Msgs: msgs, Total: len(msgs),
		Sessions:    []apiclient.Session{{ID: wpd1SessionID, Title: "WP-D1 当前窗口"}},
		MessageMeta: map[string]pubapi.MessageMetadata{},
	}
	data.MessageMeta[pubapi.MessageKey(wpd1SessionID, msgs[1], 1)] = pubapi.MessageMetadata{Time: 1762222445123}
	tree := &gfx.Tree{}
	feat := tree.Feature("wpd1-two-size")
	var view ui.Page
	config := app.Config{
		Title: "wpd1-two-size", Width: 1182, Height: 832, Tree: tree, WindowMode: app.WindowFrameless,
		TextInput: ui.Fields(rightbar.Fields()...),
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		rightPref := size.Width * 0.45
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			MainWidth: size.Width - 280 - rightPref,
			Geometry: pluginapi.Geometry{
				Width: size.Width, Height: size.Height, SidebarWidth: 280,
				RightWidth: rightPref, MainWidth: size.Width - 280 - rightPref, BottomHeight: 24,
			},
			Scroll: view.ScrollState,
			Act:    pluginapi.Actions{ToggleRightbar: func() { data.ShowRightbar = !data.ShowRightbar }},
			Has: func(slot pluginapi.Slot) bool {
				if slot == pluginapi.SlotShellOverlay {
					return false
				}
				return kernel.Has(slot)
			},
		}
		canvas.Fill(bounds, theme.BgBase)
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	capture := func(width, height int, name string) {
		t.Helper()
		path := filepath.Join(dir, name+"-"+strconv.Itoa(width)+"x"+strconv.Itoa(height)+".png")
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
			t.Fatalf("截图像素 = %v，期望 %dx%d", picture.Bounds(), width, height)
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("截图未落盘：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("WPD1_CAPTURE path=%s size=%dx%d bytes=%d", path, width, height, info.Size())
	}
	countTools := func() int {
		count := 0
		for _, text := range session.Snapshot().Texts {
			if strings.HasPrefix(text.Value, "tool-") {
				count++
			}
		}
		return count
	}

	for _, size := range []struct{ width, height int }{{1182, 832}, {1920, 1032}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if err := session.ClickLabel("搜索当前窗口"); err != nil {
			t.Fatalf("%dx%d 搜索框不可点击：%v", size.width, size.height, err)
		}
		if err := session.Type("result-needle"); err != nil {
			t.Fatal(err)
		}
		if !session.HasText("AlphaTool") || session.HasText("BetaTool") {
			t.Fatalf("%dx%d 真实搜索结果错误：%q", size.width, size.height, session.Texts())
		}
		if err := session.ClickLabel("Turn 1 · Step 7 · AlphaTool"); err != nil {
			t.Fatalf("%dx%d Step 展开失败：%v", size.width, size.height, err)
		}
		for _, want := range []string{"Inspect", "call-alpha", "1762222445123", "RESULT-Needle"} {
			if !session.HasText(want) {
				t.Fatalf("%dx%d Inspect 缺 %q：%q", size.width, size.height, want, session.Texts())
			}
		}
		turnKey, stepKey := rightbar.selectionState(wpd1SessionID)
		if turnKey == "" || stepKey == "" {
			t.Fatalf("%dx%d Turn↔Step 选择为空：%q/%q", size.width, size.height, turnKey, stepKey)
		}
		capture(size.width, size.height, "search-inspect")
		if err := session.ClickLabel("Turn 1 · Step 7 · AlphaTool"); err != nil {
			t.Fatal(err)
		}
		if session.HasText("call-alpha") {
			t.Fatalf("%dx%d 单步 Inspect 未收起：%q", size.width, size.height, session.Texts())
		}
		if err := session.ClickLabel("清除轨迹搜索"); err != nil {
			t.Fatal(err)
		}

		data.Msgs = wpd1ThousandMessages()
		data.Total = len(data.Msgs)
		data.MessageMeta = map[string]pubapi.MessageMetadata{}
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if got := countTools(); got == 0 || got >= 80 {
			t.Fatalf("%dx%d 1000 条首屏布局工具行 = %d，不是可见/overscan 窗口", size.width, size.height, got)
		}
		if !session.HasText("1000 次") || !session.HasText("920") {
			t.Fatalf("%dx%d 1000 条截断真值缺失：%q", size.width, size.height, session.Texts())
		}
		if err := session.ClickLabel("搜索当前窗口"); err != nil {
			t.Fatal(err)
		}
		if err := session.Type("TOOL-0000"); err != nil {
			t.Fatal(err)
		}
		if got := countTools(); got != 1 || !session.HasText("tool-0000") {
			t.Fatalf("%dx%d 1000 条全量搜索失败：layout=%d texts=%q", size.width, size.height, got, session.Texts())
		}
		// Viva 聚焦且 PlatformEditing=true 时由原生文本服务画字；无头 PNG 只接管
		// Canvas。点空白处正常 blur，再核对并截取同一个真实查询词。
		if err := session.Click(100, 400); err != nil {
			t.Fatal(err)
		}
		if !session.HasText("TOOL-0000") {
			t.Fatalf("%dx%d 截图前搜索框没有真实显示查询词：%q", size.width, size.height, session.Texts())
		}
		capture(size.width, size.height, "1000-search")
		if err := session.ClickLabel("清除轨迹搜索"); err != nil {
			t.Fatal(err)
		}

		// 下一尺寸从原始小语料开始，避免把上一尺寸的 1000 条状态当作新输入。
		data.Msgs = msgs
		data.Total = len(msgs)
		data.MessageMeta = map[string]pubapi.MessageMetadata{
			pubapi.MessageKey(wpd1SessionID, msgs[1], 1): {Time: 1762222445123},
		}
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
	}
}
