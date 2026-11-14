// reference_wiring.go —— 把 `@` 候选、`/` 技能候选与消息反馈接到插件动作上。
//
// 三条共同纪律（与 FSList/ScheduleList 一致）：
//  1. **绘制线程不发 HTTP**：插件每帧都可能问，同步只读壳缓存；首次问触发一次
//     后台拉取，拉到后 Invalidate。
//  2. **空与未加载必须分开**：缓存里没有 = 还没拉到（插件可以画"加载中"或不画）；
//     拉到空切片 = 真的没有候选。两者都不许伪造成对方。
//  3. **失败不假装成功**：拉取失败只记日志/状态栏，不把失败写成一个空结果。
//
// 反馈多一条：宿主把业务失败放在 HTTP 200 的 union 里（`ok:false`），
// 所以这里必须解 `error.code`；`version-conflict` 用回包里的 authoritative
// `current` 就地收敛，再让插件下一帧读到新版本。
package main

import (
	"context"
	"log"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// referenceFetchTimeout 与 FSRead 同量级：候选是本地目录/索引读取，不该更慢。
const referenceFetchTimeout = 5 * time.Second

func refKey(sessionID, query string) string { return sessionID + "\x00" + query }

// fileReferences 返回 `@` 文件候选的壳缓存；首次问触发后台拉取。
func (m *model) fileReferences(sessionID, query string) []pluginapi.FileReferenceCandidate {
	if m == nil || m.cli == nil || sessionID == "" {
		return nil
	}
	key := refKey(sessionID, query)
	m.mu.Lock()
	if m.fileRefs == nil {
		m.fileRefs = map[string][]pluginapi.FileReferenceCandidate{}
		m.fileRefsPending = map[string]bool{}
		m.fileRefsFailed = map[string]bool{}
	}
	out, ok := m.fileRefs[key]
	pending := m.fileRefsPending[key]
	failed := m.fileRefsFailed[key]
	if !ok && !pending && !failed {
		m.fileRefsPending[key] = true
	}
	m.mu.Unlock()
	if !pending && !failed {
		go m.loadFileReferences(sessionID, query, key)
	}
	return out
}

func (m *model) loadFileReferences(sessionID, query, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
	defer cancel()
	list, err := m.cli.FileReferences(ctx, sessionID, query)
	m.mu.Lock()
	delete(m.fileRefsPending, key)
	if err != nil {
		log.Printf("fileReferences.list 失败（会话 %s, query=%q）：%v", sessionID, query, err)
		// 失败不伪造空结果；记下"这个 query 失败过"，避免插件每帧问一次
		// 就每帧重发一次请求。换 query 会换键，自然重试。
		delete(m.fileRefs, key)
		m.fileRefsFailed[key] = true
	} else {
		delete(m.fileRefsFailed, key)
		out := make([]pluginapi.FileReferenceCandidate, 0, len(list))
		for _, item := range list {
			out = append(out, pluginapi.FileReferenceCandidate{Path: item.Path, IsDir: item.Kind == "directory"})
		}
		m.fileRefs[key] = out
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// sessionReferenceCandidates 返回 `@` 会话候选的壳缓存。
func (m *model) sessionReferenceCandidates(sessionID, query string) []pluginapi.SessionReferenceCandidate {
	if m == nil || m.cli == nil || sessionID == "" {
		return nil
	}
	key := refKey(sessionID, query)
	m.mu.Lock()
	if m.sessionRefs == nil {
		m.sessionRefs = map[string][]pluginapi.SessionReferenceCandidate{}
		m.sessionRefsPending = map[string]bool{}
		m.sessionRefsFailed = map[string]bool{}
	}
	out, ok := m.sessionRefs[key]
	pending := m.sessionRefsPending[key]
	failed := m.sessionRefsFailed[key]
	if !ok && !pending && !failed {
		m.sessionRefsPending[key] = true
	}
	m.mu.Unlock()
	if !pending && !failed {
		go m.loadSessionReferenceCandidates(sessionID, query, key)
	}
	return out
}

func (m *model) loadSessionReferenceCandidates(sessionID, query, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
	defer cancel()
	list, err := m.cli.SessionReferenceCandidates(ctx, sessionID, query)
	m.mu.Lock()
	delete(m.sessionRefsPending, key)
	if err != nil {
		log.Printf("sessionReferenceResolver.candidates 失败（会话 %s, query=%q）：%v", sessionID, query, err)
		delete(m.sessionRefs, key)
		m.sessionRefsFailed[key] = true
	} else {
		delete(m.sessionRefsFailed, key)
		out := make([]pluginapi.SessionReferenceCandidate, 0, len(list))
		for _, item := range list {
			out = append(out, pluginapi.SessionReferenceCandidate{
				Mention: item.Mention, SessionID: item.SessionID, Label: item.Label,
				DisplayTitle: item.DisplayTitle, Cwd: item.Cwd,
				SameWorkspace: item.SameWorkspace, CreatedAt: item.CreatedAt,
			})
		}
		m.sessionRefs[key] = out
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// skillCandidates 返回 `/` 候选的壳缓存；首次问触发后台拉取。
func (m *model) skillCandidates(sessionID string) []pluginapi.SkillCandidate {
	if m == nil || m.cli == nil || sessionID == "" {
		return nil
	}
	m.mu.Lock()
	if m.skills == nil {
		m.skills = map[string][]pluginapi.SkillCandidate{}
		m.skillsPending = map[string]bool{}
		m.skillsFailed = map[string]bool{}
	}
	out, ok := m.skills[sessionID]
	pending := m.skillsPending[sessionID]
	failed := m.skillsFailed[sessionID]
	if !ok && !pending && !failed {
		m.skillsPending[sessionID] = true
	}
	m.mu.Unlock()
	if !pending && !failed {
		go m.loadSkillCandidates(sessionID)
	}
	return out
}

func (m *model) loadSkillCandidates(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
	defer cancel()
	result, err := m.cli.SkillsList(ctx, sessionID)
	m.mu.Lock()
	delete(m.skillsPending, sessionID)
	if err != nil {
		log.Printf("skills.list 失败（会话 %s）：%v", sessionID, err)
		delete(m.skills, sessionID)
		m.skillsFailed[sessionID] = true
	} else {
		delete(m.skillsFailed, sessionID)
		out := make([]pluginapi.SkillCandidate, 0, len(result.Skills))
		for _, item := range result.Skills {
			out = append(out, pluginapi.SkillCandidate{Name: item.Name, Description: item.Description, ModelInvocable: item.ModelInvocable})
		}
		m.skills[sessionID] = out
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// ---------- 消息反馈 ----------

// messageFeedback 返回某会话的反馈缓存；首次问触发后台拉取。
func (m *model) messageFeedback(sessionID string) (map[string]pluginapi.FeedbackItem, bool) {
	if m == nil || m.cli == nil || sessionID == "" {
		return nil, false
	}
	m.mu.Lock()
	if m.feedback == nil {
		m.feedback = map[string]map[string]pluginapi.FeedbackItem{}
		m.feedbackLoaded = map[string]bool{}
		m.feedbackPending = map[string]bool{}
		m.feedbackFailed = map[string]bool{}
	}
	items, loaded := m.feedback[sessionID]
	pending := m.feedbackPending[sessionID]
	failed := m.feedbackFailed[sessionID]
	if !loaded && !pending && !failed {
		m.feedbackPending[sessionID] = true
	}
	// 返回副本：插件在绘制期读，不能拿到会被后台 goroutine 改写的 map。
	out := map[string]pluginapi.FeedbackItem{}
	for k, v := range items {
		out[k] = v
	}
	m.mu.Unlock()
	if !loaded && !pending && !failed {
		go m.loadMessageFeedback(sessionID)
	}
	return out, loaded
}

func (m *model) loadMessageFeedback(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
	defer cancel()
	result, err := m.cli.MessageFeedbackList(ctx, sessionID)
	m.mu.Lock()
	delete(m.feedbackPending, sessionID)
	if err != nil {
		log.Printf("messageFeedback.list 失败（会话 %s）：%v", sessionID, err)
		m.feedbackFailed[sessionID] = true
	} else if !result.OK {
		code := "unknown"
		if result.Error != nil {
			code = result.Error.Code
		}
		log.Printf("messageFeedback.list 业务失败（会话 %s）：%s", sessionID, code)
		m.feedbackFailed[sessionID] = true
	} else {
		delete(m.feedbackFailed, sessionID)
		items := map[string]pluginapi.FeedbackItem{}
		if result.Value != nil {
			for _, item := range result.Value.Items {
				items[item.MessageID] = feedbackItem(item)
			}
		}
		if m.feedback == nil {
			m.feedback = map[string]map[string]pluginapi.FeedbackItem{}
		}
		m.feedback[sessionID] = items
		if m.feedbackLoaded == nil {
			m.feedbackLoaded = map[string]bool{}
		}
		m.feedbackLoaded[sessionID] = true
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

func feedbackItem(item apiclient.MessageFeedbackItem) pluginapi.FeedbackItem {
	return pluginapi.FeedbackItem{
		MessageID: item.MessageID, Rating: item.Rating, Note: item.Note,
		Category: item.Category, Version: item.Version,
	}
}

// messageFeedbackPut 写一条反馈（异步）。成功后更新缓存；冲突用 current 收敛。
func (m *model) messageFeedbackPut(sessionID, messageID, rating, note, category string) {
	if m == nil || m.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
		defer cancel()
		ifVersion := m.feedbackVersion(sessionID, messageID)
		result, err := m.cli.MessageFeedbackPut(ctx, sessionID, messageID, rating, note, category, ifVersion)
		m.finishFeedbackWrite(sessionID, messageID, result, err)
	}()
}

// messageFeedbackDelete 撤回一条反馈（异步）。
func (m *model) messageFeedbackDelete(sessionID, messageID string) {
	if m == nil || m.cli == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), referenceFetchTimeout)
		defer cancel()
		version := ""
		m.mu.Lock()
		if item, ok := m.feedback[sessionID][messageID]; ok {
			version = item.Version
		}
		m.mu.Unlock()
		if version == "" {
			// 没有已记录版本：撤回的稳定后置条件已经成立，不必打宿主。
			m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: true})
			return
		}
		result, err := m.cli.MessageFeedbackDelete(ctx, sessionID, messageID, version)
		if err != nil {
			m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: false, Code: "transport", Message: err.Error()})
			return
		}
		if result.OK {
			m.mu.Lock()
			if m.feedback[sessionID] != nil {
				delete(m.feedback[sessionID], messageID)
			}
			m.mu.Unlock()
			m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: true})
			return
		}
		code := "unknown"
		if result.Error != nil {
			code = result.Error.Code
		}
		m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: false, Code: code})
	}()
}

