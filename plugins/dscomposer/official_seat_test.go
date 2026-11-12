//go:build darwin || windows || linux

package dscomposer

// official_seat_test.go —— 官方 `conversation.composer` 座位的**输入卡片**断言。
//
// GP 报告 §5 第 2 条的缺口：官方座位只画审批卡、不画输入卡片。本文件把
// 「输入卡片真的画在官方座位上、每个元素都在官方几何位置」变成机器断言：
//
//	① 元素：输入框（占位「说点什么…（Enter 发送）」）/ 发送·停止 / 权限徽标 /
//	   上下文计数 / 附件（文件引用）入口；
//	② 两态：空输入禁用、发送中变停止；
//	③ 未知：宿主没给上下文容量时显示「未知」，不编 0、不算百分比；
//	④ 相对位置：审批卡在输入卡片**上方**（官方 composer 接管，不是消息流里的一段）；
//	⑤ 行为：Enter 由输入框提交（HandleKey 不接管），Shift+Enter 被吃掉（不发送）。
//
// 这些断言都用**真 flex 布局**（不是 mock 框）：几何值与官方 CSS 的对应写在
// `internal/uikernel/official_geometry.go` 的常量表里，本文件只核对结果。

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// officialSeatColumn 是官方座位在 1182 宽窗口下的主列宽（与几何表一致）。
const officialSeatColumn = 902

// renderOfficialSeat 用真 flex 画一次官方座位。
//
// 容器模拟 `uikernel.drawComposerSeat` 给占用方的形状：左右各 clearance(16)、
// 上下无内边距（底边距归 InputBar.root 自己）。click 非空时走事件 pass，
// 测到的是按钮真实的 Clicked 分支。
func renderOfficialSeat(t *testing.T, p *Plugin, data pluginapi.Snapshot, act pluginapi.Actions, click string) *gfx.Canvas {
	t.Helper()
	if p.theme == nil {
		p.theme = dscore.Dark()
	}
	if p.prims == nil {
		p.prims = &dscore.Primitives{Theme: p.theme}
	}
	tree := &gfx.Tree{}
	feat := tree.Feature("official-seat-test")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size * 0.55
	})
	if click != "" {
		canvas.BeginEventPass(tree.ID(feat, click))
		defer canvas.EndEventPass()
	}
	env := &pluginapi.Env{
		Theme: p.theme, Data: data, Act: act, Tree: tree, Feat: feat,
		Canvas: canvas, Bounds: ui.Rect{Width: 1182, Height: 832},
		MainWidth: officialSeatColumn,
		Geometry:  pluginapi.Geometry{Width: 1182, Height: 832, SidebarWidth: 280, MainWidth: officialSeatColumn},
	}
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(p.theme), env.Bounds, func(f *flex.Frame) {
		f.Column().Width(officialSeatColumn - 2*composerClearance).
			PadX(composerClearance).Children(func() {
			p.drawOfficialComposer(f, env)
		})
	})
	p.afterDraw(canvas, env)
	return canvas
}

// fileSource 造一份"壳接了文件引用"的动作集（附件入口的唯一真实来源）。
func fileSource() pluginapi.Actions {
	return pluginapi.Actions{
		Send:   func(string) {},
		Cancel: func() {},
		FSList: func(string) []pluginapi.FSEntry {
			return []pluginapi.FSEntry{{Name: "main.go"}}
		},
	}
}

