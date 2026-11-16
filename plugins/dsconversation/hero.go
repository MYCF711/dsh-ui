package dsconversation

import (
	"github.com/MYCF711/dsh-ui/plugins/dsbrand"
	"path"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 本文件是「居中欢迎区」——官方 `ui-conversation` 的 HeroShell。
//
// 逐条数值都来自官方 CSS / JSX 原文（asar 内
// `ui-conversation/src/client/skeleton/HeroShell.module.css`，类名前缀 WOwLGG；
// 结构见同一包的 HeroShell 组件）：
//
//	.WOwLGG_root{justify-content:center;align-items:center;min-width:0;height:100%;
//	             padding:0 24px;display:flex}
//	.WOwLGG_stack{width:100%;max-width:var(--dsh-composer-card-max-width);
//	              flex-direction:column;align-items:stretch;gap:12px;display:flex}
//	.WOwLGG_headline{color:label-primary;flex-wrap:wrap;justify-content:center;
//	                 align-items:center;gap:12px 10px;font-size:26px;font-weight:500;
//	                 line-height:32px;display:flex}
//	.WOwLGG_titleGroup{gap:4px 7px;justify-content:center;align-items:center;display:flex}
//	.WOwLGG_previewBadge{border:.5px solid var(--dsw-alias-interactive-bg-hover);
//	                     background:var(--dsw-alias-state-business-tertiary);
//	                     color:var(--dsw-alias-label-primary-bluish);
//	                     border-radius:999px;margin-top:2px;padding:1px 7px 0;
//	                     font-size:12px;font-weight:500;line-height:18px}
//	.WOwLGG_body{flex-direction:column;gap:12px;min-width:0;display:flex}
//	.WOwLGG_body>.WOwLGG_workspaceRow{padding-left:8px;align-items:center;display:flex}
//	.WOwLGG_workspace{min-height:28px;border-radius:var(--dsw-radius-sm);
//	                  padding:0 8px;color:label-primary;font-size:13px;
//	                  font-weight:500;line-height:20px;display:inline-flex}
//	HeroFish: width:34, height:34 * VIEWBOX.height / VIEWBOX.width
//	phase=hero 时 .scrollBody{justify-content:center} —— 整块垂直居中
//
// 文案取自官方 zh 语言包：hero.headline =「探索未至之境」，hero.preview =「预览版」。
const (
	// heroFishWidth 是官方 HeroFish 的 width（源码注释原文：「The hero fish (34px wide)」）。
	heroFishWidth = 34
	// heroFishHeight 由 viewBox 等比算出：34 * 17.04 / 23.16 = 25.02。
	heroFishHeight = heroFishWidth * dsbrand.FishViewBoxH / dsbrand.FishViewBoxW

	// heroPadX 是官方 root 的 `padding:0 24px`。
	heroPadX = 24

	// heroStackGap 是官方 stack 的 `gap:12px`。
	heroStackGap = 12
	// 官方 body：chat content = clamp(680, main*.64, 920)，composer card 再 +32。
	heroContentMin = 680
	heroContentMax = 920
	heroCardExtra  = 32

	// headline 的 `gap:12px 10px`（行间距 12、列间距 10）。
	heroHeadlineGapX = 10
	// headline 的 `font-size:26px;line-height:32px`（Theme 里没有 26 这一档，
	// 它是官方首屏的专门字号，按原值用，并留出处）。
	heroHeadlineSize = 26
	heroHeadlineLine = 32
	// titleGroup 的 `gap:4px 7px`。
	heroTitleGapX = 7

	// previewBadge 的 `padding:1px 7px 0`、`margin-top:2px`、`border-radius:999px`、
	// `border:.5px`。字号/行高走 Theme 的 Caption/CaptionLine（12/18，与官方同值）。
	heroBadgePadX      = 7
	heroBadgePadTop    = 1
	heroBadgeMarginTop = 2
	heroBadgeRadius    = 999
	heroBadgeBorder    = 0.5

	// ConversationRoot 的 heroWorkspaceRow 是 `padding:0 16px 0 20px`；
	// 胶囊本身 `padding:0 8px;min-height:28px;max-width:360px`。
	heroWorkspaceRowPadLeft  = 20
	heroWorkspaceRowPadRight = 16
	heroWorkspacePadX        = 8
	heroWorkspaceMinHeight   = 28
	heroWorkspaceMaxWidth    = 360
	// workspace 文字是官方的 13/20（Theme 里 Secondary/SecondaryLine = 13/20，同值）。
)

// 官方 zh 语言包文案（ui-conversation 的 `hero.headline` / `hero.preview`）。
const (
	heroHeadlineText = "探索未至之境"
	heroBadgeText    = "预览版"
	// 以下逐字来自官方 conversation.hero / ui-workspace 的 zh 字典。
	heroChooseWorkspaceText  = "选择工作区"
	heroDefaultWorkspaceText = "默认工作区"
	heroAddWorkspaceText     = "添加工作区…"
	heroWorkspaceLoadingText = "正在加载工作区…"
)

// drawHero 画官方首屏：居中的「品牌标记 + 标题 + 预览徽标」，底下是当前工作区。
//
// 什么时候画：一次会话都没有（`Cur == ""`）或会话里一条消息都没有、
// 没有流式正文、也没有在跑的 turn（`Busy`）。
//
// 最后那一条是**真 bug 的边界**：发出 prompt 之后、第一个正文 token 到达之前，
// Msgs 是空的、Stream 也是空的——这时候画欢迎区，等于把用户刚发的消息"顶掉"，
// 画面会在几百毫秒里闪回首屏。官方同一位置的判据是 conversationPhase，
// 不是"消息数组是否为空"。
//
// 什么时候不画：**拿不到真实内容就只画静态品牌面，不编任何"最近会话""推荐问题"。**
// 官方 hero 里剩下的内容都需要真实数据：
//   - `conversation.hero.workspace`：WorkspaceChip + 单选 WorkspacePicker；pending/registry
//     未落地时的 cwd 桥接逐条照官方，没有当前会话就显示 placeholder，不借别的会话 cwd。
//   - `conversation.hero.options` / `agentPreset`：需要真实的模型/预设动作，未实现。
type heroBoxes struct {
	Root, Stack, Headline, Mark, TitleGroup, Badge, WorkspaceRow, Workspace ui.Box
}

// heroStackWidth 是官方 `--dsh-composer-card-max-width`：
// clamp(680px, mainWidth*.64, 920px) + 32px。
func heroStackWidth(mainWidth float32) float32 {
	content := mainWidth * 0.64
	if content < heroContentMin {
		content = heroContentMin
	}
	if content > heroContentMax {
		content = heroContentMax
	}
	return content + heroCardExtra
}

func (p *Plugin) drawHero(f ui.C, env *pluginapi.Env) {
	t := p.theme
	p.heroBoxes = heroBoxes{}
	mainWidth := env.MainWidth
	if mainWidth <= 0 {
		mainWidth = f.Bounds().Width
	}

	p.heroBoxes.Root = ui.Column(f).WidthPercent(100).Grow(1).MinHeight(160).
		Justify(ui.Center).Items(ui.Center).PadX(heroPadX).Children(func() {
		p.heroBoxes.Stack = ui.Column(f).WidthPercent(100).MaxWidth(heroStackWidth(mainWidth)).Gap(heroStackGap).Children(func() {
			// headline：品牌标记 + (标题 + 徽标)，三者垂直居中、整体水平居中。
			p.heroBoxes.Headline = ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Center).
				Gap(heroHeadlineGapX).Children(func() {
				// 品牌标记：官方 34px 宽的鲸鱼，纯填充，颜色 label-primary。
				// 品牌标记：优先用注入的（ui.brand），没有就退回包级默认。
				// **两个分支是同一个 FishIcon**，所以注入与否画出来一模一样——
				// 这里要的是可替换，不是换一种画法。
				brandIcon, brandW := dsbrand.FishIcon(), float32(heroFishWidth)
				if p.brand != nil {
					brandIcon, brandW = p.brand.Icon, p.brand.Width
				}
				p.heroBoxes.Mark = ui.Icon(f, brandIcon).Width(brandW).Height(brandW * dsbrand.FishViewBoxH / dsbrand.FishViewBoxW).
					Color(t.LabelPrimary)

				p.heroBoxes.TitleGroup = ui.Row(f).Items(ui.Center).Gap(heroTitleGapX).Children(func() {
					ui.Text(f, heroHeadlineText).
						FontSize(heroHeadlineSize).LineHeight(heroHeadlineLine).
						Weight(gfx.WeightMedium).Color(t.LabelPrimary)
					p.drawPreviewBadge(f)
				})
			})

			// body：官方 WorkspaceChip + WorkspacePicker。没有会话时显示官方 placeholder；
			// registry 为空且已 ready 时，点击直接升起目录流，不多点一次单行菜单。
			label := heroWorkspaceLabel(env)
			interactive := heroWorkspaceMenuAvailable(env)
			if label == "" {
				label = heroChooseWorkspaceText
			}
			if label != heroChooseWorkspaceText || interactive {
				p.heroBoxes.WorkspaceRow = ui.Row(f).FillWidth().
					PadSides(0, heroWorkspaceRowPadRight, 0, heroWorkspaceRowPadLeft).
					Children(func() {
						chip := ui.Row(f).Height(heroWorkspaceMinHeight).MaxWidth(heroWorkspaceMaxWidth).
							PadX(heroWorkspacePadX).Radius(t.Radii.SM).Items(ui.Center).Gap(4)
						if interactive {
							chip = chip.Hover(ui.Look{}.Background(t.Hover)).
								Hit(env.Tree.ID(env.Feat, "hero-workspace"), heroChooseWorkspaceText)
						}
						chip = chip.Children(func() {
							ui.Text(f, label).
								FontSize(t.Fonts.Secondary).LineHeight(t.Fonts.SecondaryLine).
								Weight(gfx.WeightMedium).Color(t.LabelPrimary).
								MaxLines(1).Ellipsis()
							if interactive {
								ui.Text(f, "▾").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
							}
						})
						p.heroBoxes.Workspace = chip
						p.drawWorkspacePicker(f, env, chip, interactive && chip.Clicked())
					})
			}
		})
	})
}

