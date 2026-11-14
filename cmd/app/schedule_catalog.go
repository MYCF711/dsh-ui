// schedule_catalog.go —— 官方 `dsh-client-ui-schedule` 的**壳侧数据面**：
// 任务目录（schedule.catalog）、投递历史（schedule.history）、以及三条真实动作
// （schedule.edit delete / patch.active、schedule.run）。
//
// # 为什么壳要有这一份，而不是复用既有的 schedules 缓存
//
// `schedules`（`cmd/app/main.go`）是宿主扩展面 `schedule.list` 的投影，只有
// `id/title/kind/nextAt` 四个字段。官方任务管理页要画状态（active/paused/ended）、
// 下次运行、关联会话、投递记录，这些只有官方 `schedule.catalog` 给得出
// （形状见 `internal/apiclient/schedule.go` 顶部注释）。两者并存不是重复：
// `dsschedule` 的侧栏摘要仍走 list，任务页走 catalog。
//
// # 为什么加载是"按需 + 动作后重拉"，没有轮询
//
// 官方页面自己也是**读一次目录、动作后读回**（`OFF:ui-schedule/src/client/
// catalog-source.ts` 的 `schedule/changed` 订阅）。本仓 Transport 没有
// `schedule/changed` 推送，所以边界是：面板打开时拉、每个成功动作后重拉、
// 失败给重试。**不拿"这一帧没有"当"宿主没有任务"**——loaded 单独一位。
//
// # 写动作的三条边界（与 jobs 的 stop 同一套态度）
//
//  1. **不乐观更新**：删除/暂停/执行都不改本地目录，等宿主回话后重拉。
//     乐观删除一条后台还在触发的任务，是拿界面骗用户。
//  2. **miss 不是错误**：`schedule_not_found` / `schedule_conflict` /
//     `schedule_execution_pending` 都是 HTTP 200 的"没有改动"（官方 union），
//     处置是重拉目录收敛，不弹"失败"。
//  3. **失败当场说一次**：动作 id 交给插件画官方那一句本地化文案，宿主原文
//     写状态栏（诊断用），不另编文案。

package main

import (
	"context"
	"log"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// scheduleHistoryPageSize 是官方 `schedule.history` 的一页条数。
//
// 官方页面打开记录页时请求 newest 20（`OFF:TaskManagerPage` 的
// "newest 20 saved records"），limit 的合同范围是 1..100。
const scheduleHistoryPageSize = 20

// refreshScheduleCatalog 拉一次官方任务目录。
//
// 失败**不清旧缓存**：把已经看到过的任务突然清空，用户会以为"任务没了"；
// 只把失败原因放进 errText，让列表画错误行 + 重试。
func (m *model) refreshScheduleCatalog() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := m.cli.ScheduleCatalog(ctx)

	m.mu.Lock()
	m.schedCatalogPending = false
	if err != nil {
		m.schedCatalogErr = err.Error()
		m.mu.Unlock()
		m.inv.Invalidate()
		return
	}
	m.schedCatalog = entries
	m.schedCatalogLoaded = true
	m.schedCatalogErr = ""
	m.mu.Unlock()
	m.inv.Invalidate()
}

// scheduleCatalog 是 `Actions.ScheduleCatalog` 的实现：同步返回缓存，
// 首次问时触发一次后台拉取（与 JobsList 同一模式，插件每帧都会问）。
func (m *model) scheduleCatalog() ([]apiclient.ScheduleEntry, bool, string) {
	m.mu.Lock()
	entries := m.schedCatalog
	loaded := m.schedCatalogLoaded
	errText := m.schedCatalogErr
	pending := m.schedCatalogPending
	if !loaded && !pending {
		m.schedCatalogPending = true
	}
	m.mu.Unlock()

	if !loaded && !pending {
		go m.refreshScheduleCatalog()
	}
	// 浅拷贝：缓存整体替换，不原地改元素；拷贝只是防止调用方 append 到同底层数组。
	out := make([]apiclient.ScheduleEntry, len(entries))
	copy(out, entries)
	return out, loaded, errText
}

// scheduleRefresh 是 `Actions.ScheduleRefresh`：显式重拉（列表的重试按钮）。
func (m *model) scheduleRefresh() {
	m.mu.Lock()
	if m.schedCatalogPending {
		m.mu.Unlock()
		return // 已经有一次在飞，别叠加
	}
	m.schedCatalogPending = true
	m.mu.Unlock()
	go m.refreshScheduleCatalog()
}

// scheduleHistory 是 `Actions.ScheduleHistory`：同步返回缓存，首次问触发拉取。
func (m *model) scheduleHistory(id string) (apiclient.ScheduleHistoryPage, bool, string) {
	m.mu.Lock()
	if m.schedHistoryLoaded == nil {
		m.schedHistoryLoaded = map[string]bool{}
	}
	if m.schedHistoryPending == nil {
		m.schedHistoryPending = map[string]bool{}
	}
	page, has := m.schedHistory[id]
	loaded := m.schedHistoryLoaded[id]
	errText := m.schedHistoryErr[id]
	pending := m.schedHistoryPending[id]
	if !loaded && !pending {
		m.schedHistoryPending[id] = true
	}
	m.mu.Unlock()

	if !loaded && !pending {
		go m.refreshScheduleHistory(id, "")
	}
	if !has || page == nil {
		return apiclient.ScheduleHistoryPage{ID: id}, loaded, errText
	}
	out := *page
	out.Records = append([]apiclient.ScheduleHistoryRecord(nil), page.Records...)
	return out, loaded, errText
}

