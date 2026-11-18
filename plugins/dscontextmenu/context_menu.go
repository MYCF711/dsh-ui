package dscontextmenu

// context_menu.go —— 官方 `dsh-client-ui-context-menu` 的 Go+Viva 等价实现。
//
// 视觉来源（rc.2；context-menu/src 本身没有 module.css，Menu 来自 primitives 的
// 打包 CSS `_70kcw_` / `_1w4el_`）：
//   - `_list_70kcw_7`：padding 4、min 144 / max 360、elevation-prominent。
//   - `_surface_1w4el_1`：radius-lg=16、menu-surface-fill、backdrop material。
//   - `_item_70kcw_173`：min-height 34、6×8 padding、6 gap、13/20、radius-md。
//   - `_itemIcon_70kcw_233`：14×14；`_separator_70kcw_157`：0.5px、3×8 margin。
//
// Viva 内建菜单使用 32px 行、16px 图标、9px 分隔线和 ShadowMD，不能表达上述
// 官方值，因此本插件用 Overlay 画同一套交互（右键 / Shift+F10 / 锚点按钮），
// 不再把内建 shadcn 菜单的近似几何冒充官方。

import (
	"fmt"
	"sort"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	menuPadding          float32 = 4   // primitives-menu-official.css:2
	menuMinWidth         float32 = 144 // :3
	menuMaxWidth         float32 = 360 // :3
	menuViewportMargin   float32 = 12  // :5,55
	menuAnchorGap        float32 = 4   // :5
	menuRadius           float32 = 16  // menu-material-official.css:1；base.css:21 radius-lg
	menuItemHeight       float32 = 34  // primitives-menu-official.css:19
	menuItemPadY         float32 = 6   // :19
	menuItemPadX         float32 = 8   // :19
	menuItemGap          float32 = 6   // :19
	menuItemFont         float32 = 13  // :19
	menuItemLine         float32 = 20  // :19
	menuItemRadius       float32 = 12  // :19；base.css:20 radius-md
	menuItemIcon         float32 = 14  // primitives-menu-official.css:30-31
	menuDisabledOpacity  float32 = 0.4 // :29
	menuSeparatorHeight  float32 = 0.5 // :44
	menuSeparatorMarginY float32 = 3   // :44
	menuSeparatorMarginX float32 = 8   // :44
	menuButtonIcon       float32 = 16  // Button.tsx:49；sprite 默认 size=16
	// 这里原来还有 menuShadowOffsetY / menuShadowBlur / menuShadowAlpha /
	// menuGlowBlur / menuGlowAlpha / menuStroke 六个本地常量（官方
	// `--dsw-elevation-prominent` 的三个数）。它们已并进 `pluginapi.Shadows.Prominent`：
	// 官方菜单面 `._list_70kcw_7,._submenu_70kcw_8` 与 `MenuView.module.css`
	// 的 `.pc3rQa_menu` 都是 `box-shadow:var(--dsw-elevation-prominent)`。
)

type popupState struct {
	open       bool
	point      ui.Rect
	fromAnchor bool
	focused    bool
	// actions / rowIDs / highlight 是**键盘导航**的全部状态。
	//
	// 为什么插件自己记高亮，而不是全靠 `menu.Shortcut(...)`：Viva 只从
	// `ui.Page.Key` 派发 `ui.Box.Shortcut`（`viva@v0.6.4/ui/page.go:1090-1093`），
	// 而本壳的 `cfg.OnKey`（`cmd/app/main.go` 的 OnKey）只走
	// `kernel.HandleKey` + 自绘页面，从不调用 `Page.Key`——所以那几条 Shortcut
	// 在真装配下永不触发（↑↓ 靠 Viva 的焦点路由器还能动，Shift+F10 完全打不开）。
	// 这里改用**插件自己的** `pluginapi.KeyHandler`（壳已经每帧调它），
	// 官方语义（`_item:focus-visible{background:interactive-bg-hover}`）由
	// `highlight` 直接落到画面上，不依赖 Viva 的 `:focus-visible` 判定。
	actions   []pluginapi.ContextMenuAction
	rowIDs    []gfx.ID
	highlight int
	keyboard  bool    // 上一次是键盘在操作（焦点跟着 highlight 走）
	rect      ui.Rect // 目标元素的矩形（Shift+F10 / 键盘打开时贴它下方弹）
}

