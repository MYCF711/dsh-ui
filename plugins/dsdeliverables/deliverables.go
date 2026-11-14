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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
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

	// ── 变更记录缓存（见 changes.go）──
	//
	// 按 (会话, seq, 路径) 缓存一次解析结果：绘制是每帧跑的，不缓存就等于每帧
	// 两次 HTTP。键里带 seq 与路径，是因为同一个路径在不同会话/不同轮次
	// 是完全不同的比较结果——只按路径缓存会把上一轮的 diff 画在这一轮上。
	changesKey   changesKey
	changes      changesView
	changesValid bool

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
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
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

	ui.Column(f).FillWidth().Gap(2).PadY(4).Children(func() {
		ui.Text(f, "写过的文件").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		for i, path := range shown {
			path := path
			// 只显示文件名：侧栏窄，完整路径会被 Ellipsis 截成"…本.jsp"这种没用的东西。
			name := filepath.Base(path)
			// **可点**：viva 的点击模型是"先 Hit 给 id，再问 Clicked"。
			// 点了就在右边的浮层里看内容——这是这块唯一有真实数据源的动作
			// （`fs.read` 一直在，而"文件的 diff"要宿主提供历史，那是另一件事）。
			row := ui.Row(f).FillWidth().Items(ui.Center).Gap(6).PadSides(4, 4, 2, 2).
				Hit(env.Tree.ID(env.Feat, "deliv-"+itoa(i)), name)
			if row.Clicked() {
				p.preview = path
				p.previewText = ""
				p.scrolling = env.Act.FSRead
			}
			row.Children(func() {
				ui.Text(f, name).FontSize(12).Color(t.LabelSecondary).Ellipsis()
			})
		}
		if more > 0 {
			ui.Text(f, "还有 "+itoa(more)+" 个").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
	})
}

// maxPreviewLines 浮层里最多显示几行**全文预览**。
//
// 60 行：够看清一个函数或一段配置，又不至于把整屏占满。
// 完整文件该在编辑器里看——这个浮层的作用是"是不是我改的那个文件"。
const maxPreviewLines = 60

// 浮层几何。
//
// 位置与宽度沿用本插件原有的取值（右侧错开侧栏的那个浮层，`Top(56).Left(280)`）；
// 新增的只有"正文区高度"：官方那一半是右栏面板里的可滚动区，**撑满面板**
// （OFF:FileDiff.module.css:5 `.body { overflow: auto }`），本仓的浮层是绝对定位的
// 自由面板，没有面板高度可撑，只能按窗口高度取一个有界值——不然一节 diff
// 会直接顶出屏幕底部，而它下面的行永远看不到。
const (
	overlayTop      = 56
	overlayLeft     = 280
	overlayWidth    = 720
	overlayMinBody  = 160 // 小窗口里也留得下几行
	overlayMaxBody  = 640 // 再高就成了全屏编辑器，那不是这个浮层的用途
	overlayFallback = 320 // 拿不到窗口尺寸时（无头测试）的默认高度
	overlayChrome   = 96  // 上边距 + 头部 + 说明行 + 离窗口底边的留白
)

// diffFillAlpha 是增删行底色的透明度。
//
// 这不是随手取的数：官方色板里就有这个档——`--dsw-static-green-500-a12:
// rgb(34 197 94 / 12%)` 与 `--dsw-static-red-*-a12`
// （design-platform.css:37-38,80），而 `t.Success` 正是 static-green-500、
// `t.Error` 是同一族的红。见 drawDiffRow 里"哪一半是官方原值、哪一半是 derived"。
const diffFillAlpha = 0.12

// diffGutterWidth 是行号列的宽度（官方是 3.5em，OFF:FileDiff.module.css:15）。
const diffGutterWidth = 34

// overlayBodyHeight 是 diff 正文滚动区的高度。
func overlayBodyHeight(env *pluginapi.Env) float32 {
	if env == nil || env.Bounds.Height <= 0 {
		return overlayFallback
	}
	h := env.Bounds.Height - overlayChrome
	if h < overlayMinBody {
		return overlayMinBody
	}
	if h > overlayMaxBody {
		return overlayMaxBody
	}
	return h
}

