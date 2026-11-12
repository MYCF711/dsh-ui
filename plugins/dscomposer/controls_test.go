package dscomposer

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func composerText(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

func composerHit(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func trianglesInside(canvas *gfx.Canvas, bounds ui.Rect, color ui.Color) int {
	count := 0
	for _, triangle := range canvas.Triangles() {
		if triangle.Color != color {
			continue
		}
		inside := true
		for _, point := range triangle.Points {
			if point.X < bounds.X-0.01 || point.Y < bounds.Y-0.01 ||
				point.X > bounds.X+bounds.Width+0.01 || point.Y > bounds.Y+bounds.Height+0.01 {
				inside = false
				break
			}
		}
		if inside {
			count++
		}
	}
	return count
}

func TestPermissionTruthUsesChineseProductAndPolicyLabels(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{value: "read-only", want: "仅可查看"},
		{value: "workspace-write", want: "工作区内修改"},
		{value: "danger-full-access", want: "完全权限"},
		{value: "allow", want: "完全权限"},
		{value: "ask", want: "逐次询问"},
		{value: "deny", want: "全部拒绝"},
		{value: "auto", want: "自动审批"},
		{value: "host-custom-policy", want: "host-custom-policy"},
		{value: "  ", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			_, got := hostLabels(pluginapi.Snapshot{Permission: tc.value})
			if got != tc.want {
				t.Fatalf("permission %q 显示为 %q，want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestKnownPermissionHasVectorIconBesideLabel(t *testing.T) {
	for _, tc := range []struct {
		value string
		label string
	}{
		{value: "read-only", label: "仅可查看"},
		{value: "workspace-write", label: "工作区内修改"},
		{value: "danger-full-access", label: "完全权限"},
		{value: "allow", label: "完全权限"},
		{value: "ask", label: "逐次询问"},
		{value: "deny", label: "全部拒绝"},
		{value: "auto", label: "自动审批"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			p := &Plugin{}
			canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Permission: tc.value}, pluginapi.Actions{}, 902, 1182, "")
			label, ok := composerText(canvas, tc.label)
			if !ok {
				t.Fatalf("已知权限 %q 应显示中文真值 %q", tc.value, tc.label)
			}
			// 官方 triggerIcon 为 14×14，位于 label 左侧、gap 4；这里只量这枚图标的座位，
			// 避免发送键的矢量三角形误满足权限图标断言。
			iconBounds := ui.Rect{X: label.Bounds.X - 18, Y: label.Bounds.Y + label.Bounds.Height/2 - 8, Width: 16, Height: 16}
			if got := trianglesInside(canvas, iconBounds, p.theme.LabelSecondary); got == 0 {
				t.Fatalf("权限 label 左侧没有 14px 矢量图标；label=%+v triangles=%d", label.Bounds, len(canvas.Triangles()))
			}
		})
	}
}

func TestModelReadOnlyHasNoDropdownSemantics(t *testing.T) {
	p := &Plugin{}
	canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Model: "deepseek-v4-pro", Permission: "ask"}, pluginapi.Actions{}, 902, 1182, "")
	for _, hit := range canvas.Hits() {
		if hit.Expandable || hit.Expanded || hit.Label == "选择模型" || hit.Label == "deepseek-v4-pro" || hit.Label == "逐次询问" || hit.Label == "命令" || hit.Label == "附件" {
			t.Fatalf("无 catalog/mutation 时出现假选择或假命令命中区：%+v", hit)
		}
	}
	for _, text := range canvas.Texts() {
		if text.Value == "⌄" || text.Value == "▾" || text.Value == "▼" || text.Value == "+" {
			t.Fatalf("无 Action 时不得用文字 glyph 伪造下拉/命令：%q", text.Value)
		}
	}
}

