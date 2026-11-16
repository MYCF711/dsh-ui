// account.go —— 官方账号状态、资料、余额与登录动作的 Transport 客户端。
//
// 所有请求都只打到宿主 /api；Platform 凭据不进入 GUI。响应形状逐字对应
// DSH-GO internal/transport/handlers_account_official.go：
//   - getState/start/cancel/signOut → 平铺 AccountView；
//   - getProfile/getBalance → {profile|balance: T|null}（本仓成功信封不能是 null）；
//   - hasRunningAccountTasks → 裸 boolean。
package apiclient

import "context"

// AccountCredentialStored 是官方 AccountView.status 的“已登录”取值。
const AccountCredentialStored = "credential-stored"

// AccountLinks 是账号状态自带的 Platform 目的地。
type AccountLinks struct {
	UsageURL string `json:"usageUrl"`
	TopUpURL string `json:"topUpUrl"`
}

// AccountAttempt 是一次浏览器登录尝试；Phase 原样保留官方状态机值。
type AccountAttempt struct {
	ID           string `json:"id"`
	Phase        string `json:"phase"`
	AuthorizeURL string `json:"authorizeUrl,omitempty"`
	UserCode     string `json:"userCode,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
}

// AccountView 是官方安全账号投影。
type AccountView struct {
	Status  string          `json:"status"`
	Links   AccountLinks    `json:"links"`
	Attempt *AccountAttempt `json:"attempt"`
	Source  string          `json:"source,omitempty"`
}

// SignedIn 只有 credential-stored 算已登录；未知/空串绝不能冒充真值。
func (v AccountView) SignedIn() bool { return v.Status == AccountCredentialStored }

// ActiveSignIn 对应官方 AccountSection 的 active 四相位。
func (v AccountView) ActiveSignIn() bool {
	if v.Attempt == nil {
		return false
	}
	switch v.Attempt.Phase {
	case "initializing", "waiting-browser", "exchanging", "committing":
		return true
	default:
		return false
	}
}

// AccountProfileValue 是 ready 资料；指针区分 Platform null 与真实字符串。
type AccountProfileValue struct {
	ID        *string `json:"id"`
	Name      *string `json:"name"`
	Contact   *string `json:"contact"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}

// AccountProfile 是 {status:"ready",value} | {status:"failed"}。
type AccountProfile struct {
	Status string               `json:"status"`
	Value  *AccountProfileValue `json:"value,omitempty"`
	Source string               `json:"source,omitempty"`
}

// AccountWallet 的金额保持字符串，绝不经 float64 损失精度。
type AccountWallet struct {
	Currency string `json:"currency"`
	Balance  string `json:"balance"`
}

// AccountBalance 含充值钱包与赠送钱包。
type AccountBalance struct {
	Status       string          `json:"status"`
	Value        []AccountWallet `json:"value"`
	BonusWallets []AccountWallet `json:"bonusWallets"`
	Source       string          `json:"source,omitempty"`
}

// AccountClientMetadata 与官方 AccountClientMetadata 三必填字段一致。
type AccountClientMetadata = NotificationClientMetadata

// AccountSnapshot 是壳发布给账号页/账号菜单的一帧不可变快照。
// nil View = 状态未知；Profile/Balance 的独立失败位不把查询失败伪装成空资料或 0 余额。
type AccountSnapshot struct {
	View          *AccountView
	Profile       *AccountProfile
	Balance       *AccountBalance
	Known         bool
	Failed        bool
	ProfileFailed bool
	BalanceFailed bool
	LoginVisible  bool
	LoginFailed   bool
}

// GetAccountState 取当前账号状态机投影。
func (c *Client) GetAccountState(ctx context.Context) (*AccountView, error) {
	var out AccountView
	if err := c.Call(ctx, "account.getState", struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAccountProfile 读取资料；nil 表示宿主明确回 profile:null。
func (c *Client) GetAccountProfile(ctx context.Context, meta AccountClientMetadata) (*AccountProfile, error) {
	var out struct {
		Profile *AccountProfile `json:"profile"`
	}
	if err := c.Call(ctx, "account.getProfile", map[string]any{"client": meta}, &out); err != nil {
		return nil, err
	}
	return out.Profile, nil
}

// GetAccountBalance 读取充值/赠送余额；nil 表示宿主明确回 balance:null。
func (c *Client) GetAccountBalance(ctx context.Context, meta AccountClientMetadata) (*AccountBalance, error) {
	var out struct {
		Balance *AccountBalance `json:"balance"`
	}
	if err := c.Call(ctx, "account.getBalance", map[string]any{"client": meta}, &out); err != nil {
		return nil, err
	}
	return out.Balance, nil
}

// StartAccountSignIn 创建一次登录尝试。
func (c *Client) StartAccountSignIn(ctx context.Context, meta AccountClientMetadata, callbackOrigin, loginSource string) (*AccountView, error) {
	var out AccountView
	params := map[string]any{"client": meta, "callbackOrigin": callbackOrigin, "loginSource": loginSource}
	if err := c.Call(ctx, "account.startSignIn", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelAccountSignIn 取消指定尝试；attemptId 必须原样来自 AccountView。
func (c *Client) CancelAccountSignIn(ctx context.Context, attemptID string) (*AccountView, error) {
	var out AccountView
	if err := c.Call(ctx, "account.cancelSignIn", map[string]any{"attemptId": attemptID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SignOutAccount 移除本地账号授权（不是删除 API key）。
func (c *Client) SignOutAccount(ctx context.Context, meta AccountClientMetadata) (*AccountView, error) {
	var out AccountView
	if err := c.Call(ctx, "account.signOut", map[string]any{"client": meta}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HasRunningAccountTasks 返回官方裸 boolean，不包 {running:…}。
func (c *Client) HasRunningAccountTasks(ctx context.Context) (bool, error) {
	var running bool
	if err := c.Call(ctx, "account.hasRunningAccountTasks", struct{}{}, &running); err != nil {
		return false, err
	}
	return running, nil
}
