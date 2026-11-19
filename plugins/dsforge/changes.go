// changes.go —— 官方 Changes 视图的**本地那一半**（右栏 tab）。
//
// # 官方形状
//
//	OFF:…/src/client/changes-address.ts:22      tab 的 id / kind（本插件 key = 官方完整 id）
//	OFF:…/src/client/mount.ts:364-369           体 + 标题两个 keyed 槽
//	OFF:…/src/client/ChangesTab.tsx:252-…       本地来源（`source.kind === 'local'`）那条分支
//	OFF:…/src/client/ChangesTab.module.css:3-…  根/头/工具栏/文件卡/行
//	OFF:…/src/client/changes-store.ts:81-86     首次进入的默认选择：scope=branch、split、wrap
//
// 几何取到的都在下面常量里，逐条对得上 CSS；**没取到/做不到**的一律不做，登记在账本 #64 行：
//
//	文件树左栏（`.treePane` / `--forge-tree-width`）——本仓没有可拖宽的分栏视图
//	左右对比（`data-split`）与换行开关（`data-wrap`）——Viva 单元格宽度不可控
//	行内评论 / 评审队列（`ReviewComposer` 等）——走 CR 面（#64b）
//
// # 状态放哪
//
// 官方把每个 tab 的选择放 `@deepseek-ai/dsh-client-store` 的 per-tab 桶
// （`changes-store.ts:11-27`）。本仓没有那个 store，界面态留在插件里（内存，
// 不落盘）：范围菜单开合、文件卡展开、已读标记。**已读标记**按官方
// `viewed-files.ts` 的三态：`未读` / `已读` / `已变更`（存 digest，不是时间）。
package dsforge

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/ui"
)

// 官方 ChangesTab.module.css 的几何（数字逐条取自 CSS，注释给类名）。
const (
	// `.root{font-size:12px;line-height:18px}`。
	changesFont float32 = 12
	changesLine float32 = 18
	// `.header{min-height:44px;padding:0 8px 0 14px;gap:8px}`。
	changesHeaderH    float32 = 44
	changesHeaderPadR float32 = 8
	changesHeaderPadL float32 = 14
	changesHeaderGap  float32 = 8
	// `.select{height:28px;padding:0 6px 0 8px;gap:4px}`。
	changesSelectH    float32 = 28
	changesSelectPadR float32 = 6
	changesSelectPadL float32 = 8
	changesSelectGap  float32 = 4
	// `.tool{width:28px;height:28px}`。
	changesToolSize float32 = 28
	// `.files{--forge-files-inset:10px}`。
	changesFilesInset float32 = 10
	// `.file{border:.5px l4;radius:lg}` / `.fileHeader{min-height:36px;padding:0 4px 0 8px;gap:6px}`。
	changesFileBorder     float32 = 0.5
	changesFileHeaderH    float32 = 36
	changesFileHeaderPadR float32 = 4
	changesFileHeaderPadL float32 = 8
	changesFileHeaderGap  float32 = 6
	// `.viewed{height:24px;padding:0 8px 0 6px;border:.5px l2}`。
	changesViewedH    float32 = 24
	changesViewedPadR float32 = 8
	changesViewedPadL float32 = 6
	// `.badge{padding:0 6px;border-radius:4px;font-size:11px;line-height:18px}`。
	changesBadgePadX float32 = 6
	changesBadgeFont float32 = 11
	changesBadgeLine float32 = 18
	// `.tool svg{width:15px;height:15px}`。
	changesRefreshSize float32 = 15
	// `.line{grid-template-columns:number number 1.4em text}` / `.number{padding-right:6px}`；
	// DiffFile.tsx:195 的动态 number 宽是 `max(2,digits)ch + 12px`。
	changesNumberPadR  float32 = 6
	changesNumberExtra float32 = 12
	// 行号列的 `1ch`：等宽字体下 ch 约为字号的 0.5 倍（官方 CSS 用 `ch` 单位，
	// 本仓没有测量接口，按等宽字体的比例折算；字号取主题的 `Fonts.Code`）。
	changesNumberChEm float32 = 0.5
	changesSignEm     float32 = 1.4
	// `.line{min-height:22px;line-height:22px}` / `.text{padding:0 16px 0 6px}`。
	changesLineH    float32 = 22
	changesTextPadR float32 = 16
	changesTextPadL float32 = 6
	// `.gap,.fileNote{min-height:26px;padding:0 12px;background:forge-fill}`。
	changesNoteH    float32 = 26
	changesNotePadX float32 = 12
	// `.centered{padding:24px;gap:8px;font-size:13px}`。
	changesCenteredPad float32 = 24
	changesCenteredGap float32 = 8
	changesCenteredFnt float32 = 13
	// 渲染上限：宿主已按自己的上限截断，这里再兜一层——一帧画几万行会把右栏卡住，
	// 而本仓没有官方的行虚拟化（`.unreached` 占位行）。超出的行不画。
	changesMaxLinesPerFile = 2000
)

