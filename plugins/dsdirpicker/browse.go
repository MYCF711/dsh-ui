// Package dsdirpicker 是官方 `dsh-client-ui-directory-picker-browse` 的 Go 版：
// 「选择工作区目录」模态对话框（680×500，Miller 双列目录浏览器）。
//
// # 结构证据（唯一权威，几何/文案/结构全部照它）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-directory-picker-browse/src/client
//
//	OFF:DirectoryBrowser.tsx          1052 行，结构与交互
//	OFF:DirectoryBrowser.module.css   455 行，全部几何值
//	OFF:index.ts                      zh locale 的 13 个键（本文件只许用这 13 条文案）
//	OFF:flow.ts                       43 行，流程注入面
//
// # 它要壳注入什么（**接线时照这个来**）
//
//  1. 数据面：`SetSource(DirSource)`。source 是 `*apiclient.Client` 的薄包装，
//     实现 `List` / `Create` / `Register` 三条（宿主方法名见 dirpicker.go）。
//     **必须在 `Load` 之后注入**——那之前 apiclient 还没拿到宿主地址。
//     为什么走包级注入而不是 `Register` 里 `ctx.Inject`：插件拿不到
//     `*apiclient.Client`（界面线 R2/R7：插件不建 Transport、不直连宿主），
//     而 `pluginapi` 的 Env 里没有"目录浏览"这一格（那一格要改公共契约，
//     不在本次任务范围）。与 `plugins/dsagentteam` 的 `AttachShellTier` 同一个
//     时序与理由：都是"Load 完成之后才解析得出"的东西，壳调一次即可。
//  2. 打开：`(*Plugin).Open() bool`。壳按 `kernel.Plugins()` 做类型断言调用
//     （`interface{ Open() bool }`，与 `AttachShellTier` 同一种做法）。
//     返回 false = 没有 source，**没打开**（不许画一个点了没反应的入口）。
//  3. 选定回调：`SetOnPicked(func(path string))`。用户点「打开」→ 本插件对
//     当前目录调 `workspace.create`，成功后才回调它；壳在这里切到新工作区。
//  4. 文案/键盘：实现 `Fields()` 把「新建文件夹」的输入框交给壳登记
//     （不登记收不到键盘，见 pluginapi.Fields）。
//
// **没有 source 时整块不画**（一个节点都不画）：这与官方"空槽不占位"同义。
// 画死按钮比不画更坏——用户会以为点了就能选目录。
//
// # 交互（照 OFF:DirectoryBrowser.tsx）
//
//   - 点左列某行 = 选中它，右列列它的子层（`:506-533` 的 select）
//   - 点右列某行 = 那一层成为左列，再往下一层（`:566-571` 的 advance）
//   - 点面包屑 = 回到那一层（`:828-837` 的 navigate）
//   - 「打开」= 对 targetPath 调 `workspace.create`，成功后关闭并回调壳
//   - 「新建文件夹」= 弹子对话框，`createDirectory` 成功后**进入新目录**
//   - 「显示隐藏文件」= 纯客户端切换（`:292` 的 showHidden，默认关）
//   - 请求中显示「加载中…」；`truncated` 为真显示「文件夹过多，仅显示开头部分。」
//   - 宿主错误**原文**显示在 `.error` 位置，不编文案
//
// # 没搬的官方细节（照实登记，不假装搬了）
//
//   - 路径编辑区（pencil + `.pathInput`，`:839-923`）与它的 250ms 防抖、草稿前缀
//     过滤：那是另一整套状态机（`:142-218`、`:673-686`），不在本次范围。
//   - `SLOW_SCAN_DELAY_MS=300`（`:91`）的"慢扫描才显示加载中"门：本仓没有定时器
//     链路，请求一出就显示「加载中…」——宁可早显示，也不假装有门。
//   - `PARENT_LEG_WAIT_MS=200`（`:100`）的两腿同时落地：本仓每次导航只发一个
//     请求（不做"目标 + 父层"双腿），所以没有闪烁问题也就没有这个窗口。
//   - 图标（folder / chevron / plus / check / pencil 都是 sprite SVG）：按仓规
//     **不手画 SVG**，只保留官方那两处槽宽（行首 16、行尾 12）。
package dsdirpicker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ── 文案：只有这 13 条，逐字来自 OFF:index.ts 的 zh 字典 ──────────────────
//
// **不许新写中文**：界面上一旦出现这 13 条以外的字样，就不是官方那个对话框了。
const (
	zhTitle          = "选择工作区目录"         // browser.title
	zhHome           = "主目录"             // browser.home
	zhNewFolder      = "新建文件夹"           // browser.newFolder
	zhFolderName     = "文件夹名称"           // browser.folderName（输入框的无障碍名）
	zhCreateIn       = `在"{name}"中新建文件夹` // browser.createIn
	zhUntitledFolder = "未命名文件夹"          // browser.untitledFolder（placeholder）
	zhCreate         = "创建"              // browser.create
	zhCancel         = "取消"              // browser.cancel
	zhOpen           = "打开"              // browser.open
	zhEditPath       = "编辑路径"            // browser.editPath（本版没有路径编辑器，见包注释）
	zhLoading        = "加载中…"            // browser.loading
	zhTruncated      = "文件夹过多，仅显示开头部分。"  // browser.truncated
	zhShowHidden     = "显示隐藏文件"          // browser.showHidden
)

