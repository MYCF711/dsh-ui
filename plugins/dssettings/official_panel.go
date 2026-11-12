// official_panel.go —— rc.2 官方面板的**绘制层**：CSS Grid 四区（official_grid.go
// 求解）+ 模态层（official_modal.go）+ 数据投影（official_describe.go）。
//
// # 与旧浮层的关系
//
// rc.1 的 `settings.go` 是「四块动作摊开」的最小浮层（外观 / 模型 / 权限 / 会话），
// 结构树 §10 已判定它**错层**（把 general item 当 section、自造「默认设置」等）。
// rc.2 官方面板是 5 个 section 的完整设置面。
//
// 两者并存的方式是**兼容桥**：
//
//	壳接上了 settings.describe（真数据面）→ 画官方四区面板（本文件）
//	壳没接（老壳 / 旧夹具）              → 退回 rc.1 浮层（settings.go）
//
// 判据是 `OfficialEnabled()`（= describe 接线了吗），不是编译开关。这样旧测试
// 与旧路径不受影响，而产品一旦接上数据面就自动是官方结构。
package dssettings

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// panelWriteTimeout 是一次设置写的上限：宿主写盘 + 重读，8 秒足够；
// 超时按传输层失败处理（可重试），**不阻塞绘制线程**。
const panelWriteTimeout = 8 * time.Second

// officialPanel 是官方四区面板的运行时状态。
type officialPanel struct {
	mu sync.Mutex

	open     bool
	activeID string
	model    PanelModel
	write    WriteState
	modal    *ModalLayer

	// 数据面（每帧从 env.Act 同步；nil = 壳没接）。
	describe  *apiclient.SettingsDescribeResult
	facts     HostFacts
	writer    SettingsWriter
	hasSource bool

	// rows 是当前 section 的可点行（键盘光标用）。
	rows   []PanelRow
	cursor int

	// layout / titleID 是上一帧算出来的几何与标题 id（测试与 AfterDraw 用）。
	layout  GridLayout
	titleID string

	notify func(string)
}

// ensureOfficial 惰性造面板状态（零值可用）。
func (p *Plugin) ensureOfficial() *officialPanel {
	if p.official == nil {
		p.official = &officialPanel{cursor: -1}
		p.official.modal = NewModalLayer(func() { p.CloseOfficial() })
	}
	if p.official.modal == nil {
		p.official.modal = NewModalLayer(func() { p.CloseOfficial() })
	}
	return p.official
}

// OfficialEnabled 报告官方四区面板是否可用（= 壳接上了 settings.describe）。
//
// 为什么以 describe 为判据：结构骨架可以画，但**没有数据面的面板等于空壳**；
// 官方在「宿主没接」时的正确行为是退回旧路径，而不是画一整页「未知」。
func (p *Plugin) OfficialEnabled() bool {
	o := p.official
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hasSource
}

// OpenOfficial 打开官方面板；返回「这一帧确实有可操作的东西」。
func (p *Plugin) OpenOfficial() bool {
	if !p.OfficialEnabled() {
		return false
	}
	o := p.ensureOfficial()
	o.mu.Lock()
	o.open = true
	o.cursor = -1
	active := o.activeID
	if active == "" && len(o.model.Sections) > 0 {
		active = o.model.Sections[0].ID
		o.activeID = active
	}
	titleID := o.titleID
	rows := o.rows
	o.mu.Unlock()

	o.modal.Open(titleID, focusableIDs(rows), "", "")
	notify := p.notify
	if notify != nil {
		notify("设置已打开 · Esc 关闭")
	}
	return true
}

// CloseOfficial 收起官方面板（幂等）。
func (p *Plugin) CloseOfficial() {
	o := p.official
	if o == nil {
		return
	}
	o.mu.Lock()
	was := o.open
	o.open = false
	o.cursor = -1
	o.mu.Unlock()
	if !was {
		return
	}
	_, _ = o.modal.Close()
	if notify := p.notify; notify != nil {
		notify("设置已关闭")
	}
}

// OfficialShown 报告官方面板是否开着。
func (p *Plugin) OfficialShown() bool {
	o := p.official
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.open
}