// 官方本地范围的文案（locales.ts:198-206，逐字）。
const (
	copyScopeBranch       = "分支"
	copyScopeUncommitted  = "未提交"
	copyScopeUnstaged     = "未暂存"
	copyScopeStaged       = "已暂存"
	copyDiffRefresh       = "刷新变更"
	copyDiffEmpty         = "没有变更"
	copyDiffError         = "无法读取变更"
	copyDiffOmitted       = "另有 %s 个未跟踪的文件没有列出"
	copyDiffBinary        = "二进制文件，无法显示变更"
	copyDiffFileTooLarge  = "文件太大，无法显示变更"
	copyDiffUnchanged     = "%s 行未变更"
	copyDiffNewFile       = "新文件"
	copyDiffDeleted       = "已删除"
	copyDiffRenamed       = "由 %s 重命名"
	copyDiffBadgeUncommit = "未提交"
	copyViewedUnviewed    = "未读"
	copyViewedViewed      = "已读"
	copyViewedChanged     = "已变更"
)

// 已读三态（官方 `viewed-files.ts` 的 `viewedStateOf`）。
const (
	viewedUnviewed = "unviewed"
	viewedDone     = "viewed"
	viewedChanged  = "changed"
)

// changesOpen 是本视图的界面态（全部内存；键是会话 id）。
type changesOpen struct {
	local    map[string]bool
	files    map[string]map[string]bool
	viewed   map[string]map[string]string
	selected map[string]string
	menu     map[string]bool
}

func newChangesOpen() changesOpen {
	return changesOpen{
		local:    map[string]bool{},
		files:    map[string]map[string]bool{},
		viewed:   map[string]map[string]string{},
		selected: map[string]string{},
		menu:     map[string]bool{},
	}
}

// localExpanded 报告本地卡的展开态（默认收起，与官方 `useState(false)` 一致）。
func (s *changesOpen) localExpanded(session string) bool { return s.local[session] }

// toggleLocal 翻转本地卡的展开态。
func (s *changesOpen) toggleLocal(session string) { s.local[session] = !s.local[session] }

// selectFile 记下"要看哪个文件"。
func (s *changesOpen) selectFile(session, path string) { s.selected[session] = path }

// selectedFile 读回"要看哪个文件"。
func (s *changesOpen) selectedFile(session string) string { return s.selected[session] }

// fileExpanded 报告某个文件卡的展开态。默认展开：官方的文件卡默认展开，只有
// "读者还没滚到"的文件才用 22px 占位行收起（本仓没有虚拟化，故全展开）。
func (s *changesOpen) fileExpanded(session, path string) bool {
	if byPath := s.files[session]; byPath != nil {
		if open, ok := byPath[path]; ok {
			return open
		}
	}
	return true
}

// toggleFile 翻转一个文件卡。
func (s *changesOpen) toggleFile(session, path string) {
	if s.files[session] == nil {
		s.files[session] = map[string]bool{}
	}
	s.files[session][path] = !s.fileExpanded(session, path)
}

// viewedState 是官方三态：digest 与存的相同 = 已读；不同 = 已变更；没存过 = 未读。
func (s *changesOpen) viewedState(session, path, digest string) string {
	marks := s.viewed[session]
	if marks == nil {
		return viewedUnviewed
	}
	marked, ok := marks[path]
	if !ok {
		return viewedUnviewed
	}
	if marked == digest {
		return viewedDone
	}
	return viewedChanged
}

// markViewed 记下（或撤销）一个文件的已读标记，存 digest 而不是时间。
func (s *changesOpen) markViewed(session, path, digest string, viewed bool) {
	if s.viewed[session] == nil {
		s.viewed[session] = map[string]string{}
	}
	if viewed {
		s.viewed[session][path] = digest
		return
	}
	delete(s.viewed[session], path)
}

