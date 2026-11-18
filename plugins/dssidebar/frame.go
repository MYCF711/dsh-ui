package dssidebar

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/entrypins"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// FG-2 容器数值逐项来自官方 SidebarRoot / WorkspaceBrowser CSS。
//
// 品牌名**不是文字**：官方 `sidebar.brand.name` 的官方占用方是
// `<BrandWordmark includeMark={false} />`（brand-official/src/client/Brand.tsx:17-19），
// 也就是 9 条字形 + 7 条徽标字形的矢量（见 dsbrand.brand_wordmark.go 的出处）。
// 所以我们画 icon，而不是 `ui.Text("deepseek")`——照旧的写法是把品牌名当文字排，
// 字宽、字重、字距全都只能猜，而官方是画好的路径。
const (
	logoRowHeight          = 40
	brandIdentityHeight    = 24
	brandMarkSize          = dsbrand.SidebarMarkSize // 24：SidebarRoot.tsx:297
	brandNameHeight        = 24                      // BrandWordmark 的 size（Brand.tsx:18 不传 size）
	brandNameViewBoxW      = dsbrand.BrandNameViewBoxW
	brandNameViewBoxH      = dsbrand.BrandNameViewBoxH
	brandGap               = 5 // SidebarRoot.module.css:279-285 `.brandIdentity{gap:5px}`
	brandPaddingLeft       = 8 // SidebarRoot.module.css:64-66（Windows 展开态）
	brandMarkOffsetX       = 1 // SidebarRoot.module.css:72-74
	newSessionHeight       = 38
	newSessionMarginTop    = 6  // SidebarRoot.module.css:84-86（Windows）
	newSessionMarginInline = 4  // SidebarRoot.module.css:430：paint-inset 3 + 1
	panelListTop           = 12 // scrollRegion margin-top 4 + scrollArea 首行 8（:533-555）
	newSessionFontSize     = 14
	newSessionLineHeight   = 22
	panelRowHeight         = 32 // SidebarRoot.module.css:685-703
	panelListGap           = 2
	panelListBottom        = 8
	workspaceHeaderHeight  = 36
	workspaceHeaderBottom  = 0 // WorkspaceBrowser.module.css:66-79：--workspace-toolbar-gap:0
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
	SearchCards     []ui.Box
	ActivePanel     ui.Box
}

