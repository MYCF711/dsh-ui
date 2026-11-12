// Package dscore 是界面线的**根本依赖插件**：设计令牌 + 基础组件画法。
//
// 为什么它是第一个：所有画东西的插件都要先有「颜色、字号、圆角、间距」这套
// 语言，否则每个组件各写一套色号，改一次主题要改十几个包。所以它 Provides
// 两个键（ui.theme / ui.primitives），其余插件全部 Requires 它们。
//
// 令牌的取值**不是我定的**，是从官方界面包里量出来的：
//
//	@deepseek-ai/dsh-client-ui-theme  ← 注入 --dsw-static-* / --dsw-alias-*
//	tmp/official-theme.js             ← 那份 CSS 的落盘副本
//	docs/official-visual-spec.md      ← 解出来的规格表
//
// 所以「外表像」不是靠眼力，是靠照抄同一套令牌：官方改一个色号，这里改一行。
package dscore

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// Plugin 实现 pluginapi.Plugin。
type Plugin struct {
	// Dark 选深色还是浅色（官方两套都在下面的表里）。
	Dark  bool
	theme *pluginapi.Theme
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-core" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Requires 实现 pluginapi.Plugin。根本依赖插件不依赖任何插件。
func (*Plugin) Requires() []string { return nil }

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	if p.Dark {
		p.theme = Dark()
	} else {
		p.theme = Light()
	}
	ctx.Provide(pluginapi.KeyTheme, p.theme)
	ctx.Provide(pluginapi.KeyPrimitives, &Primitives{Theme: p.theme})
	return nil
}

// Theme 返回本插件装上的主题（注册后才有值）。
func (p *Plugin) Theme() *pluginapi.Theme { return p.theme }

// 官方色板里的原始值。名字与 --dsw-static-* 一一对应，别改名。
const (
	staticNeutralBluish00   = 0xffffff // 纯白
	staticNeutralBluish50   = 0xf9fafb
	staticNeutralBluish60   = 0xf9fafb
	staticNeutralBluish75   = 0xf1f3f5
	staticNeutralBluish100  = 0xebeef2
	staticNeutralBluish150  = 0xe9ecf2
	staticNeutralBluish200  = 0xe1e5ee
	staticNeutralBluish300  = 0xcfd3d6
	staticNeutralBluish400  = 0xadb2b8
	staticNeutralBluish500  = 0x979da6
	staticNeutralBluish600  = 0x81858c
	staticNeutralBluish700  = 0x61666b
	staticNeutralBluish750  = 0x43454a
	staticNeutralBluish800  = 0x353638
	staticNeutralBluish850  = 0x2c2c2e
	staticNeutralBluish875  = 0x232324
	staticNeutralBluish900  = 0x1b1b1c
	staticNeutralBluish950  = 0x151517
	staticNeutralBluish1000 = 0x0f1115

	staticNeutral200 = 0xe5e5e5
	staticNeutral300 = 0xd4d4d4
	staticNeutral400 = 0xa2a4a6
	staticNeutral550 = 0x65676b
	staticNeutral600 = 0x545557
	staticNeutral700 = 0x3c3c3d
	staticNeutral800 = 0x292929

	staticDeepseek100 = 0xe4edfd
	staticDeepseek200 = 0xd3e2ff
	staticDeepseek400 = 0x7aaaff
	staticDeepseek450 = 0x5686fe
	staticDeepseek500 = 0x4176e6
	staticDeepseek50  = 0xedf3fe

	staticGreen500 = 0x22c55e
	staticRed400   = 0xf25a5a
	staticRed600   = 0xec1313
	staticAmber500 = 0xf59e0b

	// 状态"容器"色（官方 `--dsw-alias-state-*` 指向的 `--dsw-static-*` 档位）。
	//
	// 取值来源：官方 `dsh-client-ui-theme/src/styles/design-platform.css`
	// （rc.2 asar 内，`--dsw-static-amber-*` / `--dsw-static-green-*` 原值）：
	//
	//	amber-100 rgb(254, 245, 231) / amber-600 rgb(221, 134, 41) / amber-900 rgb(39, 36, 31)
	//	green-100 rgb(230, 250, 237) / green-900 rgb(35, 60, 44)
	//
	// 暗色用 900 档（`state-warn-tertiary: var(--dsw-static-amber-900)`），
	// 亮色用 100 档；label / primary 两档两套主题同值。
	staticAmber100 = 0xfef5e7
	staticAmber600 = 0xdd8629
	staticAmber900 = 0x27241f
	staticGreen100 = 0xe6faed
	staticGreen900 = 0x233c2c
)

// 圆角：官方 --dsw-radius-* 的全部取值。改这里等于改所有组件的手感。
var radii = pluginapi.Radii{XS: 4, SM: 8, MD: 12, LG: 16, XL: 20, Panel: 28}

