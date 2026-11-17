package dstool

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// 这个文件就是「按工具类型分派」的那张表：每种工具一个 Renderer。
//
// 想加一种工具的画法，只加一个 Renderer 到 registerDefaults 里，
// **不碰消息流**（dsconversation 不知道任何具体工具名）。

// bashRenderer 是 bash 的定制画法：命令 + 退出码 + 输出预览。
//
// 收起时和别的工具一样只有一行——那是消息流里最密的地方，
// 一次会话几十条 bash，全展开等于没有消息流。
//
// 展开体用官方的 **IN/OUT 卡**（ToolRow.module.css:247-255 的 `.ioCard`，
// bash-sample.module.css:24-35 明写它也是 bash 的兜底画法）：4px 左缩进、
// 0.5px border-l1、radius 16、代码块底，每段 12px/16px 内边距；
// 命令与输出之间是官方 `.ioDivider`（0.5px border-l2，ToolRow.module.css:297-301）。
func bashRenderer() Renderer {
	return Renderer{
		Tool: "bash",
		Head: func(c Card) (string, string) {
			title, summary := defaultHead(c)
			if c.Summary != "" || viewOverridesArgs(c.View) {
				// 宿主摘要/结构化结果优先；bash 摘要里的 exit 原样保留。
				return title, summary
			}
			// 没有摘要：从参数里取命令，而不是把整段 JSON 摆上去。
			return title, command(c.Args)
		},
		Body: func(b *BodyCtx) {
			t := b.Env.Theme
			cmd := command(b.Card.Args)
			code, rest := splitExit(b.Card.Summary)
			if cmd == "" && code == "" && rest == "" {
				return
			}
			drawToolCard(b.Frame, t, func(f ui.C) {
				if cmd != "" {
					// `.ioText`：代码块字档 11/16（gradient-shadow-text.css:178-182），
					// 颜色 label-secondary（ToolRow.module.css:303-308）。
					drawIOSection(f, t, func(f ui.C) {
						ui.Text(f, cmd).Mono().FontSize(t.Fonts.Tiny).LineHeight(toolCodeLine).
							Color(t.LabelSecondary).MaxLines(6)
					})
				}
				if cmd != "" && (code != "" || rest != "") {
					drawIODivider(f, t)
				}
				switch {
				case code != "":
					drawIOSection(f, t, func(f ui.C) {
						ui.Row(f).WidthPercent(100).Gap(8).Items(ui.Center).Children(func() {
							color := t.Success
							if badExit(code) {
								color = t.Error
							}
							b.Badge(code, color)
							if rest != "" {
								ui.Text(f, rest).FontSize(t.Fonts.Tiny).LineHeight(toolCodeLine).
									Color(t.LabelSecondary).MaxLines(6)
							}
						})
					})
				case rest != "":
					// 拆不出退出码：原样显示摘要，不硬造一个"exit 0"。
					drawIOSection(f, t, func(f ui.C) {
						ui.Text(f, rest).FontSize(t.Fonts.Tiny).LineHeight(toolCodeLine).
							Color(t.LabelSecondary).MaxLines(12)
					})
				}
			})
		},
	}
}

// fileSpec 描述一种"文件类"工具的参数形状。
//
// 它们共用一套画法（路径 / 模式 + 摘要），差别只在**认哪些键**：
// read/edit 认 filePath，glob/grep 认 pattern。这不是省事，
// 是因为这几类工具在界面上本来就该长得一样——用户扫的是"动了哪个文件"。
type fileSpec struct {
	tool string
	// primary 是"宿主没给摘要时，标题行该显示哪一段"。按可能性从高到低。
	primary []string
	// keys 是展开体里按顺序列出的字段。
	keys []string
}

// fileRenderers 返回文件类工具的分派项。路径同时消费现有 filePath 与真实工具
// 参数 file_path；read/write/edit 都保留 offset/limit 行范围真值。
// edit 只有 old/new 两类字段同时存在时才画真实旧/新内容，缺任一方都不伪造 diff。
func fileRenderers() []Renderer {
	specs := []fileSpec{
		{tool: "read", primary: []string{"filePath", "file_path", "path", "file"}, keys: []string{"filePath", "file_path", "path", "file", "offset", "limit"}},
		{tool: "edit", primary: []string{"filePath", "file_path", "path", "file"}, keys: []string{"filePath", "file_path", "path", "file", "offset", "limit"}},
		{tool: "write", primary: []string{"filePath", "file_path", "path", "file"}, keys: []string{"filePath", "file_path", "path", "file", "offset", "limit"}},
		{tool: "list", primary: []string{"path", "dir"}, keys: []string{"path", "dir"}},
		{tool: "glob", primary: []string{"pattern", "glob"}, keys: []string{"pattern", "path", "glob"}},
		{tool: "grep", primary: []string{"pattern", "query"}, keys: []string{"pattern", "query", "path", "glob", "include"}},
	}
	out := make([]Renderer, 0, len(specs))
	for _, spec := range specs {
		out = append(out, fileRenderer(spec))
	}
	return out
}

