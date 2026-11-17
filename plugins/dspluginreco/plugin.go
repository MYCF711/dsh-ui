// Package dspluginreco 是官方 `dsh-client-ui-plugin-recommendations` 的 Go 版：
// 「官方插件推荐」这张动作卡（action = `enable_official_plugin`）的正文。
//
// # 结构证据（唯一权威，几何/文案/状态机全部照它）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-plugin-recommendations
//	OFF:src/client/Card.tsx        159 行：正文（header / 标题+描述 / 状态 / footer 两个按钮）
//	OFF:src/client/Card.module.css  13 行：全部几何值
//	                                （1 header / 2 badge / 3 body / 4 details / 5-6 details 的
//	                                 hover 与 focus-visible / 7 title / 8 description /
//	                                 9 placeholder / 10 footer / 11 scope / 12 actions / 13 status）
//	OFF:src/client/locales.ts:2-28  zh 字典 25 条（25 个键在 :3-27；本包**只许**用这 25 条原文）
//	OFF:src/client/mount.ts:37-40   describe = `remote.pluginRecommendations.describe`
//	OFF:src/client/mount.ts:41      openDetails = `pluginNavigation.openBundle(packageName)`
//	OFF:src/client/mount.ts:19      inject: ['locale', 'remote', 'actionCards', 'pluginNavigation']
//
// 座位由**动作卡框架**提供（`dsh-client-ui-action-cards` 的 turnTail 槽）；本包不占槽，
// 只通过 `pluginapi.KeyActionCardBodies` 把正文登记进框架的注册表
// （官方 `ctx.actionCards.register({action:'enable_official_plugin', …})`，
// OFF:src/client/mount.ts:45-48）。
//
// # 数据面证据（宿主是真的，不是注释）
//
//	HOST:internal/recommendations/service.go:120-181   describe 的真实现
//	HOST:internal/recommendations/service.go:183-185   refused 的五个 reason
//	HOST:internal/actioncards/service.go:45            VerbDismiss
//	HOST:cmd/dshd/serve_plugin_recommendations.go:20  真实的宿主接线（pluginMarket + pluginManager）
//	本仓客户端：internal/apiclient/recommendations.go（同目录证据行号）
//
// # 它要壳注入什么
//
//	SetSource(RecoSource)        —— describe（`*apiclient.Client` 的薄包装）
//	SetOnOpenDetails(func(name)) —— 「查看详情」跳到插件页的那个 bundle
//
// 两个都必须由壳转一手：插件拿不到 `*apiclient.Client`（R2/R7），也拿不到侧栏页
// （插件之间不 direct 引用）。**source 为 nil 时这张卡仍画标题，但没有目录文案、
// 也不画「启用」**——官方在没有 describe 的部署里就是这样（`inject: ['remote']`
// 缺失即整卡退化为只读）。
//
// # 没搬 / 对不上的地方（照实登记，不假装搬了）
//
//  1. **focus-visible 的 2px 内描边不画**：官方 `.details:focus-visible` 是
//     `box-shadow: inset 0 0 0 2px var(--dsw-focus-ring-color, var(--dsw-alias-state-business-primary))`
//     （Card.module.css:6，宽度出处 focus.css:16 的 `--dsw-focus-ring-width:2px`）。
//     viva 的 `ui.Look.Ring` 画在元素**外侧**（ui/style.go:92-96「around the outside」），
//     画出来会溢出 `.details` 的 -8px 外扩，形状和官方的 inset 描边不是一回事，所以不画；
//     focus-visible 的**底色**（与 hover 同一条 `--dsw-alias-interactive-bg-hover`，:5-6）
//     已经照官方画了，见 `Focus(...)`。
//  2. **`.header` / `.footer` 的 `flex-wrap: wrap` 没有对等物**（Card.module.css:1/10）：
//     viva 的 `ui.Row` 不换行。两侧文案与按钮按官方的**同一行**摆；窗口极窄时官方会折行，
//     本仓不会（与 dscreationreco 登记的同一条限制，见 plugins/dscreationreco/plugin.go）。
//  3. **`useId` 的 aria-describedby 关联**：viva 没有 ARIA 关系表，标题上的
//     `aria-label` 语义由 Hit 的 label 承担（读屏念出「查看详情：<标题>」）。
//  4. **`transition: background-color 120ms ease`**（:4）不画：viva 的
//     `Hover`/`Focus` 状态切换本身有一帧插值，官方写的时长没有对等开关，不另编。

