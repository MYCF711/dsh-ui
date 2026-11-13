// Package dssettings 是最小可操作的**设置浮层**：一块盖在三栏上的卡片，
// 把壳已经接上的设置动作按「外观 / 模型 / 权限 / 会话」四块摊开。
//
// # 为什么不照官方那 11 页做
//
// 官方的设置面是十来个独立页面（外观、模型、权限、快捷键、隐私……），
// 每一页背后都有宿主侧的配置面。我们这边**真正被消费的动作只有四个**：
// `SetTheme` / `ModelsList`+`SetModel` / `SetPermission`（外加只读的会话信息）。
// 照着页数抄一遍就是十几个空壳。这里只画**点了真的会发生什么**的那几行。
//
// # 入口
//
// `Shift+F2`。**普通 F2 一个字节都不碰**——那是 `dscommands` 的命令面板
// （它只看 `ev.Key`，不看修饰键；所以这里也必须精确匹配 Shift，否则两边
// 会有一个被另一个抢掉）。`cmd/app` 把这个插件排在装配顺序的最前（见
// main.go 里准备插件列表那一段），于是：
//
//	Esc  设置浮层开着时先关它（而不是被 composer 当成"停止当前 turn"）
//	↑↓   浮层开着时移光标（不会漏给侧栏切会话）
//
// 关着的时候所有键一律放行：Enter 照旧发送、Esc 照旧停 turn、字母照旧进输入框
// （本插件**不实现 Fields**，不抢文本焦点——这是"不和 composer/@菜单打架"的
// 具体做法，不是一个口头承诺）。
//
// # 只画真值
//
// 数据模型的判断全在 model.go（那边有单测）；这里只负责画和收键。
package dssettings

import (
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 卡片的几何。宽度是设计值（官方设置面是宽卡片），窄窗口按边距收窄；
// 顶距固定——viva 的 Absolute 在"没有 edge 的那根轴"上按内容 pref 走，
// 给不了垂直居中（dsshortcuts 里踩过同一条）。
const (
	cardMaxWidth = 560
	cardMinWidth = 360
	cardMargin   = 24
	cardTop      = 64

	rowHeight  = 26
	labelWidth = 64
	markWidth  = 14
)

// Plugin 是浮层的全部状态。
//
// 为什么带锁：HandleKey 会从**两个线程**进来（窗口线程的 OnKey，以及
// `cmd/app -keys` 的注入 goroutine），而 shown/rows/cursor 是在绘制线程里写的
// （dsshortcuts 踩过同一个坑）。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	mu    sync.Mutex
	shown bool
	act   pluginapi.Actions // 上一帧的 Actions：HandleKey 没有 env，只能记着
	rows  []Row             // 上一帧拍平出来的可点行（键盘光标用）
	// cursor 是键盘光标：**-1 = 还没有光标**。
	//
	// 为什么不是 0：浮层一打开就给第一行画高亮，会被看成"当前选中的是它"——
	// 而第一行是不是当前值我们并不知道（未知时一行都不该像被选中）。
	// 用户按下 ↑↓ 之后才有光标。
	cursor int
	notify func(string)

	// updateFrameTimer 是桌面更新指示器 busy spinner（官方 1s 线性旋转）自己排的
	// 那一帧重绘。与 dsstatusbar 的同名机制同构：本仓没有渲染循环，不排就停在一帧。
	updateFrameTimer *time.Timer

	// official 是 rc.2 官方四区面板的状态（official_panel.go）。
	//
	// 为什么和旧浮层共存：官方面板需要 settings.describe 数据面；老壳没接时
	// 退回旧浮层（兼容桥），这样两条路径都有真实测试，而不是二选一时删掉一条。
	official *officialPanel
}

// ID 必须全局唯一。
func (*Plugin) ID() string { return "ds-settings" }

// Provides 只提供"怎么打开我"这一个能力（见 register.go 的 ServiceKey）。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

// Requires 与所有画东西的插件一致：颜色令牌 + 基础图元。
// 装配器据此保证 ds-core 先就位，Register 里 Inject 不会拿到 nil。
func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 只做注入与登记，不画东西。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	ctx.Provide(ServiceKey, func() bool { return p.Open() })
	return nil
}

