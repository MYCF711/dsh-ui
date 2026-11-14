// references.go —— `@` 引用 / `/` 技能候选 / 消息反馈的**插件侧只读契约**。
//
// 这些类型只描述插件能看到的投影，不复制宿主 wire 的字段名（wire 在
// internal/apiclient 里）。插件拿不到 *apiclient.Client，也不发请求：候选与
// 反馈缓存由壳持有、按需后台拉，插件只读这一层。
package pluginapi

// FileReferenceCandidate 是 `@` 文件候选的一行。
type FileReferenceCandidate struct {
	// Path 是相对会话工作目录的路径（官方 fileReferences.list 的 `path`）。
	Path string
	// IsDir 对应官方 `kind == "directory"`。
	IsDir bool
}

// SessionReferenceCandidate 是 `@` 会话候选的一行。
//
// Mention 是官方 wire 的 `@[label](dsh-session:id)` 原文——**不是**我们拼的：
// 会话 id 和标题都来自宿主 `sessionReferenceResolver.candidates`。
type SessionReferenceCandidate struct {
	Mention       string
	SessionID     string
	Label         string
	DisplayTitle  string
	Cwd           string
	SameWorkspace bool
	CreatedAt     int64
}

// SkillCandidate 是 `/` 候选的一行（宿主 `skills.list`，按会话取）。
type SkillCandidate struct {
	Name        string
	Description string
	// ModelInvocable 对应官方 `SkillEntry.modelInvocable`：false = 只对用户开放，
	// 菜单里显示官方 `menu.userOnly`「仅用户」。
	ModelInvocable bool
}

// FeedbackItem 是一条已记录的消息反馈（插件侧投影）。
type FeedbackItem struct {
	MessageID string
	// Rating 是 `positive` / `negative`（官方枚举，不是 up/down）。
	Rating   string
	Note     string
	Category string
	Version  string
}

// FeedbackResult 是一次 put/delete 的终态，供插件显示 toast。
//
// OK=false 时 Code 是宿主业务码（`version-conflict` / `note-blank` /
// `note-too-large` / …）或传输层错误摘要；插件按官方文案表显示，不自己编。
type FeedbackResult struct {
	MessageID string
	OK        bool
	Code      string
	Message   string
}
