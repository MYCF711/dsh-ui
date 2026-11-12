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
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
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
	// read 是每帧从 env.Act 刷新的读文件函数（Open 走测试开关时用最近一次的值）。
	read func(string) (string, error)

	curDir  string // 当前目录（相对于 CWD；空 = 根）
	sel     string // 当前预览的文件名（空 = 没选）
	content string // 读到的内容（只取前 maxPreviewBytes）
	err     string // 读取失败的原因（原样给用户看，不翻译成"加载失败"）
}

func (*Plugin) ID() string { return "ds-files" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

// maxPreviewBytes 是一次预览最多读多少字节。
//
// 为什么限：`fs.read` 会把整个文件传过来，一个 10 MB 的日志会把界面卡住。
// 16 KB 够看清一个源码文件的结构；更大的文件应该在编辑器里开，不是在侧栏预览。
const maxPreviewBytes = 16 * 1024

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
func (p *Plugin) drawList(f *flex.Frame, env *pluginapi.Env) {
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
		return
	}

	shown := names
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	f.Column().FillWidth().Gap(2).PadY(4).Children(func() {
		title := "文件"
		if p.curDir != "" {
			title = "▸ " + p.curDir
		}
		f.Text(title).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		if p.curDir != "" {
			if back := p.prims.ListRow(f, env.Tree.ID(env.Feat, "files-up"), false, 22, func() {
				f.Text("← 上一级").FontSize(12).Color(t.LabelTertiary)
			}); back.Clicked() {
				p.curDir = parentDir(p.curDir)
				p.sel = ""
			}
		}
		for _, e := range shown {
			e := e
			label := e.Name
			if e.IsDir {
				label = "▸ " + label // 目录用 ▸ 前缀，和文件区分开
			} else if label == p.sel {
				label = "• " + label
			}
			active := !e.IsDir && e.Name == p.sel
			row := p.prims.ListRow(f, env.Tree.ID(env.Feat, "file-"+e.Name), active, 22, func() {
				f.Text(label).FontSize(12).Color(t.LabelSecondary)
			})
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
		if more > 0 {
			f.Text("还有 " + itoa(more) + " 个").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

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

// drawPreview 在右上角画预览卡片。
//
// 用 Absolute + 显式 Width/Height（本仓唯一被真机证明能出现在 overlay 上的写法，
// 见 dsshortcuts 的注释）；**必须 Column**，Row 会把所有内容挤成一行。
func (p *Plugin) drawPreview(f *flex.Frame, env *pluginapi.Env) {
	if p.sel == "" {
		return
	}
	t := p.theme
	f.Column().Absolute().Top(56).Right(24).Width(520).Children(func() {
		f.Column().Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
			PadSides(16, 16, 14, 16).Gap(8).Children(func() {
			// 标题行：文件名 + 「用默认程序打开」。
			//
			// 官方 `open-in-app` 的这半句是 "the document preview's
			// default-application controls for one file"——预览里得能把它交给
			// 真正的编辑器/查看器，因为侧栏预览只有 20 行、16 KB。
			//
			// 三个前置条件缺一不画：**读成功了**（读不出来还去开，开的是同一个坏路径）、
			// **壳接了 OpenFile**（老壳没有这个动作）、**有选中文件**。
			f.Row().FillWidth().Items(ui.Center).Gap(8).Children(func() {
				f.Text(p.sel).FontSize(14).Color(t.LabelPrimary).Grow(1)
				if env.Act.OpenFile != nil && p.err == "" {
					if f.Button(env.Tree.ID(env.Feat, "files-open-external"), "用默认程序打开").
						Small().Clicked() {
						if dst, err := fileURL(dirPath(env.Data.CWD, p.curDir) + "/" + p.sel); err == nil {
							// 失败由壳写状态栏（它统一管反馈），插件不重复报。
							_ = env.Act.OpenFile(dst)
						}
					}
				}
			})
			if p.err != "" {
				f.Text("读不出来：" + p.err).FontSize(12).Color(t.Error)
			} else {
				// 正文用等宽、行高 18、最多 20 行——超出的截断（这也是"别卡住界面"）
				lines := strings.Split(p.content, "\n")
				if len(lines) > 20 {
					lines = append(lines[:20], "… 还有 "+itoa(len(lines)-20)+" 行")
				}
				for _, ln := range lines {
					f.Text(ln).FontSize(12).Color(t.LabelSecondary)
				}
			}
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
