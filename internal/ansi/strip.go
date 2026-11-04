// Package ansi 处理终端输出里的控制序列。
//
// 为什么自己写：`go.mod` 里没有任何终端库（tcell / vt100 / ansi 都没有），
// 而界面要显示 shell 的输出就必然要处理这些字节。
//
// **这个包只做"去掉"**，不做"解释"——颜色、光标位置、清屏都不模拟。
// 理由：模拟终端是一个**无底洞**（VT100 已经几百条序列，再加上各家扩展）。
// 先把可读的文本拿出来，比画一个半对半错的彩色终端有用。
package ansi

import "strings"

// Strip 去掉 ANSI 转义序列，只留可读文本。
//
// 覆盖两类（这是 shell 输出里真正会出现的）：
//   - **CSI**：`ESC [ … 终结字节`（0x40–0x7E）。颜色、光标移动、清屏都走这条
//   - **OSC**：`ESC ] … BEL` 或 `ESC ] … ESC \`。改标题、设超链接走这条
//
// 其它 `ESC x` 双字节序列也一并丢弃——它们对"读文本"没有贡献。
//
// **不去 `\r`**：进度条（`\r` 覆盖同一行）的语义是"最后一次覆盖的结果"，
// 简单删 `\r` 会把 `10%\r50%\r100%` 变成 `10%50%100%`，比原样更糟。
// 那是渲染层的事（该按行取最后一个 `\r` 之后的部分），不是剥离层的。
func Strip(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != 0x1b { // 不是 ESC：原样留
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(s) {
			break // 末尾孤立的 ESC：丢掉
		}
		switch s[i+1] {
		case '[': // CSI
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++ // 吃掉终结字节
			}
			i = j - 1
		case ']': // OSC
			j := i + 2
			for j < len(s) {
				if s[j] == 0x07 { // BEL 结尾
					j++
					break
				}
				if s[j] == 0x1b && j+1 < len(s) && s[j+1] == 0x5c { // ESC 反斜杠结尾（0x5c）
					j += 2
					break
				}
				j++
			}
			i = j - 1
		default: // 其它双字节转义（ESC ( B、ESC = 之类）
			i++
		}
	}
	return b.String()
}

// LastLine 取 `\r` 覆盖之后的最终画面（单行进度条用）。
//
// 和 Strip 分开：Strip 不知道"这是终端"，它只做字节层面的剥离；
// `\r` 的语义只有"在终端上"才成立。
func LastLine(s string) string {
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		return s[i+1:]
	}
	return s
}