// panelRoute 是本插件当前真正可切换的面板描述。
// Draw==nil 的扩展描述不会生成入口，从结构上杜绝死按钮。
type panelRoute struct {
	ID    string
	Label string
	Order int
	Glyph string
	Draw  func(f ui.C, env *pluginapi.Env)
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
func (p *Plugin) drawHeader(f ui.C, env *pluginapi.Env) {
	t := p.theme
	p.frameBoxes.PanelRows = p.frameBoxes.PanelRows[:0]
	ui.Column(f).FillWidth().Children(func() {
		p.frameBoxes.Brand = ui.Row(f).FillWidth().Height(logoRowHeight).Items(ui.Center).Children(func() {
			p.frameBoxes.BrandIdentity = ui.Row(f).Height(brandIdentityHeight).Gap(brandGap).
				MarginSides(1, 0, -1, brandPaddingLeft).Items(ui.Center).Children(func() {
				p.frameBoxes.BrandMark = ui.Icon(f, fishIcon()).Width(brandMarkSize).
					Height(brandMarkSize*fishAspect(fishViewBoxW, fishViewBoxH)).
					MarginSides(0, 0, 0, brandMarkOffsetX).Color(t.LabelPrimary)
				// 品牌名 = 官方 BrandWordmark 的 `includeMark={false}` 部分：
				// 9 条字形（LabelPrimary）+ 徽标板（52×14、圆角 2、LabelPrimary）
				// + 板内 7 条反色字形（ButtonPrimaryFg）。
				// 官方对应结构：Brand.tsx:17-19 + BrandWordmark 的
				// `<rect x="129.348" y="5.5" width="52" height="14" rx="2">`
				// 与 `clipPath#dsh-wordmark-badge-clip` translate(132.348 5.5) 46×14。
				p.frameBoxes.BrandName = ui.Row(f).Height(brandNameHeight).
					Items(ui.Center).Children(func() {
					ui.Icon(f, dsbrand.BrandNameIcon()).Width(brandNameViewBoxW).
						Height(brandNameViewBoxH).Color(t.LabelPrimary)
					ui.Row(f).Height(dsbrand.BadgePlateH).
						MarginLeft(dsbrand.BadgePlateX - dsbrand.BrandNameViewBoxW - dsbrand.BadgeInset).
						Radius(dsbrand.BadgeRadius).Background(t.LabelPrimary).
						Items(ui.Center).Children(func() {
						ui.Icon(f, dsbrand.BrandBadgeInkIcon()).Size(dsbrand.BadgeClipW, dsbrand.BadgePlateH).
							Color(t.ButtonPrimaryFg)
					})
				})
			})
			// 没有 ToggleSidebar 动作时不画官方收起钮；品牌也不冒充按钮。
			ui.Spacer(f)
		})

		newBox := ui.Row(f).FillWidth().Height(newSessionHeight).Radius(t.Radii.MD).
			Background(t.BgLayer2).Border(0.5, t.BorderL3).Hover(ui.Look{}.Background(t.Hover)).
			MarginSides(newSessionMarginTop, newSessionMarginInline, 0, newSessionMarginInline).
			Items(ui.Center).Justify(ui.Center).Hit(env.Tree.ID(env.Feat, "new-session"), "新建会话").Children(func() {
			ui.Row(f).Items(ui.Center).Gap(6).Children(func() {
				if icon, ok := dsassets.SidebarNewChatMedium(t.LabelPrimary); ok {
					ui.Icon(f, icon).Size(14, 14).Color(t.LabelPrimary)
				}
				ui.Text(f, "新会话").FontSize(newSessionFontSize).LineHeight(newSessionLineHeight).
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

		// 官方列表判据（SidebarRoot.tsx:404-405）：未固定的面板不进列表，
		// 改由 More 菜单列出；没有一个未固定时与原来逐像素相同。
		routes := p.visiblePanelRoutes(env)
		actionsLive := p.entryActionsLive(env)
		if len(routes) == 0 && !actionsLive {
			p.frameBoxes.PanelList = ui.Box{}
			return
		}
		p.frameBoxes.PanelList = ui.Column(f).FillWidth().Gap(panelListGap).
			MarginSides(panelListTop, 0, panelListBottom, 0).Children(func() {
			for _, route := range routes {
				route := route
				var row ui.Box
				rowBody := func() {
					row = p.prims.ListRow(f, env.Tree.ID(env.Feat, "panel-"+route.ID),
						p.activePanel == route.ID, panelRowHeight, func() {
							ui.Row(f).Width(16).Height(22).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
								if icon, ok := sidebarPanelIcon(route.ID, t.LabelSecondary); ok {
									ui.Icon(f, icon).Size(16, 16).Color(t.LabelSecondary)
								}
							})
							ui.Text(f, route.Label).FontSize(t.Fonts.Body).LineHeight(22).
								Color(t.LabelPrimary).MarginSides(0, 0, 0, 6).Ellipsis()
						})
				}
				// 行尾「取消固定」：官方 `.panelUnpin`（绝对定位在行右端）。本仓同列
				// 布局：行占剩余宽度、按钮 28×28 跟在后面（见 entry_menu.go 差异 2）。
				panelRowID := env.Tree.ID(env.Feat, "panel-"+route.ID)
				if actionsLive && p.pins != nil && route.ID != entrypins.FixedPanelID {
					ui.Row(f).FillWidth().Children(func() {
						ui.Row(f).Grow(1).MinWidth(0).Children(rowBody)
						// 行 id 一起传：`.panelUnpin` 的显隐按"指针在不在这—行"判。
						p.drawPanelUnpin(f, env, route, panelRowID)
					})
				} else {
					ui.Row(f).FillWidth().Children(rowBody)
				}
				p.frameBoxes.PanelRows = append(p.frameBoxes.PanelRows, row)
				if row.Clicked() {
					// 与 More 菜单里的 onOpen 走同一条路（任务页顺带拉一次目录）。
					p.openPanel(route.ID)
				}
			}
			// More 行：只在座位有占用方时出现（官方 `hasEntryActions`）。
			if actionsLive {
				p.drawPanelMoreRow(f, env)
			}
		})
	})
}

// drawWorkspace 把官方工作区工具带和原会话列表组合成一个 slot 子树。
func (p *Plugin) drawWorkspace(f ui.C, env *pluginapi.Env) {
	ui.Column(f).FillWidth().FillHeight().Children(func() {
		p.drawWorkspaceHeaderMargin(f, env, workspaceShellMarginTop)
		p.drawList(f, env)
	})
}

func (p *Plugin) drawWorkspaceHeader(f ui.C, env *pluginapi.Env) {
	p.drawWorkspaceHeaderMargin(f, env, 0)
}

func (p *Plugin) drawWorkspaceHeaderMargin(f ui.C, env *pluginapi.Env, marginTop float32) {
	t := p.theme
	p.frameBoxes.SearchButton = ui.Box{}
	p.frameBoxes.Search = ui.Box{}
	// listArea 的 -4/+4 相抵；右侧到侧栏边缘 2px。sectionHeader 自身
	// padding-left:12、非搜索态 margin-right:-4/padding-right:1（官方 :66-91）。
	header := ui.Row(f).Width(p.width - sidebarInline - 4).Height(workspaceHeaderHeight).
		Background(t.SidebarFill).Items(ui.Center)
	if p.searchExpanded {
		header = header.MarginSides(marginTop, 0, workspaceHeaderBottom, 0).PadSides(0, 4, 0, 4).Gap(0)
	} else {
		header = header.MarginSides(marginTop, -4, workspaceHeaderBottom, 0).PadSides(0, 1, 0, 12).Gap(4)
	}
	p.frameBoxes.WorkspaceHeader = header.Children(func() {
		if p.searchExpanded {
			p.frameBoxes.Search = ui.Row(f).FillWidth().Height(searchHeight).Radius(t.Radii.MD).
				Border(0.5, t.BorderL4).Items(ui.Center).Children(func() {
				ui.Row(f).Width(28).Height(searchHeight).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
					if icon, ok := dsassets.WorkspaceSearch(t.LabelCaption); ok {
						ui.Icon(f, icon).Size(11, 11).Color(t.LabelCaption)
					}
				})
				ui.TextFieldID(f, env.Tree.ID(env.Feat, "search"), &p.search, "搜索", "过滤会话…").
					Grow(1).Height(searchHeight).FontSize(13)
				clear := ui.Row(f).Width(24).Height(24).Shrink(0).Radius(t.Radii.SM).
					Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).
					Hit(env.Tree.ID(env.Feat, "workspace-search-clear"), "清空搜索").Children(func() {
					if icon, ok := dsassets.WorkspaceCloseFill(t.LabelSecondary); ok {
						ui.Icon(f, icon).Size(14, 14).Color(t.LabelSecondary)
					}
				})
				if clear.Clicked() {
					p.search.SetText("")
					p.searchExpanded = false
				}
			})
			return
		}

		ui.Text(f, "工作区").FontSize(t.Fonts.Body).LineHeight(20).Color(t.LabelTertiary).
			MaxWidthPercent(45).Ellipsis()
		ui.Spacer(f)
		search, searchClicked := p.workspaceIconButton(f, env, "workspace-search", "搜索会话", func() {
			if icon, ok := dsassets.WorkspaceSearch(t.LabelSecondary); ok {
				ui.Icon(f, icon).Size(14, 14).Color(t.LabelSecondary)
			}
		})
		p.frameBoxes.SearchButton = search
		if searchClicked {
			p.searchExpanded = true
		}
		_, viewClicked := p.workspaceIconButton(f, env, "workspace-view-options", "全部收起或展开", func() {
			drawSlidersTwoIcon(f, t.LabelSecondary, 16)
		})
		if viewClicked {
			p.toggleAll()
		}
		// 只有壳接了真实目录流程才画；图标是 WorkspaceBrowser.tsx:1310 的 project-add 16px。
		if env.Act.OpenDirectoryPicker != nil {
			_, addClicked := p.workspaceIconButton(f, env, "workspace-add", "添加工作区", func() {
				if icon, ok := dsassets.WorkspaceProjectAdd(t.LabelSecondary); ok {
					ui.Icon(f, icon).Size(16, 16).Color(t.LabelSecondary)
				}
			})
			if addClicked {
				env.Act.OpenDirectoryPicker()
			}
		}
	})
}

