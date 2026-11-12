package dssidebar

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// FG-2 容器数值逐项来自官方 SidebarRoot / WorkspaceBrowser CSS。
// Windows titlebar 下 logoRow 为 40；brandName 的字号/行高是 18/24。
const (
	logoRowHeight          = 40
	brandIdentityHeight    = 24
	brandMarkSize          = 24
	brandNameWidth         = 156 // BrandWordmark(includeMark=false)，viewBox 156×24
	brandNameHeight        = 24
	brandNameFontSize      = 18
	brandNameLineHeight    = 24
	brandGap               = 8
	brandBadgeWidth        = 46 // 官方 wordmark badge clip rect
	brandBadgeHeight       = 14
	newSessionHeight       = 38
	newSessionMarginTop    = 8
	newSessionMarginBottom = 12
	newSessionFontSize     = 14
	newSessionLineHeight   = 22
	panelRowHeight         = 36
	panelListGap           = 2
	panelListBottom        = 8
	workspaceHeaderHeight  = 36
	workspaceHeaderBottom  = 4
	workspaceIconSize      = 28

	// dsshell 在两个 sidebar slot 之间固定放 8+1+8=17px。
	// 官方 regionArea 此处只有 sectionHeader 的 margin-top:2px，
	// 因而本插件自己的 header 向上补 15px；背景同时盖住那条非官方分隔线。
	workspaceShellMarginTop = -15
)

// frameBoxes 记录真实 flex 矩形，供两档无头窗口和组件测试量测。
// 不使用“常量等于自己”的自证代替布局证据。
type frameBoxes struct {
	Brand           ui.Box
	BrandIdentity   ui.Box
	BrandMark       ui.Box
	BrandName       ui.Box
	NewSession      ui.Box
	PanelList       ui.Box
	PanelRows       []ui.Box
	WorkspaceHeader ui.Box
	SearchButton    ui.Box
	Search          ui.Box
	ActivePanel     ui.Box
}

// panelRoute 是本插件当前真正可切换的面板描述。
// Draw==nil 的扩展描述不会生成入口，从结构上杜绝死按钮。
type panelRoute struct {
	ID    string
	Label string
	Order int
	Glyph string
	Draw  func(f *flex.Frame, env *pluginapi.Env)
}

