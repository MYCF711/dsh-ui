package dscontextmenu

// context_menu.go —— 官方 `dsh-client-ui-context-menu` 的 Go+Viva 等价实现。
//
// # 官方结构（rc.2 随包 src/**）
//
//	OFF:ui-context-menu/src/client/index.ts:98-134
//	  contextMenus 服务：active / registeredKinds / register / open / resolve / close / notify
//	OFF:ui-context-menu/src/client/index.ts:135-145
//	  `contextMenu.button` 工厂槽：目标 kind 没有贡献者时**整枚按钮不渲染**
//	OFF:ui-context-menu/src/client/index.ts:170-181
//	  `shell.overlay` 上的 MenuOverlay：一个共享菜单 + 通知 Toast
//	OFF:ui-context-menu/src/client/registry.ts:30-47
//	  按 target kind 注册、同 target 内 id 去重、order 升序、卸载即撤回
//
// # 本仓等价边界（写清，别当成同形）
//
//   - 渲染：viva v0.6.0 自带 `ui.ContextMenu`（右键 / Shift+F10 / 指针点开）
//     与 `ui.Menu`（行尾省略号按钮的锚定菜单），都由 components 画 shadcn 菜单。
//     官方是 React Menu portal；本仓不另造一套菜单视觉。
//   - 服务：本文件实现注册表与两个消费入口（Attach / Button）。业务不接触
//     viva 的 MenuItems，只注册 `ContextMenuContribution`。
//   - 未做：`contextMenu.content` 的自定义 React 行（本仓没有 React；该槽只登记
//     结构证据）与 `contextMenu.icon` 的业务矢量图标（官方 primitives 不在
//     rc.2 随包导出集里，本仓没有对应图标资产，菜单项按文案渲染，不自画图标）。
//   - 未做 `notify` 的 Toast：失败反馈由消费方自己的对话框/状态栏承担；不画一个
//     没有官方几何依据的浮动条。

import (
	"fmt"
	"sort"
	"sync"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Plugin 是 context menu 服务插件。它没有自己的常驻可见面：菜单由消费方
// 在目标元素上声明（Attach/Button），这正是官方「纯 UI 基础设施」的形状。
type Plugin struct {
	theme *pluginapi.Theme

	mu       sync.RWMutex
	byKind   map[string][]pluginapi.ContextMenuContribution
	buttons  map[string]*bool
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
	p.buttons = map[string]*bool{}
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
func (p *Plugin) Attach(c ui.C, target ui.Box, obj pluginapi.ContextMenuTarget) {
	if c == nil || obj.Kind == "" || !p.HasTarget(obj.Kind) {
		return
	}
	ui.ContextMenu(c, target, func(items *ui.MenuItems) {
		p.declare(items, obj)
	})
}

// Button 实现 pluginapi.ContextMenus：官方 `contextMenu.button` 的等价物。
//
// 官方语义（index.ts:37-40）：**只有目标 kind 有已注册贡献者时才渲染**。
// 这里通过 HasTarget 保证；没有贡献者时连命中区都不创建（空槽不占位）。
func (p *Plugin) Button(c ui.C, id gfx.ID, obj pluginapi.ContextMenuTarget, label string) bool {
	if c == nil || p.theme == nil || obj.Kind == "" || !p.HasTarget(obj.Kind) {
		return false
	}
	key := buttonKey(obj)
	p.mu.Lock()
	open := p.buttons[key]
	if open == nil {
		open = new(bool)
		p.buttons[key] = open
	}
	p.mu.Unlock()

	box := ui.Row(c).Width(20).Height(20).Radius(p.theme.Radii.SM).
		Items(ui.Center).Justify(ui.Center).
		Hover(ui.Look{}.Background(p.theme.Hover)).
		HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton})
	box.Children(func() {
		ui.Text(c, "⋯").FontSize(14).Color(p.theme.LabelSecondary)
	})
	if box.Clicked() {
		*open = !*open
	}
	ui.Menu(c, box, open, func(items *ui.MenuItems) {
		p.declare(items, obj)
	})
	return true
}

// declare 把当前 target 的所有贡献摊成 viva 菜单行。
func (p *Plugin) declare(items *ui.MenuItems, obj pluginapi.ContextMenuTarget) {
	if items == nil {
		return
	}
	for i, action := range p.resolve(obj) {
		if action.SeparatorBefore && i > 0 {
			items.Separator()
		}
		// viva 的菜单行由 Scope 身份驱动（MenuItems.Item 内部分配），不是 Box.Hit；
		// 写成链式调用，静态守卫才不会把菜单项误判成"没有命中区的 Box"。
		if p.menuItem(items, action).Clicked() && action.Run != nil {
			action.Run()
		}
	}
}

// menuItem 声明一条菜单行并套上官方视觉状态（destructive / disabled）。
func (p *Plugin) menuItem(items *ui.MenuItems, action pluginapi.ContextMenuAction) ui.MenuItem {
	item := items.Item(action.Label)
	if action.Danger {
		item = item.Destructive()
	}
	if action.Disabled {
		item = item.Disabled(true)
	}
	return item
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