// feedbackVersion 读当前缓存里的 version（nil = 要求当前不存在）。
func (m *model) feedbackVersion(sessionID, messageID string) *string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.feedback[sessionID][messageID]; ok && item.Version != "" {
		version := item.Version
		return &version
	}
	return nil
}

// finishFeedbackWrite 处理 put 的 union 回包：成功/冲突都更新缓存。
func (m *model) finishFeedbackWrite(sessionID, messageID string, result apiclient.MessageFeedbackPutResult, err error) {
	if err != nil {
		m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: false, Code: "transport", Message: err.Error()})
		return
	}
	if result.OK && result.Value != nil {
		m.mu.Lock()
		if m.feedback[sessionID] == nil {
			m.feedback[sessionID] = map[string]pluginapi.FeedbackItem{}
		}
		m.feedback[sessionID][messageID] = feedbackItem(*result.Value)
		m.mu.Unlock()
		m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: true})
		return
	}
	code := "unknown"
	if result.Error != nil {
		code = result.Error.Code
	}
	if result.Error != nil && result.Error.Code == "version-conflict" {
		if current, present := result.Error.FeedbackCurrent(); present {
			m.mu.Lock()
			if m.feedback[sessionID] == nil {
				m.feedback[sessionID] = map[string]pluginapi.FeedbackItem{}
			}
			if current == nil {
				delete(m.feedback[sessionID], messageID)
			} else {
				m.feedback[sessionID][messageID] = feedbackItem(*current)
			}
			m.mu.Unlock()
		}
	}
	m.recordFeedbackResult(sessionID, messageID, pluginapi.FeedbackResult{MessageID: messageID, OK: false, Code: code})
}

func (m *model) recordFeedbackResult(sessionID, messageID string, result pluginapi.FeedbackResult) {
	m.mu.Lock()
	if m.feedbackResults == nil {
		m.feedbackResults = map[string]pluginapi.FeedbackResult{}
	}
	m.feedbackResults[sessionID] = result
	m.mu.Unlock()
	m.inv.Invalidate()
}

// feedbackTakeResult 取走最近一次终态（取走即清空，避免同一 toast 重放）。
func (m *model) feedbackTakeResult(sessionID string) (pluginapi.FeedbackResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result, ok := m.feedbackResults[sessionID]
	if ok {
		delete(m.feedbackResults, sessionID)
	}
	return result, ok
}
