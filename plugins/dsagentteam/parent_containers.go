// parent_containers.go —— 本单的两个官方**父容器**（模型菜单 / chat 命令节点）+ 数据接线。
//
// GB 已经把 5 个 rc.2 槽位注册好了，但其中两个在本仓**没有父级 drawSlot**，
// 所以界面上永远看不到（GB 报告 §7.3）：
//
//	② conversation.input.model.options        ← 父级 `conversation.input.model`（模型菜单）
//	④ conversation.chat.commandview（team）   ← 父级 `Chat.commandNode`（命令节点渲染器）
//
// # 容器 C：模型菜单（官方 `ui-model-selection` 的 ModelSelect）
//
// 官方层级（结构树 §5.2 C18 + `ui-model-selection/src/client/**`）：
//
//	conversation.input.model          single/session   trigger（28px chip）
//	├─ conversation.input.model.label single/inline
//	└─ conversation.input.model.options list/session    ← 本容器要 dispatch 的子槽
//
// 菜单本身画在 `internal/uikernel/official_model_menu.go`（`DrawModelMenu`）：
// 那是壳层结构（min240/max420/max360/padding4 官方几何 + 子槽 dispatch 点），
// 这里只负责**挂载**：两个座位各一条 painter，`Children` 里把内核的 drawSlot
// 转给容器，容器画完菜单面再回头 dispatch `conversation.input.model.options`。
//
// 为什么挂两个座位：
//
//	uikernel.OffInputModel         官方 composer 行里的模型座位（官方骨架活了就用它）
//	pluginapi.SlotMainComposer     旧三栏（dsshell）的 composer 列 —— **今天的产品走这条**
//
// 两条互斥：官方骨架与 dsshell 三栏不会同时画，所以菜单不会画两遍。
//
// # 容器 D：chat 命令节点渲染器（官方 `ui-chat` 的 CommandNodeView）
//
// 官方（`CommandNodeView.tsx:18`）：`.callRow` 里 renderSlot 渲染
// `conversation.chat.commandview`，entryKey 取 node.name（缺省空串），
// 没有认领者时用 GenericCommandCard 兜底。
// 画法在 `internal/uikernel/official_command_node.go`（`DrawCommandNodes`）：
// 有认领 → 只画 `.callRow` + dispatch 子槽；没认领 → 画官方通用命令卡兜底。
//
// 挂载同样两个座位：`uikernel.OffConvView`（官方 chat 列）与
// `pluginapi.SlotMainBody`（旧 transcript）——后者是今天产品上的位置。
//
// # 数据接线（只做这两个容器相关的）
//
//	provider 目录   Store.SetProviders(...)  ← 本文件 refreshProviders()，经 internal/apiclient
//	菜单开关        Store.ToggleModelMenu()  ← 点官方 trigger 的命中区
//	命令节点        Store.SetCommand(...)    ← 父级喂（见 SetCommand 注释与报告 §6）
//
// # 边界（不编 0、不假成功）
//
//   - 宿主没有 `llm.listConfigurableProviders`（或没注入 ui.apiclient）→ known=false →
//     菜单整段不画。**不是**画一个空菜单。
//   - 宿主回空数组 → 也不画（官方没有"空菜单"这种形状）。
//   - 取目录失败 → 把错误原文交给容器，容器画官方 `.error` 行（不编造 provider）。
//   - provider 行不可点：本仓宿主只给 provider 目录，选不出 model，宁可不给命中区。
package dsagentteam

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// ModelProviderOption 是模型菜单里的一行 provider（容器 C 的视图模型）。
//
// 与 `apiclient.ConfigurableProvider` 的对应关系写在 `providerOptions()`：
// 只带界面要显示的字段，不把整个 wire 结构塞进插件状态。
type ModelProviderOption struct {
	// Provider / DisplayName 是官方 `provider` / `displayName`。
	Provider    string
	DisplayName string
	// Seed 是行下说明：settingsNs（有值）或"未在模型目录中声明"（declared=false）。
	Seed string
	// Failure 是这一项自己的错误（官方 `error`）：非空 = 不可用行。
	Failure string
}

// ProvidersRemote 是容器 C 的可选远端（官方 `llm.listConfigurableProviders`）。
//
// `*apiclient.Client` **直接满足**它（方法签名逐字相同），插件侧零适配层。
// 没注入 → 菜单整段隐藏（空态，不是错误）。
type ProvidersRemote interface {
	ListConfigurableProviders(ctx context.Context) ([]apiclient.ConfigurableProvider, error)
}

// providerFetchTimeout 是一次目录拉取的上限（宿主卡住时不能把界面拖死）。
const providerFetchTimeout = 10 * time.Second

// ── 挂载点 ────────────────────────────────────────────────────────────

