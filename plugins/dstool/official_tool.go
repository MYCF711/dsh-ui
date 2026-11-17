// official_tool.go —— 官方 `@deepseek-ai/dsh-client-ui-tool` 面的**唯一事实表**（rc.2 口径）。
//
// # 官方包核实结果：**有**
//
// 解包树 `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/` 下存在
// `dsh-client-ui-tool@0.2.2-rc.2`（README 一行：*"Tool call rows and their
// expandable bodies"*），带 `src/**` 源码与 7 个 CSS Module：
//
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/ToolCallTree.module.css          12 行
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/components/ToolRow.module.css   392 行
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/components/ToolDetails.module.css 266 行
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/components/ExitCodeBadge.module.css 14 行
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/components/AskQuestionCard.module.css
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/components/QuestionToolRow.module.css
//	@deepseek-ai/dsh-client-ui-tool/src/client/tool/toolviews/bash-sample.module.css  261 行
//
// 所以本文件里**没有**任何"无官方依据"的数值：每一条都能指到上面某个
// `file:line`。下文路径省略前缀 `@deepseek-ai/dsh-client-ui-tool/src/client/tool/`。
//
// # 标题行的几何来自哪
//
// 官方 ToolRow 的标题行是共享原语 `DisclosureRow` 画的，而 `@deepseek-ai/dsh-client-ui-primitives`
// **不在 rc.2 的 node_modules 里**（它是被 bundle 进安装态的打包件），所以
// `DisclosureRow.module.css` 本身取不到。但官方自己把那份几何**逐条复制**在了
// `toolviews/bash-sample.module.css` 里，并在注释里点名同源：
//
//	:1-2   "Bash toolview: same geometry/tokens as ToolRow (figma Bash · description)"
//	:126-131 "Same glyph-scaling rule as DisclosureRow's leading: the CSS edge
//	          overrides the svg attributes so the 14px business glyph follows…"
//
// 互证还有第二处：`dsh-client-ui-chat/src/client/chat/MessageItem.module.css:108-123`
// 的 `.compactionLeading`（16×16 盒 + `margin-right:6px` + svg 14×14，注释同样写
// "Same glyph-scaling rule as DisclosureRow's leading"）。
//
// 因此标题行的每个数值都取 `bash-sample.module.css`（它自己就是 ToolRow 的复制）：
//
//	.row       display:flex; align-items:center; height:calc(24px + delta)   :93-96
//	.leading   16×16; margin-right:6px                                      :114-124
//	.leading svg 14×14                                                      :132-135
//	.title     font-size:13px; line-height:calc(24px + delta)               :167-171
//	.sep       2×2; radius 1; margin 0 8px; background label-caption        :173-180
//	.summary   font-size:13px; line-height:calc(24px + delta); nowrap+ellipsis :182-190
//	.root 颜色 label-tertiary，hover 只变色**不铺底**                            :97-106
//
// 官方的行内三档间距是 `gap6 / gap8 / gap8`（ToolRow.module.css:1-7 的表头注释：
// `[16 leading] gap6 [title 13/24] gap8 [2x2 dot] gap8 [summary FILL truncate]`）。
// 在 flex 里它落成 `.leading{margin-right:6px}` + `.sep{margin:0 8px}`，
// 所以本包用 Gap(0) + 两个 margin 复刻，**不用** 一个 6 的 gap 再叠 8 的 margin。
//
// # 展开体：两套官方卡
//
// ① **代码 / IN-OUT 卡**（ToolRow.module.css:247-255，bash-sample.module.css:27-35 同值）
//
//	.ioCard     margin:4px 0 4px 4px; border:.5px border-l1; radius lg(16);
//	            background markdown-code-block; font markdown-code-block-small
//	.ioSection  grid max-content 1fr; column-gap 14px; align-items baseline;
//	            padding 12px 16px; max-height 150px                     :259-267
//	.ioLabel    sticky top 0; color label-caption                        :288-293
//	.ioDivider  height:.5px; background border-l2                        :297-301
//	.ioText     color label-secondary; pre-wrap                           :303-308
//
// ② **参数 / 记录面板**（ToolDetails.module.css，官方 `ToolDetails` 组件）
//
//	.root       margin:4px 0 4px 4px; border:.5px border-l1; radius lg;
//	            background markdown-code-block; color label-secondary;
//	            font:var(--dsw-font-xs-13); overflow:hidden             :1-9
//	.item       padding:10px 14px                                        :19-23
//	.item+item  border-top:.5px border-l2                                :25-27
//	.caption    padding:10px 14px 6px; font-size:12px; label-caption      :35-39
//	.fields     margin:0                                                  :125-127
//	.field      grid-template-columns:5.5em minmax(0,1fr); gap:12px       :133-137
//	.field+.field margin-top:4px                                          :139-141
//	.field dt   color label-caption                                       :143-145
//	.field dd   margin:0; pre-wrap                                        :147-151
//	.subtitle   margin-top:3px; font-size:12px; label-tertiary            :176-181
//	.list       max-height:320px                                          :11-17
//
// 字号换算（都来自 `dsh-client-ui-theme/src/styles/gradient-shadow-text.css`）：
//
//	--dsw-font-xs-13                      13px/20px 无衬线   :234-237
//	--dsw-font-markdown-code-block-small  11px/16px 等宽     :178-182
//	--dsw-radius-lg                       16px               base.css:21
//	--dsw-alias-markdown-code-block       暗 neutral-bluish-900 / 亮 -50
//	                                      design-platform.css:267,419
//
// 本仓 `pluginapi.Theme` 的对应关系：`Fonts.Secondary`=13、`Fonts.Caption`=12、
// `Fonts.Tiny`=11、`Radii.LG`=16、`CodeBlock` ← `--dsw-alias-markdown-code-block`。
//
// # 规矩（与本仓 `plugins/dsskill/official_skill.go`、
// `plugins/dsagentteam/commandrow.go` 的常量表同源）
//
//	① 每个数值只有一个出处：本文件的常量。
//	② 官方值全部从 rc.2 随包 `src/**` 的 CSS / TSX 读出来，**不是从截图猜的**。
//	③ 取不到的官方件（`DisclosureRow`、`Tag`、TerminalBlock 的横幅）不编号，
//	   由消费方按"无官方依据"处理，绝不补一个看起来像的值。
package dstool

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

