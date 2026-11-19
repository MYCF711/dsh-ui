package dsassets

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"github.com/vivaui/viva/gfx"
)

// ParsePath 把官方 SVG `d` 原文转成 viva 的路径命令。
//
// # 为什么是「先展开、后解析」
//
// 官方这套图标在 `d` 里用了 H / V（水平/垂直直线），个别是相对命令（小写 letter）。
// viva 的词汇只有 M/L/Q/C/Z，所以必须先把 H/V 等**无损**展开成绝对坐标——
// H 换成同一条水平直线、V 换成同一条竖直直线、`s` 按定义镜像上一段三次曲线的
// 控制点，都是恒等变形，形状一个像素都不变。
//
// 展开完就是一串纯 M/L/C/Z，交给 `dsbrand.ParseSVGPath`。**全仓只有那一个解析器**：
// 本包不复制第二份数字扫描/命令循环，避免"两个解析器对同一条路径给出两种结果"
// 这种最难查的问题。
//
// 支持：M m L l H h V v C c S s A a Z z（含隐式重复参数，如 `C a b c d e f g h i j k l`）。
//
// **椭圆弧 `A` 是真换算，不是近似**：viva 的词汇里没有弧，但任意椭圆弧都能切成
// ≤90° 的段、每段用一条三次贝塞尔表达，且误差有数学上界（每段径向误差 < 0.03% of r）。
// 换算与误差推导见 `arc.go` 的文件头。`large-arc` / `sweep` 两个标志按 SVG 1.1
// F.6.5 的符号规则参与圆心求解，不是"选一条看起来顺的弧"。
//
// 不支持：Q/q/T/t（二次曲线）。注意 viva **有** `gfx.QuadTo`——这里拒绝它们不是
// "viva 给不了"，而是本包尚未接入（登记为缺口，不是能力缺失）。官方现有素材一条
// 都没用到；将来真用到时这里会明着失败，而不是"画一半"。
func ParsePath(d string) ([]gfx.PathCommand, bool) {
	expanded, ok := expandPath(d)
	if !ok {
		return nil, false
	}
	return dsbrand.ParseSVGPath(expanded)
}

