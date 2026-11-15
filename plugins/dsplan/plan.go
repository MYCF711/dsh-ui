// Package dsplan 是官方 `dsh-client-ui-plan` 的 Go 版：**计划产物**的
// 读取（会话 journal 里的 `exit_plan_mode` 调用）+ 两处可见面（Turn 尾计划卡、
// 右栏计划全文）。
//
// # 结构证据（唯一权威）
//
//	OFF = DSH-GO/tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/dsh-client-ui-plan/src/client
//
//	OFF:plan.ts              33-48 行：`submittedPlan(event)`（真值来源与校验）
//	OFF:plan.ts:56-87        `planAddress` / `parsePlanAddress`（`dsh-resource://plan/…`）
//	OFF:PlanCard.tsx         83 行：Turn 尾卡片（图标槽 / 标题 / 说明 / 打开）
//	OFF:PlanPreview.tsx      54 行：右栏全文（loading / failed / 完整 Markdown）
//	OFF:PlanPreview.module.css      30 行：卡片与全文**全部几何值**
//	OFF:index.ts:79-113      两个可见座位的注册（turnTail / sidebar.right.pane.tab）
//	OFF:locales.ts:4-21      zh 字典 16 条（本包**只许**用这些原文）
//
// # 数据面证据（宿主是真的，不是注释）
//
//	HOST:modules/tool/plan/plan.go:196-217   `exit_plan_mode` 工具（schema 只有 plan）
//	HOST:modules/tool/plan/plan.go:218-240   成功即 `Result.Stop`（:234）——一轮停在方案上
//	HOST:internal/session/log.go:168-176     `tool/call` 帧带 callId/name/arguments
//	官方 plan.ts:33-48 与它是同一个形状（官方也是从 `tool/call` 的 arguments 读）
//
// 所以本包**不需要新的宿主方法**：数据已经在 `env.Data.Frames`（原始 journal 帧）
// 里，与官方 `planResourceProvider` 读 session history 是同一份事实，只是本仓
// 客户端已经把这段帧缓存好了（`pluginapi.Snapshot.Frames` 的注释写明它就是
// `session.history` 的 frames[]）。
//
// # 官方四个座位，本包只做了两个（照实登记）
//
//	conversation.chat.turnTail          ✅ 本包 cards.go（Turn 尾计划卡）
//	sidebar.right.pane.tab (key plan)   ✅ 本包 preview.go（右栏计划全文）
//	conversation.input.plan             ❌ HIDE：计划模式 chip 读的是官方 `plan` 投影
//	                                       （OFF:dsh-plan-mode/src/types.ts:21-24 的
//	                                       `{active, pending}`），本仓宿主没有这个投影
//	                                       （`session.values` 只有 title/actionCards/todos/
//	                                       subagent/… 这几项，见
//	                                       HOST:internal/transport/handlers_session_official.go:930-987）
//	conversation.plan-review.actions    ❌ HIDE：官方这条是**待批计划**的「查看全文」
//	                                       链接，挂在 ui-user-questions 的 plan-review
//	                                       提问卡里（OFF:user-questions index.ts:390）。
//	                                       本仓 dsquestions 只认 `ask_user_question`，
//	                                       `exit_plan_mode` 从来不产生待答问题——没有
//	                                       那条提问卡，这个链接就无处可挂（画了永不出现）
package dsplan

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// ServiceID 是本插件的注册名（官方包名 → 本仓插件 id 的既有命名法）。
const ServiceID = "ds-plan"

// planToolName 是官方认的那个工具名（OFF:plan.ts:35-40 `data.name !== 'exit_plan_mode'`）。
//
// 逐字照抄：宿主 `modules/tool/plan/plan.go:211` 的 `Name()` 返回同一个字符串。
const planToolName = "exit_plan_mode"

// 计划帧的三种事件名，逐字来自 OFF:plan.ts:35-38。
//
// 本仓宿主只发 `tool/call`（没有 PTC 运行时），另外两种保留常量是为了
// **解析器与官方同一形状**：将来宿主接 PTC 时不用改这里。
const (
	eventToolCall         = "tool/call"
	eventPTCDispatch      = "tool/ptc-dispatch"
	eventPTCDispatchStart = "tool/ptc-dispatch-start"
)

