// model_slot.go —— 官方 `conversation.input.model` 槽的**占用方**（模型 trigger）。
//
// # 官方事实（rc.2 `ui-model-selection/src/client/**`）
//
//	conversation.input.model          single/session   index.ts:177
//	├─ conversation.input.model.label single/inline
//	└─ conversation.input.model.options list/session    ← 菜单里那一次 renderSlot
//
// 官方由 `ui-model-selection` 的 `ModelSelect` 占用这个槽，它同时画两样东西：
//
//	button.trigger   28 高、padding 0 4px 0 8px、gap 4、13/20/400、radius sm、
//	                 max-width min(360px, 45cqw)；展开时底 = interactive-bg-hover
//	                 （`ModelSelect.module.css:11-38`）
//	MenuSurface      min 240 / max 420 / max-height 360 / padding 4 / radius md
//	                 （`ModelSelect.module.css:107-135`；画法在
//	                 `internal/uikernel/official_model_menu.go` 的 DrawModelMenu）
//
// # 为什么要占用这个槽（本任务的核心）
//
// 改造前：模型名是 `plugins/dscomposer` 在控件行里**硬编码画的只读 chip**，
// 而 dsagentteam 的官方模型菜单只挂了一块透明命中区（`parent_containers.go`）——
// 因为一旦直接 dispatch 这个槽，画布上就会出现**两个模型标签**（旧 chip + 官方
// trigger）。GR 报告 §8 第 1 条点名了这件事：正确做法是**把 chip 本身迁成槽位占用方**。
//
// 现在：`conversation.input.model` 有占用方（本文件）时，composer 只派发、不画 chip；
// 没有占用方时 composer 才画自己的只读 chip（`dscomposer` 的 drawModelChip）。
// **同帧只有一个活**，判据是内核的 `env.Has(OffInputModel)`。
//
// # 降级（两种，都不编 0）
//
//	provider 目录可用（known 且非空）  → 官方 trigger：可真值、可点、开菜单
//	目录未知 / 空目录                   → **只读 chip**：显示宿主真值（`Snapshot.Model`），
//	                                     宿主没给就写「未知」；**不画 chevron、不给命中区**
//	                                     （点了会打开空菜单 = 假入口，官方 available=false 时
//	                                     trigger 也不可用）
//
// # 菜单锚点（trigger 自己的矩形，不再靠过渡几何）
//
// 官方 trigger 用 `anchor-name: --dsh-model-anchor`，菜单 `bottom:calc(anchor(top)+8px)`。
// viva 的 `Box.Rect()` 要等布局解算（End）之后才有效，而 Popup 必须在本帧 build
// 阶段声明——所以：Draw 阶段记下 trigger 的 Box，**AfterDraw** 读矩形存起来，
// 下一帧 Children 画菜单时用它当锚点。用户点开菜单那一刻，trigger 已经至少画过
// 一帧，所以位置是准的；只有程序化打开（测试/首帧）才退回过渡几何
// （`modelTriggerRect`）：那条路径没人点，位置不准也不影响交互。
package dsagentteam

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 `ModelSelect.module.css:11-38` 的 trigger 几何（逐条注出处）。
const (
	// modelChipHeight 是 `.trigger{height:28px}`（:22）。
	modelChipHeight float32 = 28
	// modelChipPadLeft 是 `.trigger{padding:0 4px 0 8px}` 的左 8（:23）。
	modelChipPadLeft float32 = 8
	// modelChipPadRight 是同一行的右 4：官方右边那一格留给 chevron。
	// 降级态不画 chevron，用 modelChipPadRightBare（8）补回视觉平衡。
	modelChipPadRight     float32 = 4
	modelChipPadRightBare float32 = 8
	// modelChipGap 是 `.trigger{gap:4px}`（:24）。
	modelChipGap float32 = 4
	// modelChipFont / modelChipLine 是 `.trigger{font-size:13px;line-height:20px;font-weight:400}`（:25-27）。
	modelChipFont float32 = 13
	modelChipLine float32 = 20
	// modelChevronSize 是官方 trigger 里 chevron 的绘制尺寸（`IconChevronDownOutlineRegular`，
	// 与 `IconChevronDownOutlineRegular size=14` 同档；见 icons.go 的素材说明）。
	modelChevronSize float32 = 14
	// modelChipMaxW 是 `.trigger{max-width:min(360px,45cqw)}`（:30）里 360 那一档。
	modelChipMaxW float32 = 360
	// modelChipCardRatio 是同一处 45cqw：cqw 是 composer 卡片宽（官方 45cqw）。
	modelChipCardRatio float32 = 0.45
	// modelCardExtra / modelContentMin / modelContentMax / modelContentRatio 与
	// composer 的 `--dsh-chat-content-width` / `--dsh-composer-card-max-width` 同源
	// （`ConversationRoot.module.css`）：卡片宽 = clamp(680, 主列*0.64, 920) + 32。
	modelContentMin   float32 = 680
	modelContentMax   float32 = 920
	modelContentRatio float32 = 0.64
	modelCardExtra    float32 = 32

	// zhModelUnknown 是宿主没给当前模型时的显示文案（与 dscomposer 的上下文
	// 「未知」同一口径：结构位置存在，就写「未知」，**不编一个默认型号**）。
	zhModelUnknown = "未知"
)

