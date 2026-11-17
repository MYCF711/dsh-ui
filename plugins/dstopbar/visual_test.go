package dstopbar

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar/viewapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func drawTopbarVisualTest(t *testing.T, data pluginapi.Snapshot, hasRightbar bool) (*gfx.Canvas, *Plugin) {
	t.Helper()
	return drawTopbarVisualTestViews(t, data, hasRightbar, nil)
}

func drawTopbarVisualTestViews(t *testing.T, data pluginapi.Snapshot, hasRightbar bool, views []pluginapi.Tab) (*gfx.Canvas, *Plugin) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &gfx.Tree{}
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	// v0.6.0 没有 Begin/End：Draw 内部就完成声明与解算，主题构造期给。
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, gfx.Rect{Width: 900, Height: 90}, func(f ui.C) {
		p.draw(f, &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("topbar-visual-test"), Canvas: canvas,
			Bounds: gfx.Rect{Width: 900, Height: 832}, MainWidth: 900, Views: views, ActiveView: viewapi.Chat,
			Act: pluginapi.Actions{ToggleRightbar: func() {}},
			Has: func(slot pluginapi.Slot) bool { return slot == pluginapi.SlotRightbar && hasRightbar },
		})
	})
	return canvas, p
}

func topbarTestText(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("顶栏缺少文本 %q；实际文本：%+v", value, canvas.Texts())
	return gfx.Text{}
}

func topbarHasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func TestBlankHeaderHidesSessionChrome(t *testing.T) {
	canvas, _ := drawTopbarVisualTest(t, pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "blank session", Cwd: `D:\\work\\dsh-ui`}},
		Model: "real-model", Permission: "ask", Total: 0,
	}, true)

	for _, hidden := range []string{"blank session", "未选择会话", "real-model · 审批 ask", "共 0 条"} {
		if topbarHasText(canvas, hidden) {
			t.Errorf("官方 blank Header 不应显示 %q", hidden)
		}
	}
	if !topbarHasRightbarControl(canvas, "打开右侧边栏") {
		t.Error("blank Header 应保留有真实 action 的右侧栏展开入口")
	}
}

func TestBlankHeaderOmitsToggleWithoutRightbarSlot(t *testing.T) {
	canvas, _ := drawTopbarVisualTest(t, pluginapi.Snapshot{}, false)
	if topbarHasRightbarControl(canvas, "打开右侧边栏") || topbarHasText(canvas, "轨迹 关") {
		t.Error("右栏 slot 不存在时不得留下无实际效果的开关")
	}
}

func TestActiveHeaderUsesOfficialCrumbTypography(t *testing.T) {
	canvas, _ := drawTopbarVisualTest(t, pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "真实会话"}},
		Msgs: []apiclient.Message{{Role: "user", Text: "hi"}}, Total: 1,
		Model: "real-model", Permission: "ask",
	}, true)
	title := topbarTestText(t, canvas, "真实会话")
	if title.Style.Size != 14 || title.Bounds.Height != 20 || title.Style.Weight != gfx.WeightMedium {
		t.Errorf("官方当前 crumb 应为 14/20/500，实际 size=%v line=%v weight=%v",
			title.Style.Size, title.Bounds.Height, title.Style.Weight)
	}
	for _, legacy := range []string{"共 1 条", "real-model · 审批 ask"} {
		if topbarHasText(canvas, legacy) {
			t.Errorf("active Header 不应重复 composer/旧占位信息 %q", legacy)
		}
	}
	for _, tab := range []string{"对话", "轨迹"} {
		if !topbarHasRightbarControl(canvas, tab) {
			t.Errorf("active Header 缺少可点击的 %q 页签", tab)
		}
	}
	if topbarHasText(canvas, "主题鱼") || topbarHasText(canvas, "团队") || topbarHasText(canvas, "子智能体") {
		t.Error("没有扩展/团队/子智能体真值时不得画对应标签")
	}
}

