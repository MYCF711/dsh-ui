// official_describe.go —— `settings.describe` 的**界面投影**：把宿主给的数据变成
// 可渲染的 section / group / row 树。
//
// # 唯一规矩：只画真值
//
//   - **宿主没给的字段显示「未知」，绝不编 0**：field 缺失 / `current` 为 nil /
//     namespace 不存在，都走 `Known=false`，渲染成「未知」并禁用写入。
//   - **整段隐藏也是合法降级**：官方按条件注册的 section（account）在条件不满足时
//     **根本不出现**，不是画一个空页。
//   - **未知字段名不猜**：describe 里没有的字段，界面不编一个「看起来合理」的
//     默认值；官方默认值只用于「宿主给了 schema 但没给 current」的展示提示，
//     且必须标出「默认」而不是「当前」。
//
// 数据来源：`internal/apiclient.SettingsDescribeResult`（宿主 wire 形状）。
package dssettings

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// PanelState 是一个 section 右页的渲染状态。
type PanelState string

const (
	// PanelOK 正常渲染（有数据）。
	PanelOK PanelState = "ok"
	// PanelReadOnly 数据在、但命名空间不可写（宿主 writable=false）。
	PanelReadOnly PanelState = "read_only"
	// PanelUnknown 宿主没给这个 section 需要的数据（整段显示「未知」，
	// **不画任何编造的字段行**）。
	PanelUnknown PanelState = "unknown"
	// PanelEmpty 宿主给了、但这一帧没有内容（例如没有任何 provider）。
	PanelEmpty PanelState = "empty"
)

// PanelChoice 是选择器的一个候选及选中态。
type PanelChoice struct {
	Value    string
	Label    string
	Desc     string
	Selected bool
	// Disabled 表示这一项当前不可写（只读 / 宿主未开放）。
	Disabled bool
}

// PanelRow 是右页的一行（可写行或只读行）。
type PanelRow struct {
	// ID 是稳定行 id（拼交互 id `settings-<id>`）。
	ID string
	// Title / Desc 是官方行的两段文案。
	Title string
	Desc  string
	// Kind 是控件形状。
	Kind ItemKind
	// NS / Field / Path 是写入目标（只读行留空）。
	NS    string
	Field string
	Path  []string
	// Value 是**已格式化**的显示值；Known=false 时它必须是空串，
	// 渲染层用 Display() 显示「未知」。
	Value string
	// Known 表示宿主真的给了这个值。false 时**不许显示任何具体数字**。
	Known bool
	// Selected 是当前值（选择器行的选中 value；Switch 行是 "true"/"false"）。
	Selected string
	// Choices 是选择器候选（非选择器行为空）。
	Choices []PanelChoice
	// Disabled 表示这一行当前不可写。
	Disabled bool
	// Reason 是禁用/未知的原因（给人看的，来自宿主缺失说明）。
	Reason string
	// Bounds 是数值行的官方边界（font-size 这类）。
	Bounds Bounds
	// Revision 是写这一行时的 expectedRevision 种子（来自 section.revision）。
	Revision int
}

// Display 是这一行**该画出来的值文本**。
//
// 未知统一显示「未知」：**绝不显示 0**（0 是一个具体数值，会被用户当成真值）。
func (r PanelRow) Display() string {
	if !r.Known {
		return "未知"
	}
	return r.Value
}

// Writable 报告这一行是否可写（有目标、未禁用、宿主给了值）。
func (r PanelRow) Writable() bool {
	return !r.Disabled && len(r.Path) > 0
}

// PanelGroup 是右页的一组（general 的「通用」、models 的 provider 组等）。
type PanelGroup struct {
	Title string
	Rows  []PanelRow
}

// PanelSection 是左导航的一页 + 右页内容。
type PanelSection struct {
	ID       string
	Title    string
	Icon     string
	Order    int
	State    PanelState
	Writable bool
	Revision int
	// Note 是分组级说明（未知原因 / 只读原因 / 截断说明）。
	Note   string
	Groups []PanelGroup
	// Rows 是**拍平**的行（绘制与键盘导航按它走，group 只为视觉分组）。
	Rows []PanelRow
	// Anchor 是 rc.2 注册点（报告与结构测试用）。
	Anchor string
}

// PanelModel 是一帧设置面板的全部内容。
type PanelModel struct {
	Writable    bool
	HasDocument bool
	Sections    []PanelSection
	Errors      []string
	// Unknown 是「宿主没给」的字段清单（`<ns>.<field>` 形式），
	// 报告与测试用它证明「没有编值」。
	Unknown []string
	// RowCount 是 general 页的可见行数（官方口径 14 / 13 / 15）。
	RowCount int
}

