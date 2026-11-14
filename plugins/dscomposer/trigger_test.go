package dscomposer

// trigger_test.go —— 统一输入触发管道（`@` 文件/会话、`/` 技能）。
//
// 分两层：`detectTrigger` 的纯函数规则（对齐官方 `detect.ts`），
// 以及一帧真布局（菜单真的分组画出来、点得动、pick 落进草稿）。
//
// 反面样本比正面重要：`a@b.com`、`https://…`、`//` 里的斜杠都不能触发——
// 误触发会让 Enter 被菜单吃掉，用户发不出消息。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// canvasTextStrings 把画布上的文本抽成字符串（viva 的 Text 是结构体）。
func canvasTextStrings(canvas interface{ Texts() []gfx.Text }) []string {
	var out []string
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

func TestDetectTrigger(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		ok    bool
		char  byte
		query string
	}{
		{name: "行首 @", text: "@", ok: true, char: '@', query: ""},
		{name: "空格后 @", text: "看下 @tmp/a", ok: true, char: '@', query: "tmp/a"},
		{name: "引号 @", text: `@"a b`, ok: true, char: '@', query: "a b"},
		{name: "邮箱不触发", text: "a@b.com", ok: false},
		{name: "已结束的 token", text: "@tmp/a 然后", ok: false},
		{name: "斜杠技能", text: "/sk", ok: true, char: '/', query: "sk"},
		{name: "空格后斜杠", text: "帮我 /sk", ok: true, char: '/', query: "sk"},
		{name: "URL 不触发", text: "https://example.com", ok: false},
		{name: "协议相对首个斜杠仍触发（官方 boundary 只看左边）", text: "//example.com", ok: true, char: '/', query: "/example.com"},
		{name: "路径中斜杠不触发", text: "src/foo", ok: false},
		{name: "两个 token 取后者", text: "/a @b", ok: true, char: '@', query: "b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit, ok := detectTrigger(tc.text)
			if ok != tc.ok {
				t.Fatalf("detectTrigger(%q) ok = %v，want %v", tc.text, ok, tc.ok)
			}
			if !ok {
				return
			}
			if hit.Char != tc.char || hit.Query != tc.query {
				t.Errorf("detectTrigger(%q) = {%c %q}，want {%c %q}", tc.text, hit.Char, hit.Query, tc.char, tc.query)
			}
		})
	}
}

func TestAtMenuGroupsFilesAndSessions(t *testing.T) {
	act := pluginapi.Actions{
		FileReferences: func(sessionID, query string) []pluginapi.FileReferenceCandidate {
			return []pluginapi.FileReferenceCandidate{
				{Path: "plugins/dsopen/open.go"},
				{Path: "plugins/dsopen", IsDir: true},
			}
		},
		SessionReferenceCandidates: func(sessionID, query string) []pluginapi.SessionReferenceCandidate {
			return []pluginapi.SessionReferenceCandidate{{
				Mention: "@[上一次对话](dsh-session:s-2)", SessionID: "s-2", Label: "上一次对话",
			}}
		},
	}
	data := pluginapi.Snapshot{Cur: "s-1", CWD: `D:\ws`}
	p := &Plugin{}
	p.field.SetText("@")
	canvas := renderComposer(t, p, data, act, 1640, 1920, "")
	if len(p.triggerRows) != 3 {
		t.Fatalf("候选 = %d 行，want 3（2 文件 + 1 会话）：%v", len(p.triggerRows), p.triggerRows)
	}
	texts := strings.Join(canvasTextStrings(canvas), "\n")
	for _, want := range []string{zhTriggerFiles, zhTriggerSessions, "open.go", "上一次对话"} {
		if !strings.Contains(texts, want) {
			t.Errorf("菜单里没有 %q；实际文本：\n%s", want, texts)
		}
	}
}

