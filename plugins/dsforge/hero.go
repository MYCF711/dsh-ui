// hero.go —— 新会话行上的工作树勾选（官方 `WorktreeToggle`）。
//
// # 官方形状（逐条）
//
//	OFF:…/src/client/WorktreeToggle.tsx:105-165   光标：分支芯片 + checkbox
//	OFF:…/src/client/WorktreeToggle.module.css:3-40
//	  .root{inline-flex;gap:2px;align-items:center}
//	  .toggle,.base,.current{height:28px;padding:0 8px;gap:6px;radius:sm;
//	                        13px/20px;weight:500;color:label-secondary}
//	  .checkbox{width:14px;height:14px}
//	  .toggle[data-disabled]{color:label-dimmed}
//
// # 官方有、本仓没有的三样（都登记在账本行）
//
//  1. `worktree.editable` / `blocker`（not-git / outside-writable-root / read-only）没有读出口。
//     这里用**可证伪的替代判据**：`checkout.state == 'ready'` 才画（官方同样要求它）。
//     少了的那一层是"能读 git、但沙箱写不进去"的挡板。
//  2. **基准分支下拉菜单**（官方 `.base` 是一次 `branches()` + 菜单）。本仓没有菜单
//     浮层可用，所以勾选始终按官方的默认语义发 `request(当前分支)`——官方 unchecked →
//     checked 走的就是 `request(option.current)`（`WorktreeToggle.tsx:150-153`）。
//     代价：选不到**别的**基准分支。
//  3. `creation`（正在创建/停在哪一步）没有读出口，所以勾上之后只显示"已请求"，
//     不画"创建分支 / 检出文件"那两步（官方 WorktreeProgress，见 #64 行的 HIDE 说明）。
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

// 官方 WorktreeToggle 的中文文案（locales.ts:303-312，逐字）。
const (
	copyWorktreeLabel         = "新建工作树"
	copyWorktreeBase          = "基于 %s"
	copyWorktreeBaseHead      = "基于当前 HEAD"
	copyWorktreeCurrentBranch = "当前分支：%s"
	copyWorktreeDetached      = "未在分支上"
)

// drawWorktreeToggle 画 `conversation.hero.options` 上的工作树勾选。
//
// 可见性三层（官方同样三层，缺一层就整块不画）：
//
//	Forge 数据面在、且属于当前会话      → forgeView
//	checkout 读成功                    → Kind == ready
//	壳接了动作面                        → Act.Forge != nil
func (p *Plugin) drawWorktreeToggle(f ui.C, env *pluginapi.Env) {
	view := forgeView(env)
	if view == nil || env.Act.Forge == nil {
		return
	}
	if view.Checkout.Kind != pluginapi.ForgeCheckoutReady {
		return
	}
	t := p.themeOf(env)
	worktree := view.Worktree
	// 官方 `disabled = held || pending !== null || (!usable && !requested)` 的前两项：
	// creation 在跑（held）本仓读不到，只能判 pending。第三项（blocker 挡着）同样
	// 读不到，所以这里少了"被挡时禁止勾选"——登记为差异。
	disabled := worktree.Pending
	fg := t.LabelSecondary
	if disabled {
		fg = t.LabelDimmed
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
			PadSides(0, worktreeChipPadX, 0, worktreeChipPadX).Items(ui.Center).
			Children(func() {
				ui.Text(f, chipText).FontSize(worktreeFont).LineHeight(worktreeLine).
					Weight(gfx.WeightMedium).Color(t.LabelTertiary).Ellipsis()
			})

		// 勾选行：整行可点（官方是 `<label>` 包 `<input type=checkbox>`，点标签同样生效）。
		box := ui.Row(f).Height(worktreeChipHeight).Shrink(0).
			PadSides(0, worktreeChipPadX, 0, worktreeChipPadX).Gap(worktreeChipGap).
			Items(ui.Center).Radius(t.Radii.SM)
		if !disabled {
			box = box.Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "forge-worktree-toggle"), copyWorktreeLabel)
		}
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
