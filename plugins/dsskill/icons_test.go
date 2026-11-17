//go:build darwin || windows || linux

// icons_test.go —— 三枚 sprite glyph 的**出处断言**。
//
// 判据不是"画得像"，而是「viewBox / 描边宽度 / 段数与首点」四项与 rc.2 sprite 原文一致：
//
//	skill-outline        viewBox 0 0 17 17  4 段（2 条直线 + 1 填充页框 + 1 枚四角星）
//	chevron-down-outline viewBox 0 0 16 16  1 段
//	inspect-outline      viewBox 0 0 16 16  3 段（左右尖括号 + 斜杠）
//
// stroke-width 必须是 1（官方 `regular` 档，`p7={regular:1}`），且 Cap/Join 留零值
// （sprite 没写 linecap/linejoin → butt/miter；改成 Round 会让 14px 下的拐角变胖）。
// 出处与字节偏移见 `icons.go` 文件头；登记表见 `docs/official-icon-sprite-register.md` §1。
package dsskill

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestSkillIconsMatchRc2Sprite 逐项核对三枚图标的官方形状。
func TestSkillIconsMatchRc2Sprite(t *testing.T) {
	cases := []struct {
		name   string
		icon   gfx.Icon
		box    float32
		stroke float32
		count  int
		first  gfx.PathCommand
	}{
		// 4 段里 3 段描边：2 条文档横线 + 1 枚四角星（M + 4×C + Z = 6 条指令）
		{"skill-outline（描边段）", skillGlyphStroke(), 17, 1, 10, gfx.MoveTo(4.57788, 5.77124)},
		// 第 3 段是 fill="currentColor" 的页框：Stroke 0 才画成填充（见 icons.go 文件头）
		{"skill-outline（填充段）", skillGlyphFill(), 17, 0, 18, gfx.MoveTo(12.1404, 1.19446)},
		{"chevron-down-outline", chevronDownIcon(), 16, 1, 4, gfx.MoveTo(4, 6)},
		{"inspect-outline", inspectIcon(), 16, 1, 10, gfx.MoveTo(4.67398, 4.25061)},
	}
	for _, c := range cases {
		if c.icon.ViewBox != c.box {
			t.Errorf("%s：viewBox = %v，want %v（**不改 viewBox**）", c.name, c.icon.ViewBox, c.box)
		}
		if c.icon.Stroke != c.stroke {
			t.Errorf("%s：stroke = %v，want %v", c.name, c.icon.Stroke, c.stroke)
		}
		if c.icon.Cap != 0 || c.icon.Join != 0 {
			t.Errorf("%s：Cap/Join = %v/%v，want 0/0（sprite 没写 linecap/linejoin）",
				c.name, c.icon.Cap, c.icon.Join)
		}
		if len(c.icon.Commands) != c.count {
			t.Fatalf("%s：段数 %d，want %d", c.name, len(c.icon.Commands), c.count)
		}
		if first := c.icon.Commands[0]; first != c.first {
			t.Errorf("%s：首条指令 %+v，want %+v", c.name, first, c.first)
		}
	}
}

// TestSkillIconSizes 钉住三处**显示**尺寸（不是 viewBox）。
//
//	skill   14 —— `SkillRow.tsx:78,105` 的 `size={14}`
//	chevron 14 —— `SkillRow.module.css:42-43` 的 `.leading svg{width:14px;height:14px}`
//	inspect 12 —— sprite 默认 size（TSX 未传 size，CSS 也没覆盖）
func TestSkillIconSizes(t *testing.T) {
	if SkillGlyphSize != 14 {
		t.Errorf("skill 图标显示尺寸 = %v，want 14（SkillRow.tsx:78）", SkillGlyphSize)
	}
	if SkillChevronSize != 14 {
		t.Errorf("chevron 显示尺寸 = %v，want 14（SkillRow.module.css:42）", SkillChevronSize)
	}
	if SkillInspectSize != 12 {
		t.Errorf("inspect 显示尺寸 = %v，want 12（sprite 默认）", SkillInspectSize)
	}
}