// heroWorkspaceLabel 按官方 ConversationContent 的优先级解析 chip：
// pending → 当前会话所属 registry 项 → registry 未就绪时的 cwd 桥接 → placeholder。
// 没有当前会话时绝不借用列表里别的会话 cwd。
func heroWorkspaceLabel(env *pluginapi.Env) string {
	if env.Data.PendingWorkspaceID != "" {
		for _, workspace := range env.Data.Workspaces {
			if workspace.WorkspaceID == env.Data.PendingWorkspaceID {
				return heroWorkspaceTitle(workspace)
			}
		}
	}
	if env.Data.Cur == "" {
		return ""
	}
	for _, workspace := range env.Data.Workspaces {
		for _, sessionID := range workspace.SessionIDs {
			if sessionID == env.Data.Cur {
				return heroWorkspaceTitle(workspace)
			}
		}
	}
	if env.Data.WorkspacesKnown {
		return "" // 已 ready 但没有 owner：官方显示 placeholder，不回退已删除目录的 cwd。
	}
	for _, session := range env.Data.Sessions {
		if session.ID == env.Data.Cur {
			return workspaceOf(session.Cwd)
		}
	}
	return workspaceOf(env.Data.CWD)
}

func heroWorkspaceTitle(workspace pluginapi.Workspace) string {
	if workspace.Title == "" {
		return heroDefaultWorkspaceText
	}
	return workspace.Title
}