func TestPickSessionInsertsWireMention(t *testing.T) {
	const mention = "@[上一次对话](dsh-session:s-2)"
	act := pluginapi.Actions{
		SessionReferenceCandidates: func(sessionID, query string) []pluginapi.SessionReferenceCandidate {
			return []pluginapi.SessionReferenceCandidate{{Mention: mention, SessionID: "s-2", Label: "上一次对话"}}
		},
	}
	p := &Plugin{}
	p.field.SetText("@上")
	renderComposer(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, 1640, 1920, "trigger-session-0")
	if got := p.field.Text(); got != mention {
		t.Fatalf("点会话候选后草稿 = %q，want %q", got, mention)
	}
	if p.suppressed != mention {
		t.Errorf("pick 之后应抑制同一份草稿重开菜单，suppressed=%q", p.suppressed)
	}
}

func TestSlashMenuPicksSkillWithTrailingSpace(t *testing.T) {
	act := pluginapi.Actions{
		Skills: func(sessionID string) []pluginapi.SkillCandidate {
			return []pluginapi.SkillCandidate{
				{Name: "go-build", Description: "构建 Go 项目"},
				{Name: "other"},
			}
		},
	}
	p := &Plugin{}
	p.field.SetText("/go")
	canvas := renderComposer(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, 1640, 1920, "")
	if len(p.triggerRows) != 1 || p.triggerRows[0].Label != "go-build" {
		t.Fatalf("`/go` 前缀筛出 %v，want 只有 go-build", p.triggerRows)
	}
	if !strings.Contains(strings.Join(canvasTextStrings(canvas), "\n"), "构建 Go 项目") {
		t.Errorf("技能说明没画出来：%v", canvasTextStrings(canvas))
	}
	renderComposer(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, 1640, 1920, "trigger-skill-0")
	if got := p.field.Text(); got != "/go-build " {
		t.Fatalf("pick 技能后草稿 = %q，want %q（官方落 `/<name> `）", got, "/go-build ")
	}
}

func TestDirectoryPickDrillsWithoutSuppressing(t *testing.T) {
	act := pluginapi.Actions{
		FileReferences: func(sessionID, query string) []pluginapi.FileReferenceCandidate {
			if query == "pl" {
				return []pluginapi.FileReferenceCandidate{{Path: "plugins", IsDir: true}}
			}
			return []pluginapi.FileReferenceCandidate{{Path: "plugins/dsopen/open.go"}}
		},
	}
	data := pluginapi.Snapshot{Cur: "s-1"}
	p := &Plugin{}
	p.field.SetText("@pl")
	renderComposer(t, p, data, act, 1640, 1920, "")
	if len(p.triggerRows) != 1 || !p.triggerRows[0].Drill {
		t.Fatalf("目录候选 = %v，want 一行 Drill", p.triggerRows)
	}
	if !p.acceptMention() {
		t.Fatal("有目录候选时 Enter 应被菜单接住")
	}
	if got := p.field.Text(); got != "@plugins/" {
		t.Fatalf("drill 后草稿 = %q，want @plugins/", got)
	}
	if p.suppressed != "" {
		t.Fatalf("目录 drill 不该抑制菜单（还要继续列下一层），suppressed=%q", p.suppressed)
	}
	renderComposer(t, p, data, act, 1640, 1920, "")
	if len(p.triggerRows) != 1 || p.triggerRows[0].Label != "open.go" {
		t.Fatalf("drill 之后应列出下一层，得到 %v", p.triggerRows)
	}
}

func TestDismissTriggerKeepsEnterUsable(t *testing.T) {
	act := pluginapi.Actions{
		FileReferences: func(sessionID, query string) []pluginapi.FileReferenceCandidate {
			return []pluginapi.FileReferenceCandidate{{Path: "abc.go"}}
		},
	}
	p := &Plugin{}
	p.field.SetText("@a")
	renderComposer(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, 1640, 1920, "")
	if len(p.triggerRows) == 0 {
		t.Fatal("样本本身没有菜单")
	}
	if !p.dismissTrigger() {
		t.Fatal("菜单开着时 Esc 应被接住")
	}
	if p.acceptMention() {
		t.Fatal("Esc 关掉菜单后 Enter 必须让回去（否则消息发不出去）")
	}
	// 同一份草稿不再重开。
	renderComposer(t, p, pluginapi.Snapshot{Cur: "s-1"}, act, 1640, 1920, "")
	if len(p.triggerRows) != 0 {
		t.Fatalf("dismiss 之后同一份草稿又重开了菜单：%v", p.triggerRows)
	}
}