// modelSlotState 是 `conversation.input.model` 座位的跨帧几何。
//
// chip 是 Draw 阶段量到的 trigger 盒子；Rect 要等 End 之后才有效，所以 AfterDraw
// 才读（同 dscomposer 的 fieldBox/afterDraw 做法）。rect 只用于下一帧的菜单锚点。
//
// panel/panelDrawn 是**菜单面**的同一套：v0.6.0 的 Absolute 子元素高度由内容
// 决定，声明期拿不到，而"放不下翻到下方"的判定要用它（见
// `internal/uikernel/official_popup_place.go`）。
type modelSlotState struct {
	chip       ui.Box
	drawn      bool
	rect       ui.Rect
	panel      ui.Box
	panelDrawn bool
	panelRect  ui.Rect
}

// modelSlotAvailable 报告官方模型菜单这一帧能不能用（provider 目录 known 且非空）。
//
// 与 `DrawModelMenu` 的隐藏判据同一条（官方 `available`）：目录未知/空目录时
// 官方 trigger 也不可用，所以占用方只画降级 chip，不给命中区。
func (p *Plugin) modelSlotAvailable() (known bool, failure string, list []ModelProviderOption) {
	known, failure, list = p.team.Providers()
	return known && len(list) > 0, failure, list
}

// drawModelSlot 是本包在 `conversation.input.model` 槽上的 `Draw`：画 trigger（或降级 chip）。
//
// 两态（见文件头）：
//
//	可用   → 官方 trigger：文本 + chevron，`HitWith` 成 button，点它开/关菜单
//	不可用 → 只读 chip：文本（宿主真值 / 「未知」），没有 chevron、没有命中区
//
// # 为什么内边距用定宽占位盒而不是 PadSides
//
// viva 的换行判据是 `TextWidthWeight(line) <= rect.Width - padX()`。文本的期望宽度
// 恰好等于"父容器宽度 - 内边距"时，float32 往返（加内边距再减回来）会差 ~1e-7，
// 于是"放得下"被判成"放不下"，整行 CJK 文本被画成一个省略号（实测「未知」→「…」，
// ASCII 标签碰巧不受影响）。占位盒不参与那次加减，宽度是精确的。
//
//	左内边距 = 容器 gap + 左占位（8 = 4 + 4）
//	右内边距：可用态 4 = 4 + 0（chevron 已经吃掉 padding 右侧的 4）；
//	          降级态 8 = 4 + 4（没有 chevron，补回官方的视觉留白）
func (p *Plugin) drawModelSlot(f ui.C, env *pluginapi.Env) {
	if f == nil || env == nil {
		return
	}
	p.model.drawn = false
	t := p.themeFor(env)
	available, _, _ := p.modelSlotAvailable()
	label := modelSlotLabel(env)
	maxW := modelSlotMaxWidth(env)

	// 占位盒宽度：内边距减掉容器 gap（负值夹到 0）。
	inset := func(pad float32) float32 {
		if pad <= modelChipGap {
			return 0
		}
		return pad - modelChipGap
	}

	if !available {
		// 降级：只读 chip（宿主真值 / 「未知」）——不画 chevron、不给命中区。
		ui.Row(f).Height(modelChipHeight).MaxWidth(maxW).Radius(t.Radii.SM).
			Items(ui.Center).Gap(modelChipGap).Children(func() {
			ui.Row(f).Width(inset(modelChipPadLeft))
			ui.Row(f).MaxWidth(maxW - modelChipPadLeft - modelChipPadRightBare).Children(func() {
				ui.Text(f, label).FontSize(modelChipFont).LineHeight(modelChipLine).
					Weight(gfx.WeightRegular).Color(t.LabelSecondary).Ellipsis()
			})
			ui.Row(f).Width(inset(modelChipPadRightBare))
		})
		return
	}

	open := p.team.ModelMenuOpen()
	textMax := maxW - modelChipPadLeft - modelChipPadRight - modelChevronSize - modelChipGap
	if textMax < 0 {
		textMax = 0
	}
	chip := ui.Row(f).Height(modelChipHeight).MaxWidth(maxW).Radius(t.Radii.SM).
		Items(ui.Center).Gap(modelChipGap)
	if open {
		// `.trigger[aria-expanded=true]{background:interactive-bg-hover}`（:34-38）。
		chip = chip.Background(t.Hover)
	} else {
		chip = chip.Hover(ui.Look{}.Background(t.Hover))
	}
	chip = chip.Children(func() {
		ui.Row(f).Width(inset(modelChipPadLeft))
		ui.Row(f).MaxWidth(textMax).Children(func() {
			ui.Text(f, label).FontSize(modelChipFont).LineHeight(modelChipLine).
				Weight(gfx.WeightRegular).Color(t.LabelSecondary).Ellipsis()
		})
		ui.Icon(f, chevronDownIcon()).Size(modelChevronSize, modelChevronSize).Color(t.LabelSecondary)
		ui.Row(f).Width(inset(modelChipPadRight))
	})
	// 官方 trigger 是 button（`aria-haspopup` + 展开态）；点它开/关菜单。
	chip = chip.HitWith(env.Tree.ID(env.Feat, "model-menu-trigger"), zhModelMenuAria,
		gfx.HitOptions{Role: gfx.RoleButton, Expandable: true, Expanded: open})
	p.model.chip, p.model.drawn = chip, true
	if chip.Clicked() {
		p.team.ToggleModelMenu()
	}
}

