// Package secretmask 把文本里**看起来像凭据**的部分遮掉。
//
// 官方对应物：`@deepseek-ai/dsh-client-ui-secret-mask`。官方按**发布格式**识别：
// provider key / token 前缀、JWT、私钥块、带口令的 URL（`credential-formats.ts:1-9`），
// 每个命中是一条独立的 `secret` 节点，界面可以逐值显示/再隐藏
// （`SecretText.tsx:44-80`）。本包只做**识别 + 生成遮罩文本**；交互在
// `plugins/dsconversation` 的那一层。
//
// # 一条重要的边界：宁可少遮，不可多遮
//
// 多遮的后果是**正常的聊天内容被吞掉**（用户说到 "Bearer" 这个词就被打成星号），
// 少遮的后果是**个别凭据露出来**。这两个都坏，但前者**每次都会发生**、
// 后者只在真出现凭据时才发生。所以规则要**窄**：只认有明确格式的串。
//
// # 为什么还有一条"高熵"兜底
//
// 官方只认发布格式；本仓保留原来的高熵长串规则作为额外一层（长度 ≥32、
// 同时有大小写与数字）。它比官方**多遮**一些，但只在极端形状上命中，
// 并且有 `TestKeepsNormalText` 一组反证钉住常见文本不被误伤。
package secretmask

import "strings"

// Kind 是一条命中属于官方哪一类格式。
type Kind int

const (
	// KindPrefixed 是带固定前缀的 provider key / token（sk- / ghp_ / eyJ…）。
	KindPrefixed Kind = iota
	// KindKeyBlock 是 PEM 私钥块（保留 BEGIN/END 行，正文遮掉）。
	KindKeyBlock
	// KindAddress 是 URL 里的口令（`scheme://user:password@host`，只遮口令）。
	KindAddress
	// KindBearer 是 `Bearer <token>` 里的 token（本仓额外规则）。
	KindBearer
	// KindHighEntropy 是无前缀但够长且高熵的 token（本仓额外规则）。
	KindHighEntropy
)

// Span 是原文里的一条凭据范围（半开区间，字节偏移）。
//
// Raw 与 Masked 是同一段原文与它的显示形。Masked 保证**与 Raw 等长**：
// 界面按长度估高，长度一变就会和绘制错位（见 dsconversation 的 mask_test.go）。
type Span struct {
	Start  int
	End    int
	Kind   Kind
	Raw    string
	Masked string
}

// keepHead / keepTail 是 prefixed / bearer / 高熵三条规则的保留首尾长度。
//
// 留首尾是为了**用户还能认出这是哪一个密钥**（"sk-…a1b2 那个"），
// 全打星号会让"我想确认是不是用错了 key"变成不可能。
// 私钥块与 URL 口令不走这条：前者保留 BEGIN/END 行已足够定位，后者必须整段遮。
const (
	keepHead = 6
	keepTail = 4
)

// minEntropyLen 是一条无前缀串要走"高熵"判定时的最短长度。
const minEntropyLen = 32

// maxScanWork 是单次扫描最多尝试的格式候选数（防止极端长文本的 O(n²)）。
// 聊天消息通常只有几百字节；这个上限只是给粘贴的巨型日志一个护栏。
const maxScanWork = 1 << 16

// Mask 返回遮过之后的文本。没有命中时原样返回。
func Mask(s string) string {
	spans := Spans(s)
	if len(spans) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, sp := range spans {
		if sp.Start < last || sp.Start >= sp.End || sp.End > len(s) {
			continue
		}
		b.WriteString(s[last:sp.Start])
		b.WriteString(sp.Masked)
		last = sp.End
	}
	if last < len(s) {
		b.WriteString(s[last:])
	}
	return b.String()
}