// Plugin 是 context menu 服务插件。它没有自己的常驻可见面：菜单由消费方
// 在目标元素上声明（Attach/Button），这正是官方「纯 UI 基础设施」的形状。
type Plugin struct {
	theme *pluginapi.Theme

	mu       sync.RWMutex
	byKind   map[string][]pluginapi.ContextMenuContribution
	buttons  map[string]*popupState
	contexts map[gfx.ID]*popupState
	disposed bool
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-contextmenu" }

// Provides 实现 pluginapi.Plugin：只提供菜单服务。
func (*Plugin) Provides() []string { return []string{pluginapi.KeyContextMenus} }

// Requires 实现 pluginapi.Plugin：按钮需要主题令牌。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 实现 pluginapi.Plugin。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.byKind = map[string][]pluginapi.ContextMenuContribution{}
	p.buttons = map[string]*popupState{}
	p.contexts = map[gfx.ID]*popupState{}
	if err := RegisterOfficialSlots(); err != nil {
		return err
	}
	ctx.Provide(pluginapi.KeyContextMenus, pluginapi.ContextMenus(p))
	return nil
}

// RegisterContribution 实现 pluginapi.ContextMenus：登记一个 target kind 的贡献。
func (p *Plugin) RegisterContribution(contribution pluginapi.ContextMenuContribution) (func(), error) {
	if contribution.Target == "" {
		return nil, fmt.Errorf("context menu: contribution %q 没有 target kind", contribution.ID)
	}
	if contribution.ID == "" {
		return nil, fmt.Errorf("context menu: target %q 的贡献没有 id", contribution.Target)
	}
	if contribution.Resolve == nil {
		return nil, fmt.Errorf("context menu: 贡献 %s/%s 没有 Resolve", contribution.Target, contribution.ID)
	}
	p.mu.Lock()
	if p.disposed {
		p.mu.Unlock()
		return nil, fmt.Errorf("context menu: registry 已卸载")
	}
	for _, existing := range p.byKind[contribution.Target] {
		if existing.ID == contribution.ID {
			p.mu.Unlock()
			return nil, fmt.Errorf("context menu: target %q 的贡献 id %q 重复", contribution.Target, contribution.ID)
		}
	}
	p.byKind[contribution.Target] = append(p.byKind[contribution.Target], contribution)
	sort.SliceStable(p.byKind[contribution.Target], func(i, j int) bool {
		a, b := p.byKind[contribution.Target][i], p.byKind[contribution.Target][j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		list := p.byKind[contribution.Target]
		for i := range list {
			if list[i].ID == contribution.ID {
				p.byKind[contribution.Target] = append(list[:i], list[i+1:]...)
				if len(p.byKind[contribution.Target]) == 0 {
					delete(p.byKind, contribution.Target)
				}
				return
			}
		}
	}, nil
}

// HasTarget 实现 pluginapi.ContextMenus。
func (p *Plugin) HasTarget(kind string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byKind[kind]) > 0
}

// Attach 实现 pluginapi.ContextMenus：给一个已有身份的元素挂右键菜单。
func (p *Plugin) Attach(c ui.C, id gfx.ID, target ui.Box, obj pluginapi.ContextMenuTarget) {
	if c == nil || id.IsZero() || obj.Kind == "" || !p.HasTarget(obj.Kind) {
		return
	}
	state := p.contextState(id)
	if canvas := c.Canvas(); canvas != nil {
		if event, ok := canvas.PointerOn(id); ok && event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary {
			p.closeMenus()
			state.open, state.point, state.fromAnchor, state.focused = true, ui.Rect{X: event.X, Y: event.Y}, false, false
			canvas.Consume()
		}
	}
	if target.Shortcut(gfx.ModifierShift, "f10") {
		p.closeMenus()
		state.open, state.fromAnchor, state.focused = true, true, false
	}
	p.drawMenu(c, id, target, obj, state)
}

// Button 实现 pluginapi.ContextMenus：官方 `contextMenu.button` 的等价物。
// 只有目标 kind 有已注册贡献者时才渲染（index.ts:37-40）。
func (p *Plugin) Button(c ui.C, id gfx.ID, obj pluginapi.ContextMenuTarget, label string) bool {
	if c == nil || p.theme == nil || obj.Kind == "" || !p.HasTarget(obj.Kind) {
		return false
	}
	state := p.buttonState(buttonKey(obj))
	box := ui.Row(c).Width(20).Height(20).Radius(p.theme.Radii.SM).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(p.theme.Hover)).
		HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton, Expandable: true, Expanded: state.open})
	box.Children(func() {
		if icon, ok := dsassets.ContextMenuButtonIcon(p.theme.LabelSecondary); ok {
			ui.Icon(c, icon).Size(menuButtonIcon, menuButtonIcon).Color(p.theme.LabelSecondary)
		}
	})
	if box.Clicked() {
		opening := !state.open
		if opening {
			p.closeMenus()
		}
		state.open = opening
		state.fromAnchor = true
		state.focused = false
	}
	p.drawMenu(c, id, box, obj, state)
	return true
}