// TestOfficialSeatRendersInputCardElements ①：输入卡片每个元素都在。
func TestOfficialSeatRendersInputCardElements(t *testing.T) {
	p := &Plugin{}
	data := pluginapi.Snapshot{
		Cur: "s-1", CWD: "D:/work",
		Model: "deepseek-v4-pro", Permission: "ask",
		ContextTokens: 12345, ContextWindow: 200000,
	}
	canvas := renderOfficialSeat(t, p, data, fileSource(), "")

	for _, want := range []string{
		placeholderText,   // 输入框占位（官方文案）
		"逐次询问",            // 权限徽标（宿主 policy=ask 的真值）
		"deepseek-v4-pro", // 模型徽标（右侧 trailing）
		"上下文",             // 上下文计数（官方 ContextMeter 位置）
		"12k / 200k（6%）",  // 计数真值：12.3k 取整到 k 档 → 12k
	} {
		if _, ok := composerText(canvas, want); !ok {
			t.Fatalf("官方座位没画 %q：%v", want, canvasTextValues(canvas))
		}
	}
	// 输入框：真 flex.Field 的命中区（role=text field）。
	var field gfx.Hit
	found := false
	for _, hit := range canvas.Hits() {
		if hit.Label == "输入" {
			field, found = hit, true
		}
	}
	if !found {
		t.Fatalf("官方座位没有输入框命中区：%+v", canvas.Hits())
	}
	// 输入区：官方 `.input{min-height:36}`；viva 的 Field 是 32（`fieldHeight`
	// 的注释写了为什么：36 会让文字带偏 2px）。36 由卡片高 98 = 8+36+12+42 兜住。
	if field.Bounds.Height != fieldHeight {
		t.Fatalf("输入框高 = %v，实现常量 fieldHeight=%v（官方 .input 36 的文字带等价）",
			field.Bounds.Height, fieldHeight)
	}
	// 发送键：34×34、空草稿时禁用（官方 disabled 语义）。
	send, ok := composerHit(canvas, "发送")
	if !ok {
		t.Fatalf("官方座位没画发送键：%v", canvasTextValues(canvas))
	}
	if send.Bounds.Width != 34 || send.Bounds.Height != 34 {
		t.Fatalf("发送键 = %v×%v，官方 34×34", send.Bounds.Width, send.Bounds.Height)
	}
	if !send.Disabled {
		t.Fatal("空草稿时发送键必须 disabled（官方 .primary:disabled）")
	}
	// 附件入口：28×28 圆形，只在壳接了文件引用时出现。
	add, ok := composerHit(canvas, "添加文件引用")
	if !ok {
		t.Fatalf("官方座位没画附件/文件引用入口：%+v", canvas.Hits())
	}
	if add.Bounds.Width != 28 || add.Bounds.Height != 28 {
		t.Fatalf("附件入口 = %v×%v，官方 .add 28×28", add.Bounds.Width, add.Bounds.Height)
	}
	// 卡片本体：InputMajor 圆角面板、宽 = 官方卡片宽、圆角 28。
	card, ok := inputCardSurface(canvas, p.theme.InputMajor)
	if !ok {
		t.Fatal("官方座位没画输入卡片面板")
	}
	if card.Bounds.Width != cardWidth(officialSeatColumn) {
		t.Fatalf("卡片宽 = %v，官方 %v", card.Bounds.Width, cardWidth(officialSeatColumn))
	}
	if card.Radius != 28 {
		t.Fatalf("卡片圆角 = %v，官方 radius-panel 28", card.Radius)
	}
	// 卡片高 = 8 + 36 + 12 + 42（控件行被 34px 主钮撑开）。
	if card.Bounds.Height != 98 {
		t.Fatalf("卡片高 = %v，官方 8+36+12+42 = 98", card.Bounds.Height)
	}
}

// TestOfficialSeatContextUnknown ③：宿主没给上下文容量 → 显示「未知」。
func TestOfficialSeatContextUnknown(t *testing.T) {
	p := &Plugin{}
	canvas := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1"}, pluginapi.Actions{Send: func(string) {}}, "")
	if _, ok := composerText(canvas, "上下文"); !ok {
		t.Fatalf("官方座位没有上下文计数：%v", canvasTextValues(canvas))
	}
	if _, ok := composerText(canvas, unknownField); !ok {
		t.Fatalf("宿主没给 ContextWindow 时应显示 %q：%v", unknownField, canvasTextValues(canvas))
	}
	// 不许编一个数字：既不该画 0/0，也不该画 0%。
	for _, banned := range []string{"0%", "0 / 0"} {
		if _, ok := composerText(canvas, banned); ok {
			t.Fatalf("未知上下文被编成了 %q", banned)
		}
	}
}