func TestModelTriggerUsesOfficialWiderMaxWidth(t *testing.T) {
	p := &Plugin{}
	model := strings.Repeat("very-long-model-", 40)
	canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Model: model}, pluginapi.Actions{}, 1640, 1920, "")
	var found bool
	for _, text := range canvas.Texts() {
		if !strings.HasPrefix(text.Value, "very-long-model-") {
			continue
		}
		found = true
		// 952px 卡片上官方 max-width=min(360,45cqw)=360；扣水平 padding 后
		// 文本预算明显大于旧实现误用 PermissionSelect 的 204px。
		if text.Bounds.Width <= 220 {
			t.Errorf("模型仍被权限的 220px 上限压住：文本宽 %.2f，官方模型 trigger 应取 360px 档", text.Bounds.Width)
		}
		if text.Bounds.Height != 20 || text.Style.Size != 13 || text.Style.Weight != gfx.WeightRegular {
			t.Errorf("模型 trigger 文本应为 13/20/400，实际 %+v %+v", text.Bounds, text.Style)
		}
	}
	if !found {
		t.Fatal("找不到模型真值文本")
	}
	if chipHeight != 28 || sendSize != 34 {
		t.Fatalf("控件几何回归：trigger=%v primary=%v，官方 28/34", chipHeight, sendSize)
	}
}

func TestPrimaryButtonsUseOfficialVectorsAnd34pxHitArea(t *testing.T) {
	for _, tc := range []struct {
		name    string
		running bool
		label   string
		banned  string
	}{
		{name: "发送", label: "发送", banned: "↑"},
		{name: "停止", running: true, label: "停止", banned: "■"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			p.field.SetText("draft")
			act := pluginapi.Actions{Send: func(string) {}, Cancel: func() {}}
			canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Busy: tc.running}, act, 902, 1182, "")
			primary, ok := largestSurface(canvas, p.theme.Info)
			if !ok || primary.Bounds.Width != 34 || primary.Bounds.Height != 34 {
				t.Fatalf("主按钮应为 34×34 Info 圆钮，实际 %+v found=%v", primary.Bounds, ok)
			}
			if _, found := composerText(canvas, tc.banned); found {
				t.Fatalf("仍在依赖字体 glyph %q，必须改官方 16 viewBox 矢量", tc.banned)
			}
			if got := trianglesInside(canvas, primary.Bounds, onInfo(p.theme)); got == 0 {
				t.Fatalf("34px 主按钮内没有官方矢量三角形")
			}
			hit, found := composerHit(canvas, tc.label)
			if !found || hit.Bounds.Width != 34 || hit.Bounds.Height != 34 || hit.Disabled {
				t.Fatalf("原 %s ID/34px 命中区必须保留且可用：%+v found=%v", tc.label, hit, found)
			}
			// 官方 onMouseDown=keepDraftFocus：按钮动作存在，但鼠标按下不把编辑器/IME
			// 焦点拿走。viva 用 SkipFocus 表达同一路由；Enter/Esc 仍由输入区负责。
			if hit.Focusable {
				t.Errorf("%s 按钮会参与焦点导航，可能抢输入框/IME 焦点；应 SkipFocus", tc.label)
			}
		})
	}
}

func TestPrimaryDisabledTruthAndDraftSafety(t *testing.T) {
	t.Run("空草稿禁发", func(t *testing.T) {
		p := &Plugin{}
		sent := 0
		canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001"}, pluginapi.Actions{Send: func(string) { sent++ }}, 902, 1182, "send")
		hit, ok := composerHit(canvas, "发送")
		if !ok || !hit.Disabled {
			t.Fatalf("空草稿必须呈现官方 disabled 语义：%+v found=%v", hit, ok)
		}
		if sent != 0 {
			t.Fatalf("空草稿发送了 %d 次", sent)
		}
	})

	t.Run("无Send保留草稿", func(t *testing.T) {
		p := &Plugin{}
		p.field.SetText("不能丢的草稿")
		canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001"}, pluginapi.Actions{}, 902, 1182, "send")
		hit, ok := composerHit(canvas, "发送")
		if !ok || !hit.Disabled {
			t.Fatalf("Send=nil 必须 disabled，不能是假可点：%+v found=%v", hit, ok)
		}
		if got := p.field.Text(); got != "不能丢的草稿" {
			t.Fatalf("Send=nil 时草稿被清空为 %q", got)
		}
	})

	t.Run("无Cancel停止禁用", func(t *testing.T) {
		p := &Plugin{}
		canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-001", Busy: true}, pluginapi.Actions{}, 902, 1182, "stop")
		hit, ok := composerHit(canvas, "停止")
		if !ok || !hit.Disabled {
			t.Fatalf("Cancel=nil 时停止必须 disabled：%+v found=%v", hit, ok)
		}
	})
}
