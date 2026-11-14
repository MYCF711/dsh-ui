package dsconversation

// usertext_test.go —— 用户消息里的引用/凭据识别与分段。
//
// 纯函数部分钉住语法（官方 wire 形式 + 文件 mention 的边界），
// 布局/点击在 usertext_visual_test.go 里用一帧真画布验。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestParseReferenceSpans(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		want  []string // 期望的 label 序列
		kinds []refKind
	}{
		{"文件", "看下 @plugins/dsopen/open.go 谢谢", []string{"open.go"}, []refKind{refFile}},
		{"目录", "进 @plugins/dsopen/ 看看", []string{"dsopen"}, []refKind{refFolder}},
		{"引号路径", `打开 @"a b/c.txt" 吧`, []string{"c.txt"}, []refKind{refFile}},
		{"会话 wire", "参考 @[上一次对话](dsh-session:s-2) 的结论", []string{"上一次对话"}, []refKind{refSession}},
		{"邮箱不是引用", "发到 a@b.com", nil, nil},
		{"普通文本", "没有引用", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := parseReferenceSpans(tc.text)
			if len(spans) != len(tc.want) {
				t.Fatalf("命中 %d 条，want %d：%+v", len(spans), len(tc.want), spans)
			}
			for i := range spans {
				if spans[i].Label != tc.want[i] {
					t.Errorf("第 %d 条 label = %q，want %q", i, spans[i].Label, tc.want[i])
				}
				if spans[i].Kind != tc.kinds[i] {
					t.Errorf("第 %d 条 kind = %v，want %v", i, spans[i].Kind, tc.kinds[i])
				}
			}
		})
	}
}

func TestUserInlineSegmentsKeepsOrderAndSecretWins(t *testing.T) {
	// 凭据与引用在同一段：凭据优先（引用不再画 chip）。
	text := "key sk-" + strings.Repeat("a", 24) + "012345 和 @a.go"
	segs := userInlineSegments(text)
	if len(segs) == 0 {
		t.Fatal("没有分段")
	}
	var hasSecret, hasRef, hasText bool
	for _, seg := range segs {
		switch seg.kind {
		case segSecret:
			hasSecret = true
		case segReference:
			hasRef = true
		case segText:
			hasText = true
		}
	}
	if !hasSecret || !hasRef || !hasText {
		t.Fatalf("分段不全：secret=%v ref=%v text=%v（%+v）", hasSecret, hasRef, hasText, segs)
	}
	// 顺序：拼接各段 raw 必须还原成原文。
	var rebuilt strings.Builder
	for _, seg := range segs {
		switch seg.kind {
		case segText:
			rebuilt.WriteString(seg.text)
		case segSecret:
			rebuilt.WriteString(seg.secret.Raw)
		case segReference:
			rebuilt.WriteString(seg.ref.Ref)
		}
	}
	if rebuilt.String() != text {
		t.Fatalf("分段拼接没有还原原文：%q vs %q", rebuilt.String(), text)
	}
}

func TestSecretMaskLengthIsPreservedForInlineLayout(t *testing.T) {
	text := "key=" + "sk-" + strings.Repeat("a", 20) + "0123"
	segs := userInlineSegments(text)
	for _, seg := range segs {
		if seg.kind != segSecret {
			continue
		}
		if len(seg.secret.Masked) != len(seg.secret.Raw) {
			t.Fatalf("遮罩改了长度（%d → %d），行内排版会跳", len(seg.secret.Raw), len(seg.secret.Masked))
		}
	}
}

func TestHasUserInline(t *testing.T) {
	if hasUserInline("普通一句话") {
		t.Fatal("普通文本被当成行内富文本")
	}
	if !hasUserInline("看 @a.go") {
		t.Fatal("文件引用没被识别")
	}
	if !hasUserInline("key sk-" + strings.Repeat("a", 20) + "0123") {
		t.Fatal("凭据没被识别")
	}
}

// TestMessageSlotKeyUsesRealIdentity 钉住"眼睛开关的键不靠消息下标"。
func TestMessageSlotKeyUsesRealIdentity(t *testing.T) {
	p := &Plugin{}
	env := &pluginapi.Env{Data: pluginapi.Snapshot{Cur: "s-1"}}
	v4 := int64(7)
	a := p.messageSlotKey(env, apiclient.Message{V4Seq: &v4, Seq: 3})
	b := p.messageSlotKey(env, apiclient.Message{V4Seq: &v4, Seq: 99})
	if a != b {
		t.Fatalf("同一条 v4 消息因为 load seq 不同生成了两个键：%q vs %q", a, b)
	}
	x := p.messageSlotKey(env, apiclient.Message{MessageID: "m-1"})
	y := p.messageSlotKey(env, apiclient.Message{MessageID: "m-2"})
	if x == y {
		t.Fatal("两条不同消息生成了同一个键")
	}
}