package dspluginreco

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Action 是官方推荐卡的 kind 名（OFF:src/client/mount.ts:46 的 `action:`；
// 宿主侧同一常量 HOST:internal/recommendations/service.go:34）。
const Action = "enable_official_plugin"

// ServiceID 是本插件的装配 id（壳与测试按它查表）。
const ServiceID = "ds-pluginreco"

// ── 文案：只有这 25 条，逐字来自 OFF:src/client/locales.ts:2-28 的 zh 字典 ─────
//
// **不许新写中文**：界面上一旦出现这 25 条以外的字样，就不是官方那张卡了。
const (
	zhHeading       = "推荐插件"                         // heading
	zhOfficialBadge = "官方"                           // official
	zhDetailsFor    = "查看详情：%s"                      // detailsFor（`查看详情：{title}`）
	zhVersionError  = "插件与当前 DSH 版本不兼容，请更新 DSH 后重试。" // version-error
	zhManageError   = "无法启用插件，请打开详情检查错误后重试。"         // management-error
	zhEnable        = "启用"                           // enable
	zhDismiss       = "暂不需要"                         // dismiss
	zhWorking       = "正在处理…"                        // working
	zhScope         = "启用后可在所有会话中使用"                 // scope
	zhEnabled       = "已启用"                          // enabled
	zhAlreadyOn     = "已在插件页开启，可查看详情了解运行状态"          // already-enabled
	zhDismissed     = "已忽略"                          // dismissed
	zhReplaced      = "已替换"                          // replaced
	zhRestartNeeded = "已保存，重启 DSH 后生效"               // restart-required
	zhFailed        = "启用失败。可重试或在插件页检查。"             // failed
	zhRetryEnable   = "重试启用"                         // retry
	zhUnconfirmed   = "无法确认插件是否已启用，可重试确认。"           // unconfirmed
	zhLookupFailed  = "无法读取插件信息，请重试。"                // lookup-failed
	zhRetryLookup   = "重试"                           // retryLookup
	zhUnavailable   = "这个插件当前无法启用，请在插件页检查。"          // unavailable
	zhUnknown       = "找不到这条推荐，请重新打开会话。"             // unknown
	zhAccessDenied  = "没有修改此配置的权限。"                  // access-denied
	zhNotLiveRoot   = "请在原会话中操作这条推荐。"                // not-live-root
	zhClosing       = "服务正在重载，请稍后重试。"                // closing
	zhTransport     = "无法确认操作结果，请刷新后重试。"             // transport
)