func (p *Plugin) panelRoutes() []panelRoute {
	// 官方 plugin-manager(order 0) 与 schedule(order 10) 都同时注册 main 与
	// sidebar.panellist；这里的两个入口也都有下方真实 Draw，不是截图特判。
	routes := []panelRoute{
		{ID: "plugins", Label: "插件", Order: 0, Glyph: "▦", Draw: p.drawPluginsPanel},
		{ID: "schedules", Label: "自动化任务", Order: 10, Glyph: "◷", Draw: p.drawSchedulesPanel},
	}
	routes = append(routes, p.extensionRoutes...)
	out := make([]panelRoute, 0, len(routes))
	seen := make(map[string]bool, len(routes))
	for _, route := range routes {
		route.ID = strings.TrimSpace(route.ID)
		route.Label = strings.TrimSpace(route.Label)
		if route.ID == "" || route.Label == "" || route.Draw == nil || seen[route.ID] {
			continue
		}
		seen[route.ID] = true
		out = append(out, route)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// drawHeader 画官方品牌行、新会话按钮和真实导航；工作区 header 在 list 槽，
// 这样 V2/U3A 的 drawList 与全部搜索/滚动测试仍走原函数。
func (p *Plugin) drawHeader(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	p.frameBoxes.PanelRows = p.frameBoxes.PanelRows[:0]
	f.Column().FillWidth().Children(func() {
		p.frameBoxes.Brand = f.Row().FillWidth().Height(logoRowHeight).Items(ui.Center).Children(func() {
			p.frameBoxes.BrandIdentity = f.Row().Height(brandIdentityHeight).Gap(brandGap).Items(ui.Center).Children(func() {
				p.frameBoxes.BrandMark = f.Icon(fishIcon()).Width(brandMarkSize).
					Height(brandMarkSize * fishAspect(fishViewBoxW, fishViewBoxH)).Color(t.LabelPrimary)
				p.frameBoxes.BrandName = f.Row().Width(brandNameWidth).Height(brandNameHeight).
					Gap(6).Items(ui.Center).Children(func() {
					f.Text("deepseek").FontSize(brandNameFontSize).LineHeight(brandNameLineHeight).
						Color(t.LabelPrimary).Grow(1).Ellipsis()
					f.Row().Width(brandBadgeWidth).Height(brandBadgeHeight).Radius(t.Radii.XS).
						Background(t.LabelPrimary).Items(ui.Center).Justify(ui.Center).Children(func() {
						f.Text("HARNESS").FontSize(9).LineHeight(brandBadgeHeight).
							Weight(gfx.WeightMedium).Color(t.ButtonPrimaryFg)
					})
				})
			})
			// 没有 ToggleSidebar 动作时不画官方收起钮；品牌也不冒充按钮。
			f.Spacer()
		})

		newBox := f.Row().FillWidth().Height(newSessionHeight).Radius(t.Radii.MD).
			Background(t.BgLayer2).Border(0.5, t.BorderL3).Hover(ui.Look{}.Background(t.Hover)).
			MarginSides(newSessionMarginTop, 2, newSessionMarginBottom, 2).
			Items(ui.Center).Justify(ui.Center).Hit(env.Tree.ID(env.Feat, "new-session"), "新建会话").Children(func() {
			f.Row().Items(ui.Center).Gap(6).Children(func() {
				f.Text("+").FontSize(newSessionFontSize).LineHeight(newSessionLineHeight).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary)
				f.Text("新会话").FontSize(newSessionFontSize).LineHeight(newSessionLineHeight).
					Weight(gfx.WeightMedium).Color(t.LabelPrimary)
			})
		})
		p.frameBoxes.NewSession = newBox
		if newBox.Clicked() {
			p.activePanel = ""
			if env.Act.NewSession != nil {
				env.Act.NewSession()
			}
		}

		routes := p.panelRoutes()
		if len(routes) == 0 {
			p.frameBoxes.PanelList = ui.Box{}
			return
		}
		p.frameBoxes.PanelList = f.Column().FillWidth().Gap(panelListGap).
			MarginSides(0, 0, panelListBottom, 0).Children(func() {
			for _, route := range routes {
				route := route
				var row ui.Box
				f.Row().FillWidth().PadSides(0, 2, 0, 2).Children(func() {
					row = p.prims.ListRow(f, env.Tree.ID(env.Feat, "panel-"+route.ID),
						p.activePanel == route.ID, panelRowHeight, func() {
							f.Row().Width(16).Height(22).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
								f.Text(route.Glyph).FontSize(16).LineHeight(22).Color(t.LabelSecondary)
							})
							f.Text(route.Label).FontSize(t.Fonts.Body).LineHeight(22).
								Color(t.LabelPrimary).MarginSides(0, 0, 0, 8).Ellipsis()
						})
				})
				p.frameBoxes.PanelRows = append(p.frameBoxes.PanelRows, row)
				if row.Clicked() {
					p.activePanel = route.ID
				}
			}
		})
	})
}

// drawWorkspace 把官方工作区工具带和原会话列表组合成一个 slot 子树。
func (p *Plugin) drawWorkspace(f *flex.Frame, env *pluginapi.Env) {
	f.Column().FillWidth().FillHeight().Children(func() {
		p.drawWorkspaceHeaderMargin(f, env, workspaceShellMarginTop)
		p.drawList(f, env)
	})
}

func (p *Plugin) drawWorkspaceHeader(f *flex.Frame, env *pluginapi.Env) {
	p.drawWorkspaceHeaderMargin(f, env, 0)
}

func (p *Plugin) drawWorkspaceHeaderMargin(f *flex.Frame, env *pluginapi.Env, marginTop float32) {
	t := p.theme
	p.frameBoxes.SearchButton = ui.Box{}
	p.frameBoxes.Search = ui.Box{}
	p.frameBoxes.WorkspaceHeader = f.Row().Width(p.width-sidebarInline-8).Height(workspaceHeaderHeight).
		MarginSides(marginTop, -4, workspaceHeaderBottom, 4).Background(t.SidebarFill).
		Items(ui.Center).Gap(4).Children(func() {
		if p.searchExpanded {
			p.frameBoxes.Search = f.Row().FillWidth().Height(searchHeight).Radius(t.Radii.MD).
				Border(0.5, t.BorderL4).Items(ui.Center).Children(func() {
				f.Row().Width(28).Height(searchHeight).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
					f.Text("⌕").FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
				})
				f.Field(&p.search, env.Tree.ID(env.Feat, "search"), "搜索", "过滤会话…").
					Grow(1).Height(searchHeight).FontSize(t.Fonts.Secondary)
				clear := f.Row().Width(28).Height(searchHeight).Shrink(0).Radius(t.Radii.SM).
					Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
					Hit(env.Tree.ID(env.Feat, "workspace-search-clear"), "清空搜索").Children(func() {
					f.Text("×").FontSize(t.Fonts.Secondary).Color(t.LabelTertiary)
				})
				if clear.Clicked() {
					p.search.SetText("")
					p.searchExpanded = false
				}
			})
			return
		}

		f.Text("工作区").FontSize(t.Fonts.Body).LineHeight(20).Color(t.LabelTertiary).
			MaxWidthPercent(45).Ellipsis()
		f.Spacer()
		search, searchClicked := p.workspaceIconButton(f, env, "workspace-search", "⌕", "搜索会话")
		p.frameBoxes.SearchButton = search
		if searchClicked {
			p.searchExpanded = true
		}
		_, viewClicked := p.workspaceIconButton(f, env, "workspace-view-options", "▾", "全部收起或展开")
		if viewClicked {
			p.toggleAll()
		}
		// 没有 OS 目录选择/workspace mutation，官方“添加工作区”整钮隐藏。
	})
}

