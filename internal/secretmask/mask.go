// Package secretmask 把文本里**看起来像凭据**的部分遮掉。
//
// 官方的说法是 "Web display masking for recognized credentials in sent user text"。
// 我们这一版只做"识别 + 遮"，不做"记住原值"——遮掉就是遮掉，
// 界面上没有"点一下看原文"的入口（那等于把防线又开了一个口子）。
//
// # 一条重要的边界：宁可少遮，不可多遮
//
// 多遮的后果是**正常的聊天内容被吞掉**（用户说到 "Bearer" 这个词就被打成星号），
// 少遮的后果是**个别凭据露出来**。这两个都坏，但前者**每次都会发生**、
// 后者只在真出现凭据时才发生。所以规则要**窄**：
// 只认有明确前缀的、以及足够长且高熵的串。
package secretmask

import "strings"

// minEntropyLen 是一条无前缀串要走"高熵"判定时的最短长度。
//
// 32 是经验值：常见 token（GitHub PAT 36、Anthropic 40+、OpenAI sk- 48+）都更长；
// 而正常单词很少连续 32 个字符全是字母数字且混着大小写与数字。
const minEntropyLen = 32

// keepHead / keepTail 是保留的首尾长度。
//
// 留首尾是为了**用户还能认出这是哪一个密钥**（"sk-…a1b2 那个"），
// 全打星号会让"我想确认是不是用错了 key"变成不可能。
const (
	keepHead = 6
	keepTail = 4
)

// prefixes 是有明确格式的凭据前缀。
//
// **只列真见过的**：每加一个前缀，就多一批"以它为开头的正常词"被误伤的风险。
var prefixes = []string{
	"sk-",  // OpenAI / DeepSeek 系
	"ghp_", // GitHub PAT（classic）
	"gho_", // GitHub OAuth
	"ghs_", // GitHub server-to-server
	"github_pat_",
	"AKIA",   // AWS access key id
	"xoxb-",  // Slack bot
	"xoxp-",  // Slack user
	"glpat-", // GitLab PAT
}

// Mask 返回遮过之后的文本。
func Mask(s string) string {
	if s == "" {
		return s
	}
	// 先处理 `Bearer xxx`：它没有固定前缀，但前后文很清楚。
	s = maskBearer(s)
	return maskTokens(s)
}

// maskBearer 处理 `Bearer <token>`。
//
// 只认 `Bearer ` 后面**紧跟的连续非空白**，长度 ≥ 20 才算——
// 否则 "Bearer 认证" 这种正常句子会被误伤。
func maskBearer(s string) string {
	const kw = "Bearer "
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		j := strings.Index(s[i:], kw)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		j += i
		b.WriteString(s[i : j+len(kw)])
		start := j + len(kw)
		end := start
		for end < len(s) && !isSpace(s[end]) {
			end++
		}
		if end-start >= 20 {
			b.WriteString(maskOne(s[start:end]))
		} else {
			b.WriteString(s[start:end])
		}
		i = end
	}
	return b.String()
}

// maskTokens 处理有前缀的凭据和"够长且高熵"的串。
func maskTokens(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if !isTokenByte(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		// 切出连续 token 字节（字母数字 + `-` + `_`）
		j := i
		for j < len(s) && isTokenByte(s[j]) {
			j++
		}
		word := s[i:j]
		if looksLikeSecret(word) {
			b.WriteString(maskOne(word))
		} else {
			b.WriteString(word)
		}
		i = j
	}
	return b.String()
}

// looksLikeSecret 判断一个"词"是不是凭据。
//
// 两条路，都要求**窄**：
//   - 命中已知前缀，且总长 ≥ 16（`sk-` 后面太短的多半是打字示例）
//   - 无前缀，但长度 ≥ 32 且**同时**有大小写和数字（高熵的样子）
func looksLikeSecret(w string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(w, p) && len(w) >= 16 {
			return true
		}
	}
	if len(w) < minEntropyLen {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for k := 0; k < len(w); k++ {
		switch c := w[k]; {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// maskOne 保留首尾、中间打星号。
func maskOne(w string) string {
	if len(w) <= keepHead+keepTail {
		return strings.Repeat("*", len(w))
	}
	return w[:keepHead] + strings.Repeat("*", len(w)-keepHead-keepTail) + w[len(w)-keepTail:]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isTokenByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' || c == '-' || c == '_'
}