// expandPath 返回只含绝对 M/L/C/Z 的等价路径文本。
func expandPath(d string) (string, bool) {
	var (
		s       = d
		i       = 0
		n       = len(d)
		b       strings.Builder
		cx, cy  float64 // 当前点
		sx, sy  float64 // 子路径起点（Z 之后回到这里）
		cubicAt [2]float64
		hadC    bool // 上一条命令是不是 C/S（决定 s 的镜像控制点）
		started bool
	)
	b.Grow(len(d) + 16)

	skipSep := func() {
		for i < n {
			switch s[i] {
			case ' ', ',', '\n', '\r', '\t':
				i++
			default:
				return
			}
		}
	}
	num := func() (float64, bool) {
		start := i
		for i < n {
			c := s[i]
			if c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9') || c == 'e' || c == 'E' {
				i++
				continue
			}
			break
		}
		if start == i {
			return 0, false
		}
		v, err := strconv.ParseFloat(s[start:i], 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	// 写一个数字：`f` 格式保证不会出现指数形式，`-1` 精度保证十进制原文往返不变。
	put := func(v float64) {
		b.WriteByte(' ')
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	// flag 读 `A` 的两个标志位。SVG 规定它们是**单个字符** 0 或 1，可以写成
	// `0 1` 也可以紧贴成 `01`；用 num() 读会把 `01` 读成 1、把 `11` 读成 11，
	// 所以必须单独按单字符读。
	flag := func() (bool, bool) {
		skipSep()
		if i >= n || (s[i] != '0' && s[i] != '1') {
			return false, false
		}
		v := s[i] == '1'
		i++
		return v, true
	}
	more := func() bool {
		skipSep()
		return i < n && s[i] != 'Z' && s[i] != 'z' && !isCommandLetter(s[i])
	}

	for {
		skipSep()
		if i >= n {
			break
		}
		cmd := s[i]
		i++
		switch cmd {
		case 'M', 'm':
			first := true
			for {
				var x, y float64
				var ok bool
				if x, ok = num(); !ok {
					return "", false
				}
				skipSep()
				if y, ok = num(); !ok {
					return "", false
				}
				if cmd == 'm' {
					x += cx
					y += cy
				}
				if first {
					b.WriteString(" M")
					put(x)
					put(y)
					cx, cy = x, y
					sx, sy = x, y
					started = true
					first = false
				} else {
					b.WriteString(" L")
					put(x)
					put(y)
					cx, cy = x, y
				}
				hadC = false
				if !more() {
					break
				}
			}
		case 'L', 'l':
			if !started {
				return "", false
			}
			for {
				var x, y float64
				var ok bool
				if x, ok = num(); !ok {
					return "", false
				}
				skipSep()
				if y, ok = num(); !ok {
					return "", false
				}
				if cmd == 'l' {
					x += cx
					y += cy
				}
				b.WriteString(" L")
				put(x)
				put(y)
				cx, cy = x, y
				hadC = false
				if !more() {
					break
				}
			}
		case 'H', 'h':
			if !started {
				return "", false
			}
			for {
				x, ok := num()
				if !ok {
					return "", false
				}
				if cmd == 'h' {
					x += cx
				}
				b.WriteString(" L")
				put(x)
				put(cy)
				cx = x
				hadC = false
				if !more() {
					break
				}
			}
		case 'V', 'v':
			if !started {
				return "", false
			}
			for {
				y, ok := num()
				if !ok {
					return "", false
				}
				if cmd == 'v' {
					y += cy
				}
				b.WriteString(" L")
				put(cx)
				put(y)
				cy = y
				hadC = false
				if !more() {
					break
				}
			}
		case 'C', 'c':
			if !started {
				return "", false
			}
			for {
				var v [6]float64
				for k := 0; k < 6; k++ {
					x, ok := num()
					if !ok {
						return "", false
					}
					v[k] = x
					if k != 5 {
						skipSep()
					}
				}
				if cmd == 'c' {
					for k := 0; k < 6; k += 2 {
						v[k] += cx
						v[k+1] += cy
					}
				}
				b.WriteString(" C")
				for k := 0; k < 6; k++ {
					put(v[k])
				}
				cubicAt = [2]float64{v[2], v[3]}
				hadC = true
				cx, cy = v[4], v[5]
				if !more() {
					break
				}
			}
		case 'S', 's':
			if !started {
				return "", false
			}
			for {
				var v [4]float64
				for k := 0; k < 4; k++ {
					x, ok := num()
					if !ok {
						return "", false
					}
					v[k] = x
					if k != 3 {
						skipSep()
					}
				}
				if cmd == 's' {
					for k := 0; k < 4; k += 2 {
						v[k] += cx
						v[k+1] += cy
					}
				}
				// 第一个控制点是上一段三次曲线第二控制点关于当前点的镜像；
				// 上一段不是 C/S 时，按规范取当前点。
				c1x, c1y := cx, cy
				if hadC {
					c1x, c1y = 2*cx-cubicAt[0], 2*cy-cubicAt[1]
				}
				b.WriteString(" C")
				put(c1x)
				put(c1y)
				for k := 0; k < 4; k++ {
					put(v[k])
				}
				cubicAt = [2]float64{v[0], v[1]}
				hadC = true
				cx, cy = v[2], v[3]
				if !more() {
					break
				}
			}
		case 'Z', 'z':
			if !started {
				return "", false
			}
			b.WriteString(" Z")
			cx, cy = sx, sy
			hadC = false
		case 'A', 'a':
			if !started {
				return "", false
			}
			for {
				var rx, ry, phi, x, y float64
				var ok bool
				if rx, ok = num(); !ok {
					return "", false
				}
				skipSep()
				if ry, ok = num(); !ok {
					return "", false
				}
				skipSep()
				if phi, ok = num(); !ok {
					return "", false
				}
				largeArc, ok := flag()
				if !ok {
					return "", false
				}
				sweep, ok := flag()
				if !ok {
					return "", false
				}
				skipSep()
				if x, ok = num(); !ok {
					return "", false
				}
				skipSep()
				if y, ok = num(); !ok {
					return "", false
				}
				if cmd == 'a' {
					x += cx
					y += cy
				}
				// 弧 → 三次贝塞尔（见 arc.go）。三种去向：真弧逐段 C、退化直线 L、
				// 起止点重合则整段省略——最后一种什么都不写，但当前点仍要前移。
				segs, action := arcToCubics(cx, cy, x, y, rx, ry, phi, largeArc, sweep)
				switch action {
				case arcLine:
					b.WriteString(" L")
					put(x)
					put(y)
				case arcCubic:
					for _, s := range segs {
						b.WriteString(" C")
						put(s.C1X)
						put(s.C1Y)
						put(s.C2X)
						put(s.C2Y)
						put(s.X)
						put(s.Y)
					}
				}
				cx, cy = x, y
				hadC = false
				if !more() {
					break
				}
			}
		case 'Q', 'q', 'T', 't':
			// 见函数注释：viva 有 QuadTo，但本包还没接二次曲线；整条作废而不是画一半。
			return "", false
		default:
			return "", false
		}
	}
	out := b.String()
	if strings.TrimSpace(out) == "" {
		return "", false
	}
	return out, true
}

// isCommandLetter 判断 c 是不是 SVG 路径命令字母。
func isCommandLetter(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	}
	return false
}