// TestOfficialSeatSendingShowsStop ②：发送中主钮变停止，点一下真取消。
func TestOfficialSeatSendingShowsStop(t *testing.T) {
	p := &Plugin{}
	p.field.SetText("排队中的草稿")
	cancelled, sent := 0, 0
	act := pluginapi.Actions{
		Send:   func(string) { sent++ },
		Cancel: func() { cancelled++ },
	}
	canvas := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Busy: true}, act, "stop")
	if _, ok := composerHit(canvas, "停止"); !ok {
		t.Fatalf("发送中没有停止键：%+v", canvas.Hits())
	}
	if _, ok := composerHit(canvas, "发送"); ok {
		t.Fatal("发送中仍然画了发送键（两个主钮）")
	}
	// 活动文案也是真值：Busy → 「正在生成」。
	//
	// 用前缀断言：viva 的 `Ellipsis` 在"文本宽正好等于可用宽"时会把末字换成
	// 省略号（测试量测器 0.55 的浮点边界，真机是真实字宽，不受影响）。
	if !canvasHasTextPrefix(canvas, "正在") {
		t.Fatalf("发送中没有活动文案：%v", canvasTextValues(canvas))
	}
	if cancelled != 1 || sent != 0 {
		t.Fatalf("点停止应只取消一次：cancel=%d send=%d", cancelled, sent)
	}
	if got := p.field.Text(); got != "排队中的草稿" {
		t.Fatalf("停止不应清草稿，得到 %q", got)
	}
}

// TestOfficialSeatEmptyInputDisabled ②：空输入禁用；有草稿可发且 trim。
func TestOfficialSeatEmptyInputDisabled(t *testing.T) {
	p := &Plugin{}
	var sent []string
	act := pluginapi.Actions{Send: func(text string) { sent = append(sent, text) }}
	empty := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, "")
	send, ok := composerHit(empty, "发送")
	if !ok || !send.Disabled {
		t.Fatalf("空输入时发送键应禁用：%+v found=%v", send, ok)
	}

	p.field.SetText("  hello official seat  ")
	canvas := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, "send")
	if hit, _ := composerHit(canvas, "发送"); hit.Disabled {
		t.Fatal("有草稿时发送键不该禁用")
	}
	if len(sent) != 1 || sent[0] != "hello official seat" {
		t.Fatalf("发送内容 = %#v，want 一次 trim 后的文本", sent)
	}
	if got := p.field.Text(); got != "" {
		t.Fatalf("发送成功后草稿应清空，得到 %q", got)
	}
}

// TestOfficialSeatApprovalSitsAboveInputCard ④：审批卡在输入卡片上方。
func TestOfficialSeatApprovalSitsAboveInputCard(t *testing.T) {
	p := &Plugin{}
	ap := officialApproval()
	canvas := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1", Approval: &ap}, fileSource(), "")
	approvalText, ok := composerText(canvas, "需要审批")
	if !ok {
		t.Fatalf("官方座位没画审批卡：%v", canvasTextValues(canvas))
	}
	card, ok := inputCardSurface(canvas, p.theme.InputMajor)
	if !ok {
		t.Fatal("官方座位没画输入卡片")
	}
	if approvalText.Bounds.Y >= card.Bounds.Y {
		t.Fatalf("审批卡 Y=%.1f 不在输入卡片 Y=%.1f 之上（composer 接管要求它在上面）",
			approvalText.Bounds.Y, card.Bounds.Y)
	}
	// 卡片必须仍完整（没有被审批卡挤掉一行）。
	if card.Bounds.Height != 98 {
		t.Fatalf("有审批卡时输入卡片高 = %v，官方 98", card.Bounds.Height)
	}
}

// TestOfficialSeatAddEntryOnlyWithFileSource ⑤：没有文件引用来源就不画附件入口。
//
// 画一个点了没反应的加号，比不画更糟（composer.go 文件头第 2 条硬规矩）。
func TestOfficialSeatAddEntryOnlyWithFileSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  pluginapi.Actions
		cwd  string
		want bool
	}{
		{name: "有 FSList + CWD", act: fileSource(), cwd: "D:/work", want: true},
		{name: "没有 FSList", act: pluginapi.Actions{Send: func(string) {}}, cwd: "D:/work", want: false},
		{name: "没有 CWD", act: fileSource(), cwd: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &Plugin{}
			canvas := renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1", CWD: tc.cwd}, tc.act, "")
			_, ok := composerHit(canvas, "添加文件引用")
			if ok != tc.want {
				t.Fatalf("附件入口出现 = %v，want %v", ok, tc.want)
			}
		})
	}
}

// TestOfficialSeatClickAddStartsFileReference 点附件入口 → 草稿末尾出现 `@`
// （下一帧 computeMention 就会列出 CWD 的候选，见 mention.go）。
func TestOfficialSeatClickAddStartsFileReference(t *testing.T) {
	p := &Plugin{}
	p.field.SetText("看下 ")
	renderOfficialSeat(t, p, pluginapi.Snapshot{Cur: "s-1", CWD: "D:/work"}, fileSource(), "composer-add")
	if got := p.field.Text(); got != "看下 @" {
		t.Fatalf("点附件入口后草稿 = %q，want %q", got, "看下 @")
	}
}

