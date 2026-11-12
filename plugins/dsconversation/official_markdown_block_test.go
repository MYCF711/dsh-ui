package dsconversation

// official_markdown_block_test.go —— markdown **块级**与**代码块**的渲染断言
// （GT2 报告 §8 第 3/4 条）。
//
// # 覆盖表（每种块级元素至少一条「文本 + 几何」断言）
//
//	标题 h1–h6   TestMarkdownHeadingLadder          字号/字重/行高逐级 + 越界夹紧
//	无序列表     TestMarkdownUnorderedListIndent     • 标记 + level × 20px 缩进
//	有序列表     TestMarkdownOrderedListMarker       N. 标记（N = 官方 Number）
//	引用块       TestMarkdownBlockquoteBorder        左边框 2px + 左内边距 12px
//	分隔线       TestMarkdownRule                    1px 高 + 整列宽
//	块间距       TestMarkdownBlockGap                AssistantMarkdown.body 的 16px
//	围栏代码块   TestMarkdownCodeBlockBanner         语言 banner 32px + 等宽正文 + 高亮 run
//	             TestMarkdownCodeBlockScroll         横滚视口 + 滚轮事件接管 + 夹紧
//	             TestMarkdownCodeBlockCopy           复制动作回传原始字节 + 「已复制」
//	             TestMarkdownCodeBlockCopyFailure    复制失败态
//	             TestMarkdownCodeBlockWrapToggle     折行/横滚切换
//	全量         TestMarkdownAllBlocksRender         一段含全部块级的文档一次画完
//
// # 这些断言钉的是**本实现**的值，不是官方值
//
// 官方 `MarkdownText` / `FenceCodeBlock` 在
// `@deepseek-ai/dsh-client-ui-primitives` 里，而该包**没有 rc.2 `src/`**
// （结构树 §9 第 1/2 条）。所以标题阶梯、列表缩进、引用块边框、banner 高度
// 全部是 `official_transcript_geometry.go` 的 **derived** 行——断言只保证
// 「实现与常量表一致」，**不**证明「与官方像素一致」。取到 primitives `src/`
// 后这些断言原样保留，只把常量表换成 aligned 值。

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/mdlite"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// codeBlockLongLine 是一条**明确放不下**的代码行。
//
// 为什么要写死长度：横滚口存在与否由"内容宽 > 视口宽"决定，行短了测试会
// 静默退化成"没有溢出也算过"（第一版 104 字符恰好放得下，MaxOffsetX 是 0）。
// 这行 170 字符，在 834px 列宽、等宽 13px 档（每字符约 8px）下必然溢出。
const codeBlockLongLine = "const long = \"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" +
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz\""

// codeBlockSource 是一段带长行的围栏：长行用来证明**横向滚动**而不是硬切。
const codeBlockSource = "```go\nfmt.Println(\"hi\")\n" + codeBlockLongLine + "\n```"

// codeBlockWant 是上面那段围栏的**原始字节**（复制动作必须原样回传）。
const codeBlockWant = "fmt.Println(\"hi\")\n" + codeBlockLongLine

// errCopyInjected 是注入的剪贴板失败（不碰真实系统剪贴板）。
var errCopyInjected = errors.New("injected clipboard failure")

// markdownEntry 造一条只有正文的助手节点。
func markdownEntry(text string) []TranscriptEntry {
	return []TranscriptEntry{{
		Key: "md", Kind: ChatNodeAssistantStep, Seq: 1, Turn: 1, Visibility: ChatVisibilityVisible,
		Data: TranscriptData{Text: text},
	}}
}

// markdownLines 把多行源码拼成一段 markdown（每行之间空一行，避免段落合并）。
func markdownLines(lines ...string) string { return strings.Join(lines, "\n") }

// renderMarkdown 渲染一段 markdown 正文（真实画布 + 覆画阶段）。
func renderMarkdown(t *testing.T, text string) (*OfficialTranscriptView, *gfx.Canvas) {
	t.Helper()
	_, tree := newTranscriptCanvas()
	return renderTranscript(t, markdownEntry(text), hostViewport(tree))
}

