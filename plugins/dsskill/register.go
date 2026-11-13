// register.go —— `dsh-client-ui-skill` 的 Go 外壳：把专用工具行装进 ds-tool。
//
// # 落座判据（官方 `index.ts` 的五个注册点逐条对照）
//
// 官方 `apply` 里注册了五个面，本包按「**今天有没有可挂的座位**」逐条判定：
//
//	① `ctx.locale.register('skill', {zh,en,…})`
//	   → 本仓**没有** locale 注册表（`grep -rn "locale.register" .` 零命中）。
//	     本包把 8 条字典做成数据（`locales.go`），由 `SkillText` 走回退阶梯。
//	     缺口见报告 §3。
//
//	② `ctx.uiConversation.userText.register(skillReferenceRule)`
//	   → 本仓**没有** `userText` 注册表。规则实现成纯函数
//	     （`user_text_rules.go` 的 `skillReferenceRule`），逐条断言见
//	     `user_text_rules_test.go`。缺口见报告 §3。
//
//	③ `ctx.slots.inject('tool.call.toolview', …{key:'skill'}…)` → `SkillRow`
//	   → **这是唯一今天可达的座位**。本仓的 `tool.call.toolview` 由
//	     `plugins/dstool` 的 `Renderer` 分派表承担（`Cards.Register`），
//	     本包 `Register` 里 `cards.Register(dstool.Renderer{Tool:"skill", Draw:…})`
//	     就是官方那一行。**不做成独立插件挂槽**：官方注册的是 keyed 槽的
//	     一个 key，本仓对应的就是分派表里的一行。
//
//	④ `ctx.slots.inject('conversation.user.text.node', …{key:'skill',priority:100}…)`
//	   → 槽名在本仓**存在**（`uikernel.OffUserTextNode`），但**没有插件 dispatch 它**
//	     （`shell_skeleton.go` 里只 `dispatch(OffUserTextNode)` 到一个空座）。
//	     本包提供 `SkillReference` 的绘制与 `openSkill` 注入面（`skillreference.go`），
//	     座位接通后一行就能挂上。缺口见报告 §3。
//
//	⑤ `inputTriggers.registerSource({trigger:'/', name:'skill', order:2, …})`
//	   → 本仓**没有** inputTriggers 注册表。候选排序契约（`rankByName`）与
//	     `menu.userOnly` 标记的语义写在 `official_skill.go` 的常量与
//	     `locales.go` 里。缺口见报告 §3。
//
// 一句话：**只有 ③ 今天能挂，其余四个面本仓还没有座位。**
// 这不是"做不完"，是"座位表里就没有那几个座位"——报告 §3 逐条登记。
package dsskill

import (
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
	"github.com/vivaui/viva/ui"
)

// ServiceKey 是本插件提供的能力键。
const ServiceKey = "ui.skill"

// SkillToolName 是官方 keyed 槽的 key，也是宿主工具名（`tool.name === 'skill'`）。
const SkillToolName = "skill"

// defaultLang 是字典默认语言（官方 `zh` 是键集真源）。
const defaultLang = "zh"

// registrar 是 ds-tool 暴露给第三方插件的登记面（`*dstool.Cards` 满足它）。
//
// 为什么不直接 assert 到 `*dstool.Cards`：那样会把「换一套工具卡实现」
// 变成改本包一行代码。官方 `index.ts` 依赖的是 `ctx.slots` 的**能力**，
// 不是某个实现类型——Go 侧用最小接口表达同一件事。
type registrar interface {
	Register(dstool.Renderer)
}

func init() {
	pluginapi.Register("ds-skill", func() pluginapi.Plugin { return &Plugin{} })
}