// fonts 是字号阶梯。官方正文由用户可调的 --dsh-content-font-size 决定，
// 其余字号从它推出来；这里取默认档次（14）。
func fonts() pluginapi.Fonts {
	const base float32 = 14
	return pluginapi.Fonts{
		Body:          base,
		BodyLine:      base + 8,
		Secondary:     base - 1,
		SecondaryLine: base + 6,
		Caption:       base - 2,
		CaptionLine:   base + 4,
		Code:          base - 1,
		CodeLine:      base + 7,
		Title:         base + 1,
		Tiny:          base - 3,
	}
}

// Dark 返回官方深色主题（body[data-ds-dark-theme] 那一套）。
func Dark() *pluginapi.Theme {
	return &pluginapi.Theme{
		Name: "ds-dark",
		Dark: true,

		BgBase:      ui.RGB24(staticNeutralBluish950),
		SidebarFill: ui.RGB24(staticNeutralBluish900),
		BgLayer1:    ui.RGB24(staticNeutralBluish875),
		BgLayer2:    ui.RGB24(staticNeutralBluish850),
		BgLayer3:    ui.RGB24(staticNeutralBluish800),

		NavActive: ui.RGB24(staticNeutralBluish750),
		NavHover:  ui.RGB24(staticNeutralBluish850),
		Hover:     ui.RGBA(255, 255, 255, 0.08), // --dsw-alias-interactive-bg-hover = #ffffff14

		BorderL1: ui.RGBA(255, 255, 255, 0.06), // #ffffff0f
		BorderL2: ui.RGBA(255, 255, 255, 0.12), // #ffffff1f
		BorderL3: ui.RGBA(255, 255, 255, 0.16), // #ffffff29
		BorderL4: ui.RGBA(255, 255, 255, 0.20), // #ffffff33

		LabelPrimary:   ui.RGB24(staticNeutralBluish50),
		LabelSecondary: ui.RGB24(staticNeutralBluish300),
		LabelTertiary:  ui.RGB24(staticNeutralBluish400),
		LabelCaption:   ui.RGB24(staticNeutralBluish600),
		LabelDimmed:    ui.RGB24(staticNeutralBluish750),

		Link:              ui.RGB24(staticDeepseek400),
		BrandPrimary:      ui.RGB24(staticNeutralBluish50),
		ButtonPrimaryFill: ui.RGB24(staticNeutralBluish50),
		ButtonPrimaryFg:   ui.RGB24(staticNeutralBluish1000),

		CodeBlock:  ui.RGB24(staticNeutralBluish900),
		CodeBanner: ui.RGB24(staticNeutralBluish850),
		InlineCode: ui.RGB24(staticNeutral800),
		// 语法色官方没有公开令牌，这六个是我们自己的取值（已用真实代码块目视校验）。
		CodeComment:  ui.RGB24(0x6a9955),
		CodeString:   ui.RGB24(0xce9178),
		CodeNumber:   ui.RGB24(0xb5cea8),
		CodeKeyword:  ui.RGB24(0x569cd6),
		CodeFunction: ui.RGB24(0xdcdcaa),
		CodeVariable: ui.RGB24(0x9cdcfe),

		Bubble:     ui.RGB24(staticNeutralBluish850), // --dsw-specific-bubble
		InputMajor: ui.RGB24(staticNeutralBluish850), // --dsw-specific-input-major

		Scrollbar:      ui.RGB24(staticNeutral700),
		ScrollbarHover: ui.RGB24(staticNeutral600),

		Success: ui.RGB24(staticGreen500),
		Error:   ui.RGB24(staticRed400),
		Warn:    ui.RGB24(staticAmber500),
		Info:    ui.RGB24(staticDeepseek400),

		// 暗色：tertiary = 900 档（官方 design-platform.css 的暗色别名解析）。
		SuccessTertiary: ui.RGB24(staticGreen900),
		SuccessPrimary:  ui.RGB24(staticGreen500),
		WarnTertiary:    ui.RGB24(staticAmber900),
		WarnLabel:       ui.RGB24(staticAmber600),

		Selection: ui.RGBA(59, 130, 246, 0.40), // --dsw-alias-bg-document-selection

		Radii: radii,
		Fonts: fonts(),
	}
}