// menuOpen 报告范围菜单是否展开。
func (s *changesOpen) menuOpen(session string) bool { return s.menu[session] }

// toggleMenu 翻转范围菜单。
func (s *changesOpen) toggleMenu(session string) { s.menu[session] = !s.menu[session] }

// drawChangesTitle 画 tab 芯片上的标题（官方 `changesTitle` → `diff.tab` = 变更）。
func (p *Plugin) drawChangesTitle(f ui.C, env *pluginapi.Env) {
	t := p.themeOf(env)
	ui.Row(f).Gap(4).Items(ui.Center).Children(func() {
		ui.Text(f, changesTabLabel).FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).
			Color(t.LabelPrimary)
	})
}

// drawChangesBody 画变更视图的本地那一半。
func (p *Plugin) drawChangesBody(f ui.C, env *pluginapi.Env) {
	diff := forgeDiffOf(env)
	if diff == nil || env.Act.Forge == nil {
		// 数据面没准备好 / 没有动作面：不画（画一个空壳会让人以为"这里没有改动"）。
		return
	}
	t := p.themeOf(env)
	ui.Column(f).FillWidth().FillHeight().Background(t.BgBase).Children(func() {
		p.drawChangesHeader(f, env, diff)
		p.drawChangesToolbar(f, env, diff)
		p.drawChangesNotices(f, env, diff)
		p.drawChangesFiles(f, env, diff)
	})
}

// drawChangesHeader 画 `.header`：HEAD → 基准（branch 范围）/ 范围名（其余范围）。
func (p *Plugin) drawChangesHeader(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	t := p.themeOf(env)
	session := diff.SessionID
	ui.Row(f).FillWidth().MinHeight(changesHeaderH).Gap(changesHeaderGap).Items(ui.Center).
		PadSides(0, changesHeaderPadR, 0, changesHeaderPadL).
		Border(0.5, t.BorderL3).Children(func() {
		ui.Text(f, copyDiffHeadRef).FontSize(changesFont).LineHeight(changesLine).Color(t.LabelSecondary)
		ui.Text(f, "→").FontSize(changesFont).LineHeight(changesLine).Color(t.LabelTertiary)
		base := diff.Base
		if diff.Scope != apiclient.ForgeScopeBranch {
			base = scopeLabel(diff.Scope)
		}
		if base == "" {
			base = copyDiffHeadRef
		}
		ui.Text(f, base).FontSize(changesFont).LineHeight(changesLine).
			Color(t.LabelSecondary).Shrink(1).Ellipsis()
		ui.Spacer(f).Grow(1)
		selectBox := ui.Row(f).Height(changesSelectH).Shrink(0).
			PadSides(0, changesSelectPadR, 0, changesSelectPadL).Gap(changesSelectGap).
			Items(ui.Center).Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover)).
			Hit(env.Tree.ID(env.Feat, "forge-scope-select"), scopeLabel(diff.Scope))
		selectBox.Children(func() {
			ui.Text(f, scopeLabel(diff.Scope)).FontSize(changesFont).LineHeight(changesLine).
				Color(t.LabelPrimary)
			if icon, ok := chevronDownIcon(); ok {
				ui.Icon(f, icon).Size(chevronSize, chevronSize).Color(t.LabelTertiary)
			}
		})
		if selectBox.Clicked() {
			p.open.toggleMenu(session)
		}
		// 菜单锚在这个芯片上：官方 `anchor` 就是 `.select` 这个按钮本身
		// （`OFF:lib/client.js:6474-6485`），所以两者必须在同一次声明里。
		p.drawScopeMenu(f, env, diff, selectBox)
	})
}