func TestViewTabsUseOfficialTypographyAndSelection(t *testing.T) {
	canvas, _ := drawTopbarVisualTest(t, pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "真实会话"}},
		Msgs: []apiclient.Message{{Role: "user", Text: "hi"}}, Total: 1,
	}, true)
	theme := dscore.Dark()
	for _, label := range []string{"对话", "轨迹"} {
		text := topbarTestText(t, canvas, label)
		if text.Style.Size != 13 || text.Bounds.Height != 16 || text.Style.Weight != gfx.WeightMedium {
			t.Errorf("%s 应为官方 13/16/500，实际 size=%v line=%v weight=%v",
				label, text.Style.Size, text.Bounds.Height, text.Style.Weight)
		}
		wantColor := theme.LabelTertiary
		if label == "对话" {
			wantColor = theme.Info
		}
		if text.Style.Color != wantColor {
			t.Errorf("%s 颜色 = %+v，官方选中 business-primary / 未选中 label-tertiary = %+v",
				label, text.Style.Color, wantColor)
		}
	}
}

func TestViewTabLabelUsesOfficial88pxLimit(t *testing.T) {
	const longLabel = "统计统计统计统计统计统计统计"
	canvas, _ := drawTopbarVisualTestViews(t, pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "真实会话"}},
		Msgs: []apiclient.Message{{Role: "user", Text: "hi"}}, Total: 1,
	}, true, []pluginapi.Tab{
		{Key: viewapi.Chat, Label: "对话"}, {Key: pluginapi.ViewStats, Label: longLabel, Order: 40},
	})
	var truncated gfx.Text
	found := false
	for _, text := range canvas.Texts() {
		if strings.HasPrefix(text.Value, "统计") {
			truncated, found = text, true
			break
		}
	}
	if !found {
		t.Fatalf("长 view label 未绘制：%+v", canvas.Texts())
	}
	if got := truncated.Bounds.Width; got != 88 {
		t.Errorf("截断后的长 view label 宽 = %v，官方 .tabLabel max-width:88", got)
	}
}

func TestRightbarIconUsesOfficialPanelLeftMirror(t *testing.T) {
	icon, ok := rightbarPanelIcon()
	if !ok {
		t.Fatal("官方 panel-left 两条 path 应可解析")
	}
	if icon.ViewBox != 16 || icon.Stroke != 1 {
		t.Fatalf("右栏图标 viewBox/stroke = %v/%v，官方 16/1", icon.ViewBox, icon.Stroke)
	}
	if len(icon.Commands) < 2 {
		t.Fatalf("右栏图标命令不足：%+v", icon.Commands)
	}
	move, line := icon.Commands[len(icon.Commands)-2], icon.Commands[len(icon.Commands)-1]
	if move.Kind != gfx.PathCommandMove || move.Point.X != 10.5 || move.Point.Y != 1.5 ||
		line.Kind != gfx.PathCommandLine || line.Point.X != 10.5 || line.Point.Y != 14.5 {
		t.Errorf("官方 scaleX(-1) 后分隔线应为 (10.5,1.5)→(10.5,14.5)，实际 %+v %+v", move, line)
	}
}