// ── 几何：全部来自 OFF:Card.module.css（13 行，逐行对着写）─────────────────
const (
	// `.header{ display:flex; align-items:center; justify-content:space-between; gap:12px;
	// padding:10px 20px; background:var(--dsw-alias-bg-module-platform); font-size:12px;
	// line-height:1.5; color:var(--dsw-alias-label-secondary) }`（Card.module.css:1）。
	recoHeaderPadY  float32 = 10
	recoHeaderPadX  float32 = 20
	recoHeaderGap   float32 = 12
	recoHeaderFont  float32 = 12
	recoHeaderLineH float32 = 18 // 12 × 1.5
	// `.badge{ font-size:12px }`（:2）——与 header 同字号，无底色、无边框，只是同行右侧一格字。
	// `.body{ padding:20px }`（:3）。
	recoBodyPad float32 = 20
	// `.details{ display:block; box-sizing:border-box; width:calc(100% + 16px); margin:-8px;
	// padding:8px; border:0; border-radius:var(--dsw-radius-sm); background:transparent; … }`（:4）。
	//
	// 三个值怎么落到 viva 上：`FillWidth()` 在交叉轴上取的是"父内容宽 − 左右外边距"，
	// 因此 `FillWidth + MarginSides(-8,…,-8)` 得到的正是 `calc(100% + 16px)`，
	// 左边距 -8 让它左缘落在 body 内容框外 8px（同 dsagentteam 的 `.taskRow` 先例，
	// plugins/dsagentteam/board.go:44/143）。圆角 `--dsw-radius-sm` = 8px（base.css:19）。
	recoDetailsBleed float32 = 8
	recoDetailsPad   float32 = 8
	// `.title{ display:block; font-size:16px; font-weight:500; line-height:1.5 }`（:7）。
	recoTitleFont   float32 = 16
	recoTitleLineH  float32 = 24
	recoTitleWeight float32 = 500
	// `.description{ display:block; margin:8px 0 0; font-size:13px; line-height:1.7 }`（:8）。
	recoDescMarginTop float32 = 8
	recoDescFont      float32 = 13
	recoDescLineH     float32 = 22.1 // 13 × 1.7
	// `.placeholder{ display:inline-block; width:50%; height:0.8em; vertical-align:middle;
	// border-radius:var(--dsw-radius-xs); background:var(--dsw-alias-bg-skeleton) }`（:9）。
	//
	// `height:0.8em` 的 em 取 `.description` 的字号 13px → 10.4px；
	// `--dsw-radius-xs` = 4px（base.css:18）；`--dsw-alias-bg-skeleton` 在
	// design-platform.css:192（浅 rgba(0,0,0,.04)）/ :344（深 rgba(255,255,255,.08)），
	// 本仓对等令牌是 `Theme.BgSkeleton`（plugins/dscore/dscore.go:153/243）。
	// `vertical-align:middle` 落成"在 22.1px 的行盒里居中"。
	recoPlaceholderPercent float32 = 50
	recoPlaceholderHeight  float32 = 10.4
	// `.status{ margin:16px 0 0; padding-top:12px; border-top:0.5px solid var(--dsw-alias-border-l1);
	// font-size:12px; line-height:1.6 }`（:13）。
	recoStatusMarginTop float32 = 16
	recoStatusPadTop    float32 = 12
	recoStatusBorder    float32 = 0.5
	recoStatusFont      float32 = 12
	recoStatusLineH     float32 = 19.2 // 12 × 1.6
	// `.footer{ display:flex; align-items:center; justify-content:space-between; gap:12px;
	// margin-top:16px; padding-top:16px; border-top:0.5px solid var(--dsw-alias-border-l1);
	// flex-wrap:wrap }`（:10）。
	recoFooterMarginTop float32 = 16
	recoFooterPadTop    float32 = 16
	recoFooterBorder    float32 = 0.5
	recoFooterGap       float32 = 12
	// `.scope{ font-size:12px; line-height:1.5 }`（:11）。
	recoScopeFont  float32 = 12
	recoScopeLineH float32 = 18 // 12 × 1.5
	// `.actions{ display:flex; align-items:center; gap:8px; margin-left:auto }`（:12）。
	// `margin-left:auto` 就是 footer 的 `justify-content:space-between` 把 actions 推到右端。
	recoActionGap float32 = 8
)

// recoLookupTimeout 是一次 describe 的上限。
//
// 为什么要有：查询挂在绘制帧触发的 goroutine 上，一次挂住的请求会让这张卡永远停在
// "加载中"（按钮全 disabled）——那比显示"无法读取插件信息"更糟。
const recoLookupTimeout = 15 * time.Second

// 官方 OPEN_PHASES（OFF:src/client/Card.tsx:45）：这两个 phase 仍然给「启用 / 暂不需要」。
var openPhases = map[string]bool{"failed": true, "unconfirmed": true}

// 官方 REASON_KEYS（OFF:src/client/Card.tsx:47）：有专属文案的拒绝理由。
var reasonText = map[string]string{
	apiclient.PluginRecommendationReasonUnknown:      zhUnknown,
	apiclient.PluginRecommendationReasonUnavailable:  zhUnavailable,
	apiclient.PluginRecommendationReasonAccessDenied: zhAccessDenied,
	apiclient.PluginRecommendationReasonNotLiveRoot:  zhNotLiveRoot,
	apiclient.PluginRecommendationReasonClosing:      zhClosing,
}

// RecoSource 由壳注入；nil = 没有目录文案能力（卡片退化成只读标题）。
type RecoSource interface {
	Describe(ctx context.Context, sessionID, callID string) (*apiclient.PluginRecommendationDetails, error)
}

