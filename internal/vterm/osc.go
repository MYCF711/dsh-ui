package vterm

// 这个文件接上「程序改色」的那几个 OSC 序列（xterm 语义）：
//
//	OSC 4 ; index ; color (; index ; color)*   SET   改 256 调色板里某一项的色
//	OSC 104 (; index)*                         RESET 复位（不带参数 = 整张表）
//	OSC 10 / 11 / 12 (; color)+                SET   默认前景 / 背景 / 光标色
//	OSC 110 / 111 / 112                        RESET 上面三项
//
// # 上游到哪一步（判据，不猜）
//
// `charmbracelet/x/vt@v0.1.0`：
//
//   - `registerDefaultOscHandlers`（handlers.go:304-337）只注册了
//     0/1/2/7/8/10/11/12/110/111/112。**OSC 4 与 104 没有派发器**，落到
//     `osc.go:14-18` 那句 `logf("unhandled sequence: OSC %q")`——探针实测
//     （2026-11-19，`tmp/GO8-RC5/_curs/probe`）：喂 `\e]4;1;#ff0000\e\\` 之后
//     `IndexedColor(1)` 仍是 `ansi.IndexedColor(1)`，一个字都没改。
//   - 但**调色板本身是有的，而且是公开 API**：`IndexedColor` / `SetIndexedColor`
//     （emulator.go:422-446，字段 `e.colors[256]color.Color`），`SafeEmulator`
//     还各有一份带锁包装（safe_emulator.go:113-127）。
//   - `RegisterOscHandler(cmd, fn)`（handlers.go:62-68）就是给「上游没内置的 OSC」
//     留的公开挂载点；派发是逆序（handlers.go:146-157，后注册的先跑）。
//   - OSC 10/11/12/110/111/112 **上游自己就派发**（`osc.go:52-104` 的
//     `handleDefaultColor` + `emulator.go:339-419` 的 getter/setter）。
//
// 所以 **4/104 这一半不是「兜底」**：缺的只是一段派发，不是能力。用上游 API 把
// 派发接上，真值仍在上游那张表里——本包渲染层每解析一个索引色都问
// `IndexedColor`（见 cell.go 的 `indexed`），不存在第二份调色板副本。
//
// # 什么条件下可以删
//
// 上游在 `registerDefaultOscHandlers` 里自己派发 4/104 之后，删掉
// `registerPaletteOSC` 的那两个 handler 即可。`defaults()` 不受影响：10/11/12 的
// 状态本来就在上游，这一份只为回答「有没有被程序改过」。
//
// # 为什么 10/11/12 要自己再观察一遍
//
// 上游的 `ForegroundColor()` 在「没设置过」时回 `defaultFg`，而 OSC 110 复位做的是
// `fgColor = defaultFg`（emulator.go:347-352）——**getter 分不出「改过又复位」与
// 「从没改过」**，而这两种在官方是不同结果：复位后要落回 DSH 主题色，改过时用
// 程序给的颜色（terminal-theme.ts:67 `get cursor`、:73 `{...this.defaults, ...this.special}`）。
//
// 官方是怎么做的：`registerOscHandler` 挂一个**返回 false** 的观察者
// （terminal-theme.ts:20-21 `observe`），既记下值，又不吃掉 xterm 自己的处理。
// 这里同一形状：我们记状态，`return false` 让上游照旧跑它的 handler。

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// registerOSC 挂上本包的 OSC 观察者/派发器。构造时调一次，此后不再动
// （上游的 handlers map 不带锁）。
func (e *Emulator) registerOSC() {
	// OSC 4 / 104：上游没有派发器，这里**接上**并吃掉（返回 true），否则上游会
	// 把它当未处理序列记日志。
	e.inner.RegisterOscHandler(4, func(data []byte) bool { e.setPalette(data); return true })
	e.inner.RegisterOscHandler(104, func(data []byte) bool { e.resetPalette(data); return true })

	// OSC 10/11/12 与 110/111/112：上游有派发器（且比我们早注册，所以派发时
	// 我们排在前面）。只观察，返回 false 让它继续跑到上游那个 handler。
	for offset := range 3 {
		cmd, reset := 10+offset, 110+offset
		e.inner.RegisterOscHandler(cmd, func(data []byte) bool {
			e.observeDefaultColor(offset, data)
			return false
		})
		e.inner.RegisterOscHandler(reset, func([]byte) bool {
			e.clearDefaultColor(offset)
			return false
		})
	}
}

