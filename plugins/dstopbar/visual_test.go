package dstopbar

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func drawTopbarVisualTest(t *testing.T, data pluginapi.Snapshot, hasRightbar bool) (*ui.Canvas, *Plugin) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme, prims: &dscore.Primitives{Theme: theme}}
	tree := &ui.Tree{}
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	var view flex.View
	f := view.Begin(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 900, Height: 90})
	p.draw(f, &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("topbar-visual-test"), Canvas: canvas,
		Bounds: ui.Rect{Width: 900, Height: 832}, MainWidth: 900,
		Act: pluginapi.Actions{ToggleRightbar: func() {}},
		Has: func(slot pluginapi.Slot) bool { return slot == pluginapi.SlotRightbar && hasRightbar },
	})
	f.End()
	return canvas, p
}

func topbarTestText(t *testing.T, canvas *ui.Canvas, value string) ui.Text {
	t.Helper()
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text
		}
	}
	t.Fatalf("顶栏缺少文本 %q；实际文本：%+v", value, canvas.Texts())
	return ui.Text{}
}

func topbarHasText(canvas *ui.Canvas, value string) bool {
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
	}, true)
	title := topbarTestText(t, canvas, "真实会话")
	if title.Style.Size != 14 || title.Bounds.Height != 20 {
		t.Errorf("官方当前 crumb 应为 14/20，实际 size=%v line=%v", title.Style.Size, title.Bounds.Height)
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
	if blank.headerBoxes.Divider != (flex.Box{}) {
		t.Errorf("blank 不应声明分割线，实际 %+v", blank.headerBoxes.Divider.Rect())
	}
	t.Logf("V1_GEOMETRY blank root=%+v body=%+v titleRow=%+v corner=%+v",
		blank.headerBoxes.Root.Rect(), blank.headerBoxes.Body.Rect(),
		blank.headerBoxes.TitleRow.Rect(), blank.headerBoxes.Corner.Rect())

	activeData := blankData
	activeData.Msgs = []apiclient.Message{{Role: "user", Text: "hi"}}
	activeData.Total = 1
	_, active := drawTopbarVisualTest(t, activeData, true)
	if got := active.headerBoxes.Root.Rect().Height; got != 50.5 {
		t.Errorf("active 无 tabs Header 高 = %v，官方 50+0.5=50.5", got)
	}
	if got := active.headerBoxes.Divider.Rect().Height; got != 0.5 {
		t.Errorf("active 分割线 = %v，官方 0.5", got)
	}
	t.Logf("V1_GEOMETRY active root=%+v body=%+v titleRow=%+v corner=%+v divider=%+v",
		active.headerBoxes.Root.Rect(), active.headerBoxes.Body.Rect(),
		active.headerBoxes.TitleRow.Rect(), active.headerBoxes.Corner.Rect(), active.headerBoxes.Divider.Rect())

	_, sessionless := drawTopbarVisualTest(t, pluginapi.Snapshot{}, true)
	if got := sessionless.headerBoxes.Root.Rect().Height; got != 10 {
		t.Errorf("sessionless Header 高 = %v，官方 10", got)
	}
	t.Logf("V1_GEOMETRY sessionless root=%+v body=%+v",
		sessionless.headerBoxes.Root.Rect(), sessionless.headerBoxes.Body.Rect())
}

func topbarHasRightbarControl(canvas *ui.Canvas, label string) bool {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return true
		}
	}
	return false
}
