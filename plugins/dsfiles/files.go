// Package dsfiles 是侧栏的「工作区文件」列表 + 点开预览。
//
// 官方对应 `dsh-client-ui-sidebar-files` + `sidebar-documentpreview`。
//
// # 一层还是一棵树：现在是树，但**薄**
//
// `workspace.list` 早年只回 `{name,isDir,size}` 且没有 path 入参，那时这里只能列根目录。
// 后来宿主补了 `path` 入参，所以现在能进子目录（`curDir` 记住相对位置，"← 上一级"返回）。
//
// 仍然是"薄"的一层：没有缓存、没有排序、没有按需展开，进一次目录问一次宿主。
// 壳的 `FSList` 是**异步缓存**（第一次问返回空，下一帧才有），所以
// "这一帧没有条目"不等于"目录是空的"——那种情况什么都不画，等下一帧。
//
// # 路径怎么拼
//
// 绝对路径 = `Snapshot.CWD` + "/" + `curDir`（相对 CWD，可能为空）+ "/" + name。
// **CWD 为空就别拼**——拼出来的 `/name` 会被 `fs.read` 当"文件不存在"，
// 而界面上看起来像"这个文件读不了"，用户会去查文件，其实是我们没数据。
//
// ⚠️ 拼路径**只走 `dirPath` 一处**。这里真出过 bug：列表已经会进子目录了，
// 但点文件时仍然用 `CWD + "/" + name`，于是子目录里的文件一律"读不出来"——
// 而这个包当时**一个测试都没有**，所以它一直没被发现（2026-11-04 补的测试）。
package dsfiles

import (
	"path/filepath"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// maxShown 是列表最多显示几项。根目录可能有一两百个文件，
// 侧栏底部那块地方塞不下——超出的截断，并在末尾写「还有 N 个」。
const maxShown = 8

// Plugin 持有当前选中的文件与它的内容。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives
	// menus 是 context menu 服务（可选：没有它文件行就没有右键菜单，
	// 列表本身照常画——官方 ui-context-menu 与 ui-sidebar-files 也是两条独立的包）。
	menus pluginapi.ContextMenus
	// read 是每帧从 env.Act 刷新的读文件函数（Open 走测试开关时用最近一次的值）。
	read func(string) (string, error)

	curDir  string // 当前目录（相对于 CWD；空 = 根）
	sel     string // 当前预览的文件名（空 = 没选）
	content string // 读到的内容（只取前 maxPreviewBytes）
	err     string // 读取失败的原因（原样给用户看，不翻译成"加载失败"）
	// draggable 是上一帧每个文件行的命中身份 → 绝对路径，press 是按下还没越过阈值的
	// 那一次，dragErr 是拖出失败的原因（原样给用户看）。见 drag_files.go。
	draggable map[gfx.ID]string
	press     filePress
	dragErr   string
}

func (*Plugin) ID() string { return "ds-files" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	// 可选注入：没有菜单服务时文件行不挂右键菜单（不报错、也不假装有）。
	if value := ctx.Inject(pluginapi.KeyContextMenus); value != nil {
		if menus, ok := value.(pluginapi.ContextMenus); ok {
			p.menus = menus
		}
	}
	return nil
}

// maxPreviewBytes 是一次预览最多读多少字节。
//
// 为什么限：`fs.read` 会把整个文件传过来，一个 10 MB 的日志会把界面卡住。
// 16 KB 够看清一个源码文件的结构；更大的文件应该在编辑器里开，不是在侧栏预览。
const maxPreviewBytes = 16 * 1024

// 文件树几何逐项来自 rc.2 ui-sidebar-files：
// FilesBody.module.css:140-151（body padding 8 0 8 8）；
// FileTree.module.css:18-42（行 radius-md、padding 5 10、gap 6）；
// FileTree.tsx:222-224（FileTypeIcon/文件夹 size=16）；
// FilesBody.module.css:2-12（默认 13px、line-height 1.5）。
const (
	fileTreePadTop    float32 = 8
	fileTreePadRight  float32 = 0
	fileTreePadBottom float32 = 8
	fileTreePadLeft   float32 = 8
	fileRowPadY       float32 = 5
	fileRowPadX       float32 = 10
	fileRowGap        float32 = 6
	fileRowIconSize   float32 = 16
	fileRowLineHeight float32 = 13 * 1.5
	fileRowHeight             = fileRowLineHeight + 2*fileRowPadY
	fileNotePadY      float32 = 3
	fileNotePadX      float32 = 10
	fileNoteFontSize  float32 = 12
	fileBackIconSize  float32 = 14
)

