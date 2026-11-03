// Package dsfiles 是侧栏的「工作区文件」列表 + 点开预览。
//
// 官方对应 `dsh-client-ui-sidebar-files` + `sidebar-documentpreview`。
//
// # 只做一层，因为宿主只给一层
//
// `workspace.list` 回的是 `{name,isDir,size}`——**没有 path，也没有"列子目录"的入参**。
// 所以这里只列工作区**根目录的文件**（目录不列：点不进去，列出来就是骗人）。
// 要做完整文件树，得先在宿主 `workspace.list` 上补 `path` 入参。
//
// # 路径怎么拼
//
// `Snapshot.CWD`（宿主的 `session.load.cwd`，会话自己的工作目录）+ "/" + name。
// **CWD 为空就别拼**——拼出来的 `/name` 会被 `fs.read` 当"文件不存在"，
// 而界面上看起来像"这个文件读不了"，用户会去查文件，其实是我们没数据。
package dsfiles

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// maxShown 是列表最多显示几项。根目录可能有一两百个文件，
// 侧栏底部那块地方塞不下——超出的截断，并在末尾写「还有 N 个」。
const maxShown = 8

// Plugin 持有当前选中的文件与它的内容。
type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

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
	names := env.Act.FSList()
	if len(names) == 0 {
		return
	}
	t := p.theme
	shown := names
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	f.Column().FillWidth().Gap(2).PadY(4).Children(func() {
		f.Text("文件").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		for _, name := range shown {
			label := name
			if label == p.sel {
				label = "▸ " + label
			}
			row := p.prims.ListRow(f, env.Tree.ID(env.Feat, "file-"+name), label == p.sel, 22, func() {
				f.Text(label).FontSize(12).Color(t.LabelSecondary)
			})
			if row.Clicked() {
				p.open(env, name)
			}
		}
		if more > 0 {
			f.Text("还有 " + itoa(more) + " 个").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// open 读一个文件。失败把原因原样记下来（预览里显示），不吞。
func (p *Plugin) open(env *pluginapi.Env, name string) {
	p.sel = name
	p.content, p.err = "", ""
	if env.Act.FSRead == nil {
		p.err = "壳没接 fs.read"
		return
	}
	body, err := env.Act.FSRead(env.Data.CWD + "/" + name)
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
			f.Text(p.sel).FontSize(14).Color(t.LabelPrimary)
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

var _ = ui.KeyEscape // 保留 import（后续加"Esc 关预览"时直接用）
