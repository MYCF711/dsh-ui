package dsassets

import (
	"github.com/vivaui/viva/gfx"
)

// Build 把一条素材编译成 viva 的 gfx.Icon。
//
// fill 是**填充层**的颜色：官方这些图标用 `fill="currentColor"`，颜色由使用处决定，
// 所以本包不写死色号。纯描边素材用不到它（颜色来自调用方的 `.Color()` 着色）。
//
// 返回 false 的三种情况：
//
//   - 素材本身标注了 Unsupported（`<rect rx>` / `<circle>` 这类需要弧的图元）；
//   - 某条 `d` 用了 M/L/H/V/C/S/Z 之外的东西（弧、二次曲线）；
//   - 同一枚图标里两处要求互相矛盾的线帽/线接（一个 gfx.Icon 只有一个 Cap/Join）。
//
// 返回 false 时调用方**必须**自己决定退回什么（保留旧实现、或干脆不画），
// 不许拿近似形状顶上去——那正是本包存在的理由。
func Build(a Asset, fill gfx.Color) (gfx.Icon, bool) {
	if !a.Buildable() {
		return gfx.Icon{}, false
	}
	var (
		stroke      []gfx.PathCommand
		layers      []gfx.IconLayer
		capRound    bool
		capButt     bool
		joinRound   bool
		joinMiter   bool
		anyStrokeAt bool
	)
	for _, el := range a.Elements {
		if el.Kind != PathElement || el.D == "" {
			return gfx.Icon{}, false
		}
		commands, ok := ParsePath(el.D)
		if !ok {
			return gfx.Icon{}, false
		}
		if el.Stroke {
			stroke = append(stroke, commands...)
			anyStrokeAt = true
			if el.CapSet {
				switch el.Cap {
				case gfx.CapRound:
					capRound = true
				case gfx.CapButt:
					capButt = true
				}
			}
			if el.JoinSet {
				switch el.Join {
				case gfx.JoinRound:
					joinRound = true
				case gfx.JoinMiter:
					joinMiter = true
				}
			}
		}
		if el.Fill {
			layers = append(layers, gfx.IconLayer{Fill: fill, Commands: commands})
		}
	}
	// 一个 Icon 只有一个 Cap/Join：两处要求相反时无法同时满足，宁可不画也不选错一个。
	if capRound && capButt {
		return gfx.Icon{}, false
	}
	if joinRound && joinMiter {
		return gfx.Icon{}, false
	}

	icon := gfx.Icon{ViewBox: a.ViewBox}
	if capRound {
		icon.Cap = gfx.CapRound
	}
	if joinRound {
		icon.Join = gfx.JoinRound
	}
	switch {
	case anyStrokeAt:
		width := a.StrokeWidth
		if width <= 0 {
			width = regularStroke
		}
		icon.Stroke = width
		icon.Commands = stroke
		icon.Layers = layers
	case len(layers) == 1:
		// 纯填充素材：走 Commands 而不是 Layers，这样 `.Color()` 的 tint（含 alpha）
		// 直接生效——禁用态的 opacity 才能整体降下来，不用调用方再传一次颜色。
		icon.Commands = layers[0].Commands
	default:
		icon.Layers = layers
	}
	if len(icon.Commands) == 0 && len(icon.Layers) == 0 {
		return gfx.Icon{}, false
	}
	return icon, true
}

// CommandCount 返回素材解析后的命令总数（描边与填充都算）。
//
// 测试用它盯住"整条路径都吃完了"：官方 `d` 里有多少段，这里就该有多少条命令，
// 少一条＝截断，多一条＝把坐标当成了命令。
func CommandCount(a Asset) int {
	total := 0
	for _, el := range a.Elements {
		if el.Kind != PathElement {
			continue
		}
		if commands, ok := ParsePath(el.D); ok {
			total += len(commands)
		}
	}
	return total
}

// Bounds 返回素材所有路径的几何包围盒（viewBox 坐标）。
//
// 只统计端点与控制点——它不追求"紧包围盒"（三次曲线的极值点可能落在控制多边形
// 内部），而是给测试一个稳定的外部边界：官方 16 viewBox 的图标，包围盒必须落在
// [0,16] 之内，且不能是空的。越界就说明解析把坐标读错了（例如把相对命令当绝对）。
func Bounds(a Asset) (minX, minY, maxX, maxY float32, ok bool) {
	first := true
	add := func(x, y float32) {
		if first {
			minX, minY, maxX, maxY = x, y, x, y
			first = false
			return
		}
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}
	for _, el := range a.Elements {
		if el.Kind != PathElement {
			continue
		}
		commands, good := ParsePath(el.D)
		if !good {
			continue
		}
		for _, c := range commands {
			if c.Kind == gfx.PathCommandClose {
				continue
			}
			add(c.Point.X, c.Point.Y)
			if c.Kind == gfx.PathCommandCubic || c.Kind == gfx.PathCommandQuad {
				add(c.Control1.X, c.Control1.Y)
			}
			if c.Kind == gfx.PathCommandCubic {
				add(c.Control2.X, c.Control2.Y)
			}
		}
	}
	return minX, minY, maxX, maxY, !first
}

// ShieldOutline 返回官方四枚权限图标共用的盾牌轮廓（描边、linejoin round）。
//
// 给"宿主自己的 policy"用：`ask` / `deny` 是宿主额外暴露的审批档，官方没有同名
// 图标，本仓只能自造中间的字形；但**盾牌本身仍然是官方的**，不该再手画一面。
// 这样自造的部分缩到最小（一个问号 / 一根斜线），也一眼能看出哪些是官方、哪些不是。
func ShieldOutline() ([]gfx.PathCommand, bool) {
	return ParsePath(permissionShieldPath)
}

// ChevronDownOutline 返回官方 `IconChevronDownOutlineRegular` 的路径命令。
//
// 用它的是输入框里的两枚可选 trigger（模型 / 权限）：官方在那两枚 28px 按钮的
// 右端画这枚 14px 的箭头（`._9r0rkG_chevron{color:var(--dsw-alias-label-caption)}`，
// `._86kJGG_chevron` 同款）。**只读态不画它**——箭头在官方语义里就代表"能展开"。
//
// 它是**未登记**素材（不在 `All()` 里），与 `ShieldOutline` 同一档：路径原文在
// `paths.go`，出处与整文件 sha256 也写在那里。不单列一条登记表，是因为它没有
// 独立的官方组件（和权限盾牌一样是图标内部的一笔），而登记表要求同步
// `docs/official-assets.md` 的表格——那份文件不属于本单可改范围。
func ChevronDownOutline() ([]gfx.PathCommand, bool) {
	return ParsePath(chevronDownPath)
}

// CheckOutline 返回官方 `IconCheckOutlineRegular` 的路径命令。
//
// 官方 composer 的模型菜单用它在当前选中的那一行最左边打勾
// （`._9r0rkG_check{flex:0 0 14px}`）；同一枚图标在它自己的其它菜单里也用来
// 标选中项。未登记的理由同 `ChevronDownOutline`。
func CheckOutline() ([]gfx.PathCommand, bool) {
	return ParsePath(checkPath)
}
