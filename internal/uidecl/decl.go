// Package uidecl 是「运行时声明式插件」通道：**一个 JSON 文件 = 一个只读界面部件**。
//
// 为什么要有它：内置插件（plugins/ds*）都写在 Go 里，加一块内容就要改代码、
// 重新编译、再发一次二进制；仓库外的人更是连壳都没有。这条通道把「挂一块只读内容」
// 从代码降级成文件——往目录里放一个 hello.json，重启界面就多一行，Go 一行都不用动。
//
// 边界刻意画得很窄，三条都要记牢：
//
//	只读   不接键盘、没有点击、不发请求、不持有状态（每次画都从快照现算）
//	只画   拿到的东西和普通插件一样（pluginapi.Env），颜色一律问 Theme，不写色号
//	不静默 槽位不认识、颜色不认识、JSON 坏了，一律报错并指出是哪个文件
//
// 最后一条是这个包最重要的设计。「写了但不会生效」是配置类通道最典型的坑：
// 第三方放了个文件、界面没变化、日志也没说话，于是以为是自己写错了、反复折腾。
// 所以这里凡是「写了却不生效」的情况都必须炸出来，宁可启动失败，也不许装死。
package uidecl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// ── 形状 ────────────────────────────────────────────────────────────────

// Kind 是部件的画法。刻意只给四种：这四种能把「一行只读状态」覆盖完，
// 再多就得写 Go 了——通道的定位是**降低门槛**，不是取代插件。
type Kind string

const (
	// KindLabel 是一行文字。
	KindLabel Kind = "label"
	// KindKeyValue 是左标签 + 右值（中间用 Spacer 顶开）。
	KindKeyValue Kind = "keyvalue"
	// KindDot 是状态点 + 一行文字。
	KindDot Kind = "dot"
	// KindList 是多行文字（text 用 "\n" 分行，或 items 给数组）。
	KindList Kind = "list"
)

// Kinds 返回支持的 kind（报错信息用，顺序固定便于测试）。
func Kinds() []string {
	return []string{string(KindDot), string(KindKeyValue), string(KindLabel), string(KindList)}
}

// 尺寸缺省值。22 是侧栏底部那行（dssidebar.drawFooter）的高度，
// 声明部件不写 height 时就照着它来，混在一起才对得齐。
const (
	defaultHeight   = 22
	defaultOrder    = 100
	listLineHeight  = 18
	listLineSpacing = 2
)

// Decl 是一份校验过的声明。字段名与 JSON 一一对应，多了 File 用于报错定位。
type Decl struct {
	// ID 是部件 id（全局唯一，建议带自己的前缀：hello.badge）。
	ID string
	// Slot 是挂载槽位，必须是 pluginapi.Slot* 之一。
	Slot pluginapi.Slot
	// Order 是同一个槽位里的先后（小的先画，= 贴在上面）。
	Order int
	// Kind 是画法。
	Kind Kind
	// Text 是主内容，支持 {sessions} 这类模板变量。
	Text string
	// Label 只在 kind=keyvalue 时用作左侧标签。
	Label string
	// Items 只在 kind=list 时用作多行内容（优先级高于 Text）。
	Items []string
	// Color 是取色名（label-primary / success / …），空 = 按 kind 取默认。
	Color string
	// Height 是这一块的高度（像素）；0 = 按 kind 的默认值。
	Height float32
	// File 是这份声明来自哪个文件（报错、日志用；不是 JSON 字段）。
	File string
}

// rawDecl 是 JSON 的原始形状。
//
// Order / Height 用指针是为了区分「没写」和「写了 0」：
// 没写 order 要落到 defaultOrder，写了 order:0 就是「画槽位最前面」，两者语义不同。
type rawDecl struct {
	ID      string   `json:"id"`
	Slot    string   `json:"slot"`
	Order   *int     `json:"order"`
	Kind    string   `json:"kind"`
	Text    string   `json:"text"`
	Label   string   `json:"label"`
	Items   []string `json:"items"`
	Color   string   `json:"color"`
	Height  *float32 `json:"height"`
	Comment string   `json:"comment"` // 允许写备注，不生效也不报错
}

// ── 槽位表 ──────────────────────────────────────────────────────────────

