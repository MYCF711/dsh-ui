// changes_tree.go —— 官方 `FileTreePane` 的文件树左栏（GUI 账本 #64 ②）。
//
// # 官方规格（逐条 file:line，rc.2 随包 src）
//
//	OFF:…/src/client/FileTreePane.tsx:9-11     MIN_WIDTH=200 / MAX_WIDTH=600 / DIFF_MIN_WIDTH=320
//	OFF:…/src/client/FileTreePane.tsx:29       `useState(240)` 初始宽度
//	OFF:…/src/client/FileTreePane.tsx:38       maximum = max(200,min(600,available-320))
//	OFF:…/src/client/FileTreePane.tsx:45-48    shown=min(width,maximum)，拖动钳在 [MIN,maximum]
//	OFF:…/src/client/FileTreePane.tsx:49       direction：左侧树 +1（向右拖变宽）
//	OFF:…/src/client/FileTreePane.tsx:79       `data-position` + `hidden` + 内联 width（见下）
//	OFF:…/src/client/FileTreePane.module.css:2 `.pane{display:none;...border-inline-start:.5px l3}`
//	OFF:…/src/client/FileTreePane.module.css:4 `[data-position='left']{order:-1;border-inline-end:.5px l3}`
//	OFF:…/src/client/FileTreePane.module.css:6 `.resize{inset-inline-start:-4px;width:8px;cursor:col-resize}`
//	OFF:…/src/client/ChangesTab.module.css:356 `@container forge-changes (min-width:760px){.treePane:not([hidden]){display:block}}`
//	OFF:…/src/client/ChangesTab.module.css:4   `.root{container:forge-changes / inline-size}`
//	OFF:…/src/client/ChangedFiles.tsx:46-54   行是 button，目录点击 toggle、文件点击 jump
//	OFF:…/src/client/ChangedFiles.tsx:47      `paddingInlineStart = 8 + depth*14`
//	OFF:…/src/client/ChangedFiles.tsx:55-58   目录有 12px chevron（收起转 -90°）+ 14px folder/file 图标
//	OFF:…/src/client/ChangedFiles.module.css:2 `.root{font-size:12px;color:label-primary}`
//	OFF:…/src/client/ChangedFiles.module.css:4 `.rows{overflow:auto;padding:0 4px 6px}`
//	OFF:…/src/client/ChangedFiles.module.css:5 `.row{display:flex;gap:6px;min-height:30px;padding:4px 6px;radius:sm}`
//	OFF:…/src/client/ChangedFiles.module.css:10 `.selected{background:--dsw-alias-interactive-bg-hover}`
//	OFF:…/src/client/file-tree.ts:17-32       目录优先、单目录链压缩、名字排序
//	OFF:…/src/client/file-tree.ts:61-70       filesInTreeOrder 是深度优先
//
// # 「容器查询」怎么落地
//
// 官方 `@container (min-width:760px)` 在 immediate mode 下**不需要容器查询**：
// Viva 的组件从父级拿 bounds（`components` 层全部如此），所以这里等价于
// 「读传入的宽度分支」——`changesTreeVisible(width)` 就是那条判据：
//
//   - 首帧还没量过：用壳算好的 `env.Geometry.RightWidth`（右栏宽度）；
//   - 之后每帧用上一帧 `root.Rect().Width`（doc 注释：AfterDraw 拿不到尺寸接口，
//     要在 Draw 阶段量 `Box.Rect()`）——比 Geometry 更接近真实可用宽度。
//
// 这**不是**容器查询的替代品，而是同一判据在 immediate mode 下的自然写法。
//
// # 可拖宽分栏用 Viva 的 `ui.Splitter`
//
// 官方 `.resize` 是一个 8px 的手柄（拖动时只改前面的树宽，窗口 resize 时把
// maximum 重算）。Viva `ui.Splitter(c, state *SplitterState)`（v0.6.5 的
// `ui/elements.go:257`，状态 `ui/splitter.go:18`）正好是「第一个 pane 由
// state.Pos 定宽、第二个 Grow(1)、拖动/键盘都被框架接管」：
//
//	OFF:viva@v0.6.5/ui/splitter.go:14-17  Pos 是 splitter 前那个 pane 的宽度
//	OFF:viva@v0.6.5/ui/splitter.go:86-100 声明顺序：第一个 pane → Splitter → 第二个 pane(Grow 1)
//	OFF:viva@v0.6.5/ui/splitter.go:115-122 Mins(first, second) 是两个 pane 的最小宽度
//	OFF:viva@v0.6.5/ui/splitter.go:136-154 布局后 hi = span - 8 - minSecond（即 maximum 的来源）
//	OFF:viva@v0.6.5/ui/splitter.go:221-238 按下捕获指针、移动改 Pos、松开结束
//
// 官方的 maximum 还要被 MAX_WIDTH=600 夹住；Viva 的 Splitter 只有「最小」，没有
// 「最大」，所以 **maximum 通过把第二个 pane 的最小宽度反算成 minSecond** 实现：
//
//	minSecond = available - 8 - max(200, min(600, available-320))
//
// 于是 Viva 的 hi = span - 8 - minSecond = max(200,min(600,available-320))，
// 与官方 `FileTreePane.tsx:38` 同一个值。
package dsforge