func (p *Plugin) contextState(id gfx.ID) *popupState {
	if p.contexts == nil {
		p.contexts = map[gfx.ID]*popupState{}
	}
	state := p.contexts[id]
	if state == nil {
		state = &popupState{}
		p.contexts[id] = state
	}
	return state
}

func (p *Plugin) buttonState(key string) *popupState {
	if p.buttons == nil {
		p.buttons = map[string]*popupState{}
	}
	state := p.buttons[key]
	if state == nil {
		state = &popupState{}
		p.buttons[key] = state
	}
	return state
}

func (p *Plugin) closeMenus() {
	for _, state := range p.buttons {
		state.open = false
	}
	for _, state := range p.contexts {
		state.open = false
	}
}

// drawMenu 画官方共享 MenuOverlay 的一个实例。
func (p *Plugin) drawMenu(c ui.C, id gfx.ID, anchor ui.Box, obj pluginapi.ContextMenuTarget, state *popupState) {
	if !state.open || p.theme == nil {
		return
	}
	actions := p.resolve(obj)
	if len(actions) == 0 {
		state.open = false
		return
	}
	width, height := p.menuSize(c, actions)
	bounds := c.Bounds()
	if width <= 0 || height <= 0 {
		state.open = false
		return
	}
	state.actions, state.rowIDs = actions, make([]gfx.ID, len(actions))
	// anchor 可能是零值 Box（`context_menu_test.go` 的几何用例直接给 `ui.Box{}`）：
	// 读零值 Box 的 Rect 会 panic（"a Box is used after its paint"），
	// 零值时保留上一帧记下的矩形。
	if anchor.Valid() {
		state.rect = anchor.Rect()
	}
	if state.highlight < 0 || state.highlight >= len(actions) || actions[state.highlight].Disabled {
		state.highlight = max(nextEnabled(actions, -1, 1), 0)
	}
	rowIDs := state.rowIDs
	layer := ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerDrag, Bounds: bounds})
	layer.Children(func() {
		dismissID := ui.ID(c.Key("context-menu-dismiss"))
		scrim := ui.Row(c).FillWidth().FillHeight().HitWith(dismissID, "关闭菜单", gfx.HitOptions{
			Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true,
		})
		if scrim.Clicked() {
			state.open = false
		}
	})
	// Overlay 本身就是 Viva Popup：BelowBox 在布局后读取锚点矩形，不能在声明
	// 阶段读 anchor.Rect()（那时仍是零值）。右键档则直接落在指针点。
	menu := ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerDrag, Bounds: bounds}).
		Width(width).Height(height).Radius(menuRadius).Background(p.theme.Menu).
		Draw(func(d *ui.DrawContext) {
			d.Canvas.PushFocusScope(id)
			d.Canvas.TrapFocus(id)
			d.Canvas.ReturnFocus(id)
			drawProminentElevation(d, menuRadius, p.theme)
		}).
		DrawOver(func(d *ui.DrawContext) { d.Canvas.PopFocusScope() })
	if state.fromAnchor {
		menu = menu.BelowBox(anchor, menuAnchorGap, ui.Start)
	} else {
		menu = menu.At(state.point.X, state.point.Y)
	}
	menu = menu.Within(bounds.Inset(gfx.All(menuViewportMargin)))
	menu.Children(func() {
		ui.Scroll(c).FillWidth().FillHeight().Children(func() {
			ui.Column(c).FillWidth().Pad(menuPadding).Gap(0).Children(func() {
				for i, action := range actions {
					if action.SeparatorBefore && i > 0 {
						drawSeparator(c, p.theme)
					}
					rowID := ui.ID(c.Key("context-menu-row-" + action.ID))
					rowIDs[i] = rowID
					p.drawItem(c, rowID, i, action, state)
				}
			})
		})
	})
	if menu.Shortcut(0, "escape") {
		state.open = false
	}
	for _, key := range []string{"up", "down", "home", "end"} {
		if menu.Shortcut(0, key) {
			p.moveFocus(c, key, actions, rowIDs)
		}
	}
	if !state.focused {
		if first := nextEnabled(actions, -1, 1); first >= 0 && c.Canvas() != nil {
			state.highlight = first
			state.keyboard = false
			c.Canvas().RequestFocus(rowIDs[first])
		}
		state.focused = true
	}
	// 键盘操作过之后，焦点跟着高亮走（无障碍的那份也必须一致）。
	if state.keyboard && state.highlight >= 0 && state.highlight < len(rowIDs) && c.Canvas() != nil {
		c.Canvas().RequestFocus(rowIDs[state.highlight])
	}
}