// Section 按 id 取一页；没有就是 false（例如 account 条件不满足时**不存在**）。
func (m PanelModel) Section(id string) (PanelSection, bool) {
	for _, s := range m.Sections {
		if s.ID == id {
			return s, true
		}
	}
	return PanelSection{}, false
}

// Row 在一页里按行 id 找一行。
func (s PanelSection) Row(id string) (PanelRow, bool) {
	for _, r := range s.Rows {
		if r.ID == id {
			return r, true
		}
	}
	return PanelRow{}, false
}

// BuildPanel 把 describe 结果 + 宿主事实投影成面板模型。
//
// describe 为 nil 表示宿主没接线（老壳）：那时**结构骨架照画**（5 个 section、
// 13 个 general item 的标题），但每行都是「未知」——这是「结构先于数据」的落点。
func BuildPanel(describe *apiclient.SettingsDescribeResult, facts HostFacts) PanelModel {
	m := PanelModel{RowCount: facts.VisibleRowCount()}
	if describe != nil {
		m.Writable = describe.Writable
		m.HasDocument = describe.HasDocument
		m.Errors = append([]string(nil), describe.Errors...)
	}

	registeredIcons := map[string]string{}
	for _, sec := range facts.Sections() {
		panel := PanelSection{
			ID:       sec.ID,
			Title:    sec.Title,
			Icon:     SectionIcon(sec.ID, registeredIcons),
			Order:    sec.Order,
			State:    PanelOK,
			Writable: m.Writable,
			Anchor:   sec.Anchor,
		}
		switch sec.ID {
		case "general":
			buildGeneralSection(&panel, describe, facts, &m)
		case "models":
			buildModelsSection(&panel, describe, &m)
		case "plugins":
			buildStaticSection(&panel, describe, "plugins",
				"内置插件清单需要 pluginInventory.list（settings.describe 不含该数据）")
		case "agent-presets":
			buildStaticSection(&panel, describe, "agent-presets",
				"Agent 预设需要 agentPreset 数据（settings.describe 不含该数据）")
		case "account":
			buildStaticSection(&panel, describe, "account",
				"账号与余额需要 credential-stored 与 account 数据")
		}
		m.Sections = append(m.Sections, panel)
	}
	sort.SliceStable(m.Sections, func(i, j int) bool { return m.Sections[i].Order < m.Sections[j].Order })
	return m
}

// buildGeneralSection 按官方 13 个 slot 造 general 页。
//
// 每个 item 一行（transcript-view 三行），**顺序与官方 order 一致**；数据缺失时
// 行照样在（结构骨架），值是「未知」。
func buildGeneralSection(panel *PanelSection, describe *apiclient.SettingsDescribeResult, facts HostFacts, m *PanelModel) {
	var rows []PanelRow
	for _, it := range facts.GeneralItems() {
		switch it.ID {
		case "transcript-view":
			// 一个 slot 三行（官方 TranscriptViewRow.tsx:57-93）。
			for i := 0; i < facts.ItemRows(it); i++ {
				rows = append(rows, generalRow(it.ID+"-"+strconv.Itoa(i), transcriptViewRowTitles[i],
					it.HostField, transcriptViewRowFields[i], transcriptViewRowChoices[i],
					transcriptViewRowDefaults[i], ItemSelector, describe, facts, m))
			}
		case "current-version":
			rows = append(rows, PanelRow{
				ID:    it.ID,
				Title: it.Title,
				Kind:  ItemVersion,
				// 版本号是 build 注入的；**缺失时整行不渲染**（官方
				// CurrentVersionRow.tsx:10-14），所以条件已经把它挡在外面了。
				Value: facts.ClientVersion,
				Known: facts.ClientVersion != "",
				Field: it.HostField,
			})
		default:
			rows = append(rows, generalRow(it.ID, it.Title, it.HostField, it.HostField,
				it.Choices, it.Default, it.Kind, describe, facts, m))
		}
	}
	panel.Rows = rows
	panel.Groups = []PanelGroup{{Title: "", Rows: rows}}
	panel.Writable = m.Writable
	if describe == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：宿主未接 settings.describe（结构骨架照画，值一律未知）"
	}
}