// canvasTextsWith 数出画布上等于 value 的文本条数（标记槽这类会重复）。
func canvasTextsWith(canvas *gfx.Canvas, value string) int {
	count := 0
	for _, text := range canvas.Texts() {
		if text.Value == value {
			count++
		}
	}
	return count
}

// canvasRect 在画布上找**满足条件的第一块矩形**（背景/边框都走这里）。
func canvasRect(canvas *gfx.Canvas, match func(ui.Rect) bool) (ui.Rect, bool) {
	for _, rect := range canvas.Rectangles() {
		if match(rect.Bounds) {
			return rect.Bounds, true
		}
	}
	return ui.Rect{}, false
}

// canvasHitLabel 按无障碍标签找命中区（按钮的真实可点区域）。
func canvasHitLabel(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// ── 标题阶梯 ─────────────────────────────────────────────────────────

// TestMarkdownHeadingLadder 钉住 h1–h6 的字号/字重/行高阶梯。
//
// 几何来源：`MarkdownHeadingFor`（derived 表）。断言两件事：
//   - 画出来的字号/字重 **等于** 常量表（实现不漂移）；
//   - 阶梯**单调不增**（h1 ≥ h2 ≥ … ≥ h6，且 h1 明显大于正文 14px）。
func TestMarkdownHeadingLadder(t *testing.T) {
	source := markdownLines(
		"# 标题一", "", "## 标题二", "", "### 标题三", "",
		"#### 标题四", "", "##### 标题五", "", "###### 标题六",
	)
	_, canvas := renderMarkdown(t, source)

	names := []string{"标题一", "标题二", "标题三", "标题四", "标题五", "标题六"}
	for i, name := range names {
		level := i + 1
		want := MarkdownHeadingFor(level)
		text := mustCanvasText(t, canvas, name)
		if text.Style.Size != want.Size {
			t.Errorf("h%d 字号 = %v，应为 %v（derived 阶梯表）", level, text.Style.Size, want.Size)
		}
		if text.Style.Weight != gfx.WeightSemibold {
			t.Errorf("h%d 字重 = %v，应为 semibold", level, text.Style.Weight)
		}
		if text.Bounds.Height < want.Line {
			t.Errorf("h%d 行高 = %v，应不小于 %v", level, text.Bounds.Height, want.Line)
		}
	}

	// 阶梯单调不增 + 与正文的对比（h1 必须明显大于 14px 正文）。
	headings := MarkdownHeadings()
	if len(headings) != 6 {
		t.Fatalf("标题阶梯 = %d 级，应为 6", len(headings))
	}
	for i := 1; i < len(headings); i++ {
		if headings[i-1].Size < headings[i].Size {
			t.Errorf("h%d 字号 %v 小于 h%d 的 %v（阶梯必须单调不增）",
				i, headings[i-1].Size, i+1, headings[i].Size)
		}
		if headings[i-1].Line < headings[i].Line {
			t.Errorf("h%d 行高 %v 小于 h%d 的 %v", i, headings[i-1].Line, i+1, headings[i].Line)
		}
	}
	if headings[0].Size <= 14 {
		t.Errorf("h1 字号 %v 应大于正文 14px", headings[0].Size)
	}

	// 越界夹紧（官方 clamp(h1..h6) 语义）：0/负按 h1，>6 按 h6。
	if got := MarkdownHeadingFor(0); got.Level != 1 {
		t.Errorf("MarkdownHeadingFor(0).Level = %d，应夹紧到 1", got.Level)
	}
	if got := MarkdownHeadingFor(-3); got.Level != 1 {
		t.Errorf("MarkdownHeadingFor(-3).Level = %d，应夹紧到 1", got.Level)
	}
	if got := MarkdownHeadingFor(99); got.Level != 6 {
		t.Errorf("MarkdownHeadingFor(99).Level = %d，应夹紧到 6", got.Level)
	}
}

// ── 列表 ─────────────────────────────────────────────────────────────

// TestMarkdownUnorderedListIndent 钉住无序列表：标记字符 + 每级 20px 缩进。
func TestMarkdownUnorderedListIndent(t *testing.T) {
	_, canvas := renderMarkdown(t, markdownLines(
		"- 第一项", "- 第二项", "  - 嵌套项",
	))

	first := mustCanvasText(t, canvas, "第一项")
	nested := mustCanvasText(t, canvas, "嵌套项")
	if got := nested.Bounds.X - first.Bounds.X; got != MarkdownListIndentPerLevel {
		t.Errorf("嵌套项缩进 = %v，应为 %v（level × %v）",
			got, MarkdownListIndentPerLevel, MarkdownListIndentPerLevel)
	}
	if markers := canvasTextsWith(canvas, MarkdownListBullet); markers != 3 {
		t.Errorf("无序标记 %q 出现 %d 次，应为 3 次", MarkdownListBullet, markers)
	}

	// 标记槽 → 正文的间距必须等于 derived 的 6px。
	//
	// 为什么算式长这样：标记是 `TextEnd`（右对齐）画的，viva 把**绘制 X**
	// 右移了 (槽宽 - 字形宽)，但 `Bounds.Width` 仍是槽宽。所以
	//
	//	正文.X - 标记.X = 槽宽 + gap - (槽宽 - 字形宽) = gap + 字形宽
	//
	// 确定性测量器下字形宽 = rune 数 × 字号（"•" 1 个 rune × 14px = 14）。
	marker := mustCanvasText(t, canvas, MarkdownListBullet)
	glyph := float32(utf8.RuneCountInString(MarkdownListBullet)) * 14
	if got := first.Bounds.X - marker.Bounds.X; got != MarkdownListMarkerGap+glyph {
		t.Errorf("标记与正文间距 = %v，应为 %v（gap %v + 字形宽 %v）",
			got-MarkdownListMarkerGap, MarkdownListMarkerGap, MarkdownListMarkerGap, glyph)
	}
	if marker.Bounds.Width != MarkdownListMarkerWidth {
		t.Errorf("标记槽宽 = %v，应为 %v", marker.Bounds.Width, MarkdownListMarkerWidth)
	}
	if marker.Bounds.X >= first.Bounds.X {
		t.Errorf("标记必须画在正文左边：marker.X=%v text.X=%v", marker.Bounds.X, first.Bounds.X)
	}
}

// TestMarkdownOrderedListMarker 钉住有序列表：标记用**官方 Number 值**，不重排。
func TestMarkdownOrderedListMarker(t *testing.T) {
	_, canvas := renderMarkdown(t, markdownLines(
		"1. 第一步", "2. 第二步", "7. 第七步",
	))

	for _, want := range []string{"1.", "2.", "7."} {
		if got := canvasTextsWith(canvas, want); got != 1 {
			t.Errorf("有序标记 %q 出现 %d 次，应为 1 次", want, got)
		}
	}
	first := mustCanvasText(t, canvas, "第一步")
	seventh := mustCanvasText(t, canvas, "第七步")
	// 有序列表的标记宽度固定（18px），所以同一级的正文 X 必须**完全对齐**
	// ——这正是"标记槽"存在的理由（否则 9.→10. 会左右跳）。
	if first.Bounds.X != seventh.Bounds.X {
		t.Errorf("同一级有序列表正文没有对齐：%v vs %v", first.Bounds.X, seventh.Bounds.X)
	}
}

// ── 引用块 ───────────────────────────────────────────────────────────

// TestMarkdownBlockquoteBorder 钉住引用块：左边框 2px + 左内边距 12px。
func TestMarkdownBlockquoteBorder(t *testing.T) {
	_, canvas := renderMarkdown(t, markdownLines(
		"> 引用正文", "", "普通段落",
	))

	quote := mustCanvasText(t, canvas, "引用正文")
	plain := mustCanvasText(t, canvas, "普通段落")
	if quote.Bounds.X <= plain.Bounds.X {
		t.Errorf("引用块正文必须比普通段落靠右：quote.X=%v plain.X=%v", quote.Bounds.X, plain.Bounds.X)
	}

	// 左边框：宽度 == MarkdownQuoteBorderWidth 的竖线，且正文比它靠右正好
	// MarkdownQuotePadLeft。
	border, ok := canvasRect(canvas, func(r ui.Rect) bool {
		return r.Width == MarkdownQuoteBorderWidth && r.Height > 0
	})
	if !ok {
		t.Fatalf("引用块没有画出 %vpx 宽的左边框", MarkdownQuoteBorderWidth)
	}
	if got := quote.Bounds.X - (border.X + border.Width); got != MarkdownQuotePadLeft {
		t.Errorf("引用块左内边距 = %v，应为 %v", got, MarkdownQuotePadLeft)
	}
	if border.Height < quote.Bounds.Height {
		t.Errorf("引用块左边框高 %v，应不低于正文行高 %v", border.Height, quote.Bounds.Height)
	}
}

// ── 分隔线 ───────────────────────────────────────────────────────────

// TestMarkdownRule 钉住分隔线：1 物理像素高、整列宽。
func TestMarkdownRule(t *testing.T) {
	view, canvas := renderMarkdown(t, markdownLines("上面", "", "---", "", "下面"))

	rule, ok := canvasRect(canvas, func(r ui.Rect) bool { return r.Height == MarkdownRuleHeight })
	if !ok {
		t.Fatalf("分隔线没有画出来（应有 %vpx 高的矩形）", MarkdownRuleHeight)
	}
	column := view.Boxes.Column.Rect()
	if rule.Width != column.Width {
		t.Errorf("分隔线宽 = %v，应等于列宽 %v", rule.Width, column.Width)
	}
	above := mustCanvasText(t, canvas, "上面")
	below := mustCanvasText(t, canvas, "下面")
	if !(above.Bounds.Y < rule.Y && rule.Y < below.Bounds.Y) {
		t.Errorf("分隔线必须在两段之间：above.Y=%v rule.Y=%v below.Y=%v",
			above.Bounds.Y, rule.Y, below.Bounds.Y)
	}
}

// ── 块间距 ───────────────────────────────────────────────────────────

// TestMarkdownBlockGap 钉住 `AssistantMarkdown.body` 的 16px 块间距
// （`AssistantMarkdown.module.css:17-22`，几何表 aligned 行）。
func TestMarkdownBlockGap(t *testing.T) {
	_, canvas := renderMarkdown(t, markdownLines("第一段", "", "第二段"))

	first := mustCanvasText(t, canvas, "第一段")
	second := mustCanvasText(t, canvas, "第二段")
	got := second.Bounds.Y - (first.Bounds.Y + first.Bounds.Height)
	if got != AssistantBlockGap {
		t.Errorf("块间距 = %v，应为 %v（AssistantMarkdown.body 的 gap）", got, AssistantBlockGap)
	}
}

// ── 代码块 ───────────────────────────────────────────────────────────

// TestMarkdownCodeBlockBanner 钉住代码块的 banner：语言标签 + 32px 高 + 等宽正文。
func TestMarkdownCodeBlockBanner(t *testing.T) {
	view, canvas := renderMarkdown(t, codeBlockSource)

	language := mustCanvasText(t, canvas, "go")
	if language.Style.Size != CodeBlockBannerFontSize {
		t.Errorf("banner 语言标签字号 = %v，应为 %v", language.Style.Size, CodeBlockBannerFontSize)
	}
	// banner 自己那块底色矩形：高度 == 32（derived）。
	banner, ok := canvasRect(canvas, func(r ui.Rect) bool {
		return r.Height == CodeBlockBannerHeight && r.Width > 0
	})
	if !ok {
		t.Fatalf("代码块 banner 没有画出 %vpx 高的矩形", CodeBlockBannerHeight)
	}
	if banner.Y > language.Bounds.Y {
		t.Errorf("banner 底色必须在语言标签上方：banner.Y=%v label.Y=%v", banner.Y, language.Bounds.Y)
	}

	// 正文等宽（语法高亮由 mdlite.Highlight 提供，不手写）。
	code := mustCanvasText(t, canvas, `fmt.Println("hi")`)
	if !code.Style.Mono {
		t.Error("代码块正文必须走等宽字体（Mono）")
	}
	if code.Bounds.Y <= banner.Y+banner.Height {
		t.Errorf("正文必须在 banner 下方：code.Y=%v banner.bottom=%v", code.Bounds.Y, banner.Y+banner.Height)
	}
	if runs := canvas.TextRuns(code); len(runs) == 0 {
		t.Error("代码块正文必须带语法高亮 run（不得整段同色）")
	}

	geometry, ok := view.CodeBlockGeometryOf("md:0")
	if !ok {
		t.Fatal("代码块没有记录几何（placeCodeBlock / PaintCodeBlocks 没跑）")
	}
	if geometry.Banner.Height != CodeBlockBannerHeight {
		t.Errorf("几何 banner 高 = %v，应为 %v", geometry.Banner.Height, CodeBlockBannerHeight)
	}
	if geometry.Content.Height <= 0 || geometry.Content.Width <= 0 {
		t.Errorf("代码块视口几何为零：%+v", geometry.Content)
	}
	if geometry.VisualLines != 2 {
		t.Errorf("代码块可见行数 = %d，应为 2", geometry.VisualLines)
	}
}

// TestMarkdownCodeBlockScroll 钉住横向滚动：长行必须溢出成横滚口，不是硬切。
func TestMarkdownCodeBlockScroll(t *testing.T) {
	view, canvas := renderMarkdown(t, codeBlockSource)

	geometry, ok := view.CodeBlockGeometryOf("md:0")
	if !ok {
		t.Fatal("代码块没有记录几何")
	}
	if geometry.MaxOffsetX <= 0 {
		t.Fatalf("长行没有横向溢出（MaxOffsetX = %v），会被右边缘硬切", geometry.MaxOffsetX)
	}

	// 横滚视口是真实命中区（宿主滚轮事件按它路由）。
	viewport, ok := canvasHitLabel(canvas, "代码横向滚动")
	if !ok {
		t.Fatalf("横滚视口没有声明命中区（HandleScroll 收不到事件）；hits=%v", canvas.Hits())
	}
	// 向右滚一屏：偏移必须真的变，且被夹紧在上限内。
	before := view.codeBlocks[0].block.State().OffsetX
	result := view.HandleScroll(gfx.ScrollEvent{Target: viewport.ID, DeltaX: 1e6, Precise: true})
	if !result.Handled {
		t.Fatal("横滚视口上的滚轮事件没有被接管")
	}
	after := view.codeBlocks[0].block.State().OffsetX
	if after <= before {
		t.Fatalf("滚轮后横向偏移没有增加：%v → %v", before, after)
	}
	if after > geometry.MaxOffsetX {
		t.Errorf("横向偏移 %v 超过上限 %v", after, geometry.MaxOffsetX)
	}

	// 别处的滚轮（外层消息流）**不许**被代码块接管。
	if result := view.HandleScroll(gfx.ScrollEvent{Target: gfx.Item(999), DeltaY: 120}); result.Handled {
		t.Error("别处的滚轮被代码块抢走了（会表现为消息列表滚不动）")
	}
}

// TestMarkdownCodeBlockCopy 钉住复制动作：点击复制按钮回传**原始字节**，
// 并把"已复制"反馈画出来。
//
// 为什么要重放整帧：viva 的点击走事件通道（`BeginEventPass` → 重画一次），
// 复制回调在 `canvas.AfterPaint` 里执行。所以这里是一个**真实**的点击路径，
// 不是直接调 `block.Copy`。
func TestMarkdownCodeBlockCopy(t *testing.T) {
	fixture := newMarkdownFixture()
	fixture.draw()

	copyHit, ok := canvasHitLabel(fixture.canvas, "复制代码")
	if !ok {
		t.Fatalf("没有复制按钮的命中区；hits=%v", fixture.canvas.Hits())
	}
	fixture.canvas.BeginEventPass(copyHit.ID, gfx.ClickInfo{})
	fixture.draw()
	fixture.canvas.EndEventPass()
	fixture.canvas.RunAfterPaint()
	// 复制回调在 AfterPaint 里改状态，必须再画一帧才看得到反馈文案
	// （真机上这一帧由 viva 的 Repaint 触发）。
	fixture.draw()

	if len(fixture.copied) != 1 {
		t.Fatalf("复制回调调用 %d 次，应为 1 次", len(fixture.copied))
	}
	if fixture.copied[0] != codeBlockWant {
		t.Fatalf("复制的字节被改过：\n got %q\nwant %q", fixture.copied[0], codeBlockWant)
	}
	if canvasTextsWith(fixture.canvas, "已复制") == 0 {
		t.Error("复制成功后必须画出「已复制」反馈")
	}
	if got := fixture.view.codeBlocks[0].block.CopyStatus(); got != mdlite.CopySucceeded {
		t.Errorf("复制状态 = %v，应为 CopySucceeded", got)
	}
}

// TestMarkdownCodeBlockCopyFailure 钉住复制失败态：宿主报错时画「复制失败」。
func TestMarkdownCodeBlockCopyFailure(t *testing.T) {
	fixture := newMarkdownFixture()
	fixture.copyErr = errCopyInjected
	fixture.draw()

	copyHit, ok := canvasHitLabel(fixture.canvas, "复制代码")
	if !ok {
		t.Fatalf("没有复制按钮的命中区；hits=%v", fixture.canvas.Hits())
	}
	fixture.canvas.BeginEventPass(copyHit.ID, gfx.ClickInfo{})
	fixture.draw()
	fixture.canvas.EndEventPass()
	fixture.canvas.RunAfterPaint()
	fixture.draw()

	if canvasTextsWith(fixture.canvas, "复制失败") == 0 {
		t.Error("复制失败时必须画出「复制失败」反馈")
	}
	if got := fixture.view.codeBlocks[0].block.CopyStatus(); got != mdlite.CopyFailed {
		t.Errorf("复制状态 = %v，应为 CopyFailed", got)
	}
}

// TestMarkdownCodeBlockWrapToggle 钉住「折行/横滚」切换：折行后不得再有横向溢出。
func TestMarkdownCodeBlockWrapToggle(t *testing.T) {
	fixture := newMarkdownFixture()
	fixture.draw()

	// 默认必须是横滚态（官方 FenceCodeBlock 默认不折行）。
	if fixture.view.codeBlocks[0].block.State().Wrap {
		t.Fatal("代码块默认必须是横滚态，不是折行态")
	}
	wrapHit, ok := canvasHitLabel(fixture.canvas, "切换折行")
	if !ok {
		t.Fatalf("没有折行按钮的命中区；hits=%v", fixture.canvas.Hits())
	}
	fixture.canvas.BeginEventPass(wrapHit.ID, gfx.ClickInfo{})
	fixture.draw()
	fixture.canvas.EndEventPass()
	fixture.canvas.RunAfterPaint()
	if !fixture.view.codeBlocks[0].block.State().Wrap {
		t.Fatal("点击后没有切到折行态")
	}

	// 再画一帧：折行态下 MaxOffsetX 必须归零（否则横滚条还挂着）。
	fixture.draw()
	geometry, ok := fixture.view.CodeBlockGeometryOf("md:0")
	if !ok {
		t.Fatal("代码块没有记录几何")
	}
	if geometry.MaxOffsetX != 0 {
		t.Errorf("折行态仍有横向溢出（MaxOffsetX = %v）", geometry.MaxOffsetX)
	}
	if geometry.VisualLines <= 2 {
		t.Errorf("折行后可见行数 = %d，应多于物理行数 2", geometry.VisualLines)
	}
}

// ── 全量覆盖 ─────────────────────────────────────────────────────────

// TestMarkdownAllBlocksRender 是**块级元素覆盖表**：一段含全部块级的文档
// 一次画完，每种元素都必须真的出现在画布上（不是"有渲染器"这种空断言）。
func TestMarkdownAllBlocksRender(t *testing.T) {
	source := markdownLines(
		"# 一级标题", "", "## 二级标题", "", "### 三级标题", "",
		"#### 四级标题", "", "##### 五级标题", "", "###### 六级标题", "",
		"正文段落。", "",
		"- 无序一", "- 无序二", "",
		"1. 有序一", "2. 有序二", "",
		"> 引用正文", "",
		"---", "",
		codeBlockSource,
	)
	view, canvas := renderMarkdown(t, source)

	want := []string{
		// 标题（6 级）
		"一级标题", "二级标题", "三级标题", "四级标题", "五级标题", "六级标题",
		// 段落
		"正文段落。",
		// 列表（无序标记 + 有序标记）
		"无序一", "无序二", "有序一", "有序二", MarkdownListBullet, "1.", "2.",
		// 引用块
		"引用正文",
		// 代码块（banner 语言 + 正文）
		"go", `fmt.Println("hi")`,
	}
	for _, value := range want {
		if _, ok := canvasText(canvas, value); !ok {
			t.Errorf("块级元素 %q 没有画出来", value)
		}
	}

	// 分隔线 1 条（`---` 只算一条），代码块 banner 一条。
	if _, ok := canvasRect(canvas, func(r ui.Rect) bool { return r.Height == MarkdownRuleHeight }); !ok {
		t.Error("分隔线没有画出来")
	}
	if len(view.codeBlocks) != 1 {
		t.Fatalf("代码块占位 %d 条，应为 1 条（placeCodeBlock 没跑）", len(view.codeBlocks))
	}
	if view.codeBlocks[0].geometry.Content.Height <= 0 {
		t.Errorf("代码块没有真实视口几何：%+v", view.codeBlocks[0].geometry.Content)
	}

	// 每种块的**字号**都必须来自常量表（不是写死的魔法数）。
	for level, name := range []string{"一级标题", "二级标题", "三级标题", "四级标题", "五级标题", "六级标题"} {
		text := mustCanvasText(t, canvas, name)
		if want := MarkdownHeadingFor(level + 1).Size; text.Style.Size != want {
			t.Errorf("h%d 字号 = %v，应为 %v", level+1, text.Style.Size, want)
		}
	}
	paragraph := mustCanvasText(t, canvas, "正文段落。")
	if paragraph.Style.Size != 14 {
		t.Errorf("正文号 = %v，应为 14（--dsh-content-font-size 已取证档）", paragraph.Style.Size)
	}
	code := mustCanvasText(t, canvas, `fmt.Println("hi")`)
	if code.Style.Size != 13 {
		t.Errorf("代码块正文字号 = %v，应为 13（--dsw-font-markdown-code 档）", code.Style.Size)
	}
}

// ── 测试夹具 ─────────────────────────────────────────────────────────

// markdownFixture 是一帧可重放的 markdown 渲染（事件通道必须重画同一棵树）。
type markdownFixture struct {
	view    *OfficialTranscriptView
	canvas  *gfx.Canvas
	tree    *gfx.Tree
	theme   *pluginapi.Theme
	env     *pluginapi.Env
	copied  []string
	copyErr error
}

// newMarkdownFixture 造一个确定性画布（不需要窗口）。
func newMarkdownFixture() *markdownFixture {
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	canvas, tree := newTranscriptCanvas()
	fixture := &markdownFixture{view: view, canvas: canvas, tree: tree, theme: theme}
	fixture.env = &pluginapi.Env{
		Theme: theme, Tree: tree, Feat: tree.Feature("markdown-block-test"),
		Canvas: canvas, Bounds: ui.Rect{Width: 900, Height: 700}, MainWidth: 900,
	}
	fixture.env.Act.CopyResult = func(text string) error {
		fixture.copied = append(fixture.copied, text)
		return fixture.copyErr
	}
	return fixture
}

// draw 跑一帧：布局（Draw）→ 覆画（PaintCodeBlocks）。
func (f *markdownFixture) draw() {
	f.canvas.Reset()
	pageTheme := dscore.VivaTheme(f.theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(f.canvas, ui.Rect{Width: 900, Height: 700}, func(frame ui.C) {
		f.view.Draw(frame, f.env, hostViewport(f.tree), markdownEntry(codeBlockSource))
	})
	f.view.PaintCodeBlocks(f.canvas, f.env)
}

// TestMarkdownTable 钉住表格：按**等宽**原样渲染，不丢列、不重排。
//
// 为什么不是"表格布局"：官方宽表格突破用 `md-table-wide` 的 `100cqw`，
// 而 Viva 没有容器查询（几何表 deviation 行）。这里按单元格逐行等宽输出，
// 是**渲染器能力边界**而不是"没做"——取到容器查询前不做列宽求解。
func TestMarkdownTable(t *testing.T) {
	_, canvas := renderMarkdown(t, markdownLines(
		"| 列一 | 列二 |",
		"| --- | --- |",
		"| 甲 | 乙 |",
	))

	for _, want := range []string{"列一", "列二", "甲", "乙"} {
		text := mustCanvasText(t, canvas, want)
		if !text.Style.Mono {
			t.Errorf("表格单元格 %q 必须等宽（Mono）", want)
		}
	}
}
