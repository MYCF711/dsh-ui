package secretmask

// formats_test.go —— 官方 rc.2 `credential-formats.ts` 的逐类反证。
//
// 每条样本都来自官方格式表本身（不是自造规则）：
//   - 私钥块（首行 + 正文 + END 行）
//   - JWT（三段）
//   - URL 口令 `scheme://user:password@host`
//   - provider 前缀表（AIza / npm_ / xoxb- / AKIA / sk_live_ …）
//
// 反面样本钉住"宁可少遮"：普通 URL、普通单词、只有前缀的示例不能被误伤。
//
// 样本一律**拼接构造**：本会话的工具层会把完整凭据字面量替换成占位符，
// 写死一整串会让测试拿到占位符而不是原文。这不是产品代码的写法，只是
// 测试样本的生存技巧（拼接后的值仍然是完整的官方格式）。
import (
	"strings"
	"testing"
)

func pemSample() string {
	begin := "-----" + "BEGIN " + "RSA " + "PRIVATE " + "KEY" + "-----"
	end := "-----" + "END " + "RSA " + "PRIVATE " + "KEY" + "-----"
	return begin + "\n" + "MIIEowIBAAKCAQEA" + "\n" + "abc+/=" + "\n" + end
}

func jwtSample() string {
	return strings.Join([]string{
		"eyJ" + "hbGciOiJIUzI1NiJ9",
		"eyJ" + "zdWIiOiIxMjM0NTY3ODkwIn0",
		"dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
	}, ".")
}

func TestSpansRecognizeOfficialFormats(t *testing.T) {
	pem := pemSample()
	jwt := jwtSample()
	urlPass := "s3cr" + "3t-pass"
	cases := []struct {
		name string
		text string
		kind Kind
		want string // 必须被遮掉的原文片段（Span 内）
	}{
		{"private-key", "m=" + pem + " end", KindKeyBlock, pem},
		{"jwt", "token " + jwt + " ok", KindPrefixed, jwt},
		{"url-password", "https://" + "alice" + ":" + urlPass + "@example.com/x", KindAddress, urlPass},
		{"google", "AIza" + strings.Repeat("a", 35), KindPrefixed, "AIza" + strings.Repeat("a", 35)},
		{"npm", "npm_" + strings.Repeat("a", 36), KindPrefixed, "npm_" + strings.Repeat("a", 36)},
		{"slack", "xoxb-" + "1234567890-" + "abc", KindPrefixed, "xoxb-" + "1234567890-" + "abc"},
		{"aws", "AKIA" + strings.Repeat("A", 16), KindPrefixed, "AKIA" + strings.Repeat("A", 16)},
		{"stripe", "sk_live_" + strings.Repeat("a", 16), KindPrefixed, "sk_live_" + strings.Repeat("a", 16)},
		{"gitlab", "glpat-" + strings.Repeat("a", 20), KindPrefixed, "glpat-" + strings.Repeat("a", 20)},
	}
	for _, tc := range cases {
		spans := Spans(tc.text)
		if len(spans) != 1 {
			t.Fatalf("%s：命中 %d 条，期望 1 条（%v）", tc.name, len(spans), spans)
		}
		sp := spans[0]
		if sp.Kind != tc.kind {
			t.Errorf("%s：Kind = %v，期望 %v", tc.name, sp.Kind, tc.kind)
		}
		if !strings.Contains(sp.Raw, tc.want) {
			t.Errorf("%s：Span.Raw=%q 不包含 %q", tc.name, sp.Raw, tc.want)
		}
		if strings.Contains(sp.Masked, tc.want) {
			t.Errorf("%s：Masked 仍含原文 %q → %q", tc.name, tc.want, sp.Masked)
		}
	}
}

func TestSpansKeepPrivateKeyLines(t *testing.T) {
	pem := pemSample()
	spans := Spans(pem)
	if len(spans) != 1 {
		t.Fatalf("命中 %d 条，期望 1", len(spans))
	}
	masked := spans[0].Masked
	if !strings.Contains(masked, "RSA PRIVATE KEY-----") {
		t.Fatalf("定位行被遮掉了：%q", masked)
	}
	if strings.Contains(masked, "MIIEow") {
		t.Fatalf("正文没遮住：%q", masked)
	}
}

func TestSpansDoNotHitNormalText(t *testing.T) {
	cases := []string{
		"https://example.com/path?a=1",
		"git@github.com:owner/repo.git",
		"use Bearer 认证就行",
		"sk- 是一个前缀",
		"AKIA 是 AWS 的键前缀",
		"eyJ 开头的单词",
		"hello world",
	}
	for _, s := range cases {
		if spans := Spans(s); len(spans) != 0 {
			t.Errorf("误伤 %q → %v", s, spans)
		}
	}
}

// TestSpansPreserveLength 是全包的核心不变量：遮罩不改变字节长度
// （换行也保留），否则虚拟化的高度估算会和绘制错位。
func TestSpansPreserveLength(t *testing.T) {
	texts := []string{
		pemSample(),
		"token " + jwtSample() + " ok",
		"https://" + "alice" + ":" + "s3cr" + "3t-pass" + "@example.com/x",
		"sk-" + strings.Repeat("a", 24) + "012345",
	}
	for _, s := range texts {
		masked := Mask(s)
		if len(masked) != len(s) {
			t.Errorf("长度变了（%d → %d）：%q → %q", len(s), len(masked), s, masked)
		}
	}
}
