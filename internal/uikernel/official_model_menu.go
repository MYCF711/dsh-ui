package uikernel

// official_model_menu.go —— 官方 composer 的**模型菜单父容器**（本任务的"容器 C"）。
//
// # 官方事实（rc.2 随包 src/**）
//
//	conversation.input.model          single/session  ui-model-selection/src/client/index.ts:177
//	├─ conversation.input.model.label single/session  同一处 children 声明
//	└─ conversation.input.model.options list/session  同一处 children 声明
//
// 渲染点：`ModelSelect.tsx:505` 的 `renderSlot('conversation.input.model.options', …)`
// 落在菜单 **root pane** 里（模型 cell / 推理等级 cell 之后）。本容器就是那个
// root pane 所在的菜单面（官方 DOM 在 `ModelSelect.tsx:479-505`）：
//
//	button.trigger                                   ModelSelect.module.css:11-38
//	└─ (open) MenuSurface.menu（portal 到 body）       ModelSelect.module.css:107-135
//	   ├─ button.cell  「模型」 + 当前值 + chevron     :340-390
//	   ├─ button.cell  「推理等级」（有 reasoning 才画）同一处
//	   └─ renderSlot('conversation.input.model.options')   ← 本文件负责 dispatch 这个子槽
//
// # 本文件画一半、不画另一半（写清边界，别当成"全做了"）
//
//   - **画**：菜单面（min 240 / max 420 / max-height 360 / padding 4 / 圆角 md /
//     elevation-prominent）+ 一级 cell 行 + provider 行 + error/empty 行 + 子槽 dispatch。
//   - **不画**：trigger。官方 trigger 属于 `conversation.input.model` 自己的组件
//     （ModelSelect 的 button），本仓那个座位由 composer 侧（`plugins/dscomposer`）
//     画成只读 chip；再画一枚就是同屏两个模型名。trigger 的**命中区**由挂载方
//     （`plugins/dsagentteam` 的父容器 painter）盖在同一位置，点它开这个菜单。
//
// # 数据真假边界
//
//	provider 列表未知（没接线 / 501 / 请求失败）  → 整段不画（Known=false）
//	provider 列表为空（宿主真的没有）              → 整段不画（官方也没有空菜单）
//	取列表失败（FetchFailure 非空）                → 画官方 .error 行（不编造 provider）
//	provider 行自己带 error                        → 照实画成不可用行（不隐藏真值）
//
// provider 行**不可点**：官方 model 行是 provider+model(+effort) 的完整选择，
// 本仓宿主目前只给 provider 目录，点一个 provider 选不出 model 来——所以宁可不给
// 命中区，也不做一个"点了没反应/假装选中"的假下拉。

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// 官方 composer 模型菜单相关的两个子槽（槽表里由 dsagentteam 登记进
// officialSlots；这里给出常量，避免各处再写一遍字面量）。
const (
	// OffInputModelOptions 是 `conversation.input.model.options`（list/session）。
	OffInputModelOptions pluginapi.Slot = "conversation.input.model.options"
	// OffInputModelLabel 是 `conversation.input.model.label`（single/inline）。
	OffInputModelLabel pluginapi.Slot = "conversation.input.model.label"
)