func (p *Plugin) workspaceIconButton(f *flex.Frame, env *pluginapi.Env, id, glyph, label string) (ui.Box, bool) {
	box := f.Row().Width(workspaceIconSize).Height(workspaceIconSize).Shrink(0).Radius(p.theme.Radii.SM).
		Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(p.theme.Hover)).
		Hit(env.Tree.ID(env.Feat, id), label).Children(func() {
		f.Text(glyph).FontSize(14).LineHeight(20).Color(p.theme.LabelSecondary)
	})
	return box, box.Clicked()
}

// drawActivePanel 在主列实画当前路由。它不改写 conversation、composer 或其状态，
// 因而返回会话不会丢 session id、草稿或滚动位置；F1 快捷键层仍可在其上显示。
func (p *Plugin) drawActivePanel(f *flex.Frame, env *pluginapi.Env) {
	p.frameBoxes.ActivePanel = ui.Box{}
	if p.activePanel == "" {
		return
	}
	var selected *panelRoute
	for _, route := range p.panelRoutes() {
		if route.ID == p.activePanel {
			route := route
			selected = &route
			break
		}
	}
	if selected == nil {
		p.activePanel = ""
		return
	}
	left := env.Geometry.SidebarWidth
	if left <= 0 {
		left = p.width
	}
	width := env.Geometry.MainWidth
	if width <= 0 {
		width = env.Bounds.Width - left - env.Geometry.RightWidth
	}
	height := env.Bounds.Height - env.Geometry.BottomHeight
	if height <= 0 {
		height = env.Geometry.Height - env.Geometry.BottomHeight
	}
	if width <= 0 || height <= 0 {
		return
	}
	p.frameBoxes.ActivePanel = f.Row().Absolute().Left(left).Top(0).Width(width).Height(height).
		Background(p.theme.BgBase).Clip().Hit(env.Tree.ID(env.Feat, "active-panel"), selected.Label).Children(func() {
		f.Column().FillWidth().FillHeight().PadSides(28, 28, 28, 28).Children(func() {
			selected.Draw(f, env)
		})
	})
}

func (p *Plugin) drawPluginsPanel(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Column().FillWidth().Gap(8).Children(func() {
		f.Text("插件").FontSize(24).LineHeight(32).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		f.Text("当前二进制已登记的插件").FontSize(t.Fonts.Secondary).LineHeight(20).Color(t.LabelTertiary)
		f.Column().FillWidth().Gap(2).PadSides(12, 0, 0, 0).Children(func() {
			for _, id := range pluginapi.Registered() {
				f.Text(id).FontSize(t.Fonts.Body).LineHeight(22).Color(t.LabelPrimary)
			}
		})
		// Registered 是编译期注册真值，不冒充 kernel 的 enabled/装配状态；
		// 没有管理 mutation，因此不画安装、启停或配置按钮。
	})
}

func (p *Plugin) drawSchedulesPanel(f *flex.Frame, env *pluginapi.Env) {
	t := p.theme
	f.Column().FillWidth().Gap(8).Children(func() {
		f.Text("自动化任务").FontSize(24).LineHeight(32).Weight(gfx.WeightMedium).Color(t.LabelPrimary)
		f.Text("当前宿主未提供任务目录").FontSize(t.Fonts.Body).LineHeight(22).Color(t.LabelTertiary)
		// 没有 schedule catalog/history/action 时不编任务、不画新建/删除死按钮。
	})
}