// Spans 返回文本里全部凭据范围，按 Start 升序、互不重叠。
//
// 先按官方格式表扫一遍，再补两条本仓额外规则（Bearer / 高熵）；
// 重叠的候选合并到最早那条（官方 `match` 的同一句："No credential is drawn
// half masked"）。
func Spans(s string) []Span {
	if s == "" {
		return nil
	}
	spans := scanFormats(s)
	spans = append(spans, scanBearer(s)...)
	spans = append(spans, scanHighEntropy(s, spans)...)
	return normalize(s, spans)
}

// normalize 排序、合并重叠、生成 Masked。
func normalize(s string, spans []Span) []Span {
	if len(spans) == 0 {
		return nil
	}
	// 插入排序：命中数量极少，不需要 sort 包。
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].Start < spans[j-1].Start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
	out := spans[:0]
	for _, sp := range spans {
		if sp.Start < 0 || sp.End <= sp.Start || sp.End > len(s) {
			continue
		}
		if len(out) > 0 && sp.Start < out[len(out)-1].End {
			// 重叠：并到最早那条；保留较早的 Kind（它决定怎么遮）。
			if sp.End > out[len(out)-1].End {
				out[len(out)-1].End = sp.End
			}
			continue
		}
		out = append(out, sp)
	}
	for i := range out {
		out[i].Raw = s[out[i].Start:out[i].End]
		out[i].Masked = maskSpan(s, out[i])
	}
	return out
}

// maskSpan 按 Kind 生成与原文等长的遮罩。
func maskSpan(s string, sp Span) string {
	switch sp.Kind {
	case KindKeyBlock:
		return maskKeyBlock(s, sp.Start, sp.End)
	case KindAddress:
		return maskAll(s[sp.Start:sp.End])
	default:
		return maskOne(s[sp.Start:sp.End])
	}
}

// maskOne 保留首尾、中间打星号（按 rune 切，不按字节切）。
func maskOne(w string) string {
	r := []rune(w)
	if len(r) <= keepHead+keepTail {
		return strings.Repeat("*", len(r))
	}
	return string(r[:keepHead]) +
		strings.Repeat("*", len(r)-keepHead-keepTail) +
		string(r[len(r)-keepTail:])
}

// maskAll 整段替换为星号，保留换行（不保留任何原字符）。
func maskAll(w string) string {
	var b strings.Builder
	b.Grow(len(w))
	for _, r := range w {
		if r == '\n' || r == '\r' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('*')
	}
	return b.String()
}

// maskKeyBlock 保留 `-----BEGIN…` 首行与 `-----END…` 行，正文逐字符遮掉；
// 单行块（没有换行）整段遮掉。
func maskKeyBlock(s string, start, end int) string {
	body := s[start:end]
	nl := strings.IndexByte(body, '\n')
	if nl < 0 {
		return maskAll(body)
	}
	bodyStart := start + nl + 1
	bodyEnd := end
	// 从后往前找最后一个 "-----END " 行首。
	for i := strings.LastIndex(body, "\n-----END "); i >= 0; i = strings.LastIndex(body[:i], "\n-----END ") {
		bodyEnd = start + i
		break
	}
	if bodyEnd < bodyStart {
		bodyEnd = end
	}
	return s[start:bodyStart] + maskAll(s[bodyStart:bodyEnd]) + s[bodyEnd:end]
}

// ---------------------------------------------------------------------------
// 官方格式表（`dsh-client-ui-secret-mask/src/client/credential-formats.ts`）
// ---------------------------------------------------------------------------

// runClass 是前缀后面允许的字符类。
type runClass int

const (
	classKey runClass = iota
	classAlnum
	classWord
	classAlnumDash
	classUpperAlnum
)

// shape 是前缀后的形状校验方式。
type shape int

const (
	shapeRun shape = iota
	shapeBounded
	shapeSK
	shapeGoogle
	shapeJWT
)

type format struct {
	literal string
	shape   shape
	run     runClass
	min     int
	max     int
}

