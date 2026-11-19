// uistate.go —— 宿主 additive 的 `uiState.get` / `uiState.set` / `uiState.delete` 客户端。
//
// 契约唯一来源：宿主实现 `../DSH-GO/internal/transport/handlers_ui_state.go`
// （三个 handler）与 `../DSH-GO/cmd/dshd/serve_ui_state.go`（落点
// `<home>/state/ui-state/<key>.json`）。
//
// # 它是什么
//
// 官方 GUI 把一批**纯前端记忆**放在 Electron 的 `Local Storage` 里：官方包里能
// 直接读到的两处是 `dsh-client-ui-sidebar-browser/lib/client.js:1695` 的
// `persist: "dsh.sidebar-browser.v1"` 与 `dsh-client-ui-user-questions/lib/client.js:439`
// 的 `persist: "dsh.user-questions.drafts.v1"`（都是 `defineStore` 的命名空间，
// 实际键由 `@deepseek-ai/dsh-client-store` 按会话补后缀）。本仓没有 Electron，
// 所以由宿主提供同语义的窄通道，键名由调用方**逐字**给。
//
// # 为什么不是 settings.mutate
//
// settings 文档有 revision 栅栏：界面每点一次"已读"都会推进 revision，用户这时
// 保存设置就会 revision_conflict。界面状态不是设置，也不该占设置的闸门。
//
// # 值的形状
//
// 原样 JSON（`json.RawMessage`）：宿主不解释、不改写，写进去什么读回来就是什么。
// 没存过是 `found=false`（**不是错误**）：官方 `localStorage.getItem` 也回 null。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
)

// UIStateGet 读一份界面状态（`uiState.get`）。
//
// 第二个返回值是"存过没有"：false 时 value 一定是 nil，调用方按"第一次用"处理，
// 不要当异常。四字段错误原样上抛（宿主未接线是 501 `not_implemented`）。
func (c *Client) UIStateGet(ctx context.Context, key string) (json.RawMessage, bool, error) {
	if key == "" {
		return nil, false, errors.New("uiState.get 需要非空 key")
	}
	var out struct {
		Found bool            `json:"found"`
		Value json.RawMessage `json:"value"`
	}
	if err := c.Call(ctx, "uiState.get", map[string]any{"key": key}, &out); err != nil {
		return nil, false, err
	}
	if !out.Found {
		return nil, false, nil
	}
	return out.Value, true, nil
}

// UIStateSet 整份覆盖一份界面状态（`uiState.set`）。
//
// value 必须是合法 JSON；空值与 `null` 宿主会 400（清状态用 UIStateDelete，
// 两条路各自一个语义）。这是 last-write-wins，没有版本比较——界面状态本来就是
// 单写者。
func (c *Client) UIStateSet(ctx context.Context, key string, value json.RawMessage) error {
	if key == "" {
		return errors.New("uiState.set 需要非空 key")
	}
	if len(value) == 0 {
		return errors.New("uiState.set 需要非空 value（清状态用 uiState.delete）")
	}
	return c.Call(ctx, "uiState.set", map[string]any{"key": key, "value": value}, nil)
}

// UIStateDelete 删掉一份界面状态（`uiState.delete`）。返回值表示删之前存不存在：
// false 表示本来就没有，不是错误。
func (c *Client) UIStateDelete(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, errors.New("uiState.delete 需要非空 key")
	}
	var out struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call(ctx, "uiState.delete", map[string]any{"key": key}, &out); err != nil {
		return false, err
	}
	return out.Deleted, nil
}