// Panels 挂在 shell.overlay：它盖住整个框架，正是模态卡片该在的地方。
//
// Order 100 = 压在 99（F2 命令面板，模态档）之上：设置是个"我明确打开了它"的
// 面板，两个同时开着时它该在最上面。分层约定见 plugins/dscommands 的注释，
// `cmd/app/panels_order_test.go` 守着同槽不许撞号。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			// rc.1 最小浮层（兼容桥）：只在壳没接 settings.describe 时画。
			ID:    "settings-overlay",
			Slot:  pluginapi.SlotShellOverlay,
			Order: 100,
			Draw:  p.draw,
		},
		{
			// rc.2 官方四区面板：壳接上 settings.describe 后由它接管。
			//
			// Order 101 压在旧浮层之上（两个同时存在时官方结构优先）；两者的
			// Draw 各自以 OfficialEnabled() 为判据互斥，不会同时画出来。
			ID:    "settings-panel",
			Slot:  pluginapi.SlotShellOverlay,
			Order: 101,
			Draw:  p.drawOfficialPanel,
		},
		{
			// 官方 `SettingsRoot.triggerRow` 里的设置按钮（`settings.launcher`
			// 的 fallback，见 official_trigger.go）。挂 `sidebar.settings` 官方槽：
			// 壳把这一槽摆进**一行**（行容器在 uikernel 骨架里），本画家只画
			// 行里的 `flex:1` 那一格。
			//
			// Order 55：**排在连接指示（61）之前**——官方 `triggerRow` 的排法是
			// `[设置按钮(flex:1)] [连接指示(flex:none)] [更新指示]`，行内顺序
			// 就是 Order 顺序。
			// 55 同时排在账号启动器（dsnotifications 的 60）之前；两者互斥
			// （有启动器时本按钮是官方 fallback，不画），所以不会出现"两个设置入口"。
			ID:    "settings-trigger",
			Slot:  uikernel.OffSidebarSettings,
			Order: 55,
			Draw:  p.drawSettingsTrigger,
		},
		{
			// 行内第 3 格：官方 `DesktopUpdateIndicator`（**不是 slot**，官方把它
			// 硬写在 `SettingsRoot` 里，所以由设置面 owner 画，见 official_update.go）。
			// Order 62 排在连接指示（dsstatusbar 的 61）之后，与官方行内顺序一致。
			ID:    "settings-desktop-update",
			Slot:  uikernel.OffSidebarSettings,
			Order: desktopUpdateOrder,
			Draw:  p.drawDesktopUpdateIndicator,
		},
	}
}

// Open 打开浮层，返回"这一帧确实有可操作的东西"。
//
// 谁调它：`cmd/app -open-settings`（无头截图用，和 `-open-file` 同一个思路：
// 截图路径注入不了鼠标，只能调插件的公开入口）。产品路径是 Shift+F2。
func (p *Plugin) Open() bool {
	// **官方四区面板接管时，必须走它自己的打开路径。**
	//
	// 为什么：两个画家各自以 `OfficialEnabled()` 判据互斥——旧浮层的 `draw` 在
	// 官方接管时直接 return，而 `drawOfficialPanel` 又要求 `o.open`。若这里只置
	// 旧的 `p.shown`，`o.open` 永远是 false，于是**两边都不画**，用户看到整页空白。
	// 2026-11-13 真机复现：`-open-settings` 日志打"设置浮层已打开"，截图什么都没有；
	// 产品路径（Shift+F2 / 侧栏设置按钮，都走 `ui.settings.open`）同一个坑。
	if p.OfficialEnabled() {
		return p.OpenOfficial()
	}
	p.mu.Lock()
	ok := HasSettingsActions(p.act)
	if ok {
		p.shown = true
		p.cursor = -1
	}
	notify := p.notify
	p.mu.Unlock()
	if ok && notify != nil {
		notify("设置已打开 · Esc 关闭")
	}
	return ok
}

// Close 收起浮层（幂等）。
func (p *Plugin) Close() {
	if p.OfficialEnabled() {
		p.CloseOfficial()
		return
	}
	p.mu.Lock()
	was := p.shown
	p.shown = false
	notify := p.notify
	p.mu.Unlock()
	if was && notify != nil {
		notify("设置已关闭")
	}
}