// drawScopeMenu 画官方的范围菜单：**锚在范围芯片下方的浮层**。
//
// 官方形状（`OFF:lib/client.js:6436-6479` 的 `ScopeMenu`）：`primitives.Menu`
// 带 `portal: true` / `align: "start"` / `dense` / `autoFocus` / `selectedId`，
// 锚点是那个 `.select` 按钮；条目顺序是 **未提交 → 未暂存 → 已暂存 → 分隔线 → 分支**
// （分支单独一组：它比的是整段分支范围，前三个比的是工作区/索引）。
//
// 本仓用 Viva 的 `ui.Menu`：同样是锚定浮层（`popup.BelowBox(anchor, 4, Start)`，
// `ui/menu.go:126-131`）、点外部或 Esc 关闭、键盘可走、选中项打勾
// （`MenuItem.Chosen` → `ui/menu.go:465-473` 的 checked）。为此**不再**把菜单画成
// 头部下方的一列——那会占布局、把下面内容整体下推。
//
// 仍差一档：官方菜单面用 `--dsw-elevation-prominent`，而 Viva 的浮层固定
// `gfx.ShadowMD`（`viva@v0.6.4/components/dropdown_menu.go:769-771`）。这一条是
// 渲染器令牌缺口，已登记在账本 #64；手绘面板能换回官方阴影但会失去浮层，两害相权
// 保留浮层（用户可见的是"菜单位置对不对"，不是阴影偏一档）。
func (p *Plugin) drawScopeMenu(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff, anchor ui.Box) {
	session := diff.SessionID
	open := p.open.menuOpen(session)
	ui.Menu(f, anchor, &open, func(items *ui.MenuItems) {
		if items == nil {
			return
		}
		for _, scope := range []string{
			apiclient.ForgeScopeUncommitted,
			apiclient.ForgeScopeUnstaged,
			apiclient.ForgeScopeStaged,
		} {
			if items.Item(scopeLabel(scope)).Chosen(scope == diff.Scope).Clicked() {
				p.chooseScope(env, scope)
			}
		}
		items.Separator()
		if items.Item(scopeLabel(apiclient.ForgeScopeBranch)).
			Chosen(diff.Scope == apiclient.ForgeScopeBranch).Clicked() {
			p.chooseScope(env, apiclient.ForgeScopeBranch)
		}
	})
	p.open.menu[session] = open
}

// chooseScope 是选中一个范围之后的动作（官方 `onSelect={(id) => { setOpen(false); choose(id) }}`，
// `OFF:lib/client.js:6477-6480`）。
//
// 单独一个方法是**测试接缝**：Viva 的弹层条目由框架自己持有身份（`MenuItems.Item`
// 内部按 scope 分配），仓里的测试点不到它；所以"选中之后发什么"要有一条能直接
// 调用的真路径（`plugins/dscreationreco` 的 `chooseLocation` 同一做法）。
func (p *Plugin) chooseScope(env *pluginapi.Env, scope string) {
	if env.Act.Forge == nil || env.Act.Forge.LoadScope == nil {
		return
	}
	env.Act.Forge.LoadScope(scope)
}

// drawChangesToolbar 画 `.toolbar`：总增删 + 刷新。
func (p *Plugin) drawChangesToolbar(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	t := p.themeOf(env)
	added, deleted := 0, 0
	for _, file := range diff.Files {
		added += file.Additions
		deleted += file.Deletions
	}
	ui.Row(f).FillWidth().Items(ui.Center).Gap(6).
		PadSides(4, changesHeaderPadR, 4, changesHeaderPadL).
		Border(0.5, t.BorderL3).Children(func() {
		ui.Text(f, "+"+itoa(added)).FontSize(changesFont).LineHeight(changesLine).Color(t.Success)
		ui.Text(f, "−"+itoa(deleted)).FontSize(changesFont).LineHeight(changesLine).Color(t.Error)
		ui.Spacer(f).Grow(1)
		tool := ui.Row(f).Width(changesToolSize).Height(changesToolSize).Shrink(0).
			Items(ui.Center).Justify(ui.Center).Radius(t.Radii.SM).
			Hover(ui.Look{}.Background(t.Hover)).
			Hit(env.Tree.ID(env.Feat, "forge-diff-refresh"), copyDiffRefresh)
		tool.Children(func() {
			// RefreshGlyph.tsx:32 是 IconRefreshOutlineRegular；复用 dsassets 已登记的
			// 官方 sprite 路径（market.go:14-20），不复制坐标。CSS `.tool svg` 是 15×15。
			if icon, ok := dsassets.MarketRefreshOutline(t.LabelSecondary); ok {
				ui.Icon(f, icon).Size(changesRefreshSize, changesRefreshSize).Color(t.LabelSecondary)
			}
		})
		if tool.Clicked() {
			env.Act.Forge.Refresh()
		}
	})
}