// TestShiftEnterDoesNotSend Enter 由输入框提交，Shift+Enter 被吃掉。
//
// 官方是多行编辑器：Enter 发送、Shift+Enter 换行。viva 的 flex.Field 是单行
// （Enter 就是提交），所以"插换行"这一半做不了；**"不发送"这一半必须做**——
// 不拦的话 Shift+Enter 会落到驱动的提交路径（painter.key → ClickedByKey），
// 用户以为在换行，草稿已经发出去了。
func TestShiftEnterDoesNotSend(t *testing.T) {
	p := &Plugin{}
	sent := 0
	p.lastAct = pluginapi.Actions{Send: func(string) { sent++ }}
	p.field.SetText("换行测试")
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter, Modifiers: gfx.ModifierShift}) {
		t.Fatal("Shift+Enter 必须被 HandleKey 吃掉（否则会落到提交路径）")
	}
	if sent != 0 {
		t.Fatalf("Shift+Enter 发送了 %d 次", sent)
	}
	if got := p.field.Text(); got != "换行测试" {
		t.Fatalf("Shift+Enter 改动了草稿：%q", got)
	}
	// 普通 Enter 必须**让回去**：输入框的 Submitted 才是发送路径。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter}) {
		t.Fatal("普通 Enter 被 HandleKey 抢走了，输入框就永远收不到提交")
	}
	// 其它修饰键组合也不接管（Ctrl+Enter 之类留给将来的插话/多行语义）。
	for _, mod := range []gfx.Modifiers{gfx.ModifierControl, gfx.ModifierAlt} {
		if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter, Modifiers: mod}) {
			t.Fatalf("修饰键 %v+Enter 被接管了", mod)
		}
	}
}

// inputCardSurface 找输入卡片那张圆角面板（radius-panel 28）。
//
// 不能只用 largestSurface：审批卡也是 InputMajor 底（radius 20/19），
// 它比输入卡片还高（156 vs 98），按面积取会拿到审批卡。
func inputCardSurface(canvas *gfx.Canvas, color ui.Color) (gfx.Rectangle, bool) {
	var best gfx.Rectangle
	found := false
	for _, r := range canvas.Rectangles() {
		if r.Color != color || r.Radius != 28 {
			continue
		}
		area := r.Bounds.Width * r.Bounds.Height
		if !found || area > best.Bounds.Width*best.Bounds.Height {
			best, found = r, true
		}
	}
	return best, found
}

// canvasHasTextPrefix 断言画布上存在以 prefix 开头的文本。
func canvasHasTextPrefix(canvas *gfx.Canvas, prefix string) bool {
	for _, text := range canvas.Texts() {
		if len(text.Value) >= len(prefix) && text.Value[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// TestContextMeterTextFormat 上下文读数的三种量级与「未知」态。
func TestContextMeterTextFormat(t *testing.T) {
	cases := []struct {
		name     string
		used     int64
		window   int64
		wantText string
	}{
		{name: "k 档一位小数", used: 1234, window: 200000, wantText: "1.2k / 200k（1%）"},
		{name: "k 档整数 + M 档整数", used: 15537, window: 1000000, wantText: "15k / 1M（2%）"},
		{name: "M 档两位小数", used: 1050000, window: 2000000, wantText: "1.05M / 2M（53%）"},
		{name: "小数目", used: 42, window: 1000, wantText: "42 / 1k（4%）"},
		{name: "容量未给", used: 42, window: 0, wantText: unknownField},
		{name: "用量未给", used: -1, window: 1000, wantText: unknownField},
		{name: "超容量按官方钳 100%", used: 2000000, window: 1000000, wantText: "2M / 1M（100%）"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := contextMeterText(pluginapi.Snapshot{ContextTokens: tc.used, ContextWindow: tc.window})
			if got != tc.wantText {
				t.Fatalf("contextMeterText(%d,%d) = %q，want %q", tc.used, tc.window, got, tc.wantText)
			}
		})
	}
}

// canvasTextValues 列画布文本（失败诊断用）。
func canvasTextValues(canvas *gfx.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}
