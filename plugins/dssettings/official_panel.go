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
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 键盘焦点环的几何：官方 `:focus-visible{outline:var(--dsw-focus-ring-width)
// solid …; outline-offset:2px}`（`index-CSAdOBx6.css`）。viva 的
// `RoundedOutline` 画在矩形上、没有 outline-offset，所以按 offset 把矩形外扩。
const (
	SettingsFocusRingWidth  float32 = 2
	SettingsFocusRingOffset float32 = 2
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
	// roster 是 `agentPresets.list` 的最后一帧结果（nil = 壳没接那条能力）。
	//
	// 为什么要留一份：写完「设为默认」后壳会重拉名册，但**本帧的模型**是用
	// 同步快照算出来的；把它存在这里，才能让"写成功 → 再算一次模型"这条路径
	// 与绘制用同一份来源，不必再问一次壳。
	roster *apiclient.AgentPresetRoster
	// agentPresetRefresh 是壳的"重读名册"入口（写成功后触发一次）。
	agentPresetRefresh func()

	// modelsEdit 是唯一展开中的 provider retryPolicy 草稿。字段点击只改它；
	// 只有 Apply 行会经 writer 发一次 settings.mutate。
	modelsEdit           *retryEditorState
	nextModelsGeneration uint64
	// modelsCatalog 是"模型目录"编辑卡的草稿（nil = 卡关闭）。
	// 与 modelsEdit 分开：那张卡写的是 provider 的设置文档（retryPolicy/凭据），
	// 这张卡写的是宿主的模型目录（`models.catalog.*`），两个库、两套 revision。
	modelsCatalog         *catalogEditor
	nextCatalogGeneration uint64
	// catalogSnapshot 是这一帧的宿主目录快照（开卡时用它建草稿）。
	catalogSnapshot *apiclient.ModelCatalogSnapshot
	// modelsPage 是这一帧的三路只读目录快照（开卡时用它的 provider 事实算协议候选
	// 与 baseURL 占位；不参与写入）。
	modelsPage *apiclient.ModelsPageSnapshot
	// catalogWritable 表示壳接上了目录写口（判据是 `replace` 在：它才是官方
	// 整数组写入语义，删除/排序/新增都走它）。
	catalogWritable bool
	// 目录 DataPlane 钩子（nil = 老壳）。
	catalogReplace   func([]json.RawMessage, *int64) (apiclient.ModelCatalogDocument, error)
	catalogDiscover  func(string, apiclient.ModelDiscoveryRequest) ([]apiclient.DiscoveredModel, error)
	catalogUpsert    func(json.RawMessage, *string, *int64) (apiclient.ModelCatalogDocument, error)
	catalogReorder   func([]string, *int64) (apiclient.ModelCatalogDocument, error)
	catalogTest      func(string) (apiclient.ModelCatalogTestResult, error)
	catalogSetActive func(string, *int64) (apiclient.ModelCatalogSetActiveResult, error)
	catalogRefresh   func()
	// catalogInvalidate 是同步时记下的 Invalidate：非绘制路径（点行头、写失败收尾）
	// 要靠它要下一帧。
	catalogInvalidate func()
	// catalogFields/catalogSeen 是目录卡里的输入框登记与上一帧文本（窗口线程独有，
	// 与 modelsKeyField 同一约束：Viva 的 Field 是输入法/光标/选区的宿主）。
	catalogFields map[string]*ui.Field
	catalogSeen   map[string]string
	// credentialSet 是壳的 `credentials.set`（官方 `operations.storeCredential`，
	// operations.ts:78-81）。nil = 壳没接：密钥行这时不画（不画点了没反应的框）。
	//
	// 为什么单独一路而不是塞进 SettingsWriter：凭据与设置文档是**两个库、两套拒绝
	// 语义**（设置只读时凭据仍可写），官方也是两条 Remote 命名空间。
	credentialSet func(ref, value string) error
	// credentialUnset 是壳的 `credentials.unset`（官方 `operations.removeCredential`，
	// `OFF:operations.ts:82-85`）。nil = 壳没接 → 删除这一步报"壳未接"，
	// **不**把删除静默当成成功（那会让用户以为密钥没了，其实还在库里）。
	credentialUnset func(ref string) error
	// modelsKeyClearPending 表示"这次 Apply 已经把密钥写进凭据库了，下一个窗口
	// 线程帧把输入框清空"。
	//
	// 为什么不在写成功的 goroutine 里清：Viva 的 ui.Field 只能在窗口线程读写
	// （`ui/field.go` 的 Field 注释），跨线程 SetText 是数据竞争。
	modelsKeyClearPending bool
	// modelsKeyLastDraft 是上一绘制帧读到的草稿，用来判"这一帧被清空了"——
	// 官方 `onChange` 的 `''` 分支（ProviderEditor.tsx:507-509）就是"草稿变空
	// 且已有本地存储密钥 → 请求清除"。只在绘制线程与加锁路径读写。
	modelsKeyLastDraft string

	// rows 是当前 section 的可点行（键盘光标用）。
	rows   []PanelRow
	cursor int

	// 内置插件页的交互态（官方那几个 `useState`，见 official_plugins.go 的 PluginsView）。
	//
	// 为什么放在面板状态里而不是模型里：模型每帧从数据面重算，交互态必须跨帧活着
	// （展开的卡片、搜索串、段的开合）。
	// pluginsSearchField 是「内置插件」页的搜索框。
	pluginsSearchField ui.Field
	// modelsKeyField 是 Models 页 provider 卡里的 API 密钥输入框。
	//
	// 为什么和 editor 草稿分开：`retryEditorState` 每帧 clone、可并存多代，而
	// Viva 的 ui.Field 是**窗口线程上的唯一一份**（输入法/光标/选区的宿主）。
	// 所以草稿文本活在 Field 里，`modelsEdit` 只保存"写哪、写什么引用"这类元信息。
	// Secure 恒为 true（官方 `type={revealed ? 'text' : 'password'}` 的默认态，
	// ApiKeyInput.tsx:38）；显隐钮在 draw 里翻它。
	modelsKeyField    ui.Field
	pluginsGlobalOpen *bool
	pluginsExpanded   string
	// pluginsActiveTab 是 `settings.plugins.tab` 里选中的那一项（空串 = 第一项）。
	pluginsActiveTab string
	// pluginsCursorKey 是键盘光标所在的卡片 rowKey（空串 = 还没有光标）。
	pluginsCursorKey string

	// layout / titleID 是上一帧算出来的几何与标题 id（测试与 AfterDraw 用）。
	layout  GridLayout
	titleID string

	notify func(string)

	// session-log-deepseek 的 shell.overlay Toast（官方 UploadToast）。
	// generation 让旧定时器不能清掉后一次写的通知。
	sessionLogBusy         bool
	sessionLogToast        string
	sessionLogToastSuccess bool
	sessionLogToastTimer   *time.Timer
	sessionLogGeneration   uint64

	// 账号页：官方 AccountMenu 在 Settings false→true 边沿刷新一次。
	accountOpenRefreshed bool
	accountBusy          bool
	// avatar 是官方 `AccountAvatarLoader` 的插件生命周期唯一一份
	// （ui-settings-account/src/client/index.ts:61-62：侧栏与设置页共读同一张图）。
	// 零值可用的判据是 `snapshot==nil`；懒建见 accountAvatarLoader。
	avatar *accountAvatarLoader
	// card 是上一帧画出来的设置面板卡片（AfterDraw 里取它布局后的矩形画官方
	// `--dsw-elevation-prominent`）。卡片自己 `.Clip()`，阴影不能在 Draw 钩子里画。
	// 只在绘制线程读写，所以不走 mu。
	card      ui.Box
	cardShown bool
	// pendingFocus 是"模态层的 Tab 已经算出下一个可聚焦 id，但那份账只有插件
	// 认得"的交接位：`ModalLayer.Focused()` 给的是插件侧字符串 id，而真正把
	// 焦点画出来、让 Enter/Space 落到它身上的，是渲染器的焦点
	// （`gfx.Canvas.RequestFocus`）。所以 Tab 之后把 id 放在这里，下一帧
	// `drawOfficialPanel` 提交给渲染器。
	//
	// 套 mu：写它的是**按键线程**（`handleOfficialKey`），读它的是**绘制线程**；
	// 与 model/activeID 同一档，不套就是 -race 门禁下的数据竞争。
	pendingFocus string
	// catalogDrag 是模型目录行的拖放状态（official_models_catalog_drag.go）。
	// 与 card 同一档：只有窗口线程的绘制/指针/AfterDraw 读写它，故意不套 mu。
	catalogDrag catalogDragState
}