// drawChangesNotices 画 `.notice`：被略掉的未跟踪文件数。
func (p *Plugin) drawChangesNotices(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	if diff.Omitted <= 0 {
		return
	}
	t := p.themeOf(env)
	ui.Row(f).FillWidth().PadSides(4, changesHeaderPadL, 4, changesHeaderPadL).Children(func() {
		ui.Text(f, sprintf(copyDiffOmitted, itoa(diff.Omitted))).
			FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).Color(t.LabelTertiary).
			Shrink(1).Ellipsis()
	})
}

// drawChangesFiles 画 `.files`：失败 / 空 / 文件卡列表。
func (p *Plugin) drawChangesFiles(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	t := p.themeOf(env)
	if diff.Err != "" {
		p.drawCentered(f, t, copyDiffError, diff.Err, t.Error)
		return
	}
	if len(diff.Files) == 0 {
		if diff.Loading {
			// 官方没有单独的"读取中"文案（它是骨架行）；本仓不造文案，空着不画。
			return
		}
		p.drawCentered(f, t, copyDiffEmpty, "", t.LabelSecondary)
		return
	}
	files := diff.Files
	session := diff.SessionID
	ui.Scroll(f).FillWidth().Grow(1).Shrink(1).MinHeight(0).Children(func() {
		ui.Column(f).FillWidth().Pad(changesFilesInset).Gap(12).Children(func() {
			for _, file := range files {
				p.drawFileCard(f, env, session, file)
			}
		})
	})
}

// drawCentered 画官方 `.centered`（居中标题 + 可选原因）。
func (p *Plugin) drawCentered(f ui.C, t *pluginapi.Theme, title, detail string, color ui.Color) {
	ui.Row(f).FillWidth().Grow(1).Items(ui.Center).Justify(ui.Center).
		Pad(changesCenteredPad).Children(func() {
		ui.Column(f).Items(ui.Center).Gap(changesCenteredGap).Children(func() {
			ui.Text(f, title).FontSize(changesCenteredFnt).Color(color)
			if detail != "" {
				ui.Text(f, detail).FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).
					Color(t.LabelTertiary)
			}
		})
	})
}

// drawFileCard 画一个文件卡（`.file` + `.fileHeader` + 行）。
func (p *Plugin) drawFileCard(f ui.C, env *pluginapi.Env, session string, file apiclient.ForgeDiffFile) {
	t := p.themeOf(env)
	expanded := p.open.fileExpanded(session, file.Path)
	ui.Column(f).FillWidth().Border(changesFileBorder, t.BorderL4).Radius(t.Radii.LG).
		Background(t.BgBase).Children(func() {
		ui.Row(f).FillWidth().MinHeight(changesFileHeaderH).Gap(changesFileHeaderGap).
			Items(ui.Center).PadSides(0, changesFileHeaderPadR, 0, changesFileHeaderPadL).
			Border(0.5, t.BorderL3).Children(func() {
			head := ui.Row(f).Grow(1).Shrink(1).Gap(changesFileHeaderGap).Items(ui.Center).
				Hit(env.Tree.ID(env.Feat, "forge-file:"+file.Path), file.Path)
			head.Children(func() {
				ui.Text(f, file.Path).FontSize(changesFont).LineHeight(changesLine).
					Color(t.LabelPrimary).Shrink(1).Ellipsis()
				if badge := fileBadge(file); badge != "" {
					ui.Text(f, badge).FontSize(changesBadgeFont).LineHeight(changesBadgeLine).
						PadSides(0, changesBadgePadX, 0, changesBadgePadX).
						Radius(t.Radii.XS).Background(t.CardFill).Color(t.LabelSecondary).
						Shrink(1).Ellipsis()
				}
				ui.Spacer(f).Grow(1)
				// 官方 Counts 是两个独立 span（DiffFile.tsx:123-126），不是一串第三级色文字。
				ui.Text(f, "+"+itoa(file.Additions)).FontSize(changesFont).LineHeight(changesLine).
					Mono().Color(t.Success).Shrink(0)
				ui.Text(f, "−"+itoa(file.Deletions)).FontSize(changesFont).LineHeight(changesLine).
					Mono().Color(t.Error).Shrink(0)
			})
			if head.Clicked() {
				p.open.toggleFile(session, file.Path)
			}
			state := p.open.viewedState(session, file.Path, file.Digest)
			mark := ui.Row(f).Height(changesViewedH).Shrink(0).Gap(changesFileHeaderGap).
				PadSides(0, changesViewedPadR, 0, changesViewedPadL).Items(ui.Center).
				Border(changesFileBorder, t.BorderL2).Radius(t.Radii.SM).
				Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "forge-viewed:"+file.Path), viewedLabel(state))
			mark.Children(func() {
				if icon, ok := checkIcon(); ok && state == viewedDone {
					ui.Icon(f, icon).Size(checkSize, checkSize).Color(t.Success)
				}
				ui.Text(f, viewedLabel(state)).FontSize(t.Fonts.Tiny).
					Color(viewedColor(t, state))
			})
			if mark.Clicked() {
				p.open.markViewed(session, file.Path, file.Digest, state != viewedDone)
			}
		})
		if !expanded {
			return
		}
		ui.Column(f).FillWidth().Children(func() {
			if file.Binary {
				p.drawNoteLine(f, t, copyDiffBinary)
				return
			}
			if file.Oversized {
				p.drawNoteLine(f, t, copyDiffFileTooLarge)
				return
			}
			p.drawHunks(f, env, file)
		})
	})
}

