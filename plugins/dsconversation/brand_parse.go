package dsconversation

import (
	"strconv"

	"github.com/vivaui/viva/ui"
)

// parseSVGPath 只认官方这条路径真正用到的四个**绝对**命令（M / L / C / Z）。
//
// 官方路径数过：4 个 M、71 个 C、1 个 L、4 个 Z。相对命令、弧线、二次曲线
// 这里一律不支持——遇到就把整条路径作废（返回 false），而不是"画一半"：
// 半个 logo 比没有 logo 更像 bug。要扩展时先确认官方路径真的换了形状。
func parseSVGPath(d string) ([]ui.PathCommand, bool) {
	var (
		out   []ui.PathCommand
		nums  []float32
		i     int
		total = len(d)
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
			out = append(out, ui.MoveTo(nums[0], nums[1]))
		case 'L':
			i++
			if !take(2) {
				return nil, false
			}
			out = append(out, ui.LineTo(nums[0], nums[1]))
		case 'C':
			i++
			if !take(6) {
				return nil, false
			}
			out = append(out, ui.CubicTo(nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]))
		case 'Z', 'z':
			i++
			out = append(out, ui.ClosePath())
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

// fishAspect 是官方 viewBox 的宽高比（34px 宽时高 25.02px）。
func fishAspect(w, h float32) float32 {
	if w <= 0 {
		return 1
	}
	return h / w
}