import (
	"math"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 FileTreePane.tsx:9-11,29 的三条宽度钳与初始宽度。
const (
	changesTreeMinWidth     float32 = 200
	changesTreeMaxWidth     float32 = 600
	changesDiffMinWidth     float32 = 320
	changesTreeDefaultWidth float32 = 240
	// 官方 ChangesTab.module.css:356 的 `@container (min-width:760px)`。
	changesTreeBreakpoint float32 = 760
	// Viva Splitter 的手柄厚度（`ui/splitter.go:8` 的 splitterThickness = 8）。
	changesTreeSplitterThickness float32 = 8
)

// ChangedFiles.module.css 的几何（数字逐条取自 CSS）。
const (
	changesTreeRowH     float32 = 30 // `.row{min-height:30px}`
	changesTreePadY     float32 = 4  // `.row{padding:4px 6px}`
	changesTreePadX     float32 = 6
	changesTreeGap      float32 = 6 // `.row{gap:6px}`
	changesTreeIndent   float32 = 14
	changesTreeIndent0  float32 = 8
	changesTreeChevron  float32 = 12 // `.chevron{flex:0 0 12px}`
	changesTreeIconSize float32 = 14
	changesTreeRowsPadR float32 = 4 // `.rows{padding:0 4px 6px}`
	changesTreeRowsPadB float32 = 6
	changesTreeFont     float32 = 12
	changesTreeLine     float32 = 18
)

// 官方 locales.ts:265/281 的文件树文案（逐字）。
const copyDiffRenamedPath = "%s → %s"

// changedFileNode 是文件树的一个节点。对应官方 `ChangedFileNode`
// （OFF:src/client/file-tree.ts:6-8）。
type changedFileNode struct {
	kind     string // "file" / "directory"
	path     string
	name     string
	file     *apiclient.ForgeDiffFile
	children []*changedFileNode
}

const (
	changedNodeFile      = "file"
	changedNodeDirectory = "directory"
)

// changedFileTree 把一份 diff 的文件清单折成目录树：目录优先、单目录链压缩、
// 同级按 name 排序。逐条对应官方 `file-tree.ts:17-53` 的 `childrenOf`/`changedFileTree`。
func changedFileTree(files []apiclient.ForgeDiffFile) []*changedFileNode {
	root := &treeDir{}
	for i := range files {
		file := &files[i]
		directory := root
		parts := splitPath(file.Path)
		for _, name := range parts[:max(len(parts)-1, 0)] {
			child := directory.directories[name]
			if child == nil {
				child = &treeDir{name: name}
				if directory.path == "" {
					child.path = name
				} else {
					child.path = directory.path + "/" + name
				}
				if directory.directories == nil {
					directory.directories = map[string]*treeDir{}
				}
				directory.directories[name] = child
			}
			directory = child
		}
		directory.files = append(directory.files, file)
	}
	return treeChildren(root)
}

// treeDir 是建树过程的中间目录（官方 file-tree.ts:10-15 的 `Directory`）。
type treeDir struct {
	path        string
	name        string
	directories map[string]*treeDir
	files       []*apiclient.ForgeDiffFile
}

// treeChildren 生成一个目录的子节点：先目录后文件，各自按 name 排序。
// 单目录链压缩（官方 `while (leaf.files.length === 0 && leaf.directories.size === 1)`）。
func treeChildren(directory *treeDir) []*changedFileNode {
	nodes := make([]*changedFileNode, 0, len(directory.directories)+len(directory.files))
	for _, child := range directory.directories {
		leaf := child
		name := child.name
		for len(leaf.files) == 0 && len(leaf.directories) == 1 {
			for _, next := range leaf.directories {
				name += "/" + next.name
				leaf = next
			}
		}
		nodes = append(nodes, &changedFileNode{
			kind:     changedNodeDirectory,
			path:     leaf.path,
			name:     name,
			children: treeChildren(leaf),
		})
	}
	for _, file := range directory.files {
		nodes = append(nodes, &changedFileNode{
			kind: changedNodeFile,
			path: file.Path,
			name: baseName(file.Path),
			file: file,
		})
	}
	sortChangedNodes(nodes)
	return nodes
}

// sortChangedNodes 按 name 做稳定排序，目录组在前、文件组在后（官方
// file-tree.ts:30-31 的 `[...ordered(directories), ...ordered(files)]`）。
func sortChangedNodes(nodes []*changedFileNode) {
	if len(nodes) < 2 {
		return
	}
	directories := make([]*changedFileNode, 0, len(nodes))
	files := make([]*changedFileNode, 0, len(nodes))
	for _, node := range nodes {
		if node.kind == changedNodeDirectory {
			directories = append(directories, node)
		} else {
			files = append(files, node)
		}
	}
	sortNodesByName(directories)
	sortNodesByName(files)
	copy(nodes, append(directories, files...))
}

// sortNodesByName 是插入排序：树很小（一份 diff 几十个文件），保持确定性即可。
func sortNodesByName(nodes []*changedFileNode) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j-1].name > nodes[j].name; j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
}