// 官方几何（`ui-model-selection/src/client/ModelSelect.module.css`，逐行有出处）。
const (
	// ModelMenuMinWidth / MaxWidth / MaxHeight 是 `.menu` 的
	// min-width:min(240px,100vw-24px) / max-width:min(420px,100vw-24px) /
	// max-height:min(360px,100dvh-96px)（:107-135）。
	ModelMenuMinWidth  float32 = 240
	ModelMenuMaxWidth  float32 = 420
	ModelMenuMaxHeight float32 = 360
	// ModelMenuPad 是 `.menu{padding:4px}`（:132）。
	ModelMenuPad float32 = 4
	// ModelMenuEdgeMargin 是 `calc(100vw - 24px)` 里的 12+12（两侧各 12）。
	ModelMenuEdgeMargin float32 = 12
	// ModelMenuTopMargin 是 `100dvh - 96px` 里的 96（上方留白）。
	ModelMenuTopMargin float32 = 96
	// ModelMenuAnchorGap 是 `bottom: calc(anchor(top) + 8px)` 的 8（:118）。
	ModelMenuAnchorGap float32 = 8
	// ModelTriggerHeight 是 `.trigger{height:28px}`（:22），与权限 trigger 同档。
	ModelTriggerHeight float32 = 28
	// ModelMenuCellHeight 是 `.cell{height:34px}`（:347）。
	ModelMenuCellHeight float32 = 34
	// ModelMenuCellPadX 是 `.cell{padding:0 8px}`（:348）。
	ModelMenuCellPadX float32 = 8
	// ModelMenuCellGap 是 `.cell{gap:6px}`（:344）。
	ModelMenuCellGap float32 = 6
	// ModelMenuCellFont / Line 是 `.cell{font-size:13px;line-height:20px}`（:353-354）。
	ModelMenuCellFont float32 = 13
	ModelMenuCellLine float32 = 20
	// ModelMenuStatusPad / Font / Line 是 `.status,.empty{padding:8px;12px;18px}`（:138-144）。
	ModelMenuStatusPad  float32 = 8
	ModelMenuStatusFont float32 = 12
	ModelMenuStatusLine float32 = 18
	// ModelMenuErrorPadX / PadY 是 `.error{padding:6px 7px}`（:151）。
	ModelMenuErrorPadX float32 = 7
	ModelMenuErrorPadY float32 = 6
	// ModelMenuErrorFont / Line 是 `.error{font-size:11px;line-height:16px}`（:154-155）。
	ModelMenuErrorFont float32 = 11
	ModelMenuErrorLine float32 = 16
	// ModelMenuSeedRow 是 provider 行里的次要说明行（settingsNs / declared）；
	// 官方 model 行没有这一层，这里只用 12/18 —— 与 `.status` 同一档字号。
	ModelMenuSeedFont float32 = 12
	ModelMenuSeedLine float32 = 18
)

// 官方文案（`ui-model-selection/src/client/locales.ts` zh 字典，逐字）。
const (
	zhModelMenuAria     = "模型与推理等级"
	zhModelMenuModel    = "模型"
	zhModelProviderNone = "宿主没有可配置的模型提供方"
	zhModelProviderFail = "读取模型提供方失败：{message}"
	zhModelProviderBare = "未在模型目录中声明"
)

// ModelProviderOption 是本容器渲染的一行 provider（界面视图）。
//
// 只带"这一帧要显示什么"，不带请求参数：界面的数据面只有
// `apiclient.ConfigurableProvider`（经 `plugins/dsagentteam` 的 Store 喂进来）。
type ModelProviderOption struct {
	// Provider / DisplayName 是官方 `provider` / `displayName`。
	Provider    string
	DisplayName string
	// Seed 是行下方的次要说明（settingsNs，或"未在模型目录中声明"）。
	Seed string
	// Failure 是官方 `ConfigurableProvider.error`：非空 = 这一项不可用。
	Failure string
}

// ModelMenuOptions 是画一次模型菜单父容器的全部输入。
type ModelMenuOptions struct {
	Theme *pluginapi.Theme
	// Anchor 是 trigger 的矩形（**窗口坐标**）。菜单贴在它上方 8px、右缘对齐
	// （官方 `position-anchor` + `bottom:calc(anchor(top)+8px)` + `right:anchor(right)`）。
	Anchor ui.Rect
	// Bounds 是整窗矩形，用来夹 max-height / max-width（官方也夹视口）。
	Bounds ui.Rect
	// Open 是菜单展开态（官方 `open`）。
	Open bool
	// Known 表示 provider 列表已知。false = 没接线/没取到 → 整段隐藏。
	Known bool
	// Providers 是宿主给的真值列表（顺序照宿主，不自己排序）。
	Providers []ModelProviderOption
	// FetchFailure 是取列表失败的原因；非空时画官方 .error 行。
	FetchFailure string
	// Selected 是当前选择（官方 currentChoice）；空 = 未知，不打勾。
	Selected string
	// Locked 表示本会话当前不允许改选择（官方 locked/busy）。
	Locked bool
	// Dispatch 把子槽交给内核画（**必须**是内核的 DrawSlot：这就是"父容器"的意义）。
	Dispatch func(pluginapi.Slot)
}

