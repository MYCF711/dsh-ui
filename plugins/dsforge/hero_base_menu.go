// hero_base_menu.go —— 新会话行上的**基准分支菜单**（官方 `WorktreeToggle.tsx:41-95`
// 的内嵌 `BaseMenu`）。
//
// 账本 #64 ④ 记的「hero 的基准分支菜单没做」就是这一份：勾上「新建工作树」之后，
// 官方那枚 `.base` 芯片是**可点开的下拉菜单**，能换一个基准分支重发请求；本仓此前把它
// 画成只读芯片（`hero.go` 文件头旧注：「本仓没有菜单浮层可用」）。
//
// # 官方形状（逐条）
//
//	WorktreeToggle.tsx:140-141  requested && usable 时才把 `.current` 换成 `.base`
//	                            （否则仍是只读的 `.current` 芯片）
//	WorktreeToggle.tsx:75-93    `<Menu open={open} items={items} selectedId={base}
//	                            side="bottom" align="end" portal anchor={…}>`
//	WorktreeToggle.tsx:66      条目 0 是分组标题 `worktree.baseMenu`（「选择基准分支」）
//	WorktreeToggle.tsx:67-72   其后每个本地分支一行；当前分支那一行多一段
//	                            `.optionNote` = `worktree.currentBranch`（「当前分支」）
//	WorktreeToggle.tsx:52-64   show()：开着→关；否则先读 branches()，**读到才开**
//	                            （`if (result === undefined) return`：读失败整场不开），
//	                            读的过程中 `disabled`
//	WorktreeToggle.tsx:79-82   onSelect：关菜单，`id !== base` 才 `request(id)`
//
//	WorktreeToggle.module.css:10-29   `.toggle,.base,.current{height:28px;padding:0 8px;
//	                                  gap:6px;radius:sm;13px/20px;weight:500;
//	                                  color:label-secondary}`
//	WorktreeToggle.module.css:44-50   `.base{gap:4px;max-width:200px;padding-right:4px}`
//	WorktreeToggle.module.css:78-92   `.baseLabel{ellipsis}` / `.chevron{flex:none;
//	                                  color:label-tertiary}` / `.chevronOpen{rotate 180}`
//	WorktreeToggle.module.css:99-110  `.option{gap:8px;align-items:baseline}` /
//	                                  `.optionName{12px;font-family:code}` /
//	                                  `.optionNote{12px;label-tertiary}`
//	WorktreeToggle.module.css:31-35   `.base:hover:not(:disabled){interactive-bg-hover}`
//
// 面与行不在这里画：primitives `Menu` 的外观唯一实现在 `plugins/dscore/menu.go`
// （`PaintMenuPanel` / `PaintMenuLabel` / `PaintMenuItemRuns`），与范围菜单、终端 shell
// 菜单同一份读数。
//
// # 数据面
//
// 分支清单与"重读分支"早就在壳里：`pluginapi.ForgeView.Branches` / `BranchesKnown` /
// `BranchesErr`（`cmd/app/forge_wiring.go` 的 `forgeLoadBranches`）与
// `Act.Forge.LoadBranches`（官方 `branches()`）。这一份只补界面，**不动数据面**。
package dsforge

import (
	"math"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	// `.base{padding-right:4px}`（WorktreeToggle.module.css:44-50）；左边仍是 8。
	worktreeBasePadRight float32 = 4
	// `.base{gap:4px}`：标签与 chevron 之间。
	worktreeBaseGap float32 = 4
	// `<IconChevronDownOutlineRegular size={12}>`（WorktreeToggle.tsx:90）。
	worktreeChevronSize float32 = 12
	// `.optionName`/`.optionNote{font-size:12px}`（WorktreeToggle.module.css:99-110）。
	worktreeOptionFont float32 = 12
)