// afterModelSlot 把本帧 trigger 的**真实矩形**留给下一帧的菜单锚点。
func (p *Plugin) afterModelSlot(canvas *gfx.Canvas, env *pluginapi.Env) {
	if p.model.drawn {
		p.model.drawn = false
		if rect := p.model.chip.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.model.rect = rect
		}
	}
	// 菜单面的高度同理：这一帧量到，下一帧给 DrawModelMenu 做翻面判定。
	if p.model.panelDrawn {
		p.model.panelDrawn = false
		if rect := p.model.panel.Rect(); rect.Width > 0 && rect.Height > 0 {
			p.model.panelRect = rect
		} else {
			// 菜单这一帧没画出来（关掉了/目录未知）：高度清零，下次打开按
			// "还没量过"处理，不要拿着上一次的高度去翻面。
			p.model.panelRect = ui.Rect{}
		}
	}
}

// modelAnchor 返回菜单锚点：优先上一帧 trigger 的真实矩形，还没画过才用过渡几何。
func (p *Plugin) modelAnchor(env *pluginapi.Env) ui.Rect {
	if r := p.model.rect; r.Width > 0 && r.Height > 0 {
		return r
	}
	return modelTriggerRect(env)
}

// modelSlotLabel 是 trigger 上的文字：宿主真值优先；宿主没给就写「未知」。
//
// 不拿会话 mode、不拿 provider 名、不拿"默认模型"顶替——那些都不是"当前模型"。
func modelSlotLabel(env *pluginapi.Env) string {
	if env == nil {
		return zhModelUnknown
	}
	if model := strings.TrimSpace(env.Data.Model); model != "" {
		return model
	}
	return zhModelUnknown
}

// modelSlotMaxWidth 按官方 `.trigger{max-width:min(360px,45cqw)}` 算上限。
//
// cqw 是 composer 卡片宽：卡片宽 = clamp(680, 主列*0.64, 920) + 32（`--dsh-chat-content-width`
// 与 `--dsh-composer-card-max-width` 的关系）。主列宽度取壳算好的 Geometry.MainWidth。
func modelSlotMaxWidth(env *pluginapi.Env) float32 {
	column := float32(0)
	if env != nil {
		column = env.Geometry.MainWidth
		if column <= 0 {
			column = env.MainWidth
		}
	}
	if column <= 0 {
		return modelChipMaxW
	}
	content := column * modelContentRatio
	if content < modelContentMin {
		content = modelContentMin
	}
	if content > modelContentMax {
		content = modelContentMax
	}
	card := content + modelCardExtra
	if card > column {
		card = column
	}
	maxW := card * modelChipCardRatio
	if maxW > modelChipMaxW {
		maxW = modelChipMaxW
	}
	if maxW <= 0 {
		return modelChipMaxW
	}
	return maxW
}