// Panels 挂两块：侧栏底部是文件列表，主区右下是预览浮层。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{ID: "files-list", Slot: pluginapi.SlotSidebarFooter, Order: 40, Draw: p.drawList},
		{ID: "files-preview", Slot: pluginapi.SlotShellOverlay, Order: 97, Draw: p.drawPreview},
	}
}

// drawList 画侧栏底部的文件列表。
//
// **CWD 为空就整块不画**——没有工作区根，列出来的东西点了也读不了。
func (p *Plugin) drawList(f ui.C, env *pluginapi.Env) {
	if env.Data.CWD == "" || env.Act.FSList == nil {
		return
	}
	// 目录路径 = dirPath(CWD, curDir)。**CWD 为空整块不画**（见文件头）。
	dir := dirPath(env.Data.CWD, p.curDir)
	names := env.Act.FSList(dir)
	p.read = env.Act.FSRead
	t := p.theme

	// 首次问某个目录返回空是**正常的**（壳在后台拉），不是"这个目录没东西"。
	// 所以这里什么都不画，等下一帧——不能写"空目录"，那是在编。
	if len(names) == 0 {
		p.dragRows(nil) // 这一帧没有行，就没有可拖的东西
		return
	}

	shown := names
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	ui.Column(f).FillWidth().Gap(0).
		PadSides(fileTreePadTop, fileTreePadRight, fileTreePadBottom, fileTreePadLeft).Children(func() {
		title := "文件"
		if p.curDir != "" {
			title = p.curDir
		}
		// FileTree.module.css:108-113 的 note 档也用于本仓保留的目录提示。
		ui.Row(f).FillWidth().PadSides(fileNotePadY, fileNotePadX, fileNotePadY, fileNotePadX).Children(func() {
			ui.Text(f, title).FontSize(fileNoteFontSize).Color(t.LabelTertiary).Ellipsis()
		})
		if p.curDir != "" {
			back := p.fileRow(f, false, func() {
				if icon, ok := dsassets.FileChevron(true); ok {
					ui.Icon(f, icon).Size(fileBackIconSize, fileBackIconSize).Color(t.LabelTertiary)
				}
				ui.Text(f, "上一级").FontSize(t.Fonts.Secondary).LineHeight(fileRowLineHeight).Color(t.LabelTertiary)
			}).Hit(env.Tree.ID(env.Feat, "files-up"), "")
			if back.Clicked() {
				p.curDir = parentDir(p.curDir)
				p.sel = ""
			}
		}
		draggable := make(map[gfx.ID]string, len(shown))
		for _, e := range shown {
			e := e
			active := !e.IsDir && e.Name == p.sel
			row := p.fileRow(f, active, func() {
				p.drawFileIcon(f, t, e)
				ui.Text(f, e.Name).FontSize(t.Fonts.Secondary).LineHeight(fileRowLineHeight).
					Color(t.LabelPrimary).Ellipsis().Grow(1)
			}).Hit(env.Tree.ID(env.Feat, "file-"+e.Name), "")
			// 只有普通文件能拖出窗口（目录在 CF_HDROP 里也是合法的，但把一整个目录
			// 拖到别的应用是另一个承诺——官方 file target 的 `read` 家族同样只认普通
			// 文件，这里取同一口径）。
			if !e.IsDir {
				draggable[env.Tree.ID(env.Feat, "file-"+e.Name)] = filepath.Join(dir, e.Name)
			}
			// 文件行是官方 `file` target 的生产方（#18）：身份是
			// (会话, 工作区相对路径)，**不是**"当前选中"——菜单打开后用户可能
			// 已经切走会话或进别的目录（官方 types.ts:30 的 sessionId + path）。
			// 目录不挂：官方 file target 的 `read` 家族只认普通文件
			// （HOST:cmd/dshd/serve_workspace_api.go 的 statRegularFile）。
			if !e.IsDir && p.menus != nil && env.Data.Cur != "" {
				p.menus.Attach(f, env.Tree.ID(env.Feat, "file-"+e.Name), row, pluginapi.ContextMenuTarget{
					Kind:         pluginapi.TargetFile,
					SessionID:    env.Data.Cur,
					Path:         relPath(p.curDir, e.Name),
					DisplayTitle: e.Name,
				})
			}
			if !row.Clicked() {
				continue
			}
			if e.IsDir {
				// 进目录：curDir 是相对 CWD 的路径
				if p.curDir == "" {
					p.curDir = e.Name
				} else {
					p.curDir += "/" + e.Name
				}
				p.sel = ""
			} else {
				// 读的是**当前目录**里的文件，不是根目录里的同名文件。
				p.open(env, dirPath(env.Data.CWD, p.curDir), e.Name)
			}
		}
		// 记下本帧可拖的行，壳的 OnPointer 用它把按下变成一次真的 OS 拖出。
		p.dragRows(draggable)
		if more > 0 {
			ui.Row(f).FillWidth().PadSides(fileNotePadY, fileNotePadX, fileNotePadY, fileNotePadX).Children(func() {
				ui.Text(f, "还有 "+itoa(more)+" 个").FontSize(fileNoteFontSize).Color(t.LabelTertiary)
			})
		}
		// 拖出失败要说出来：官方 `file-drag.feedback`（feedback.tsx:18-25）是同一个
		// 意思的 Toast，本仓没有 Toast 服务，就落在列表下面一行，原样给原因。
		if p.dragErr != "" {
			ui.Row(f).FillWidth().PadSides(fileNotePadY, fileNotePadX, fileNotePadY, fileNotePadX).Children(func() {
				ui.Text(f, "拖出失败: "+p.dragErr).FontSize(fileNoteFontSize).Color(t.LabelTertiary).Ellipsis()
			})
		}
	})
}