// accountAvatar 是头像加载器的懒建出口（零值可用）。
//
// 与官方一致：**一个插件一份**，侧栏启动器（dsnotifications 经 Accountsnapshot）
// 与设置页读同一张图，两次同 URL 只下一份（account-avatar.ts:27）。
func (p *Plugin) accountAvatar() *accountAvatarLoader {
	o := p.ensureOfficial()
	o.mu.Lock()
	if o.avatar != nil {
		loader := o.avatar
		o.mu.Unlock()
		return loader
	}
	// 官方 `publish` 的等价物是壳的重画入口（Notify → Invalidate）。
	var invalidate func()
	if o.notify != nil {
		invalidate = func() { o.notify("") }
	}
	o.avatar = &accountAvatarLoader{invalidate: invalidate}
	loader := o.avatar
	o.mu.Unlock()
	return loader
}

// accountAvatarSnapshot 取当前头像加载状态（nil = 没有图 / 未登录时的 undefined）。
func (p *Plugin) accountAvatarSnapshot() *AccountAvatarSnapshot {
	o := p.ensureOfficial()
	o.mu.Lock()
	loader := o.avatar
	o.mu.Unlock()
	if loader == nil {
		return nil
	}
	return loader.snapshotValue()
}

// syncAccountAvatar 按官方 index.ts:84 把 profile 的 avatarUrl 交给加载器。
//
// 官方只在 `result.value?.status === 'ready'` 时 `avatar.setUrl(value.avatarUrl)`；
// 未就绪/为 null 时**不动**加载器（不是清空）——profile 失败不该把已有头像打掉。
func (p *Plugin) syncAccountAvatar(snapshot apiclient.AccountSnapshot) {
	if snapshot.Profile == nil || snapshot.Profile.Status != "ready" || snapshot.Profile.Value == nil {
		return
	}
	url := ""
	if snapshot.Profile.Value.AvatarURL != nil {
		url = *snapshot.Profile.Value.AvatarURL
	}
	p.accountAvatar().setURL(url)
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
	inventory := (*apiclient.PluginInventoryResult)(nil)
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
	// 凭据域是**独立**的一路能力（官方 `remote.credentials` 与 `remote.settings`
	// 两条命名空间）：设置写接上了不代表凭据也能写，所以单独判 nil。
	credentialSet := env.Act.CredentialSet
	// 删与写各判一次 nil：宿主可以只实现一半（见 Actions.CredentialUnset）。
	credentialUnset := env.Act.CredentialUnset
	// 内置插件页的数据面是**另一条**宿主能力（官方 `pluginInventory.list`），
	// 与 describe 各自独立：describe 在、清单没接时，那一页显示"未知·宿主未接"。
	if env.Act.PluginInventory != nil {
		inventory = env.Act.PluginInventory()
	}
	// 「Agent 预设」页同理是**第三条**独立能力（官方 roster 不在 describe 里）。
	// nil = 宿主没接 `agentPresets.list` -> 那一页显示「未知」，不画空网格。
	roster := (*apiclient.AgentPresetRoster)(nil)
	refresh := (func())(nil)
	if env.Act.AgentPresetRoster != nil {
		roster = env.Act.AgentPresetRoster()
		refresh = env.Act.AgentPresetRefresh
	}
	var account *apiclient.AccountSnapshot
	if env.Act.AccountSnapshot != nil {
		snapshot := env.Act.AccountSnapshot()
		account = &snapshot
		// 官方 index.ts:84：profile ready 时把 avatarUrl 交给加载器（同一份图
		// 侧栏与设置页共用）。URL 没变时 setURL 是空操作（account-avatar.ts:27），
		// 所以每帧调是安全的。
		p.syncAccountAvatar(snapshot)
	}

	model := BuildPanelWithSources(describe, facts, inventory, o.pluginsView(), roster)
	var modelsSnapshot *apiclient.ModelsPageSnapshot
	if env.Act.ModelsPageSnapshot != nil {
		modelsSnapshot = env.Act.ModelsPageSnapshot()
	}
	// 模型目录编辑器是**第四条**独立能力：官方没有这条命名空间（官方写 provider
	// profile 的 `models`，本仓宿主不发布它），等价真值是 `models.catalog.*`（#41）。
	// nil = 老壳 → 那一卡整块不画（画一张"读取失败"的卡只会让人以为宿主坏了）。
	var catalogSnapshot *apiclient.ModelCatalogSnapshot
	if env.Act.ModelsCatalogSnapshot != nil {
		catalogSnapshot = env.Act.ModelsCatalogSnapshot()
	}
	catalogWritable := env.Act.ModelsCatalogReplace != nil
	o.mu.Lock()
	modelsEdit := o.modelsEdit.clone()
	catalogEdit := o.modelsCatalog
	o.mu.Unlock()
	for index := range model.Sections {
		if model.Sections[index].ID == "models" {
			model.Sections[index] = buildModelsPage(model.Sections[index], describe, nil, modelsSnapshot, modelsEdit,
				env.Act.CredentialStatus)
			model.Sections[index] = appendModelsCatalogEditor(model.Sections[index], catalogSnapshot, catalogEdit, catalogWritable)
		}
		if model.Sections[index].Account != nil && account != nil {
			model.Sections[index].Account = account
		}
	}

	o.mu.Lock()
	o.describe = describe
	o.facts = facts
	o.writer = writer
	o.credentialSet = credentialSet
	o.credentialUnset = credentialUnset
	o.roster = roster
	o.agentPresetRefresh = refresh
	o.catalogSnapshot = catalogSnapshot
	o.modelsPage = modelsSnapshot
	o.catalogWritable = catalogWritable
	o.catalogReplace = env.Act.ModelsCatalogReplace
	o.catalogDiscover = env.Act.LLMDiscoverModels
	o.catalogUpsert = env.Act.ModelsCatalogUpsert
	o.catalogReorder = env.Act.ModelsCatalogReorder
	o.catalogTest = env.Act.ModelsCatalogTest
	o.catalogSetActive = env.Act.ModelsCatalogSetActive
	o.catalogRefresh = env.Act.ModelsCatalogRefresh
	o.catalogInvalidate = env.Act.Invalidate
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
	refreshAccount := o.open && account != nil && !o.accountOpenRefreshed
	if refreshAccount {
		o.accountOpenRefreshed = true
	}
	if !o.open {
		o.accountOpenRefreshed = false
	}
	o.mu.Unlock()

	if refreshAccount && env.Act.AccountRefresh != nil {
		env.Act.AccountRefresh()
	}
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
		// 壳更新渠道（2026-11-13 补）。**这三行不能漏**：漏了的话探针照样成功、
		// 事实位照样为真，但转换时被丢掉 -> 面板永远看不到 -> 「更新渠道」行
		// 永远不出现。截图看不出来（那一行在面板很下面），只有端到端测试能钉住。
		ShellChannelServed:  f.ShellChannelServed,
		ShellChannel:        f.ShellChannel,
		ShellChannelFailure: f.ShellChannelFailure,
		ShortcutCount:       f.ShortcutCount,
		SidebarRightTabs:    f.SidebarRightTabs,
		SessionLogServed:    f.SessionLogServed,
		ClientVersion:       f.ClientVersion,
		PermissionCatalog:   f.PermissionCatalog,
		TranscriptView:      f.TranscriptView,
		CredentialStored:    f.CredentialStored,
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
	if w.act.SettingsMutateRevision != nil {
		return w.act.SettingsMutateRevision(ns, ops, expectedRevision)
	}
	if w.act.SettingsMutate == nil {
		return apiclient.SettingsNamespaceView{}, errNoSettingsWriter
	}
	if expectedRevision != nil && *expectedRevision == 0 {
		return apiclient.SettingsNamespaceView{}, &apiclient.APIError{
			Code: apiclient.SettingsCodeInternal, Message: "老壳不能对 revision 0 做乐观锁写",
			Hint: "升级壳的 SettingsMutateRevision；本次未写入",
		}
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
func (p *Plugin) drawOfficialPanel(f ui.C, env *pluginapi.Env) {
	o := p.ensureOfficial()
	p.syncPanel(env)
	o.mu.Lock()
	open := o.open
	model := o.model
	activeID := o.activeID
	write := o.write
	o.mu.Unlock()
	o.cardShown = false
	// 拖放声明跟着绘制帧走：viva 的 DragState 只对本帧声明过的元素成立，
	// 所以每帧开头把上一帧的落点行清掉（official_models_catalog_drag.go 的 beginPaint）。
	// 可写性由本帧的行绘制填（只有真画出目录行才算可落）。
	p.catalogDrag().beginPaint(env)
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
		ui.Row(f).Absolute().Left(0).Top(0).Width(env.Bounds.Width).Height(24).
			Background(ui.RGBA(180, 40, 40, 1)).Children(func() {
			ui.Text(f, "设置面板布局失败: "+err.Error()).FontSize(12).Color(ui.RGBA(255, 255, 255, 1))
		})
		return
	}
	o.mu.Lock()
	o.layout = layout
	o.titleID = "settings-panel-title"
	o.mu.Unlock()

	// 模态层 Tab 的落点交给渲染器：不交的话 Tab 只改了一个没人读的字符串，
	// 焦点在画面上完全看不出来（2026-11-22 实测：Tab 后 `Snapshot().Focused`
	// 仍是零，整块面板没有焦点环）。
	o.mu.Lock()
	pending := o.pendingFocus
	o.pendingFocus = ""
	o.mu.Unlock()
	if pending != "" {
		id := env.Tree.ID(env.Feat, pending)
		if canvas := f.Canvas(); canvas != nil && !id.IsZero() {
			canvas.RequestFocus(id)
		}
	}

	// ① 遮罩（createPortal 到 body 级的等价物：整窗、z-index 1000、拦截点击）。
	//
	// 模态层的**焦点**语义由渲染器承担，不只是插件自己的 Tab 计数：
	// `ModalLayer` 只记录「焦点该在哪个 id 上」，而键盘 Traversal 是 viva 的
	// `Router` 在驱动（Tab 根本不进 `kernel.HandleKey`）。所以这里必须像
	// 右键菜单（`dscontextmenu/context_menu.go:328-336`）那样把这一层推进
	// 一个 focus scope 并 `TrapFocus`，否则 Tab 会走到遮罩**背后**的侧栏行上
	// （实测：设置打开后 Tab 1..14 全在背后，见 official_modal_trap_test.go）。
	//
	// 遮罩自己 `SkipFocus`：它是整窗的关闭面，不该成为 Tab 的停靠点；`Backdrop`
	// 表明它盖住窗口但不挡窗口拖动，与右键菜单的 scrim 同一语义。
	scope := env.Tree.ID(env.Feat, "settings-modal")
	mask := ui.Row(f).Absolute().Inset(0).Background(ui.RGBA(0, 0, 0, 0.24)).
		HitWith(env.Tree.ID(env.Feat, "settings-mask"), "关闭设置", gfx.HitOptions{
			Role: gfx.RoleButton, SkipFocus: true, Backdrop: true,
		})
	if !scope.IsZero() {
		mask = mask.
			Draw(func(d *ui.DrawContext) {
				d.Canvas.PushFocusScope(scope)
				d.Canvas.TrapFocus(scope)
				d.Canvas.ReturnFocus(scope)
			}).
			DrawOver(func(d *ui.DrawContext) { d.Canvas.PopFocusScope() })
	}
	mask.Children(func() {
		// ② 卡片：四区网格的相对坐标（children 坐标相对卡片 padding box）。
		// 阴影 = 官方 `--dsw-elevation-prominent`，描边走默认档 `border-l4`：
		// `SettingsRoot.module.css:110`
		// `.AnXs9G_panel{box-shadow:var(--dsw-elevation-prominent)}`——官方这张面板
		// **没有**重绑 `--dsw-elevation-stroke-color`，所以用默认
		// `--dsw-alias-border-l4`（gradient-shadow-text.css:17）。官方也没给它
		// `border`：那圈 0.5px 分离线在 elevation 的描边层里，原来的
		// `Border(1, BorderL2)` 是多出来的一圈占布局描边。
		//
		// 画法在 AfterDraw（见 afterOfficialPanel）：卡片自己 `.Clip()`，
		// Draw 钩子会被那道裁剪吞掉。
		card := ui.Row(f).Absolute().Left(layout.X).Top(layout.Y).
			Width(layout.Width).Height(layout.Height).
			Radius(t.Radii.Panel).Background(t.BgLayer2).Clip()
		o.card, o.cardShown = card, true
		card.Children(func() {
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

// afterOfficialPanel 在布局完成之后覆画设置面板的官方 elevation。
//
// 为什么不在 Draw 钩子里画：官方 `.AnXs9G_panel{overflow:hidden}`，本仓对应
// `.Clip()`；viva 对带 Clip 的元素是**先推 Clip、再跑 Draw 钩子**
// （`ui/paint.go:50-52` 与 `:132`），阴影记进 canvas 时带着那道裁剪，而阴影又被
// 自己的投射矩形挖空（`gfx/canvas.go:78-80`），相减是空集——等于没画（#150 实测）。
// AfterDraw 是布局后的覆画阶段（`pluginapi/plugin.go:286-292`），裁剪栈已清空。
func (p *Plugin) afterOfficialPanel(canvas *gfx.Canvas, env *pluginapi.Env) {
	o := p.ensureOfficial()
	// card / cardShown 只在绘制线程读写（Draw 与 AfterDraw 同一线程），与
	// model/activeID 那些跨线程字段不同，故意不套 mu——套一半才是错的。
	box, shown := o.card, o.cardShown
	// `Valid()`：面板没画的帧与跨帧的旧 Box 都不能取 Rect（零值 Box 会 panic）。
	if !shown || !box.Valid() {
		return
	}
	rect := box.Rect()
	if rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	t := p.theme
	if env.Theme != nil {
		t = env.Theme
	}
	dscore.PaintElevationOverlay(canvas, rect, t.Radii.Panel, t.Shadows.Prominent, t.BorderL4)
	// 模型目录行的拖放视觉（落点插入线 + 拖拽预览）也只能在这里画：两者都落在
	// 自己那个盒子**外面**，Draw 钩子里的会被 viva 的逐元素裁剪吃掉
	// （official_models_catalog_drag.go 的 paintCatalogDragOverlay）。
	p.paintCatalogDragOverlay(canvas, env, t, rect)
	// 键盘焦点环：官方 `:focus-visible{outline:var(--dsw-focus-ring-width) solid
	// var(--dsw-focus-ring-color, var(--dsw-alias-state-business-primary))}`
	// （`index-CSAdOBx6.css`）。面板里的行有很多种画法（选择器/开关/cube 网格/
	// 按钮/版本/模型目录），逐行挂 `Focus(...)` 一定会漏掉某一种；这里按
	// **命中区矩形**统一画一圈，任何能聚焦的控件都覆盖到，且只在键盘可见时画
	// （`FocusVisible` 就是 `:focus-visible`，鼠标点出来的焦点不画环）。
	if id := canvas.FocusVisible(); !id.IsZero() {
		for _, hit := range canvas.Hits() {
			if hit.ID != id || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				continue
			}
			cx, cy := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			if cx < rect.X || cy < rect.Y || cx >= rect.X+rect.Width || cy >= rect.Y+rect.Height {
				continue // 焦点不在面板里（例如面板刚关）：别在别处画环
			}
			b := hit.Bounds
			ring := gfx.Rect{X: b.X - SettingsFocusRingOffset, Y: b.Y - SettingsFocusRingOffset,
				Width: b.Width + 2*SettingsFocusRingOffset, Height: b.Height + 2*SettingsFocusRingOffset}
			canvas.RoundedOutline(ring, t.Radii.MD, SettingsFocusRingWidth, t.Link)
			break
		}
	}
}

// relArea 把绝对矩形转成卡片内相对矩形。
func relArea(a AreaRect, layout GridLayout) AreaRect {
	return AreaRect{X: a.X - layout.X, Y: a.Y - layout.Y, Width: a.Width, Height: a.Height}
}

// drawNavTitle 画左标题（aria-labelledby 指向它）。
func (p *Plugin) drawNavTitle(f ui.C, t *pluginapi.Theme, a AreaRect) {
	ui.Row(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(NavTitlePadTop, NavTitlePadRight, NavTitlePadBottom, NavTitlePadLeft).
		Items(ui.Center).Children(func() {
		ui.Text(f, "设置").FontSize(NavTitleFontSize).LineHeight(NavTitleLineHeight).
			Weight(NavTitleWeight).Color(t.LabelPrimary)
	})
}

// drawPanelHeader 画右列头：action 区（空）+ 关闭按钮。
func (p *Plugin) drawPanelHeader(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect) {
	ui.Row(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(HeaderPadTop, HeaderPadRight, HeaderPadBottom, HeaderPadLeft).
		Gap(HeaderGap).Items(ui.Center).Children(func() {
		ui.Row(f).Grow(1)
		close := ui.Row(f).Size(CloseSize, CloseSize).Radius(t.Radii.SM).
			Items(ui.Center).Justify(ui.Center).
			Hover(ui.Look{}.Background(t.Hover)).
			Hit(env.Tree.ID(env.Feat, "settings-close"), "关闭").Children(func() {
			ui.Icon(f, closeOutlineIcon()).Size(CloseIconSize, CloseIconSize).Color(t.LabelPrimary)
		})
		if close.Clicked() {
			p.CloseOfficial()
		}
	})
}

// drawPanelNav 画左导航（settings.section list）。
func (p *Plugin) drawPanelNav(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, model PanelModel, activeID string) {
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(NavPadTop, NavPadX, 0, NavPadX).Gap(NavListGap).Children(func() {
		for _, section := range model.Sections {
			section := section
			active := section.ID == activeID
			bg := ui.Color{}
			if active {
				bg = t.NavActive
			}
			if ui.Row(f).Height(NavCellHeight).Radius(t.Radii.MD).
				PadSides(NavCellPadTop, NavCellPadRight, NavCellPadBottom, NavCellPadLeft).
				Gap(NavCellGap).Items(ui.Center).Background(bg).Hover(ui.Look{}.Background(t.Hover)).
				Hit(env.Tree.ID(env.Feat, "settings-nav-"+section.ID), section.Title).
				Children(func() {
					drawSectionIcon(f, section.Icon, t.LabelPrimary)
					ui.Text(f, section.Title).FontSize(NavCellFontSize).LineHeight(NavCellLineHeight).
						Color(t.LabelPrimary).Grow(1).Ellipsis()
				}).Clicked() {
				p.SelectSection(section.ID)
			}
		}
	})
}

// drawPanelOptions 画右页（active section 的行）。
func (p *Plugin) drawPanelOptions(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, a AreaRect, section PanelSection, write WriteState) {
	if section.Models != nil {
		p.drawModelsPage(f, env, t, a, section, write)
		return
	}
	if section.Account != nil {
		p.drawAccountPage(f, env, t, a, *section.Account)
		return
	}
	// 「内置插件」页有**自己的**渲染面（搜索 + 分组 + 卡片网格 + 展开详情），
	// 不是"标题 + 一行一个控件"那一套；官方也是另一个组件树
	// （`settings.plugins.tab` 里渲染 `PluginInventorySettingsTab`）。
	if section.Plugins != nil {
		p.drawPluginsPage(f, env, t, a, section)
		return
	}
	// 「Agent 预设」页同理是另一套组件树（官方 `AgentPresetSection` 是卡片网格，
	// 卡片本体就是选择控件），不是「标题 + 一行一个控件」那一套。
	if section.AgentPresets != nil {
		p.drawAgentPresetsPage(f, env, t, a, section)
		return
	}
	ui.Column(f).Absolute().Left(a.X).Top(a.Y).Width(a.Width).Height(a.Height).
		PadSides(OptionsPadTop, OptionsPadRight, OptionsPadBottom, OptionsPadLeft).Children(func() {
		if section.Note != "" {
			ui.Text(f, section.Note).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.LabelTertiary).Ellipsis()
		}
		if write.Render() != "" && write.Failure() {
			ui.Text(f, write.Render()).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
				Color(t.Error)
		}
		for i, row := range section.Rows {
			row := row
			// 官方 UploadRow 在 mutation.busy 时禁用 Switch；下一帧由
			// syncPanel 重建宿主真值。
			if row.NS == apiclient.SettingsNSSessionLog && p.sessionLogBusySnapshot() {
				row.Disabled = true
				row.Reason = "保存中…"
			}
			p.drawOfficialRow(f, env, t, row)
			if i < len(section.Rows)-1 {
				// border-bottom:0.5px 属于上一行自身；负 margin 让分隔线
				// 覆盖在上一行底部，不额外撑高通用行。
				ui.Row(f).FillWidth().Height(0.5).MarginTop(-0.5).Background(t.BorderL2)
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
	case ItemCubes:
		return AppearanceRowHeight
	default:
		return RowTitleLineHeight + RowTextGap + RowDescLineHeight + 2*RowPadY
	}
}

// drawOfficialRow 画一行（选择器 / 开关 / stepper / cube / 按钮 / 版本）。
func (p *Plugin) drawOfficialRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow) {
	id := env.Tree.ID(env.Feat, "settings-row-"+row.ID)
	if row.Kind == ItemCubes {
		p.drawAppearanceRow(f, t, row, id)
		return
	}
	if row.Kind == ItemVersion {
		ui.Row(f).FillWidth().Height(OfficialRowHeight(row.Kind)).
			PadSides(VersionRowPadY, 0, VersionRowPadY, 0).Items(ui.Center).Children(func() {
			ui.Text(f, "当前版本："+row.Display()).FontSize(VersionRowFontSize).
				LineHeight(VersionRowLineHeight).Color(t.LabelPrimary)
		})
		return
	}

	height := OfficialRowHeight(row.Kind)
	rowGap := RowGap
	titleLine := RowTitleLineHeight
	descColor := t.LabelTertiary
	if row.Kind == ItemSwitch {
		rowGap = DevToolsRowGap
		titleLine = DevToolsTitleLineHeight
		descColor = t.LabelSecondary
	}
	body := func() {
		text := ui.Column(f).Grow(1).MinWidth(0).Gap(RowTextGap)
		if row.Kind != ItemSwitch {
			text = text.PadRight(RowTextPadRight)
		}
		text.Children(func() {
			ui.Text(f, row.Title).FontSize(RowTitleFontSize).LineHeight(titleLine).
				Color(t.LabelPrimary).Ellipsis()
			if row.Desc != "" {
				ui.Text(f, row.Desc).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
					Color(descColor).Ellipsis()
			}
			if row.Reason != "" {
				ui.Text(f, row.Reason).FontSize(RowDescFontSize).LineHeight(RowDescLineHeight).
					Color(t.Warn).Ellipsis()
			}
		})
		// 字号 stepper 的 hover 与两枚箭头各要自己的命中区（drawFontStepper 里用法）：
		// 官方 `._stepper:hover ._arrows` 按**胶囊**而不是整行判 hover，
		// 两枚 arrow 在官方是独立 button，命中区不能共用（共用会让"点 ↑"变 −1）。
		p.drawRowControl(f, env, t, row, id)
	}
	box := ui.Row(f).FillWidth().Height(height).
		PadSides(RowPadY, 0, RowPadY, 0).Gap(rowGap).Items(ui.Center).
		Hit(id, row.Title).Children(body)
	if row.Writable() {
		if box.Clicked() {
			p.activateRow(row)
		}
	}
}

// drawRowControl 画行尾控件（按 Kind 分支）。
func (p *Plugin) drawRowControl(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, row PanelRow, id gfx.ID) {
	switch row.Kind {
	case ItemSwitch:
		on := row.Known && row.Selected == "true"
		bg := t.BorderL3
		justify := ui.Start
		thumb := t.BgLayer2
		if on {
			bg = t.BrandPrimary
			justify = ui.End
			thumb = t.ButtonPrimaryFg
		}
		control := ui.Row(f).Width(SwitchWidth).Height(SwitchHeight).Radius(SwitchHeight/2).
			Background(bg).PadSides(SwitchPad, SwitchPad, SwitchPad, SwitchPad).
			Justify(justify).Children(func() {
			ui.Row(f).Width(SwitchThumbSize).Height(SwitchThumbSize).
				Radius(SwitchThumbSize / 2).Background(thumb)
		})
		if row.Disabled {
			control.Opacity(0.5)
		}
	case ItemStepper:
		if row.ID == "font-size" {
			// 官方 FontSizeRow.tsx:69-79 的两枚 chevron 各是一次 ±1 的写；
			// 只有行可写且值已知时才挂命中区（否则不画假按钮）。
			ids := fontStepperIDs{
				pill: env.Tree.ID(env.Feat, "settings-stepper-"+row.ID),
				up:   env.Tree.ID(env.Feat, "settings-stepper-"+row.ID+"/increase"),
				down: env.Tree.ID(env.Feat, "settings-stepper-"+row.ID+"/decrease"),
			}
			step := drawFontStepper(f, t, ids, row.Title, row.Display(), row.Writable() && row.Known)
			if step != fontStepperNone {
				p.stepRowValue(row, step)
			}
			break
		}
		ui.Row(f).Height(SelectorHeight).Radius(t.Radii.MD).PadX(SelectorPadX).
			Gap(SelectorGap).Items(ui.Center).Background(settingsModuleFill(t)).Children(func() {
			ui.Text(f, "−").FontSize(RowTitleFontSize).Color(t.LabelSecondary)
			ui.Text(f, row.Display()).FontSize(RowTitleFontSize).Color(t.LabelPrimary)
			ui.Text(f, "+").FontSize(RowTitleFontSize).Color(t.LabelSecondary)
		})
	case ItemCubes:
		ui.Row(f).Gap(SelectorGap).Items(ui.Center).Children(func() {
			for _, c := range row.Choices {
				selected := row.Known && row.Selected == c.Value
				bg := t.BgLayer3
				if selected {
					bg = t.ButtonPrimaryFill
				}
				ui.Row(f).Width(28).Height(28).Radius(t.Radii.SM).Background(bg).
					Justify(ui.Center).Items(ui.Center).Children(func() {
					ui.Text(f, firstRune(c.Label)).FontSize(RowDescFontSize).Color(t.LabelPrimary)
				})
			}
		})
	case ItemButton:
		ui.Row(f).Height(ShortcutButtonHeight).MinWidth(ShortcutButtonMinWidth).
			MaxWidth(ShortcutButtonMaxWidth).Radius(t.Radii.MD).PadX(ShortcutButtonPadX).
			Items(ui.Center).Justify(ui.Center).Background(settingsModuleFill(t)).Children(func() {
			ui.Text(f, "编辑快捷键").FontSize(RowTitleFontSize).LineHeight(RowTitleLineHeight).Color(t.LabelPrimary)
		})
	case ItemVersion:
		// 官方：`当前版本：` + build 版本（缺失时整行不渲染，见 BuildPanel）。
		ui.Text(f, "当前版本："+row.Display()).FontSize(VersionRowFontSize).
			LineHeight(VersionRowLineHeight).Color(t.LabelPrimary)
	default: // ItemSelector
		ui.Row(f).Height(SelectorHeight).Radius(t.Radii.MD).PadX(SelectorPadX).
			Gap(SelectorGap).Items(ui.Center).Background(settingsModuleFill(t)).Children(func() {
			ui.Text(f, row.Display()).FontSize(RowTitleFontSize).LineHeight(RowTitleLineHeight).Color(t.LabelPrimary)
			ui.Icon(f, chevronOutlineIcon()).Size(SelectorChevron, SelectorChevron).Color(t.LabelTertiary)
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
	if p.activateModelsRow(row) {
		return
	}
	if row.NS == apiclient.SettingsNSSessionLog {
		o := p.ensureOfficial()
		o.mu.Lock()
		busy := o.sessionLogBusy
		o.mu.Unlock()
		if busy {
			return
		}
		// 官方 UploadPreference 先发布 busy，再启动 mutation：同一帧的第二次
		// 点击/Enter 也会被挡住，不会叠加两笔 Session Log 设置写。
		p.setWriteState(WriteState{Kind: WriteSaving, NS: row.NS, Path: append([]string(nil), row.Path...)})
	}
	go func() { _ = p.applyRowWrite(row) }()
}

// stepRowValue 是 font-size 一行 ↑/↓ 的写入（官方 FontSizeRow.tsx:69-79 →
// `setFontSize(role, px ± 1)`，区间由 isFontSize 卡在 FONT_SIZE_SPECS 内，
// theme-settings.ts:40-61 的 text min/max）。
//
// 与选择器行的「点一下循环下一项」是两条路：stepper 点边缘只会 ±1，
// 到了官方区间端点就**不发请求**（宿主会以 out_of_range 拒绝，且官方也不发）。
func (p *Plugin) stepRowValue(row PanelRow, step fontStepperStep) {
	if !row.Known || step == fontStepperNone {
		return
	}
	current, err := strconv.Atoi(strings.TrimSpace(row.Value))
	if err != nil {
		return
	}
	next := current + int(step)*int(row.Bounds.Step)
	if row.Bounds.HasMin && float64(next) < row.Bounds.Min {
		return
	}
	if row.Bounds.HasMax && float64(next) > row.Bounds.Max {
		return
	}
	if next == current {
		return
	}
	target := row
	target.Value = strconv.Itoa(next)
	go func() {
		o := p.ensureOfficial()
		o.mu.Lock()
		writer := o.writer
		o.mu.Unlock()
		if writer == nil {
			p.setWriteState(ClassifyWriteError(target.NS, target.Path, errNoSettingsWriter))
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), panelWriteTimeout)
		defer cancel()
		p.setWriteState(ApplySet(ctx, writer, target.NS, target.Path, next, target.Revision))
	}()
}

// applyRowWrite 同步执行一次行写入（测试直接调它，拿到确定结果）。
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
	if row.NS == apiclient.SettingsNSModel && row.Field == "defaultModel" && value == "" {
		// defaultModel 的 MinLength=1；空选项是清除 user 覆盖，不是写空串。
		state = ApplyReset(ctx, writer, row.NS, row.Path, row.Revision)
	} else if len(row.Path) > 2 {
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
	p.updateSessionLogWrite(state)
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
func (p *Plugin) handleOfficialKey(ev gfx.KeyEvent) bool {
	if ev.Release {
		return false
	}
	o := p.ensureOfficial()
	shown := p.OfficialShown()

	// 入口：Shift+F2（与旧浮层同一个键，产品里只会有其中一个生效）。
	if ev.Key == gfx.KeyF2 {
		if ev.Modifiers != gfx.ModifierShift {
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
		if ev.Key == gfx.KeyTab {
			// Tab 只算出"下一个可聚焦 id"，画面上要真的看得见：交给渲染器。
			o.mu.Lock()
			o.pendingFocus = o.modal.Focused()
			o.mu.Unlock()
		}
		return true
	}
	if ev.Modifiers != 0 {
		return false
	}
	switch ev.Key {
	case gfx.KeyDown, gfx.KeyUp:
		delta := 1
		if ev.Key == gfx.KeyUp {
			delta = -1
		}
		// 插件页的"行"是卡片（`o.rows` 对它为空），所以按卡片走。
		if section, ok := p.activeSectionModel(); ok && section.Plugins != nil {
			p.movePluginsCursor(section, delta)
			return true
		}
		p.moveOfficialCursor(delta)
		return true
	case gfx.KeyEnter:
		if section, ok := p.activeSectionModel(); ok && section.Plugins != nil {
			p.activatePluginsCursor(section)
			return true
		}
		p.activateOfficialCursor()
		return true
	}
	return false
}

// activeSectionModel 取当前 active section 的模型（键盘分流用）。
func (p *Plugin) activeSectionModel() (PanelSection, bool) {
	o := p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.model.Section(o.activeID)
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

// pluginsView 读这一帧的交互态（官方那几个 useState）。
func (o *officialPanel) pluginsView() PluginsView {
	o.mu.Lock()
	defer o.mu.Unlock()
	return PluginsView{
		Query:      o.pluginsSearchField.Text(),
		GlobalOpen: o.pluginsGlobalOpen,
		Expanded:   o.pluginsExpanded,
		ActiveTab:  o.pluginsActiveTab,
	}
}

// pluginCardFocused 报告某张卡是不是键盘光标所在（画焦点环用）。
func (p *Plugin) pluginCardFocused(rowKey string) bool {
	o := p.official
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pluginsCursorKey != "" && o.pluginsCursorKey == rowKey
}

// visiblePluginCards 把当前页的可见卡片拍平（键盘光标按它走）。
//
// **只算展开那一段里的卡**：收起段里的卡在屏幕上不存在，光标不该停在看不见的东西上。
func visiblePluginCards(section PanelSection) []PluginCard {
	if section.Plugins == nil {
		return nil
	}
	var out []PluginCard
	for _, group := range section.Plugins.Groups {
		if !group.Open {
			continue
		}
		out = append(out, group.Cards...)
	}
	return out
}

// movePluginsCursor 在可见卡片之间移动键盘光标。
//
// 与 moveOfficialCursor 同一个形状（-1 = 还没有光标）；插件页的"行"是卡片，
// 所以另走一条——`o.rows` 对插件页是空的。
func (p *Plugin) movePluginsCursor(section PanelSection, delta int) {
	o := p.ensureOfficial()
	cards := visiblePluginCards(section)
	if len(cards) == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	current := -1
	for i, card := range cards {
		if card.RowKey == o.pluginsCursorKey {
			current = i
			break
		}
	}
	if current < 0 {
		if delta > 0 {
			o.pluginsCursorKey = cards[0].RowKey
		} else {
			o.pluginsCursorKey = cards[len(cards)-1].RowKey
		}
		return
	}
	next := (current + delta + len(cards)) % len(cards)
	o.pluginsCursorKey = cards[next].RowKey
}

// activatePluginsCursor 展开/收起键盘光标所在的那张卡（官方卡片是按钮，Enter = 点它）。
func (p *Plugin) activatePluginsCursor(section PanelSection) {
	o := p.ensureOfficial()
	o.mu.Lock()
	key := o.pluginsCursorKey
	if key == "" {
		o.mu.Unlock()
		return
	}
	if o.pluginsExpanded == key {
		o.pluginsExpanded = ""
	} else {
		o.pluginsExpanded = key
	}
	o.mu.Unlock()
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