func heroWorkspaceMenuAvailable(env *pluginapi.Env) bool {
	return env.Act.OpenDirectoryPicker != nil ||
		(env.Act.SelectWorkspace != nil && len(env.Data.Workspaces) > 0)
}

func heroSelectedWorkspaceID(env *pluginapi.Env) string {
	if env.Data.PendingWorkspaceID != "" {
		return env.Data.PendingWorkspaceID
	}
	for _, workspace := range env.Data.Workspaces {
		for _, sessionID := range workspace.SessionIDs {
			if sessionID == env.Data.Cur {
				return workspace.WorkspaceID
			}
		}
	}
	return ""
}

// drawWorkspacePicker 是官方 WorkspacePickFlow 的首屏菜单面：已有工作区单选，
// 「添加工作区…」升起同一个 directory-flow；busy 时所有菜单动作禁用。
func (p *Plugin) drawWorkspacePicker(f ui.C, env *pluginapi.Env, anchor ui.Box, clicked bool) {
	if !heroWorkspaceMenuAvailable(env) {
		p.workspaceMenuOpen = false
		return
	}
	open := p.workspaceMenuOpen
	addOnlyDirect := env.Data.WorkspacesKnown && len(env.Data.Workspaces) == 0 &&
		env.Act.OpenDirectoryPicker != nil && !env.Data.DirectoryBusy
	if clicked {
		if addOnlyDirect {
			open = false
			env.Act.OpenDirectoryPicker()
		} else {
			open = !open
		}
	}
	ui.Menu(f, anchor, &open, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		items.WidthRange(280, heroWorkspaceMaxWidth)
		if !env.Data.WorkspacesKnown && len(env.Data.Workspaces) == 0 {
			items.Item(heroWorkspaceLoadingText).Disabled(true)
			return
		}
		selected := heroSelectedWorkspaceID(env)
		if env.Act.SelectWorkspace != nil {
			for _, workspace := range env.Data.Workspaces {
				workspace := workspace
				if items.Item(heroWorkspaceTitle(workspace)).
					Chosen(workspace.WorkspaceID == selected).
					Disabled(env.Data.DirectoryBusy).
					Clicked() && !env.Data.DirectoryBusy {
					env.Act.SelectWorkspace(workspace.WorkspaceID)
				}
			}
		}
		if env.Act.OpenDirectoryPicker != nil {
			if len(env.Data.Workspaces) > 0 {
				items.Separator()
			}
			if items.Item(heroAddWorkspaceText).Disabled(env.Data.DirectoryBusy).
				Clicked() && !env.Data.DirectoryBusy {
				env.Act.OpenDirectoryPicker()
			}
		}
	})
	p.workspaceMenuOpen = open
}

