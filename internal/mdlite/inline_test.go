package mdlite

// inline_test.go —— 行内排版的纯几何（`#18` (b) ①：图夹在句子里）。
//
// 这条守的是「**图与前后文字在同一行**」：官方正文的图是 inline-block 原子盒
// （`OFF:dist/assets/index-CSAdOBx6.css` 的 `._imageButton_13jw0_471{display:inline-block}`），
// 切成块级会让「文字 + 图 + 文字」多一次换行。
//
// 量宽用一个确定的假量法（每个 rune 10px），不依赖字体：这里钉的是切分与换行，
// 像素级判据在 `plugins/dsconversation` 的真画布/真窗口用例里。

import (
	"strings"
	"testing"
)

// inlineTestMeasure 是假量法：每个 rune 10px（空格也是 10px）。
func inlineTestMeasure(text string, _ bool) float32 {
	return float32(len([]rune(text))) * 10
}

// inlineTestImage 造一张原子图。
func inlineTestImage(destination string, width, height float32) *MarkdownInlineImage {
	return &MarkdownInlineImage{Destination: destination, Alt: "图", Width: width, Height: height}
}

// inlineLineTexts 把一行的元素摊成可读的账：`t:文字` / `i:目的地`。
func inlineLineTexts(lines []MarkdownInlineLine) [][]string {
	out := make([][]string, 0, len(lines))
	for _, line := range lines {
		parts := make([]string, 0, len(line.Items))
		for _, item := range line.Items {
			if item.Image != nil {
				parts = append(parts, "i:"+item.Image.Destination)
				continue
			}
			parts = append(parts, "t:"+item.Text.Text)
		}
		out = append(out, parts)
	}
	return out
}