var (
	injectMu        sync.RWMutex
	injected        RecoSource
	injectedDetails func(name string)
)

// SetSource 由壳注入数据面（**必须在 Load 之后**；传 nil 收回能力）。
func SetSource(src RecoSource) {
	injectMu.Lock()
	injected = src
	injectMu.Unlock()
}

// SetOnOpenDetails 由壳注入「查看详情」（官方 `pluginNavigation.openBundle`）。
//
// 传 nil = 宿主没有插件页 → 标题不可点（不画假按钮：点了没反应比不可点更糟）。
func SetOnOpenDetails(fn func(name string)) {
	injectMu.Lock()
	injectedDetails = fn
	injectMu.Unlock()
}

func currentSource() RecoSource {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return injected
}

func currentOpenDetails() func(string) {
	injectMu.RLock()
	defer injectMu.RUnlock()
	return injectedDetails
}

// lookup 是一张卡的目录查询状态（键是 sessionId + callId）。
type lookup struct {
	// phase 是**发起这次查询时的决定 phase**：phase 变了要重查（官方 useEffect 的依赖里
	// 就有 `phase`，OFF:src/client/Card.tsx:98）——启用之后 `enabled` 必须是宿主的新真值。
	phase string
	done  bool
	// inFlight 只用于"这一帧别重复发"；done 之后由 phase 判重。
	inFlight bool
	// lost = 回复丢失（官方 `setLookup(null)`），与"被拒"是两件事。
	lost    bool
	details apiclient.PluginRecommendationDetails
}

// Plugin 是官方插件推荐卡的正文。
type Plugin struct {
	theme *pluginapi.Theme
	// run 执行一次宿主调用；默认丢给 goroutine（测试换成同步）。
	run func(fn func())

	mu sync.Mutex
	// catalog 是"这个插件名 → 目录文案"的插件生命周期缓存（官方 `catalog` store，
	// OFF:src/client/mount.ts:36）：同一插件的后一张卡立刻有文案，不用等自己的查询回来。
	catalog map[string]apiclient.PluginRecommendationDetails
	// lookups 是每张卡的查询状态。
	lookups map[string]*lookup
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return ServiceID }

// Provides 实现 pluginapi.Plugin：正文由框架持有，本包不对外提供服务键。
func (*Plugin) Provides() []string { return nil }

// Requires 实现 pluginapi.Plugin：没有正文注册表就没地方登记，没有主题就画不出官方配色。
func (p *Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyActionCardBodies}
}

// Register 实现 pluginapi.Plugin：把正文登记进动作卡框架的注册表。
//
// 重复登记**返回错误**（官方 registry 抛异常）：这说明有两个插件抢
// `enable_official_plugin` 的绘制权，静默覆盖会让其中一个永远看不见自己的改动。
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme, _ = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	registry, _ := ctx.Inject(pluginapi.KeyActionCardBodies).(pluginapi.ActionCardBodyRegistry)
	if registry == nil {
		return errors.New("ds-pluginreco: 没有动作卡正文注册表（ds-actioncards 未装配）")
	}
	return registry.RegisterBody(p)
}

// Action 实现 pluginapi.ActionCardBody：这个正文画哪一种卡。
func (*Plugin) Action() string { return Action }

