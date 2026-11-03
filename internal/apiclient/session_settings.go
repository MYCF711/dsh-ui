package apiclient

import "context"

// SetSessionModel 设一个会话"下一轮用哪个模型"的覆盖。
//
// **空串 = 清除覆盖**（回到宿主默认）。宿主把它存进
// `<home>/state/session-model.json`，不写 v4——那是界面状态，不是会话内容。
func (c *Client) SetSessionModel(ctx context.Context, sessionID, model string) error {
	params := map[string]any{"id": sessionID}
	if model != "" {
		params["model"] = model
	}
	return c.Call(ctx, "session.setModel", params, nil)
}

// SetSessionPermission 设一个会话的审批策略覆盖（deny/ask/allow/auto）。
//
// **空串 = 清除覆盖**。宿主会白名单校验——传一个没定义的值会 400 并带回
// 「只能是 deny/ask/allow/auto」，不要自己猜。
func (c *Client) SetSessionPermission(ctx context.Context, sessionID, perm string) error {
	params := map[string]any{"id": sessionID}
	if perm != "" {
		params["permission"] = perm
	}
	return c.Call(ctx, "session.setPermission", params, nil)
}

// ModelEntry 是 `models.list` 的一行。
type ModelEntry struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// ModelsList 问宿主有哪些模型可用。
//
// **宿主转发、界面不直连**：网关地址与凭据只在宿主那一侧（R7）。
// 失败时宿主回 502 并带原文——**不会回空列表**，
// 因为空列表在界面上等于"没有可用模型"，而真实原因可能是网关没起。
func (c *Client) ModelsList(ctx context.Context) ([]ModelEntry, error) {
	var out struct {
		Models []ModelEntry `json:"models"`
	}
	if err := c.Call(ctx, "models.list", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out.Models, nil
}