// ── 几何：全部来自 OFF:DirectoryBrowser.module.css，行号逐个标了 ─────────────
const (
	// 卡片：`.dialog.dialog{width:min(680px,100%);height:min(500px,calc(100dvh - 32px))}`
	// （:5-7）。
	dialogMaxWidth      float32 = 680
	dialogMaxHeight     float32 = 500
	dialogViewportInset float32 = 32  // calc(100dvh - 32px)
	hairlineWidth       float32 = 0.5 // 官方到处用的 0.5px 分隔线（:32 / :236 / :357）

	// header：`.header{padding:16px 14px 8px 24px;gap:8px}`（:26-33）。
	headerPadTop    float32 = 16
	headerPadRight  float32 = 14
	headerPadBottom float32 = 8
	headerPadLeft   float32 = 24
	headerGap       float32 = 8

	// 标题：`.title{min-height:28px;font-size:16px;line-height:24px;font-weight:510}`（:35-44）。
	titleMinHeight float32 = 28
	titleFontSize  float32 = 16
	titleLineH     float32 = 24
	titleWeight    float32 = 510

	// 面包屑条：`.crumbBar{min-height:24px;margin-left:-9px;padding:0 8px;gap:4px}`（:51-61）。
	crumbBarMinHeight float32 = 24
	crumbBarPullLeft  float32 = -9
	crumbBarPadX      float32 = 8
	crumbBarGap       float32 = 4

	// 面包屑：`.crumb{max-width:160px;font-size:13px;line-height:20px;font-weight:500}`（:114-127）。
	crumbMaxWidth float32 = 160
	crumbFontSize float32 = 13
	crumbLineH    float32 = 20
	crumbWeight   float32 = 500

	// 内容区：`.content{padding:16px 16px 16px 24px}`（:205-215）。
	contentPadTop    float32 = 16
	contentPadRight  float32 = 16
	contentPadBottom float32 = 16
	contentPadLeft   float32 = 24

	// Miller 行：`.millerRow{gap:12px}`（:83-94）——两列之间 12px。
	millerGap float32 = 12

	// 列：`.column{flex:1 1 0;min-width:256px;gap:2px;padding-right:8px}`（:221-232）。
	columnMinWidth float32 = 256
	columnGap      float32 = 2
	columnPadRight float32 = 8
	// 中间分隔线：`.divider{width:0.5px}`（:234-238）。
	dividerWidth float32 = 0.5

	// 行：`.row{height:28px;gap:4px;padding:4px}`（:245-258）。
	rowHeight float32 = 28
	rowGap    float32 = 4
	rowPad    float32 = 4
	// 行名：`.rowName{font-size:13px;line-height:20px;font-weight:500}`（:293-303）。
	rowNameFontSize float32 = 13
	rowNameLineH    float32 = 20
	rowNameWeight   float32 = 500
	// 行首 folder 图标 16px、行尾 chevron 12px（`:256` / `:259`）。
	// **不手画 SVG**，只保留槽宽。
	rowIconSize    float32 = 16
	rowChevronSize float32 = 12

	// 状态/错误行：`.status,.error{font-size:12px;line-height:18px;padding:4px}`（:310-327）。
	statusFontSize float32 = 12
	statusLineH    float32 = 18

	// footer：`.footerBar{padding:16px 24px;gap:8px}`（:348-358）。
	footerPadV float32 = 16
	footerPadH float32 = 24
	footerGap  float32 = 8
	// 取消/打开的最小宽：`.footerAction{min-width:72px}`（:398-400）。
	footerActionMinWidth float32 = 72

	// 新建文件夹子对话框：`.createDialog{width:min(380px,100%)}`（:403-407）。
	createDialogMaxWidth float32 = 380
	// `.createBody{padding:22px 24px 20px;gap:12px}`（:409-414）。
	createBodyPadTop    float32 = 22
	createBodyPadX      float32 = 24
	createBodyPadBottom float32 = 20
	createBodyGap       float32 = 12
	// `.createTitle{font-size:16px;line-height:24px;font-weight:510}`（:416-422）。
	createTitleFontSize float32 = 16
	createTitleLineH    float32 = 24
	createTitleWeight   float32 = 510
	// `.createIn{font-size:14px;line-height:22px}`（:424-429）。
	createInFontSize float32 = 14
	createInLineH    float32 = 22
	// `.createInput{height:44px;border:0.5px solid …;border-radius:--dsw-radius-lg}`（:431-443）。
	createInputHeight float32 = 44
	// `.createActions{justify-content:flex-end;gap:8px;margin-top:8px}`（:449-455）。
	createActionsGap       float32 = 8
	createActionsMarginTop float32 = 8
)

// reqTimeout 是一次宿主数据调用的上限。
//
// 为什么要有：本对话框是**模态**的，一次慢盘/网络挂载的扫描如果不设上限，
// 「加载中…」会永远停在那儿，用户除了关掉重开没有别的出路。
const reqTimeout = 15 * time.Second

