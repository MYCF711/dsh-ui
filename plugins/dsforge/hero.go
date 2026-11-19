// hero.go —— 新会话行上的工作树勾选（官方 `WorktreeToggle`）。
//
// # 官方形状（逐条）
//
//	OFF:…/src/client/WorktreeToggle.tsx:121-165   光标：分支芯片 + checkbox
//	OFF:…/src/client/WorktreeToggle.module.css:3-40
//	  .root{inline-flex;gap:2px;align-items:center}
//	  .toggle,.base,.current{height:28px;padding:0 8px;gap:6px;radius:sm;
//	                        13px/20px;weight:500;color:label-secondary}
//	  .checkbox{width:14px;height:14px}
//	  .toggle[data-disabled]{color:label-dimmed}
//
// # 可见性与挡板（官方 `optionOf` 与 Tooltip，逐条）
//
//	WorktreeToggle.tsx:103   !state.worktree.editable              → 整块不画
//	WorktreeToggle.tsx:104-105 blocker === 'not-git'              → 整块不画
//	WorktreeToggle.tsx:106-107 checkout.state !== 'ready'         → 整块不画
//	WorktreeToggle.tsx:128-129 usable = blocker === null
//	                            disabled = held(creation!=null) || pending || (!usable && !requested)
//	WorktreeToggle.tsx:151   Tooltip label = blocker===null ? 'worktree.hint'
//	                                                       : `worktree.blocked.${blocker}`
//	WorktreeToggle.tsx:152-161 `<label data-disabled>` + `<input type=checkbox disabled>`
//
// **`not-git` 没有挡板文案**：官方 `worktree.blocked.*` 只有两句
// （`locales.ts:311-312`：`outside-writable-root` / `read-only`），因为 `not-git` 在
// `optionOf` 的 `:104-105` 就整块否掉了，永远走不到 `:151` 那句 Tooltip。
// 所以本仓也不给它编文案、更不画成"禁用 + 浮条"（见 `worktreeBlockerCopy`）。
//
// 这四样现在全部由 `forgePanel.follow` 的官方 `ForgePanelState` 供真值
// （`pluginapi.ForgeWorktree.PanelKnown/Editable/Blocker/Creation`）。
// 没有 follow 帧时（老宿主 / 测试）退回"checkout ready 才画"这条可证伪的弱判据，
// 不拿零值当"宿主说不能建"。
//
// # 官方有、本仓没有的一样（登记在账本行）
//
//  1. **基准分支下拉菜单**（官方 `.base` 是一次 `branches()` + 菜单）。本仓没有菜单
//     浮层可用，所以勾选始终按官方的默认语义发 `request(当前分支)`——官方 unchecked →
//     checked 走的就是 `request(option.current)`（`WorktreeToggle.tsx:134-137` 的
//     `change()` + `:158` 的 onChange）。
//     代价：选不到**别的**基准分支。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 WorktreeToggle.module.css 的几何（逐条抄，见文件头）。
const (
	// `.toggle,.base,.current{height:28px;padding:0 8px;gap:6px}`。
	worktreeChipHeight float32 = 28
	worktreeChipPadX   float32 = 8
	worktreeChipGap    float32 = 6
	// `.current` 里先画 `<IconBranchOutlineRegular size={14} />`（WorktreeToggle.tsx:146），
	// 图标本身是 `flex:none`（`.current svg`，WorktreeToggle.module.css:55-57）。
	worktreeChipGlyphSize float32 = 14
	// `.root{gap:2px}`。
	worktreeRootGap float32 = 2
	// `.checkbox{width:14px;height:14px}`。
	worktreeCheckboxSize float32 = 14
	// `.toggle,.current{font-size:13px;line-height:20px;font-weight:500}`。
	worktreeFont float32 = 13
	worktreeLine float32 = 20
	// `.current{max-width:200px}`。
	worktreeChipMaxW float32 = 200
	// `.checkbox{border-radius:2px}` 一档的圆角：官方用原生 checkbox，本仓自绘一个
	// 14px 方框（Viva 没有原生控件）。圆角取 theme 的 SM，与同一行的芯片同档。
	// `.check` 里的打勾图标按 14px 画（官方 checkbox 的原生勾是系统字形的近似，
	// 本仓用官方 `IconCheckOutlineRegular`，仍是官方素材）。
)

