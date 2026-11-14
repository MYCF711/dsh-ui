package dscomposer

// trigger.go —— 官方 `dsh-client-ui-input-trigger` 的输入触发管道。
//
// 官方是"根服务持有 source 名单、每个会话一个 controller"：
//
//   - `core/detect.ts` 负责在光标处认出 `@` / `/` token（含 URL 的 `/` carve-out）；
//   - source 注册表（`ui-reference` 的 `@`、`ui-skill` 的 `/`）各自给候选；
//   - `onPick` 返回 `{insert}` 或 `{text, continue}`（目录 drill 用后者）；
//   - 菜单按 source 分组，`source-failed` 只静默移除该组。
//
// 本仓此前只有一条硬编码的 `@` 文件路径补全（mention.go）。这一版把骨架补上：
//
//	detectTrigger   @ / /（最后的 token，含 URL carve-out 与引号路径）
//	source 名单      文件（fileReferences.list，fallback 到老的 FSList）
//	                会话（sessionReferenceResolver.candidates）
//	                技能（skills.list）
//	分组菜单 + 选中    group title 用官方 zh 字典；Tab = 目录 drill；Enter = 选中
//
// 仍然**没有**做的：官方 editor 把引用做成结构化 chip 节点（`ReferenceChipNode`），
// 这里输入框是纯文本，所以 pick 的结果是官方 wire 的文本形式
//（`@path` / `@[label](dsh-session:id)` / `/name `）。用户消息渲染层
//（`plugins/dsconversation/usertext.go`）再把这些文本识别回 chip——两边用的是
// 同一套官方 wire 语法，但**发送出去的仍是纯文本**，这一点在 parity 里必须写清楚。

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/reficon"
	"github.com/vivaui/viva/ui"
)

// triggerGroupMaxRows 是整个菜单最多画几行（跨 source 合计）。
// 与原来的 maxMentionRows 同一个理由：输入框上方弹一个要滚的列表没人用。
const triggerGroupMaxRows = 8

// triggerHit 是光标/草稿末尾处的触发 token。
type triggerHit struct {
	// Char 是 '@' 或 '/'。
	Char byte
	// Start 是触发字符的字节下标。
	Start int
	// Query 是触发字符之后到文本末尾的那一段。
	Query string
	// Quoted 表示这是一个未闭合的 `@"…`（路径里可以有空格）。
	Quoted bool
}

// triggerCandidate 是菜单里的一行。
type triggerCandidate struct {
	// Source 是官方 source 名（file / session / skill）。
	Source string
	Label  string
	Detail string
	// Insert 是 pick 之后替换整个 token 的文本（官方 wire 的 mention 形式）。
	Insert string
	// Drill 表示这一行是目录：pick 后继续留在菜单里往里走。
	Drill bool
	// Icon 是 chip 图标种类；空 = 不画图标（技能行按官方菜单只画名字）。
	Icon string
	// UserOnly 对应官方 skill 的 `menu.userOnly`「仅用户」角标。
	UserOnly bool
}

// triggerGroup 是一个 source 的一组候选。
type triggerGroup struct {
	Title string
	Rows  []triggerCandidate
}

// triggerMenu 是这一帧算出来的菜单。
type triggerMenu struct {
	Hit    triggerHit
	Groups []triggerGroup
}

// rows 把分组摊平成菜单行（按键/高亮按这个下标）。
func (m triggerMenu) rows() []triggerCandidate {
	var out []triggerCandidate
	for _, g := range m.Groups {
		out = append(out, g.Rows...)
	}
	return out
}

// detectTrigger 从草稿末尾认一个触发 token。
//
// 只认**最后一个** token（本仓没有 caret 通道，输入框的光标默认在末尾；
// 这与旧 mention 的行为一致）。规则逐条对齐官方 `detect.ts`：
//
//   - `@`：必须出现在行首、空白之后或标点之后（`a@b.com` 不算）；
//   - `@"…"`：未闭合的引号 token 允许空格；
//   - `/`：向前扫到空白；URL carve-out（`://` 与 `//` 里的斜杠）不触发；
//   - 两个都命中时取更靠后的那个。
func detectTrigger(text string) (triggerHit, bool) {
	at, atOK := detectAtTrigger(text)
	slash, slashOK := detectSlashTrigger(text)
	switch {
	case atOK && slashOK:
		if slash.Start > at.Start {
			return slash, true
		}
		return at, true
	case atOK:
		return at, true
	case slashOK:
		return slash, true
	}
	return triggerHit{}, false
}