// Draw 实现 pluginapi.ActionCardBody：画官方 PluginCardBody（OFF:src/client/Card.tsx:99-159）。
func (p *Plugin) Draw(f ui.C, env *pluginapi.Env, props pluginapi.ActionCardBodyProps) {
	// 主题在 Register 里注入；env.Theme 是同一份（无头测试直接构造 Plugin 时用它兜底）。
	if p.theme == nil && env != nil {
		p.theme = env.Theme
	}
	t := p.theme
	if t == nil {
		return
	}

	name, hasName := detailsName(props.Details)

	// 官方：details 里没有 name → 只画 summary（标题 + brief），不给任何动作
	// （OFF:src/client/Card.tsx:99 `if (name === undefined) return <div …>{summary}</div>`）。
	if !hasName {
		body := ui.Column(f).FillWidth().PadSides(recoBodyPad, recoBodyPad, recoBodyPad, recoBodyPad)
		body.Children(func() {
			ui.Text(f, props.Title).FontSize(recoTitleFont).LineHeight(recoTitleLineH).
				Weight(recoTitleWeight).Color(t.LabelPrimary)
			if strings.TrimSpace(props.Brief) != "" {
				ui.Text(f, props.Brief).MarginTop(recoDescMarginTop).
					FontSize(recoDescFont).LineHeight(recoDescLineH).Color(t.LabelSecondary)
			}
		})
		return
	}

	// 没有数据源 = 这个部署没有 `pluginRecommendations` Remote（官方那边整个插件
	// 都不会 mount，`inject: ['remote', …]`）。这里退化成"只画标题与简介"：
	// 画一个点了没反应的「启用」比不画更糟。
	if currentSource() == nil {
		body := ui.Column(f).FillWidth().PadSides(recoBodyPad, recoBodyPad, recoBodyPad, recoBodyPad)
		body.Children(func() {
			ui.Text(f, props.Title).FontSize(recoTitleFont).LineHeight(recoTitleLineH).
				Weight(recoTitleWeight).Color(t.LabelPrimary)
			if strings.TrimSpace(props.Brief) != "" {
				ui.Text(f, props.Brief).MarginTop(recoDescMarginTop).
					FontSize(recoDescFont).LineHeight(recoDescLineH).Color(t.LabelSecondary)
			}
		})
		return
	}

	p.ensureLookup(env, props, name)
	state := p.stateOf(props)

	// ── 计算这一帧要画什么（逐条对齐 OFF:src/client/Card.tsx:104-113）────────
	available := state.current != nil && state.current.Available()
	var currentEnabled bool
	if available {
		currentEnabled = state.current.Enabled
	}
	phase := ""
	if props.HasDecision {
		phase = props.Phase
	}
	data := cardDataOf(props.Data)
	// settled：已知 phase 且不在 OPEN_PHASES 里（官方 `settled`，:117-118）。
	settled := phase != "" && !openPhases[phase]
	statusKey := ""
	switch {
	case phase == "":
		if available && currentEnabled {
			statusKey = zhAlreadyOn
		}
	case phase == "enabled":
		// `restart-required` 细化「已启用」（`data.application`，OFF:src/client/Card.tsx:57）。
		if data.application == "restart-required" {
			statusKey = zhRestartNeeded
		} else {
			statusKey = zhEnabled
		}
	default:
		if text, ok := statusOf(phase); ok {
			statusKey = text
		}
	}
	// offered：还没定局，且宿主没有说它已经开着（官方 :119-121）。
	offered := !settled && !(available && currentEnabled)
	enableShown := offered && (available || state.loading)
	// 查询问题行（官方 `lookupProblem`，:124-126）：settled 之后不再提示。
	lookupProblem := ""
	if !settled && !state.loading && !available {
		if state.lost {
			lookupProblem = zhLookupFailed
		} else if state.current != nil {
			if text, ok := reasonText[state.current.Reason]; ok {
				lookupProblem = text
			} else {
				lookupProblem = zhUnavailable
			}
		}
	}
	// 启用失败的两条专属文案（官方 `failure`，:127-128）。
	failure := ""
	if phase == "failed" && data.errorCode != "" {
		if data.errorCode == "incompatible-version" {
			failure = zhVersionError
		} else {
			failure = zhManageError
		}
	}
	// 动作反馈行（官方 `error`，:130 + :150）：只有还能操作时才画。
	actFeedback := ""
	if offered {
		switch {
		case props.ActError != "":
			actFeedback = props.ActError
		case props.LastResult != nil &&
			(props.LastResult.Status == apiclient.ActionCardRefused ||
				props.LastResult.Status == apiclient.ActionCardUnconfirmed):
			if text, ok := reasonText[props.LastResult.Reason]; ok {
				actFeedback = text
			} else {
				actFeedback = zhUnavailable
			}
		case props.LastResult == nil:
			// 还没有结果：不画。
		}
	}

	title := props.Title
	if available {
		title = state.current.Title
	}

	// ── header：通栏（官方 `.header`，Card.module.css:1）─────────────────────
	header := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).Gap(recoHeaderGap).
		PadSides(recoHeaderPadY, recoHeaderPadX, recoHeaderPadY, recoHeaderPadX).
		Background(t.BgModulePlatform)
	header.Children(func() {
		ui.Text(f, zhHeading).FontSize(recoHeaderFont).LineHeight(recoHeaderLineH).Color(t.LabelSecondary)
		// 官方 `.badge`（:2）没有额外底色/边框，只是同一行右侧的一格字。
		ui.Text(f, zhOfficialBadge).FontSize(recoHeaderFont).LineHeight(recoHeaderLineH).Color(t.LabelSecondary)
	})

	body := ui.Column(f).FillWidth().PadSides(recoBodyPad, recoBodyPad, recoBodyPad, recoBodyPad)
	body.Children(func() {
		// ── 标题 + 描述（官方 `.details`，:4-8）：**整块**是一个按钮，含标题与描述 ──
		//
		// 几何：`width:calc(100% + 16px); margin:-8px; padding:8px` → 四边各外扩 8px，
		// 于是标题文字的左缘落在 body 内容框左缘（而不是再内缩 8px），hover 面比内容宽 16px。
		details := ui.Column(f).FillWidth().
			MarginSides(-recoDetailsBleed, -recoDetailsBleed, -recoDetailsBleed, -recoDetailsBleed).
			Pad(recoDetailsPad).Radius(t.Radii.SM).
			Hover(ui.Look{}.Background(t.Hover)).
			// `:focus-visible` 的底色与 hover 同一条令牌（:5-6）；2px 内描边见文件头「没搬」1。
			Focus(ui.Look{}.Background(t.Hover))
		// 没有插件页就没有可点入口（不画点了没反应的按钮）。
		open := currentOpenDetails()
		if open != nil {
			var hitID gfx.ID
			if env != nil && env.Tree != nil {
				hitID = env.Tree.ID(env.Feat, "reco-details-"+props.CallID)
			}
			details = details.Hit(hitID, detailsLabel(title))
			if details.Clicked() {
				open(name)
			}
		}
		details.Children(func() {
			ui.Text(f, title).FontSize(recoTitleFont).LineHeight(recoTitleLineH).
				Weight(recoTitleWeight).Color(t.LabelPrimary)
			// 描述区：官方 `(loading || available !== undefined)` 才画这一块（Card.tsx:134）。
			// 文案没到之前画 `.placeholder` 骨架条（:9），到货后换成正文——两者占同一块高度，
			// 所以卡片不会因为描述到达而跳高。
			switch {
			case available:
				ui.Text(f, state.current.Description).MarginTop(recoDescMarginTop).
					FontSize(recoDescFont).LineHeight(recoDescLineH).Color(t.LabelSecondary)
			case state.loading:
				// 行盒高度 = `.description` 的 line-height（22.1），骨架条在其中垂直居中。
				line := ui.Row(f).FillWidth().MarginTop(recoDescMarginTop).
					Height(recoDescLineH).Items(ui.Center)
				line.Children(func() {
					ui.Row(f).WidthPercent(recoPlaceholderPercent).Height(recoPlaceholderHeight).
						Radius(t.Radii.XS).Background(t.BgSkeleton)
				})
			}
		})

		// ── 状态行（`.status`，role=status）──────────────────────────────────
		for _, line := range []string{statusKey, failure, lookupProblem, actFeedback} {
			if line == "" {
				continue
			}
			// 官方 `.status` 是 `border-top` + `padding-top:12px` + `margin-top:16px`。
			// viva 的 Box 没有单边框，用一条 0.5px 的分隔线盒表达同一个几何。
			ui.Divider(f).FillWidth().Height(recoStatusBorder).MarginTop(recoStatusMarginTop).
				Background(t.BorderL1)
			ui.Text(f, line).PadTop(recoStatusPadTop).
				FontSize(recoStatusFont).LineHeight(recoStatusLineH).Color(t.LabelSecondary)
		}

		// ── footer：左侧 scope 文案，右侧三个按钮（官方 `.footer`，:10）────────
		if !offered {
			return
		}
		// 官方 `.footer` 的 `border-top:0.5px`：同样用分隔线盒（`flex-wrap` 见文件头「没搬」2）。
		ui.Divider(f).FillWidth().Height(recoFooterBorder).MarginTop(recoFooterMarginTop).
			Background(t.BorderL1)
		footer := ui.Row(f).FillWidth().Items(ui.Center).Justify(ui.Between).
			Gap(recoFooterGap).PadTop(recoFooterPadTop)
		footer.Children(func() {
			ui.Text(f, zhScope).FontSize(recoScopeFont).LineHeight(recoScopeLineH).Color(t.LabelSecondary)
			// `.actions{gap:8px; margin-left:auto}`（:12）：右对齐由 footer 的 space-between 给出。
			actions := ui.Row(f).Items(ui.Center).Gap(recoActionGap)
			actions.Children(func() {
				busy := props.Acting || state.loading
				// 1) 重试查询（官方只在 lookupProblem 时给，:145-148）。
				if lookupProblem != "" {
					btn := ui.Button(f, zhRetryLookup).
						Hit(p.hitID(env, "reco-retry-lookup-"+props.CallID), zhRetryLookup).
						Disabled(props.Acting)
					if btn.Clicked() {
						p.resetLookup(props)
						p.invalidate(env)
					}
				}
				// 2) 暂不需要（官方 ghost，:149-151）。
				dismiss := ui.Button(f, zhDismiss).
					Hit(p.hitID(env, "reco-dismiss-"+props.CallID), zhDismiss).Disabled(busy)
				if dismiss.Clicked() {
					props.Act("dismiss", nil)
				}
				// 3) 启用 / 重试启用（官方 primary；phase 已有值就是 `retry`，:152-154）。
				if enableShown {
					label := zhEnable
					if props.Acting {
						label = zhWorking
					} else if phase != "" {
						label = zhRetryEnable
					}
					enable := ui.Button(f, label).
						Hit(p.hitID(env, "reco-enable-"+props.CallID), label).
						Background(t.ButtonPrimaryFill).Color(t.ButtonPrimaryFg).Disabled(busy)
					if enable.Clicked() {
						props.Act("enable", nil)
					}
				}
			})
		})
	})
}