// 官方 WorktreeToggle 的中文文案（locales.ts:315-316，逐字）。
const (
	// copyWorktreeBaseMenu 是 `worktree.baseMenu`：菜单的分组标题。
	copyWorktreeBaseMenu = "选择基准分支"
	// copyWorktreeBranchTag 是 `worktree.currentBranch`：当前分支那一行的注记。
	//
	// 它不是 `worktree.currentBranchTip`（hero.go 的 `copyWorktreeCurrentBranch`，
	// 「当前分支：{branch}」）——那是 `.current` 芯片的浮条文案，两者别混。
	copyWorktreeBranchTag = "当前分支"
)

// baseMenuOpen 报告基准菜单是否展开（官方 `useState(false)`）。
func (s *changesOpen) baseMenuOpen(session string) bool { return s.heroBaseMenu[session] }

// setBaseMenuOpen 记下基准菜单的展开态。
func (s *changesOpen) setBaseMenuOpen(session string, open bool) { s.heroBaseMenu[session] = open }

// baseMenuWant 报告"点了、正在等分支清单"（官方 `show()` 里 `loading` 那一段）。
func (s *changesOpen) baseMenuWant(session string) bool { return s.heroBaseWant[session] }

// setBaseMenuWant 记下"等分支清单"这个过渡态。
func (s *changesOpen) setBaseMenuWant(session string, want bool) { s.heroBaseWant[session] = want }

// drawWorktreeBaseChip 画 `.base`：基准芯片 + 它下面的菜单（官方 `WorktreeToggle.tsx:41-95`）。
//
// **只在 `requested && usable` 时画**（官方 :140-141）；其余情形由 `hero.go` 画只读的
// `.current` 芯片。调用方已经判过这两条，这里不再判一遍。
//
// `held`（创建正在跑）时芯片禁用：官方 `disabled={loading || held}`（:88），
// 禁用只挡点击，**不改字色**——`.base` 没有 `[data-disabled]` 那档，只有
// `.base:disabled{cursor:progress}`（WorktreeToggle.module.css:70-72）。
func (p *Plugin) drawWorktreeBaseChip(f ui.C, env *pluginapi.Env, view *pluginapi.ForgeView,
	worktree pluginapi.ForgeWorktree, held bool) {
	session := view.SessionID
	// 上一次点击还在等分支清单：清单到了这一帧才把菜单打开（官方 show() 的 .then）。
	p.settleWorktreeBaseMenu(session, view)
	open := p.open.baseMenuOpen(session)
	loading := p.open.baseMenuWant(session)
	disabled := loading || held

	// 芯片文案用的是 `request.from` 本身：null 就说"基于当前 HEAD"（官方 :132 的
	// `base === null ? worktree.baseHead : worktree.base`）。这与菜单的 selectedId
	// 是两个值，见 worktreeBaseRef。
	label := copyWorktreeBaseHead
	if worktree.From != nil && *worktree.From != "" {
		label = sprintf(copyWorktreeBase, *worktree.From)
	}
	t := p.themeOf(env)
	box := ui.Row(f).Height(worktreeChipHeight).Shrink(0).MaxWidth(worktreeChipMaxW).
		PadSides(0, worktreeBasePadRight, 0, worktreeChipPadX).Gap(worktreeBaseGap).
		Items(ui.Center).Radius(t.Radii.SM).
		Hit(env.Tree.ID(env.Feat, "forge-worktree-base"), label)
	if !disabled {
		// `.base:hover:not(:disabled){background:var(--dsw-alias-interactive-bg-hover)}`
		// （WorktreeToggle.module.css:31-35）。
		box = box.Hover(ui.Look{}.Background(t.Hover))
	}
	box.Children(func() {
		// `.baseLabel{overflow:hidden;text-overflow:ellipsis}`（:78-84），芯片 `max-width:200px`。
		ui.Text(f, label).FontSize(worktreeFont).LineHeight(worktreeLine).
			Weight(gfx.WeightMedium).Color(t.LabelSecondary).Ellipsis()
		if icon, ok := chevronDownIcon(); ok {
			mark := ui.Icon(f, icon).Size(worktreeChevronSize, worktreeChevronSize).Color(t.LabelTertiary)
			if open {
				// `.chevronOpen{transform:rotate(180deg)}`（:86-88）：同一枚官方 chevron
				// 转半圈，不是另画一个上箭头。
				mark.IconTransform(gfx.IconTransform{Rotate: math.Pi})
			}
		}
	})
	if !disabled && box.Clicked() {
		p.toggleWorktreeBaseMenu(env, view)
	}
	p.paintWorktreeBaseMenu(f, env, view, worktree, box)
}

