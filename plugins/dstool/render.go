package dstool

import (
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
			if cmd := command(b.Card.Args); cmd != "" {
				b.Frame.Text(cmd).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelCaption).MaxLines(6)
			}
			code, rest := splitExit(b.Card.Summary)
			switch {
			case code != "":
				b.Frame.Row().WidthPercent(100).Gap(8).Items(ui.Center).Children(func() {
					color := t.Success
					if badExit(code) {
						color = t.Error
					}
					b.Badge(code, color)
					if rest != "" {
						b.Frame.Text(rest).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(6)
					}
				})
			case rest != "":
				// 拆不出退出码：原样显示摘要，不硬造一个"exit 0"。
				b.Frame.Text(rest).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(12)
			}
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
			t := b.Env.Theme
			cw := contentWidth(b.Env)
			args := parseArgs(b.Card.Args)
			shown := 0
			for _, key := range spec.keys {
				value := args[key]
				if value == "" {
					continue
				}
				shown++
				label := argLabel(key)
				b.Frame.Row().WidthPercent(100).Gap(8).Children(func() {
					b.Frame.Text(label).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
					// MaxWidth 是 Ellipsis 的边界：不给的话长路径会画到窗口外
					// （和 drawCard 同一个坑，实测过）。
					b.Frame.Text(value).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelSecondary).
						MaxWidth(cw - 120).Ellipsis()
				})
			}
			if shown == 0 && b.Card.Args != "" {
				// 参数不是 JSON（或没有认得的键）：退回原文，别把内容吞掉。
				b.Frame.Text(b.Card.Args).Mono().FontSize(t.Fonts.Tiny).Color(t.LabelCaption).MaxLines(6)
			}
			if spec.tool == "edit" {
				drawEditDiff(b, args)
			}
			if b.Card.Summary != "" {
				b.Frame.Text(b.Card.Summary).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).MaxLines(12)
			}
		},
	}
}

// drawEditDiff 只展示参数里真实存在的旧/新内容，不计算或编造宿主未提供的
// unified diff、行号、增删统计。两个键同时缺失或只来一个时都整段隐藏。
func drawEditDiff(b *BodyCtx, args map[string]string) {
	oldValue, hasOld := presentArg(args, "old_string", "oldString")
	newValue, hasNew := presentArg(args, "new_string", "newString")
	if !hasOld || !hasNew {
		return
	}
	t := b.Env.Theme
	for _, field := range []struct {
		label string
		value string
		color ui.Color
	}{
		{label: "旧内容", value: oldValue, color: t.LabelTertiary},
		{label: "新内容", value: newValue, color: t.LabelSecondary},
	} {
		b.Frame.Column().WidthPercent(100).Gap(2).Children(func() {
			b.Frame.Text(field.label).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			b.Frame.Text(field.value).Mono().FontSize(t.Fonts.Tiny).Color(field.color).MaxLines(6)
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