func (p *Plugin) workspaceIconButton(f ui.C, env *pluginapi.Env, id, label string, draw func()) (ui.Box, bool) {
	box := ui.Row(f).Width(workspaceIconSize).Height(workspaceIconSize).Shrink(0).Radius(p.theme.Radii.SM).
		Items(ui.Center).Justify(ui.Center).Hover(ui.Look{}.Background(p.theme.Hover)).
		Hit(env.Tree.ID(env.Feat, id), label).Children(draw)
	return box, box.Clicked()
}

// drawActivePanel 在主列实画当前路由。它不改写 conversation、composer 或其状态，
// 因而返回会话不会丢 session id、草稿或滚动位置；F1 快捷键层仍可在其上显示。
func (p *Plugin) drawActivePanel(f ui.C, env *pluginapi.Env) {
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
	p.frameBoxes.ActivePanel = ui.Row(f).Absolute().Left(left).Top(0).Width(width).Height(height).
		Background(p.theme.BgBase).Clip().Hit(env.Tree.ID(env.Feat, "active-panel"), selected.Label).Children(func() {
		ui.Column(f).FillWidth().FillHeight().PadSides(28, 28, 28, 28).Children(func() {
			selected.Draw(f, env)
		})
	})
}

func (p *Plugin) drawPluginsPanel(f ui.C, env *pluginapi.Env) {
	// 官方 plugin-manager 的管理页在 plugins_page.go（数据面
	// Actions.PluginBundles / PluginBundleDetail + 两条真实写动作）。
	// 入口仍是本文件的 `plugins` 路由（官方 `sidebar.panellist` id=plugins、
	// label=插件、order=0——三者逐字一致）。
	p.drawPluginManagerPage(f, env)
}

func (p *Plugin) drawSchedulesPanel(f ui.C, env *pluginapi.Env) {
	// 官方 TaskManagerPage 的落地在 schedule_page.go（数据面 Actions.ScheduleCatalog）。
	p.drawSchedulePage(f, env)
}