// HandleKey 实现 pluginapi.KeyHandler：菜单打开时的 ↑↓ / Home / End / Enter /
// Space / Escape，以及 Shift+F10 打开菜单。
//
// 为什么由插件自己接：壳的按钮链是 `kernel.HandleKey`（`cmd/app/main.go` 的
// OnKey），它每帧都已经调到这里；而 `ui.Box.Shortcut` 走的是 `ui.Page.Key`，
// 本壳从不调用（全仓 `grep '\.Key(ev'` 为空）。见 popupState 里那段注释。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Release {
		return false
	}
	p.mu.Lock()
	open := make([]*popupState, 0, len(p.contexts))
	for _, state := range p.contexts {
		if state.open {
			open = append(open, state)
		}
	}
	for _, state := range p.buttons {
		if state.open {
			open = append(open, state)
		}
	}
	p.mu.Unlock()
	if len(open) > 0 {
		for _, state := range open {
			switch ev.Key {
			case gfx.KeyEscape:
				p.closeMenus()
				return true
			case gfx.KeyDown, gfx.KeyUp, gfx.KeyHome, gfx.KeyEnd:
				if p.stepHighlight(state, ev.Key) {
					return true
				}
				return true // 菜单开着就把方向键吃掉，别让焦点路由器跑出菜单
			case gfx.KeyEnter, gfx.KeySpace:
				if p.runHighlighted(state) {
					return true
				}
			}
		}
		return false
	}
	// Shift+F10：官方在 Windows 上的键盘入口（`context_menu.go` 早先那条
	// `target.Shortcut(gfx.ModifierShift,"f10")` 想要的形状）。
	if ev.Key == gfx.KeyF10 && ev.Modifiers&gfx.ModifierShift != 0 {
		p.mu.Lock()
		var target *popupState
		for _, state := range p.contexts {
			target = state
			break
		}
		if target != nil {
			p.closeMenus()
			target.open = true
			target.fromAnchor = false
			target.focused = false
			target.keyboard = true
			target.highlight = max(nextEnabled(target.actions, -1, 1), 0)
			target.point = ui.Rect{X: target.rect.X, Y: target.rect.Y + target.rect.Height}
		}
		p.mu.Unlock()
		return target != nil
	}
	return false
}