// refreshScheduleHistory 读一页投递记录。before 空 = 第一页（newest first）。
func (m *model) refreshScheduleHistory(id, before string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	page, code, err := m.cli.ScheduleHistory(ctx, id, scheduleHistoryPageSize, before)

	m.mu.Lock()
	if m.schedHistoryPending == nil {
		m.schedHistoryPending = map[string]bool{}
	}
	delete(m.schedHistoryPending, id)
	if m.schedHistoryErr == nil {
		m.schedHistoryErr = map[string]string{}
	}
	if m.schedHistoryLoaded == nil {
		m.schedHistoryLoaded = map[string]bool{}
	}
	switch {
	case err != nil:
		m.schedHistoryErr[id] = err.Error()
	case code != "":
		// 官方 miss（schedule_not_found / delivery_cursor_not_found）不是错误：
		// 它是"这条任务在宿主上已经不可用"的事实。界面按 code 画对应文案。
		m.schedHistoryErr[id] = code
	default:
		if before == "" || m.schedHistory[id] == nil {
			m.schedHistory[id] = &page
		} else {
			// 续页：官方分页是 newest-first 追加顺序，旧记录接在已有记录之后。
			old := m.schedHistory[id]
			merged := *old
			merged.Records = append(append([]apiclient.ScheduleHistoryRecord(nil), old.Records...), page.Records...)
			merged.EarlierRecordsUnavailable = page.EarlierRecordsUnavailable
			merged.EarlierRecordsPruned = page.EarlierRecordsPruned
			merged.Retention = page.Retention
			merged.NextBefore = page.NextBefore
			m.schedHistory[id] = &merged
		}
		m.schedHistoryLoaded[id] = true
		m.schedHistoryErr[id] = ""
	}
	m.mu.Unlock()
	m.inv.Invalidate()
}

// scheduleHistoryMore 是 `Actions.ScheduleHistoryMore`：续读更旧一页。
func (m *model) scheduleHistoryMore(id string) {
	m.mu.Lock()
	page := m.schedHistory[id]
	pending := m.schedHistoryPending[id]
	if page == nil || page.NextBefore == "" || pending {
		m.mu.Unlock()
		return
	}
	m.schedHistoryPending[id] = true
	before := page.NextBefore
	m.mu.Unlock()
	go m.refreshScheduleHistory(id, before)
}

// markScheduleActionFailed 记下一次用户动作失败（动作 id + 状态栏原文）。
func (m *model) markScheduleActionFailed(action string, err error) {
	if err != nil {
		log.Printf("schedule %s 失败: %v", action, err)
	}
	m.mu.Lock()
	m.schedActionFail = action
	m.mu.Unlock()
	m.inv.Invalidate()
}

// scheduleActionError 是 `Actions.ScheduleActionError`：取走最近一次动作失败。
func (m *model) scheduleActionError() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.schedActionFail == "" {
		return "", false
	}
	action := m.schedActionFail
	m.schedActionFail = ""
	return action, true
}

// dropScheduleHistory 丢掉一条任务的历史缓存（删除后重读）。
func (m *model) dropScheduleHistory(id string) {
	m.mu.Lock()
	delete(m.schedHistory, id)
	delete(m.schedHistoryLoaded, id)
	delete(m.schedHistoryErr, id)
	delete(m.schedHistoryPending, id)
	m.mu.Unlock()
}

// scheduleDelete 是 `Actions.ScheduleDelete`（异步，不乐观更新）。
func (m *model) scheduleDelete(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := m.cli.ScheduleDelete(ctx, id)
		if err != nil {
			m.markScheduleActionFailed("delete", err)
			return
		}
		if out.Code != "" {
			// 官方 miss：任务已在别处变了。重拉收敛，不报失败。
			m.refreshScheduleCatalog()
			return
		}
		m.dropScheduleHistory(id)
		m.refreshScheduleCatalog()
	}()
}

// scheduleSetActive 是 `Actions.ScheduleSetActive`（暂停/恢复，异步）。
func (m *model) scheduleSetActive(id string, active bool, expectedUpdatedAt string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := m.cli.ScheduleSetActive(ctx, id, active, expectedUpdatedAt)
		if err != nil {
			m.markScheduleActionFailed("update", err)
			return
		}
		_ = out // 条目正文会在重拉里回来：写路径不拿它更新本地（不乐观更新）
		m.refreshScheduleCatalog()
	}()
}

// scheduleRun 是 `Actions.ScheduleRun`（立即执行，异步）。
func (m *model) scheduleRun(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := m.cli.ScheduleRun(ctx, id)
		if err != nil {
			m.markScheduleActionFailed("run", err)
			return
		}
		if out.Code == "schedule_execution_pending" || out.Code == "schedule_not_found" {
			// 两者都是官方的"没有改动"：已排队/执行中/任务不存在。重拉收敛。
			m.refreshScheduleCatalog()
			return
		}
		m.refreshScheduleCatalog()
	}()
}
