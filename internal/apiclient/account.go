// account.go —— 账号状态（官方 `account/getState` 的客户端）。
//
// # 为什么界面需要它
//
// 官方 rc.2 的账号通知入口由**父级**（账号菜单）决定"何时可见"：
//
//	active = signedIn && !onboarding（AccountMenu.tsx:60 传给 settings.launcher.notifications）
//	signedIn = view?.status === 'credential-stored'（src/client/index.ts:44-48）
//
// 也就是：**没有账号状态就没有通知**。本仓此前没有这条数据面，所以
// `plugins/dsnotifications` 的控制器只能停在"未授权"空态（整块不渲染）。
// 这个文件补的是那条最小数据面：一条 unary 查询，不引流、不引轮询协议。
//
// # 契约（宿主侧逐字核对）
//
//	方法   account.getState（`OfficialAccountAPI.AccountGetState`）
//	入参   { }（宿主 p.finish() 只做完结校验，不读任何字段）
//	结果   **平铺的 AccountView**（不是 {state:…}）：
//	       handlers_account_official.go:929 `return api.AccountGetState(ctx)`
//	       → OfficialAccountView{status, links, attempt, source?}
//
// `links` / `attempt` 界面这一轮不用（账号面板还没落地），所以**不建模**：
// 多解码两个我们用不上的字段，等于给"以后"留一个没人验过的形状。
package apiclient

import "context"

// AccountCredentialStored 是官方 `AccountView.status` 的"已登录"取值。
//
// 官方 `status` 是闭集两值：`credential-stored` | `signed-out`
// （handlers_account_official.go:134 的枚举注释）。界面只认这一个，
// 其余（含空串）一律按"未登录"——空串是"宿主没给"，不是"登录了"。
const AccountCredentialStored = "credential-stored"

// AccountView 是官方 `AccountView` 的安全账号投影（本仓只取用得上的两个字段）。
type AccountView struct {
	// Status 是 `credential-stored` / `signed-out`；空 = 宿主没给。
	Status string `json:"status"`
	// Source 是本仓额外字段（官方 schema additionalProperties:true），
	// 标明这条状态来自哪种账号提供方（local / official / oidc）。
	Source string `json:"source,omitempty"`
}

// SignedIn 按官方判据判断是否已登录：只有 `credential-stored` 算。
//
// 单独一个方法而不是让调用方比字符串：判据只有一处，别到处写 `== "credential-stored"`。
func (v AccountView) SignedIn() bool { return v.Status == AccountCredentialStored }

// GetAccountState 取账号状态机的当前投影（官方 account/getState）。
//
// 宿主三种真实回答，界面必须分别对待（都不许编）：
//
//	(status, nil)   宿主有账号状态机：Status 是**真值**
//	(err = nil, 但不是上面那种)  不会发生——本方法只在成功时返回 view
//	(not_implemented)  宿主没有账号状态机 → 界面按"未知"处理（隐藏，不画成未登录）
//
// 因此调用方要区分"查询失败"与"确定未登录"：失败时**不要**把状态写成
// `signed-out`，那会把"不知道"说成"没登录"。
func (c *Client) GetAccountState(ctx context.Context) (*AccountView, error) {
	var out AccountView
	if err := c.Call(ctx, "account.getState", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