// drawPreview 在浮层里显示被点中的那个文件。
//
// # 两种正文
//
// 有变更记录（宿主的 `workspace/changes` 路由给得出这个文件）就画 **hunk**；
// 拿不到就退回**全文预览**，并在正文上方写明原因（"这一轮没有变更记录"）。
// 没有第三种结局——**绝不画一个空 diff 框**：那会让人以为"这个文件没改"，
// 而真相是"宿主这里没有记录"（非 git 工作目录的会话就是如此，已登记裁剪）。
func (p *Plugin) drawPreview(f ui.C, env *pluginapi.Env) {
	if p.preview == "" {
		return
	}
	t := p.theme
	plan := planOverlay(p.resolveChanges(env), filepath.Base(p.preview))
	ui.Column(f).Absolute().Top(overlayTop).Left(overlayLeft).Width(overlayWidth).Children(func() {
		ui.Column(f).Radius(t.Radii.Panel).Background(t.InputMajor).Border(1, t.BorderL2).
			PadSides(14, 14, 12, 14).Gap(4).Children(func() {
			ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
				ui.Text(f, plan.Path).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Grow(1).Ellipsis()
				// 计数：官方 Counts 是**两个 span 两种色**
				// （OFF:ChangedFiles.module.css:30-31）：+N 用
				// `--dsw-alias-state-success-primary`（=t.SuccessPrimary，
				// 都是 static-green-500），-M 用 `--dsw-alias-state-error-primary`
				// （=t.Error：暗色 red-400 / 亮色 red-600，与官方同一档）。
				// 所以这里分两块画，而不是拼成一串单色文字。
				if plan.CountsLabel != "" {
					ui.Text(f, plan.CountsLabel).FontSize(t.Fonts.Caption).Mono().Color(t.LabelTertiary)
				}
				if plan.ShowCounts {
					ui.Text(f, fmt.Sprintf(msgChangesAdded, plan.Added)).FontSize(t.Fonts.Caption).
						Mono().Color(t.SuccessPrimary)
					ui.Text(f, fmt.Sprintf(msgChangesDeleted, plan.Deleted)).FontSize(t.Fonts.Caption).
						Mono().Color(t.Error)
				}
				// 关：**必须有个关的办法**，否则浮层一直挡着侧栏。
				closeRow := ui.Row(f).PadSides(4, 4, 2, 2).Hit(env.Tree.ID(env.Feat, "deliv-close"), "关闭预览")
				if closeRow.Clicked() {
					p.preview, p.previewText = "", ""
					p.invalidateChanges()
				}
				closeRow.Children(func() {
					ui.Text(f, "关闭").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				})
			})
			for _, note := range plan.Notes {
				ui.Text(f, note).FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
			}
			if plan.Retry {
				retryRow := ui.Row(f).PadSides(4, 4, 2, 2).Hit(env.Tree.ID(env.Feat, "deliv-retry"), msgRetry)
				if retryRow.Clicked() {
					// 只丢缓存，不在这一帧里重拉：下一帧的 resolveChanges 会重拉，
					// 于是同一帧里"说明行"和"正文"永远一致——不会出现说明说
					// 读不到、正文却是刚拉回来的那种自相矛盾。
					p.invalidateChanges()
				}
				retryRow.Children(func() {
					ui.Text(f, msgRetry).FontSize(t.Fonts.Caption).Color(t.Link)
				})
			}
			if len(plan.Rows) > 0 {
				// 滚动区的名字带 (会话, 路径)：viva 的滚动偏移**按区域名字**
				// 记着（ui/scroll.go:8-16），共用一个名字会让 A 文件滚到一半、
				// 点开 B 文件时直接停在中间。
				diffBodyKey := "deliv-diff:" + env.Data.Cur + ":" + p.preview
				ui.Scroll(f.Key(diffBodyKey)).FillWidth().
					Height(overlayBodyHeight(env)).Scrollbars().Clip().Children(func() {
					for _, row := range plan.Rows {
						drawDiffRow(f, t, row)
					}
				})
			}
			if plan.FullText {
				for _, ln := range p.fullTextLines(env) {
					ui.Text(f, ln).FontSize(11).Color(t.LabelSecondary)
				}
			}
		})
	})
}