// filesInTreeOrder 按树的深度优先顺序读回文件（官方 file-tree.ts:61-70）。
func filesInTreeOrder(nodes []*changedFileNode) []*apiclient.ForgeDiffFile {
	out := make([]*apiclient.ForgeDiffFile, 0, len(nodes))
	for _, node := range nodes {
		if node.kind == changedNodeFile {
			out = append(out, node.file)
			continue
		}
		out = append(out, filesInTreeOrder(node.children)...)
	}
	return out
}

// splitPath 按 `/` 切 Git 路径（官方注释：Git 路径永远用正斜杠，Windows 也是）。
func splitPath(path string) []string {
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}

// baseName 是路径最后一段（官方 `file.path.slice(lastIndexOf('/')+1)`）。
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// changesTreeMaximum 是官方 `maximum`（FileTreePane.tsx:38）：
// `max(MIN_WIDTH, min(MAX_WIDTH, available-DIFF_MIN_WIDTH))`。
func changesTreeMaximum(available float32) float32 {
	if available <= 0 {
		return changesTreeMaxWidth
	}
	maximum := available - changesDiffMinWidth
	if maximum > changesTreeMaxWidth {
		maximum = changesTreeMaxWidth
	}
	if maximum < changesTreeMinWidth {
		maximum = changesTreeMinWidth
	}
	return maximum
}

// changesSplitterMins 把官方的 maximum 折算成 Viva Splitter 的 `Mins(first, second)`：
// 第一个 pane 的最小宽度 = 200；第二个 pane 的最小宽度反算出
// hi = available - 8 - second = max(200,min(600,available-320))。
//
// 注意 second **不能**再钳到 DIFF_MIN_WIDTH：Viva 的 8px 手柄是独立兄弟，而官方的
// 8px `.resize` 是叠在树右缘上的，所以 available=760 时 second=312（视觉上 diff
// 仍有 320−8=312px，加上手柄 8px 正好 320）——钳到 320 会让 maximum 少 8px。
func changesSplitterMins(available float32) (float32, float32) {
	if available <= 0 {
		return changesTreeMinWidth, changesDiffMinWidth
	}
	maxWidth := changesTreeMaximum(available)
	second := available - changesTreeSplitterThickness - maxWidth
	if second < 0 {
		second = 0
	}
	return changesTreeMinWidth, second
}

// changesTreeVisible 是官方 `@container (min-width:760px)` 的等价判据。
func changesTreeVisible(width float32) bool { return width >= changesTreeBreakpoint }

// treeWidthOf 把状态里的 Pos 钳进官方区间（与 FileTreePane.tsx:45 的 `shown` 同义）。
func treeWidthOf(state *ui.SplitterState, available float32) float32 {
	if state == nil {
		return changesTreeDefaultWidth
	}
	width := state.Pos
	maximum := changesTreeMaximum(available)
	if width > maximum {
		width = maximum
	}
	if width < changesTreeMinWidth {
		width = changesTreeMinWidth
	}
	return width
}