// 官方 WorktreeToggle 的中文文案（locales.ts:309-312，逐字）。
const (
	copyWorktreeLabel         = "新建工作树"
	copyWorktreeBase          = "基于 %s"
	copyWorktreeBaseHead      = "基于当前 HEAD"
	copyWorktreeCurrentBranch = "当前分支：%s"
	copyWorktreeDetached      = "未在分支上"
	// copyWorktreeHint 是 `worktree.hint`（locales.ts:310）：勾选框的默认浮条。
	copyWorktreeHint = "在独立的工作树中运行本会话：发送后基于所选分支新建分支，修改和提交不影响项目目录"
	// copyWorktreeBlocked 是两块**挡板**文案（locales.ts:311-312）。
	copyWorktreeBlockedOutsideRoot = "这个工作区的 Git 数据不在可写目录中（例如工作区本身就是工作树），暂不支持使用工作树"
	copyWorktreeBlockedReadOnly    = "只读模式下无法使用工作树"
)

// worktreeBlockerCopy 把官方 blocker 值翻成官方文案（`worktree.blocked.<blocker>`）。
//
// 官方是 `t(`worktree.blocked.${blocker}`)`（WorktreeToggle.tsx:151），键只有两条
// （locales.ts:311-312）；未知值返回空串 = 不显示浮条，而不是编一句原因。
//
// **`not-git` 故意没有文案**：它在 `optionOf` 的 `:104-105` 就整块不画，官方那两句
// 挡板文案里也就没有它（locales.ts:311-312 只有 outside-writable-root / read-only）。
func worktreeBlockerCopy(blocker string) string {
	switch blocker {
	case pluginapi.ForgeWorktreeBlockerOutsideWritableRoot:
		return copyWorktreeBlockedOutsideRoot
	case pluginapi.ForgeWorktreeBlockerReadOnly:
		return copyWorktreeBlockedReadOnly
	}
	return ""
}