// Shown 报告浮层当前是不是开着（壳的测试与调试用）。
func (p *Plugin) Shown() bool {
	if p.OfficialEnabled() {
		return p.OfficialShown()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.shown
}

// show/hide 是内部的两个动作，统一走 Notify 请求重绘。
//
// **必须请求重绘**：HandleKey 跑在绘制之外，改完 shown 之后没有任何一帧会自动
// 发生。pluginapi 没有"请重绘"这个动作，插件手里唯一的公开入口是
// Actions.Notify（宿主实现是 cmd/app 的 setStatus：写状态栏 + Invalidate）。
// dsshortcuts / dscommands 都是这么做的，不是这里发明的。
func (p *Plugin) show(msg string) {
	p.mu.Lock()
	p.shown = true
	p.cursor = -1
	notify := p.notify
	p.mu.Unlock()
	if notify != nil {
		notify(msg)
	}
}

func (p *Plugin) hide(msg string) {
	p.mu.Lock()
	was := p.shown
	p.shown = false
	notify := p.notify
	p.mu.Unlock()
	if was && notify != nil {
		notify(msg)
	}
}

// move 移动键盘光标（只在可点行之间走；只读行不参与）。
func (p *Plugin) move(delta int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.rows)
	if n == 0 {
		return
	}
	if p.cursor < 0 {
		// 第一次按键才落光标：↓ 落到第一行，↑ 落到最后一行。
		if delta > 0 {
			p.cursor = 0
		} else {
			p.cursor = n - 1
		}
		return
	}
	p.cursor = (p.cursor + delta + n) % n
}

// activate 执行光标所在那一行（Enter）。
func (p *Plugin) activate() {
	p.mu.Lock()
	var run func()
	if p.cursor >= 0 && p.cursor < len(p.rows) {
		run = p.rows[p.cursor].Run
	}
	p.mu.Unlock()
	if run != nil {
		run()
	}
}

// HandleKey 实现 pluginapi.KeyHandler。
//
// 返回 true 是有代价的（后面的插件再也看不到这个键），所以：
//   - 普通 F2、关着时的所有键、带修饰键的键 —— 一律 false
//   - 只有 "Shift+F2"、"浮层开着时的 Esc/↑/↓/Enter" 这几种才吃
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	if ev.Release {
		return false
	}
	// 官方四区面板可用时由它接管键盘（同一个 Shift+F2 入口，互斥）。
	if p.OfficialEnabled() {
		return p.handleOfficialKey(ev)
	}
	p.mu.Lock()
	shown := p.shown
	hasActions := HasSettingsActions(p.act)
	p.mu.Unlock()

	// 入口：Shift+F2。**精确匹配**修饰键：dscommands 的 F2 不看 modifiers，
	// 这里只要判定稍宽一点，两边就有一方永远收不到。
	if ev.Key == gfx.KeyF2 {
		if ev.Modifiers != gfx.ModifierShift {
			return false
		}
		if shown {
			p.hide("设置已关闭")
			return true
		}
		// 没有任何真实设置动作时**不响应**（也不画入口）：空面板没有意义。
		if !hasActions {
			return false
		}
		p.show("设置已打开 · Esc 关闭")
		return true
	}
	if !shown {
		return false
	}
	if ev.Key == gfx.KeyEscape {
		p.hide("设置已关闭")
		return true
	}
	// 浮层开着时也不抢组合键（Ctrl+Shift+C 复制这类留给别的插件）。
	if ev.Modifiers != 0 {
		return false
	}
	switch ev.Key {
	case gfx.KeyDown:
		p.move(1)
		return true
	case gfx.KeyUp:
		p.move(-1)
		return true
	case gfx.KeyEnter:
		p.activate()
		return true
	}
	return false
}

// cardWidth 返回卡片在给定窗口宽度下的宽度（与 dsshortcuts 同一套收窄规则）。
func cardWidth(window float32) float32 {
	w := float32(cardMaxWidth)
	if window > 0 && w > window-2*cardMargin {
		w = window - 2*cardMargin
	}
	if w < cardMinWidth {
		w = cardMinWidth
	}
	return w
}

