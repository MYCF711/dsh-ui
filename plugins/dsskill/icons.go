// icons.go —— 官方 sprite 里的三枚 glyph（**不是手画的**）。
//
// # 出处（逐字，rc.2）
//
// 三枚都来自官方 web 前端 bundle 的 sprite 表：
//
//	文件    dsh-web-frontend/dist/assets/index-ghOpC9uV.js
//	sha256  061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8（1,093,231 B）
//	行      129
//
//	symbol                 列       viewBox     sprite 默认 size  svg sha256
//	skill-outline          56310    0 0 17 17   16               88dd2473512b155d4756633dc8c14f4951b5e6384874556d6742fa0e78b80e7e
//	chevron-down-outline   11631    0 0 16 16   14               8fabc0c6c62873ed5a14e8117a84c5016a896723f8cd06d75a84344641d6a120
//	inspect-outline        55832    0 0 16 16   12               3cd41f1c24f29278bb7aac0da984657388e96fd915d6e0e156fa1e112323b798
//
// sprite 的 `<svg>` 只写 `fill="none" stroke-width="${weight}"`，
// `weight` 由 weight 名字决定（`regular` → **1**）。没有 `linecap` / `linejoin`，
// 所以 Cap/Join 就是 svg 的默认（butt / miter）——**不要**改成 Round。
//
// # 一个必须分开画的事实：skill 图标是**混用 fill 与 stroke** 的
//
// 官方 `skill-outline` 的 4 段 path 里，第 3 段（`M12.1404 1.19446…`，页框）
// 写的是 `fill="currentColor"`，另外 3 段是 `stroke="currentColor"`。
//
// viva 的 `gfx.Icon` 只有一个 `Stroke` 字段：`Stroke != 0` 时 `Commands`
// **全部描边**、`Stroke == 0` 时**全部填充**，`Layers` 又拿不到 tint
// （"the tint contributes only its alpha to them"）。所以一枚图标画不出
// 「三段描边 + 一段填充」。
//
// 解法不是把填充段也描边（那会让页框从 1 单位宽变成 2 单位，14px 下肉眼可见
// 变粗），而是**叠画两枚图标**：`ui.Stack` 把两枚放在同一个盒里，
// 一枚 `Stroke:1`（三段线）、一枚 `Stroke:0`（页框填充）。两枚同 viewBox、
// 同 tint，合起来就是官方那一枚。
package dsskill

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方三处调用点的显示尺寸（不是 viewBox，viewBox 见各函数注释）。
const (
	// SkillGlyphSize 是官方 `IconSkillOutlineRegular size={14}`
	// （`SkillRow.tsx:78` 与 `:105`）。
	SkillGlyphSize float32 = 14
	// SkillChevronSize 是官方 `.leading svg{width:14px;height:14px}`
	// （`SkillRow.module.css:42-43`）——chevron 自己没传 size，
	// sprite 默认也是 14，两处一致。
	SkillChevronSize float32 = 14
	// SkillInspectSize 是官方 `IconInspectOutlineRegular`（`SkillRow.tsx:165`）
	// 没传 size，取 sprite 默认 **12**；`.inspectButton` 没有覆盖 svg 尺寸。
	SkillInspectSize float32 = 12
)

// skillGlyphStroke 是官方 `skill-outline` 的**三段描边 path**（第 1、2、4 段）。
//
// viewBox 是 `0 0 17 17`（**不是 14**；14 是显示尺寸）。
func skillGlyphStroke() gfx.Icon {
	return gfx.Icon{
		ViewBox: 17,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段：`M4.57788 5.77124H10.7029`（文档横线）
			gfx.MoveTo(4.57788, 5.77124),
			gfx.LineTo(10.7029, 5.77124),
			// 第 2 段：`M4.57788 8.89819H7.91879`（短横线）
			gfx.MoveTo(4.57788, 8.89819),
			gfx.LineTo(7.91879, 8.89819),
			// 第 4 段：`M12.0051 15.1056…Z`（右下四角星 / sparkle）
			gfx.MoveTo(12.0051, 15.1056),
			gfx.CubicTo(12.0051, 13.6395, 10.8166, 12.451, 9.35059, 12.451),
			gfx.CubicTo(10.8166, 12.451, 12.0051, 11.2626, 12.0051, 9.79651),
			gfx.CubicTo(12.0051, 11.2626, 13.1936, 12.451, 14.6597, 12.451),
			gfx.CubicTo(13.1936, 12.451, 12.0051, 13.6395, 12.0051, 15.1056),
			gfx.ClosePath(),
		},
	}
}