// fileRenderer 造一种文件类工具的画法。
func fileRenderer(spec fileSpec) Renderer {
	return Renderer{
		Tool: spec.tool,
		Head: func(c Card) (string, string) {
			title, summary := defaultHead(c)
			if c.Summary != "" || viewOverridesArgs(c.View) {
				return title, summary
			}
			// 没有宿主结果：显示参数里最"像标题"的那一个（路径或模式）。
			if p := firstArg(parseArgs(c.Args), spec.primary...); p != "" {
				return title, p
			}
			return title, summary
		},
		Body: func(b *BodyCtx) {
			env := b.Env
			t := env.Theme
			args := parseArgs(b.Card.Args)
			if len(args) == 0 && b.Card.Args == "" && b.Card.Summary == "" {
				return
			}
			// 官方参数面板：`ToolDetails.root` 外壳 + `.item` 的 10px/14px 内边距
			// + `.field` 的 5.5em/12px 栅格（ToolDetails.module.css:1-9,19-23,133-141）。
			drawToolCard(b.Frame, t, func(f ui.C) {
				drawDetailsItem(f, t, func(f ui.C) {
					shown := 0
					ui.Column(f).FillWidth().Gap(toolFieldRowGap).Children(func() {
						for _, key := range spec.keys {
							value := args[key]
							if value == "" {
								continue
							}
							shown++
							drawField(f, env, argLabel(key), value)
						}
					})
					if shown == 0 && b.Card.Args != "" {
						// 参数不是 JSON（或没有认得的键）：退回原文，别把内容吞掉。
						ui.Text(f, b.Card.Args).Mono().FontSize(t.Fonts.Tiny).
							LineHeight(toolCodeLine).Color(t.LabelCaption).MaxLines(6)
					}
					if spec.tool == "edit" {
						drawEditDiff(f, env, args)
					}
					if b.Card.Summary != "" {
						// 官方 `.subtitle`：12px、label-tertiary、margin-top 3px
						// （ToolDetails.module.css:176-181）。
						ui.Text(f, b.Card.Summary).FontSize(toolSubtitleSize).
							LineHeight(toolFieldLine).Color(t.LabelTertiary).
							MarginSides(toolSubtitleGap, 0, 0, 0).MaxLines(12)
					}
				})
			})
		},
	}
}

// drawEditDiff 只展示参数里真实存在的旧/新内容，不计算或编造宿主未提供的
// unified diff、行号、增删统计。两个键同时缺失或只来一个时都整段隐藏。
//
// 几何取官方参数面板的两条：标签是 `.field dt`（13/20、label-caption，
// ToolDetails.module.css:143-145），内容是面板里的代码档（`.code` font-size:13px，
// ToolDetails.module.css:229-233）。
func drawEditDiff(f ui.C, env *pluginapi.Env, args map[string]string) {
	oldValue, hasOld := presentArg(args, "old_string", "oldString")
	newValue, hasNew := presentArg(args, "new_string", "newString")
	if !hasOld || !hasNew {
		return
	}
	t := env.Theme
	for _, field := range []struct {
		label string
		value string
		color ui.Color
	}{
		{label: "旧内容", value: oldValue, color: t.LabelTertiary},
		{label: "新内容", value: newValue, color: t.LabelSecondary},
	} {
		ui.Column(f).FillWidth().Gap(2).MarginSides(toolFieldRowGap, 0, 0, 0).Children(func() {
			ui.Text(f, field.label).FontSize(t.Fonts.Secondary).LineHeight(toolFieldLine).
				Color(t.LabelCaption)
			ui.Text(f, field.value).Mono().FontSize(t.Fonts.Code).LineHeight(toolFieldLine).
				Color(field.color).MaxLines(6)
		})
	}
}

// argLabel 是参数名的中文标签。键是宿主真会给的那些名字（见 fileSpec）。
func argLabel(key string) string {
	switch key {
	case "filePath", "file_path", "path", "file", "dir":
		return "路径"
	case "pattern":
		return "模式"
	case "query":
		return "查询"
	case "glob", "include":
		return "范围"
	case "offset":
		return "起始行"
	case "limit":
		return "行数"
	}
	return key
}