// stepHighlight 把高亮移到下一个/上一个可用项（跳过 disabled），Home/End 到两端。
func (p *Plugin) stepHighlight(state *popupState, key gfx.Key) bool {
	if len(state.actions) == 0 {
		return false
	}
	next := state.highlight
	switch key {
	case gfx.KeyHome:
		next = nextEnabled(state.actions, -1, 1)
	case gfx.KeyEnd:
		next = nextEnabled(state.actions, len(state.actions), -1)
	case gfx.KeyDown:
		if found := nextEnabled(state.actions, state.highlight, 1); found >= 0 {
			next = found
		}
	case gfx.KeyUp:
		if found := nextEnabled(state.actions, state.highlight, -1); found >= 0 {
			next = found
		}
	}
	if next < 0 {
		return false
	}
	state.highlight, state.keyboard = next, true
	return true
}

// runHighlighted 执行高亮那一项并关菜单（官方 Enter/Space 裁决）。
func (p *Plugin) runHighlighted(state *popupState) bool {
	if state.highlight < 0 || state.highlight >= len(state.actions) {
		return false
	}
	action := state.actions[state.highlight]
	state.open = false
	if action.Disabled {
		return true
	}
	if action.Run != nil {
		action.Run()
	}
	return true
}

func (p *Plugin) drawItem(c ui.C, id gfx.ID, index int, action pluginapi.ContextMenuAction, state *popupState) {
	t := p.theme
	foreground, hover := t.LabelPrimary, t.Hover
	if action.Danger {
		foreground, hover = t.Error, t.HoverDanger
	}
	row := ui.Row(c).FillWidth().Height(menuItemHeight).
		PadSides(menuItemPadY, menuItemPadX, menuItemPadY, menuItemPadX).
		Gap(menuItemGap).Radius(menuItemRadius).Items(ui.Center).
		HitWith(id, action.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: action.Disabled})
	if action.Disabled {
		row = row.Opacity(menuDisabledOpacity)
	} else {
		// 官方菜单原语（rc.2 asar 偏移 **123324111**）：
		//	`._item_70kcw_173:hover:not(:disabled):not([aria-disabled=true]){background:interactive-bg-hover}`
		//	`._item_70kcw_173:focus-visible:not(:disabled){background:interactive-bg-hover;outline:none}`
		// 键盘焦点行**就是**菜单的高亮（官方没有别的键盘位置指示），所以两个焦点
		// look 都挂上同一层底：
		//   `Focused`（任何焦点）—— 菜单打开时插件自己 `RequestFocus` 到第一行，
		//     之后 ↑↓/Home/End 走 `moveFocus`；Viva 的 `keyboardFocus` 只在 Tab
		//     时翻真（`viva@app/painter.go:559`），方向键不会把它置真，所以只挂
		//     `Focus`（= `:focus-visible`）时，方向键走一圈画面一动不动。
		//   `Focus`（focus-visible）—— 键盘 Tab 进来的那条路径，与官方逐字一致。
		// disabled 行按官方 `:not(:disabled)` 不给底色。
		row = row.Hover(ui.Look{}.Background(hover)).
			Focused(ui.Look{}.Background(hover)).
			Focus(ui.Look{}.Background(hover))
		// 键盘高亮那一行：官方 `_item.active{background:interactive-bg-hover}`
		// （MenuView）/ `:focus-visible{background:interactive-bg-hover}`
		// （primitives Menu）画的是同一层底。索引由 `HandleKey` 维护，
		// 不依赖 `:focus-visible` 的模态判定。
		if index == state.highlight {
			row = row.Background(hover)
		}
	}
	row.Children(func() {
		if action.Icon != "" {
			ui.Row(c).Size(menuItemIcon, menuItemIcon).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
				iconColor := t.MenuIcon
				if action.Danger {
					iconColor = t.Error
				}
				if icon, ok := dsassets.ContextMenuIcon(action.Icon, iconColor); ok {
					ui.Icon(c, icon).Size(menuItemIcon, menuItemIcon).Color(iconColor)
				}
			})
		}
		ui.Text(c, action.Label).FontSize(menuItemFont).LineHeight(menuItemLine).
			Color(foreground).Grow(1).Ellipsis()
	})
	if row.Clicked() && !action.Disabled {
		state.open = false
		if action.Run != nil {
			action.Run()
		}
	}
}