// drawHunks 画一个文件的全部 hunk（含 hunk 头与"未变更行"折叠提示）。
//
// 每行是官方的四轨 `.line`：旧行号 · 新行号 · 符号 · 正文
// （`OFF:src/client/ChangesTab.module.css:604`，单侧文件 :915/:916）。
// 行号由 hunk 坐标现算，与官方 `hunkRows()` 同一口径
// （`OFF:src/client/diff-rows.ts:53-68`）：上下文两边都数、删行只数旧侧、
// 加行只数新侧、`\ No newline at end of file` 不是一行。
func (p *Plugin) drawHunks(f ui.C, env *pluginapi.Env, file apiclient.ForgeDiffFile) {
	t := p.themeOf(env)
	// 行号列宽是**整份 diff** 共用的（官方 `maxDiffLineNo` 一次算出
	// `--forge-number-width`，见 changesDiffLineNo）。这里按本文件算是个近似：
	// 官方取的是全 diff 的最大行号，而面板里每张文件卡各自渲染。
	numbers := changesNumberW(file, t.Fonts.Code)
	singleSide := changesSingleSide(file)
	drawn := 0
	prevNewEnd := 0
	for _, hunk := range file.Hunks {
		if drawn >= changesMaxLinesPerFile {
			return
		}
		if gap := hunk.NewStart - prevNewEnd; gap > 1 {
			p.drawNoteLine(f, t, sprintf(copyDiffUnchanged, itoa(gap-1)))
			drawn++
		}
		ui.Row(f).FillWidth().Height(changesLineH).Children(func() {
			ui.Text(f, hunkHeader(hunk)).FontSize(t.Fonts.Code).LineHeight(changesLineH).
				Color(t.CodeComment).PadSides(0, changesTextPadR, 0, changesTextPadL)
		})
		drawn++
		oldNo, newNo := hunk.OldStart, hunk.NewStart
		for _, line := range hunk.Lines {
			if drawn >= changesMaxLinesPerFile {
				return
			}
			row := changesDiffRowOf(line, oldNo, newNo)
			if row.old != nil {
				oldNo++
			}
			if row.new != nil {
				newNo++
			}
			p.drawDiffRow(f, t, numbers, singleSide, row)
			drawn++
		}
		prevNewEnd = hunk.NewStart + hunk.NewLines
	}
}

// changesRow 是画出来的一行：两侧行号（各有/皆无）加正文。对应官方
// `DiffRow`（`OFF:src/client/diff-rows.ts:9-13`）。
type changesRow struct {
	old  *int
	new  *int
	sign string
	text string
}

// changesDiffRowOf 把一行 patch 文本连同当前两侧行号折成 changesRow。
// 行号引用的推进由调用方做（官方 `oldNo++` / `newNo++` 也在 `hunkRows()` 里）。
func changesDiffRowOf(line string, oldNo, newNo int) changesRow {
	sign := " "
	text := line
	old, next := &oldNo, &newNo
	if line != "" {
		switch line[0] {
		case '+':
			sign, text, old = "+", line[1:], nil
		case '-':
			sign, text, next = "−", line[1:], nil
		case '\\':
			// `\ No newline at end of file` 不是一行：两侧都不给行号。
			sign, text, old, next = " ", line, nil, nil
		case ' ':
			text = line[1:]
		}
	}
	return changesRow{old: old, new: next, sign: sign, text: text}
}

