package dscommands

import (
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// draw 复刻官方 PopupSelectView：搜索框在上、选项 viewport 在下，
// 整张卡片贴 composer 顶边上方 4px，左右与 composer 对齐。
// 官方结构见 ui-commands/src/client/PopupSelectView.tsx:155-195；
// 几何全部集中在 geometry.go，并逐项带官方 file:line。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	if !p.shown || env == nil || env.Tree == nil {
		return
	}
	t := p.theme
	if t == nil {
		return
	}
	p.act = env.Act // 每帧刷新：壳可能中途接上新能力
	cs := p.filtered()

	// ScrollArea 的 preferred height 是 0（它本来用于固定父高度 + Grow）。官方这里
	// 却是「内容撑高、320 封顶」，所以用上一帧 layout 已量到的 Content/搜索框高
	// 决定这一帧高度；首帧量一次，第二帧即稳定，不猜一个 CSS 里不存在的行高。
	contentHeight := p.viewportContent
	chromeHeight := 2*commandPanelPad + p.searchHeight
	if p.searchHeight > 0 {
		chromeHeight += commandSearchMarginTop + commandSearchMarginBottom
	}
	viewportHeight := contentHeight
	if max := commandPanelMaxHeight - chromeHeight; viewportHeight > max {
		viewportHeight = max
	}
	if viewportHeight < 0 {
		viewportHeight = 0
	}
	if p.searchHeight == 0 && p.viewportContent == 0 {
		// 首次必须给 ScrollArea 一个非零视口，才能量到完整 Content；外层已有
		// 320 cap，下一帧即按真实内容收缩（短列表）或精确封顶（长列表）。
		viewportHeight = commandPanelMaxHeight
	}

	inputHeight := uikernel.ComposerInputMinH
	if env.Data.Cur == "" {
		inputHeight = uikernel.ComposerInputHeroH
	}

	// 官方 bottom:calc(100% + 4px) 以输入区顶边为基准；Viva 的 Bottom 从父盒底边
	// 向上量，因此要加回输入区高度。Left(0)+Right(0) 对应官方左右 inset，
	// 不再保留旧的固定 360px / 右上 56,24 自造几何。
	// 阴影是官方 `--dsw-elevation-prominent`，不是 Viva 的 ShadowMD：
	// 官方 PopupSelectView.module.css:28-29 写的是
	//
	//	--dsw-elevation-stroke-color: var(--dsw-alias-border-l1);
	//	box-shadow: var(--dsw-elevation-prominent);
	//
	// 而 `gfx.ShadowMD` 是 Tailwind 的 `0 4px 6px -1px .10 + 0 2px 4px -2px .10`
	// （viva@v0.6.4/gfx/canvas.go:123），没有发丝描边、位移/模糊/透明度也都不对。
	// 它画在 afterDraw 里，原因见那里。
	panelRadius := t.Radii.MD
	panel := ui.Column(f).Absolute().Left(0).Right(0).
		Bottom(inputHeight + commandPanelAnchorGap).
		MaxHeight(commandPanelMaxHeight).Clip().
		Radius(panelRadius).Background(t.BgLayer3).
		Pad(commandPanelPad).Gap(0)
	p.panelBox = panel
	panel.Children(func() {
		search := ui.TextFieldBase(f, &p.search, commandSearchAria, commandSearchPlaceholder).
			FillWidth().
			MarginSides(commandSearchMarginTop, commandSearchMarginRight,
				commandSearchMarginBottom, commandSearchMarginLeft).
			PadSides(commandSearchPadY, commandSearchPadX, commandSearchPadY, commandSearchPadX).
			Border(commandSearchBorder, ui.Color{}).Radius(t.Radii.MD).
			Background(ui.Color{}).FontSize(commandSearchFont).Color(t.LabelPrimary).
			AutoFocus()
		p.searchBox = search
		search.OnKey(func(ev gfx.KeyEvent) gfx.EventResult {
			if p.HandleKey(ev) {
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		})
		if search.Changed() {
			p.cursor = 0
		}
		if len(cs) == 0 {
			ui.Row(f).FillWidth().
				PadSides(commandStatusPadY, commandStatusPadX, commandStatusPadY, commandStatusPadX).
				Children(func() {
					ui.Text(f, "没有匹配的命令").FontSize(commandStatusFont).Color(t.LabelTertiary)
				})
			return
		}
		// 官方 .viewport{min-height:0;overflow-y:auto}（PopupSelectView.module.css:33-38）。
		viewport := ui.Scroll(f).MinHeight(0).Height(viewportHeight).Scrollbars()
		p.viewportState = viewport.ScrollState()
		viewport.Children(func() {
			for i, c := range cs {
				i, c := i, c
				row := ui.Row(f).FillWidth().Items(ui.Center).Gap(commandRowGap).
					PadSides(commandRowPadY, commandRowPadX, commandRowPadY, commandRowPadX).
					Radius(t.Radii.MD).Hover(ui.Look{}.Background(t.Hover)).
					Hit(env.Tree.ID(env.Feat, "command-option-"+itoa(i)), c.Name)
				if i == p.cursor {
					// 官方 .rowActive 只改背景，不把文字改成 Info 色（同 CSS :51-53）。
					row = row.Background(t.Hover)
				}
				row.Children(func() {
					drawCommandIcon(f, c.Name, t.LabelTertiary)
					ui.Text(f, c.Name).FontSize(commandRowFont).Color(t.LabelPrimary).Ellipsis()
					if c.Hint != "" {
						ui.Text(f, c.Hint).FontSize(commandDetailFont).Color(t.LabelTertiary).
							Grow(1).MinWidth(0).Ellipsis()
					}
				})
				if row.Clicked() && c.Action != nil {
					c.Action()
					p.shown = false
				}
			}
		})
	})
}

// afterDraw 在当前 paint 仍有效时读取布局结果，只把数值带到下一帧。
// 这是官方「内容撑高 + max-height」与 Viva「ScrollArea preferred height=0」之间的桥。
func (p *Plugin) afterDraw(canvas *gfx.Canvas, _ *pluginapi.Env) {
	if !p.shown {
		return
	}
	if p.viewportState != nil {
		p.viewportContent = p.viewportState.Content
	}
	if p.searchBox.Rect().Height > 0 {
		p.searchHeight = p.searchBox.Rect().Height
	}
	// 官方 elevation 只能在这里画：面板自己 `.Clip()`（官方
	// `.popup{overflow:hidden}`，PopupSelectView.module.css:22），而 Viva 对带
	// Clip 的元素是**先裁再跑 Draw 钩子**（ui/paint.go:50-52 推 Clip、:132 才
	// 调钩子），阴影记进 canvas 时带着那道裁剪（gfx/canvas.go:1169 的
	// clipSnapshot），画出来是空的——这正是原来 `Shadow(gfx.ShadowMD)` 在这块
	// 面板上一直等于没画的原因。AfterDraw 是布局完成后的覆画阶段
	// （pluginapi/plugin.go:286-292），那时裁剪栈已清空。
	if rect := p.panelBox.Rect(); rect.Width > 0 && p.theme != nil {
		dscore.PaintElevationOverlay(canvas, rect, p.theme.Radii.MD,
			p.theme.Shadows.Prominent, p.theme.BorderL1)
	}
}