// PanelModel 返回上一帧算出来的面板模型（测试/调试用）。
func (p *Plugin) PanelModel() PanelModel {
	o := p.official
	if o == nil {
		return PanelModel{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.model
}

// ActiveSection 返回当前 section id。
func (p *Plugin) ActiveSection() string {
	o := p.official
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.activeID
}

// SelectSection 切到某一页（导航点击）。
func (p *Plugin) SelectSection(id string) {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.model.SectionExists(id) {
		o.activeID = id
		o.cursor = -1
	}
	rows := o.rows
	titleID := o.titleID
	o.mu.Unlock()
	if o.modal.Opened() {
		o.modal.SetFocusables(focusableIDs(rows), titleID)
	}
}

// SectionExists 报告模型里有没有这一页（account 条件不满足时不存在）。
func (m PanelModel) SectionExists(id string) bool {
	_, ok := m.Section(id)
	return ok
}

// WriteState 返回最近一次写的结果。
func (p *Plugin) WriteState() WriteState {
	o := p.official
	if o == nil {
		return WriteState{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.write
}

// ModalSemantics 返回对话框语义（ARIA 等价物；测试用）。
func (p *Plugin) ModalSemantics() DialogSemantics {
	o := p.ensureOfficial()
	return o.modal.Semantics()
}

// PanelLayout 返回上一帧的四区布局（测试用）。
func (p *Plugin) PanelLayout() GridLayout {
	o := p.official
	if o == nil {
		return GridLayout{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.layout
}

// focusableIDs 是模态层的可聚焦 id 列表（关闭按钮 + 导航 cell + 当前页行）。
func focusableIDs(rows []PanelRow) []string {
	out := make([]string, 0, len(rows)+1)
	out = append(out, "settings-close")
	for _, r := range rows {
		out = append(out, "settings-row-"+r.ID)
	}
	return out
}

// syncPanel 把这一帧的数据面同步进面板状态，并重算模型。
//
// **每帧都重算**：壳可能中途接上 describe / 改 revision；判据以这一帧为准，
// 不缓存成「第一次看到的样子」。
func (p *Plugin) syncPanel(env *pluginapi.Env) {
	o := p.ensureOfficial()
	describe := (*apiclient.SettingsDescribeResult)(nil)
	var writer SettingsWriter
	hasSource := false
	var facts HostFacts
	if env.Act.SettingsDescribe != nil {
		describe = env.Act.SettingsDescribe()
		hasSource = true
		writer = actionWriter{act: env.Act}
	}
	if env.Act.SettingsHostFacts != nil {
		facts = fromAPIHostFacts(env.Act.SettingsHostFacts())
	}

	model := BuildPanel(describe, facts)

	o.mu.Lock()
	o.describe = describe
	o.facts = facts
	o.writer = writer
	o.hasSource = hasSource
	o.model = model
	if env.Act.Notify != nil {
		o.notify = env.Act.Notify
	}
	active := o.activeID
	if _, ok := model.Section(active); !ok {
		active = ActiveSectionDefault(model.sectionsAsOfficial())
		o.activeID = active
	}
	section, _ := model.Section(active)
	o.rows = section.Rows
	if o.cursor >= len(o.rows) {
		o.cursor = -1
	}
	if !hasSource {
		o.open = false
	}
	o.mu.Unlock()

	if p.notify == nil && o.notify != nil {
		p.notify = o.notify
	}
}

// sectionsAsOfficial 把投影出来的 section 转回官方结构（只取 id/order/title）。
func (m PanelModel) sectionsAsOfficial() []OfficialSection {
	out := make([]OfficialSection, 0, len(m.Sections))
	for _, s := range m.Sections {
		out = append(out, OfficialSection{ID: s.ID, Title: s.Title, Order: s.Order})
	}
	return out
}

// fromAPIHostFacts 把 pluginapi 的宿主事实转成 dssettings 的判据结构。
func fromAPIHostFacts(f pluginapi.SettingsHostFacts) HostFacts {
	return HostFacts{
		DesktopProtocolVersion: f.DesktopProtocolVersion,
		UpdatesChannels:        f.UpdatesChannels,
		SidebarRightTabs:       f.SidebarRightTabs,
		SessionLogServed:       f.SessionLogServed,
		ClientVersion:          f.ClientVersion,
		PermissionCatalog:      f.PermissionCatalog,
		TranscriptView:         f.TranscriptView,
		CredentialStored:       f.CredentialStored,
	}
}

// actionWriter 把 pluginapi.Actions 的写函数包成 SettingsWriter。
type actionWriter struct{ act pluginapi.Actions }

func (w actionWriter) SettingsFormSet(_ context.Context, ns string, path []string, value any, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	if w.act.SettingsFormSet == nil {
		return apiclient.SettingsSectionView{}, errNoSettingsWriter
	}
	return w.act.SettingsFormSet(ns, path, value, derefRevision(expectedRevision))
}

func (w actionWriter) SettingsFormReset(_ context.Context, ns string, path []string, expectedRevision *int) (apiclient.SettingsSectionView, error) {
	if w.act.SettingsFormReset == nil {
		return apiclient.SettingsSectionView{}, errNoSettingsWriter
	}
	return w.act.SettingsFormReset(ns, path, derefRevision(expectedRevision))
}

func (w actionWriter) SettingsMutate(_ context.Context, ns string, ops []apiclient.SettingsOp, expectedRevision *int) (apiclient.SettingsNamespaceView, error) {
	if w.act.SettingsMutate == nil {
		return apiclient.SettingsNamespaceView{}, errNoSettingsWriter
	}
	return w.act.SettingsMutate(ns, ops, derefRevision(expectedRevision))
}

// errNoSettingsWriter 是「壳没接写路径」的失败（不是静默成功）。
var errNoSettingsWriter = &apiclient.APIError{
	Code:    apiclient.SettingsCodeInternal,
	Message: "壳未接 settings.form.set|reset（老壳）",
	Hint:    "升级壳；界面不会在本地假装写成功",
}

// derefRevision 把 nil 当成 0（= 不 fence）。
func derefRevision(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ── 绘制 ─────────────────────────────────────────────────────────────

// drawOfficialPanel 画官方四区面板。没开 / 没接数据面时什么都不画。
func (p *Plugin) drawOfficialPanel(f *flex.Frame, env *pluginapi.Env) {
	o := p.ensureOfficial()
	p.syncPanel(env)
	o.mu.Lock()
	open := o.open
	model := o.model
	activeID := o.activeID
	write := o.write
	o.mu.Unlock()
	if !open || !p.OfficialEnabled() {
		return
	}

	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	layout, err := LayoutSettingsPanel(env.Bounds.Width, env.Bounds.Height, PanelOverlayTop)
	if err != nil {
		// 模板错了是编程错误：画一个可见的错误条，而不是静默空白。
		f.Row().Absolute().Left(0).Top(0).Width(env.Bounds.Width).Height(24).
			Background(ui.RGBA(180, 40, 40, 1)).Children(func() {
			f.Text("设置面板布局失败: " + err.Error()).FontSize(12).Color(ui.RGBA(255, 255, 255, 1))
		})
		return
	}
	o.mu.Lock()
	o.layout = layout
	o.titleID = "settings-panel-title"
	o.mu.Unlock()

	// ① 遮罩（createPortal 到 body 级的等价物：整窗、z-index 1000、拦截点击）。
	f.Row().Absolute().Inset(0).Background(ui.RGBA(0, 0, 0, 0.24)).
		Hit(env.Tree.ID(env.Feat, "settings-mask"), "").Children(func() {
		// ② 卡片：四区网格的相对坐标（children 坐标相对卡片 padding box）。
		f.Row().Absolute().Left(layout.X).Top(layout.Y).
			Width(layout.Width).Height(layout.Height).
			Radius(t.Radii.Panel).Background(t.BgLayer2).
			Border(1, t.BorderL2).Shadow(ui.ShadowXS).Clip().
			Children(func() {
				if area, ok := layout.Area("title"); ok {
					p.drawNavTitle(f, t, relArea(area, layout))
				}
				if area, ok := layout.Area("header"); ok {
					p.drawPanelHeader(f, env, t, relArea(area, layout))
				}
				if area, ok := layout.Area("nav"); ok {
					p.drawPanelNav(f, env, t, relArea(area, layout), model, activeID)
				}
				if area, ok := layout.Area("options"); ok {
					section, _ := model.Section(activeID)
					p.drawPanelOptions(f, env, t, relArea(area, layout), section, write)
				}
			})
	})
}

// relArea 把绝对矩形转成卡片内相对矩形。
func relArea(a AreaRect, layout GridLayout) AreaRect {
	return AreaRect{X: a.X - layout.X, Y: a.Y - layout.Y, Width: a.Width, Height: a.Height}
}

// drawNavTitle 画左标题（aria-labelledby 指向它）。
func (p *Plugin) drawNavTitle(f *flex.Frame, t *pluginapi.Theme, a AreaRect) {
	f.Row().Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(NavTitlePadTop, NavTitlePadRight, NavTitlePadBottom, NavTitlePadLeft).
		Items(flex.Center).Children(func() {
		f.Text("设置").FontSize(NavTitleFontSize).LineHeight(NavTitleLineHeight).
			Weight(NavTitleWeight).Color(t.LabelPrimary)
	})
}

// drawPanelHeader 画右列头：action 区（空）+ 关闭按钮。
func (p *Plugin) drawPanelHeader(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect) {
	f.Row().Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(HeaderPadTop, HeaderPadRight, HeaderPadBottom, HeaderPadLeft).
		Gap(HeaderGap).Items(flex.Center).Children(func() {
		f.Row().Grow(1)
		if p.prims != nil {
			if p.prims.CircleButton(f, env.Tree.ID(env.Feat, "settings-close"), "×", false) {
				p.CloseOfficial()
			}
		}
	})
}

// drawPanelNav 画左导航（settings.section list）。
func (p *Plugin) drawPanelNav(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, model PanelModel, activeID string) {
	f.Column().Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(NavPadTop, NavPadX, 0, NavPadX).Gap(NavListGap).Children(func() {
		for _, section := range model.Sections {
			section := section
			active := section.ID == activeID
			bg := ui.Color{}
			if active {
				bg = t.NavActive
			}
			if f.Column().Height(NavCellHeight).Radius(t.Radii.MD).
				PadSides(NavCellPadTop, NavCellPadRight, NavCellPadBottom, NavCellPadLeft).
				Gap(NavCellGap).Items(flex.Center).Background(bg).Hover(flex.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "settings-nav-"+section.ID), section.Title).
				Children(func() {
					f.Text(IconGlyph(section.Icon)).FontSize(NavCellIconSize).Color(t.LabelSecondary)
					f.Text(section.Title).FontSize(NavCellFontSize).LineHeight(NavCellLineHeight).
						Color(t.LabelPrimary).Grow(1).Ellipsis()
				}).Clicked() {
				p.SelectSection(section.ID)
			}
		}
	})
}

// drawPanelOptions 画右页（active section 的行）。
func (p *Plugin) drawPanelOptions(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, section PanelSection, write WriteState) {
	f.Column().Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).Children(func() {
		if section.Note != "" {
			f.Text(section.Note).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.LabelTertiary).Ellipsis()
		}
		if write.Render() != "" && write.Failure() {
			f.Text(write.Render()).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.Error)
		}
		for i, row := range section.Rows {
			row := row
			p.drawOfficialRow(f, env, t, row)
			if i < len(section.Rows)-1 {
				if p.prims != nil {
					p.prims.Divider(f)
				}
			}
		}
	})
}

// OfficialRowHeight 返回一行的高度（官方行样式 §4.1 的 padding + 行高）。
func OfficialRowHeight(kind ItemKind) float32 {
	switch kind {
	case ItemVersion:
		return VersionRowLineHeight + 2*VersionRowPadY
	case ItemButton:
		return ShortcutRowMinHeight
	case ItemSwitch:
		return DevToolsTitleLineHeight + RowTextGap + RowDescLineHeight + 2*RowPadY
	default:
		return RowTitleLineHeight + RowTextGap + RowDescLineHeight + 2*RowPadY
	}
}

// drawOfficialRow 画一行（选择器 / 开关 / stepper / cube / 按钮 / 版本）。
func (p *Plugin) drawOfficialRow(f *flex.Frame, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	height := OfficialRowHeight(row.Kind)
	id := env.Tree.ID(env.Feat, "settings-row-"+row.ID)
	body := func() {
		f.Column().Grow(1).Gap(RowTextGap).Children(func() {
			f.Text(row.Title).FontSize(RowTitleFontSize).LineHeight(RowTitleLineHeight).
				Color(t.LabelPrimary).Ellipsis()
			if row.Desc != "" {
				f.Text(row.Desc).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
					Color(t.LabelTertiary).Ellipsis()
			}
			if row.Reason != "" {
				f.Text(row.Reason).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
					Color(t.Warn).Ellipsis()
			}
		})
		p.drawRowControl(f, t, row)
	}
	box := f.Row().FillWidth().Height(height).
		PadSides(RowPadY, 0, RowPadY, 0).Gap(RowGap).Items(flex.Center).
		Hit(id, row.Title).Children(body)
	if row.Writable() {
		if box.Clicked() {
			p.activateRow(row)
		}
	}
}

// drawRowControl 画行尾控件（按 Kind 分支）。
func (p *Plugin) drawRowControl(f *flex.Frame, t *pluginapi.Theme, row PanelRow) {
	switch row.Kind {
	case ItemSwitch:
		on := row.Known && row.Selected == "true"
		bg := t.BorderL3
		if on {
			bg = t.BrandPrimary
		}
		f.Row().Width(36).Height(20).Radius(10).Background(bg).PadSides(2, 2, 2, 2).
			Justify(flex.End).Children(func() {
			f.Row().Width(16).Height(16).Radius(8).Background(t.BgLayer2)
		})
	case ItemStepper:
		f.Row().Height(SelectorHeight).Radius(t.Radii.MD).PadX(SelectorPadX).
			Gap(SelectorGap).Items(flex.Center).Background(t.BgLayer3).Children(func() {
			f.Text("−").FontSize(RowTitleFontSize).Color(t.LabelSecondary)
			f.Text(row.Display()).FontSize(RowTitleFontSize).Color(t.LabelPrimary)
			f.Text("+").FontSize(RowTitleFontSize).Color(t.LabelSecondary)
		})
	case ItemCubes:
		f.Row().Gap(SelectorGap).Items(flex.Center).Children(func() {
			for _, c := range row.Choices {
				selected := row.Known && row.Selected == c.Value
				bg := t.BgLayer3
				if selected {
					bg = t.ButtonPrimaryFill
				}
				f.Row().Width(28).Height(28).Radius(t.Radii.SM).Background(bg).
					Justify(flex.Center).Items(flex.Center).Children(func() {
					f.Text(firstRune(c.Label)).FontSize(RowDescFontSize).Color(t.LabelPrimary)
				})
			}
		})
	case ItemButton:
		f.Row().Height(ShortcutButtonHeight).MinWidth(ShortcutButtonMinWidth).
			MaxWidth(ShortcutButtonMaxWidth).Radius(t.Radii.MD).PadX(ShortcutButtonPadX).
			Items(flex.Center).Justify(flex.Center).Background(t.BgLayer3).Children(func() {
			f.Text("编辑快捷键").FontSize(ShortcutDescFontSize).Color(t.LabelPrimary)
		})
	case ItemVersion:
		// 官方：`当前版本：` + build 版本（缺失时整行不渲染，见 BuildPanel）。
		f.Text("当前版本：" + row.Display()).FontSize(VersionRowFontSize).
			LineHeight(VersionRowLineHeight).Color(t.LabelPrimary)
	default: // ItemSelector
		f.Row().Height(SelectorHeight).Radius(t.Radii.MD).PadX(SelectorPadX).
			Gap(SelectorGap).Items(flex.Center).Background(t.BgLayer3).Children(func() {
			f.Text(row.Display()).FontSize(RowTitleFontSize).Color(t.LabelPrimary)
			f.Text("›").FontSize(SelectorChevron).Color(t.LabelTertiary)
		})
	}
}

// firstRune 取首字符（cube 里的单字标签）。
func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// activateRow 执行一行（点击 / Enter）。写操作**异步**，不阻塞绘制线程。
func (p *Plugin) activateRow(row PanelRow) {
	if !row.Writable() {
		return
	}
	go func() { _ = p.applyRowWrite(row) }()
}

// applyRowWrite 同步执行一次行写入（测试直接调它，拿到确定结果）。
//
// 选择器行在骨架里**循环到下一项**（官方是弹出菜单；本仓未实现菜单，报告已登记）。
func (p *Plugin) applyRowWrite(row PanelRow) WriteState {
	o := p.ensureOfficial()
	o.mu.Lock()
	writer := o.writer
	o.mu.Unlock()
	if writer == nil {
		state := ClassifyWriteError(row.NS, row.Path, errNoSettingsWriter)
		p.setWriteState(state)
		return state
	}
	value := p.nextRowValue(row)
	ctx, cancel := context.WithTimeout(context.Background(), panelWriteTimeout)
	defer cancel()
	var state WriteState
	if len(row.Path) > 2 {
		// 多段路径（retryPolicy）走 mutate。
		ops := []apiclient.SettingsOp{apiclient.SettingsSetOp(row.Path, value)}
		state = ApplyMutate(ctx, writer, row.NS, ops, row.Revision)
	} else {
		state = ApplySet(ctx, writer, row.NS, row.Path, value, row.Revision)
	}
	p.setWriteState(state)
	return state
}

// nextRowValue 算出这一行「点一下之后」要写的值。
func (p *Plugin) nextRowValue(row PanelRow) any {
	switch row.Kind {
	case ItemSwitch:
		return row.Selected != "true"
	case ItemSelector, ItemCubes:
		for i, c := range row.Choices {
			if c.Value == row.Selected {
				return row.Choices[(i+1)%len(row.Choices)].Value
			}
		}
		if len(row.Choices) > 0 {
			return row.Choices[0].Value
		}
		return ""
	default:
		return row.Value
	}
}

// setWriteState 记录写结果并请求重绘（壳的 Notify 会 Invalidate）。
func (p *Plugin) setWriteState(state WriteState) {
	o := p.ensureOfficial()
	o.mu.Lock()
	o.write = state
	notify := o.notify
	o.mu.Unlock()
	if notify == nil {
		notify = p.notify
	}
	if notify != nil && state.Render() != "" {
		notify(state.Render())
	}
}

// handleOfficialKey 是官方面板的键盘路由。
//
// 返回 true 表示消费了这个键（后面插件看不到）。
func (p *Plugin) handleOfficialKey(ev ui.KeyEvent) bool {
	if ev.Release {
		return false
	}
	o := p.ensureOfficial()
	shown := p.OfficialShown()

	// 入口：Shift+F2（与旧浮层同一个键，产品里只会有其中一个生效）。
	if ev.Key == ui.KeyF2 {
		if ev.Modifiers != ui.ModifierShift {
			return false
		}
		if shown {
			p.CloseOfficial()
			return true
		}
		return p.OpenOfficial()
	}
	if !shown {
		return false
	}
	// 模态层：Escape / Tab / Shift+Tab。
	if o.modal.HandleKey(ev) {
		return true
	}
	if ev.Modifiers != 0 {
		return false
	}
	switch ev.Key {
	case ui.KeyDown:
		p.moveOfficialCursor(1)
		return true
	case ui.KeyUp:
		p.moveOfficialCursor(-1)
		return true
	case ui.KeyEnter:
		p.activateOfficialCursor()
		return true
	}
	return false
}

// moveOfficialCursor 在可写行之间移动键盘光标。
func (p *Plugin) moveOfficialCursor(delta int) {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	var writable []int
	for i, row := range o.rows {
		if row.Writable() {
			writable = append(writable, i)
		}
	}
	if len(writable) == 0 {
		return
	}
	pos := -1
	for i, idx := range writable {
		if idx == o.cursor {
			pos = i
			break
		}
	}
	if pos < 0 {
		if delta > 0 {
			o.cursor = writable[0]
		} else {
			o.cursor = writable[len(writable)-1]
		}
		return
	}
	pos = (pos + delta + len(writable)) % len(writable)
	o.cursor = writable[pos]
}

// activateOfficialCursor 执行键盘光标所在行。
func (p *Plugin) activateOfficialCursor() {
	o := p.ensureOfficial()
	o.mu.Lock()
	var row PanelRow
	ok := o.cursor >= 0 && o.cursor < len(o.rows)
	if ok {
		row = o.rows[o.cursor]
	}
	o.mu.Unlock()
	if ok {
		p.activateRow(row)
	}
}

// ── 报告/测试用的小工具 ────────────────────────────────────────────

// PanelRowIDs 返回一页的行 id 列表（顺序即渲染顺序）。
func PanelRowIDs(section PanelSection) []string {
	out := make([]string, 0, len(section.Rows))
	for _, r := range section.Rows {
		out = append(out, r.ID)
	}
	return out
}

// RenderedTexts 把一页渲染成文本行（测试断言「未知」/「不出现 0」用）。
func RenderedTexts(section PanelSection) []string {
	out := make([]string, 0, len(section.Rows)*2)
	for _, r := range section.Rows {
		line := r.Title
		if r.Desc != "" {
			line += " · " + r.Desc
		}
		line += " = " + r.Display()
		out = append(out, strings.TrimSpace(line))
	}
	return out
}