// changesSingleSide 是官方 `singleSide`（`OFF:src/client/DiffFile.tsx:190`）：
// 新增文件只有新侧、删除文件只有旧侧，其余两侧都画。
func changesSingleSide(file apiclient.ForgeDiffFile) string {
	switch file.Status {
	case apiclient.ForgeFileAdded:
		return "R"
	case apiclient.ForgeFileDeleted:
		return "L"
	}
	return ""
}

// changesNumberW 是官方 `--forge-number-width`（`OFF:src/client/DiffFile.tsx:194-195`）：
// `calc(max(2, 最大行号的位数)ch + 12px)`。ch 折算见 changesNumberChEm；
// `code` 是主题的等宽字号（`.line` 继承代码字体）。数字列右侧 `padding-right:6px`
// （`.number{padding-right:6px}`，ChangesTab.module.css:648）另算，不在这一格里。
func changesNumberW(file apiclient.ForgeDiffFile, code float32) float32 {
	digits := 2
	for _, hunk := range file.Hunks {
		if w := changesDigitsOf(hunk.OldStart + hunk.OldLines - 1); w > digits {
			digits = w
		}
		if w := changesDigitsOf(hunk.NewStart + hunk.NewLines - 1); w > digits {
			digits = w
		}
	}
	return float32(digits)*code*changesNumberChEm + changesNumberExtra
}

// changesDigitsOf 数一个非负整数的十进制位数（0 也算 1 位）。
func changesDigitsOf(n int) int {
	if n < 0 {
		n = -n
	}
	digits := 1
	for n >= 10 {
		n /= 10
		digits++
	}
	return digits
}

// drawDiffRow 画官方的四轨 `.line`：旧行号 · 新行号 · 符号 · 正文。
//
// 配色逐条取官方 `ChangesTab.module.css`：
//
//	.add { background: --dsw-alias-file-diff-added-bg }          :675-679
//	.del { background: --dsw-alias-file-diff-deleted-bg }        :681-685
//	.add .number,.del .number { background: *-gutter; color: *-marker }  :682-687
//	.add .sign,.del .sign { color: *-marker }                    :690-692
//	.context .text { color: --dsw-alias-label-secondary }        :694-696
//
// 六档令牌在 `pluginapi.Theme.FileDiff*`（值来自 design-platform.css 亮 281-286 / 暗 433-438）。
func (p *Plugin) drawDiffRow(f ui.C, t *pluginapi.Theme, numberW float32, singleSide string, row changesRow) {
	textColor := t.LabelSecondary
	numberColor := t.LabelTertiary
	background := ui.Color{} // 零值 = 不铺底（上下文行官方也只有文字色）
	isAdd, isDel := row.sign == "+", row.sign == "−"
	switch {
	case isAdd:
		textColor, numberColor = t.FileDiffAddedMarker, t.FileDiffAddedMarker
		background = t.FileDiffAddedBg
	case isDel:
		textColor, numberColor = t.FileDiffDeletedMarker, t.FileDiffDeletedMarker
		background = t.FileDiffDeletedBg
	case row.old == nil && row.new == nil:
		textColor = t.LabelTertiary // `\ No newline at end of file`
	}
	gutter := ui.Color{}
	switch {
	case isAdd:
		gutter = t.FileDiffAddedGutter
	case isDel:
		gutter = t.FileDiffDeletedGutter
	}
	line := ui.Row(f).FillWidth().Height(changesLineH)
	if isAdd || isDel {
		line = line.Background(background)
	}
	line.Children(func() {
		// 单侧文件的列序：官方的 `.code[data-single-side]` 收掉**另一侧**那一轨
		// （`ChangesTab.module.css:915`），剩下的仍是"旧 · 新 · 符号 · 正文"，
		// 所以这里只判"画哪两轨里的哪一轨"，不换顺序。
		if singleSide != "R" {
			p.drawLineNo(f, t, numberW, row.old, numberColor, gutter)
		}
		if singleSide != "L" {
			p.drawLineNo(f, t, numberW, row.new, numberColor, gutter)
		}
		ui.Text(f, row.sign).FontSize(t.Fonts.Code).LineHeight(changesLineH).Color(numberColor).
			Width(changesSignEm * t.Fonts.Code)
		ui.Text(f, row.text).FontSize(t.Fonts.Code).LineHeight(changesLineH).Color(textColor).
			PadSides(0, changesTextPadR, 0, changesTextPadL).Ellipsis()
	})
}