// workspaceOf 把 cwd 变成短名（首屏只放得下一个名字，放全路径会把标题挤掉）。
func workspaceOf(cwd string) string {
	c := strings.TrimRight(strings.TrimSpace(cwd), `/\`)
	if c == "" {
		return ""
	}
	base := path.Base(strings.ReplaceAll(c, `\`, "/"))
	if base == "." || base == "/" {
		return c
	}
	return base
}

// drawPreviewBadge 画官方的「预览版」胶囊徽标。
//
// 官方三色是 `state-business-tertiary` / `label-primary-bluish` / `interactive-bg-hover`；
// 这三个 token 不在 `pluginapi.Theme` 里（dscore 不在本任务的改动范围），
// 所以退到现有 theme 档位：深色下 BgLayer3 与官方 tertiary 同为 #292929，
// 描边 Hover 对应 interactive-bg-hover，文字 LabelPrimary 对应 primary-bluish。
// 形状（999 圆角、1px 上内边距、2px 上外边距、12/18 字号行高）与官方逐项一致。
func (p *Plugin) drawPreviewBadge(f ui.C) {
	t := p.theme
	// Viva 的 mono 文本按字体 cell 高测量，忽略 Text.LineHeight。用固定 20px
	// 容器表达官方 CSS：0.5 双边 + 1px 上 padding + 18px 内容行；
	// 等宽字形在 18px 内容区居中，不把 15px glyph 框误当成 line-height。
	p.heroBoxes.Badge = ui.Row(f).Height(20).
		PadSides(heroBadgePadTop, heroBadgePadX, 0, heroBadgePadX).
		MarginSides(heroBadgeMarginTop, 0, 0, 0).
		Radius(heroBadgeRadius).Border(heroBadgeBorder, t.Hover).
		Background(t.BgLayer3).Items(ui.Center).Children(func() {
		ui.Text(f, heroBadgeText).
			FontSize(t.Fonts.Caption).Weight(gfx.WeightMedium).Mono().
			Color(t.LabelPrimary)
	})
}