// fileRow 画 FileTree.module.css:18-42 的行：行与行之间 gap=0，
// hover/current 分别使用 official design-platform.css:238,242（亮）与 :390,394（暗）。
func (p *Plugin) fileRow(f ui.C, active bool, body func()) ui.Box {
	t := p.theme
	row := ui.Row(f).FillWidth().Height(fileRowHeight).Radius(t.Radii.MD).
		PadSides(fileRowPadY, fileRowPadX, fileRowPadY, fileRowPadX).
		Items(ui.Center).Gap(fileRowGap)
	if active {
		row = row.Background(t.InteractiveActive)
	} else {
		row = row.Hover(ui.Look{}.Background(t.Hover))
	}
	return row.Children(body)
}

// drawFileIcon 复用 FileTree.tsx:222-224 的 16px 文件夹与 FileTypeIcon。
func (p *Plugin) drawFileIcon(f ui.C, t *pluginapi.Theme, entry pluginapi.FSEntry) {
	ui.Row(f).Size(fileRowIconSize, fileRowIconSize).Shrink(0).Items(ui.Center).Justify(ui.Center).Children(func() {
		if entry.IsDir {
			if icon, ok := dsassets.WorkspaceFolder(t.LabelTertiary, false); ok {
				ui.Icon(f, icon).Size(fileRowIconSize, fileRowIconSize).Color(t.LabelTertiary)
			}
			return
		}
		asset, ok := dsassets.FileType(entry.Name)
		if !ok {
			return
		}
		ui.Icon(f, asset.Sheet).Size(fileRowIconSize, fileRowIconSize)
		if asset.Mark != nil {
			ui.Icon(f, *asset.Mark).Absolute().Inset(0).Color(fileTypeMarkColor())
		}
	})
}

// fileTypeMarkColor 是 FileTypeIcon 的 --dsw-static-neutral-00 标记色。
func fileTypeMarkColor() ui.Color { return gfx.RGB24(0xffffff) }

// Open 是**给壳的测试开关用的**公开入口：无头截图没法点鼠标，
// 所以壳用 `-open-file` 直接调它来验预览。产品路径不走这里（走 drawList 的点击）。
//
// 它要求先有 CWD——没有就什么都不做（和 drawList 的判断一致）。
func (p *Plugin) Open(cwd, name string) {
	p.open(nil, cwd, name)
}

// open 读一个文件。失败把原因原样记下来（预览里显示），不吞。
func (p *Plugin) open(env *pluginapi.Env, cwd, name string) {
	if cwd == "" {
		return
	}
	p.sel = name
	p.content, p.err = "", ""
	read := p.read
	if env != nil {
		read = env.Act.FSRead
	}
	if read == nil {
		p.err = "壳没接 fs.read"
		return
	}
	body, err := read(cwd + "/" + name)
	if err != nil {
		p.err = err.Error()
		return
	}
	if len(body) > maxPreviewBytes {
		body = body[:maxPreviewBytes] + "\n… （只显示前 16 KB）"
	}
	p.content = body
}

// 文档预览几何：TextPreview.module.css:42-51（38px header、0 6 0 16、
// 0.5px border-l3）、:53-55（path margin-right 12）、:71-89（13px mono、
// line-height 1.6）与 :113-122（正文 8px inset）；浮层 inset/radius 来自
// FilesBody.module.css:123-134。打开按钮来自 open-in-app
// OpenTargetButton.module.css:10-44,64-66 与 OpenTargetButton.tsx:72-76,112。
const (
	previewOverlayInset   float32 = 8
	previewHeaderHeight   float32 = 38
	previewHeaderGap      float32 = 4
	previewHeaderPadRight float32 = 6
	previewHeaderPadLeft  float32 = 16
	previewPathMargin     float32 = 12
	previewHairline       float32 = 0.5
	previewBodyPad        float32 = 8
	previewBodyLineHeight float32 = 13 * 1.6
	previewOpenHeight     float32 = 24
	previewOpenBorder     float32 = 0.5
	previewOpenFontSize   float32 = 11
	previewOpenLineHeight float32 = 16
	previewOpenPadY       float32 = 3
	previewOpenPadX       float32 = 5
	previewOpenGap        float32 = 4
	previewOpenIconSize   float32 = 13
)

