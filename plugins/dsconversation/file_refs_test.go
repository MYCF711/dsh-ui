package dsconversation

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func fileRefsToolMessage(tool, key, path string) apiclient.Message {
	args, _ := json.Marshal(map[string]string{key: path})
	return apiclient.Message{Role: "tool", Tool: tool, Args: string(args)}
}

func fileRefsData(cwd string, messages ...apiclient.Message) pluginapi.Snapshot {
	return pluginapi.Snapshot{Cur: "s-file-refs", CWD: cwd, Msgs: messages}
}

// fileRefDestinations 只返回经过显式文件映射的链接；HTTP 自动链接不算文件。
func fileRefDestinations(p *Plugin, index int) []string {
	if index >= len(p.views) {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for i := 0; i < p.views[index].LinkCount(); i++ {
		fake := p.views[index].Link(i)
		if destination, ok := p.fileLinks[index][fake]; ok && !seen[destination] {
			seen[destination] = true
			out = append(out, destination)
		}
	}
	return out
}

func fileRefDestination(t *testing.T, destinations []string, fragment string) *url.URL {
	t.Helper()
	for _, destination := range destinations {
		u, err := url.Parse(destination)
		if err != nil {
			t.Fatalf("文件目的地不是 URL：%q: %v", destination, err)
		}
		if u.Scheme == "file" && u.Fragment == fragment {
			return u
		}
	}
	t.Fatalf("缺少 fragment=%q 的文件链接：%q", fragment, destinations)
	return nil
}

func TestFileRefsKnownRelativeAndAbsolutePathsBecomeFileLinks(t *testing.T) {
	cwd := t.TempDir()
	absolute := filepath.Join(t.TempDir(), "绝对 文件.go")
	relative := "docs/交付 文件.md"
	assistant := "已更新 " + relative + "#L24-L30，另见 " + absolute + "#L7-L9。\n\n" +
		"未知 docs/未写入.md 与普通文本保持原样；前缀X" + relative + " 也不猜。\n\n" +
		"URL https://example.com/docs/%E4%BA%A4%E4%BB%98%20%E6%96%87%E4%BB%B6.md 不变成文件。\n\n" +
		"`" + relative + "`\n\n```text\n" + relative + "\n" + absolute + "\n```\n"
	data := fileRefsData(cwd,
		fileRefsToolMessage("write", "file_path", relative),
		fileRefsToolMessage("edit", "filePath", absolute),
		apiclient.Message{Role: "assistant", Text: assistant},
	)
	p := newStreamingMarkdownTestPlugin()
	canvas := drawStreamingMarkdownTest(t, p, data)

	const assistantIndex = 2
	destinations := fileRefDestinations(p, assistantIndex)
	if len(destinations) != 2 {
		t.Fatalf("只有正文中的两个已知路径应成为文件链接，得到 %d：%q；texts=%q", len(destinations), destinations, sessionTextValues(canvas.Texts()))
	}
	relativeURL := fileRefDestination(t, destinations, "L24-L30")
	wantRelativePath := "/" + filepath.ToSlash(filepath.Join(cwd, relative))
	if relativeURL.Path != wantRelativePath {
		t.Fatalf("相对路径没有按 Snapshot.CWD 解析：got=%q want=%q", relativeURL.Path, wantRelativePath)
	}
	absoluteURL := fileRefDestination(t, destinations, "L7-L9")
	if absoluteURL.Path != "/"+filepath.ToSlash(absolute) {
		t.Fatalf("绝对路径被改写：got=%q want=%q", absoluteURL.Path, "/"+filepath.ToSlash(absolute))
	}
	view := &p.views[assistantIndex]
	plain := view.SelectedText(components.MarkdownSelection{Anchor: 0, Focus: view.TextLen()})
	for _, visible := range []string{relative + "#L24-L30", absolute + "#L7-L9", "docs/未写入.md", "前缀X" + relative} {
		if !strings.Contains(plain, visible) {
			t.Fatalf("链接化后可选择原文缺少 %q：%q", visible, plain)
		}
	}
}

func TestFileRefsMissingCwdDoesNotGuessRelativePath(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "absolute.txt")
	data := fileRefsData("",
		fileRefsToolMessage("write", "path", "relative.txt"),
		fileRefsToolMessage("write", "path", absolute),
		apiclient.Message{Role: "assistant", Text: "relative.txt 不可猜；" + absolute + " 可以打开。"},
	)
	p := newStreamingMarkdownTestPlugin()
	drawStreamingMarkdownTest(t, p, data)
	destinations := fileRefDestinations(p, 2)
	if len(destinations) != 1 || !strings.HasPrefix(destinations[0], "file:") {
		t.Fatalf("缺 CWD 时只应保留绝对路径链接：%q", destinations)
	}
	u, err := url.Parse(destinations[0])
	if err != nil || u.Path != "/"+filepath.ToSlash(absolute) {
		t.Fatalf("绝对路径目的地错误：%q, %v", destinations[0], err)
	}
}

func TestFileRefsMarkdownCacheReusesFramesAndTracksCWD(t *testing.T) {
	p := newStreamingMarkdownTestPlugin()
	canvas := &gfx.Canvas{}
	canvas.SetTree(&gfx.Tree{})
	measurements := 0
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		measurements++
		return float32(len([]rune(value))) * size
	})
	data := fileRefsData(t.TempDir(),
		fileRefsToolMessage("write", "filePath", "cached.go"),
		apiclient.Message{Role: "assistant", Text: "cached.go#L4"},
	)
	p.layoutAssistantMarkdown(canvas, 1, data.Msgs[1].Text, 860, data)
	first := measurements
	if first == 0 {
		t.Fatal("首次文件链接 Markdown 排版没有测量文本")
	}
	for range 8 {
		p.layoutAssistantMarkdown(canvas, 1, data.Msgs[1].Text, 860, data)
	}
	if measurements != first {
		t.Fatalf("正文/宽度/CWD/已知路径未变仍重复排版：first=%d after=%d", first, measurements)
	}
	data.CWD = t.TempDir()
	p.layoutAssistantMarkdown(canvas, 1, data.Msgs[1].Text, 860, data)
	destinations := fileRefDestinations(p, 1)
	if len(destinations) != 1 {
		t.Fatalf("CWD 改变后文件链接丢失：%q", destinations)
	}
	u, err := url.Parse(destinations[0])
	if err != nil || u.Path != "/"+filepath.ToSlash(filepath.Join(data.CWD, "cached.go")) {
		t.Fatalf("CWD 改变后仍指向旧绝对路径：%q, %v", destinations[0], err)
	}
}

func TestFileRefsUnknownTextURLsAndCodeAreNeverPromotedToFiles(t *testing.T) {
	cwd := t.TempDir()
	known := "known.go"
	for _, test := range []struct {
		name string
		text string
	}{
		{name: "普通文本与未知路径", text: "普通字符串 other.go；unknown/known.go；prefix" + known},
		{name: "URL", text: "https://example.com/" + known + "?next=" + known},
		{name: "行内码", text: "`" + known + "`"},
		{name: " fenced code", text: "```go\n" + known + "\n```"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := fileRefsData(cwd,
				fileRefsToolMessage("write", "filePath", known),
				apiclient.Message{Role: "assistant", Text: test.text},
			)
			p := newStreamingMarkdownTestPlugin()
			drawStreamingMarkdownTest(t, p, data)
			if got := fileRefDestinations(p, 1); len(got) != 0 {
				t.Fatalf("不得从普通文本、URL 或代码猜文件链接：%q", got)
			}
		})
	}
}
