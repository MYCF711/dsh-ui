// notifications.go —— 官方 rc.2 账号通知三件套的客户端（`dsh-deepseek-account`）。
//
// 官方契约（rc.2 随包源码 + 本仓宿主 DSH-GO 的 L1 实现）：
//
//	account/getNotifications        参数 {client} → 结果 {batch: AccountNotificationBatch | null}
//	account/getNotificationContent  参数 {id, client} → 结果 {content: AccountNotification | null}
//	account/reportNotificationEvent 参数 {id, event, client} → 结果**裸 boolean**
//
// `client` 是官方的 `AccountClientMetadata`（version / locale / timezoneOffsetSeconds
// 三个必填），宿主缺它会直接回 invalid_arg——不是可选项。
//
// # 为什么 null 必须保留成 nil
//
// 官方把"未登录 / 查询中授权变了"定义为 **null**，不是空批次：
//
//	AccountNotificationBatch | null
//
// 所以这里 `*NotificationBatch` / `*Notification` 用指针。把 null 解成零值批次
// 会让界面显示"没有通知"，而真实含义是"现在不知道 / 不该显示"——这正是官方
// 结构树 §5.1 里"整块不渲染"的那条分支。
//
// 数据来源说明（R7：界面不直连平台）：本文件的三个方法都打到**宿主** /api，
// 宿主再去官方 Platform 取；凭据只在宿主那一侧。
package apiclient

import (
	"context"
	"encoding/json"
)

// NotificationClientMetadata 是官方 `AccountClientMetadata`。
//
// 三个字段官方都标了 required；宿主 `officialAccountClient` 缺 client 直接
// invalid_arg，缺字段也报错，所以这里不给 omitempty（宁可发显式零值）。
type NotificationClientMetadata struct {
	Version               string  `json:"version"`
	Locale                string  `json:"locale"`
	TimezoneOffsetSeconds float64 `json:"timezoneOffsetSeconds"`
}

// NotificationEvent 是官方 `AccountNotificationEvent` 的三个合法取值。
const (
	NotificationEventDisplayed  = "displayed"
	NotificationEventCTAClicked = "cta_clicked"
	NotificationEventDismissed  = "dismissed"
)

// NotificationTarget 是动作目的地（官方的闭集 union）：
//
//	{type:"app_route", name:"account"}  → 打开账号设置，卡片保留
//	{type:"dismiss"}                   → 关闭这条通知
//
// 用结构体而不是 map：官方 `assertNever` 对未知 target 抛错，界面上我们也
// 只认这两条；多出来的字段留给服务端，界面不解析（原则：文案与动作全部来自服务端）。
type NotificationTarget struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// NotificationAction 是通知上的一个动作。
//
// Style 只有 `primary` / `secondary` 两档（官方闭集），界面按它选按钮变体。
type NotificationAction struct {
	Label  string             `json:"label"`
	Style  string             `json:"style"`
	Target NotificationTarget `json:"target"`
}

// Notification 是一条**已本地化**的账号通知（官方 `AccountNotification`）。
//
// DisplayUntil 是 `string | null`，所以必须用指针：nil = 永不过期（官方
// describe 明写），空串是"字段给了但解析不出"，两者语义不同。
type Notification struct {
	ID           string               `json:"id"`
	CreatedAt    string               `json:"createdAt"`
	DisplayUntil *string              `json:"displayUntil"`
	Title        string               `json:"title"`
	Body         string               `json:"body"`
	Actions      []NotificationAction `json:"actions"`
	// Data 是可选业务数据；界面不解释它，原样保留。
	Data json.RawMessage `json:"data,omitempty"`
}

// NotificationBatch 是官方 `AccountNotificationBatch`。
//
// NextPollAfterSeconds 是**正数秒**：下一轮该等多久由服务端给，界面不许自选
// 一个"看起来合理"的常数（官方 config 的 600s 只是首轮默认）。
type NotificationBatch struct {
	NextPollAfterSeconds float64        `json:"nextPollAfterSeconds"`
	Notifications        []Notification `json:"notifications"`
}

// GetNotifications 取通知列表（官方 account/getNotifications）。
//
// 返回 (nil, nil) = 宿主回了 **null**：未登录或查询中授权变了。**不是空批次**。
func (c *Client) GetNotifications(ctx context.Context, meta NotificationClientMetadata) (*NotificationBatch, error) {
	var out struct {
		Batch *NotificationBatch `json:"batch"`
	}
	if err := c.Call(ctx, "account.getNotifications", map[string]any{"client": meta}, &out); err != nil {
		return nil, err
	}
	return out.Batch, nil
}

// GetNotificationContent 取一条通知的（可能已按当前语言刷新的）内容。
//
// 返回 (nil, nil) = 宿主回 null（官方：那条已经不在 / 语言刷新期间被丢掉）。
func (c *Client) GetNotificationContent(ctx context.Context, id string, meta NotificationClientMetadata) (*Notification, error) {
	var out struct {
		Content *Notification `json:"content"`
	}
	params := map[string]any{"id": id, "client": meta}
	if err := c.Call(ctx, "account.getNotificationContent", params, &out); err != nil {
		return nil, err
	}
	return out.Content, nil
}

// ReportNotificationEvent 上报一次交互（官方 account/reportNotificationEvent）。
//
// 结果是**裸 boolean**：true = 平台接受了这次上报。界面不为它做重试或回滚
// （官方 controller 失败只 console.error，不改 UI）。
func (c *Client) ReportNotificationEvent(ctx context.Context, id, event string, meta NotificationClientMetadata) (bool, error) {
	var accepted bool
	params := map[string]any{"id": id, "event": event, "client": meta}
	if err := c.Call(ctx, "account.reportNotificationEvent", params, &accepted); err != nil {
		return false, err
	}
	return accepted, nil
}