func TestHeaderGeometryMatchesOfficialBlankActiveAndSessionless(t *testing.T) {
	blankData := pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "blank"}},
	}
	_, blank := drawTopbarVisualTest(t, blankData, true)
	if got := blank.headerBoxes.Root.Rect().Height; got != 40 {
		t.Errorf("blank Header 高 = %v，官方 10+30=40", got)
	}
	if got := blank.headerBoxes.TitleRow.Rect().Height; got != 30 {
		t.Errorf("blank titleRow 高 = %v，官方 30", got)
	}
	if got := blank.headerBoxes.Corner.Rect(); got.Width != 28 || got.Height != 28 {
		t.Errorf("corner 几何 = %+v，官方 28×28", got)
	}
	if blank.headerBoxes.Divider != (ui.Box{}) {
		t.Errorf("blank 不应声明分割线，实际 %+v", blank.headerBoxes.Divider.Rect())
	}
	t.Logf("V1_GEOMETRY blank root=%+v body=%+v titleRow=%+v corner=%+v",
		blank.headerBoxes.Root.Rect(), blank.headerBoxes.Body.Rect(),
		blank.headerBoxes.TitleRow.Rect(), blank.headerBoxes.Corner.Rect())

	activeData := blankData
	activeData.Msgs = []apiclient.Message{{Role: "user", Text: "hi"}}
	activeData.Total = 1
	activeCanvas, active := drawTopbarVisualTest(t, activeData, true)
	if got := active.headerBoxes.Body.Rect().Height; got != 76 {
		t.Errorf("active Header 高 = %v，官方 76", got)
	}
	if got := active.headerBoxes.Root.Rect().Height; got != 76.5 {
		t.Errorf("active Header+分割线高 = %v，官方 76+0.5=76.5", got)
	}
	if got := active.headerBoxes.TitleRow.Rect().Height; got != 30 {
		t.Errorf("active titleRow 高 = %v，官方 30", got)
	}
	if got := active.headerBoxes.Divider.Rect().Height; got != 0.5 {
		t.Errorf("active 分割线 = %v，官方 0.5", got)
	}
	chatHit := topbarTestHit(t, activeCanvas, "对话")
	trajectoryHit := topbarTestHit(t, activeCanvas, "轨迹")
	for label, hit := range map[string]gfx.Hit{"对话": chatHit, "轨迹": trajectoryHit} {
		if hit.Bounds.Height != 35 {
			t.Errorf("%s 页签命中高 = %v，官方 35（负 margin 后布局行仍为 25）", label, hit.Bounds.Height)
		}
		if got := active.headerBoxes.Tabs.Rect().Y - hit.Bounds.Y; got != 10 {
			t.Errorf("%s 页签 margin-top = %v，官方 -10", label, -got)
		}
		text := topbarTestText(t, activeCanvas, label)
		if got := text.Bounds.X - hit.Bounds.X; got != 10 {
			t.Errorf("%s 左侧命中 padding = %v，官方 10", label, got)
		}
	}
	if got := trajectoryHit.Bounds.X - (chatHit.Bounds.X + chatHit.Bounds.Width); got != 32 {
		t.Errorf("viewTabs 净间距 = %v，官方 32", got)
	}
	if got := active.headerBoxes.Tabs.Rect().Height; got != 25 {
		t.Errorf("viewTabs 布局行高 = %v，官方 25", got)
	}
	t.Logf("FG3_GEOMETRY active root=%+v body=%+v titleRow=%+v corner=%+v divider=%+v",
		active.headerBoxes.Root.Rect(), active.headerBoxes.Body.Rect(),
		active.headerBoxes.TitleRow.Rect(), active.headerBoxes.Corner.Rect(), active.headerBoxes.Divider.Rect())

	_, sessionless := drawTopbarVisualTest(t, pluginapi.Snapshot{}, true)
	if got := sessionless.headerBoxes.Root.Rect().Height; got != 10 {
		t.Errorf("sessionless Header 高 = %v，官方 10", got)
	}
	t.Logf("V1_GEOMETRY sessionless root=%+v body=%+v",
		sessionless.headerBoxes.Root.Rect(), sessionless.headerBoxes.Body.Rect())
}

func TestCornerOnlyExpandsCollapsedRightbar(t *testing.T) {
	canvas, _ := drawTopbarVisualTest(t, pluginapi.Snapshot{
		Cur: "s1", Sessions: []apiclient.Session{{ID: "s1", Title: "active"}},
		Msgs: []apiclient.Message{{Role: "user", Text: "hi"}}, ShowRightbar: true,
	}, true)
	if topbarHasRightbarControl(canvas, "打开右侧边栏") || topbarHasRightbarControl(canvas, "收起右侧边栏") {
		t.Error("官方 ExpandButton 只在右栏收起时出现；展开态应由右栏自己的 PanelChrome 收起")
	}
}

func topbarTestHit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit
		}
	}
	t.Fatalf("顶栏缺少命中区 %q；实际命中：%+v", label, canvas.Hits())
	return gfx.Hit{}
}

func topbarHasRightbarControl(canvas *gfx.Canvas, label string) bool {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return true
		}
	}
	return false
}