// slotEntry 是「JSON 里能写的槽位」到契约常量的映射。
//
// 名字是契约（pluginapi/plugin.go 的 Slot* 常量），这里只做一次翻译。
// 加新槽位时**先**加常量、**再**加这一行——两边漂移会让第三方挂到不存在的地方。
type slotEntry struct {
	// Const 是 Go 常量名，JSON 里也接受这个名字（方便照着文档抄）。
	Const string
	Slot  pluginapi.Slot
}

var slotTable = []slotEntry{
	{"SlotSidebarHeader", pluginapi.SlotSidebarHeader},
	{"SlotSidebarList", pluginapi.SlotSidebarList},
	{"SlotSidebarFooter", pluginapi.SlotSidebarFooter},
	{"SlotMainHeader", pluginapi.SlotMainHeader},
	{"SlotMainBody", pluginapi.SlotMainBody},
	{"SlotMainComposer", pluginapi.SlotMainComposer},
	{"SlotShellRoot", pluginapi.SlotShellRoot},
	{"SlotShellLeading", pluginapi.SlotShellLeading},
	{"SlotShellBottom", pluginapi.SlotShellBottom},
	{"SlotShellOverlay", pluginapi.SlotShellOverlay},
	{"SlotRightbar", pluginapi.SlotRightbar},
	{"SlotStatusbar", pluginapi.SlotStatusbar},
}

// Slots 返回可用槽位（按槽位字符串排序，报错信息用）。
//
// 排序是为了让报错稳定：测试与截图都能对比，人也容易扫。
func Slots() []string {
	out := make([]string, 0, len(slotTable))
	for _, e := range slotTable {
		out = append(out, string(e.Slot))
	}
	sort.Strings(out)
	return out
}

// lookupSlot 接受槽位字符串（sidebar.footer）或常量名（SlotSidebarFooter）。
func lookupSlot(s string) (pluginapi.Slot, bool) {
	for _, e := range slotTable {
		if s == string(e.Slot) || s == e.Const {
			return e.Slot, true
		}
	}
	return "", false
}

// ── 取色表 ──────────────────────────────────────────────────────────────

// colorTable 把 JSON 里的颜色名映射到 Theme 字段。
//
// 为什么不收十六进制：插件里写死色号 = 换主题时漏一块（见 AGENTS.md 红线 6）。
// 声明式插件更没理由例外——它连主题是什么都不知道，只能按名字要。
var colorTable = map[string]func(*pluginapi.Theme) ui.Color{
	"label-primary":   func(t *pluginapi.Theme) ui.Color { return t.LabelPrimary },
	"label-secondary": func(t *pluginapi.Theme) ui.Color { return t.LabelSecondary },
	"label-tertiary":  func(t *pluginapi.Theme) ui.Color { return t.LabelTertiary },
	"label-caption":   func(t *pluginapi.Theme) ui.Color { return t.LabelCaption },
	"success":         func(t *pluginapi.Theme) ui.Color { return t.Success },
	"warn":            func(t *pluginapi.Theme) ui.Color { return t.Warn },
	"info":            func(t *pluginapi.Theme) ui.Color { return t.Info },
	"error":           func(t *pluginapi.Theme) ui.Color { return t.Error },
}