// formats 逐条对照官方 `CREDENTIAL_FORMATS`（顺序与值都一样；
// 较长的字面量先试，避免 `sk-` 吃掉 `sk_live_`）。
var formats = []format{
	{literal: "-----BEGIN ", shape: shapeRun, run: classKey, min: 0},
	{literal: "://", shape: shapeRun, run: classKey, min: 0},
	{literal: "sk-", shape: shapeSK, run: classKey, min: 20},
	{literal: "AIza", shape: shapeGoogle, run: classKey, min: 35},
	{literal: "ghp_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "gho_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "ghu_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "ghs_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "ghr_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "github_pat_", shape: shapeRun, run: classWord, min: 22},
	{literal: "glpat-", shape: shapeRun, run: classKey, min: 20},
	{literal: "npm_", shape: shapeRun, run: classAlnum, min: 36},
	{literal: "hf_", shape: shapeRun, run: classAlnum, min: 30},
	{literal: "AKIA", shape: shapeBounded, run: classUpperAlnum, min: 16, max: 16},
	{literal: "ASIA", shape: shapeBounded, run: classUpperAlnum, min: 16, max: 16},
	{literal: "LTAI", shape: shapeBounded, run: classAlnum, min: 12, max: 20},
	{literal: "AKID", shape: shapeBounded, run: classAlnum, min: 28, max: 36},
	{literal: "xoxa-", shape: shapeRun, run: classAlnumDash, min: 10},
	{literal: "xoxb-", shape: shapeRun, run: classAlnumDash, min: 10},
	{literal: "xoxp-", shape: shapeRun, run: classAlnumDash, min: 10},
	{literal: "xoxr-", shape: shapeRun, run: classAlnumDash, min: 10},
	{literal: "xoxs-", shape: shapeRun, run: classAlnumDash, min: 10},
	{literal: "sk_live_", shape: shapeRun, run: classAlnum, min: 16},
	{literal: "sk_test_", shape: shapeRun, run: classAlnum, min: 16},
	{literal: "rk_live_", shape: shapeRun, run: classAlnum, min: 16},
	{literal: "rk_test_", shape: shapeRun, run: classAlnum, min: 16},
	{literal: "whsec_", shape: shapeRun, run: classAlnum, min: 24},
	{literal: "eyJ", shape: shapeJWT, run: classKey, min: 8, max: 2048},
}

// literalOrder 是 formats 的下标，按字面量长度降序；同长保持原顺序。
func literalOrder() []int {
	order := make([]int, len(formats))
	for i := range order {
		order[i] = i
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && len(formats[order[j]].literal) > len(formats[order[j-1]].literal); j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	return order
}

var order = literalOrder()

// scanFormats 扫官方格式表的全部命中。
func scanFormats(s string) []Span {
	var out []Span
	work := 0
	for i := 0; i < len(s); {
		if work > maxScanWork {
			break
		}
		work++
		// 首字节剪枝：只有这些字节可能开启一条官方格式。没有它，每个字符都要
		// 做 28 次 HasPrefix；聊天消息每帧都要扫，长会话里那是可见的开销。
		if !formatStartByte(s[i]) {
			i++
			continue
		}
		matched := false
		for _, fi := range order {
			f := formats[fi]
			if !strings.HasPrefix(s[i:], f.literal) {
				continue
			}
			// 前缀格式要求左边不是 word 字符（官方 `isWordAt(at-1)`）；
			// `-----BEGIN ` 与 `://` 不适用：前者的左边是空白/标点，后者的左边
			// 按定义就是 scheme 的最后一个字母。
			if i > 0 && f.literal != "://" && f.literal != "-----BEGIN " && isWordByte(s[i-1]) {
				continue
			}
			if f.literal == "-----BEGIN " {
				if end := keyBlockEnd(s, i); end > i {
					out = append(out, Span{Start: i, End: end, Kind: KindKeyBlock})
					i, matched = end, true
					break
				}
				continue
			}
			if f.literal == "://" {
				if st, end := addressPassword(s, i); end > st {
					out = append(out, Span{Start: st, End: end, Kind: KindAddress})
					i, matched = end, true
					break
				}
				continue
			}
			if end := prefixedEnd(s, f, i); end > i {
				out = append(out, Span{Start: i, End: end, Kind: KindPrefixed})
				i, matched = end, true
				break
			}
		}
		if !matched {
			i++
		}
	}
	return out
}

// formatStartByte 报告 c 是否可能是某个格式字面量的首字节。
func formatStartByte(c byte) bool {
	switch c {
	case '-', ':', 's', 'A', 'g', 'n', 'h', 'L', 'x', 'r', 'w', 'e':
		return true
	}
	return false
}

// prefixedEnd 校验 `literal` 在 `at` 开始的候选，返回结束偏移或 -1。
// 逐条对应官方 `CredentialScanner.prefixedEnd`。
func prefixedEnd(s string, f format, at int) int {
	start := at + len(f.literal)
	switch f.shape {
	case shapeSK:
		// `(?=[A-Za-z0-9_-]{0,255}\d)[A-Za-z0-9_-]{20,}`
		end := runEnd(s, start, classKey)
		if end-start < f.min {
			return -1
		}
		hasDigit := false
		for k := start; k < end && k < start+256; k++ {
			if s[k] >= '0' && s[k] <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return -1
		}
		return end
	case shapeGoogle:
		if start+f.min > len(s) || !allClass(s[start:start+f.min], classKey) {
			return -1
		}
		return start + f.min
	case shapeBounded:
		end := runEnd(s, start, f.run)
		length := end - start
		if length < f.min || length > f.max {
			return -1
		}
		if end < len(s) && isWordByte(s[end]) {
			return -1
		}
		return end
	case shapeJWT:
		first := runEnd(s, start, classKey)
		if first-start < f.min || first-start > f.max || !strings.HasPrefix(s[first:], ".eyJ") {
			return -1
		}
		payloadStart := first + len(".eyJ")
		payloadEnd := runEnd(s, payloadStart, classKey)
		if payloadEnd-payloadStart < f.min || payloadEnd >= len(s) || s[payloadEnd] != '.' {
			return -1
		}
		sigStart := payloadEnd + 1
		sigEnd := runEnd(s, sigStart, classKey)
		if sigEnd-sigStart < f.min {
			return -1
		}
		return sigEnd
	default: // shapeRun
		end := runEnd(s, start, f.run)
		if end-start < f.min {
			return -1
		}
		return end
	}
}

// keyBlockEnd 处理 `-----BEGIN [A-Z0-9 ]{0,40}PRIVATE KEY-----` 块。
func keyBlockEnd(s string, at int) int {
	start := at + len("-----BEGIN ")
	labelEnd := privateKeyLabelEnd(s, start)
	if labelEnd < 0 {
		return -1
	}
	for i := labelEnd; i < len(s); {
		idx := strings.Index(s[i:], "-----END ")
		if idx < 0 {
			return len(s)
		}
		line := i + idx
		// 只认行首的 END。
		if line == 0 || s[line-1] == '\n' || s[line-1] == '\r' {
			if end := privateKeyLabelEnd(s, line+len("-----END ")); end >= 0 {
				return end
			}
		}
		i = line + len("-----END ")
	}
	return len(s)
}

// privateKeyLabelEnd 匹配 `[A-Z0-9 ]{0,40}PRIVATE KEY-----`，返回结束偏移。
func privateKeyLabelEnd(s string, start int) int {
	limit := start + 52
	if limit > len(s) {
		limit = len(s)
	}
	i := start
	for i < limit {
		c := s[i]
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != ' ' {
			break
		}
		i++
	}
	const tail = "PRIVATE KEY"
	if i < start+len(tail) || !strings.HasPrefix(s[i:], "-----") {
		return -1
	}
	if s[i-len(tail):i] != tail {
		return -1
	}
	return i + 5
}

// addressPassword 处理 `[a-z][a-z0-9+.-]{1,15}://user:password@host`，
// 返回口令的 [start,end)；不合法时 end=0。
func addressPassword(s string, at int) (int, int) {
	if !schemeBefore(s, at) {
		return 0, 0
	}
	userStart := at + len("://")
	userEnd := runEndCustom(s, userStart, func(c byte) bool {
		return !isSpaceByte(c) && c != '/' && c != ':' && c != '@' && c != '?' && c != '#'
	})
	if userEnd-userStart > 256 || userEnd >= len(s) || s[userEnd] != ':' {
		return 0, 0
	}
	start := userEnd + 1
	end := runEndCustom(s, start, func(c byte) bool {
		return !isSpaceByte(c) && c != '/' && c != '@' && c != '?' && c != '#'
	})
	if end == start || end >= len(s) || s[end] != '@' {
		return 0, 0
	}
	return start, end
}

// schemeBefore 判断 `at` 前是否刚好是一个 `[a-z][a-z0-9+.-]{1,15}` scheme。
func schemeBefore(s string, at int) bool {
	i := at - 1
	for i >= 0 && at-i <= 16 {
		c := s[i]
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '+' && c != '.' && c != '-' {
			break
		}
		if c >= 'a' && c <= 'z' && at-i >= 2 && (i == 0 || !isWordByte(s[i-1])) {
			return true
		}
		i--
	}
	return false
}

// ---------------------------------------------------------------------------
// 本仓额外规则：Bearer 与高熵 token
// ---------------------------------------------------------------------------

// scanBearer 处理 `Bearer <token>`：只认紧跟的连续非空白、全 ASCII、长度 ≥20。
func scanBearer(s string) []Span {
	const kw = "Bearer "
	var out []Span
	for i := 0; ; {
		j := strings.Index(s[i:], kw)
		if j < 0 {
			break
		}
		j += i
		start := j + len(kw)
		end := start
		for end < len(s) && !isSpaceByte(s[end]) {
			end++
		}
		if end-start >= 20 && isASCII(s[start:end]) {
			out = append(out, Span{Start: start, End: end, Kind: KindBearer})
		}
		i = end
	}
	return out
}

// scanHighEntropy 找无前缀但长度 ≥32、同时有大小写与数字的 token。
func scanHighEntropy(s string, existing []Span) []Span {
	var out []Span
	for i := 0; i < len(s); {
		if !isTokenByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && isTokenByte(s[j]) {
			j++
		}
		if looksLikeHighEntropy(s[i:j]) && !insideAny(i, j, existing) {
			out = append(out, Span{Start: i, End: j, Kind: KindHighEntropy})
		}
		i = j
	}
	return out
}

func looksLikeHighEntropy(w string) bool {
	if len(w) < minEntropyLen {
		return false
	}
	var upper, lower, digit bool
	for i := 0; i < len(w); i++ {
		switch c := w[i]; {
		case c >= 'A' && c <= 'Z':
			upper = true
		case c >= 'a' && c <= 'z':
			lower = true
		case c >= '0' && c <= '9':
			digit = true
		}
	}
	return upper && lower && digit
}

func insideAny(start, end int, spans []Span) bool {
	for _, sp := range spans {
		if start < sp.End && end > sp.Start {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 字节类
// ---------------------------------------------------------------------------

func runEnd(s string, start int, class runClass) int {
	i := start
	for i < len(s) && classByte(s[i], class) {
		i++
	}
	return i
}

func runEndCustom(s string, start int, ok func(byte) bool) int {
	i := start
	for i < len(s) && ok(s[i]) {
		i++
	}
	return i
}

func allClass(s string, class runClass) bool {
	for i := 0; i < len(s); i++ {
		if !classByte(s[i], class) {
			return false
		}
	}
	return true
}

func classByte(c byte, class runClass) bool {
	alnum := c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
	switch class {
	case classKey:
		return alnum || c == '_' || c == '-'
	case classAlnum:
		return alnum
	case classWord:
		return alnum || c == '_'
	case classAlnumDash:
		return alnum || c == '-'
	case classUpperAlnum:
		return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z'
	}
	return false
}

func isWordByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_'
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isTokenByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' || c == '-' || c == '_'
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
