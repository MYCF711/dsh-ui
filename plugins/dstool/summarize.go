package dstool

import "strings"

// summarize 压成一行并截断（工具标题行只有一行，长参数必须收住）。
//
// 从 dsconversation 搬过来的，逐字未改：它是标题行的排版规则，属于工具卡。
func summarize(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	// n<=0 时没有"摘要"可言：返回 "…" 会误导成"有内容但被截断了"，
	// 而实际是调用方给的长度预算为零。返回空串，让调用方决定要不要画这一行。
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