// settleWorktreeBaseMenu 处理"点了、正在等分支清单"的那一帧。
//
// 官方 `show()` 是 `branches().then(result => { if (result === undefined) return;
// setListed(result); setOpen(true) })`（WorktreeToggle.tsx:57-63）：所以
//   - 清单到了才开；
//   - 读失败（`BranchesErr` 非空）**整场不开**，也不报错、不编一行假数据。
func (p *Plugin) settleWorktreeBaseMenu(session string, view *pluginapi.ForgeView) {
	if !p.open.baseMenuWant(session) || !view.BranchesKnown {
		return
	}
	p.open.setBaseMenuWant(session, false)
	if view.BranchesErr == "" {
		p.open.setBaseMenuOpen(session, true)
	}
}

// toggleWorktreeBaseMenu 是官方 `show()`（WorktreeToggle.tsx:52-64）：开着 → 关；
// 没开 → 先 `LoadBranches()` 重读一遍，读到（或本来就有）才开。
//
// 官方每次打开都重读一次（`show()` 第一句就是 `branches()`），所以这里也每次都发，
// 不去猜清单有没有变。
func (p *Plugin) toggleWorktreeBaseMenu(env *pluginapi.Env, view *pluginapi.ForgeView) {
	session := view.SessionID
	if p.open.baseMenuOpen(session) {
		p.open.setBaseMenuOpen(session, false)
		return
	}
	if env.Act.Forge != nil && env.Act.Forge.LoadBranches != nil {
		env.Act.Forge.LoadBranches()
	}
	if view.BranchesKnown {
		p.open.setBaseMenuOpen(session, true)
		return
	}
	// 还没读到过清单：先记下"要开"，等清单到的第一帧再开（`settleWorktreeBaseMenu`）。
	p.open.setBaseMenuWant(session, true)
}

// paintWorktreeBaseMenu 画基准分支菜单本体。
//
// 条目顺序照官方：分组标题「选择基准分支」在前，其后按宿主给的顺序每个本地分支一行；
// `selectedId` 是当前的基准（`Chosen` 走原语的单选档，勾在同一枚官方 check 里）。
func (p *Plugin) paintWorktreeBaseMenu(f ui.C, env *pluginapi.Env, view *pluginapi.ForgeView,
	worktree pluginapi.ForgeWorktree, anchor ui.Box) {
	session := view.SessionID
	open := p.open.baseMenuOpen(session)
	t := p.themeOf(env)
	current := view.Checkout.Branch
	base := worktreeBaseRef(worktree, view)
	ui.Menu(f, anchor, &open, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		items.Unpainted().
			DrawPanel(func(canvas *gfx.Canvas, bounds gfx.Rect, _ gfx.Theme) {
				dscore.PaintMenuPanel(canvas, bounds, t)
			}).
			DrawItem(func(parts ui.MenuItemParts) {
				if parts.GroupLabel {
					dscore.PaintMenuLabel(parts, t)
					return
				}
				dscore.PaintMenuItemRuns(parts, t, worktreeBaseRuns(parts.Label, current, t))
			})
		items.Label(copyWorktreeBaseMenu)
		// Viva 只按**声明的标签**算菜单宽（`menuItemWidth` 量的是 `items.Item` 的那句话），
		// 不知道行里还有「当前分支」那一段。所以这里按官方自动宽度的那笔账自己抬下界，
		// 否则当前分支那一行的名字会被省略号截掉（`feat/bas…`）。
		items.WidthRange(baseMenuLeastWidth(env.Canvas, view, current, t), 0)
		for _, name := range view.Branches {
			chosen := base != nil && *base == name
			if items.Item(name).Chosen(chosen).Clicked() {
				p.chooseWorktreeBase(env, name, base)
			}
		}
	})
	// `MenuItems` 在半路把 `*open` 置 false（选完/点外部/Esc），这里写回本插件的界面态。
	p.open.setBaseMenuOpen(session, open)
}

