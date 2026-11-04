package dscomposer

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/flex"
)

// mention.go 是 `@` 文件引用：在输入框里敲 `@` 之后的路径，弹一串候选，选中就把
// 路径补全进草稿。
//
// # 官方对应物，以及我们**没有**做的那一半
//
// 官方是两个包：`dsh-client-ui-input-trigger`（"Input trigger pipeline: '/' and '@'
// detection, candidate menu, pick routing to registered sources"）和
// `dsh-client-ui-reference`（"Unified Web @file and @session reference source"）。
//
// 我们只做**前半**：识别 + 候选 + 补全。后半——把 `@path` 变成结构化的引用块
// （带附件身份、会话引用、送进宿主的独立字段）——**没做**，因为那要宿主侧先有
// "附件/引用"这个概念，而现在的 Transport 契约里没有。所以：
//
//   - 补全出来的 `@路径` 是**纯文本**，跟着 prompt 一起发出去；
//   - 模型看得见它，能不能用上取决于它自己会不会去 read。
//
// 这一点必须写清楚，不能把它标成"做到了 @ 引用"——**补全不是引用**。
// parity 里这一项记 🟡，不记 ✅。

// maxMentionRows 是候选菜单最多画几行。
//
// 6 是"一眼扫完"的量：再多就得滚，而输入框上方弹一个要滚的列表，
// 用户会先按 Esc 把它关掉——那这个功能就白做了。
const maxMentionRows = 6

// atRef 是草稿里"正在敲的那一个 @ 引用"。
type atRef struct {
	// Start 是 '@' 在文本里的下标（字节）。
	Start int
	// Raw 是 '@' 之后到文本末尾的那一段（不含空白）。
	Raw string
	// Dir 是 Raw 里最后一个 '/' 之前的部分，**含尾部 '/'**；没有 '/' 时是 ""。
	Dir string
	// Prefix 是最后一个 '/' 之后的部分，也就是正在敲的名字前缀。
	Prefix string
}

// parseAt 从草稿里认出正在敲的 @ 引用。
//
// 只认**末尾**那一个 token，而且 '@' 必须在开头或前面是空白：
//
//   - `看下 @tmp/a`     → 认出来（正在敲路径）
//   - `发到 a@b.com`    → **不认**（那是邮箱，'@' 前面不是空白）
//   - `@tmp/a 然后呢`   → 不认（token 已经被空格结束了，用户敲完了）
//
// 第三条是关键：不认它的话，用户打完路径接着打字时菜单会一直挂着，
// 而 Enter 会被我们吃掉去"补全"——他**永远发不出这条消息**。
func parseAt(text string) (atRef, bool) {
	i := -1
	for k := len(text) - 1; k >= 0; k-- {
		if text[k] != '@' {
			continue
		}
		if k == 0 || isSpaceByte(text[k-1]) {
			i = k
			break
		}
	}
	if i < 0 {
		return atRef{}, false
	}
	raw := text[i+1:]
	if strings.ContainsAny(raw, " \t\r\n") {
		return atRef{}, false
	}
	ref := atRef{Start: i, Raw: raw}
	if k := strings.LastIndex(raw, "/"); k >= 0 {
		ref.Dir = raw[:k+1]
		ref.Prefix = raw[k+1:]
	} else {
		ref.Prefix = raw
	}
	return ref, true
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// completeAt 把候选补进草稿：替换掉那个 token，其余原样保留。
//
// isDir 决定要不要补一个尾斜杠——补上之后菜单自然进到下一层，
// 用户少按一次键，也不会出现"补全了却发现还得自己打斜杠"。
func completeAt(text string, ref atRef, name string, isDir bool) string {
	repl := "@" + ref.Dir + name
	if isDir {
		repl += "/"
	}
	return text[:ref.Start] + repl
}

// filterMentions 按前缀筛候选：目录在前，然后按名字排（大小写不敏感）。
//
// 大小写不敏感是刻意的：Windows 上 `@SRC` 和 `@src` 指同一个目录，
// 而这是个 Windows 优先的产品。
func filterMentions(entries []pluginapi.FSEntry, prefix string) []pluginapi.FSEntry {
	if len(entries) == 0 {
		return nil
	}
	low := strings.ToLower(prefix)
	out := make([]pluginapi.FSEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e.Name), low) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	// 一个都没匹配时返回 **nil** 而不是空切片：调用方拿它判断"要不要画菜单"，
	// nil 是"没有"，空切片是"有、但一个都没有"——后者会让读的人多停一秒。
	if len(out) == 0 {
		return nil
	}
	return out
}

