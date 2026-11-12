package mdlite

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

func TestFencedCodeBlocksMetadataAndEmpty(t *testing.T) {
	source := "```go,title=main.go\npackage main\n```\n\n" +
		"```python,title=统计.py\nprint('你好')\n```\n\n" +
		"```text\n```\n"
	var doc markdown.Document
	doc.Parse(source)
	if len(doc.Blocks) != 3 {
		t.Fatalf("fenced blocks = %d, want 3", len(doc.Blocks))
	}
	want := []struct {
		language, title, code string
	}{
		{"go", "main.go", "package main"},
		{"python", "统计.py", "print('你好')"},
		{"text", "", ""},
	}
	for i, block := range doc.Blocks {
		got := NewCodeBlock(block, nil)
		if got.Language != want[i].language || got.Title != want[i].title || got.Code != want[i].code {
			t.Errorf("block %d = language %q title %q code %q, want %+v", i, got.Language, got.Title, got.Code, want[i])
		}
	}
}

func TestParseCodeFenceInfoTitleForms(t *testing.T) {
	for _, test := range []struct {
		info, language, title string
	}{
		{"go,title=main.go", "go", "main.go"},
		{"python title=\"统计 分析.py\"", "python", "统计 分析.py"},
		{"typescript,title=ui.ts,linenos", "typescript", "ui.ts"},
		{"", "text", ""},
	} {
		language, title := ParseCodeFenceInfo(test.info)
		if language != test.language || title != test.title {
			t.Errorf("ParseCodeFenceInfo(%q) = %q, %q; want %q, %q", test.info, language, title, test.language, test.title)
		}
	}
}

func TestCodeBlockStateRestore(t *testing.T) {
	var block CodeBlock
	want := CodeBlockState{Wrap: true, OffsetX: 37.5}
	block.Restore(want)
	if got := block.State(); got != want {
		t.Fatalf("State() = %+v, want %+v", got, want)
	}
	block.Restore(CodeBlockState{OffsetX: -10})
	if got := block.State().OffsetX; got != 0 {
		t.Fatalf("negative restored offset = %v, want 0", got)
	}
}

func TestCodeBlockLongUnicodeLineWrapsOrScrollsWithoutTruncation(t *testing.T) {
	longLine := "const 演示 = \"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-你好🙂-tail\""
	code := CodeBlock{Language: "go", Code: longLine + "\n第二行"}
	style := DefaultCodeBlockStyle(ui.DarkTheme())
	ids := CodeBlockIDs{Copy: gfx.Item(1), Wrap: gfx.Item(2), Scroll: gfx.Item(3)}
	var canvas gfx.Canvas
	const width = float32(240)

	height := code.Height(&canvas, width, style)
	geometry := code.Draw(&canvas, gfx.Rect{Width: width, Height: height}, ids, style)
	if geometry.MaxOffsetX <= 0 || geometry.ContentWidth <= geometry.Content.Width {
		t.Fatalf("nowrap must expose horizontal overflow: %+v", geometry)
	}
	if got := strings.Join(monoTextValues(canvas.Texts()), "\n"); got != code.Code {
		t.Fatalf("nowrap changed code bytes on screen:\n got %q\nwant %q", got, code.Code)
	}
	nowrapLines := geometry.VisualLines

	code.Restore(CodeBlockState{Wrap: true, OffsetX: geometry.MaxOffsetX})
	canvas.Reset()
	height = code.Height(&canvas, width, style)
	geometry = code.Draw(&canvas, gfx.Rect{Width: width, Height: height}, ids, style)
	if geometry.MaxOffsetX != 0 {
		t.Fatalf("wrapped block still has horizontal overflow: %+v", geometry)
	}
	if geometry.VisualLines <= nowrapLines {
		t.Fatalf("long line did not wrap: before=%d after=%d", nowrapLines, geometry.VisualLines)
	}
	// 软折行只增加视觉行，不应成为复制文本里的新换行；去掉所有视觉行边界后，
	// 每个字节仍须按原顺序出现（原始物理换行在这里同样不参与屏幕文字比较）。
	if got, want := strings.Join(monoTextValues(canvas.Texts()), ""), strings.ReplaceAll(code.Code, "\n", ""); got != want {
		t.Fatalf("wrap lost or reordered code:\n got %q\nwant %q", got, want)
	}
	for _, text := range canvas.Texts() {
		if text.Style.Mono && text.Bounds.Width > geometry.Content.Width+0.01 {
			t.Errorf("wrapped text %q exceeds viewport: width=%v viewport=%v", text.Value, text.Bounds.Width, geometry.Content.Width)
		}
	}
	if got := code.State().OffsetX; got <= 0 {
		t.Fatalf("wrap toggle must retain restorable horizontal state, offset=%v", got)
	}
}

func TestCodeBlockEmptyAndHighlightStillPaint(t *testing.T) {
	style := DefaultCodeBlockStyle(ui.DarkTheme())
	var canvas gfx.Canvas
	empty := CodeBlock{Language: "python", Title: "空.py"}
	height := empty.Height(&canvas, 320, style)
	geometry := empty.Draw(&canvas, gfx.Rect{Width: 320, Height: height}, CodeBlockIDs{Copy: gfx.Item(4)}, style)
	if geometry.Bounds.Height <= style.BannerHeight || len(canvas.Hits()) == 0 {
		t.Fatalf("empty code block lost banner/copy target: geometry=%+v hits=%+v", geometry, canvas.Hits())
	}

	canvas.Reset()
	highlighted := CodeBlock{Language: "go", Code: `func main() { return "你好" }`}
	height = highlighted.Height(&canvas, 520, style)
	highlighted.Draw(&canvas, gfx.Rect{Width: 520, Height: height}, CodeBlockIDs{}, style)
	keywordColor := style.Look.Code[components.CodeKeyword-1]
	foundKeyword := false
	for _, text := range canvas.Texts() {
		if !text.Style.Mono {
			continue
		}
		for _, run := range canvas.TextRuns(text) {
			if run.Color == keywordColor {
				foundKeyword = true
			}
		}
	}
	if !foundKeyword {
		t.Fatalf("existing Highlight colors were not used by CodeBlock")
	}
}

func monoTextValues(texts []gfx.Text) []string {
	var values []string
	for _, text := range texts {
		if text.Style.Mono {
			values = append(values, text.Value)
		}
	}
	return values
}
