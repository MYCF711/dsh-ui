package secretmask

import (
	"strings"
	"testing"
)

// TestMasksKnownPrefixes 验有明确前缀的凭据被遮。
func TestMasksKnownPrefixes(t *testing.T) {
	cases := []string{
		"sk-abcdef1234567890abcdef",
		"ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"AKIAIOSFODNN7EXAMPLE",
		"glpat-abcdefghijklmnopqrst",
	}
	for _, in := range cases {
		got := Mask("key=" + in + " end")
		if strings.Contains(got, in) {
			t.Errorf("没遮住 %q → %q", in, got)
		}
		if !strings.Contains(got, "*") {
			t.Errorf("%q 没有星号 → %q", in, got)
		}
	}
}

// TestMasksBearer 验 `Bearer <长token>` 被遮。
func TestMasksBearer(t *testing.T) {
	const tok = "abcdefghij0123456789XYZ"
	got := Mask("Authorization: Bearer " + tok)
	if strings.Contains(got, tok) {
		t.Fatalf("没遮住：%q", got)
	}
}

// TestKeepsShortBearer 验 `Bearer 认证` 这种正常句子**不被误伤**。
//
// 宁可少遮不可多遮：多遮是**每次都会发生**的，少遮只在真出现凭据时发生。
func TestKeepsShortBearer(t *testing.T) {
	const s = "这个接口用 Bearer 认证"
	if got := Mask(s); got != s {
		t.Fatalf("误伤了正常句子：%q → %q", s, got)
	}
}

// TestKeepsNormalText 验普通文本原样。
func TestKeepsNormalText(t *testing.T) {
	cases := []string{
		"hello world",
		"中文也能过",
		"docs/transport-contract.md",
		"go test ./... -count=1",
		"https://example.com/path?a=1",
		"这个 token 是一个词",
	}
	for _, in := range cases {
		if got := Mask(in); got != in {
			t.Errorf("误伤 %q → %q", in, got)
		}
	}
}

// TestMasksHighEntropyLongString 验无前缀但**长且高熵**的串被遮。
func TestMasksHighEntropyLongString(t *testing.T) {
	const s = "aB3dE5gH7jK9mN1pQ3sT5vW7yZ9bC1dE3f" // 32 位，大小写数字都有
	got := Mask("泄露了 " + s)
	if strings.Contains(got, s) {
		t.Fatalf("没遮住高熵串：%q", got)
	}
}

// TestKeepsShortMixedCase 验**不够长**的混合串不遮（比如普通变量名）。
func TestKeepsShortMixedCase(t *testing.T) {
	const s = "camelCaseVar123"
	if got := Mask(s); got != s {
		t.Fatalf("误伤：%q → %q", s, got)
	}
}

// TestKeepsHeadAndTail 验遮完还能认出是哪一个（首尾保留）。
func TestKeepsHeadAndTail(t *testing.T) {
	got := Mask("sk-abcdef1234567890abcdef")
	if !strings.HasPrefix(got, "sk-abc") {
		t.Fatalf("首部没保留：%q", got)
	}
	if !strings.HasSuffix(got, "cdef") {
		t.Fatalf("尾部没保留：%q", got)
	}
}