func detectAtTrigger(text string) (triggerHit, bool) {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] != '@' {
			continue
		}
		if i > 0 && !atBoundary(text[i-1]) {
			continue
		}
		raw := text[i+1:]
		if strings.HasPrefix(raw, `"`) {
			if strings.Contains(raw[1:], `"`) {
				return triggerHit{}, false // 引号已闭合：token 结束了
			}
			if strings.ContainsAny(raw[1:], "\r\n") {
				return triggerHit{}, false
			}
			return triggerHit{Char: '@', Start: i, Query: raw[1:], Quoted: true}, true
		}
		if strings.ContainsAny(raw, " \t\r\n") {
			return triggerHit{}, false
		}
		return triggerHit{Char: '@', Start: i, Query: raw}, true
	}
	return triggerHit{}, false
}

func detectSlashTrigger(text string) (triggerHit, bool) {
	// 逐字符向前扫（官方 detect.ts 的做法）：遇到空白就停；每个 '/' 单独判边界，
	// 这样 `src/foo` 里的 '/' 会被 word 边界拒掉，而 URL 的两个 carve-out 也
	// 只在它们该生效的那个斜杠上生效。
	for i := len(text) - 1; i >= 0; i-- {
		c := text[i]
		if c != '/' {
			if isSpaceByte(c) {
				return triggerHit{}, false
			}
			continue
		}
		// `//` 的第二个斜杠：不触发。
		if i > 0 && text[i-1] == '/' {
			continue
		}
		// `scheme:/` 的斜杠（冒号前不是空白）：不触发。
		if i >= 2 && text[i-1] == ':' && !isSpaceByte(text[i-2]) {
			continue
		}
		if i > 0 && !atBoundary(text[i-1]) {
			continue
		}
		query := text[i+1:]
		if strings.ContainsAny(query, " \t\r\n") {
			return triggerHit{}, false
		}
		return triggerHit{Char: '/', Start: i, Query: query}, true
	}
	return triggerHit{}, false
}

// atBoundary 是官方 `boundaryOk` 的字符类：空白、ASCII 标点、CJK。
func atBoundary(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '(', '[', '{', '"', '\'', '`', ',', '.':
		return true
	}
	return c >= 0x80
}

// computeTriggerMenu 算出这一帧该不该弹菜单、有哪些组。
func (p *Plugin) computeTriggerMenu(env *pluginapi.Env) (triggerMenu, bool) {
	text := p.field.Text()
	if p.suppressed != "" {
		if p.suppressed == text {
			p.clearTrigger()
			return triggerMenu{}, false
		}
		p.suppressed = ""
	}
	hit, ok := detectTrigger(text)
	if !ok {
		p.clearTrigger()
		return triggerMenu{}, false
	}
	switch hit.Char {
	case '@':
		return p.computeAtMenu(env, hit)
	case '/':
		return p.computeSlashMenu(env, hit)
	}
	p.clearTrigger()
	return triggerMenu{}, false
}

// computeAtMenu 组装 `@` 的文件 + 会话两组候选。
func (p *Plugin) computeAtMenu(env *pluginapi.Env, hit triggerHit) (triggerMenu, bool) {
	var groups []triggerGroup
	// 文件：优先官方 fileReferences.list；老壳没有这条动作时退回 FSList 目录树。
	if env.Act.FileReferences != nil && env.Data.Cur != "" {
		if rows := p.fileSourceRows(env, hit); len(rows) > 0 {
			groups = append(groups, triggerGroup{Title: zhTriggerFiles, Rows: rows})
		}
	} else if env.Act.FSList != nil && env.Data.CWD != "" {
		if rows := p.legacyFileRows(env, hit); len(rows) > 0 {
			groups = append(groups, triggerGroup{Title: zhTriggerFiles, Rows: rows})
			// 兼容字段：旧测试和旧调用方读 mentionRows；统一菜单仍以 triggerRows 为准。
			p.mentionRows = legacyEntries(rows)
			p.mentionRef = atRef{Start: hit.Start, Raw: hit.Query, Prefix: hit.Query}
		}
	}
	// 会话：官方在引号 `@"…"` 下不列会话（那是路径专用 token）。
	if !hit.Quoted && env.Act.SessionReferenceCandidates != nil && env.Data.Cur != "" {
		if rows := sessionSourceRows(env, hit); len(rows) > 0 {
			groups = append(groups, triggerGroup{Title: zhTriggerSessions, Rows: rows})
		}
	}
	groups = trimGroups(groups)
	if len(groups) == 0 {
		p.clearTrigger()
		return triggerMenu{}, false
	}
	if p.triggerHit != hit {
		p.triggerIndex = 0
	}
	menu := triggerMenu{Hit: hit, Groups: groups}
	p.triggerRows = menu.rows()
	p.triggerHit = hit
	if p.triggerIndex >= len(p.triggerRows) {
		p.triggerIndex = 0
	}
	return menu, true
}