// generalRow 造一行 general item。
//
// 数据来源按 item 的 host 字段拆：`<ns>.<field>` 形式里 ns 是命名空间、field 是
// 字段名。`permission.defaultPreset` 这类首切字段在宿主 describe 里就有；
// `ui-theme.*` / `ui-chat.*` 这些命名空间宿主当前未提供 → 未知（不编）。
func generalRow(id, title, hostField, field string, choices []Choice, defaultValue string, kind ItemKind,
	describe *apiclient.SettingsDescribeResult, facts HostFacts, m *PanelModel) PanelRow {
	row := PanelRow{
		ID:    id,
		Title: title,
		Kind:  kind,
		Field: hostField,
		Bounds: Bounds{
			// 默认给官方的字号边界（只有 font-size 用得到）；其它行 Kind 决定
			// 是否需要，绘制层按 Kind 判。
			HasMin: kind == ItemStepper,
			HasMax: kind == ItemStepper,
		},
	}
	if kind == ItemStepper {
		row.Bounds = Bounds{HasMin: true, HasMax: true, Min: float64(FontSizeMin), Max: float64(FontSizeMax), Step: 1}
	}
	for _, c := range choices {
		row.Choices = append(row.Choices, PanelChoice{Value: c.Value, Label: c.Label, Desc: c.Desc})
	}

	ns, name := splitHostField(hostField)
	if ns == "" || describe == nil {
		row.Disabled = true
		row.Reason = unknownReason(ns, describe)
		m.Unknown = append(m.Unknown, hostField)
		return row
	}
	section, ok := describe.Section(ns)
	if !ok {
		row.Disabled = true
		row.Reason = "宿主未提供 " + ns + " 命名空间"
		m.Unknown = append(m.Unknown, hostField)
		return row
	}
	row.NS = ns
	row.Revision = section.Revision
	row.Path = []string{name}
	f, ok := section.Field(name)
	if !ok {
		row.Disabled = true
		row.Reason = "宿主未提供字段 " + ns + "." + name
		m.Unknown = append(m.Unknown, hostField)
		return row
	}
	row.Disabled = !section.Writable || !f.Writable
	if value, known := apiclient.SettingValueString(f.Current); known {
		row.Value = value
		row.Known = true
		row.Selected = value
	} else if defaultValue != "" {
		// 宿主给了 schema 但没给 current：显示官方默认值，但**标成默认**。
		row.Value = defaultValue
		row.Reason = "默认值（宿主未给 current）"
	}
	if row.Disabled {
		row.Reason = "只读：该命名空间未开放写入"
	}
	return row
}

// buildModelsSection 造 models 页：provider 列表 + retryPolicy 编辑。
//
// 数据来源：`model` 命名空间（默认模型）与 `llm` 命名空间
// （`providers.<route>.retryPolicy`，可写）。
func buildModelsSection(panel *PanelSection, describe *apiclient.SettingsDescribeResult, m *PanelModel) {
	if describe == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：宿主未接 settings.describe"
		return
	}

	// ── 默认模型（model 命名空间）──
	if section, ok := describe.Section(apiclient.SettingsNSModel); ok {
		row := PanelRow{
			ID: "model-default", Title: "默认模型", Kind: ItemSelector,
			NS: section.NS, Field: "defaultModel", Path: []string{"defaultModel"},
			Revision: section.Revision,
			Disabled: !section.Writable,
			Choices: []PanelChoice{
				{Value: "", Label: "跟随启动参数", Desc: "清除覆盖，回到宿主的 --model"},
			},
		}
		if f, ok := section.Field("defaultModel"); ok {
			if value, known := apiclient.SettingValueString(f.Current); known {
				row.Value, row.Known, row.Selected = value, true, value
			}
			if enum := f.Enum; len(enum) > 0 {
				row.Choices = nil
				for _, v := range enum {
					row.Choices = append(row.Choices, PanelChoice{Value: v, Label: v})
				}
			}
		}
		if !row.Known {
			m.Unknown = append(m.Unknown, "model.defaultModel")
		}
		panel.Rows = append(panel.Rows, row)
	} else {
		m.Unknown = append(m.Unknown, "model.*")
	}

	// ── provider 的 retryPolicy（llm 命名空间）──
	routes, policies, ok := llmRetryPolicies(describe)
	switch {
	case !ok:
		panel.Rows = append(panel.Rows, PanelRow{
			ID: "retry-policy-unknown", Title: "重试策略", Kind: ItemSelector,
			Disabled: true, Reason: "宿主未提供 llm 命名空间（providers.<route>.retryPolicy）",
		})
		m.Unknown = append(m.Unknown, "llm.providers.*.retryPolicy")
	case len(routes) == 0:
		panel.State = PanelEmpty
		panel.Note = "宿主 llm 命名空间里没有任何 provider 配置"
	default:
		// llm 的 section 可能没有单独的 form 投影（宿主只给 namespace value），
		// 那时用顶层 writable / revision：**不要**因为「没有 section」就把行判成
		// 只读——那会让可写的 retryPolicy 整页点不动。
		revision, writable := 0, describe.Writable
		if section, ok := describe.Section(apiclient.SettingsNSLLM); ok {
			revision, writable = section.Revision, section.Writable
		}
		for _, route := range routes {
			view := policies[route]
			for _, row := range RetryPolicyRows(route, view, revision, writable) {
				panel.Rows = append(panel.Rows, row)
			}
		}
	}

	panel.Groups = []PanelGroup{{Title: "模型与重试", Rows: panel.Rows}}
	panel.Writable = describe.Writable
}