// drawWorktreeToggle 画 `conversation.hero.options` 上的工作树勾选。
//
// 可见性逐条照官方 `optionOf`（WorktreeToggle.tsx:97-109）：
//
//	Forge 数据面在、且属于当前会话      → forgeView
//	有 follow 帧且 !editable            → 整块不画（官方 :103）
//	有 follow 帧且 blocker == not-git   → 整块不画（官方 :104-105）
//	checkout 读成功                     → Kind == ready（官方 :106-107）
//	壳接了动作面                        → Act.Forge != nil
//
// **这四条都是「整块不画」**，与下面「画出但禁用」是两件事：只有 blocker 落在
// `outside-writable-root` / `read-only` 这两句有文案的值上时，官方才把勾选框画出来、
// 加 `data-disabled` 并把浮条换成挡板原因（WorktreeToggle.tsx:128-129,151-152）。
//
// 没有 follow 帧（老宿主 / 测试）时不判前两条：`PanelKnown` 为零值，等于"宿主没
// 说不能建"，退回 checkout ready 这条弱判据。
func (p *Plugin) drawWorktreeToggle(f ui.C, env *pluginapi.Env) {
	view := forgeView(env)
	if view == nil || env.Act.Forge == nil {
		return
	}
	worktree := view.Worktree
	if worktree.PanelKnown {
		if !worktree.Editable {
			return
		}
		if worktree.Blocker == pluginapi.ForgeWorktreeBlockerNotGit {
			return
		}
	}
	if view.Checkout.Kind != pluginapi.ForgeCheckoutReady {
		return
	}
	t := p.themeOf(env)
	// 官方 :128-129：`usable = blocker === null`；
	// `disabled = held || pending !== null || (!usable && !requested)`。
	// held = 有创建在跑（官方 :126 `state.worktree.creation !== null`）。
	usable := worktree.Blocker == ""
	held := worktree.Creation != nil
	disabled := held || worktree.Pending || (!usable && !worktree.Requested)
	fg := t.LabelSecondary
	if disabled {
		fg = t.LabelDimmed
	}
	// 官方 :151：挡着时浮条换成那一句原因，否则是默认 hint。
	tip := copyWorktreeHint
	if !usable {
		if blocked := worktreeBlockerCopy(worktree.Blocker); blocked != "" {
			tip = blocked
		}
	}
	current := view.Checkout.Branch
	if view.Checkout.Detached {
		current = copyWorktreeDetached
	}
	// 已请求时芯片改说基准（官方 `worktree.base` / `worktree.baseHead`）。
	chipText := copyWorktreeCurrentBranch
	if current == "" {
		chipText = copyWorktreeDetached
	} else {
		chipText = sprintf(copyWorktreeCurrentBranch, current)
	}
	if worktree.Requested {
		if worktree.From == nil || *worktree.From == "" {
			chipText = copyWorktreeBaseHead
		} else {
			chipText = sprintf(copyWorktreeBase, *worktree.From)
		}
	}

	ui.Row(f).Gap(worktreeRootGap).Items(ui.Center).Shrink(0).Children(func() {
		// 分支芯片：官方这里是**按钮**（点开基准菜单）。本仓没有菜单浮层，
		// 所以画成同一几何的只读芯片，不画成按钮（画成按钮却点不开更糟）。
		ui.Row(f).Height(worktreeChipHeight).Shrink(0).MaxWidth(worktreeChipMaxW).
			PadSides(0, worktreeChipPadX, 0, worktreeChipPadX).Gap(worktreeChipGap).
			Items(ui.Center).
			Children(func() {
				// 官方 `.current` 里先画 `<IconBranchOutlineRegular size={14} />`、再画分支名
				// （WorktreeToggle.tsx:146-147），两者之间是 `.toggle,.base,.current{gap:6px}`
				// （WorktreeToggle.module.css:11-29）。墨色跟随这枚芯片自己的文字色：
				// 本仓它是只读近似件（见上一条注释），不是官方的按钮。
				if icon, ok := branchIcon(); ok {
					ui.Icon(f, icon).Size(worktreeChipGlyphSize, worktreeChipGlyphSize).
						Shrink(0).Color(t.LabelTertiary)
				}
				ui.Text(f, chipText).FontSize(worktreeFont).LineHeight(worktreeLine).
					Weight(gfx.WeightMedium).Color(t.LabelTertiary).Ellipsis()
			})

		// 勾选行：官方是 `<label>` 包 `<input type=checkbox>`，点标签同样生效。
		// 悬停底色只在**没被禁用**时出现（`.toggle:hover:not([data-disabled])`，
		// WorktreeToggle.module.css:31-36）；但身份（Hit）始终挂着——官方的
		// Tooltip 就包在这个 `<label>` 上，禁用态照样要能悬停看挡板原因。
		box := ui.Row(f).Height(worktreeChipHeight).Shrink(0).
			PadSides(0, worktreeChipPadX, 0, worktreeChipPadX).Gap(worktreeChipGap).
			Items(ui.Center).Radius(t.Radii.SM).
			Hit(env.Tree.ID(env.Feat, "forge-worktree-toggle"), copyWorktreeLabel)
		if !disabled {
			box = box.Hover(ui.Look{}.Background(t.Hover))
		}
		box = box.Tooltip(tip)
		box.Children(func() {
			p.drawCheckbox(f, env, worktree.Requested, disabled, fg)
			ui.Text(f, copyWorktreeLabel).FontSize(worktreeFont).LineHeight(worktreeLine).
				Weight(gfx.WeightMedium).Color(fg)
		})
		// 点一下 = 勾上（若没请求）或撤回（若已请求），与官方同一个 onChange。
		if !disabled && box.Clicked() {
			if worktree.Requested {
				env.Act.Forge.CancelWorktree()
			} else {
				// 官方 unchecked → checked 是 `request(option.current)`：
				// 基准就是当前分支。分离 HEAD 时官方传 null（基于 HEAD）。
				var from *string
				if !view.Checkout.Detached && view.Checkout.Branch != "" {
					branch := view.Checkout.Branch
					from = &branch
				}
				env.Act.Forge.RequestWorktree(from)
			}
			return
		}
	})
}

// drawCheckbox 画一枚 14px 方框（勾上时填品牌色 + 官方对勾图标）。
//
// Viva 没有原生 checkbox，官方那一枚是浏览器控件（`.checkbox{accent-color:brand-primary}`），
// 所以这里自绘的是**控件外观**、不是布局：位置/尺寸/所在行仍按官方 CSS。
func (p *Plugin) drawCheckbox(f ui.C, env *pluginapi.Env, checked, disabled bool, fg ui.Color) {
	t := p.themeOf(env)
	box := ui.Row(f).Width(worktreeCheckboxSize).Height(worktreeCheckboxSize).Shrink(0).
		Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM)
	if checked {
		box = box.Background(t.BrandPrimary)
	} else {
		border := t.BorderL3
		if disabled {
			border = t.BorderL1
		}
		box = box.Border(1, border)
	}
	box.Children(func() {
		if !checked {
			return
		}
		icon, ok := checkIcon()
		if !ok {
			// 素材编译不出来时**不画近似勾**：填色方块已经表示"选中"。
			return
		}
		ui.Icon(f, icon).Size(checkSize-2, checkSize-2).Color(t.ButtonPrimaryFg)
	})
}