// drawPreview 在右上角画预览卡片。浮层宽度保留本仓既有桌面适配值；
// 卡片内部的几何、字号与操作按钮逐项对齐官方 document preview/open-in-app。
func (p *Plugin) drawPreview(f ui.C, env *pluginapi.Env) {
	if p.sel == "" {
		return
	}
	t := p.theme
	ui.Column(f).Absolute().Top(previewOverlayInset).Right(previewOverlayInset).Width(520).Children(func() {
		ui.Column(f).Radius(t.Radii.LG).Background(t.InputMajor).Border(previewHairline, t.BorderL2).
			Gap(0).Children(func() {
			// 标题行：官方 16px 文件类型图标 + path + 默认应用按钮。
			ui.Row(f).FillWidth().Height(previewHeaderHeight).Items(ui.Center).Gap(previewHeaderGap).
				PadSides(0, previewHeaderPadRight, 0, previewHeaderPadLeft).Children(func() {
				p.drawFileIcon(f, t, pluginapi.FSEntry{Name: p.sel})
				ui.Text(f, p.sel).FontSize(t.Fonts.Secondary).Color(t.LabelPrimary).
					MarginSides(0, previewPathMargin, 0, 0).Ellipsis().Grow(1)
				if env.Act.OpenFile != nil && p.err == "" {
					open := ui.Row(f).Height(previewOpenHeight).Radius(t.Radii.SM).
						Border(previewOpenBorder, t.BorderL4).Items(ui.Center).Gap(previewOpenGap).
						PadSides(previewOpenPadY, previewOpenPadX, previewOpenPadY, previewOpenPadX).
						Hover(ui.Look{}.Background(t.Hover)).
						Hit(env.Tree.ID(env.Feat, "files-open-external"), "用默认程序打开")
					open.Children(func() {
						if icon, ok := dsassets.FileRightUp(t.LabelPrimary); ok {
							ui.Icon(f, icon).Size(previewOpenIconSize, previewOpenIconSize).Color(t.LabelPrimary)
						}
						ui.Text(f, "用默认程序打开").FontSize(previewOpenFontSize).
							LineHeight(previewOpenLineHeight).Color(t.LabelPrimary)
					})
					if open.Clicked() {
						if dst, err := fileURL(dirPath(env.Data.CWD, p.curDir) + "/" + p.sel); err == nil {
							// 失败由壳写状态栏（它统一管反馈），插件不重复报。
							_ = env.Act.OpenFile(dst)
						}
					}
				}
			})
			ui.Row(f).FillWidth().Height(previewHairline).Background(t.BorderL3)
			ui.Column(f).FillWidth().Pad(previewBodyPad).Children(func() {
				if p.err != "" {
					ui.Text(f, "读不出来："+p.err).FontSize(12).Color(t.Error)
					return
				}
				// 正文保留既有 20 行上限；视觉使用官方 13px/1.6 等宽正文。
				lines := strings.Split(p.content, "\n")
				if len(lines) > 20 {
					lines = append(lines[:20], "… 还有 "+itoa(len(lines)-20)+" 行")
				}
				for _, ln := range lines {
					ui.Text(f, ln).Mono().FontSize(t.Fonts.Secondary).
						LineHeight(previewBodyLineHeight).Color(t.LabelPrimary)
				}
			})
		})
	})
}

// parentDir 去掉最后一段（`a/b` → `a`）；没有斜杠时返回空串。
// 注意「空串」是一个**真实返回值**，不是错误：根目录的上一层就是没有。
// 原来这里写成 `a` → 空，末尾那对空反引号被 gofmt 改成了中文引号——
// 空的反引号代码段在文档注释里没有意义，换个说法比跟 gofmt 较劲便宜。
func parentDir(d string) string {
	for i := len(d) - 1; i >= 0; i-- {
		if d[i] == '/' {
			return d[:i]
		}
	}
	return ""
}

// itoa 是本地整数转字符串（避免为一处 import strconv 而让整个文件看起来杂乱）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

var _ = gfx.KeyEscape // 保留 import（后续加"Esc 关预览"时直接用）