const (
	// ── 标题行（bash-sample.module.css，见文件头「标题行的几何来自哪」）──
	toolRowHeight    float32 = 24 // :96   height:calc(24px + var(--dsh-content-font-delta))
	toolLeadingBox   float32 = 16 // :117-118 width/height:calc(16px + delta)
	toolLeadingGlyph float32 = 14 // :132-135 .leading svg 14px
	toolLeadingGap   float32 = 6  // :122  margin-right:6px
	toolSepSize      float32 = 2  // ToolRow.module.css:28-29 width/height:2px
	toolSepRadius    float32 = 1  // ToolRow.module.css:30  border-radius:1px
	toolSepGap       float32 = 8  // ToolRow.module.css:31  margin:0 8px

	// ── 展开卡外壳（ToolRow.module.css:247-255 = bash-sample.module.css:27-35）──
	toolCardMarginV float32 = 4   // margin:4px 0 4px 4px
	toolCardMarginH float32 = 4   // 同上（左侧缩进 4px）
	toolCardBorder  float32 = 0.5 // border:0.5px solid var(--dsw-alias-border-l1)
	toolIOPadV      float32 = 12  // .ioSection padding:12px 16px
	toolIOPadH      float32 = 16
	toolIODivider   float32 = 0.5 // .ioDivider height:0.5px
	toolCodeLine    float32 = 16  // markdown-code-block-small = 11px/16px

	// ── 参数 / 记录面板（ToolDetails.module.css）──
	toolFieldPadV    float32 = 10 // :22  padding:10px 14px
	toolFieldPadH    float32 = 14
	toolFieldLabelEm float32 = 5.5 // :135 grid-template-columns:5.5em minmax(0,1fr)
	toolFieldLine    float32 = 20  // :7   font:var(--dsw-font-xs-13) → 13px/20px
	toolFieldGap     float32 = 12  // :136 gap:12px
	toolFieldRowGap  float32 = 4   // :140 .field + .field { margin-top:4px }
	toolSubtitleGap  float32 = 3   // :177 .subtitle margin-top:3px
	toolSubtitleSize float32 = 12  // :179 .subtitle font-size:12px
)