// skillGlyphFill 是官方 `skill-outline` 的**第 3 段填充 path**（页框带）。
//
// 官方原样是 `fill="currentColor"`；这里 `Stroke: 0` 让 viva 按填充画
// （见文件头的「混用 fill 与 stroke」）。
func skillGlyphFill() gfx.Icon {
	return gfx.Icon{
		ViewBox: 17,
		Stroke:  0,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(12.1404, 1.19446),
			gfx.CubicTo(12.9442, 1.19446, 13.6404, 1.81999, 13.6404, 2.64465),
			gfx.LineTo(13.6404, 8.89856),
			gfx.LineTo(12.6404, 8.89856),
			gfx.LineTo(12.6404, 2.64465),
			gfx.CubicTo(12.6404, 2.42015, 12.4411, 2.19446, 12.1404, 2.19446),
			gfx.LineTo(3.14038, 2.19446),
			gfx.CubicTo(2.83968, 2.19446, 2.64038, 2.42015, 2.64038, 2.64465),
			gfx.LineTo(2.64038, 13.0929),
			gfx.CubicTo(2.64082, 13.3172, 2.84001, 13.5421, 3.14038, 13.5421),
			gfx.LineTo(8.88159, 13.5421),
			gfx.LineTo(8.88159, 14.5421),
			gfx.LineTo(3.14038, 14.5421),
			gfx.CubicTo(2.33675, 14.5421, 1.6408, 13.9172, 1.64038, 13.0929),
			gfx.LineTo(1.64038, 2.64465),
			gfx.CubicTo(1.64038, 1.81999, 2.33651, 1.19446, 3.14038, 1.19446),
			gfx.LineTo(12.1404, 1.19446),
			gfx.ClosePath(),
		},
	}
}

// chevronDownIcon 是官方 sprite `chevron-down-outline`（viewBox `0 0 16 16`，regular 描边 1）。
//
// 单段 path：`M4 6L7.29289 9.29289C7.68342 9.68342 8.31658 9.68342 8.70711 9.29289L12 6`。
func chevronDownIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(4, 6),
			gfx.LineTo(7.29289, 9.29289),
			gfx.CubicTo(7.68342, 9.68342, 8.31658, 9.68342, 8.70711, 9.29289),
			gfx.LineTo(12, 6),
		},
	}
}

// inspectIcon 是官方 sprite `inspect-outline`（viewBox `0 0 16 16`，regular 描边 1）。
//
// 三段 path：左右两枚尖括号 + 中间斜杠。
func inspectIcon() gfx.Icon {
	return gfx.Icon{
		ViewBox: 16,
		Stroke:  1,
		Commands: []gfx.PathCommand{
			// 第 1 段：左尖括号
			gfx.MoveTo(4.67398, 4.25061),
			gfx.LineTo(1.36094, 7.86484),
			gfx.CubicTo(1.29085, 7.9413, 1.29085, 8.05866, 1.36094, 8.13513),
			gfx.LineTo(4.67398, 11.7494),
			// 第 2 段：右尖括号
			gfx.MoveTo(11.3262, 4.25061),
			gfx.LineTo(14.6392, 7.86484),
			gfx.CubicTo(14.7093, 7.9413, 14.7093, 8.05866, 14.6392, 8.13513),
			gfx.LineTo(11.3262, 11.7494),
			// 第 3 段：中间的斜杠
			gfx.MoveTo(9.56222, 3.62573),
			gfx.LineTo(6.43774, 12.3743),
		},
	}
}

// drawSkillGlyph 画官方 `IconSkillOutlineRegular`（描边段 + 填充段叠画）。
//
// 两枚图标叠在同一个盒里：`ui.Stack` 不参与 flex 排布，两个子节点占据
// 同一个矩形（官方那枚 svg 也是「4 段 path 同一个 `<svg>`」）。
// 尺寸与 tint 由调用方给——调用点只有 `span.leading`（14px），
// 颜色继承 `.row` 的 `--dsw-alias-label-tertiary`。
func drawSkillGlyph(f ui.C, tint ui.Color, size float32) {
	ui.Stack(f).Width(size).Height(size).Children(func() {
		ui.Icon(f, skillGlyphStroke()).Size(size, size).Color(tint)
		ui.Icon(f, skillGlyphFill()).Size(size, size).Color(tint)
	})
}
