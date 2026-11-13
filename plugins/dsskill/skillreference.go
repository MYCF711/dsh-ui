// skillreference.go —— 官方 `SkillReference.tsx`（28 行）的 Go 实现。
//
// # 官方原文
//
//	export function SkillReference({ node, raw, interactive, openSkill }: SkillReferenceProps) {
//	  return <ReferenceChip
//	    appearance="skill"
//	    label={`/${node.data.name}`}
//	    title={raw}
//	    {...(interactive ? { onActivate: () => { openSkill(node.data.name) } } : {})}
//	  />
//	}
//
// 官方注释把三条语义写死了：
//
//	*"Render a skill chip; previews omit activation even when the entry has an
//	 injected action."* —— 预览态（`interactive === false`）**不挂** onActivate，
//	即使 `openSkill` 注入了。
//
// # 边界：`ReferenceChip` 不是本包的
//
// `ReferenceChip` 来自 `@deepseek-ai/dsh-client-ui-primitives`，**不在**
// `dsh-client-ui-skill` 的 201 行 TSX 里。本文件实现的是 `ui-skill` 这一侧的
// **契约**（label 怎么拼、title 放什么、什么时候挂 onActivate），
// 而 chip 本身的像素几何属于 primitives 面——那是一个独立的复刻任务。
//
// 所以本文件的绘制用本仓已有令牌（`pluginapi.Theme`）搭出**官方
// `appearance="skill"` 的语义形状**，并把每一处几何标 `derived`，
// 报告里登记为「primitives 面的 chip 几何未复刻」。
package dsskill

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ReferenceChipAppearance 是官方 `appearance` 的取值（本包只用到 `skill`）。
type ReferenceChipAppearance string

// AppearanceSkill 是官方 `appearance="skill"`。
const AppearanceSkill ReferenceChipAppearance = "skill"

// ── 官方 `appearance="skill"` 的声明（derived，见文件头边界）──────────
//
// 下面这几个数值**不是**从官方 CSS 抄的（`ReferenceChip` 的样式表在
// primitives 包里，不在本包的解包范围内），而是从本仓已有令牌 + 官方
// 语义形状推出来的。报告 §5 逐条登记为 derived。
const (
	// SkillChipHeight 是 chip 的高度（官方 `.chip` 是 20px，见
	// `plugins/dscore/primitives.go` 的 `Chip`：`Height(20)`）。
	SkillChipHeight float32 = 20
	// SkillChipPadX 是左右内边距（同上，官方 `.chip{padding-inline:6px}`）。
	SkillChipPadX float32 = 6
	// SkillChipFontSize 是字号（官方 `.chip` 用 `--dsh-content-font-size-caption`）。
	SkillChipFontSize float32 = 12
)

// SkillReferenceProps 是官方 `UserTextNodeProps<'skill'> & SkillReferenceInjected`。
type SkillReferenceProps struct {
	// Name 是官方 `node.data.name`（**不带** `/`）。
	Name string
	// Raw 是官方 `raw`：命中区在原文里的那一段（含 `/`），作为 `title`。
	Raw string
	// Interactive 是官方 `interactive`：预览态传 false（预览不算激活）。
	Interactive bool
	// OpenSkill 是官方注入的 `openSkill`；nil = 没注入（不挂 onActivate）。
	OpenSkill func(name string)
}

// SkillReferenceLabel 是官方 `label={\`/${node.data.name}\`}`。
//
// 注意是**拼一个** `/` 前缀，不是"名字里本来就带"——
// `userText` 规则交出来的 `SkillReference.Name` 已经是去掉 `/` 的。
func SkillReferenceLabel(name string) string { return "/" + name }

// Activate 是官方 `onActivate: () => { openSkill(node.data.name) }`。
//
// 返回 false = **没有 onActivate**（`interactive` 为假，或 `openSkill` 没注入）。
// 调用方据此决定挂不挂 hit 区——官方是**根本没有那个属性**，
// 不是"挂了但什么都不做"（后者点下去会吞掉事件）。
func (p SkillReferenceProps) Activate() bool {
	if !p.Interactive || p.OpenSkill == nil {
		return false
	}
	p.OpenSkill(p.Name)
	return true
}

// DrawSkillReference 画官方 `ReferenceChip appearance="skill"`。
//
// 返回的 box 只在**可激活**时挂 hit 区：`interactive === false` 的预览态
// 是一个纯标签（官方连 `onActivate` 都不传）。
//
// `id` 由调用方给（官方由 `conversation.user.text.node` 的 keyed 槽分配）。
func DrawSkillReference(f ui.C, t *pluginapi.Theme, id gfx.ID, props SkillReferenceProps) ui.Box {
	label := SkillReferenceLabel(props.Name)
	chip := ui.Row(f).Height(SkillChipHeight).Radius(t.Radii.XS).
		PadX(SkillChipPadX).Items(ui.Center).
		Background(t.BgLayer2)
	if props.Interactive && props.OpenSkill != nil {
		chip = chip.Hover(ui.Look{}.Background(t.Hover)).
			HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand})
	}
	chip.Children(func() {
		ui.Text(f, label).FontSize(SkillChipFontSize).Color(t.Info)
	})
	if props.Interactive && props.OpenSkill != nil && chip.Clicked() {
		props.Activate()
	}
	return chip
}