// drawFileTree 画树栏：`.navigation{height:100%;overflow-y:auto}`（FileTreePane.module.css:5）
// 加 `.rows{overflow:auto;padding:0 4px 6px}`（ChangedFiles.module.css:4）。
func (p *Plugin) drawFileTree(f ui.C, env *pluginapi.Env, diff *pluginapi.ForgeDiff) {
	nodes := changedFileTree(diff.Files)
	if len(nodes) == 0 {
		return
	}
	session := diff.SessionID
	ui.Scroll(f).FillWidth().FillHeight().Children(func() {
		ui.Column(f).FillWidth().PadSides(0, changesTreeRowsPadR, changesTreeRowsPadB, changesTreeRowsPadR).Children(func() {
			p.drawTreeNodes(f, env, session, nodes, 0)
		})
	})
}

// drawTreeNodes 递归画一层节点（官方 `Rows`，ChangedFiles.tsx:32-67）。
func (p *Plugin) drawTreeNodes(f ui.C, env *pluginapi.Env, session string, nodes []*changedFileNode, depth int) {
	t := p.themeOf(env)
	for _, node := range nodes {
		directory := node.kind == changedNodeDirectory
		expanded := directory && !p.open.treeCollapsed(session, node.path)
		label := node.path
		if !directory && node.file != nil && node.file.PreviousPath != nil && *node.file.PreviousPath != "" {
			label = sprintf(copyDiffRenamedPath, *node.file.PreviousPath, node.path)
		}
		selected := !directory && p.open.selectedFile(session) == node.path
		row := ui.Row(f).FillWidth().MinHeight(changesTreeRowH).Gap(changesTreeGap).Items(ui.Center).
			Radius(t.Radii.SM).PadSides(changesTreePadY, changesTreePadX, changesTreePadY, changesTreeIndent0+float32(depth)*changesTreeIndent).
			Hit(env.Tree.ID(env.Feat, "forge-tree:"+node.path), label)
		if selected {
			row = row.Background(t.Hover)
		} else {
			row = row.Hover(ui.Look{}.Background(t.Hover))
		}
		row.Children(func() {
			// 官方 chevron 只给目录：12px 的固定槽，收起转 -90°（ChangedFiles.tsx:55-57）。
			ui.Row(f).Width(changesTreeChevron).Height(changesTreeChevron).Shrink(0).
				Items(ui.Center).Justify(ui.Center).Children(func() {
				if directory {
					if icon, ok := chevronDownIcon(); ok {
						box := ui.Icon(f, icon).Size(changesTreeChevron, changesTreeChevron).Color(t.LabelTertiary)
						if !expanded {
							box.IconTransform(gfx.IconTransform{Rotate: -math.Pi / 2})
						}
					}
				}
			})
			if directory {
				if icon, ok := dsassets.WorkspaceFolder(t.LabelSecondary, expanded); ok {
					ui.Icon(f, icon).Size(changesTreeIconSize, changesTreeIconSize).Color(t.LabelSecondary)
				}
			} else if asset, ok := dsassets.FileType(node.path); ok {
				icon := ui.Column(f).Width(changesTreeIconSize).Height(changesTreeIconSize).Shrink(0)
				icon.Children(func() {
					ui.Icon(f, asset.Sheet).Width(changesTreeIconSize).Height(changesTreeIconSize)
					if asset.Mark != nil {
						ui.Icon(f, *asset.Mark).Absolute().Inset(0).Color(gfx.RGB24(0xffffff))
					}
				})
			}
			ui.Text(f, node.name).FontSize(changesTreeFont).LineHeight(changesTreeLine).
				Color(t.LabelPrimary).Grow(1).Shrink(1).Ellipsis()
			if !directory && node.file != nil && !node.file.Binary {
				ui.Text(f, "+"+itoa(node.file.Additions)).FontSize(changesTreeFont).LineHeight(changesTreeLine).
					Mono().Color(t.Success).Shrink(0)
				ui.Text(f, "−"+itoa(node.file.Deletions)).FontSize(changesTreeFont).LineHeight(changesTreeLine).
					Mono().Color(t.Error).Shrink(0)
			}
		})
		if row.Clicked() {
			if directory {
				p.open.toggleTreeDir(session, node.path)
			} else {
				p.open.selectFile(session, node.path)
			}
		}
		if directory && expanded {
			p.drawTreeNodes(f, env, session, node.children, depth+1)
		}
	}
}

// vivaThemeOf 是 components.BeginScrollArea 需要的 viva 主题。
func (p *Plugin) vivaThemeOf(env *pluginapi.Env) ui.Theme {
	return dscore.VivaTheme(p.themeOf(env))
}