// DirSource 是壳注入的**数据面**；nil = 没有数据能力，整个面板不画（不许画死按钮）。
//
// 三条方法一一对应 dirpicker.go 的三条通道，参数/返回值形状也照搬：
// `List` 的 path 为空 = 宿主 home；`Create` 的 name 必须是单个路径段；
// `Register` 就是「把这个绝对路径注册成工作区」。
//
// 为什么是一个**本地接口**而不是 `pluginapi` 里的公共契约：公共契约要动
// `pluginapi/plugin.go`，那是另一条线的事；插件侧只需要"壳能塞进来一个实现"。
// 等公共契约落定，这个接口直接删掉、换成 `pluginapi` 的类型即可。
type DirSource interface {
	List(ctx context.Context, path string) (*apiclient.DirectoryListing, error)
	Create(ctx context.Context, parent, name string) (string, error)
	Register(ctx context.Context, path string) error
}

var (
	injectMu sync.RWMutex
	// injected 是壳塞进来的数据源（nil = 没有）。
	injected DirSource
	// picked 是「用户选定了这个目录」的回调（nil = 只关对话框）。
	picked func(path string)
)

// SetSource 由壳注入数据面（**必须在 Load 之后**）。
//
// 传 nil = 收回能力（对话框从此整块不画）——老壳 / 没有 apiclient 时就是这样。
func SetSource(src DirSource) {
	injectMu.Lock()
	injected = src
	injectMu.Unlock()
}

// SetOnPicked 由壳注入「选定了某个目录」的回调。
//
// 只在 `workspace.create` **成功之后**才会被调；失败留在对话框里显示宿主原文。
func SetOnPicked(fn func(path string)) {
	injectMu.Lock()
	picked = fn
	injectMu.Unlock()
}

// currentSource 取当前注入的数据源（每帧都重新问，壳中途注入/收回都立刻生效）。
func currentSource() DirSource {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return injected
}

// currentPicked 取当前注入的选定回调。
func currentPicked() func(path string) {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return picked
}

// Plugin 是目录选择器本体。
type Plugin struct {
	theme *pluginapi.Theme

	// run 执行一次宿主数据调用。
	//
	// 默认 nil = `go fn()`：`List`/`Create`/`Register` 背后是 HTTP，在绘制线程里
	// 同步等一次慢盘扫描会把整个窗口卡住（这正是官方要"慢扫描"指示器的原因）。
	// 测试把它换成同步执行，点击 → 状态落定在同一线程，断言才是确定的
	// （与 `plugins/dssettings` 的 `applyRowWrite` 同一个思路）。
	run func(fn func())

	mu sync.Mutex
	// act 是上一帧的 Actions：异步回调要靠它的 Invalidate 请壳重绘一帧。
	act pluginapi.Actions

	open       bool
	level      *apiclient.DirectoryListing // 左列（当前层）
	selected   *apiclient.DirectoryItem    // 左列里被点中的行（null = 没有选中）
	child      *apiclient.DirectoryListing // 右列（选中行的子层）
	loading    bool
	errText    string
	showHidden bool

	createOpen bool
	creating   bool
	createErr  string

	// gen 是「这一次打开」的世代：关掉/重开之后，在飞的结果一律作废
	// （官方 openGeneration，OFF:DirectoryBrowser.tsx:302-303）。
	gen uint64
	// seq 是「这一次请求」的编号：后来者赢（官方 requestSeq / supersede，
	// OFF:DirectoryBrowser.tsx:323-328、`:428`）。
	seq uint64

	// createName 是「新建文件夹」输入框的编辑器。
	// 必须交给壳登记（`Fields()`），否则收不到键盘。
	createName ui.Field

	// boxes 是最近一帧画出来的几何（无头测试量它；产品路径只写不读）。
	boxes pickerBoxes
}

// pickerBoxes 是最近一帧的关键矩形。
type pickerBoxes struct {
	Dialog       ui.Box
	Header       ui.Box
	Title        ui.Box
	CrumbBar     ui.Box
	Content      ui.Box
	LeftColumn   ui.Box
	RightColumn  ui.Box
	Divider      ui.Box
	Footer       ui.Box
	FooterInner  ui.Box
	CreateDialog ui.Box
	CreateInput  ui.Box
	Rows         []ui.Box
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-dirpicker" }

// Provides 实现 pluginapi.Plugin：目录选择器不对外提供服务键。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：**只要令牌**。
//
// 为什么不像其它部件那样连 `KeyPrimitives` 一起要：这里一条基础图元都没用——
// 行/分隔线/状态行全部按官方几何直接摆（0.5px 的 `border-l3` 不是 `Divider` 的
// 1px `border-l1`，套图元反而画错）。少要一个依赖，装配顺序就少一处无关约束
// （dsgoal / dsopen 同样只要令牌）。
func (p *Plugin) Requires() []string { return []string{pluginapi.KeyTheme} }

// Register 实现 pluginapi.Plugin：只做注入，不画东西。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	theme, _ := ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.theme = theme
	return nil
}

// Fields 把「新建文件夹」的输入框交给壳登记（不登记就收不到键盘）。
func (p *Plugin) Fields() []*ui.Field { return []*ui.Field{&p.createName} }

