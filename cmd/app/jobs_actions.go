package main

// jobs_actions.go —— 后台任务的**两条真实动作**：读输出（`jobs.output`）与
// 请求停止（`jobs.stop`）。
//
// 为什么要单独一份：这两条动作都是"宿主方法早就有了、界面一直没有生产调用者"
// （审计 `_audit/ui-face.md` #22 的判据）。插件不能自己发请求（R2/R7：插件只拿
// `*Scope` / Actions），所以由壳持缓存、发请求、重绘。
//
// 三条硬约束：
//
//  1. **不许在绘制线程里等 HTTP**：`JobsOutput` 同步只读缓存；缓存没有这一页时
//     返回 ok=false 并在后台拉，拉到后 Invalidate。插件把 ok=false 画成"加载中"。
//  2. **归属不猜**：宿主生产路径要求 sessionId + id 同时给，拒绝跨会话猜任务
//     （`HOST:internal/transport/handlers_jobs.go:109-112`）。缺任一就返回 ok=false，
//     不拿当前会话去顶。
//  3. **错误如实冒出来**：轮次结束后子进程退出、任务 owner 不在时宿主回 404/409，
//     这些要画成错误行，不能折叠成"没有输出"。

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// jobOutputCap 是单个任务累计输出的上限（rune）。
//
// 宿主自己保留 1 MiB 的执行窗口；界面只画一小段，翻页看历史即可。
// 设上限是防止"点了很多次加载更多"把进程内存拖成无界——但达到上限时
// 不静默丢：把 Truncated 置真，让界面显示"较早的输出已丢弃"。
const jobOutputCap = 512 * 1024

// jobOutputCache 是一个任务输出的**累计视图**（不是单页）。
//
// loaded 是已经加载到的 rune 游标；插件按"累计视图"整段重绘，
// 所以翻页只是把新一页拼接进来，不需要它自己维护两页的拼接状态。
type jobOutputCache struct {
	firstLoaded bool
	output      string
	loaded      int
	total       int
	hasMore     bool
	truncated   bool
	done        bool
	exitCode    int
	err         string
	// fetching 是正在请求的游标 + 1；0 = 空闲。防同一页重复发请求。
	fetching int
}

// jobOutputKey 是缓存键：同一 id 在不同会话里是两条任务，不能合并。
func jobOutputKey(sessionID, id string) string { return sessionID + "\x00" + id }

// jobOutputView 把缓存打包成插件侧的只读视图（调用方持锁）。
func (c *jobOutputCache) view(sessionID, id string) pluginapi.JobOutput {
	return pluginapi.JobOutput{
		ID: id, SessionID: sessionID, Output: c.output,
		Done: c.done, ExitCode: c.exitCode,
		Cursor: 0, NextCursor: c.loaded,
		HasMore: c.hasMore, Total: c.total, Truncated: c.truncated,
	}
}

// jobOutput 实现 pluginapi.Actions.JobsOutput。
//
// 三种返回：
//   - ok=true  + Err=""：这一页（或累计视图）已经在缓存里，可以画；
//   - ok=true  + Err!=""：上一次拉取失败，如实报错，同时自动重排一次拉取；
//   - ok=false          ：缓存还没有这一段，壳已在后台拉，插件画"加载中"。
func (m *model) jobOutput(sessionID, id string, cursor int) (pluginapi.JobOutput, bool) {
	if sessionID == "" || id == "" {
		// 归属不明：宿主会拒绝（跨会话猜任务），这里也不拿当前会话去顶。
		return pluginapi.JobOutput{ID: id, SessionID: sessionID, Err: "这条任务没有会话归属"}, true
	}
	key := jobOutputKey(sessionID, id)
	m.mu.Lock()
	if m.jobOutputs == nil {
		m.jobOutputs = map[string]*jobOutputCache{}
	}
	c := m.jobOutputs[key]
	if c == nil {
		c = &jobOutputCache{}
		m.jobOutputs[key] = c
	}
	if c.err != "" {
		out := c.view(sessionID, id)
		out.Err = c.err
		c.err = ""
		// 报过错这一帧之后自动重试一次；重试用 0 从开头读，避免游标错位。
		start := false
		if c.fetching == 0 {
			c.fetching = 1
			start = true
		}
		m.mu.Unlock()
		if start {
			go m.loadJobOutput(sessionID, id, 0)
		}
		return out, true
	}
	// 已经加载到请求位置（或已经读到尾）→ 直接给累计视图。
	if c.firstLoaded && (cursor < c.loaded || (cursor == c.loaded && !c.hasMore)) {
		out := c.view(sessionID, id)
		m.mu.Unlock()
		return out, true
	}
	// 需要新一页：同一页在飞就不重复发。
	if c.fetching != cursor+1 {
		c.fetching = cursor + 1
		m.mu.Unlock()
		go m.loadJobOutput(sessionID, id, cursor)
		return pluginapi.JobOutput{}, false
	}
	m.mu.Unlock()
	return pluginapi.JobOutput{}, false
}