// containerPanels 返回本单两个父容器的 4 条 painter（每容器 2 个座位）。
//
// 由 `Plugin.Panels()` 追加在 4 个官方槽贡献之后；`Draw` 一律为空、全部逻辑在
// `Children` 里，因为"父容器"这件事本身就是要拿到内核的 `drawSlot` 才能成立。
func (p *Plugin) containerPanels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 容器 C：模型菜单。官方座位 + 旧三栏过渡座位。
		//   Draw     = trigger 命中区（不需要 dispatch）
		//   Children = 菜单面 + `conversation.input.model.options` 的 drawSlot
		{ID: "agent-team.model-menu", Slot: uikernel.OffInputModel, Order: 80,
			Draw: p.drawModelMenuTrigger, Children: p.modelMenuChildren},
		{ID: "agent-team.model-menu-legacy", Slot: pluginapi.SlotMainComposer, Order: 80,
			Draw: p.drawModelMenuTrigger, Children: p.modelMenuChildren},
		// 容器 D：chat 命令节点渲染器。官方 chat 列 + 旧 transcript 座位。
		//   Draw     = 官方兜底卡（没被认领的命令）
		//   Children = 认领行的 `.callRow` + `conversation.chat.commandview` 的 drawSlot
		{ID: "agent-team.command-node", Slot: uikernel.OffConvView, Order: 80,
			Draw: p.drawCommandNodeCards, Children: p.commandNodeChildren},
		{ID: "agent-team.command-node-legacy", Slot: pluginapi.SlotMainBody, Order: 80,
			Draw: p.drawCommandNodeCards, Children: p.commandNodeChildren},
	}
}

// ── 容器 C：模型菜单 ──────────────────────────────────────────────────

// modelMenuChildren 画模型菜单父容器，并把子槽交回内核。
//
// 顺序（官方 ModelSelect 的 root pane）：
//
//	trigger 命中区（透明，盖在 composer 的模型 chip 上）→ 点它开/关菜单
//	菜单面（Open && Known && 有 provider）→ 内部 dispatch `conversation.input.model.options`
func (p *Plugin) modelMenuChildren(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if f == nil || env == nil {
		return
	}
	theme := p.themeFor(env)
	anchor := modelTriggerRect(env)
	known, failure, list := p.team.Providers()
	uikernel.DrawModelMenu(f, env, uikernel.ModelMenuOptions{
		Theme:        theme,
		Anchor:       anchor,
		Bounds:       env.Bounds,
		Open:         p.team.ModelMenuOpen(),
		Known:        known,
		Providers:    menuProviders(list),
		FetchFailure: failure,
		Selected:     env.Data.Model,
		Locked:       env.Data.Busy,
		Dispatch:     drawSlot,
	})
}

// drawModelMenuTrigger 是容器 C 在 `Painter.Draw` 里的那一半：官方 trigger 的
// **透明命中区**（点它开/关菜单）。
//
// 为什么不画 trigger 本体：本仓 composer 的模型 chip 由 `plugins/dscomposer` 画成
// 只读文本（`TestModelReadOnlyHasNoDropdownSemantics` 守着"没有目录就不做假下拉"），
// 再画一枚就是同屏两个模型名。有了真目录之后，chip 变成官方 trigger 的视觉，
// 这枚命中区正好是它自己的点击面——所以只盖命中区，不重画。
//
// 目录未知时不盖：没目录的点击只会打开一个空菜单，那是"假入口"（与官方
// `available=false` 时 trigger 不可用的语义一致）。
func (p *Plugin) drawModelMenuTrigger(f *flex.Frame, env *pluginapi.Env) {
	if f == nil || env == nil {
		return
	}
	known, _, list := p.team.Providers()
	anchor := modelTriggerRect(env)
	// Absolute：不占 composer 列的布局空间（否则会把卡片挤走）。
	box := f.Row().Absolute().Size(anchor.Width, anchor.Height).
		Right(anchorRightInset(env, anchor)).
		Bottom(modelTriggerBottomInset - uikernel.LegacyColumnInset).Children(func() {})
	drawn, clicked := uikernel.DrawModelTrigger(box, env, uikernel.ModelMenuOptions{
		Theme:     p.themeFor(env),
		Anchor:    anchor,
		Bounds:    env.Bounds,
		Open:      p.team.ModelMenuOpen(),
		Known:     known,
		Providers: menuProviders(list),
	})
	_ = drawn
	if clicked {
		p.team.ToggleModelMenu()
	}
}