// draw 画浮层。没开 / 没有任何设置动作时什么都不画。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	// 兼容桥：壳接上 settings.describe 后，官方四区面板接管，旧浮层不画。
	if p.OfficialEnabled() {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}

	// 每帧刷新一次：壳可能中途接上新能力，Actions 也是每帧新给的。
	// 判据以**这一帧**的 Actions 为准，遮掉上一帧的残留状态。
	hasActions := HasSettingsActions(env.Act)
	cfg := map[string]string(nil)
	if env.Act.ConfigGet != nil {
		cfg = env.Act.ConfigGet()
	}
	secs := BuildSections(env.Act, env.Data, cfg, t.Dark)

	var clickable []Row
	for _, sec := range secs {
		for _, row := range sec.Rows {
			if row.Run != nil {
				clickable = append(clickable, row)
			}
		}
	}

	p.mu.Lock()
	p.act = env.Act
	if env.Act.Notify != nil {
		p.notify = env.Act.Notify
	}
	if !hasActions {
		// 能力被撤掉了：浮层跟着消失，不留一个点不动的空卡片。
		p.shown = false
	}
	shown := p.shown
	p.rows = clickable
	if p.cursor >= len(clickable) {
		p.cursor = -1
	}
	cursor := p.cursor
	p.mu.Unlock()

	if !shown {
		return
	}

	w := cardWidth(env.Bounds.Width)
	left := (env.Bounds.Width - w) / 2
	if left < 0 {
		left = 0
	}
	idx := 0
	ui.Row(f).Absolute().Left(left).Top(cardTop).Width(w).Children(func() {
		ui.Column(f).FillWidth().Radius(t.Radii.Panel).
			Background(t.InputMajor).Border(1, t.BorderL2).
			Shadow(gfx.ShadowXS).
			PadSides(20, 20, 16, 20).Gap(6).Children(func() {
			// 标题行：标题 + 关闭按钮（28px 圆钮交给 prims，不自己拼尺寸和颜色）。
			ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
				ui.Text(f, "设置").FontSize(t.Fonts.Title).Color(t.LabelPrimary).Grow(1)
				if p.prims.CircleButton(f, env.Tree.ID(env.Feat, "settings-close"), "×", false) {
					p.hide("设置已关闭")
				}
			})
			ui.Text(f, "Shift+F2 或 Esc 关闭 · 只列壳真接上的项").
				FontSize(t.Fonts.Caption).Color(t.LabelTertiary)

			for _, sec := range secs {
				idx = p.drawSection(f, env, t, sec, idx, cursor)
			}
		})
	})
}

// drawSection 画一块分区，返回下一个可点行的序号。
//
// idx/cursor 是**可点行**的序号（只读行不占号），和 HandleKey 的 move 对齐。
func (p *Plugin) drawSection(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, sec Section, idx, cursor int) int {
	ui.Text(f, sec.Title).FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
	if sec.Note != "" {
		ui.Text(f, sec.Note).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
	}
	for _, row := range sec.Rows {
		row := row
		if row.Run == nil {
			// 只读行：不挂命中区、不参与光标——它就不该看起来像按钮。
			ui.Row(f).FillWidth().Height(rowHeight).Items(ui.Center).Gap(10).Children(func() {
				ui.Text(f, row.Label).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Width(labelWidth)
				ui.Text(f, row.Hint).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Grow(1).Ellipsis()
			})
			continue
		}
		mark := ""
		if row.Selected {
			mark = "✓"
		}
		active := row.Selected || idx == cursor
		body := func() {
			ui.Text(f, mark).FontSize(t.Fonts.Caption).Color(t.Info).Width(markWidth)
			ui.Text(f, row.Label).FontSize(t.Fonts.Body).Color(t.LabelPrimary).Grow(1).Ellipsis()
			if row.Hint != "" {
				ui.Text(f, row.Hint).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis()
			}
		}
		id := env.Tree.ID(env.Feat, "settings-"+row.Key)
		idx++
		if p.prims.ListRow(f, id, active, rowHeight, body).Clicked() {
			row.Run()
		}
	}
	return idx
}

var (
	_ pluginapi.Plugin     = (*Plugin)(nil)
	_ pluginapi.Panels     = (*Plugin)(nil)
	_ pluginapi.KeyHandler = (*Plugin)(nil)
)
