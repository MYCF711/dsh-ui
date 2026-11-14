package dsbrand

import (
	"github.com/vivaui/viva/gfx"
	"strconv"
)

// ParseSVGPath 解析 M/L/C/H/V/Z 绝对命令子集。
//
// 官方鲸鱼 logo 只用到 M/L/C/Z；H/V 是给官方 `ReferenceIconRegular`
// （文件/文件夹/会话三枚引号图标）补的——那三枚的 path 里确实有水平/垂直
// 直线，语义唯一且不需要当前点之外的上下文。相对命令、弧线、二次曲线
// 仍然一律不支持：遇到就把整条路径作废（返回 false），而不是"画一半"：
// 半个图标比没有图标更像 bug。要扩展时先确认官方路径真的换了形状。
func ParseSVGPath(d string) ([]gfx.PathCommand, bool) {
	var (
		out   []gfx.PathCommand
		nums  []float32
		i     int
		total = len(d)
		// cur 是当前点：H/V 只给一个坐标，另一个从当前点取（SVG 绝对命令语义）。
		curX, curY float32
	)
	readNumber := func() (float32, bool) {
		start := i
		for i < total {
			c := d[i]
			if c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9') || c == 'e' || c == 'E' {
				i++
				continue
			}
			break
		}
		if start == i {
			return 0, false
		}
		v, err := strconv.ParseFloat(d[start:i], 32)
		if err != nil {
			return 0, false
		}
		return float32(v), true
	}

	skipSpace := func() {
		for i < total {
			c := d[i]
			if c == ' ' || c == ',' || c == '\n' || c == '\r' || c == '\t' {
				i++
				continue
			}
			break
		}
	}

	take := func(n int) bool {
		nums = nums[:0]
		for k := 0; k < n; k++ {
			skipSpace()
			v, ok := readNumber()
			if !ok {
				return false
			}
			nums = append(nums, v)
		}
		return true
	}

	for {
		skipSpace()
		if i >= total {
			break
		}
		switch d[i] {
		case 'M':
			i++
			if !take(2) {
				return nil, false
			}
			curX, curY = nums[0], nums[1]
			out = append(out, gfx.MoveTo(curX, curY))
		case 'L':
			i++
			if !take(2) {
				return nil, false
			}
			curX, curY = nums[0], nums[1]
			out = append(out, gfx.LineTo(curX, curY))
		case 'H':
			i++
			if !take(1) {
				return nil, false
			}
			curX = nums[0]
			out = append(out, gfx.LineTo(curX, curY))
		case 'V':
			i++
			if !take(1) {
				return nil, false
			}
			curY = nums[0]
			out = append(out, gfx.LineTo(curX, curY))
		case 'C':
			i++
			if !take(6) {
				return nil, false
			}
			curX, curY = nums[4], nums[5]
			out = append(out, gfx.CubicTo(nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]))
		case 'Z', 'z':
			i++
			out = append(out, gfx.ClosePath())
		default:
			// 未支持的命令（相对命令、弧线、二次曲线……）：整条作废。
			return nil, false
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// FishAspect 返回 viewBox 的**高宽比**：显示高度 = 显示宽度 × FishAspect(W, H)。
//
// 用法就一行（官方 `ui-conversation` 的 HeroFish）：
//
//	height = 34 * VIEWBOX.height / VIEWBOX.width   →  34 * FishAspect(23.16, 17.04) ≈ 25.02
//
// 所以它是"乘在宽度上的系数"，不是"给宽度直接算出高度"。
// 曾经有一条注释写成了后者，读起来像 `FishAspect(34, …)` 就能得 25.02——
// 实际会得到 0.5 左右。名字和实现对，注释错了；而且**这个函数是有调用方的**
// （`dsconversation/brand_parse_test.go` 用它验首屏鱼高），删之前一定先 grep。
func FishAspect(w, h float32) float32 {
	if w <= 0 {
		return 1
	}
	return h / w
}
