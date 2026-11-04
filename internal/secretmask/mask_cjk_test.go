package secretmask

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 这个文件是 2026-11-04 那个真缺陷的回归。
//
// 缺陷：`Bearer ` 后面的候选只按**非空白 + 字节数 ≥ 20** 判，于是中文散文被当成
// 凭据遮掉，而且遮罩按字节切首尾，把汉字从中间劈开、输出不是合法 UTF-8。
//
//	输入： 用 Bearer 认证就行，不要用 Basic
//	输出： 用 Bearer 认证**************\x81用 Basic
//
// 原来的 7 条单测全绿，因为没有一条用中文——而**这个项目里的用户就是说中文的**。
// 所以这里的第一条不是"补充边界"，是把漏掉的那一类输入补上。

// TestKeepsChineseAfterBearer 是那个被报出来的句子，逐字。
func TestKeepsChineseAfterBearer(t *testing.T) {
	const in = "用 Bearer 认证就行，不要用 Basic"
	if got := Mask(in); got != in {
		t.Fatalf("中文散文被遮了：\n  输入 %q\n  输出 %q", in, got)
	}
}

// TestKeepsLongChineseAfterBearer 是关键的一条：句子里**没有被空格分开的长中文**。
//
// 上一条只有 8 个汉字，靠"按 rune 数"也能救回来；这一条有 25 个汉字、75 字节，
// 光把字节数换成字符数**不够**——必须补上"候选得是 ASCII"。
// 写回归的时候容易只测短的那条，然后把一个仍然坏着的实现标成修好了。
func TestKeepsLongChineseAfterBearer(t *testing.T) {
	const in = "用 Bearer 认证就行这个方法最省事而且不需要额外配置任何东西"
	if got := Mask(in); got != in {
		t.Fatalf("长中文散文被遮了：\n  输入 %q\n  输出 %q", in, got)
	}
}

// TestBearerStillMasksRealTokens 是上两条的**正向对照**。
//
// 没有它的话，把 maskBearer 整个删掉（"永远不遮"）也能让上面两条绿——
// 而那就从"多遮"翻到了"少遮"，是同一个错的反面。
func TestBearerStillMasksRealTokens(t *testing.T) {
	// 真凭据是 ASCII。造一个明显是假的、但形状对的。
	const tok = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.fake-payload"
	got := Mask("Authorization: Bearer " + tok)
	if strings.Contains(got, tok) {
		t.Fatalf("真形状的 token 没被遮：%q", got)
	}
	if !strings.Contains(got, "*") {
		t.Fatalf("遮完没有星号：%q", got)
	}
}

// TestNeverEmitsInvalidUTF8 钉住"输出总是合法 UTF-8"。
//
// 遮罩的产物直接进渲染层，一个劈开的汉字在那里是什么表现取决于它心情。
// 与其事后猜渲染层怎么处理非法字节，不如在这里保证不会产生。
func TestNeverEmitsInvalidUTF8(t *testing.T) {
	samples := []string{
		"用 Bearer 认证就行，不要用 Basic",
		"用 Bearer 认证就行这个方法最省事而且不需要额外配置任何东西",
		"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig",
		"key=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		"我的 key 是 sk-abcdefghijklmnopqrstuvwxyz0123 别外传",
		"中文Bearer中文中文中文中文中文中文中文中文中文中文中文",
		"",
	}
	for _, in := range samples {
		got := Mask(in)
		if !utf8.ValidString(got) {
			t.Errorf("输出不是合法 UTF-8：%q → %q", in, got)
		}
	}
}

// TestMaskLengthIsStable 钉住"遮罩不改变字节长度"。
//
// `plugins/dsconversation` 依赖这一条：它的虚拟化按长度估算消息高度、
// 按遮罩后的文本绘制，两者只有长度相同才必然量出同一个高度。
// 现在成立是因为"留首尾 6+4、中间星号一换一"，而且被遮的一定是 ASCII。
func TestMaskLengthIsStable(t *testing.T) {
	for _, in := range []string{
		"key=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig",
		"我的 key 是 sk-abcdefghijklmnopqrstuvwxyz0123 别外传",
		"用 Bearer 认证就行，不要用 Basic",
	} {
		if got := Mask(in); len(got) != len(in) {
			t.Errorf("长度变了（%d → %d）：%q → %q", len(in), len(got), in, got)
		}
	}
}