// Panels 实现 pluginapi.Panels：模态卡片挂 `shell.overlay`（官方
// `dsh-client-ui-layout` 的 `shell.overlay`，正是 createPortal 到 body 的那一层）。
//
// Order 102：模态层最高一档。现有 overlay 档位：97 文件预览 / 98 终端 /
// 99 命令面板 / 100·101 设置浮层 / 96 交付物预览 —— 目录浏览器是一个
// 阻塞式模态（打开时必须压住上面全部），所以排在 102。
// `cmd/app/panels_order_test.go:84` 那条守卫要求同槽 Order 互不相同，这里避开整条既有档位带。
func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{{
		ID:    "dirpicker-dialog",
		Slot:  pluginapi.SlotShellOverlay,
		Order: 102,
		Draw:  p.draw,
	}}
}

// Open 打开对话框：清空现场，从宿主 home 开始列（官方 OFF:DirectoryBrowser.tsx:575-584）。
//
// 返回 false = **没有 source，什么都没发生**。壳据此判断要不要画入口。
func (p *Plugin) Open() bool {
	if currentSource() == nil {
		return false
	}
	p.mu.Lock()
	p.open = true
	p.gen++ // 重开 = 新世代：上一次打开遗留的结算一律作废
	p.level, p.selected, p.child = nil, nil, nil
	p.showHidden = false
	p.createOpen, p.createErr, p.creating = false, "", false
	p.loading, p.errText = false, ""
	p.mu.Unlock()
	p.createName.SetText("")
	p.navigate("")
	return true
}

// Close 关掉对话框（取消 / 选定成功之后都由它收尾）。
func (p *Plugin) Close() {
	p.mu.Lock()
	p.open = false
	p.gen++ // 关掉之后在飞的请求不许再改这个对话框
	p.seq++
	p.loading, p.errText = false, ""
	p.createOpen, p.createErr, p.creating = false, "", false
	p.level, p.selected, p.child = nil, nil, nil
	p.mu.Unlock()
	p.createName.SetText("")
}

// ── 数据调用：全部异步，结果带世代/请求号 fencing ─────────────────────────

// begin 开一次请求：清掉旧错误、置「加载中」，返回 (世代, 请求号)。
func (p *Plugin) begin() (gen, seq uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	p.loading = true
	p.errText = ""
	return p.gen, p.seq
}

// dispatch 执行一次宿主调用（默认丢给 goroutine，见 Plugin.run 的注释）。
func (p *Plugin) dispatch(fn func()) {
	if p.run != nil {
		p.run(fn)
		return
	}
	go fn()
}

// invalidate 请壳重绘一帧（异步结果落地之后必须调，否则画面停在旧状态）。
func (p *Plugin) invalidate() {
	p.mu.Lock()
	fn := p.act.Invalidate
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// staleLocked 报告 (gen, seq) 这一份结算还是不是当前的（在锁内调用）。
func (p *Plugin) staleLocked(gen, seq uint64) bool { return gen != p.gen || seq != p.seq }

// navigate 列一层目录并**整块**换掉视图（path 为空 = 宿主 home）。
func (p *Plugin) navigate(path string) {
	src := currentSource()
	if src == nil {
		return
	}
	gen, seq := p.begin()
	p.invalidate() // 先把「加载中…」那一帧放出来
	p.dispatch(func() {
		listing, err := listWithTimeout(src, path)
		p.mu.Lock()
		if !p.staleLocked(gen, seq) {
			p.loading = false
			if err != nil {
				// 导航失败：**不清空已有列**（官方 "the last readable panes stand"），
				// 只把宿主原文挂到 .error 那一行。
				p.errText = failureText(err)
			} else {
				p.level, p.selected, p.child = listing, nil, nil
				p.errText = ""
			}
		}
		p.mu.Unlock()
		p.invalidate()
	})
}

// selectRow 点左列某一行：先落选中态，再拉它的子层给右列（官方 select，:506-533）。
//
// 为什么选中态先落：点击的第一职责是"那行立刻有反馈"，而列拆分本身就是反馈；
// 等子层拉回来再亮，点击会像丢了一样（官方注释 :497-505 的原话）。
func (p *Plugin) selectRow(entry apiclient.DirectoryItem) {
	src := currentSource()
	if src == nil {
		return
	}
	gen, seq := p.begin()
	p.mu.Lock()
	selected := entry
	p.selected, p.child = &selected, nil
	p.mu.Unlock()
	p.invalidate()
	p.dispatch(func() {
		listing, err := listWithTimeout(src, entry.Path)
		p.mu.Lock()
		if !p.staleLocked(gen, seq) {
			p.loading = false
			if err != nil {
				// 读不出来的行不能当提交目标（官方失败分支 :521-532）：
				// 退回单列，并把原文挂出来。
				p.errText = failureText(err)
				p.selected, p.child = nil, nil
			} else {
				p.child = listing
				p.errText = ""
			}
		}
		p.mu.Unlock()
		p.invalidate()
	})
}

// advanceRow 点右列某一行：那一层成为左列，再往下一层（官方 advance，:566-571）。
func (p *Plugin) advanceRow(entry apiclient.DirectoryItem) {
	p.mu.Lock()
	if p.child != nil {
		p.level = p.child // 右列就是这一行的父层
	}
	p.mu.Unlock()
	p.selectRow(entry)
}

// confirmOpen 点「打开」：把目标目录注册成工作区，成功后关闭并回调壳。
//
// 官方把"打开"的语义交给 flow owner（`OFF:flow.ts:34` 的 onPicked）；
// 本仓的对等物是 `workspace.create` + `SetOnPicked`。
func (p *Plugin) confirmOpen() {
	src := currentSource()
	if src == nil {
		return
	}
	p.mu.Lock()
	target := p.targetLocked()
	gen := p.gen
	p.mu.Unlock()
	if target == "" {
		return
	}
	p.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
		defer cancel()
		err := src.Register(ctx, target)
		p.mu.Lock()
		stale := gen != p.gen
		if !stale && err != nil {
			p.errText = failureText(err) // 失败：卡片留着，宿主原文挂在 .error
		}
		p.mu.Unlock()
		if stale {
			return
		}
		if err != nil {
			p.invalidate()
			return
		}
		p.Close()
		if fn := currentPicked(); fn != nil {
			fn(target)
		}
		p.invalidate()
	})
	p.invalidate()
}