// ModelMenuWidth 按官方 `.menu` 规则算宽度：fit-content 但夹在
// min(240, 100vw-24) 与 min(420, 100vw-24) 之间。
func ModelMenuWidth(boundsWidth float32) float32 {
	available := boundsWidth - 2*ModelMenuEdgeMargin
	if available < 0 {
		available = 0
	}
	width := ModelMenuMaxWidth
	if available < width {
		width = available
	}
	if width < ModelMenuMinWidth && available >= ModelMenuMinWidth {
		width = ModelMenuMinWidth
	}
	if width < 0 {
		width = 0
	}
	return width
}

// ModelMenuMaxHeightFor 按官方 `.menu` 规则算最大高度：
// min(360px, 100dvh - 96px)。
func ModelMenuMaxHeightFor(boundsHeight float32) float32 {
	available := boundsHeight - ModelMenuTopMargin
	if available < 0 {
		available = 0
	}
	if available < ModelMenuMaxHeight {
		return available
	}
	return ModelMenuMaxHeight
}

// DrawModelTrigger 画官方 trigger 的**命中区**（点它开/关菜单）。
//
// 为什么命中区单独一个函数：viva 的 `Painter.Draw` 拿不到内核的 drawSlot
// （只有 `Painter.Children` 有），所以父容器被拆成两半——
//
//	Draw     ← 本函数：trigger 命中区（不含任何 dispatch）
//	Children ← DrawModelMenu：菜单面 + 子槽 dispatch
//
// 两半都做实事（不是"占位 Draw"）：uikernel 的装配校验明确拒绝"既没 Draw 也没
// AfterDraw 的画"，因为那种画在界面上等于不存在。
//
// 返回 drawn=本帧画了命中区；clicked=这一帧点中了它（调用方据此切展开态）。
func DrawModelTrigger(parent flex.Box, env *pluginapi.Env, opts ModelMenuOptions) (drawn, clicked bool) {
	if env == nil || env.Tree == nil {
		return false, false
	}
	// 没有目录就不给假入口（官方 available=false 时 trigger 不可用）。
	if !opts.Known || len(opts.Providers) == 0 {
		return false, false
	}
	if opts.Anchor.Width <= 0 || opts.Anchor.Height <= 0 {
		return false, false
	}
	box := parent.Size(opts.Anchor.Width, opts.Anchor.Height).HitWith(
		env.Tree.ID(env.Feat, "model-menu-trigger"), zhModelMenuAria,
		ui.HitOptions{Role: ui.RoleButton, Expandable: true, Expanded: opts.Open},
	)
	return true, box.Clicked()
}