// loadJobOutput 在后台拉一页输出，拼进累计视图后重绘。
func (m *model) loadJobOutput(sessionID, id string, cursor int) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	page, err := m.cli.JobsOutput(ctx, sessionID, id, cursor, 0)

	key := jobOutputKey(sessionID, id)
	m.mu.Lock()
	if m.jobOutputs == nil {
		m.jobOutputs = map[string]*jobOutputCache{}
	}
	c := m.jobOutputs[key]
	if c == nil {
		c = &jobOutputCache{}
		m.jobOutputs[key] = c
	}
	if c.fetching == cursor+1 {
		c.fetching = 0
	}
	if err != nil {
		// 失败就是失败：保留已加载的部分，把原因交给界面。
		c.err = err.Error()
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	switch {
	case cursor == 0:
		c.output = page.Output
		c.loaded = page.NextCursor
		c.total = page.Total
		c.hasMore = page.HasMore
		c.truncated = page.Truncated
	case cursor == c.loaded:
		c.output += page.Output
		c.loaded = page.NextCursor
		c.total = page.Total
		c.hasMore = page.HasMore
		c.truncated = c.truncated || page.Truncated
	default:
		// 迟到的乱序响应（理论上不会发生：单页在飞）：不拼接，报一次重来。
		c.err = "输出分页错位（请求游标与已加载游标不一致）"
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	c.done = page.Done
	c.exitCode = page.ExitCode
	c.firstLoaded = true
	// 上限：超过就丢最早的，并如实标 Truncated（不能静默吞）。
	if runes := []rune(c.output); len(runes) > jobOutputCap {
		c.output = string(runes[len(runes)-jobOutputCap:])
		c.truncated = true
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// jobStop 实现 pluginapi.Actions.JobsStop：异步请求停止，成功刷新清单。
//
// **不乐观更新**：点击之后界面不改任何状态，真正的"停了"以清单里
// Done/ExitCode 变化为准（宿主也只在自己确认进程结束后才回 stopped=true）。
func (m *model) jobStop(sessionID, id string) {
	if sessionID == "" || id == "" {
		m.setStatus("停止任务失败：缺会话归属")
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := m.cli.JobsStop(ctx, sessionID, id)
		if err != nil {
			m.setStatus("停止任务失败: " + err.Error())
			return
		}
		if !res.Stopped {
			m.setStatus("停止未被确认，任务可能还在跑")
			return
		}
		m.setStatus("已停止后台任务")
		// 输出缓存作废：停止后宿主给出的最后一页才是完整的。
		m.mu.Lock()
		delete(m.jobOutputs, jobOutputKey(sessionID, id))
		m.mu.Unlock()
		m.refreshJobs()
	}()
}

// pruneJobOutputs 丢掉已经不在清单里的任务缓存（调用方持 m.mu）。
func (m *model) pruneJobOutputs(jobs []pluginapi.Job) {
	if len(m.jobOutputs) == 0 {
		return
	}
	live := make(map[string]bool, len(jobs))
	for _, j := range jobs {
		live[jobOutputKey(j.SessionID, j.ID)] = true
	}
	for key := range m.jobOutputs {
		if !live[key] {
			delete(m.jobOutputs, key)
		}
	}
}