// toolFieldLabelWidth 是官方 `.field` 的标签列宽：`5.5em`，em 取 `.root` 的
// `--dsw-font-xs-13`（13px）→ 71.5px。字号跟着主题走时这列也跟着走。
func toolFieldLabelWidth(t *pluginapi.Theme) float32 {
	return toolFieldLabelEm * t.Fonts.Secondary
}

// toolFieldValueWidth 是面板里数值列的可用宽：内容宽 − 卡片左缩进 − 面板左右内边距
// − 标签列 − 列间距。给 `Ellipsis` 一个真实边界（不给的话长路径会画到窗口外，
// 与 `drawCard` / `fileRenderer` 里同一个坑）。
func toolFieldValueWidth(env *pluginapi.Env, t *pluginapi.Theme) float32 {
	w := contentWidth(env) - toolCardMarginH - 2*toolFieldPadH - toolFieldLabelWidth(t) - toolFieldGap
	if w < 40 {
		w = 40
	}
	return w
}

// drawToolCard 画官方展开卡的外壳（`ioCard` / `ToolDetails.root` 共用的那层）：
// margin 4px 0 4px 4px、0.5px border-l1、radius lg(16)、markdown-code-block 底。
//
// 官方这两层的差别只在**内层**（ioSection 12/16 vs item 10/14），外壳完全同值，
// 所以外壳只写一次。`Clip()` 对应官方的 `overflow:hidden`。
func drawToolCard(f ui.C, t *pluginapi.Theme, body func(ui.C)) {
	ui.Column(f).FillWidth().
		MarginSides(toolCardMarginV, 0, toolCardMarginV, toolCardMarginH).
		Radius(t.Radii.LG).Background(t.CodeBlock).Border(toolCardBorder, t.BorderL1).
		Clip().Children(func() { body(f) })
}

// drawIOSection 是官方 `.ioSection`（ToolRow.module.css:259-267）的一段：
// padding 12px 16px。官方那里是 `max-content 1fr` 的栅格 —— 左列是 IN/OUT
// 标签；本包不画标签（见报告「还差什么」），所以它退化成一列，内边距照旧。
func drawIOSection(f ui.C, t *pluginapi.Theme, body func(ui.C)) {
	ui.Column(f).FillWidth().PadSides(toolIOPadV, toolIOPadH, toolIOPadV, toolIOPadH).Children(func() { body(f) })
}

// drawIODivider 是官方 `.ioDivider`：0.5px、border-l2、横跨整张卡。
func drawIODivider(f ui.C, t *pluginapi.Theme) {
	ui.Row(f).FillWidth().Height(toolIODivider).Background(t.BorderL2)
}

// drawDetailsItem 是官方 `.item`（ToolDetails.module.css:19-23）：padding 10px 14px。
func drawDetailsItem(f ui.C, t *pluginapi.Theme, body func(ui.C)) {
	ui.Column(f).FillWidth().PadSides(toolFieldPadV, toolFieldPadH, toolFieldPadV, toolFieldPadH).Children(func() { body(f) })
}

// drawField 画官方 `.field` 的一行：标签列 5.5em + 12px 列间距 + 数值列。
//
// 标签颜色 label-caption、数值颜色 label-secondary，两者同为 `--dsw-font-xs-13`
// 的 13/20（ToolDetails.module.css:7,143-151）。
func drawField(f ui.C, env *pluginapi.Env, label, value string) {
	t := env.Theme
	ui.Row(f).FillWidth().Gap(toolFieldGap).Items(ui.Center).Children(func() {
		ui.Text(f, label).FontSize(t.Fonts.Secondary).LineHeight(toolFieldLine).
			Color(t.LabelCaption).Width(toolFieldLabelWidth(t))
		ui.Text(f, value).FontSize(t.Fonts.Secondary).LineHeight(toolFieldLine).
			Color(t.LabelSecondary).MaxWidth(toolFieldValueWidth(env, t)).Ellipsis()
	})
}
