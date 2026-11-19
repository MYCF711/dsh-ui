// credentials.go —— 官方凭据域 `credentials.describe` / `credentials.set` 的客户端。
//
// 契约唯一来源：官方 rc.2 的 typert 契约（`dsh-api-remotes` 的 credentials 命名空间）
// 与宿主实现 `../DSH-GO/internal/transport/handlers_settings_official.go`
// （文件头逐条记了官方行号：describe L154 / set L186）。
//
// # 为什么凭据不走 settings.mutate
//
// 官方把 `apiKey` 这类字段标成 `role: 'secret'`：值**从不上线**，读取路径只回
// "设没设"。写入也走另一条服务（credentials 域），因为它的存储与设置文档是
// 两个库、两套拒绝语义（设置文档只读时凭据仍可写，反之亦然）。所以界面里
// 密钥控件必须调这一路，**不能**把它当成 `web-search-deepseek.apiKey` 的普通字段。
//
// # 三种"不知道"
//
// `credentials.describe` 的答案是逐 ref 的：**没问到的 ref 不是"未配置"**。
// 本客户端因此返回 map，调用方只有拿到 key 才许说"未配置"；拿不到就画官方那句
// 中性话（见 `settings_items_web.go` 的 `zhWebKeyUnknown`）。
package apiclient

import "context"

// CredentialInfo 是 `credentials.describe` 对一个 ref 的回答（官方 `CredentialInfo`）。
//
// **没有值字段**：官方的读取路径就没有它，宿主也不加（`OfficialCredentialInfo`）。
type CredentialInfo struct {
	// Configured 表示当前有任何一层（存储 / 环境）供得上值。
	Configured bool `json:"configured"`
	// Source 是来源类型：`catalog`（本地凭据存储）/ `env`（同名环境变量回退）。
	Source string `json:"source,omitempty"`
	// Writable 表示 `credentials.set` 能不能影响它；false 时界面必须禁用控件。
	Writable bool `json:"writable"`
	// Masked 是 provider 生成的**掩码预览**（官方 `CredentialInfo.masked?: string`，
	// `@deepseek-ai/dsh-credentials/src/types.ts:62-75`："never the full value"）。
	//
	// 只在本地凭据存储（本仓 `source == "catalog"`，官方 `source: 'file'`）时有值：
	// env / dotenv 层官方 describe 就不回。界面拿它当已存密钥的 placeholder
	// （官方 ApiKeyInput.tsx:41 的 `saved.masked`）——**明文永远不上线**。
	Masked string `json:"masked,omitempty"`
}

// CredentialDescribe 问一批 ref 的凭据状态（官方 `credentials.describe`）。
//
// 官方参数是 `refs: string[]`，上限 64 条、每条必须匹配 `^[A-Za-z_][A-Za-z0-9_]*$`
// （宿主同一道 grammar guard）；本仓只问一两条，不在这里做客户端侧校验——
// 名字来自设置里的 `apiKeyEnv`，宿主拒绝时按它的四字段报错，界面照画。
func (c *Client) CredentialDescribe(ctx context.Context, refs []string) (map[string]CredentialInfo, error) {
	params := map[string]any{"refs": refs}
	var out map[string]CredentialInfo
	if err := c.Call(ctx, "credentials.describe", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CredentialUnset 删一个凭据引用（官方 `credentials.unset`，结果 void，幂等）。
//
// 官方调用点：`OFF:operations.ts:82-85` 的 `removeCredential` → `credentials.unset(ref)`。
// 它是**幂等**的（删一个本来就没有的引用也算成功），所以界面重试删除不需要先 describe。
//
// 语义边界与 set 对称但破坏性相反：
//   - 删掉的是**本地凭据库**那一层；环境变量 / dotenv 层不受影响，删完以后可能
//     又"回退到 env"，所以界面不能把删成功当成"现在肯定没有密钥"；
//   - 官方在 Apply 里把这一步排在**设置写之前**（ProviderEditor.tsx:350-356），
//     理由写在官方注释里：设置写失败还可以重试，而删掉的密钥**撤不回来**，
//     所以它必须先做、并且做完就地记成"已删"。
func (c *Client) CredentialUnset(ctx context.Context, ref string) error {
	params := map[string]any{"ref": ref}
	// 宿主成功信封是 `{}`（官方结果是 void），解成 struct{} 即可。
	var out struct{}
	return c.Call(ctx, "credentials.unset", params, &out)
}

// CredentialSet 写一个凭据引用（官方 `credentials.set`，结果 void）。
//
// 值是**服务器可信**的明文：调用方必须已经明确拿到用户输入的密钥，且不能把
// 它写进日志/错误串（宿主侧的错误也按这条约束回，只提 ref）。
// 空串/纯空白由宿主拒绝（官方 `min(1)`；既有存储会 trim 后当作删除），
// 所以调用方在发之前空串根本不发（见 `settings_items_web.go` 的 plan）。
func (c *Client) CredentialSet(ctx context.Context, ref, value string) error {
	params := map[string]any{"ref": ref, "value": value}
	// 宿主成功信封是 `{}`（官方结果是 void），解成 struct{} 即可。
	var out struct{}
	return c.Call(ctx, "credentials.set", params, &out)
}
