package apiclient

// session_create.go —— 官方 `session.create` 的客户端（基线 0.2.2-rc.2）。
//
// # 官方原件（rc.2 解包树，逐条给 file:line）
//
//	@deepseek-ai/dsh-api-session-controller/src/types.ts:268-284
//	  SessionCreateRequest { workspaceId?; cwd?; sessionId?; agentPreset? }
//	  SessionCreateValue   { sessionId; agentPreset? }
//	  —— `workspaceId` 与 `cwd` 的注释就是互斥关系（"exclusive with `cwd`"）；
//	     `agentPreset` 是**可选键**，宿主不知道时不出现，不回空串。
//	.../src/client/sessions/manager.ts:484-520（create）
//	  组 payload 的规则：`workspaceId !== undefined` 时**连 cwd 都不发**，
//	  否才发 `cwd`；`sessionId`/`agentPreset` 与两者并列（undefined 就不发）。
//	.../src/client/sessions/manager.ts:855-857（workspaceAttachSessionId）
//	  `error.code === 'session/workspace-attach-failed' ? error.details.sessionId : undefined`
//	  —— 那份会话**已经发布**（建出来了）但没挂上工作区，客户端要把它当 Ungrouped 暴露。
//	.../src/types.ts:199
//	  `'session/workspace-attach-failed': { sessionId, workspaceId }`
//
// # 一处有意的兼容（不是自造字段）
//
// 官方**没有** `mode`。但本仓已发布的宿主（DSH-GO `origin/main`）`session.create`
// 只解 `mode`（`HOST:internal/transport/server.go` 的 `handleSessionCreate`），
// 宿主侧 PR #63 也把 `Mode` 作为**旧名字**整段保留（"官方字段优先，其次 mode"，
// `HOST:docs/transport-contract.md` §18）。所以：点名的 preset **两个名字都发**
// （`agentPreset` + `mode` = 同一个值）——老宿主按 mode 拿到 preset，新宿主按官方
// 字段拿到同一个值，两边都不丢能力。**这是能力下限，不是新语义**。
//
// # 为什么不发 `title`
//
// 官方 `SessionCreateRequest` 没有 `title`（types.ts:268-277 四个字段就是全部）；
// 宿主从 `origin/main` 到 PR #63 都**没有**解过它（旧 handler 只解 `mode`，
// 新 handler 解 `mode` + 四个官方字段）。客户端原来发的 `title` 是空转字段，
// 本轮删掉；标题由宿主按官方 `session.title` 自己的规则生成。

import (
	"context"
	"errors"
	"strings"
)

// OfficialCodeWorkspaceAttachFailed 是官方"已发布但没挂上工作区"的错误码
// （`.../src/types.ts:199`、`.../src/commands.ts:146-157`）。
const OfficialCodeWorkspaceAttachFailed = "session/workspace-attach-failed"

// SessionCreateRequest 是官方 `session.create` 的裸参数（types.ts:268-277）。
//
// 零值 = 一个字段都不发（官方"都没给"），宿主按 `defaultCwd` / 配置里的默认 preset
// 建会话——这与官方客户端省略字段的语义一致，不要改写成"发空串"。
type SessionCreateRequest struct {
	// WorkspaceID 是官方 `workspaceId`：会话加入的工作区。**优先于 Cwd**
	// （manager.ts:498-504：非空时连 cwd 都不发）。与 Cwd 同时给是宿主侧的
	// `gateway/bad-request`（commands.ts:121-123），本客户端自己就不发出去。
	WorkspaceID string
	// Cwd 是官方 `cwd`：没有工作区时会话自己拥有的目录。WorkspaceID 非空时被忽略。
	Cwd string
	// SessionID 是官方 `sessionId`：复用这个空白会话；这个名字下没存过就用它建新会话。
	// 空 = 不发（宿主铸新 id）。
	SessionID string
	// AgentPreset 是官方 `agentPreset`：会话运行的 preset。空 = 不发（宿主用默认）。
	AgentPreset string
}

// SessionCreateResult 是官方 `SessionCreateValue`（types.ts:279-284）。
type SessionCreateResult struct {
	// SessionID 是建出来或复用到的会话。**可能与请求里的 SessionID 不同**
	// （"may differ from the requested `sessionId`"），调用方只能用这个值。
	SessionID string
	// AgentPreset 是宿主如实回的 preset；官方是可选键，宿主不知道时这里是空串
	// （不是"叫空串的 preset"），界面据此整段不显示，不要拿它当名字用。
	AgentPreset string
}