// DrawModelMenu 画模型菜单父容器，并把子槽 `conversation.input.model.options`
// 真的 dispatch 出去。返回 true 表示这一帧画了菜单面（false = 四态里的"隐藏"）。
//
// 隐藏的三条（都不画任何像素）：
//
//	!opts.Open                       菜单没展开
//	!opts.Known                      provider 列表未知（宿主没这个能力）
//	len(Providers)==0 且无 FetchFailure 宿主真的没有可配置 provider
func DrawModelMenu(f *flex.Frame, env *pluginapi.Env, opts ModelMenuOptions) bool {
	if f == nil || !opts.Open || !opts.Known {
		return false
	}
	if len(opts.Providers) == 0 && opts.FetchFailure == "" {
		return false
	}
	t := opts.Theme
	if t == nil && env != nil {
		t = env.Theme
	}
	if t == nil {
		return false
	}
	width := ModelMenuWidth(opts.Bounds.Width)
	if width <= 0 {
		return false
	}
	maxHeight := ModelMenuMaxHeightFor(opts.Bounds.Height)

	// 官方 `.menu` 是 portal 出去的一层：不占位、盖在其它节点上、被夹在框内。
	menu := f.Popup().Above(opts.Anchor, ModelMenuAnchorGap, flex.End).Width(width)
	menu.MaxHeight(maxHeight)
	menu.Radius(t.Radii.MD).Background(t.BgLayer3).Border(1, t.BorderL1).
		Shadow(ui.ShadowMD).Pad(ModelMenuPad).Gap(0).Children(func() {
		if opts.FetchFailure != "" {
			// 官方 `.error`：11/16 + error 色 + 6px 7px 内边距 + md 圆角。
			f.Row().FillWidth().Radius(t.Radii.MD).
				PadSides(ModelMenuErrorPadY, ModelMenuErrorPadX, ModelMenuErrorPadY, ModelMenuErrorPadX).
				Children(func() {
					f.Text(formatMenuMessage(zhModelProviderFail, opts.FetchFailure)).
						FontSize(ModelMenuErrorFont).LineHeight(ModelMenuErrorLine).Color(t.Error).
						Ellipsis()
				})
		}
		for _, provider := range opts.Providers {
			drawModelProviderRow(f, t, provider, opts.Selected)
		}
		// 官方：`renderSlot('conversation.input.model.options', …)` 在 root pane 里、
		// 一二级 cell 之后。本仓的 Team 开关（ds-agent-team 槽 ②）就落在这里。
		if opts.Dispatch != nil {
			opts.Dispatch(OffInputModelOptions)
		}
	})
	return true
}

// drawModelProviderRow 画一行 provider（官方 `.option`：min-height 34 / padding 5·7 /
// gap 6 / 圆角 md；这里按 `.cell` 的 34 高 + 0·8 横向，因为它是只读目录行）。
func drawModelProviderRow(f *flex.Frame, t *pluginapi.Theme, provider ModelProviderOption, selected string) {
	name := provider.DisplayName
	if name == "" {
		// 官方 displayName 必有；缺了就用 provider id（**不编**一个人话名字）。
		name = provider.Provider
	}
	label := t.LabelPrimary
	if provider.Failure != "" {
		label = t.Error
	} else if selected != "" && selected == provider.Provider {
		label = t.LabelPrimary
	}
	f.Column().FillWidth().PadSides(0, ModelMenuPad, 0, ModelMenuPad).Children(func() {
		f.Row().FillWidth().Height(ModelMenuCellHeight).Items(flex.Center).
			PadSides(0, ModelMenuCellPadX, 0, ModelMenuCellPadX).Gap(ModelMenuCellGap).Children(func() {
			f.Text(name).FontSize(ModelMenuCellFont).LineHeight(ModelMenuCellLine).Color(label).Ellipsis()
		})
		if provider.Failure != "" {
			f.Row().FillWidth().PadSides(0, ModelMenuCellPadX, 0, ModelMenuCellPadX).Children(func() {
				f.Text(provider.Failure).FontSize(ModelMenuSeedFont).LineHeight(ModelMenuSeedLine).
					Color(t.Error).Ellipsis()
			})
		} else if seed := provider.Seed; seed != "" {
			f.Row().FillWidth().PadSides(0, ModelMenuCellPadX, ModelMenuPad, ModelMenuCellPadX).Children(func() {
				f.Text(seed).FontSize(ModelMenuSeedFont).LineHeight(ModelMenuSeedLine).
					Color(t.LabelTertiary).Ellipsis()
			})
		}
	})
}

// formatMenuMessage 只替换 `{message}`（官方 i18n 的最小子集，与插件侧同规则）。
func formatMenuMessage(template, value string) string {
	out := make([]byte, 0, len(template)+len(value))
	for i := 0; i < len(template); {
		if template[i] == '{' && len(template)-i >= len("{message}") && template[i:i+len("{message}")] == "{message}" {
			out = append(out, value...)
			i += len("{message}")
			continue
		}
		out = append(out, template[i])
		i++
	}
	return string(out)
}
