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
	// actions / rowIDs / highlight 是**键盘高亮**的状态：画面上的高亮底由它直接落
	// 下来（官方 `_item:focus-visible{background:interactive-bg-hover}`），不依赖
	// Viva 的 `:focus-visible` 判定。
	//
	// 按键**不由插件处理**：官方 `dsh-client-ui-context-menu` 没有任何插件级全局按键
	// 路由——菜单键归菜单原语自己（`View.tsx:50-57` 的 `Menu` 带 `autoFocus`/`onClose`/
	// `onSelect`），触发器键归**有焦点的那个触发器元素**（`trigger.ts:29-37` 的
	// `onKeyDown` 读 `event.currentTarget`，`show()` 里 `preventDefault` +
	// `stopPropagation`，一个键只有一处处理）。本插件照这个形状落成 Viva 的两条
	// `Shortcut`（见 `drawMenu` 与 `Attach`），不再实现 `pluginapi.KeyHandler`。
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
	// Shift+F10 / ContextMenu 键：官方把它挂在**触发器元素自己的** `onKeyDown` 上
	// （`trigger.ts:29-37`：读 `event.currentTarget`，只在这个元素有焦点时成立；
	// `show()` 里 `preventDefault`+`stopPropagation`，一个键只处理一次）。
	// Viva 的 `ui.ContextMenu` 是同一条规则："at the target when it has the focus and
	// Shift+F10 is pressed"（`viva@v0.6.4/ui/menu.go:57-59`），判据是
	// `target.id == event.Target`（`ui/menu.go:631-640`）——容器只要 `Hit` 过就算数
	// （`ui/menu.go:75`）。
	//
	// 为什么不用 `target.Shortcut(gfx.ModifierShift, "f10")`：`Box.Shortcut` 对容器按
	// **作用域覆盖**匹配（`ui/shortcut.go:69-71`、`:120-123`、`:162-165`），而本仓的
	// 交互 id 是 `env.Tree.ID(env.Feat, …)` 扁平命名，永远不落在 `ui.Row` 按位置命名
	// 出来的作用域里——于是容器上声明的 Shift+F10 在真装配下**永不触发**
	// （`cmd/app/page_shortcut_test.go` 只有把目标换成叶子 `ui.Slot` 才匹配得上）。
	// 页面级 `ui.Shortcut` 没有这个限制（`ui/shortcut.go:49-51`、`:106-108`），代价是
	// 它不带「焦点在这一个元素上」的判据，所以判据由这里按官方/viva 的规则补：
	// 焦点正好落在某个有菜单的目标上才开它。
	if ui.Shortcut(c, ui.Shift, "f10") {
		if focused, ok := p.focusedContext(c); ok {
			p.closeMenus()
			focused.open, focused.fromAnchor, focused.focused = true, true, false
		}
	}
	p.drawMenu(c, id, target, obj, state)
}

// focusedContext 取「键盘焦点正好在它身上」的那个目标的菜单状态。
//
// 官方 `trigger.ts:31-37` 只在 `event.currentTarget`（= 有焦点的触发器）上开菜单，
// 不是在"某个目标"上开；`ui/menu.go:633` 的 `target.id == event.Target` 是同一条
// 判据的 Viva 写法。焦点不在任何已声明菜单的目标上就什么都不开。
func (p *Plugin) focusedContext(c ui.C) (*popupState, bool) {
	canvas := c.Canvas()
	if canvas == nil {
		return nil, false
	}
	id := canvas.Focused()
	if id.IsZero() {
		return nil, false
	}
	state, ok := p.contexts[id]
	if !ok || state == nil {
		return nil, false
	}
	return state, true
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
	// 菜单自己的键：官方把导航交给菜单原语（`View.tsx:50-57` 的 `Menu` ——
	// `autoFocus` / `contextual` / `onClose` / `onSelect`，插件不声明全局键），
	// Viva 的同一形状是**容器级** `Box.Shortcut`（`ui/shortcut.go:53-71`：焦点在
	// 这个容器里就成立），派发者是 `ui.Page.Key`（`ui/page.go:502-514`
	// → `matchShortcut`，`cmd/app/main.go` 的 `cfg.OnKey` 每帧先送页面）。
	//
	// Enter / Space **不在这里**：Viva 的输入路由器本来就把「焦点区域上的 Enter /
	// Space」变成一次点击（`viva@v0.6.4/internal/input/input.go:308-315`；
	// `gfx.RoleMenuItem` 两个键都算，`:344-345`），由 painter 兑现成点击
	// （`app/painter.go:554`），和官方 `Menu` 选中一项是同一条语义。插件再声明一遍
	// 就是两套机制处理同一个键。
	if menu.Shortcut(0, "escape") {
		state.open = false
	}
	// 四行显式声明而不是循环一张表：这张表同时是 `plugins/dsshortcuts` 的
	// `shortcuts_guard_test.go` 反向核对的依据（它按 AST 找真实的 chord 参数），
	// 藏在 `[]string{...}` 里的进去就查不到了。
	if menu.Shortcut(0, "up") {
		p.moveFocus(c, "up", actions, rowIDs)
	}
	if menu.Shortcut(0, "down") {
		p.moveFocus(c, "down", actions, rowIDs)
	}
	if menu.Shortcut(0, "home") {
		p.moveFocus(c, "home", actions, rowIDs)
	}
	if menu.Shortcut(0, "end") {
		p.moveFocus(c, "end", actions, rowIDs)
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
		// （primitives Menu）画的是同一层底。索引由 `moveFocus` / 打开时的首行
		// 建立，不依赖 `:focus-visible` 的模态判定。
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