// setIndexed 改上游那张调色板里的第 i 项。
//
// **必须用不带锁的 `Emulator.SetIndexedColor`**：这些 handler 是在
// `SafeEmulator.Write` 持着写锁的情况下被派发的，再进 `SafeEmulator.SetIndexedColor`
// 会二次加同一把 `sync.RWMutex`——不可重入，直接死锁（2026-11-19 实测：
// 测试挂到 298s 超时）。
func (e *Emulator) setIndexed(i int, c color.Color) { e.inner.Emulator.SetIndexedColor(i, c) }

// oscColor 解析 OSC 里的颜色参数。
//
// 用上游同一个解析器（`handleDefaultColor` 在 osc.go:100 调的 `ansi.XParseColor`），
// 认 `#RGB` / `#RRGGBB` / `rgb:RRRR/GGGG/BBBB` / `rgba:…/…/…/…`。
func oscColor(s string) (color.Color, bool) {
	c := ansi.XParseColor(strings.TrimSpace(s))
	return c, c != nil
}

// oscParts 切 OSC 参数，并把命令号本身去掉。
//
// 上游给 handler 的 data **带命令号**（探针 2026-11-19 实测）：
//
//	OSC 4    → "4;1;#ff0000" / "4;1;#ff0000;2;#00ff00"
//	OSC 104  → "104" / "104;1;2"
//	OSC 10   → "10;#ff0000"
//
// 这一点与 xterm.js 的 `registerOscHandler` 不同（那边给的是 `4` 之后的原文），
// 所以不能照抄官方那段 split 的下标。
func oscParts(data []byte) []string {
	parts := strings.Split(string(data), ";")
	if len(parts) == 0 {
		return nil
	}
	return parts[1:]
}

// setPalette 处理 OSC 4：成对的 `index ; color`。
//
//   - 颜色是 `?` 或解析不出来的那一对整对跳过（查询我们不答，见文件头）；
//   - 下标越界由上游 `SetIndexedColor` 自己丢（emulator.go:440-446）。
func (e *Emulator) setPalette(data []byte) {
	parts := oscParts(data)
	for i := 0; i+1 < len(parts); i += 2 {
		index, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil || index < 0 || index > 255 {
			continue
		}
		c, ok := oscColor(parts[i+1])
		if !ok {
			continue
		}
		e.setIndexed(index, c)
	}
}

// resetPalette 处理 OSC 104：不带参数复位整张表，否则只复位列出的下标。
//
// 复位就是 `SetIndexedColor(index, nil)`——上游把 `e.colors[index]` 置 nil 之后
// `IndexedColor` 会回 `ansi.IndexedColor(index)`（「没设置过」），渲染层的
// `indexed` 据此落回官方默认表。所以这里不需要知道任何默认值。
func (e *Emulator) resetPalette(data []byte) {
	parts := oscParts(data)
	if len(parts) == 0 {
		for i := range 256 {
			e.setIndexed(i, nil)
		}
		return
	}
	for _, part := range parts {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || index < 0 || index > 255 {
			continue
		}
		e.setIndexed(index, nil)
	}
}

// oscDefaultSlots 是 OSC 10/11/12 各自能写到哪几档默认色：槽位 + i，
// 槽位 0 = 前景、1 = 背景、2 = 光标色（与 `Defaults` 的字段顺序一致）。
//
// 官方的形状（terminal-theme.ts:39-48）不是「OSC 10 只改前景」：它拿
// `specialKeys[offset+i]`，所以 `OSC 10;c1;c2;c3` 一次能写前景/背景/光标三项，
// `OSC 11;c1` 写的是背景、`OSC 12;c1` 写光标。这是 X11 的「dynamic colour N」
// 语义，官方照抄了；越界的那几项 `specialKeys[…]` 是 undefined，直接跳过。
const oscDefaultSlots = 3

// observeDefaultColor 记下 OSC 10/11/12 写的默认色（offset = 命令号 - 10）。
func (e *Emulator) observeDefaultColor(offset int, data []byte) {
	for i, part := range oscParts(data) {
		slot := offset + i
		if slot >= oscDefaultSlots {
			break
		}
		c, ok := oscColor(part)
		if !ok {
			continue
		}
		e.setDefaultSlot(slot, plainColor(c))
	}
}

// clearDefaultColor 复位 OSC 110/111/112 对应的一档。
func (e *Emulator) clearDefaultColor(offset int) { e.setDefaultSlot(offset, Color{}) }

func (e *Emulator) setDefaultSlot(slot int, c Color) {
	switch slot {
	case 0:
		e.osc.Fg = c
	case 1:
		e.osc.Bg = c
	case 2:
		e.osc.Cursor = c
	}
}

// defaults 报告程序通过 OSC 改过的默认色（Set=false = 没被改过）。
func (e *Emulator) defaults() Defaults { return e.osc }