// cardState 是一次 describe 在界面上的三种落点（官方 undefined / null / 结果）。
type cardState struct {
	loading bool
	lost    bool
	current *apiclient.PluginRecommendationDetails
}

// stateOf 把查询状态投影成官方那三档（OFF:src/client/Card.tsx:104-107）。
func (p *Plugin) stateOf(props pluginapi.ActionCardBodyProps) cardState {
	name, _ := detailsName(props.Details)
	key := lookupKey(props)
	p.mu.Lock()
	st := p.lookups[key]
	cached, hasCached := p.catalog[name]
	p.mu.Unlock()

	if st == nil || !st.done {
		// 还没有结果：**目录缓存优先**（官方 `cached ?? lookup` 里 cached 先算，
		// 所以同一插件的第二张卡不必等自己的查询）。
		if hasCached {
			return cardState{current: cachedCopy(cached)}
		}
		return cardState{loading: true}
	}
	if st.lost {
		return cardState{lost: true}
	}
	if st.details.Status == apiclient.PluginRecommendationRefused {
		return cardState{current: cachedCopy(st.details)}
	}
	if hasCached {
		return cardState{current: cachedCopy(cached)}
	}
	return cardState{current: cachedCopy(st.details)}
}

// cachedCopy 复制一份目录文案：界面只读，但不该把 map 里的值按地址递出去
// （`PluginRecommendationDetails` 里有字符串，值语义复制是免费的）。
func cachedCopy(d apiclient.PluginRecommendationDetails) *apiclient.PluginRecommendationDetails {
	out := d
	return &out
}

