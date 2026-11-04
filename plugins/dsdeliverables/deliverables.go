// Package dsdeliverables 列出**这次会话里被写过的文件**。
//
// 官方对应 `dsh-client-ui-deliverables`：它回答"我这轮实际产出了什么"。
//
// # 它凭什么算"交付物"
//
// 没有一份"交付物清单"从宿主传下来，也没有 `fs.write` 的调用日志——
// 所以这里**只认一件事**：工具消息里 `write`/`edit`/`bash` 这类写类工具
// 的参数中出现过的路径。**这是痕迹推断，不是权威声明**：
//
//   - 只认**参数里有明确路径**的（`filePath` / `path`），不从命令串里做正则挖路径
//     ——`bash {"command":"rm -rf x"}` 里那个 x 不是交付物，靠猜会列出删掉的东西
//   - 同一个路径只出现一次（按首次出现排序）
//   - **不判断文件是否还在**（那要真读盘，而侧栏每帧都读盘会卡）
//
// 所以它的标题是「写过的文件」，不是「交付物」——**名字必须跟它的证据强度一致**。
package dsdeliverables

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
)

// maxShown 最多列几个。
const maxShown = 6

// writeTools 是"会写盘"的工具名。
//
// 只列这三个是刻意的：`read`/`glob`/`grep` 只读，`bash` 不列（它的路径藏在命令串里，
// 抽出来就是猜）。要加新工具，先确认它的参数里有**明确**的路径字段。
var writeTools = map[string]bool{
	"write": true,
	"edit":  true,
	"list":  false, // 列目录不写
}

// Plugin 无状态：每帧从 Msgs 现算。
type Plugin struct {
	// preview 是正在浮层里预览的那个文件（相对路径）。
	// 空 = 没在预览。**只在本次打开期间有效**：切会话不清，
	// 因为"我刚点了哪个文件"和"我在哪个会话"是两件事。
	preview string
	// previewText 是读到的内容；空且 preview 非空 = 还没读到。
	previewText string
	// scrolling 是 FSRead 的最近一份（Draw 时刷新给 Clicked 分支用）。
	scrolling func(string) (string, error)

	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

func (*Plugin) ID() string { return "ds-deliverables" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		{
			ID:    "deliverables-list",
			Slot:  pluginapi.SlotSidebarFooter,
			Order: 60, // dsfiles(40) → dsschedule(50) → 这里
			Draw:  p.draw,
		},
		{
			ID:    "deliverables-preview",
			Slot:  pluginapi.SlotShellOverlay,
			Order: 96, // 和 files-preview 同一层，靠左错开一点
			Draw:  p.drawPreview,
		},
	}
}

// WrittenPaths 从消息流里提取 write/edit 工具参数明确写过的路径。
// 返回值是按首次出现排序的新切片，调用方可以只读使用，不会共享插件内部状态。
// 它不从助手正文、bash 命令或文件系统猜测任何额外路径。
func WrittenPaths(msgs []apiclient.Message) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range msgs {
		if m.Role != "tool" || m.Tool == "" || !writeTools[m.Tool] {
			continue
		}
		var args map[string]any
		if json.Unmarshal([]byte(m.Args), &args) != nil {
			continue
		}
		for _, k := range []string{"filePath", "path", "file_path"} {
			if v, ok := args[k].(string); ok && v != "" && !seen[v] {
				seen[v] = true
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// paths 保留本包原有绘制与测试入口；跨插件复用统一走 WrittenPaths。
func paths(msgs []apiclient.Message) []string { return WrittenPaths(msgs) }

// draw 画「写过的文件」区。没有就整块不画。
func (p *Plugin) draw(f *flex.Frame, env *pluginapi.Env) {
	files := paths(env.Data.Msgs)
	if len(files) == 0 {
		return
	}
	t := p.theme
	shown := files
	more := 0
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}

	f.Column().FillWidth().Gap(2).PadY(4).Children(func() {
		f.Text("写过的文件").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		for i, path := range shown {
			path := path
			// 只显示文件名：侧栏窄，完整路径会被 Ellipsis 截成"…本.jsp"这种没用的东西。
			name := filepath.Base(path)
			// **可点**：viva 的点击模型是"先 Hit 给 id，再问 Clicked"。
			// 点了就在右边的浮层里看内容——这是这块唯一有真实数据源的动作
			// （`fs.read` 一直在，而"文件的 diff"要宿主提供历史，那是另一件事）。
			row := f.Row().FillWidth().Items(flex.Center).Gap(6).PadSides(4, 4, 2, 2).
				Hit(env.Tree.ID(env.Feat, "deliv-"+itoa(i)), name)
			if row.Clicked() {
				p.preview = path
				p.previewText = ""
				p.scrolling = env.Act.FSRead
			}
			row.Children(func() {
				f.Text(name).FontSize(12).Color(t.LabelSecondary).Ellipsis()
			})
		}
		if more > 0 {
			f.Text("还有 " + itoa(more) + " 个").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// maxPreviewLines 浮层里最多显示几行。
//
// 60 行：够看清一个函数或一段配置，又不至于把整屏占满。
// 完整文件该在编辑器里看——这个浮层的作用是"是不是我改的那个文件"。
const maxPreviewLines = 60

// drawPreview 在浮层里显示被点中的那个文件。
//
// # 只读一次
//
// `FSRead` 是**同步**的（壳里是发 HTTP 等结果）。每帧都读会把绘制卡死，
// 所以在"点了新文件"的时候读一次、缓存在 `previewText` 里。
// 代价是文件在别处被改了这里不会自动更新——**能接受**：
// 这是"看一眼改了什么"的浮层，不是编辑器。
func (p *Plugin) drawPreview(f *flex.Frame, env *pluginapi.Env) {
	if p.preview == "" {
		return
	}
	if p.previewText == "" && env.Act.FSRead != nil {
		// 读失败也记成一句话，否则每帧都会重试一次失败的 HTTP。
		if text, err := env.Act.FSRead(p.preview); err == nil {
			p.previewText = text
		} else {
			p.previewText = "（读不到：" + err.Error() + "）"
		}
	}
	// 按行切：**只认 \n**，不处理 \r\n——文件预览不是协议解析，
	// 多一个 \r 顶多让行尾多一个看不见的字符，不值得为它加分支。
	lines := strings.Split(strings.TrimRight(p.previewText, "\n"), "\n")
	if len(lines) > maxPreviewLines {
		lines = lines[:maxPreviewLines]
	}

	t := p.theme
	f.Column().Absolute().Top(56).Left(280).Width(720).Children(func() {
		f.Column().Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
			PadSides(14, 14, 12, 14).Gap(4).Children(func() {
			f.Row().FillWidth().Items(flex.Center).Gap(8).Children(func() {
				f.Text(filepath.Base(p.preview)).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Grow(1).Ellipsis()
				// 关：**必须有个关的办法**，否则浮层一直挡着侧栏。
				closeRow := f.Row().PadSides(4, 4, 2, 2).Hit(env.Tree.ID(env.Feat, "deliv-close"), "关闭预览")
				if closeRow.Clicked() {
					p.preview, p.previewText = "", ""
				}
				closeRow.Children(func() {
					f.Text("关闭").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				})
			})
			for _, ln := range lines {
				if ln == "" {
					ln = " "
				}
				f.Text(ln).FontSize(11).Color(t.LabelSecondary)
			}
		})
	})
}

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

var _ = strings.TrimSpace // 保留 import（后续做路径归一化时用）