// drawLineNo 画一格行号（`.number{padding-right:6px;color:label-tertiary;text-align:right}`，
// `OFF:src/client/ChangesTab.module.css:648-655`）。
//
// 这一侧没有行号就画同样宽的空格：官方也是空 span（`DiffFile.tsx:403`），
// 这样正文列不会因为行号有无而左右跳。右对齐靠 `Justify(ui.End)`（Viva 的内容对齐）。
// 增删行的号码格再铺一层 gutter 底（`.add .number{background:*-gutter}`），
// 而 gutter 是**零值**时（上下文行）不铺。
func (p *Plugin) drawLineNo(f ui.C, t *pluginapi.Theme, width float32, no *int, color, gutter ui.Color) {
	label := ""
	if no == nil {
		color = t.LabelTertiary
	} else {
		label = itoa(*no)
	}
	cell := ui.Row(f).Width(width).Height(changesLineH).PadRight(changesNumberPadR).Justify(ui.End)
	if gutter != (ui.Color{}) {
		cell = cell.Background(gutter)
	}
	cell.Children(func() {
		ui.Text(f, label).FontSize(t.Fonts.Code).LineHeight(changesLineH).Color(color)
	})
}

// drawNoteLine 画一行说明（未变更行 / 二进制 / 太大）。
func (p *Plugin) drawNoteLine(f ui.C, t *pluginapi.Theme, text string) {
	ui.Row(f).FillWidth().MinHeight(changesNoteH).Background(t.CardFill).Children(func() {
		ui.Text(f, text).FontSize(changesFont).LineHeight(changesLine).
			Color(t.LabelTertiary).PadSides(0, changesNotePadX, 0, changesNotePadX).Ellipsis()
	})
}

// hunkHeader 造官方的 hunk 头（`@@ -a,b +c,d @@`）。
func hunkHeader(hunk apiclient.ForgeHunk) string {
	return "@@ -" + itoa(hunk.OldStart) + "," + itoa(hunk.OldLines) +
		" +" + itoa(hunk.NewStart) + "," + itoa(hunk.NewLines) + " @@"
}

// fileBadge 是官方的状态标签（`.badge`，文案取官方 locales 逐字）。
func fileBadge(file apiclient.ForgeDiffFile) string {
	switch file.Status {
	case apiclient.ForgeFileAdded:
		return copyDiffNewFile
	case apiclient.ForgeFileDeleted:
		return copyDiffDeleted
	case apiclient.ForgeFileRenamed:
		// 官方是 `由 {from} 重命名`。没有 previousPath 就不画标签：
		// 编一个"重命名"而说不出从哪儿来，正是"少一个字段就多一句假话"。
		if file.PreviousPath != nil && *file.PreviousPath != "" {
			return sprintf(copyDiffRenamed, *file.PreviousPath)
		}
		return ""
	}
	if file.Untracked {
		return copyDiffBadgeUncommit
	}
	return ""
}

// scopeLabel 是官方 `diff.scope.*`。
func scopeLabel(scope string) string {
	switch scope {
	case apiclient.ForgeScopeBranch:
		return copyScopeBranch
	case apiclient.ForgeScopeUncommitted:
		return copyScopeUncommitted
	case apiclient.ForgeScopeUnstaged:
		return copyScopeUnstaged
	case apiclient.ForgeScopeStaged:
		return copyScopeStaged
	}
	return scope
}

// viewedLabel 是已读三态的中文。
func viewedLabel(state string) string {
	switch state {
	case viewedDone:
		return copyViewedViewed
	case viewedChanged:
		return copyViewedChanged
	}
	return copyViewedUnviewed
}

// viewedColor 是已读三态的颜色（只有"已读"是肯定色）。
func viewedColor(t *pluginapi.Theme, state string) ui.Color {
	switch state {
	case viewedDone:
		return t.Success
	case viewedChanged:
		return t.Warn
	}
	return t.LabelTertiary
}
