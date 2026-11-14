// schedule.go —— 官方 rc.2 `schedule.catalog` / `schedule.edit` / `schedule.run` /
// `schedule.history` 的客户端（官方 `dsh-client-ui-schedule` 的数据面）。
//
// 为什么不用既有的 `schedule.list`：那是本仓宿主的**扩展面**（按会话落盘的
// 精简投影，见 `HOST:internal/transport/handlers_jobs.go:246-280`），不是官方
// `dsh-schedule` 的 catalog。官方任务管理页消费的是 `schedule.catalog`
// （`OFF:ui-schedule/src/client/index.ts:130-160` 的 `remote.schedule.catalog()`），
// 它带 `status / target / involvedSessionIds / lastDelivery / deliveryFloorEndsAt`
// 这些 `schedule.list` 不提供的字段——没有它们就画不出官方的状态与"下次运行"。
//
// # 契约（逐条对宿主 handler，不是凭印象）
//
//	schedule.catalog  参数：无（官方 parameters: []，见
//	                  `HOST:internal/transport/handlers_schedule_official.go:127-135`
//	                  的 `scheduleMethodTakesNoParams`）。结果：**裸数组**，
//	                  元素是官方 `ScheduleCatalogEntry`（7 个 kind 的 union，
//	                  `:200-260` 的 `OfficialScheduleEntry`）。
//	                  宿主未接线回 501 not_implemented，不是空数组。
//	schedule.edit     参数：`request` 对象或**平铺**（`scheduleRequestObject:1085-1102`
//	                  两种都收；官方是单个 request 对象，本仓客户端发平铺）。
//	                  结果 union（`:653-720`）：catalog 条目 / {id,code} /
//	                  {id,deleted:true}。
//	schedule.run      参数：{id}；结果 union：catalog 条目 / {id,code}。
//	schedule.history  参数：{id, limit, before?}（limit 1..100，`:946-990`）；
//	                  结果 union：`OfficialScheduleHistoryPage` / {id,code}。
//
// # 不能吞掉的三件事
//
//  1. **501 不是空**：宿主没有 schedule 能力时 catalog 回 501，界面必须能区分
//     "没有任务"与"查不了"（与 `jobs.list`/`llm.listConfigurableProviders`
//     同一条规矩）。
//  2. **miss 是成功结果**：`schedule_not_found` / `schedule_conflict` /
//     `schedule_execution_pending` / `delivery_cursor_not_found` 都是 HTTP 200 +
//     `{id,code}`（官方 "non-mutating miss"）。它们不是传输错误，调用方必须
//     看 `Code` 决定处置（重新读 / 稍后再试 / 翻到第一页）。
//  3. **deleted 才是真删掉**：`{id,deleted:true}` 之外一律按未删除处理。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// ScheduleTarget 是官方 `ScheduleTarget`（下一次投递去哪）。
//
// kind=session 带 sessionId；new_once / new 带 create（本仓宿主会把 new_once
// 的会话 id 写进记录，故 create.cwd/agentPreset 只在创建时出现）。
type ScheduleTarget struct {
	Kind      string                `json:"kind"`
	SessionID string                `json:"sessionId,omitempty"`
	Create    *ScheduleTargetCreate `json:"create,omitempty"`
}

// ScheduleTargetCreate 是官方 `ScheduleCreateInfo`（new / new_once 的会话创建参数）。
type ScheduleTargetCreate struct {
	CWD         string `json:"cwd,omitempty"`
	AgentPreset string `json:"agentPreset,omitempty"`
}

// ScheduleLastDelivery 是官方一条已确认投递的凭据。
type ScheduleLastDelivery struct {
	ScheduledAt     string `json:"scheduledAt"`
	DeliveredAt     string `json:"deliveredAt"`
	MessageID       string `json:"messageId"`
	TargetSessionID string `json:"targetSessionId"`
}