// baseMenuLeastWidth 是让每一行都完整显示所需的菜单面宽度下界（官方是浏览器的自动宽度）。
//
// 每一行按 `worktreeBaseRuns` 的同一份段读数去量，再加上菜单面那一圈
// （`dscore.MenuSurfaceExtra`）；`canvas` 为空（没有绘制画布）时返回 0，交给 Viva 自己的
// 默认宽度——那是"量不出来"的降级，不是把行截断。
func baseMenuLeastWidth(canvas *gfx.Canvas, view *pluginapi.ForgeView,
	current string, t *pluginapi.Theme) float32 {
	if canvas == nil {
		return 0
	}
	least := float32(0)
	for _, name := range view.Branches {
		runs := worktreeBaseRuns(name, current, t)
		least = max(least, dscore.MenuRunsWidth(canvas, runs)+dscore.MenuSurfaceExtra)
	}
	return least
}

// worktreeBaseRuns 是一行菜单的文字分段（官方 `.option` 的两段）。
//
// 名字那段走等宽面（`.optionName{font-family:var(--ds-font-family-code);font-size:12px}`），
// 当前分支那一行再加一段 `.optionNote`（12px、label-tertiary）；其余行只有名字。
func worktreeBaseRuns(name, current string, t *pluginapi.Theme) []dscore.MenuRun {
	runs := []dscore.MenuRun{{Text: name, Size: worktreeOptionFont, Mono: true, Flex: true}}
	if current != "" && name == current {
		runs = append(runs, dscore.MenuRun{
			Text: copyWorktreeBranchTag, Size: worktreeOptionFont, Color: t.LabelTertiary,
		})
	}
	return runs
}

// chooseWorktreeBase 是官方 `onSelect`（WorktreeToggle.tsx:79-82）：先关菜单，
// 只有**选的不是当前基准**才 `request(id)`（`request` 会替换掉已打开的那个请求）。
//
// 与 `changes.go` 的 `chooseScope` 同一做法：Viva 的弹层条目身份由框架自己分配，
// 测试点不到行 ID，所以"选中之后发什么"留一条能直接调用的真路径。
func (p *Plugin) chooseWorktreeBase(env *pluginapi.Env, name string, base *string) {
	if env.Act.Forge == nil || env.Act.Forge.RequestWorktree == nil {
		return
	}
	if base != nil && *base == name {
		return
	}
	branch := name
	env.Act.Forge.RequestWorktree(&branch)
}

// worktreeBaseRef 是官方 `base={state.worktree.request?.from ?? option.current}`
// （WorktreeToggle.tsx:141）：给菜单定 selectedId 用。
//
// 与芯片文案不是同一个值：文案那边直接用 `request.from`（null = 基于当前 HEAD，
// :132），这里缺省回落到当前分支——官方 `onSelect` 也是拿这一份比 `id !== base`，
// 所以"选中当前分支"在基于 HEAD 时同样不重发请求。
func worktreeBaseRef(worktree pluginapi.ForgeWorktree, view *pluginapi.ForgeView) *string {
	if worktree.From != nil {
		return worktree.From
	}
	if view.Checkout.Branch == "" {
		return nil
	}
	branch := view.Checkout.Branch
	return &branch
}
