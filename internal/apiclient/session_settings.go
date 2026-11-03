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
