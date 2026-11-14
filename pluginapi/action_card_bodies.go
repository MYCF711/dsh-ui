package pluginapi

import (
	"encoding/json"

	"github.com/vivaui/viva/ui"
)

// KeyActionCardBodies 是动作卡正文注册表的能力键（官方 `ctx.actionCards` 服务）。
//
// 提供方：`ds-actioncards`（唯一）。消费方：各 action kind 的插件
// （目前是 `ds-pluginreco`）。nil = 老壳/没有动作卡插件 → 消费方**不注册**自己
// （注册进一个不存在的注册表没有意义），也不许因此不画自己的卡片：卡片框那一层
// 本来就不归它管。
const KeyActionCardBodies = "ui.actionCardBodies"

// action_card_bodies.go —— 官方「按 kind 注册卡片正文」这条通道的 Go 对等物
// （`ctx.actionCards.register({action, locale, Body})`）。
//
// # 官方形状（唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-action-cards
//	OFF:src/client/registry.ts:80-115   ActionCardRegistry：按 action 名注册，重复即抛
//	OFF:src/client/registry.ts:34-69    body 收到的 props（proposal / decision /
//	                                    sessionId / summary / Folded / act / t）
//	OFF:src/client/Card.tsx:99-118      **框架**负责卡片框（`<section class=card>`），
//	                                    注册了 body 就整段交给 body，否则画 FallbackBody
//	OFF:src/client/mount.ts:47-49       `ctx.reflect.provide('actionCards', registry)`
//
// 官方注册方（本仓两个消费方）：
//
//	`dsh-client-ui-plugin-recommendations` → `enable_official_plugin`（已落地）
//	`dsh-client-ui-creation-recommendations` → `create_personalized_page`（宿主无此 kind）
//
// # 为什么这是 pluginapi 的一格而不是 `dsactioncards` 的导出
//
// 插件之间协作只走 `Provide/Inject`（红线 R2：插件只拿 *Scope，不互相 import）。
// 官方那边 body 注册方是通过 cordis 的 `ctx.actionCards` 服务拿到的；本仓对等物就是
// 一个服务键 + 一个接口。这样 `plugins/dspluginreco` 不需要 import `dsactioncards`，
// 装配顺序由 `Requires()` 声明，而不是靠 init 顺序撞运气。
//
// # 与官方的三处**有意**差异（照实登记）
//
//  1. **没有 `locale`**：官方 body 自带一个 locale 命名空间，文案由 `t()` 取。
//     本仓的界面文案一律是**证据行 + 中文常量**（各插件包顶部注释逐条写 OFF:行号），
//     没有运行时字典，所以 `Action() + Draw()` 就够了；多一个永远不被读的字段
//     只会让后来的人以为这里真有 i18n。
//  2. **没有 `Folded`**：官方的 `Folded` 是"按字素宽度折叠"的组件，只有 FallbackBody
//     用它画 brief。kind body（两个推荐卡）都不画 brief——它们画自己目录里的
//     description，所以这条参数在真实调用点上是空转的。
//  3. **`Act` 是火并忘**：官方 `act()` 返回 Promise，body 自己 await 后落本地态。
//     本仓的绘制是同步的，await 无处可放；所以动作的**在飞/结果/错误**由框架持有
//     并通过 `Acting / LastResult / ActError` 回灌给 body，body 只管画。
type ActionCardBodyProps struct {
	// SessionID 是这张卡所在的会话（body 查自己的数据时用它，不许读"当前选中"）。
	SessionID string
	// CallID / Action / Title / Brief / Details 是官方 proposal 的逐字段投影。
	CallID  string
	Action  string
	Title   string
	Brief   string
	Details map[string]json.RawMessage
	// Phase / HasDecision / Data 是官方 decision 的投影。
	//
	// `HasDecision=false` 时 Phase 一定是空串：Phase 本身是开放串，空串不是合法
	// 决定，两者不能混（与 apiclient.ActionCardView.Decision 同一个理由）。
	Phase       string
	HasDecision bool
	Data        map[string]json.RawMessage
	// Acting 表示有一次动作在飞（官方 `pending`）：body 据此禁用两个按钮。
	Acting bool
	// LastResult 是宿主真实返回的四变体（nil = 还没有结果）。
	LastResult *ActionCardResultInfo
	// ActError 是动作失败时的**宿主原文**（空串 = 没有失败）。
	ActError string
	// Act 发一次用户动作；payload 只在 kind 自己的动词需要时给（dismiss 不给）。
	Act func(verb string, payload map[string]json.RawMessage)
}

// ActionCardResultInfo 是官方 `ActionCardResult` 四变体的投影。
//
// 只搬 body 真的会读的字段：`status` 决定画哪一档，`reason` 是 refused/unconfirmed
// 时的拒绝理由（官方 body 用它取本地化文案），`phase` 是 settled 带回来的新决定
// （框架已经并进投影，body 通常只读 Phase 就够，这里给出是为了让"这一帧结果"可见）。
type ActionCardResultInfo struct {
	Status string
	Reason string
	Phase  string
}

// ActionCardBody 是一种 action kind 的卡片正文。
//
// 官方注册项形状见 OFF:registry.ts:80-88（`{action, locale, Body}`）。本仓省掉 locale
// （理由见文件头第 1 条），保留 `Action()` 作为注册键——body 自己说得出自己画哪种卡，
// 而不是让注册方多传一个必然重复的字符串。
type ActionCardBody interface {
	// Action 是注册的 kind 名（官方 `ActionCardDescriptor.action`）。
	Action() string
	// Draw 在**已经画好卡片框**的列里画正文（框的描边/圆角/底色由框架给）。
	//
	// 官方两种 body 都自己画 `.header`（通栏）与 `.body`（padding 20），框架只给
	// `<section>` 那层框，所以这里也不替 body 加内边距。
	Draw(c ui.C, env *Env, props ActionCardBodyProps)
}

// ActionCardBodyRegistry 由 `ds-actioncards` 提供（官方 `ctx.actionCards`）。
//
// 重复注册同一个 action 必须**报错而不是覆盖**（OFF:registry.ts:104-106 抛
// `a body for "…" is already registered`）：两个插件抢一种卡片的绘制权，一定有一方
// 写错了，静默覆盖会让被覆盖的那一方永远看不见自己的改动。
type ActionCardBodyRegistry interface {
	// RegisterBody 登记一种 kind 的正文；同一 action 重复登记返回错误。
	RegisterBody(body ActionCardBody) error
	// BodyFor 取一种 kind 的正文；没有返回 nil（框架此时画官方 FallbackBody）。
	BodyFor(action string) ActionCardBody
}