// openCreate 弹「新建文件夹」子对话框。
func (p *Plugin) openCreate() {
	p.mu.Lock()
	p.createOpen = true
	p.createErr = ""
	p.mu.Unlock()
	p.createName.SetText("")
	p.invalidate()
}

// closeCreate 收起子对话框（创建进行中不许收起，免得让人以为已经取消了）。
func (p *Plugin) closeCreate() {
	p.mu.Lock()
	if p.creating {
		p.mu.Unlock()
		return
	}
	p.createOpen = false
	p.createErr = ""
	p.mu.Unlock()
	p.invalidate()
}

// confirmCreate 建目录；成功后**进入新目录**（任务口径）。
func (p *Plugin) confirmCreate() {
	src := currentSource()
	if src == nil {
		return
	}
	// 只 trim 判空，**不 trim 后发**：真目录名可以以空格结尾，trim 会建出另一个
	// 兄弟目录（官方注释 :610-613 的原话）。
	name := p.createName.Text()
	if strings.TrimSpace(name) == "" {
		return
	}
	p.mu.Lock()
	if p.creating {
		p.mu.Unlock()
		return
	}
	parent := p.targetLocked()
	gen := p.gen
	p.creating, p.createErr = true, ""
	p.mu.Unlock()
	if parent == "" {
		p.mu.Lock()
		p.creating = false
		p.mu.Unlock()
		return
	}
	p.invalidate()
	p.dispatch(func() {
		created, err := createWithTimeout(src, parent, name)
		p.mu.Lock()
		if gen != p.gen {
			p.mu.Unlock()
			return
		}
		p.creating = false
		if err != nil {
			p.createErr = failureText(err) // 失败留在子对话框里，卡片不关
			p.mu.Unlock()
			p.invalidate()
			return
		}
		p.createOpen = false
		p.mu.Unlock()
		if created == "" {
			// 宿主没给新路径：不猜，也不进入一个不存在的地方。
			p.mu.Lock()
			p.createErr = failureText(errNoCreatedPath)
			p.mu.Unlock()
			p.invalidate()
			return
		}
		p.navigate(created)
	})
}

// errNoCreatedPath 是「宿主回了空路径」这一种失败（不是宿主错误原文，
// 而是我们自己发现的对端不合约；文案照实写，不修饰）。
var errNoCreatedPath = errors.New("宿主没有返回新目录的路径")

// targetLocked 是「创建 / 打开」当前作用在哪个目录（官方 targetPath，:602-605）：
// 有选中行就是它，否则是当前列出的那一层。返回空串 = 还没有目标。
func (p *Plugin) targetLocked() string {
	if p.selected != nil && p.selected.Path != "" {
		return p.selected.Path
	}
	if p.level != nil {
		return p.level.Path
	}
	return ""
}

// targetNameLocked 是那个目录的显示名（子对话框的「在"{name}"中新建文件夹」用）。
func (p *Plugin) targetNameLocked() string {
	if p.selected != nil && p.selected.Name != "" {
		return p.selected.Name
	}
	if p.level == nil {
		return ""
	}
	crumbs := displayCrumbs(p.level, "")
	if n := len(crumbs); n > 0 && crumbs[n-1].Name != "" {
		return crumbs[n-1].Name
	}
	return p.level.Path
}

// ── 宿主调用的薄封装：统一超时 ────────────────────────────────────────────

func listWithTimeout(src DirSource, path string) (*apiclient.DirectoryListing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	return src.List(ctx, path)
}

func createWithTimeout(src DirSource, parent, name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()
	return src.Create(ctx, parent, name)
}

// failureText 把一次失败变成界面上那一行**原文**。
//
// 官方 failureText（OFF:DirectoryBrowser.tsx:75-82）优先读 `rpcError.message`，
// 也就是宿主那句业务错误；本仓的对等物是 `*apiclient.APIError.Message`。
// 其它错误原样 Error()——**不新写中文**，也不把宿主的话改写成人话。
func failureText(err error) string {
	if err == nil {
		return ""
	}
	var api *apiclient.APIError
	if errors.As(err, &api) && api.Message != "" {
		return api.Message
	}
	return err.Error()
}

