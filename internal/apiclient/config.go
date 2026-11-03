// config.go —— 宿主 `config.get` / `config.set` 的客户端。
//
// 它读写的是宿主的 `<home>/state/ui-config.json`：**纯 key/value 存储，宿主不解释**。
// 所以界面对"有哪些键、每个键什么含义"负全责——写进去一个错键不会报错，只是没人读。
//
// 现状：目前只有 `ui.sidebarWidth` / `ui.rightbarWidth` 两个键真被 `dsshell` 读
// （见 plugins/dsshell 的 UserSidebarWidth / UserRightbarWidth）。
package apiclient

import "context"

// ConfigGet 取全部 UI 配置。宿主没接过这两个方法时（旧的 mockd）会返回空 map 而不是报错
// ——"没有配置"和"读失败"对界面是同一件事：都用默认值。
func (c *Client) ConfigGet(ctx context.Context) (map[string]string, error) {
	// **字段名是 `values` 不是 `config`**：宿主 `handleConfigGet` 回的是
	// `{"result":{"values":{...}}}`。我第一版按 `config` 解，永远拿到空 map，
	// 于是"配置读回来了"这件事在真机上从来没发生过（单测也只测了空 map 不 panic）。
	var out struct {
		Values map[string]string `json:"values"`
	}
	if err := c.Call(ctx, "config.get", struct{}{}, &out); err != nil {
		return nil, err
	}
	if out.Values == nil {
		out.Values = map[string]string{}
	}
	return out.Values, nil
}

// ConfigSet 写若干键（**合并**语义：宿主只覆盖你给的键，没给的保持原样）。
func (c *Client) ConfigSet(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	// 同理：宿主要的是 `{"values":{...}}`，裸 map 会被当成"缺 values"直接 400。
	// 实测错误原文：`缺 values：要给至少一项`。
	return c.Call(ctx, "config.set", map[string]any{"values": kv}, nil)
}