// modelTriggerRect 是本仓 composer 模型触发器的矩形（窗口坐标）。
//
// **这是过渡几何**：官方 trigger 的锚点来自 `conversation.input.model` 自己的座位
// （`anchor-name: --dsh-model-anchor`）。本仓模型 chip 由 dscomposer 在旧
// `main.composer` 槽里按它自己的常量排版，插件之间拿不到它的 Box，所以这里按
// 官方 composer 的公开几何重算一次：
//
//	卡片宽 = clamp(680, 主列宽*0.64, 920) + 32      （--dsh-composer-card-max-width）
//	卡片水平居中于主列                               （.card max-width + 居中）
//	trigger 高 28 / max-width min(360, 45cqw)        （ModelSelect.module.css:11-38）
//	trigger 右缘 = 卡片右缘 - 8(pad) - 34(主按钮) - 12(row gap)
//
// 差异登记在报告 §8：dscomposer 迁到官方 `conversation.input.model` 座位后，
// 这个函数整段删掉，改用座位自己的 Box。
func modelTriggerRect(env *pluginapi.Env) ui.Rect {
	column := env.Geometry.MainWidth
	if column <= 0 {
		column = env.MainWidth
	}
	if column <= 0 {
		return ui.Rect{}
	}
	columnX := env.Geometry.SidebarWidth
	content := column * 0.64
	if content < 680 {
		content = 680
	}
	if content > 920 {
		content = 920
	}
	cardWidth := content + 32
	if cardWidth > column {
		cardWidth = column
	}
	cardX := columnX + (column-cardWidth)/2
	width := cardWidth * 0.45
	if width > 360 {
		width = 360
	}
	right := cardX + cardWidth - 8 - sendButtonSize - rowControlGap
	height := float32(uikernel.ModelTriggerHeight)
	top := env.Bounds.Height - modelTriggerBottomInset - height
	if right < cardX {
		width = 0
	}
	return ui.Rect{X: right - width, Y: top, Width: width, Height: height}
}

// 旧 composer 卡片底部的固定偏移（只用于过渡几何，dscomposer 迁到官方座位后整段删）：
//
//	主按钮 34 / 控件行 gap 12                         dscomposer 的 sendSize / cardGap
//	trigger 底距窗口底 21 = 12(旧中栏内边距) + 9(控件行底 pad 6 + 居中差 3)
const (
	sendButtonSize          float32 = 34
	rowControlGap           float32 = 12
	modelTriggerBottomInset float32 = 21
)

// anchorRightInset 把窗口坐标的右缘差转成 Absolute().Right(...) 需要的值。
//
// Absolute 的边距是相对**当前容器**的 padding box；旧三栏的 composer 列
// 就是从窗口左边到窗口右边整宽，所以窗口右缘距离可以直接当 Right 用。
// （官方座位上的容器是 composer 行，宽度等于窗口宽减去左右栏——那时
// `modelTriggerRect` 会跟着换实现，见它的注释。）
func anchorRightInset(env *pluginapi.Env, anchor ui.Rect) float32 {
	inset := env.Bounds.Width - (anchor.X + anchor.Width)
	if inset < 0 {
		return 0
	}
	return inset
}

// ── 容器 D：chat 命令节点 ────────────────────────────────────────────

// commandNodeChildren 画命令节点容器，并把 keyed 子槽交回内核。
//
// 命令节点只有一个来源：父级 `Store.SetCommand(...)`（宿主 v4 的 `command/run`
// 还没有投影到 Transport，见报告 §7）。没有节点 → 一个像素都不画（"无命令"态）。
func (p *Plugin) commandNodeChildren(f *flex.Frame, env *pluginapi.Env, drawSlot func(pluginapi.Slot)) {
	if f == nil || env == nil {
		return
	}
	uikernel.DrawCommandNodeSlots(f, env, p.commandNodeOptions(env, drawSlot))
}

// drawCommandNodeCards 是容器 D 在 `Painter.Draw` 里的那一半：兜底卡（不 dispatch）。
func (p *Plugin) drawCommandNodeCards(f *flex.Frame, env *pluginapi.Env) {
	if f == nil || env == nil {
		return
	}
	uikernel.DrawCommandNodeCards(f, env, p.commandNodeOptions(env, nil))
}

// commandNodeOptions 打这一帧的命令节点容器输入（两半共用，判据只有一份）。
func (p *Plugin) commandNodeOptions(env *pluginapi.Env, drawSlot func(pluginapi.Slot)) uikernel.CommandNodeOptions {
	width := env.Geometry.MainWidth
	if width <= 0 {
		width = env.MainWidth
	}
	return uikernel.CommandNodeOptions{
		Theme:    p.themeFor(env),
		Width:    width,
		Bounds:   env.Bounds,
		Nodes:    p.team.commandNodeViews(),
		Claims:   p.ClaimsCommandView,
		Toggle:   func(string) { p.team.ToggleCommandExpanded() },
		Dispatch: drawSlot,
	}
}