// Colors 返回可用颜色名（排序后，报错信息用）。
func Colors() []string {
	out := make([]string, 0, len(colorTable))
	for k := range colorTable {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// color 取这一块该用的颜色。
//
// 没写 color 时按 kind 给默认：状态点取 Success（"在跑"），文字取 LabelSecondary
// （次要信息），keyvalue 的值取 LabelPrimary（它是要被读到的那半）。
func (d *Decl) color(t *pluginapi.Theme) ui.Color {
	if fn, ok := colorTable[d.Color]; ok {
		return fn(t)
	}
	switch d.Kind {
	case KindDot:
		return t.Success
	case KindKeyValue:
		return t.LabelPrimary
	default:
		return t.LabelSecondary
	}
}

// height 返回这一块的高度；list 不写 height 时由内容撑开（返回 0）。
func (d *Decl) height() float32 {
	if d.Height > 0 {
		return d.Height
	}
	if d.Kind == KindList {
		return 0
	}
	return defaultHeight
}

// lines 返回 list 的每一行：items 优先，其次按 "\n" 切 text。
func (d *Decl) lines() []string {
	if len(d.Items) > 0 {
		return d.Items
	}
	if d.Text == "" {
		return nil
	}
	return strings.Split(d.Text, "\n")
}

// ── 解析与校验 ──────────────────────────────────────────────────────────

// parseJSON 把一份 JSON 变成声明。file 只用于报错信息——
// 这条通道的错误必须能定位到文件，否则用户面对「装配失败」四个字无从下手。
func parseJSON(file string, data []byte) (*Decl, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	// 认不出的字段直接报错：写成 "tetx" 却静默不生效，正是这条通道最坏的失败方式。
	dec.DisallowUnknownFields()
	var raw rawDecl
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: JSON 解析失败: %w（可用字段: id/slot/order/kind/text/label/items/color/height/comment）", file, err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%s: 文件里不止一个 JSON 对象（一条声明一个文件）", file)
	}

	d := &Decl{File: file}

	d.ID = strings.TrimSpace(raw.ID)
	if d.ID == "" {
		return nil, fmt.Errorf("%s: 缺 \"id\"（全局唯一的部件 id，例如 hello.badge）", file)
	}

	if strings.TrimSpace(raw.Slot) == "" {
		return nil, fmt.Errorf("%s: 缺 \"slot\"；可用槽位: %s", file, strings.Join(Slots(), ", "))
	}
	slot, ok := lookupSlot(strings.TrimSpace(raw.Slot))
	if !ok {
		// 不静默：说出写了什么、再列出能写什么。
		return nil, fmt.Errorf("%s: 未知槽位 %q；可用槽位: %s", file, raw.Slot, strings.Join(Slots(), ", "))
	}
	d.Slot = slot

	d.Order = defaultOrder
	if raw.Order != nil {
		d.Order = *raw.Order
	}

	kind := Kind(strings.ToLower(strings.TrimSpace(raw.Kind)))
	if kind == "" {
		kind = KindLabel // 不写 kind 就是一行文字：最常用的形状给最省事的默认
	}
	switch kind {
	case KindLabel, KindKeyValue, KindDot, KindList:
		d.Kind = kind
	default:
		return nil, fmt.Errorf("%s: 未知 kind %q；可用 kind: %s", file, raw.Kind, strings.Join(Kinds(), ", "))
	}

	if raw.Color != "" {
		if _, ok := colorTable[raw.Color]; !ok {
			return nil, fmt.Errorf("%s: 未知颜色 %q；可用颜色: %s", file, raw.Color, strings.Join(Colors(), ", "))
		}
		d.Color = raw.Color
	}

	if raw.Height != nil {
		if *raw.Height < 0 {
			return nil, fmt.Errorf("%s: height 不能是负数（%g）", file, *raw.Height)
		}
		d.Height = *raw.Height
	}

	d.Text = raw.Text
	d.Label = raw.Label
	d.Items = raw.Items

	// 「写了但用不上」也算错：这类字段被静默忽略时，用户会以为自己设对了。
	switch d.Kind {
	case KindKeyValue:
		// label 是它的正牌字段，什么都不用说
	case KindList:
		if strings.TrimSpace(d.Label) != "" {
			return nil, fmt.Errorf("%s: kind=list 用不到 \"label\"（label 只给 keyvalue）", file)
		}
	default:
		if strings.TrimSpace(d.Label) != "" {
			return nil, fmt.Errorf("%s: kind=%s 用不到 \"label\"（label 只给 keyvalue）", file, d.Kind)
		}
	}
	if d.Kind != KindList && len(d.Items) > 0 {
		return nil, fmt.Errorf("%s: kind=%s 用不到 \"items\"（items 只给 list）", file, d.Kind)
	}

	// 内容空着就当没挂上——那正是「以为挂上了」的感觉，所以直接报错。
	if d.Kind == KindList {
		if len(d.Items) == 0 && strings.TrimSpace(d.Text) == "" {
			return nil, fmt.Errorf("%s: kind=list 需要 \"items\" 数组或带换行的 \"text\"", file)
		}
	} else if strings.TrimSpace(d.Text) == "" {
		return nil, fmt.Errorf("%s: 缺 \"text\"（kind=%s 至少得有一行字）", file, d.Kind)
	}

	return d, nil
}

