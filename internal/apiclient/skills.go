// skills.go — 官方 `skills.list` 的客户端（`dsh-api-session-controller` 的
// `SessionSkillCatalog`，namespace `skills`）。
//
// 契约（官方 `skill-catalog.ts` + 本仓宿主 DSH-GO `cmd/dshd/serve_agent_api_skills.go`）：
//
//	参数：sessionId（lookup）——**按会话寻址**，因为目录随该会话的 cwd / preset 变
//	结果：{ skills: SkillEntry[] }，**只含 user-invocable 的**，且**不含正文**
//	错误：会话查不到 -> 404 `not_found` + `details.sessionId`；其它 -> 500 `internal`
//
// 官方两条语义值得写在这里，免得调用方猜：
//   - 官方注释 "without loading skill bodies" / "cold-readable"：返回的**只是元数据**，
//     拿它不会把 SKILL.md 正文读进来。
//   - **空目录不是错误**：真的一个 skill 都没有时返回 `skills: []`（宿主侧已断言）。
package apiclient

import "context"

// SkillEntry 是官方 `SkillEntry`（`dsh-api-session-controller/src/types.ts`）。
//
// 字段名与可选性逐字照官方：`path` / `whenToUse` 是**可选**（`omitempty`），
// 宿主不给就是没有——本仓不往空串上凑。
type SkillEntry struct {
	// Path 是 SKILL.md 的绝对路径（官方：文件系统 provider 给的时候才有）。
	Path string `json:"path,omitempty"`
	// Name 是 kebab-case 标识符，用户以 `/name` 引用。
	Name string `json:"name"`
	// Description 是短路由描述。
	Description string `json:"description"`
	// WhenToUse 是可选补充路由指引（官方：frontmatter 写了才有）。
	WhenToUse string `json:"whenToUse,omitempty"`
	// ModelInvocable 报告同一个 skill 是否也向模型宣传。
	ModelInvocable bool `json:"modelInvocable"`
}

// SkillsListResult 是官方 `SkillListValue`。
type SkillsListResult struct {
	// Skills 是用户可调用的 skill 元数据（不含正文）。
	Skills []SkillEntry `json:"skills"`
}

// SkillsList 列出某个会话可见的、用户可调用的 skill 目录（官方 `skills.list`）。
//
// sessionID 必填：目录按该会话的 cwd 与 agent preset 选视图，所以不能全局问一次。
// 宿主对不存在的会话回 404 `not_found`（*APIError），**不是**空数组。
func (c *Client) SkillsList(ctx context.Context, sessionID string) (SkillsListResult, error) {
	var out SkillsListResult
	params := map[string]any{"sessionId": sessionID}
	if err := c.Call(ctx, "skills.list", params, &out); err != nil {
		return SkillsListResult{}, err
	}
	return out, nil
}

// ResolveSkillPath 按官方 `openSkill` 的语义把 `/name` 引用解析成绝对路径。
//
// 官方原文（`dsh-client-ui-skill/src/client/index.ts`）：
//
//	catalog.find(skill => '/' + skill.name === ref)?.path
//
// 即**整串精确匹配** `/name`，找不到就返回空串（官方是 `undefined`，
// 调用方据此不画 inspect 按钮）。刻意不做模糊匹配、不去大小写、不剥后缀——
// 那是官方没有的行为，加了就是自造。
func ResolveSkillPath(skills []SkillEntry, ref string) string {
	for _, skill := range skills {
		if "/"+skill.Name == ref {
			return skill.Path
		}
	}
	return ""
}