// displayCrumbs 是面包屑要显示的那一串（官方 displayCrumbs，:116-121）。
//
// 在 home 子树里：从「主目录」那一段开始（宿主的 home 路径本身不显示成路径）；
// 不在 home 子树里：整条祖先链照原样显示，根用它的路径当地名。
func displayCrumbs(listing *apiclient.DirectoryListing, homeLabel string) []apiclient.DirectoryItem {
	if listing == nil {
		return nil
	}
	if listing.Home == "" {
		return listing.Crumbs
	}
	for i, crumb := range listing.Crumbs {
		if crumb.Path != listing.Home {
			continue
		}
		if homeLabel == "" {
			return listing.Crumbs[i:]
		}
		out := make([]apiclient.DirectoryItem, 0, len(listing.Crumbs)-i)
		out = append(out, apiclient.DirectoryItem{Name: homeLabel, Path: listing.Home})
		return append(out, listing.Crumbs[i+1:]...)
	}
	return listing.Crumbs
}

// visibleEntries 是一列真正要画的行：默认滤掉宿目标了 `Hidden` 的条目
// （官方 `visibleEntries` 的默认分支，:201-218；本版没有前缀过滤那一支）。
func visibleEntries(entries []apiclient.DirectoryItem, showHidden bool) []apiclient.DirectoryItem {
	if showHidden {
		return entries
	}
	out := make([]apiclient.DirectoryItem, 0, len(entries))
	for _, entry := range entries {
		if entry.Hidden {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// createInText 套官方模板（`browser.createIn` = 在"{name}"中新建文件夹）。
func createInText(name string) string {
	return strings.ReplaceAll(zhCreateIn, "{name}", name)
}

// ── 绘制 ──────────────────────────────────────────────────────────────────

// draw 画整个对话框。**没有 source 或没打开就一个节点都不画**。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	t := p.theme
	if t == nil {
		return
	}
	p.mu.Lock()
	p.act = env.Act
	open := p.open
	level, selected, child := p.level, p.selected, p.child
	loading, errText, showHidden := p.loading, p.errText, p.showHidden
	createOpen, creating, createErr := p.createOpen, p.creating, p.createErr
	createName := p.targetNameLocked()
	p.mu.Unlock()

	// 上一帧的几何作废：这一帧一个节点都不画时，boxes 必须是空的
	// （测试据此断言"没有 source 就什么都没画"）。
	p.boxes = pickerBoxes{}
	if !open || currentSource() == nil {
		return
	}

	vp := f.Bounds()
	cardWidth := min(dialogMaxWidth, vp.Width)
	cardHeight := min(dialogMaxHeight, vp.Height-dialogViewportInset)
	if cardHeight < 0 {
		cardHeight = 0
	}

	// 模态卡片居中：官方由 Modal 的遮罩层居中（`.dialog` 自己没有定位）。
	// 遮罩的底色官方在 primitives 的 Modal 里，本仓 dump 没有那份 CSS，
	// 所以**不画遮罩**（不自造一个半透明压色），只把卡片摆中间。
	ui.Column(f).FillWidth().FillHeight().Center().Children(func() {
		card := ui.Column(f).Width(cardWidth).Height(cardHeight).
			Background(t.BgLayer2).Radius(t.Radii.XL).Clip()
		p.boxes.Dialog = card
		card.Children(func() {
			crumbSource := level
			if child != nil {
				crumbSource = child
			}
			p.drawHeader(f, env, t, crumbSource)
			ui.Row(f).FillWidth().Height(hairlineWidth).Background(t.BorderL3)
			p.drawContent(f, env, t, level, selected, child, loading, errText, showHidden)
			ui.Row(f).FillWidth().Height(hairlineWidth).Background(t.BorderL3)
			p.drawFooter(f, env, t, level, loading, creating, showHidden)
			if createOpen {
				p.drawCreateDialog(f, env, t, cardWidth, createName, creating, createErr)
			}
		})
	})
}

// drawHeader 画 header：标题 + 面包屑条（官方 .header / .title / .crumbBar）。
func (p *Plugin) drawHeader(f ui.C, env *pluginapi.Env, t *pluginapi.Theme, crumbSource *apiclient.DirectoryListing) {
	header := ui.Column(f).FillWidth().
		PadSides(headerPadTop, headerPadRight, headerPadBottom, headerPadLeft).Gap(headerGap)
	p.boxes.Header = header
	header.Children(func() {
		p.boxes.Title = ui.Text(f, zhTitle).MinHeight(titleMinHeight).
			FontSize(titleFontSize).LineHeight(titleLineH).Weight(titleWeight).Color(t.LabelPrimary)
		bar := ui.Row(f).FillWidth().MinHeight(crumbBarMinHeight).Gap(crumbBarGap).
			MarginLeft(crumbBarPullLeft).PadX(crumbBarPadX).Radius(t.Radii.SM)
		p.boxes.CrumbBar = bar
		bar.Children(func() {
			for i, crumb := range displayCrumbs(crumbSource, zhHome) {
				if i > 0 {
					// 官方每个 crumbSeat 前有 12px chevron 图标；不手画 SVG，
					// 用同一个字号的 › 占位（几何 12px 那档、颜色 tertiary）。
					ui.Text(f, "›").FontSize(rowChevronSize).LineHeight(crumbLineH).
						Color(t.LabelTertiary)
				}
				label := crumb.Name
				if label == "" {
					label = crumb.Path
				}
				crumbBox := ui.Text(f, label).MaxWidth(crumbMaxWidth).Ellipsis().
					FontSize(crumbFontSize).LineHeight(crumbLineH).Weight(crumbWeight).
					Color(t.LabelTertiary).Hit(p.hitID(env, "crumb-"+crumb.Path), label)
				if crumbBox.Clicked() && crumb.Path != "" {
					p.navigate(crumb.Path)
				}
			}
		})
	})
}

// drawContent 画 Miller 双列 + 状态行（官方 .content / .millerRow / .column / .divider）。
func (p *Plugin) drawContent(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	level *apiclient.DirectoryListing, selected *apiclient.DirectoryItem, child *apiclient.DirectoryListing,
	loading bool, errText string, showHidden bool) {
	content := ui.Column(f).FillWidth().Grow(1).
		PadSides(contentPadTop, contentPadRight, contentPadBottom, contentPadLeft)
	p.boxes.Content = content
	content.Children(func() {
		miller := ui.Row(f).FillWidth().Grow(1).Gap(millerGap)
		miller.Children(func() {
			p.boxes.LeftColumn = p.drawColumn(f, env, t, level, selected, showHidden, false)
			// 官方：分隔线与右列都要求 `twoPane`，但右列还要求 `child !== null`
			// （`:939` `{twoPane && <span className={css.divider} />}`、`:940`
			// `{twoPane && child !== null && <LevelColumn …/>}`）——子层还没落地时
			// 只把分隔线画出来，右列等它到。
			if selected != nil {
				p.boxes.Divider = ui.Row(f).Width(dividerWidth).FillHeight().Background(t.BorderL3)
				if child != nil {
					p.boxes.RightColumn = p.drawColumn(f, env, t, child, nil, showHidden, true)
				}
			}
		})
		// 「加载中…」：官方是浮动药丸且只在慢扫描(300ms)后出现（OFF:DirectoryBrowser.module.css:329-344，
		// 本仓没有那条定时器链路，见包注释「没搬的细节」），这里直接占一行。
		if loading {
			ui.Text(f, zhLoading).Pad(rowPad).
				FontSize(statusFontSize).LineHeight(statusLineH).Color(t.LabelSecondary)
		}
		if listingTruncated(level) || listingTruncated(child) {
			ui.Text(f, zhTruncated).Pad(rowPad).
				FontSize(statusFontSize).LineHeight(statusLineH).Color(t.LabelSecondary)
		}
		if errText != "" {
			// **宿主原文**，不加工（官方 .error 的 role="alert"）。
			ui.Text(f, errText).Pad(rowPad).
				FontSize(statusFontSize).LineHeight(statusLineH).Color(t.Error)
		}
	})
}

// drawColumn 画一列（`.column`：flex:1、min-width 256、gap 2、右 padding 8）。
func (p *Plugin) drawColumn(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	listing *apiclient.DirectoryListing, selected *apiclient.DirectoryItem, showHidden, rightPane bool) ui.Box {
	column := ui.Column(f).Grow(1).MinWidth(columnMinWidth).Gap(columnGap).PadRight(columnPadRight)
	column.Children(func() {
		if listing == nil {
			return
		}
		scroll := ui.Scroll(f).FillWidth().Grow(1)
		scroll.Children(func() {
			for _, entry := range visibleEntries(listing.Entries, showHidden) {
				p.drawRow(f, env, t, entry, selected, rightPane)
			}
		})
	})
	return column
}

// drawRow 画一行（`.row`：height 28、gap 4、padding 4、圆角 sm；选中用 active 底）。
func (p *Plugin) drawRow(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	entry apiclient.DirectoryItem, selected *apiclient.DirectoryItem, rightPane bool) {
	isSelected := selected != nil && selected.Path == entry.Path
	row := ui.Row(f).FillWidth().Height(rowHeight).Gap(rowGap).Pad(rowPad).
		Items(ui.Center).Radius(t.Radii.SM)
	if isSelected {
		row = row.Background(t.NavActive) // 官方 .rowSelected 用 --dsw-alias-interactive-bg-active
	}
	row = row.Hit(p.hitID(env, "row-"+entry.Path), entry.Name)
	p.boxes.Rows = append(p.boxes.Rows, row)
	row.Children(func() {
		// 行首 folder 图标 16px（选中时是 open 变体）与行尾 chevron 12px 都是
		// sprite SVG：按仓规不手画，只保留槽宽，别让行名贴到边。
		ui.Row(f).Size(rowIconSize, rowIconSize)
		ui.Text(f, entry.Name).Grow(1).Ellipsis().
			FontSize(rowNameFontSize).LineHeight(rowNameLineH).Weight(rowNameWeight).
			Color(t.LabelPrimary)
		ui.Row(f).Size(rowChevronSize, rowChevronSize)
	})
	if !row.Clicked() {
		return
	}
	if rightPane {
		p.advanceRow(entry)
		return
	}
	p.selectRow(entry)
}

// drawFooter 画 footer（`.footerBar`：padding 16px 24px、gap 8、上边框 0.5px）。
func (p *Plugin) drawFooter(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	level *apiclient.DirectoryListing, loading, creating, showHidden bool) {
	bar := ui.Row(f).FillWidth().Items(ui.Center).
		PadSides(footerPadV, footerPadH, footerPadV, footerPadH).Gap(footerGap)
	p.boxes.Footer = bar
	bar.Children(func() {
		inner := ui.Row(f).FillWidth().Items(ui.Center).Gap(footerGap)
		p.boxes.FooterInner = inner
		inner.Children(func() {
			// 左：新建文件夹（outline）+ 显示隐藏文件（文字开关）。
			//
			// 每个按钮都再 `.Hit(…)` 一次：`ui.Button` 拿的是 page 自增的控件名，
			// 那种名字**无头测试点不到**；用 `Hit` 把它换成 Tree 上的稳定 id 之后，
			// 点击与断言走的是同一条真实路径（不是绕过按钮直接调方法）。
			if ui.Button(f, zhNewFolder).Outline().Small().
				Hit(p.hitID(env, "new-folder"), zhNewFolder).
				Disabled(level == nil || loading || creating).Clicked() {
				p.openCreate()
			}
			toggle := ui.Text(f, zhShowHidden).FontSize(rowNameFontSize).LineHeight(crumbLineH).
				Weight(crumbWeight).Color(t.LabelSecondary).
				Hit(p.hitID(env, "toggle-hidden"), zhShowHidden)
			if showHidden {
				// 官方在标签**后面**放一枚 check（Menu 的 selected 语汇），标签本身不换字。
				ui.Text(f, "✓").FontSize(rowNameFontSize).LineHeight(crumbLineH).
					Color(t.LabelPrimary)
			}
			if toggle.Clicked() {
				p.mu.Lock()
				p.showHidden = !p.showHidden
				p.mu.Unlock()
			}
			ui.Spacer(f)
			// 右：取消 + 打开。官方两个都是 Button（outline / primary），min-width 72。
			if ui.Button(f, zhCancel).Outline().Small().
				Hit(p.hitID(env, "cancel"), zhCancel).
				MinWidth(footerActionMinWidth).Disabled(creating).Clicked() {
				p.Close()
			}
			p.mu.Lock()
			target := p.targetLocked()
			p.mu.Unlock()
			if ui.Button(f, zhOpen).Small().MinWidth(footerActionMinWidth).
				Hit(p.hitID(env, "open"), zhOpen).
				Disabled(target == "" || loading || creating).Clicked() {
				p.confirmOpen()
			}
		})
	})
}

// drawCreateDialog 画「新建文件夹」子对话框（`.createDialog` / `.createBody` / `.createInput`）。
//
// 它是父卡片之内的 **absolute 子层**（`Inset(0)` + 居中）：官方是再套一个 Modal
// （portal 到 body），本仓没有 portal，等价做法是盖在父卡片上并吃掉点击。
func (p *Plugin) drawCreateDialog(f ui.C, env *pluginapi.Env, t *pluginapi.Theme,
	parentWidth float32, targetName string, creating bool, createErr string) {
	ui.Column(f).Absolute().Inset(0).Center().Children(func() {
		// `.createDialog.createDialog{width:min(380px,100%)}`（:403-407）：
		// 100% 是**父卡片**的宽，不是视口宽。
		width := min(createDialogMaxWidth, parentWidth)
		if width <= 0 {
			width = createDialogMaxWidth
		}
		card := ui.Column(f).Width(width).Background(t.BgLayer2).
			Radius(t.Radii.XL).Border(hairlineWidth, t.BorderL4).Clip()
		p.boxes.CreateDialog = card
		card.Children(func() {
			body := ui.Column(f).FillWidth().Gap(createBodyGap).
				PadSides(createBodyPadTop, createBodyPadX, createBodyPadBottom, createBodyPadX)
			body.Children(func() {
				ui.Text(f, zhNewFolder).FontSize(createTitleFontSize).
					LineHeight(createTitleLineH).Weight(createTitleWeight).Color(t.LabelPrimary)
				ui.Text(f, createInText(targetName)).FontSize(createInFontSize).
					LineHeight(createInLineH).Color(t.LabelPrimary)
				input := ui.TextField(f, &p.createName, zhFolderName, zhUntitledFolder).
					Height(createInputHeight).Radius(t.Radii.LG).Border(hairlineWidth, t.BorderL4)
				p.boxes.CreateInput = input
				if createErr != "" {
					ui.Text(f, createErr).Pad(rowPad).
						FontSize(statusFontSize).LineHeight(statusLineH).Color(t.Error)
				}
				actions := ui.Row(f).FillWidth().Justify(ui.End).Items(ui.Center).
					Gap(createActionsGap).MarginTop(createActionsMarginTop)
				actions.Children(func() {
					if ui.Button(f, zhCancel).Outline().Small().
						Hit(p.hitID(env, "create-cancel"), zhCancel).
						Disabled(creating).Clicked() {
						p.closeCreate()
					}
					draft := p.createName.Text()
					if ui.Button(f, zhCreate).Small().
						Hit(p.hitID(env, "create-confirm"), zhCreate).
						Disabled(creating || strings.TrimSpace(draft) == "").Clicked() {
						p.confirmCreate()
					}
				})
			})
		})
	})
}

// listingTruncated 报告某一列是不是被宿主截断了（官方 `truncated`，:960-961）。
func listingTruncated(listing *apiclient.DirectoryListing) bool {
	return listing != nil && listing.Truncated
}

// hitID 造交互 id（不许写死数字：环境里有 Tree 时用 Tree.ID）。
//
// 没有 Tree（单测直接画一眼）时返回零值 ID：零值 = "没有目标"，
// `Clicked()` 恒 false —— 那是**正确的降级**，不是伪造一个会响的 id。
func (p *Plugin) hitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}