// params 按官方组合规则把请求摊成裸参数（manager.ts:498-504）。
//
// 单独抽出来是为了能被测试直接钉住：**不需要起 HTTP 服务**就能证明
// "workspaceId 给了就连 cwd 都不发"这条官方规则。
func (r SessionCreateRequest) params() map[string]any {
	params := map[string]any{}
	if r.SessionID != "" {
		params["sessionId"] = r.SessionID
	}
	if r.AgentPreset != "" {
		params["agentPreset"] = r.AgentPreset
		// 旧名字（header.agentPreset 的旧称）：见文件头的兼容说明。
		params["mode"] = r.AgentPreset
	}
	if r.WorkspaceID != "" {
		params["workspaceId"] = r.WorkspaceID
		return params
	}
	if r.Cwd != "" {
		params["cwd"] = r.Cwd
	}
	return params
}

// SessionCreate 是官方 `session.create`。
//
// 结果能解码出来的 `sessionId` 为空串时**不当成功**：拿 "" 去 session.load 只会
// 变成"会话不存在"，现场全丢。宿主那边也有对应判据（PR #63 的 handler 对空
// sessionId 回 500），两边都不许把这个错误咽下去。
func (c *Client) SessionCreate(ctx context.Context, req SessionCreateRequest) (SessionCreateResult, error) {
	var out struct {
		SessionID   string `json:"sessionId"`
		AgentPreset string `json:"agentPreset"`
	}
	if err := c.Call(ctx, "session.create", req.params(), &out); err != nil {
		return SessionCreateResult{}, err
	}
	if strings.TrimSpace(out.SessionID) == "" {
		return SessionCreateResult{}, errors.New("宿主 session.create 回了一个空 sessionId")
	}
	return SessionCreateResult{SessionID: out.SessionID, AgentPreset: out.AgentPreset}, nil
}

// OfficialCode 报告这次失败对应的**官方错误码**（`domain/reason` 形状）。
//
// 两个来源都要认：
//
//   - 官方线：`error.code` 本身就是官方码（`RemoteFailure.code`）；
//   - 本仓宿主：四字段 `code` 是闭合集合（`bad_request` / `internal` / …），
//     官方码按 PR #63 的约定留在 **message 前缀**里（`session/workspace-attach-failed: …`，
//     `HOST:docs/transport-contract.md` §18）。
//
// 只在前缀里**含 `/`** 时才认它当码：普通 message 里不会长这样，误判面很小。
func (e *APIError) OfficialCode() string {
	if e == nil {
		return ""
	}
	if strings.Contains(e.Code, "/") {
		return e.Code
	}
	if i := strings.Index(e.Message, ":"); i > 0 {
		if head := strings.TrimSpace(e.Message[:i]); strings.Contains(head, "/") {
			return head
		}
	}
	return e.Code
}

// SessionCreateID 是"只要会话 id"的便利封装（等价于 SessionCreate 后取
// 结果里的 SessionID）。
//
// 单独留一个是因为绝大多数调用点（新建会话、真机夹具）只关心打开哪个会话，
// 不在乎宿主回显的 preset；但它们**仍然要拿到官方结果里的那个 id 本尊**——
// 官方 `SessionCreateValue.sessionId` 可能与请求的 sessionId 不同。
func (c *Client) SessionCreateID(ctx context.Context, req SessionCreateRequest) (string, error) {
	res, err := c.SessionCreate(ctx, req)
	if err != nil {
		return "", err
	}
	return res.SessionID, nil
}

// AttachFailedSessionID 报告"已发布但没挂上工作区"的那份会话 id
// （manager.ts:855-857 的 `workspaceAttachSessionId`）。
//
// 只有 `code === 'session/workspace-attach-failed'` 时才取 `details.sessionId`；
// 别的错误即使带了 details.sessionId 也不算——那是另一件事的字段。
func (e *APIError) AttachFailedSessionID() (string, bool) {
	if e == nil || e.OfficialCode() != OfficialCodeWorkspaceAttachFailed {
		return "", false
	}
	id, _ := e.Details["sessionId"].(string)
	if strings.TrimSpace(id) == "" {
		return "", false
	}
	return id, true
}

// AttachFailedSessionID 是上面那个方法对任意 error 的版本（不是 *APIError 也能问）。
//
// 官方注释把这件事说得很清楚：发布先于挂载，所以错误体里的 id 是一份**真实存在**的
// 会话；调用方应当立刻把它当 Ungrouped 暴露，同时保留用户已经输入的内容，
// 由用户决定要不要重试挂载。
func AttachFailedSessionID(err error) (string, bool) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return "", false
	}
	return apiErr.AttachFailedSessionID()
}