// dropExactFile 去掉"已经完整敲出来的那个文件"。
//
// 为什么必须去掉：`@plugins/dsopen/open.go` 敲全之后，候选里正好有它自己，
// 于是菜单一直挂着、Enter 一直被我们吃掉去"补全"（补成同一个字符串），
// **这条消息就永远发不出去**。
//
// 只对文件这么做，目录不这么做：`@plugins/dsopen/` 是"进到这一层"，
// 菜单该继续显示里面的东西。
func dropExactFile(cands []pluginapi.FSEntry, prefix string) []pluginapi.FSEntry {
	if len(cands) != 1 {
		return cands
	}
	if !cands[0].IsDir && cands[0].Name == prefix {
		return nil
	}
	return cands
}

// mentionState 是这一帧算出来的候选状态。
type mentionState struct {
	Ref  atRef
	Rows []pluginapi.FSEntry
}

// computeMention 算出这一帧该不该弹菜单、弹哪几个。
//
// 数据来源是 `Actions.FSList`——**和侧栏文件树同一个接口**，没有新契约。
// 它是壳的缓存：第一次问某个目录**必然返回空**，下一帧才有。
// 所以"这一帧没有候选"不代表"这个目录是空的"。这里按"没有"处理（不画），
// 不写"没有匹配"那种编出来的话。
func (p *Plugin) computeMention(env *pluginapi.Env) (mentionState, bool) {
	if env.Act.FSList == nil || env.Data.CWD == "" {
		return mentionState{}, false
	}
	ref, ok := parseAt(p.field.Text())
	if !ok {
		return mentionState{}, false
	}
	dir := env.Data.CWD
	if ref.Dir != "" {
		dir = strings.TrimRight(dir, "/") + "/" + strings.TrimSuffix(ref.Dir, "/")
	}
	rows := dropExactFile(filterMentions(env.Act.FSList(dir), ref.Prefix), ref.Prefix)
	if len(rows) == 0 {
		return mentionState{}, false
	}
	if len(rows) > maxMentionRows {
		rows = rows[:maxMentionRows]
	}
	return mentionState{Ref: ref, Rows: rows}, true
}

// drawMention 画候选菜单。
//
// 位置在输入卡片**上方**（和 GoalBar / 问题条同一条约定：越靠近输入框的东西越紧急）。
// 每行都能点。
func (p *Plugin) drawMention(f *flex.Frame, env *pluginapi.Env, column float32) {
	st, ok := p.computeMention(env)
	if !ok {
		p.mentionRows, p.mentionRef = nil, atRef{}
		return
	}
	p.mentionRows, p.mentionRef = st.Rows, st.Ref
	t := p.theme
	w := cardWidth(column)
	f.Row().FillWidth().Justify(flex.Center).Children(func() {
		f.Column().Width(w).Radius(t.Radii.SM).Background(t.BgLayer1).
			PadY(4).Children(func() {
			for i, e := range st.Rows {
				p.drawMentionRow(f, env, e, i == 0)
			}
		})
	})
}

// drawMentionRow 画一行候选。
//
// 第一行高亮并带 Enter 提示——因为 Enter 补的就是它。不把这件事说出来，
// 用户只会觉得"我按了 Enter 怎么变成这样了"。
func (p *Plugin) drawMentionRow(f *flex.Frame, env *pluginapi.Env, e pluginapi.FSEntry, first bool) {
	t := p.theme
	icon, kind := "·", "文件"
	if e.IsDir {
		icon, kind = "▸", "目录"
	}
	bg := t.BgLayer1
	hint := ""
	if first {
		bg = t.Hover
		hint = "Enter"
	}
	// 行是**值类型盒子**，`.Clicked()` 直接问它这一帧有没有被点。
	row := f.Row().FillWidth().MinHeight(24).PadX(8).Items(flex.Center).Gap(8).
		Background(bg).Hit(env.Tree.ID(env.Feat, "mention-"+e.Name), "").
		Children(func() {
			f.Text(icon).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			f.Text(e.Name).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
			f.Text(kind).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			f.Text(hint).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		})
	if row.Clicked() {
		p.field.SetText(completeAt(p.field.Text(), p.mentionRef, e.Name, e.IsDir))
	}
}

// acceptMention 是键盘那条路：把第一候选补进草稿。
//
// 返回 false 表示"没有菜单可收"，调用方必须**把按键让回去**——
// 吃掉一个没有对应操作的 Enter，用户就永远发不出消息。
func (p *Plugin) acceptMention() bool {
	if len(p.mentionRows) == 0 {
		return false
	}
	e := p.mentionRows[0]
	p.field.SetText(completeAt(p.field.Text(), p.mentionRef, e.Name, e.IsDir))
	return true
}