// ClaimsCommandView 是官方 keyed 槽的 `entryKey` 认领（`CommandNodeView.tsx:18`）。
//
// 官方：同一个 key 注册会**替换**该命令的渲染器，没被占的 key 用通用卡。
// 本包只认领 `team`（其余命令交给容器 D 的兜底卡）。
func (p *Plugin) ClaimsCommandView(name string) bool { return name == CommandKey }

// commandNodeViews 把 Store 里的命令节点翻成容器视图（含展开态）。
func (s *Store) commandNodeViews() []uikernel.CommandNode {
	s.mu.Lock()
	node := s.command
	expanded := s.commandExpanded
	s.mu.Unlock()
	if node == nil || node.Command == "" {
		return nil
	}
	out := uikernel.CommandNode{Command: node.Command, Expanded: expanded}
	if node.Outcome != nil {
		out.Outcome = &uikernel.CommandOutcome{Kind: node.Outcome.Kind, Text: node.Outcome.Text}
	}
	return []uikernel.CommandNode{out}
}

// ── 数据接线：provider 目录 ──────────────────────────────────────────

// SetProvidersRemote 接上/摘掉 provider 目录远端（Register 里由注入值来）。
//
// 一份状态存两处是有意的：Plugin 侧留一份给"这个能力接没接"的自省（壳/测试问），
// Store 侧那份是绘制路径真正读的（绘制里只能读 Store，不能读 Plugin 的可变字段）。
func (p *Plugin) SetProvidersRemote(remote ProvidersRemote) {
	p.mu.Lock()
	p.providers = remote
	p.mu.Unlock()
	p.team.mu.Lock()
	p.team.providersRemote = remote
	p.team.mu.Unlock()
}

// RefreshProviders 拉一次宿主 provider 目录，把结果写进 Store。
//
// 三条边界：
//
//	没接线                 → known=false（菜单整段隐藏，不画空菜单）
//	宿主 501/失败          → known=false + failure（容器画官方 .error 行）
//	宿主回空数组           → known=true + 空列表（菜单整段隐藏，但错误行不出现）
func (s *Store) RefreshProviders() {
	s.mu.Lock()
	if s.providersLoading {
		s.mu.Unlock()
		return
	}
	remote := s.providersRemote
	if remote == nil {
		s.mu.Unlock()
		return
	}
	s.providersLoading = true
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), providerFetchTimeout)
		defer cancel()
		list, err := remote.ListConfigurableProviders(ctx)

		s.mu.Lock()
		defer s.mu.Unlock()
		s.providersLoading = false
		if err != nil {
			// 取不到 ≠ 没有：known 保持 false，失败原因原样交给容器画。
			s.providersKnown = false
			s.providersFailure = err.Error()
			s.providers = nil
			if s.log != nil {
				s.log.Warn("llm.listConfigurableProviders 失败", "err", err)
			}
			return
		}
		s.providersKnown = true
		s.providersFailure = ""
		s.providers = providerOptions(list)
	}()
}

// menuProviders 把插件侧视图翻成 uikernel 的容器视图（同一组字段，一层薄映射）。
//
// 为什么有两个同名结构：uikernel 是壳层结构，不 import 插件；插件也不把
// uikernel 的类型当自己的数据形状。这一层映射是**唯一**的转换点。
func menuProviders(list []ModelProviderOption) []uikernel.ModelProviderOption {
	out := make([]uikernel.ModelProviderOption, 0, len(list))
	for _, item := range list {
		out = append(out, uikernel.ModelProviderOption{
			Provider:    item.Provider,
			DisplayName: item.DisplayName,
			Seed:        item.Seed,
			Failure:     item.Failure,
		})
	}
	return out
}

// providerOptions 把 wire 结构翻成容器视图（只挑界面要显示的字段）。
func providerOptions(list []apiclient.ConfigurableProvider) []ModelProviderOption {
	out := make([]ModelProviderOption, 0, len(list))
	for _, item := range list {
		option := ModelProviderOption{
			Provider:    item.Provider,
			DisplayName: item.DisplayName,
			Failure:     item.Error,
		}
		switch {
		case option.Failure != "":
			// 不可用行：不补 Seed（错误本身已经说明原因）。
		case item.SettingsNamespace != "":
			option.Seed = item.SettingsNamespace
		case !item.Declared:
			// 官方 declared=false = 当前 catalog 里没有模型声明它。
			option.Seed = zhModelProviderBare
		}
		out = append(out, option)
	}
	return out
}

// 容器 C 的文案（本包需要，与 uikernel 侧同名常量分开放在插件语言域里）。
const (
	zhModelMenuAria     = "模型与推理等级"
	zhModelProviderBare = "未在模型目录中声明"
)