func drawSeparator(c ui.C, t *pluginapi.Theme) {
	ui.Row(c).FillWidth().PadSides(menuSeparatorMarginY, menuSeparatorMarginX,
		menuSeparatorMarginY, menuSeparatorMarginX).Children(func() {
		ui.Row(c).FillWidth().Height(menuSeparatorHeight).Background(t.BorderL2)
	})
}

// drawProminentElevation 画官方 `--dsw-elevation-prominent`。
//
// 出处：菜单原语 `._list_70kcw_7,._submenu_70kcw_8
// {--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);
//
//	box-shadow:var(--dsw-elevation-prominent)}`
//
// （`dsh-web-frontend/dist/assets/index-CSAdOBx6.css`）；官方的上下文菜单
// `context-menu/src/**` 自己没有 `*.module.css`，绘制交给 primitives `Menu`
// （`View.tsx:2,50-57`），所以这就是它那一档。
//
// 原来这里手抄了 prominent 的五个数（3+8@.04 / 0+20@.05 / stroke 0.5）与一段
// 描边几何：数值本来就对，问题是**第 N 份副本**——取值现在一行在
// `Shadows.Prominent`，描边色仍由调用方按官方那一处传（`BorderL1`）。
func drawProminentElevation(d *ui.DrawContext, radius float32, t *pluginapi.Theme) {
	dscore.PaintElevation(d, radius, t.Shadows.Prominent, t.BorderL1)
}

func (p *Plugin) menuSize(c ui.C, actions []pluginapi.ContextMenuAction) (width, height float32) {
	availableW := c.Bounds().Width - 2*menuViewportMargin
	availableH := c.Bounds().Height - 2*menuViewportMargin
	width, height = menuMinWidth, 2*menuPadding
	for i, action := range actions {
		if action.SeparatorBefore && i > 0 {
			height += menuSeparatorHeight + 2*menuSeparatorMarginY
		}
		height += menuItemHeight
		rowWidth := 2 * menuItemPadX
		if action.Icon != "" {
			rowWidth += menuItemIcon + menuItemGap
		}
		if c.Canvas() != nil {
			rowWidth += c.Canvas().TextWidth(action.Label, menuItemFont)
		}
		width = max(width, rowWidth)
	}
	if availableW < menuMinWidth {
		width = availableW
	}
	width = min(max(width, min(menuMinWidth, availableW)), min(menuMaxWidth, availableW))
	height = min(height, availableH)
	return width, height
}

func (p *Plugin) moveFocus(c ui.C, key string, actions []pluginapi.ContextMenuAction, ids []gfx.ID) {
	if c.Canvas() == nil || len(actions) == 0 {
		return
	}
	current := -1
	focused := c.Canvas().Focused()
	for i, id := range ids {
		if id == focused {
			current = i
			break
		}
	}
	next := current
	switch key {
	case "home":
		next = nextEnabled(actions, -1, 1)
	case "end":
		next = nextEnabled(actions, len(actions), -1)
	case "down":
		next = nextEnabled(actions, current, 1)
		if next < 0 {
			next = nextEnabled(actions, -1, 1)
		}
	case "up":
		next = nextEnabled(actions, current, -1)
		if next < 0 {
			next = nextEnabled(actions, len(actions), -1)
		}
	}
	if next >= 0 {
		c.Canvas().RequestFocus(ids[next])
	}
}

func nextEnabled(actions []pluginapi.ContextMenuAction, from, step int) int {
	for i := from + step; i >= 0 && i < len(actions); i += step {
		if !actions[i].Disabled {
			return i
		}
	}
	return -1
}

// resolve 按 order 收集某 target 的全部动作。
func (p *Plugin) resolve(obj pluginapi.ContextMenuTarget) []pluginapi.ContextMenuAction {
	p.mu.RLock()
	contributions := append([]pluginapi.ContextMenuContribution(nil), p.byKind[obj.Kind]...)
	p.mu.RUnlock()
	var out []pluginapi.ContextMenuAction
	for _, contribution := range contributions {
		out = append(out, contribution.Resolve(obj)...)
	}
	return out
}

func buttonKey(obj pluginapi.ContextMenuTarget) string {
	return obj.Kind + "\x00" + obj.ID
}