// fullTextLines 读全文预览（**只读一次**）。
//
// `FSRead` 是**同步**的（壳里是发 HTTP 等结果）。每帧都读会把绘制卡死，
// 所以在"点了新文件"的时候读一次、缓存在 `previewText` 里。
// 代价是文件在别处被改了这里不会自动更新——**能接受**：
// 这是"看一眼改了什么"的浮层，不是编辑器。
func (p *Plugin) fullTextLines(env *pluginapi.Env) []string {
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
	for i, ln := range lines {
		if ln == "" {
			lines[i] = " "
		}
	}
	return lines
}

// drawDiffRow 画一行 diff（hunk 头、增、删、上下文）。
//
// 几何取自官方 `.line`（OFF:FileDiff.module.css:15）：两个行号列 + 一个 sign 列 +
// 正文列，行高 22px、`white-space: pre`。这里用 Row + 等宽字近似同样的四列。
//
// 颜色分两半，**哪一半是官方原值要写清**：
//
//   - 正负号与行号：官方用 `--dsw-alias-file-diff-{added,deleted}-marker`
//     （OFF:FileDiff.module.css:24-28，亮色 rgb(1,162,65)/rgb(186,39,35)），
//     这两个值**不在 `pluginapi.Theme` 里**（theme.go 只到 `--dsw-alias-state-*` 一档，
//     而本任务的写作用域不许动 `pluginapi/**`）。所以取最近的状态色
//     `t.Success`/`t.Error`（= `--dsw-alias-state-{success,error}-primary`）——
//     是**同类不同档**，属登记在案的已知差异，不冒充官方那两个原值。
//   - 行底：官方是 `--dsw-alias-file-diff-{added,deleted}-bg`（同样是 Theme 里没有的
//     独立令牌），这里用同色 **12% 透明**——那正是官方色板自己的 a12 档
//     （design-platform.css:37-38 的 `--dsw-static-green-500-a12` 等），
//     所以底色是"官方色板的 12% 档"，不是编出来的色。
func drawDiffRow(f ui.C, t *pluginapi.Theme, row diffRow) {
	if row.Kind == rowHunk {
		ui.Text(f, row.Text).FontSize(t.Fonts.Caption).LineHeight(t.Fonts.CaptionLine).
			Mono().Color(t.LabelTertiary)
		return
	}
	ink, sign, filled := t.LabelSecondary, " ", false
	var fill ui.Color
	switch row.Kind {
	case rowAdd:
		ink, sign, fill, filled = t.Success, "+", t.Success.WithAlpha(diffFillAlpha), true
	case rowDel:
		ink, sign, fill, filled = t.Error, "-", t.Error.WithAlpha(diffFillAlpha), true
	}
	r := ui.Row(f).FillWidth().Items(ui.Center).Gap(6)
	if filled {
		r = r.Background(fill)
	}
	r.Children(func() {
		ui.Text(f, diffNumber(row.OldNo)).FontSize(t.Fonts.Caption).Mono().
			Color(t.LabelTertiary).Width(diffGutterWidth).TextEnd()
		ui.Text(f, diffNumber(row.NewNo)).FontSize(t.Fonts.Caption).Mono().
			Color(t.LabelTertiary).Width(diffGutterWidth).TextEnd()
		ui.Text(f, sign).FontSize(t.Fonts.Caption).Mono().Color(ink)
		ui.Text(f, diffCell(row.Text)).FontSize(t.Fonts.Code).Mono().Color(ink).Grow(1)
	})
}

// diffNumber 画一个行号；0 = 这一侧没有行号（官方是 undefined），画成空白。
func diffNumber(n int) string {
	if n <= 0 {
		return ""
	}
	return itoa(n)
}

// diffCell 保证空行也有宽度：空字符串的 Text 在某些布局下会塌成 0 宽，
// 那一行看起来就像"被剪掉了"。
func diffCell(text string) string {
	if text == "" {
		return " "
	}
	return text
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