// Plugin 是官方 `@deepseek-ai/dsh-client-ui-skill` 的 Go 外壳。
//
// 它只做三件事：报 ID、把 `skill` 的渲染器登记进 ds-tool、持有每条卡的展开位。
// 画法与状态机全在 `skillrow.go`，所以换渲染实现不影响装配。
type Plugin struct {
	// expand 是每条卡的展开位（官方每条 `SkillRow` 一个 `useState`）。
	expand *skillExpandState
	// lang 是字典语言（官方 `ctx.locale.bind(NS)` 的等价物）。
	// 本仓没有 locale 服务，由壳通过 `SetLang` 注入；空 = zh。
	lang string
	// cards 是登记进来的工具卡服务（`Register` 时拿到）。
	cards registrar
	// inspect 是官方注入的 `openSkill`（`SkillReferenceInjected`）。
	//
	// **今天恒为 nil**：官方 `openSkill` 要用 `skills/list` 的目录把名字解析成
	// 路径（`index.ts:205-223` 的 `catalog.find(skill => '/'+skill.name === ref)?.path`），
	// 而本仓宿主**没有 `skills/list` 方法**（`grep -rn "skills/list" internal/apiclient/`
	// 零命中，见报告 §1）。按官方语义 `inspect !== undefined` 才画按钮，
	// 所以今天**整枚按钮不渲染**——这是正确形状，不是漏实现。
	inspect func(name string)
}

// ID 实现 pluginapi.Plugin。
func (*Plugin) ID() string { return "ds-skill" }

// Provides 实现 pluginapi.Plugin。
func (*Plugin) Provides() []string { return []string{ServiceKey} }

// Requires 实现 pluginapi.Plugin。
//
// 只要工具卡：本插件是"画在别人的框里"的部件（它不占自己的槽位），
// 所以不需要任何布局能力，也就不该让装配顺序为它让步。
func (*Plugin) Requires() []string { return []string{dstool.ServiceKey} }

// Register 实现 pluginapi.Plugin。
//
// 装配动作逐条对应官方 `index.ts:86-92`：
//
//	官方 ctx.slots.inject('tool.call.toolview', …register({key:'skill'})…)
//	本仓 cards.Register(dstool.Renderer{Tool:'skill', Draw:…})
func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.expand = newSkillExpandState()
	p.lang = defaultLang
	if cards, ok := ctx.Inject(dstool.ServiceKey).(registrar); ok {
		p.cards = cards
		cards.Register(dstool.Renderer{Tool: SkillToolName, Draw: p.draw})
	}
	ctx.Provide(ServiceKey, p)
	return nil
}

// SetLang 注入字典语言（壳在有 locale 服务时调；未调 = zh）。
func (p *Plugin) SetLang(lang string) {
	if lang == "" {
		lang = defaultLang
	}
	p.lang = lang
}

// SetInspect 注入官方 `openSkill`（壳在 `skills/list` 可用时调）。
//
// 传 nil = 撤掉（按钮随之消失）——官方 `inspect !== undefined` 就是这条语义。
func (p *Plugin) SetInspect(inspect func(name string)) { p.inspect = inspect }

// InspectForTest 报告当前有没有注入 resolver。
//
// 为什么要有它：接缝的**失败语义**才是关键——拉不到 skill 目录时必须撤掉
// （官方 `inspect !== undefined` 才画按钮，留一个空转函数就是"点了没反应"）。
// 壳侧的回归测试要能断言"撤掉了"，而 inspect 是未导出字段，跨包看不见。
func (p *Plugin) InspectForTest() func(name string) { return p.inspect }

// draw 是登记进 ds-tool 的那个 `Renderer.Draw`：**整行接管**。
//
// 官方 keyed toolview 的语义就是"这个工具的整行由这个组件负责"，
// 不是"在通用 ToolRow 里补两段"（见 `plugins/dstool.Renderer.Draw` 的注释）。
//
// 两条分支对应官方 `SkillRow` 的两个 return：
//
//	phase === 'preparing' → `drawPreparing`（`SkillRow.tsx:103-109`）
//	否则                  → `drawSkillRow`（`SkillRow.tsx:113-172`）
//
// **本仓没有 `phase` 字段**：`dstool.Card` 的 `Status` 由消息流从显式结果、
// 原始终局或活动 Turn 算出（`plugins/dstool/status.go`），
// `StatusPreparing` 就是官方的 `phase === 'preparing'`——两者是同一条事实的
// 两个名字（"调用参数还没落盘"），所以直接用它判别，不另造一个字段。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env, c dstool.Card) float32 {
	if c.Status == dstool.StatusPreparing {
		return p.drawPreparing(f, env, c)
	}
	return p.drawSkillRow(f, env, c)
}