// ensureLookup 在需要时发起一次 describe（每 (session, call, phase) 一次）。
func (p *Plugin) ensureLookup(env *pluginapi.Env, props pluginapi.ActionCardBodyProps, name string) {
	src := currentSource()
	if src == nil || props.SessionID == "" || props.CallID == "" {
		return
	}
	phase := ""
	if props.HasDecision {
		phase = props.Phase
	}
	key := lookupKey(props)
	p.mu.Lock()
	st := p.lookups[key]
	if st != nil && (st.inFlight || (st.done && st.phase == phase)) {
		p.mu.Unlock()
		return
	}
	if st == nil {
		if p.lookups == nil {
			p.lookups = map[string]*lookup{}
		}
		st = &lookup{}
		p.lookups[key] = st
	}
	st.phase = phase
	st.done = false
	st.inFlight = true
	p.mu.Unlock()

	sessionID, callID := props.SessionID, props.CallID
	p.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), recoLookupTimeout)
		defer cancel()
		details, err := src.Describe(ctx, sessionID, callID)
		p.mu.Lock()
		st.inFlight = false
		st.done = true
		if err != nil || details == nil {
			// 失败与丢包在界面上是同一档（官方 `setLookup(null)`）：都画「无法读取插件信息」，
			// 但**宿主原文不进卡片**——这张卡的文案只有官方的 25 条。
			st.lost = true
		} else {
			st.lost = false
			st.details = *details
			if details.Available() {
				if p.catalog == nil {
					p.catalog = map[string]apiclient.PluginRecommendationDetails{}
				}
				p.catalog[name] = *details
			}
		}
		p.mu.Unlock()
		p.invalidate(env)
	})
}

