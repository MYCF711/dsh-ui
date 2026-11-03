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
	return []pluginapi.Painter{{
		ID:    "deliverables-list",
		Slot:  pluginapi.SlotSidebarFooter,
		Order: 60, // dsfiles(40) → dsschedule(50) → 这里
		Draw:  p.draw,
	}}
}

// paths 从消息流里提取"写过的文件"。
//
// 只看 `tool` 角色的消息，且工具名在 writeTools 里、参数是 JSON 对象、
// 里面有 `filePath` 或 `path` 字符串。**取不到就跳过**，不猜。
func paths(msgs []apiclient.Message) []string {
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
		for _, path := range shown {
			// 只显示文件名：侧栏窄，完整路径会被 Ellipsis 截成"…本.jsp"这种没用的东西。
			name := filepath.Base(path)
			f.Text(name).FontSize(12).Color(t.LabelSecondary).Ellipsis()
		}
		if more > 0 {
			f.Text("还有 " + itoa(more) + " 个").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
		}
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