// ── 模板变量 ────────────────────────────────────────────────────────────

// Expand 把模板里的 {变量} 换成快照里的值。
//
// 两条硬规则：
//   - **认不出来的变量一律换成空串**，绝不把 {xxx} 原样漏到界面上。
//     界面上出现花括号，用户只会以为界面坏了，而且不知道坏在哪；
//   - `{{` / `}}` 是转义的字面量 `{` / `}`，要写 JSON 片段这类文字得这么写。
//
// 为什么是「空串」而不是「报错」：{cur_title} 在没有打开会话时本来就该是空的，
// 这不是配置错误。真正写错变量名的情况由文档里的变量表兜底（见 docs/plugin-api.md §13）。
func Expand(tmpl string, snap pluginapi.Snapshot) string {
	if tmpl == "" || !strings.ContainsAny(tmpl, "{}") {
		return tmpl
	}
	var b strings.Builder
	b.Grow(len(tmpl) + 16)
	for i := 0; i < len(tmpl); {
		switch tmpl[i] {
		case '{':
			if i+1 < len(tmpl) && tmpl[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			end := strings.IndexByte(tmpl[i+1:], '}')
			if end < 0 {
				b.WriteString(tmpl[i:])
				i = len(tmpl)
				continue
			}
			name := tmpl[i+1 : i+1+end]
			if !isVarName(name) {
				// 不像变量名（有空格/换行/太长）：当字面量，原样保留
				b.WriteByte('{')
				i++
				continue
			}
			b.WriteString(varValue(name, snap))
			i += end + 2
		case '}':
			if i+1 < len(tmpl) && tmpl[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			b.WriteByte('}')
			i++
		default:
			b.WriteByte(tmpl[i])
			i++
		}
	}
	return b.String()
}

// isVarName 判断花括号里那一段是不是变量名。
//
// 刻意宽松（不含空白、长度受限就算）：变量名写错了也要被替换成空串，
// 而不是原样露出来。真正需要字面花括号的人用 `{{`。
func isVarName(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '{', '}':
			return false
		}
	}
	return true
}

// varValue 是变量表的唯一出处。加变量先加这里，再改文档。
func varValue(name string, snap pluginapi.Snapshot) string {
	switch name {
	case "sessions":
		return strconv.Itoa(len(snap.Sessions))
	case "cur_title":
		return curTitle(snap)
	case "messages":
		if snap.Total > 0 {
			return strconv.Itoa(snap.Total)
		}
		return strconv.Itoa(len(snap.Msgs))
	case "status":
		return snap.Status
	case "tokens":
		// Usage.Known() 为假时**一个字都不显示**：假数字比没数字更糟（同 dsstatusbar）。
		if snap.Usage.Known() {
			return formatTokens(snap.Usage.TotalTokens)
		}
		return ""
	}
	return ""
}

// curTitle 找当前会话的标题；没打开会话就返回空串。
func curTitle(snap pluginapi.Snapshot) string {
	if snap.Cur == "" {
		return ""
	}
	for i := range snap.Sessions {
		if snap.Sessions[i].ID == snap.Cur {
			return snap.Sessions[i].Title
		}
	}
	return ""
}

// formatTokens 把 token 数压成「6.5k / 1.2M」，与状态栏（dsstatusbar.tok）同一套写法。
//
// 不跟着压的话，一行 22px 的文字会被 190800 这种数字撑爆。
func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return oneDecimal(float64(n)/1_000_000) + "M"
	case n >= 1_000:
		return oneDecimal(float64(n)/1_000) + "k"
	default:
		return strconv.FormatInt(n, 10)
	}
}