// TestMarkdownInlineKeepsSentenceAndImageOnOneLine 是「多一次换行」那条旧账的判据。
//
// 「文字 + 图 + 文字」放得下时**必须是一行**：三者在同一个行内流里。旧实现把图切成
// 块级，这一段会变成 3 行（文字 / 图 / 文字各一行）。
func TestMarkdownInlineKeepsSentenceAndImageOnOneLine(t *testing.T) {
	slots := []MarkdownInlineSlot{
		{Text: "看图 "},
		{Image: inlineTestImage("./shot.png", 100, 50)},
		{Text: " 结尾"},
	}
	lines := LayoutMarkdownInline(slots, 800, inlineTestMeasure)
	if len(lines) != 1 {
		t.Fatalf("放得下时必须排成 1 行（官方图是行内），实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	got := inlineLineTexts(lines)[0]
	want := []string{"t:看图 ", "i:./shot.png", "t: 结尾"}
	if len(got) != len(want) {
		t.Fatalf("一行里的元素 = %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个元素 = %q，期望 %q（整行 %v）", i, got[i], want[i], got)
		}
	}
	// 行宽 = 文字(3 rune) + 图(100) + 文字(3 rune) = 30+100+30：图与文字的**空格**都占位。
	if lines[0].Width != 160 {
		t.Fatalf("行宽 = %v，期望 160（3×10 + 100 + 3×10）", lines[0].Width)
	}
	// 图左边那格空白要保留（图是行内的，它前面的空格真的占位）：图在 x=30。
	if text := lines[0].Items[0]; text.Width != 30 {
		t.Fatalf("图前面的文字宽 = %v，期望 30（含图前那格空白）", text.Width)
	}
}

// TestMarkdownInlineWrapsOnlyWhatDoesNotFit 钉"放不下才换行"，而且**只多那一行**。
//
// 图 80 宽、列宽 100：图挤不下"看图 "后面，于是整张换到第二行；图后面的" 尾"跟着
// **图那一行**（同一行内流）——而旧实现是 3 行（文字 / 图 / 文字）。
func TestMarkdownInlineWrapsOnlyWhatDoesNotFit(t *testing.T) {
	slots := []MarkdownInlineSlot{
		{Text: "看图 "},
		{Image: inlineTestImage("./shot.png", 80, 50)},
		{Text: " 尾"},
	}
	lines := LayoutMarkdownInline(slots, 100, inlineTestMeasure)
	if len(lines) != 2 {
		t.Fatalf("列宽 100 时应是 2 行（文字 / 图+尾），实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	first := inlineLineTexts(lines)[0]
	if len(first) != 1 || first[0] != "t:看图" {
		t.Fatalf("第 1 行 = %v，期望只有「看图」（行尾空白折掉）", first)
	}
	second := inlineLineTexts(lines)[1]
	if len(second) != 2 || second[0] != "i:./shot.png" || second[1] != "t: 尾" {
		t.Fatalf("第 2 行 = %v，期望「图 + 尾」（图后文字与图同行）", second)
	}
}

// TestMarkdownInlineImageAloneIsOneLine 是**反向**判据的一半：图单独成段时行内排版
// 也只给出一行一个元素（老路径整块画，像素不进这条用例，见 dsconversation 的用例）。
func TestMarkdownInlineImageAloneIsOneLine(t *testing.T) {
	lines := LayoutMarkdownInline([]MarkdownInlineSlot{
		{Image: inlineTestImage("./shot.png", 300, 150)},
	}, 800, inlineTestMeasure)
	if len(lines) != 1 || len(lines[0].Items) != 1 || lines[0].Items[0].Image == nil {
		t.Fatalf("图单独成段应是一行一个元素，实际 %v", inlineLineTexts(lines))
	}
	if lines[0].Width != 300 {
		t.Fatalf("行宽 = %v，期望等于图的宽度 300", lines[0].Width)
	}
}

// TestMarkdownInlineHardBreakStaysALine 钉换行符仍是硬断行（与本仓既有 Wrap 同口径）。
func TestMarkdownInlineHardBreakStaysALine(t *testing.T) {
	lines := LayoutMarkdownInline([]MarkdownInlineSlot{{Text: "第一行\n第二行"}}, 800, inlineTestMeasure)
	if len(lines) != 2 {
		t.Fatalf("`\\n` 应断成 2 行，实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	// 连续两个换行 = 一个空行（与本仓 Wrap 的 splitLine 同口径）。
	lines = LayoutMarkdownInline([]MarkdownInlineSlot{{Text: "a\n\nb"}}, 800, inlineTestMeasure)
	if len(lines) != 3 {
		t.Fatalf("`a\\n\\nb` 应是 3 行（中间空行），实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	if len(lines[1].Items) != 0 {
		t.Fatalf("中间那行应是空行，实际 %v", inlineLineTexts(lines)[1])
	}
}

// TestMarkdownInlineBreaksOverlongWordByRune 钉官方 `overflow-wrap:anywhere`：
// 比整行还宽的词按字符断开（否则会整段溢出正文列）。
func TestMarkdownInlineBreaksOverlongWordByRune(t *testing.T) {
	lines := LayoutMarkdownInline([]MarkdownInlineSlot{{Text: "abcdefghij"}}, 30, inlineTestMeasure)
	if len(lines) != 4 {
		t.Fatalf("30px 列宽放 100px 的词应是 4 行，实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	var joined strings.Builder
	for _, line := range lines {
		if len(line.Items) != 1 {
			t.Fatalf("硬切之后每行应只有一段，实际 %v", inlineLineTexts(lines))
		}
		joined.WriteString(line.Items[0].Text.Text)
	}
	if joined.String() != "abcdefghij" {
		t.Fatalf("硬切之后文字被改了：%q", joined.String())
	}
}

// TestMarkdownInlineAltTextStaysInline 钉**加载中途**那一档：图还没到，官方画的是
// 行内 alt 文案 —— 它要在同一行里，且不许与正文文字合成一段（颜色不同）。
func TestMarkdownInlineAltTextStaysInline(t *testing.T) {
	slots := []MarkdownInlineSlot{
		{Text: "看图 "},
		{Text: "示意图", Alt: true},
		{Text: " 结尾"},
	}
	lines := LayoutMarkdownInline(slots, 800, inlineTestMeasure)
	if len(lines) != 1 {
		t.Fatalf("alt 文案是行内文字，应在一行里，实际 %d 行：%v", len(lines), inlineLineTexts(lines))
	}
	if len(lines[0].Items) != 3 {
		t.Fatalf("alt 段不许与正文合并（颜色不同），实际 %v", inlineLineTexts(lines)[0])
	}
	if !lines[0].Items[1].Text.Alt {
		t.Fatalf("中间那段应是 alt 段：%+v", lines[0].Items[1].Text)
	}
}

// TestMarkdownInlineTwoImagesShareALine 钉多图：官方两张 inline-block 图在同一行时
// 并肩排（本仓旧实现是各自独占一行）。
func TestMarkdownInlineTwoImagesShareALine(t *testing.T) {
	lines := LayoutMarkdownInline([]MarkdownInlineSlot{
		{Image: inlineTestImage("./a.png", 100, 40)},
		{Text: " "},
		{Image: inlineTestImage("./b.png", 100, 40)},
	}, 800, inlineTestMeasure)
	if len(lines) != 1 || len(lines[0].Items) != 3 {
		t.Fatalf("两张图应并肩在同一行（中间一格空白），实际 %v", inlineLineTexts(lines))
	}
	c := inlineLineTexts(lines)[0]
	if c[0] != "i:./a.png" || c[1] != "t: " || c[2] != "i:./b.png" {
		t.Fatalf("一行里的元素 = %v", c)
	}
	// 第二张图的 x = 100 + 空格 10。
	if lines[0].Items[2].Width != 100 || lines[0].Width != 210 {
		t.Fatalf("行宽 = %v，期望 210（图100 + 空10 + 图100）", lines[0].Width)
	}
}

// TestMarkdownInlineNilMeasureIsNoLayout 钉零值输入不 panic（老壳没有量宽入口）。
func TestMarkdownInlineNilMeasureIsNoLayout(t *testing.T) {
	if lines := LayoutMarkdownInline([]MarkdownInlineSlot{{Text: "x"}}, 100, nil); lines != nil {
		t.Fatalf("没有量宽入口时应返回 nil，实际 %v", lines)
	}
	if lines := LayoutMarkdownInline(nil, 100, inlineTestMeasure); lines != nil {
		t.Fatalf("没有内容时应返回 nil，实际 %v", lines)
	}
}