// resetLookup 是「重试」按钮：清掉这一张卡的查询状态，下一帧重发。
func (p *Plugin) resetLookup(props pluginapi.ActionCardBodyProps) {
	p.mu.Lock()
	if p.lookups != nil {
		delete(p.lookups, lookupKey(props))
	}
	p.mu.Unlock()
}

// dispatch 执行一次宿主调用（产品路径丢 goroutine，测试换成同步）。
func (p *Plugin) dispatch(fn func()) {
	if p.run != nil {
		p.run(fn)
		return
	}
	go fn()
}

// invalidate 请壳重绘一帧；env 可能为 nil（无头调用不重绘）。
func (p *Plugin) invalidate(env *pluginapi.Env) {
	if env != nil && env.Act.Invalidate != nil {
		env.Act.Invalidate()
	}
}

func (p *Plugin) hitID(env *pluginapi.Env, name string) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, name)
}

// lookupKey 是一张卡的查询键。
//
// 为什么带 sessionId：同一 callId 在不同会话里是两张不同的卡，宿主对
// "非 live root 的会话"回 refused{not-live-root}——串会话会把上一张卡的
// 目录文案贴到这一张上。
func lookupKey(props pluginapi.ActionCardBodyProps) string {
	return props.SessionID + "\x00" + props.CallID
}

// detailsSchema 是官方 `detailsSchema = z.object({ name: z.string() })`
// （OFF:src/client/Card.tsx:39）：只多认一个字段，多余的不管。
func detailsName(details map[string]json.RawMessage) (string, bool) {
	if details == nil {
		return "", false
	}
	raw, ok := details["name"]
	if !ok {
		return "", false
	}
	var name string
	if err := json.Unmarshal(raw, &name); err != nil {
		return "", false
	}
	if name == "" {
		return "", false
	}
	return name, true
}

// cardData 是官方 `dataSchema`（OFF:src/client/Card.tsx:41-42）的投影。
type cardData struct {
	application string
	errorCode   string
}

// cardDataOf 解决定里的 data；解不出来就当空（官方 `safeParse(… ?? {})`）。
func cardDataOf(raw map[string]json.RawMessage) cardData {
	var out cardData
	if raw == nil {
		return out
	}
	if value, ok := raw["application"]; ok {
		_ = json.Unmarshal(value, &out.application)
	}
	if value, ok := raw["errorCode"]; ok {
		_ = json.Unmarshal(value, &out.errorCode)
	}
	return out
}

// statusOf 是官方 `statusKey` 的 Go 版（OFF:src/client/Card.tsx:54-60）里"phase 直接对应文案"那一档。
//
// `enabled` 单独处理（它要看 data.application），所以不在这张表里。
func statusOf(phase string) (string, bool) {
	switch phase {
	case "dismissed":
		return zhDismissed, true
	case "replaced":
		return zhReplaced, true
	case "already-enabled":
		return zhAlreadyOn, true
	case "failed":
		return zhFailed, true
	case "unconfirmed":
		return zhUnconfirmed, true
	default:
		return "", false
	}
}

// detailsLabel 是标题按钮的无障碍名（官方 `t('detailsFor', {title})`）。
func detailsLabel(title string) string {
	if title == "" {
		return strings.Replace(zhDetailsFor, "%s", "", 1)
	}
	return strings.Replace(zhDetailsFor, "%s", title, 1)
}