// Document 是一份**已提交的计划**：官方 `SubmittedPlan`（OFF:plan.ts:17-22）。
//
// 字段一一对应，只多两个本仓要用的定位量：
//   - Turn / Seq 来自宿主 `tool/call` 帧（官方 plan 侧不需要，因为它的 turnTail
//     座位是**按轮**派发的；本仓座位是会话级，只能靠这两个值自己选轮）
type Document struct {
	// CallID 是这次 `exit_plan_mode` 调用的 id（官方 `ToolCallId`）。
	CallID string
	// Turn 是宿主记的轮次（0 = 帧里没有，与"第 0 轮"同值，无法区分）。
	Turn int64
	// Seq 是帧序号：同一轮里以它排序（官方按 invocation order，即日志顺序）。
	Seq int
	// Title 是计划的一级标题（OFF:plan.ts:45 取 markdown 第一行 `# `）。
	Title string
	// Markdown 是计划正文原文（含标题行；官方不裁掉它）。
	Markdown string
}

// SubmittedPlans 从 journal 原始帧里取出**全部**已提交计划，按日志顺序。
//
// 解析规则逐条照 OFF:plan.ts:33-48：
//   - 只认三种计划事件名
//   - `data.name` 必须是 `exit_plan_mode`
//   - callId：`tool/call` 用 `data.callId`；PTC 用 `data.subCallId`
//   - `tool/call` 的 `arguments` 是**字符串**（JSON 文本），解析失败就当没有
//     （官方同一句注释：畸形 JSON 留在通用工具行里，不在这里冒充计划）
//   - `arguments.plan` 必须是字符串
//   - 没有一级标题就**不算计划**（官方 `title === undefined → return undefined`）
func SubmittedPlans(frames []apiclient.Frame) []Document {
	out := make([]Document, 0, 2)
	for _, frame := range frames {
		if doc, ok := submittedPlan(frame); ok {
			out = append(out, doc)
		}
	}
	return out
}

// submittedPlan 解析一帧；不是计划帧就返回 false。
func submittedPlan(frame apiclient.Frame) (Document, bool) {
	switch frame.Type {
	case eventToolCall, eventPTCDispatch, eventPTCDispatchStart:
	default:
		return Document{}, false
	}
	var data struct {
		Name      string          `json:"name"`
		CallID    string          `json:"callId"`
		SubCallID string          `json:"subCallId"`
		Arguments json.RawMessage `json:"arguments"`
		Turn      int64           `json:"turn"`
	}
	if err := json.Unmarshal(frame.Data, &data); err != nil {
		return Document{}, false
	}
	if data.Name != planToolName {
		return Document{}, false
	}
	callID := data.CallID
	if frame.Type != eventToolCall {
		callID = data.SubCallID
	}
	if callID == "" {
		return Document{}, false
	}
	// `tool/call` 的 arguments 是**模型给的 JSON 文本**（一串字符串），
	// PTC 形态已经是对象。两种都归一成对象再取 plan。
	raw := data.Arguments
	if frame.Type == eventToolCall {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return Document{}, false
		}
		raw = json.RawMessage(text)
	}
	var args struct {
		Plan string `json:"plan"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return Document{}, false
	}
	title, ok := planTitle(args.Plan)
	if !ok {
		return Document{}, false
	}
	return Document{CallID: callID, Turn: data.Turn, Seq: frame.Seq, Title: title, Markdown: args.Plan}, true
}

// planTitle 取计划的一级标题。
//
// 官方正则：`/^#\s+(\S[^\r\n]*)/` 作用在 `trim()` 之后（OFF:plan.ts:45）。
// 逐条对上：
//   - 只认**一个** `#`（`## x` 不算：`#` 后面紧跟 `#` 不是空白）
//   - `#` 后面必须有空白（`\s+` 可以跨行，JS 的 `\s` 含换行）
//   - 标题正文从第一个非空白字符起，到行尾为止，行尾空白去掉
func planTitle(markdown string) (string, bool) {
	trimmed := strings.TrimSpace(markdown)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	rest := trimmed[1:]
	if rest == "" || !isPlanSpace(rest[0]) {
		return "", false
	}
	body := strings.TrimLeft(rest, " \t\r\n\f\v")
	if body == "" {
		return "", false
	}
	if nl := strings.IndexAny(body, "\r\n"); nl >= 0 {
		body = body[:nl]
	}
	// 去掉尾部空白，别把行尾空格画进卡片标题。
	body = strings.TrimRight(body, " \t")
	if body == "" {
		return "", false
	}
	return body, true
}

// isPlanSpace 报告一个字节是不是空白。只判 ASCII 的几个：这里判定的是 markdown
// 的语法字符，不是正文内容，与 JS 正则 `\s` 在本题上等价。
func isPlanSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '\f', '\v':
		return true
	}
	return false
}