// ScheduleEntry 是官方 `ScheduleCatalogEntry`（7 个 kind 的 union）。
//
// 字段名逐字对齐宿主 `OfficialScheduleEntry`（`HOST:internal/transport/
// handlers_schedule_official.go:220-270`）。kind 专属字段用 omitempty：
// 同一份记录只会出现其中一组；`timeZone`/`weekdays` 在 weekly/cron 分支是
// required，所以照宿主形状保留（空串/空数组也是"有这句话"）。
type ScheduleEntry struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`

	// 各 kind 专属字段（只会有其中一组）。
	AfterSeconds int64  `json:"afterSeconds,omitempty"`
	EverySeconds int64  `json:"everySeconds,omitempty"`
	Time         string `json:"time,omitempty"`
	TimeZone     string `json:"timeZone"`
	Weekdays     []int  `json:"weekdays,omitempty"`
	Expression   string `json:"expression,omitempty"`
	RRule        string `json:"rrule,omitempty"`

	ScheduledAt         string                `json:"scheduledAt"`
	Target              ScheduleTarget        `json:"target"`
	InvolvedSessionIDs  []string              `json:"involvedSessionIds"`
	Status              string                `json:"status"`
	CreatedAt           string                `json:"createdAt,omitempty"`
	UpdatedAt           string                `json:"updatedAt,omitempty"`
	ExecutionActive     bool                  `json:"executionActive,omitempty"`
	DeliveryFloorEndsAt string                `json:"deliveryFloorEndsAt,omitempty"`
	LastDelivery        *ScheduleLastDelivery `json:"lastDelivery,omitempty"`
}

// ScheduleHistoryRecord 是官方一次已保存的投递。
type ScheduleHistoryRecord struct {
	Prompt          string `json:"prompt,omitempty"`
	ScheduledAt     string `json:"scheduledAt"`
	DeliveredAt     string `json:"deliveredAt"`
	MessageID       string `json:"messageId"`
	TargetSessionID string `json:"targetSessionId"`
}

// ScheduleRetention 是宿主真实的保留口径（与写入方同一份配置）。
type ScheduleRetention struct {
	Days    int `json:"days"`
	Records int `json:"records"`
}

// ScheduleHistoryPage 是 `schedule.history` 的一页（newest first）。
type ScheduleHistoryPage struct {
	ID                        string                  `json:"id"`
	Records                   []ScheduleHistoryRecord `json:"records"`
	EarlierRecordsUnavailable bool                    `json:"earlierRecordsUnavailable"`
	EarlierRecordsPruned      bool                    `json:"earlierRecordsPruned"`
	Retention                 ScheduleRetention       `json:"retention"`
	NextBefore                string                  `json:"nextBefore,omitempty"`
}

// ScheduleEditOutcome 是 `schedule.edit` / `schedule.run` 的联合结果。
//
// 用"哪个字段非零"表达 union，而不是三个返回类型：调用方本来就要按
// Code 分支处置，多一层类型只会让 switch 更长。
type ScheduleEditOutcome struct {
	// Entry 非 nil = 成功且宿主回了任务正文（update / run 的正常结果）。
	Entry *ScheduleEntry
	// Deleted = `{id,deleted:true}`，只有 action=delete 会成功回这个。
	Deleted bool
	// Code 是官方 miss 判别式（schedule_not_found / schedule_conflict /
	// schedule_execution_pending / delivery_cursor_not_found）。空 = 没有 miss。
	Code string
}

// ErrScheduleNeedID 是本地入参校验失败：没有 id 就没有任务可谈。
var ErrScheduleNeedID = errors.New("schedule 调用需要非空 id")

// ScheduleCatalog 取官方的全量任务目录（`schedule.catalog`）。
//
// 无参数方法的官方形状是"没有参数"：传 `{}` 与空 body 等价，这里发 `{}`。
// 未接线（宿主 501）返回 *APIError，调用方必须把"查不了"与"没有任务"分开。
func (c *Client) ScheduleCatalog(ctx context.Context) ([]ScheduleEntry, error) {
	var out []ScheduleEntry
	if err := c.Call(ctx, "schedule.catalog", struct{}{}, &out); err != nil {
		return nil, err
	}
	if out == nil {
		// 宿主回 [] 时与"没有任务"同义；nil 只是 Go 的形状，不额外造数据。
		out = []ScheduleEntry{}
	}
	return out, nil
}

// ScheduleDelete 用官方 `schedule.edit`（action=delete）删掉一条任务。
//
// 删除是**破坏性**动作：只有 `Deleted=true` 才代表宿主真的删了。
// 冲突/不存在走 `Code`（HTTP 200 的 miss），不是错误。
func (c *Client) ScheduleDelete(ctx context.Context, id string) (ScheduleEditOutcome, error) {
	return c.scheduleEdit(ctx, map[string]any{"id": id, "action": "delete"})
}

// ScheduleSetActive 用官方 `schedule.edit`（action=update, patch.active）暂停/恢复。
//
// `expectedUpdatedAt` 是官方乐观锁：给上一帧读到的 updatedAt，被别处改过就回
// `schedule_conflict`。空串 = 不带锁（宿主按无条件更新处理）。
func (c *Client) ScheduleSetActive(ctx context.Context, id string, active bool, expectedUpdatedAt string) (ScheduleEditOutcome, error) {
	req := map[string]any{
		"id":     id,
		"action": "update",
		"patch":  map[string]any{"active": active},
	}
	if expectedUpdatedAt != "" {
		req["expectedUpdatedAt"] = expectedUpdatedAt
	}
	return c.scheduleEdit(ctx, req)
}

// ScheduleRun 用官方 `schedule.run` 立即执行一次（结果与 edit 同形）。
func (c *Client) ScheduleRun(ctx context.Context, id string) (ScheduleEditOutcome, error) {
	if strings.TrimSpace(id) == "" {
		return ScheduleEditOutcome{}, ErrScheduleNeedID
	}
	raw, err := c.scheduleRaw(ctx, "schedule.run", map[string]any{"id": id})
	if err != nil {
		return ScheduleEditOutcome{}, err
	}
	return decodeScheduleOutcome(raw)
}

// ScheduleHistory 读一条任务的投递历史（newest first）。
//
// limit 是官方必填（1..100）；before 是 messageId 排他游标（空 = 第一页）。
func (c *Client) ScheduleHistory(ctx context.Context, id string, limit int, before string) (ScheduleHistoryPage, string, error) {
	var page ScheduleHistoryPage
	if strings.TrimSpace(id) == "" {
		return page, "", ErrScheduleNeedID
	}
	if limit < 1 || limit > 100 {
		return page, "", errors.New("schedule.history 的 limit 必须在 1..100")
	}
	req := map[string]any{"id": id, "limit": limit}
	if before != "" {
		req["before"] = before
	}
	var out struct {
		ID                        string                  `json:"id"`
		Records                   []ScheduleHistoryRecord `json:"records"`
		EarlierRecordsUnavailable bool                    `json:"earlierRecordsUnavailable"`
		EarlierRecordsPruned      bool                    `json:"earlierRecordsPruned"`
		Retention                 ScheduleRetention       `json:"retention"`
		NextBefore                string                  `json:"nextBefore,omitempty"`
		Code                      string                  `json:"code"`
	}
	if err := c.Call(ctx, "schedule.history", req, &out); err != nil {
		return page, "", err
	}
	if out.Code != "" {
		return page, out.Code, nil
	}
	page = ScheduleHistoryPage{
		ID:                        out.ID,
		Records:                   out.Records,
		EarlierRecordsUnavailable: out.EarlierRecordsUnavailable,
		EarlierRecordsPruned:      out.EarlierRecordsPruned,
		Retention:                 out.Retention,
		NextBefore:                out.NextBefore,
	}
	if page.Records == nil {
		page.Records = []ScheduleHistoryRecord{}
	}
	if page.ID == "" {
		page.ID = id
	}
	return page, "", nil
}

// scheduleEdit 是两个 edit 动词的共用出口。
func (c *Client) scheduleEdit(ctx context.Context, req map[string]any) (ScheduleEditOutcome, error) {
	id, _ := req["id"].(string)
	if strings.TrimSpace(id) == "" {
		return ScheduleEditOutcome{}, ErrScheduleNeedID
	}
	raw, err := c.scheduleRaw(ctx, "schedule.edit", req)
	if err != nil {
		return ScheduleEditOutcome{}, err
	}
	return decodeScheduleOutcome(raw)
}

// scheduleRaw 调一个有 union 结果的 schedule 方法，把 `result` 原样交回。
//
// 为什么不用 `Call` 直接解结构体：这些方法的 result 是 7 分支 union
// （catalog 条目 / miss / deleted），先拿原始 JSON 再按键判别，比给每个分支
// 都造一个带 omitempty 的 Go 类型更不容易把"这份记录同时是 after 又是 cron"
// 这种形状错误放过去。
func (c *Client) scheduleRaw(ctx context.Context, method string, req map[string]any) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.Call(ctx, method, req, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// decodeScheduleOutcome 把官方 union 解成 ScheduleEditOutcome。
func decodeScheduleOutcome(raw json.RawMessage) (ScheduleEditOutcome, error) {
	var out ScheduleEditOutcome
	var probe struct {
		ID      string `json:"id"`
		Code    string `json:"code"`
		Deleted bool   `json:"deleted"`
		Kind    string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return out, err
	}
	switch {
	case probe.Code != "":
		out.Code = probe.Code
	case probe.Deleted:
		out.Deleted = true
	case probe.Kind != "":
		var entry ScheduleEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return out, err
		}
		out.Entry = &entry
	default:
		// 空对象是宿主"成功但没有载荷"的兜底（`handlers_schedule_official.go:465-470`）。
		// 不编条目、不报错——调用方按"这次没有新正文"处理。
	}
	return out, nil
}
