// Package vterm 是本仓对 `github.com/charmbracelet/x/vt` 的**唯一**依赖点。
//
// 为什么要这一层：上游现在是 **v0.1.0 单 tag**，且 `vt/*` 相关 PR 仍在密集合并
// （2026-11 那批 #1028–#1044）。把它挡在这里，API 变动只改这个文件，
// 不会渗到插件与渲染代码里。
//
// # 为什么需要 VT 模拟器
//
// `internal/ansi` 只做「去掉」控制序列（见该包注释），拿不到屏幕。后果有两层：
//
//  1. 界面：`vim` / `htop` 这类靠光标定位绘制的程序，剥掉序列后只剩乱序碎片；
//     `\r` 原地刷新的进度条会变成一串中间态。
//  2. **agent 读到的东西和官方不一样**：官方 `terminal_read` 读的是 xterm 的
//     屏幕缓冲，我们读到的是按写入顺序拼的行。agent 按「我看到了什么」做判断，
//     读到碎片就会误判（以为没输出、以为卡住）。
//
// 第 2 条才是这个包存在的首要理由 —— 不只是「界面像」。
//
// # 已知上游缺陷（登记，不在此处规避）
//
// charmbracelet/x#1039：**一次 grapheme cluster 被拆到两次 Write 里会坏**
// （天城文结合元音、ZWJ emoji、带重音拉丁、泰文受影响；**CJK 与区域指示符旗帜不受影响**）。
// 上游修复 PR #1044 于 2026-11-11 打开，尚未合并。
//
// 对本产品的影响：中文界面与普通命令输出**不受影响**；只有当终端里跑出上述
// 罕见文字的组合形式时才会看到错字。等 #1044 合并后升级即可，**不在这里做兜底**
// —— 按项目规则，上游问题不回灌成我们的私有 shim。
package vterm

import (
	"strings"

	"github.com/charmbracelet/x/vt"
)

// Emulator 是一块终端屏幕。
//
// 并发：与上游一致，Write 与查询不要并发调用；本仓的用法是「读取循环里 Write，
// 绘制时查询」，不需要额外加锁。
type Emulator struct {
	inner *vt.SafeEmulator
	// drained 是上游的应答流（程序发来的 DSR / DA 之类必须回，否则它会卡住）。
	// 这里丢弃应答，因为本包只做「看屏幕」；真要驱动程序请走 terminal_send 那条路。
	zombie bool
}

// New 造一块 w×h 的终端屏幕。
func New(w, h int) *Emulator {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	inner := vt.NewSafeEmulator(w, h)
	return &Emulator{inner: inner}
}

// Write 把 pty 的原始字节喂进去。
func (e *Emulator) Write(p []byte) (int, error) {
	return e.inner.Write(p)
}

// Resize 改变屏幕尺寸。
func (e *Emulator) Resize(w, h int) error {
	if w <= 0 || h <= 0 {
		return nil
	}
	e.inner.Resize(w, h) // 上游无返回值
	return nil
}

// String 返回**当前屏幕**的文本（按行，行尾空白已去掉）。
//
// 这是与 `internal/ansi` 剥序列后拼接最大的区别：返回的是「现在屏幕上写着什么」，
// 不是「曾经写过来什么」。
func (e *Emulator) String() string {
	return e.inner.String()
}

// Lines 返回当前屏幕的每一行（行尾空白去掉，空行保留以维持行号）。
func (e *Emulator) Lines() []string {
	rows := strings.Split(e.inner.String(), "\n")
	for i, row := range rows {
		rows[i] = strings.TrimRight(row, " ")
	}
	return rows
}

// CellText 返回 (x,y) 这个单元格的文字。
//
// 坐标是**屏幕绝对坐标**，左上角 (0,0)；越界返回空串。
// 一个单元格可能是一个 grapheme cluster（多个 rune），所以返回 string 而不是 rune。
func (e *Emulator) CellText(x, y int) string {
	if x < 0 || y < 0 {
		return ""
	}
	b := e.inner.Bounds()
	if x >= b.Max.X || y >= b.Max.Y {
		return ""
	}
	if c := e.inner.CellAt(x, y); c != nil {
		return c.Content
	}
	return ""
}

// CursorPosition 返回光标所在的绝对坐标。
func (e *Emulator) CursorPosition() (x, y int) {
	p := e.inner.CursorPosition()
	return p.X, p.Y
}

// CursorHidden 报告光标是否被程序隐藏。
func (e *Emulator) CursorHidden() bool {
	return e.inner.CursorHidden()
}

// Size 返回当前屏幕尺寸。
func (e *Emulator) Size() (w, h int) {
	b := e.inner.Bounds()
	return b.Dx(), b.Dy()
}