// latestTurn 取**最新一轮**的计划（官方 turnTail 座位是按轮的，
// OFF:PlanCard.tsx:43-44：`usePlans(String(turn.turn))`）。
//
// 本仓的 `conversation.chat.turnTail` 是会话级座位（`internal/uikernel/
// official_slots.go:428-431` 记的 kind/scope，壳每帧只派发一次），所以这里按
// "最大 Turn 的那一组"选——与 dsactioncards 的「最新一轮 + 最小 callId」同一取舍，
// 那一处的取舍理由与登记同样适用于本包。
func latestTurn(plans []Document) []Document {
	if len(plans) == 0 {
		return nil
	}
	var maxTurn int64
	haveTurn := false
	for _, plan := range plans {
		if !haveTurn || plan.Turn > maxTurn {
			maxTurn, haveTurn = plan.Turn, true
		}
	}
	// `turn` 缺失时所有计划都落在 Turn 0：那一轮"最新"就没有意义了（把整个会话
	// 的计划都堆到当前轮尾部会是错的），此时只留最后提交的那一份——与官方
	// "Turn 尾只显示该轮的计划"在语义上最接近，也不会把旧方案当成这一轮的产物。
	if maxTurn == 0 {
		return []Document{plans[len(plans)-1]}
	}
	out := make([]Document, 0, len(plans))
	for _, plan := range plans {
		if plan.Turn == maxTurn {
			out = append(out, plan)
		}
	}
	return out
}

// planAddress 是官方 `planAddress`（OFF:plan.ts:56-65）的逐字移植：
//
//	dsh-resource://plan/<sessionId>/<callId>
//
// 分段用 `encodeURIComponent` 转义。本仓只支持 `session` 形态（宿主没有
// subagent 会话地址这一层，且 `Cur` 永远是当前会话的 id）。
func planAddress(sessionID, callID string) string {
	if sessionID == "" || callID == "" {
		return ""
	}
	return "dsh-resource://plan/" + uriComponent(sessionID) + "/" + uriComponent(callID)
}

// parsePlanAddress 是官方 `parsePlanAddress`（OFF:plan.ts:69-87）的 session 分支。
//
// 只认两段（session + callId）：subagent 的五段形态需要宿主真发 subagent 会话，
// 本仓没有那条数据，认它就是"能解析但永远解析不到东西"。
func parsePlanAddress(address string) (sessionID, callID string, ok bool) {
	const prefix = "dsh-resource://plan/"
	if !strings.HasPrefix(address, prefix) {
		return "", "", false
	}
	rest := address[len(prefix):]
	if rest == "" || strings.ContainsAny(rest, "?#") {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	for i, part := range parts {
		if part == "" {
			return "", "", false
		}
		decoded, err := url.PathUnescape(part)
		if err != nil || decoded == "" {
			return "", "", false
		}
		parts[i] = decoded
	}
	return parts[0], parts[1], true
}

// uriComponent 是 ECMAScript `encodeURIComponent` 的等价实现。
//
// 为什么不用 `url.PathEscape`：两者转义集不同（`encodeURIComponent` 不转
// `!'()*`、转 `$&+,/:;=?@` 之外的全部；`url.PathEscape` 不转 `$&+,;=:@`）。
// 地址是**两端都要能互相解析**的契约（官方 `planAddress` / `parsePlanAddress`
// 成对），照官方那一侧的形状来才不会在 `:`、`$` 这类字符上分叉。
func uriComponent(value string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()"
	escaped := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			escaped = append(escaped, c)
			continue
		}
		escaped = append(escaped, '%',
			"0123456789ABCDEF"[c>>4],
			"0123456789ABCDEF"[c&0x0f])
	}
	return string(escaped)
}
