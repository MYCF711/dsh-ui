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
	// `.line{min-height:22px;line-height:22px}` / `.text{padding:0 16px 0 6px}`。
	changesLineH    float32 = 22
	changesTextPadR float32 = 16
	changesTextPadL float32 = 6
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
	session := diff.SessionID
	ui.Column(f).FillWidth().FillHeight().Background(t.BgBase).Children(func() {
		p.drawChangesHeader(f, env, diff)
		if p.open.menuOpen(session) {
			p.drawScopeMenu(f, env, diff)
		}
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
	})
}

// drawScopeMenu 画官方的范围菜单（`.menuRow` / `.menuList`）。本仓把它画成头部下方的
// 行内一列（没有浮层容器）——**位置**差异登记在账本行；条目与文案是官方的。
func (p *Plugin) drawScopeMenu(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	t := p.themeOf(env)
	session := diff.SessionID
	ui.Column(f).FillWidth().PadSides(4, 6, 4, 6).Border(0.5, t.BorderL3).Children(func() {
		for _, scope := range []string{
			apiclient.ForgeScopeBranch, apiclient.ForgeScopeUncommitted,
			apiclient.ForgeScopeUnstaged, apiclient.ForgeScopeStaged,
		} {
			label := scopeLabel(scope)
			color := t.LabelPrimary
			if scope == diff.Scope {
				color = t.BrandPrimary
			}
			row := ui.Row(f).FillWidth().Height(changesSelectH).Items(ui.Center).
				PadSides(0, changesSelectPadR, 0, changesSelectPadL).Radius(t.Radii.SM).
				Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "forge-scope:"+scope), label)
			row.Children(func() {
				ui.Text(f, label).FontSize(changesFont).LineHeight(changesLine).Color(color)
			})
			if row.Clicked() {
				if scope != diff.Scope {
					env.Act.Forge.LoadScope(scope)
				}
				p.open.toggleMenu(session)
			}
		}
	})
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
			// 官方这里画的是刷新图标（`IconRefreshOutlineRegular`）。素材库里这一枚
			// 只在 `plugins/dsbrowser` 里有一份**包内**常量（不跨包导出），本插件不
			// 复制那六条 path（复制一份就等于多一处会漂的官方坐标）。画官方文案的
			// 提示符代替：`.tool` 仍是官方 28×28 的可点方框。差异登记在账本行。
			ui.Text(f, "↻").FontSize(changesCenteredFnt).Color(t.LabelSecondary)
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
	ui.Column(f).FillWidth().Border(changesFileBorder, t.BorderL4).Radius(t.Radii.MD).
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
					ui.Text(f, badge).FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
				}
				ui.Spacer(f).Grow(1)
				ui.Text(f, fileCountsText(file)).FontSize(changesFont).
					LineHeight(changesLine).Color(t.LabelTertiary).Shrink(0)
			})
			if head.Clicked() {
				p.open.toggleFile(session, file.Path)
			}
			state := p.open.viewedState(session, file.Path, file.Digest)
			mark := ui.Row(f).Height(changesSelectH).Shrink(0).
				PadSides(0, changesSelectPadR, 0, changesSelectPadL).Items(ui.Center).
				Radius(t.Radii.SM).Hover(ui.Look{}.Background(t.Hover)).
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
func (p *Plugin) drawHunks(f ui.C, env *pluginapi.Env, file apiclient.ForgeDiffFile) {
	t := p.themeOf(env)
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
		for _, line := range hunk.Lines {
			if drawn >= changesMaxLinesPerFile {
				return
			}
			p.drawDiffLine(f, t, line)
			drawn++
		}
		prevNewEnd = hunk.NewStart + hunk.NewLines
	}
}

// drawDiffLine 画一行 patch 形式的内容（首字符是 ` ` / `+` / `-` / `\`）。
//
// 官方给增删行铺 `--dsw-alias-file-diff-*-bg`；本仓主题**没有**这三档令牌
// （`pluginapi/theme.go` 里没有任何 diff 色），所以这里只染符号与文字
// （+ 绿、− 红、上下文次要色），不铺底。这条差异登记在账本行。
func (p *Plugin) drawDiffLine(f ui.C, t *pluginapi.Theme, line string) {
	sign := " "
	body := line
	color := t.LabelSecondary
	if line != "" {
		switch line[0] {
		case '+':
			sign, body, color = "+", line[1:], t.Success
		case '-':
			sign, body, color = "−", line[1:], t.Error
		case '\\':
			sign, body, color = " ", line, t.LabelTertiary
		case ' ':
			body = line[1:]
		}
	}
	ui.Row(f).FillWidth().Height(changesLineH).Children(func() {
		ui.Text(f, sign).FontSize(t.Fonts.Code).LineHeight(changesLineH).Color(color).Width(14)
		ui.Text(f, body).FontSize(t.Fonts.Code).LineHeight(changesLineH).Color(color).
			PadSides(0, changesTextPadR, 0, changesTextPadL).Ellipsis()
	})
}

// drawNoteLine 画一行说明（未变更行 / 二进制 / 太大）。
func (p *Plugin) drawNoteLine(f ui.C, t *pluginapi.Theme, text string) {
	ui.Row(f).FillWidth().Height(changesLineH).Children(func() {
		ui.Text(f, text).FontSize(t.Fonts.Caption).LineHeight(changesLineH).
			Color(t.LabelTertiary).PadSides(0, changesTextPadR, 0, changesTextPadL).Ellipsis()
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