// Light 返回官方浅色主题（body 上那一套）。
func Light() *pluginapi.Theme {
	return &pluginapi.Theme{
		Name: "ds-light",
		Dark: false,

		BgBase:      ui.RGB24(staticNeutralBluish00),
		SidebarFill: ui.RGB24(staticNeutralBluish50),
		BgLayer1:    ui.RGB24(staticNeutralBluish00),
		BgLayer2:    ui.RGB24(staticNeutralBluish00),
		BgLayer3:    ui.RGB24(staticNeutralBluish00),

		NavActive: ui.RGB24(staticNeutralBluish100),
		NavHover:  ui.RGB24(staticNeutralBluish75),
		Hover:     ui.RGBA(38, 49, 72, 0.06), // #2631480f

		BorderL1: ui.RGBA(0, 0, 0, 0.04), // #0000000a
		BorderL2: ui.RGBA(0, 0, 0, 0.10), // #0000001a
		BorderL3: ui.RGBA(0, 0, 0, 0.12), // #0000001f
		BorderL4: ui.RGBA(0, 0, 0, 0.16), // #00000029

		LabelPrimary:   ui.RGB24(0x000000),
		LabelSecondary: ui.RGB24(staticNeutralBluish700),
		LabelTertiary:  ui.RGB24(staticNeutralBluish600),
		LabelCaption:   ui.RGB24(staticNeutralBluish400),
		LabelDimmed:    ui.RGB24(staticNeutralBluish200),

		Link:              ui.RGB24(staticDeepseek500),
		BrandPrimary:      ui.RGB24(staticNeutralBluish1000),
		ButtonPrimaryFill: ui.RGB24(staticNeutralBluish1000),
		ButtonPrimaryFg:   ui.RGB24(staticNeutralBluish00),

		CodeBlock:    ui.RGB24(staticNeutralBluish50),
		CodeBanner:   ui.RGB24(staticNeutralBluish50),
		InlineCode:   ui.RGB24(staticNeutralBluish50),
		CodeComment:  ui.RGB24(0x6a9955),
		CodeString:   ui.RGB24(0xa31515),
		CodeNumber:   ui.RGB24(0x098658),
		CodeKeyword:  ui.RGB24(0x0000ff),
		CodeFunction: ui.RGB24(0x795e26),
		CodeVariable: ui.RGB24(0x001080),

		Bubble:     ui.RGB24(staticDeepseek50), // --dsw-specific-bubble
		InputMajor: ui.RGB24(staticNeutralBluish00),

		Scrollbar:      ui.RGB24(staticNeutral200),
		ScrollbarHover: ui.RGB24(staticNeutral300),

		Success: ui.RGB24(staticGreen500),
		Error:   ui.RGB24(staticRed600),
		Warn:    ui.RGB24(staticAmber500),
		Info:    ui.RGB24(staticDeepseek500),

		// 亮色：tertiary = 100 档。
		SuccessTertiary: ui.RGB24(staticGreen100),
		SuccessPrimary:  ui.RGB24(staticGreen500),
		WarnTertiary:    ui.RGB24(staticAmber100),
		WarnLabel:       ui.RGB24(staticAmber600),

		Selection: ui.RGBA(59, 130, 246, 0.30),

		Radii: radii,
		Fonts: fonts(),
	}
}

// VivaTheme 把 dscore 的令牌翻成 viva 自带的 ui.Theme。
//
// 为什么要这一步：viva 的内建小部件（Field / Button / Divider / ScrollArea /
// Spinner）只认 ui.Theme。不给它一套和令牌同源的色，输入框、按钮、滚动条
// 就会是 viva 默认的灰蓝，和我们照抄的官方色打架——「外表像」就漏了。
func VivaTheme(t *pluginapi.Theme) ui.Theme {
	base := ui.DefaultMetrics()
	base.RadiusControl = t.Radii.SM
	return ui.Theme{
		Background:          t.BgBase,
		Foreground:          t.LabelPrimary,
		Card:                t.BgLayer1,
		CardForeground:      t.LabelPrimary,
		CardElevated:        t.BgLayer2,
		Popover:             t.BgLayer1,
		PopoverForeground:   t.LabelPrimary,
		Muted:               t.BgLayer2,
		MutedForeground:     t.LabelCaption,
		Border:              t.BorderL2,
		Hover:               t.BgLayer2,
		Input:               t.BorderL3,
		Primary:             t.ButtonPrimaryFill,
		PrimaryForeground:   t.ButtonPrimaryFg,
		Selection:           t.Selection,
		SelectionForeground: t.LabelPrimary,
		Secondary:           t.BgLayer2,
		SecondaryForeground: t.LabelSecondary,
		Accent:              t.NavHover,
		AccentForeground:    t.LabelPrimary,
		Ring:                t.Link,
		Success:             t.Success,
		Danger:              t.Error,
		DangerForeground:    ui.RGB24(staticNeutralBluish00),
		Chart: [5]ui.Color{
			t.Link, t.Info, t.CodeKeyword, t.Success, t.Warn,
		},
		Metrics:    base,
		Transition: ui.DefaultMotion(),
	}
}