// computeSlashMenu 组装 `/` 的技能候选。
func (p *Plugin) computeSlashMenu(env *pluginapi.Env, hit triggerHit) (triggerMenu, bool) {
	if env.Act.Skills == nil || env.Data.Cur == "" {
		p.clearTrigger()
		return triggerMenu{}, false
	}
	rows := skillSourceRows(env, hit)
	if len(rows) == 0 {
		p.clearTrigger()
		return triggerMenu{}, false
	}
	if p.triggerHit != hit {
		p.triggerIndex = 0
	}
	menu := triggerMenu{Hit: hit, Groups: []triggerGroup{{Title: zhTriggerSkills, Rows: rows}}}
	p.triggerRows = menu.rows()
	p.triggerHit = hit
	if p.triggerIndex >= len(p.triggerRows) {
		p.triggerIndex = 0
	}
	return menu, true
}

// fileSourceRows 是官方 `fileReferences.list` 的候选。
func (p *Plugin) fileSourceRows(env *pluginapi.Env, hit triggerHit) []triggerCandidate {
	list := env.Act.FileReferences(env.Data.Cur, hit.Query)
	if len(list) == 0 {
		return nil
	}
	out := make([]triggerCandidate, 0, len(list))
	for _, item := range list {
		path := strings.TrimPrefix(item.Path, "./")
		if path == "" {
			continue
		}
		label := path
		if idx := strings.LastIndexAny(path, `/\`); idx >= 0 {
			label = path[idx+1:]
		}
		if item.IsDir {
			out = append(out, triggerCandidate{
				Source: "file", Label: label + "/", Detail: path,
				Insert: "@" + path + "/", Drill: true, Icon: "folder",
			})
			continue
		}
		out = append(out, triggerCandidate{
			Source: "file", Label: label, Detail: path,
			Insert: fileMention(path), Icon: "file",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := out[i].Insert != "" && strings.HasSuffix(out[i].Insert, "/")
		dj := out[j].Insert != "" && strings.HasSuffix(out[j].Insert, "/")
		if di != dj {
			return di
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	if len(out) > triggerGroupMaxRows {
		out = out[:triggerGroupMaxRows]
	}
	return out
}

// legacyFileRows 是"宿主没有 fileReferences.list"时的目录树 fallback（原 mention）。
func (p *Plugin) legacyFileRows(env *pluginapi.Env, hit triggerHit) []triggerCandidate {
	ref, ok := parseAt(p.field.Text())
	if !ok {
		return nil
	}
	dir := env.Data.CWD
	if ref.Dir != "" {
		dir = strings.TrimRight(dir, "/") + "/" + strings.TrimSuffix(ref.Dir, "/")
	}
	rows := dropExactFile(filterMentions(env.Act.FSList(dir), ref.Prefix), ref.Prefix)
	if len(rows) == 0 {
		return nil
	}
	if len(rows) > triggerGroupMaxRows {
		rows = rows[:triggerGroupMaxRows]
	}
	out := make([]triggerCandidate, 0, len(rows))
	for _, e := range rows {
		icon := "file"
		if e.IsDir {
			icon = "folder"
		}
		out = append(out, triggerCandidate{
			Source: "file", Label: e.Name, Detail: e.Name,
			Insert: completeAt(p.field.Text(), ref, e.Name, e.IsDir),
			Drill:  e.IsDir,
			Icon:   icon,
		})
	}
	return out
}

// sessionSourceRows 是官方 `sessionReferenceResolver.candidates` 的候选。
func sessionSourceRows(env *pluginapi.Env, hit triggerHit) []triggerCandidate {
	list := env.Act.SessionReferenceCandidates(env.Data.Cur, hit.Query)
	if len(list) == 0 {
		return nil
	}
	out := make([]triggerCandidate, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		if item.SessionID == "" || item.Mention == "" || seen[item.SessionID] {
			continue
		}
		seen[item.SessionID] = true
		label := strings.TrimSpace(item.DisplayTitle)
		if label == "" {
			label = strings.TrimSpace(item.Label)
		}
		if label == "" {
			label = item.SessionID
		}
		out = append(out, triggerCandidate{
			Source: "session", Label: label, Detail: item.Cwd,
			Insert: item.Mention, Icon: "session",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	if len(out) > triggerGroupMaxRows {
		out = out[:triggerGroupMaxRows]
	}
	return out
}

// skillSourceRows 是官方 `skills.list` 的 `/` 候选（大小写不敏感前缀 + 名字序）。
func skillSourceRows(env *pluginapi.Env, hit triggerHit) []triggerCandidate {
	list := env.Act.Skills(env.Data.Cur)
	if len(list) == 0 {
		return nil
	}
	low := strings.ToLower(hit.Query)
	out := make([]triggerCandidate, 0, len(list))
	for _, item := range list {
		if item.Name == "" {
			continue
		}
		if low != "" && !strings.HasPrefix(strings.ToLower(item.Name), low) {
			continue
		}
		out = append(out, triggerCandidate{
			Source: "skill", Label: item.Name, Detail: item.Description,
			// 官方 ui-skill：pick 落 `/<name> `（含尾空格）。
			Insert:   "/" + item.Name + " ",
			UserOnly: !item.ModelInvocable,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	if len(out) > triggerGroupMaxRows {
		out = out[:triggerGroupMaxRows]
	}
	return out
}

// trimGroups 把跨组合计行数截到 triggerGroupMaxRows（保持组顺序）。
func trimGroups(groups []triggerGroup) []triggerGroup {
	left := triggerGroupMaxRows
	var out []triggerGroup
	for _, g := range groups {
		if left <= 0 {
			break
		}
		if len(g.Rows) > left {
			g.Rows = g.Rows[:left]
		}
		if len(g.Rows) > 0 {
			out = append(out, g)
			left -= len(g.Rows)
		}
	}
	return out
}

func fileMention(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `@"` + path + `"`
	}
	return "@" + path
}

// legacyEntries 把统一候选转回旧 `FSList` 形态（只为兼容旧字段/旧测试）。
func legacyEntries(rows []triggerCandidate) []pluginapi.FSEntry {
	out := make([]pluginapi.FSEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, pluginapi.FSEntry{Name: strings.TrimSuffix(r.Label, "/"), IsDir: r.Drill})
	}
	return out
}

// clearTrigger 清空这一帧的菜单状态。
func (p *Plugin) clearTrigger() {
	p.triggerRows = nil
	p.triggerHit = triggerHit{}
	p.triggerIndex = 0
	p.mentionRows = nil
	p.mentionRef = atRef{}
}

// drawMention 画统一触发菜单（组标题 + 行）。
func (p *Plugin) drawMention(f ui.C, env *pluginapi.Env, column float32) {
	menu, ok := p.computeTriggerMenu(env)
	if !ok {
		return
	}
	t := p.theme
	w := cardWidth(column)
	ui.Row(f).FillWidth().Justify(ui.Center).Children(func() {
		ui.Column(f).Width(w).Radius(t.Radii.SM).Background(t.BgLayer1).PadY(4).Children(func() {
			index := 0
			for _, group := range menu.Groups {
				if len(menu.Groups) > 1 {
					ui.Text(f, group.Title).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).PadX(8)
				}
				for _, row := range group.Rows {
					p.drawTriggerRow(f, env, row, index)
					index++
				}
			}
		})
	})
}

// drawTriggerRow 画一行候选。
func (p *Plugin) drawTriggerRow(f ui.C, env *pluginapi.Env, c triggerCandidate, index int) {
	t := p.theme
	selected := index == p.triggerIndex
	bg := t.BgLayer1
	hint := ""
	if selected {
		bg = t.Hover
		hint = "Enter"
	}
	if c.Drill {
		hint = "Tab"
	}
	hit := "mention-" + c.Label
	if c.Source != "file" {
		hit = "trigger-" + c.Source + "-" + itoaComposer(index)
	}
	row := ui.Row(f).FillWidth().MinHeight(24).PadX(8).Items(ui.Center).Gap(8).
		Background(bg).Hit(env.Tree.ID(env.Feat, hit), "").
		Children(func() {
			switch c.Icon {
			case "folder":
				ui.Icon(f, reficon.Icon(reficon.Folder, t.Info)).Width(14).Height(14)
			case "file":
				ui.Icon(f, reficon.Icon(reficon.File, t.Info)).Width(14).Height(14)
			case "session":
				ui.Icon(f, reficon.Icon(reficon.Session, t.Info)).Width(14).Height(14)
			}
			ui.Text(f, c.Label).FontSize(t.Fonts.Secondary).Color(t.LabelSecondary).Ellipsis()
			if c.UserOnly {
				ui.Text(f, zhTriggerUserOnly).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
			}
			if c.Detail != "" && c.Detail != c.Label {
				ui.Text(f, c.Detail).FontSize(t.Fonts.Tiny).Color(t.LabelCaption).Ellipsis()
			}
			ui.Text(f, hint).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		})
	if row.Clicked() {
		p.applyCandidate(c)
	}
}

// applyCandidate 把一行候选落进草稿。
func (p *Plugin) applyCandidate(c triggerCandidate) {
	text := p.field.Text()
	hit := p.triggerHit
	if hit.Start < 0 || hit.Start > len(text) {
		return
	}
	newText := text[:hit.Start] + c.Insert
	p.field.SetText(newText)
	if !c.Drill {
		p.suppressed = newText
		p.clearTrigger()
		return
	}
	// 目录 drill：保留菜单，下一帧按新 query 继续列下一层。
	p.triggerIndex = 0
}

// acceptMention 是键盘 Enter/Tab 那条路：落选中行。
//
// 返回 false 表示"没有菜单可收"，调用方必须把按键让回去——
// 吃掉一个没有对应操作的 Enter，用户就永远发不出消息。
func (p *Plugin) acceptMention() bool {
	if len(p.triggerRows) == 0 {
		// 兼容旧路径：测试/旧调用方只塞了 mentionRows 时仍按老语义补全。
		if len(p.mentionRows) > 0 {
			e := p.mentionRows[0]
			p.field.SetText(completeAt(p.field.Text(), p.mentionRef, e.Name, e.IsDir))
			p.clearTrigger()
			return true
		}
		return false
	}
	index := p.triggerIndex
	if index < 0 || index >= len(p.triggerRows) {
		index = 0
	}
	p.applyCandidate(p.triggerRows[index])
	return true
}

// moveTrigger 上下移动选中行（官方 `move` 事件）。
func (p *Plugin) moveTrigger(delta int) bool {
	if len(p.triggerRows) == 0 {
		return false
	}
	n := len(p.triggerRows)
	p.triggerIndex = (p.triggerIndex + delta%n + n) % n
	return true
}

// drillTrigger 是 Tab：目录候选往里走，文件/会话/技能等同 Enter。
func (p *Plugin) drillTrigger() bool {
	if len(p.triggerRows) == 0 {
		return false
	}
	index := p.triggerIndex
	if index < 0 || index >= len(p.triggerRows) {
		index = 0
	}
	c := p.triggerRows[index]
	p.applyCandidate(c)
	return true
}

// dismissTrigger 关掉菜单且不让同一份文本立刻重开（官方 dismissal 语义）。
func (p *Plugin) dismissTrigger() bool {
	if len(p.triggerRows) == 0 && len(p.mentionRows) == 0 {
		return false
	}
	p.suppressed = p.field.Text()
	p.clearTrigger()
	return true
}

func itoaComposer(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// 官方 zh 文案（`ui-reference/src/client/locales.ts:16-18`、
// `ui-input-trigger/src/client/locales.ts` 的 `skill`）。
const (
	zhTriggerFiles    = "文件与文件夹"
	zhTriggerSessions = "对话"
	zhTriggerSkills   = "技能"
	// 官方 `ui-skill/src/client/locales.ts` 的 menu.userOnly。
	zhTriggerUserOnly = "仅用户"
)