// buildStaticSection 是「结构在、数据不在」的 section 的统一降级：
// 只给一条未知说明行，**不编任何字段**。
func buildStaticSection(panel *PanelSection, describe *apiclient.SettingsDescribeResult, ns, reason string) {
	if describe == nil {
		panel.State = PanelUnknown
		panel.Note = "未知：宿主未接 settings.describe"
		return
	}
	if _, ok := describe.Section(ns); !ok {
		panel.State = PanelUnknown
		panel.Note = "未知：" + reason
		return
	}
	panel.State = PanelOK
	section, _ := describe.Section(ns)
	panel.Revision = section.Revision
	panel.Writable = section.Writable
	panel.Groups = []PanelGroup{{Title: "", Rows: panel.Rows}}
}

// llmRetryPolicies 从 llm 命名空间的原始 value 里取每个 route 的 retryPolicy。
//
// 官方形状（dsh-llm/src/retry-policy.ts:29 与 config-catalog）：
//
//	llm: { providers: { <route>: { retryPolicy: { mode, maxRetries,
//	                                  retryableCodes, backoff:{…} } } } }
//
// 返回按 route 名排序（界面稳定，不依赖 map 迭代顺序）。ok=false 表示命名空间
// 缺失或 value 不是对象（那时整组显示未知）。
func llmRetryPolicies(describe *apiclient.SettingsDescribeResult) (routes []string, out map[string]RetryPolicyView, ok bool) {
	if describe == nil {
		return nil, nil, false
	}
	ns, found := describe.Namespace(apiclient.SettingsNSLLM)
	if !found || len(ns.Value) == 0 {
		return nil, nil, false
	}
	var raw struct {
		Providers map[string]struct {
			RetryPolicy json.RawMessage `json:"retryPolicy"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(ns.Value, &raw); err != nil {
		return nil, nil, false
	}
	out = make(map[string]RetryPolicyView, len(raw.Providers))
	for route, provider := range raw.Providers {
		view, err := ParseRetryPolicyView(route, provider.RetryPolicy)
		if err != nil {
			// 解析失败 = 不知道这一条的策略；**不编默认值**，跳过并让调用方
			// 从 routes 里看见它（这样测试能断言「坏数据没有变成默认策略」）。
			out[route] = RetryPolicyView{Route: route, Known: false, Err: err.Error()}
			routes = append(routes, route)
			continue
		}
		out[route] = view
		routes = append(routes, route)
	}
	sort.Strings(routes)
	return routes, out, true
}

// splitHostField 把官方 host 字段拆成 `<ns>.<field>`。
//
// 只拆第一段：`session-log-deepseek.enabled` → ("session-log-deepseek", "enabled")、
// `dshDesktop.updates.channels` → ("dshDesktop", "updates.channels")（后者是多段
// 字段，属于桌面桥而不是 settings 命名空间，调用方据此走未知）。
func splitHostField(hostField string) (ns, field string) {
	for i := 0; i < len(hostField); i++ {
		if hostField[i] == '.' {
			return hostField[:i], hostField[i+1:]
		}
	}
	return "", hostField
}

// unknownReason 给「没有命名空间」的行一个可读原因。
func unknownReason(ns string, describe *apiclient.SettingsDescribeResult) string {
	if describe == nil {
		return "未知：宿主未接 settings.describe"
	}
	if ns == "" {
		return "未知：该行没有 settings 命名空间（由宿主侧服务提供）"
	}
	return "未知：宿主未提供 " + ns + " 命名空间"
}