func oneDecimal(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// ── 插件包装 ────────────────────────────────────────────────────────────

// Plugin 把一份声明包成 pluginapi.Plugin（一个声明 = 一个插件）。
//
// theme / prims 由装配者给：内核只会装配 ds-decl 这一个插件，
// 每份声明自己的 Register 不会被调用（也没有副作用要清理）。
func (d *Decl) Plugin(theme *pluginapi.Theme, prims pluginapi.Primitives) pluginapi.Plugin {
	return &declPlugin{decl: d, theme: theme, prims: prims}
}

// Plugins 把若干声明包成插件列表。
func Plugins(decls []*Decl, theme *pluginapi.Theme, prims pluginapi.Primitives) []pluginapi.Plugin {
	out := make([]pluginapi.Plugin, 0, len(decls))
	for _, d := range decls {
		out = append(out, d.Plugin(theme, prims))
	}
	return out
}

type declPlugin struct {
	decl  *Decl
	theme *pluginapi.Theme
	prims pluginapi.Primitives
}

func (p *declPlugin) ID() string         { return p.decl.ID }
func (p *declPlugin) Provides() []string { return nil }

func (p *declPlugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

// Register 什么都不做：声明部件没有运行期状态，主题和图元在构造时就给了。
func (p *declPlugin) Register(pluginapi.Context) error { return nil }

func (p *declPlugin) Panels() []pluginapi.Painter {
	d := p.decl
	return []pluginapi.Painter{{
		ID:    d.ID,
		Slot:  d.Slot,
		Order: d.Order,
		Draw:  p.draw,
	}}
}

// draw 按 kind 画。这里是整条通道唯一碰 flex 的地方。
func (p *declPlugin) draw(f *flex.Frame, env *pluginapi.Env) {
	d := p.decl
	t := p.theme
	if env != nil && t == nil {
		t = env.Theme
	}
	if t == nil || env == nil || f == nil {
		return
	}
	switch d.Kind {
	case KindKeyValue:
		p.drawKeyValue(f, env, t)
	case KindDot:
		p.drawDot(f, env, t)
	case KindList:
		p.drawList(f, env, t)
	default:
		p.drawLabel(f, env, t)
	}
}

// drawLabel 一行文字。空内容（比如 {cur_title} 恰好没值）就整块不画，
// 不占高度——否则槽位里会多一条空白，看着像界面坏了。
func (p *declPlugin) drawLabel(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	d := p.decl
	text := Expand(d.Text, env.Data)
	if text == "" {
		return
	}
	f.Row().FillWidth().Height(d.height()).Items(ui.Center).Children(func() {
		f.Text(text).FontSize(t.Fonts.Caption).Color(d.color(t)).Ellipsis()
	})
}

// drawKeyValue 左标签 + 右值。
func (p *declPlugin) drawKeyValue(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	d := p.decl
	label := Expand(d.Label, env.Data)
	value := Expand(d.Text, env.Data)
	if label == "" && value == "" {
		return
	}
	f.Row().FillWidth().Height(d.height()).Items(ui.Center).Gap(8).Children(func() {
		if label != "" {
			// Shrink(0)：标签不许被压缩，空间不够时被省略的永远是右值。
			f.Text(label).FontSize(t.Fonts.Caption).Color(t.LabelTertiary).Ellipsis().Shrink(0)
			f.Spacer()
		}
		f.Text(value).FontSize(t.Fonts.Caption).Color(d.color(t)).Ellipsis()
	})
}

// drawDot 状态点 + 一行文字。圆点用 color（默认 success），文字固定次要色——
// 让「状态点变色」和「文字变色」是两件事，别混。
func (p *declPlugin) drawDot(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	d := p.decl
	text := Expand(d.Text, env.Data)
	if text == "" {
		return
	}
	f.Row().FillWidth().Height(d.height()).Items(ui.Center).Gap(6).Children(func() {
		if p.prims != nil {
			p.prims.Dot(f, d.color(t), 6)
		}
		f.Text(text).FontSize(t.Fonts.Caption).Color(t.LabelSecondary).Ellipsis()
	})
}

// drawList 多行文字。空行整行跳过（不占高度），免得变量恰好没值时留下空洞。
func (p *declPlugin) drawList(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme) {
	d := p.decl
	col := f.Column().FillWidth().Gap(listLineSpacing)
	if h := d.height(); h > 0 {
		col = col.Height(h)
	}
	col.Children(func() {
		for _, line := range d.lines() {
			s := Expand(line, env.Data)
			if s == "" {
				continue
			}
			f.Row().FillWidth().Height(listLineHeight).Items(ui.Center).Children(func() {
				f.Text(s).FontSize(t.Fonts.Caption).Color(d.color(t)).Ellipsis()
			})
		}
	})
}

var (
	_ pluginapi.Plugin = (*declPlugin)(nil)
	_ pluginapi.Panels = (*declPlugin)(nil)
)
