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
	var out struct {
		Config map[string]string `json:"config"`
	}
	if err := c.Call(ctx, "config.get", struct{}{}, &out); err != nil {
		return nil, err
	}
	if out.Config == nil {
		out.Config = map[string]string{}
	}
	return out.Config, nil
}

// ConfigSet 写若干键（**合并**语义：宿主只覆盖你给的键，没给的保持原样）。
func (c *Client) ConfigSet(ctx context.Context, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	return c.Call(ctx, "config.set", kv, nil)
}
