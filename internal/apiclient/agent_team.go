// agent_team.go —— 官方 rc.2 `agentTeams/setEnabled` 的客户端（`dsh-agent-team`）。
//
// 官方方法面只有一个方法（rc.2 把 Team 从实验包转正：面板数据走 Session
// projection `agentTeam`，**没有** get/list 方法；唯一的写动作是这一个开关）。
//
// 契约（官方 typert 描述符 + 本仓宿主 DSH-GO `handlers_agent_team_rc2.go`）：
//
//	参数：agent（wire=agentId，source=lookup，**必须正好是树根会话**）、enabled（boolean）
//	结果：官方 `z.void()`；本仓 wire 用空对象 `{}`（出口不接受 result:null）
//	错误：非根 → 403 `team_lead_required`；持久化失败 → 500
//
// "只在根会话可用"是官方语义，不是本仓限制：子代理会话**不能**改 Team 模式。
// 界面据此在子会话里不渲染开关（结构树 §2.2 的 addressedChild/listedChild 守卫）。
package apiclient

import "context"

// TeamLeadRequiredCode 是"Team 模式只能从树根会话修改"的稳定业务码。
//
// 宿主回 403 + 这个 code。界面据此提示"去根会话改"，而不是显示"服务器错误"。
const TeamLeadRequiredCode = "team_lead_required"

// SetAgentTeamEnabled 打开/关闭一个会话的 Team 模式（官方 agentTeams/setEnabled）。
//
// agentID 必须是**树根会话**的 id；非根会拿到 `team_lead_required`（*APIError）。
// 宿主在 await 持久化**之前**就让选择生效，所以返回 nil = 选择已落盘。
func (c *Client) SetAgentTeamEnabled(ctx context.Context, agentID string, enabled bool) error {
	params := map[string]any{"agentId": agentID, "enabled": enabled}
	return c.Call(ctx, "agentTeams.setEnabled", params, nil)
}
