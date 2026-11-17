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
	rowIDs := make([]gfx.ID, len(actions))
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
					p.drawItem(c, rowID, action, state)
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
			c.Canvas().RequestFocus(rowIDs[first])
		}
		state.focused = true
	}
}

func (p *Plugin) drawItem(c ui.C, id gfx.ID, action pluginapi.ContextMenuAction, state *popupState) {
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
		row = row.Hover(ui.Look{}.Background(hover))
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
