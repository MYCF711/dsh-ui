// cordis_wiring.go —— #12（官方 `dsh-client-ui-cordis`）的壳侧数据面。
//
// # 数据从哪来（每条都指到宿主真实现）
//
//	清单           ← dynamicCordisRunner.inventory        HOST:cmd/dshd/serve_cordis_api.go（inventory）
//	运行 Host 半   ← dynamicCordisRunner.runHostHalf      HOST:（case runHostHalf）
//	停止           ← dynamicCordisRunner.stopFromPanel    HOST:（case stopFromPanel）
//	移除           ← dynamicCordisRunner.undefineFromPanel HOST:（case undefineFromPanel）
//	两条事件帧     ← WS → 归属会话 journal                 HOST:`emitFrameAt`（cordis/dynamic-package / -retract）
//
// # 为什么清单是"全局缓存"而不是挂在会话上
//
// 官方面板是 **frame-wide surface**（OFF:dsh-client-ui-cordis/src/client/index.ts:5-10）：
// 它不按会话过滤，读的是一次全局 `inventory()`，页面自己按每行自带的 `agentId`
// 分「当前会话 / 其他会话」两组（OFF:CordisPanel.tsx:183-184）。所以这里也**不挂
// 会话**：一份缓存、一次读，壳在快照里同步给插件。
//
// # 事件怎么驱动 UI（这一步是"真的走 WS"，不是轮询）
//
// 宿主只发两条有真实触发点的事件（HOST:serve_cordis_api.go:78-84）：
//
//	cordis/dynamic-package   激活成功
//	cordis/dynamic-retract   stop / undefine 撤下
//
// 它们落在**归属会话的 Transport journal**（`emitFrameAt`，与 `agent/*` 同一条管线），
// 所以壳在拼快照时扫一遍当前会话的原始帧就能看到它们（`cordisWatchEventsLocked`）。
// 看到一条**没消费过**的（seq 比水位大）就触发一次后台重读——这正是官方
// `ctx.remote.$on('cordis/dynamic-package', () => inventory.refresh())` 的等价物
// （OFF:index.ts:75-80；官方注释说得很清楚：事件里不带标签，只能整份重读）。
//
// **如实登记的局限**：另外两条（`cordis/request-run` / `cordis/request-run-resolved`）
// 宿主不发（没有 Cordis 审批队列），所以面板上的 approve/decline 两个按钮不画。
// 而且事件只进**归属会话**的 journal，本仓只订阅当前会话——别的会话的激活事件
// 要等切过去才看到。缓解措施有两层，不是"听天由命"：面板每次打开、以及每次
// 生命周期动作之后都主动重读一次清单（`Actions.CordisRefresh`）。
//
// # 缓存与线程
//
// 插件在**绘制线程**上调读写，所以：
//
//	读  —— `cordisFor` 同步返回缓存（快照组装期调用），首次/事件到达时**后台**拉一次；
//	写  —— 三个生命周期动词全部落地成 goroutine，回包后改缓存 + Invalidate。
package main

import (
	"context"
	"errors"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 宿主真实发出的两条 cordis 事件帧的 `type`（HOST:cmd/dshd/serve_cordis_api.go:440-455）。
const (
	cordisEventPackage = "cordis/dynamic-package"
	cordisEventRetract = "cordis/dynamic-retract"
)

// cordisEventThrottle 是"事件连发时"两次重读之间的最小间隔。
//
// 为什么需要：一次 undefine 会连着发 retract，而 shell 补页/重放也可能一次送来
// 好几条。官方那边是 `refresh()` 的 single-flight（OFF:inventory.ts:75-77 的
// `if (inFlight !== undefined) return`）——本仓的等价物就是这个 pending 位 +
// 落地后的节流：pending 在飞时不叠加请求，间隔内不重复开新的。
const cordisEventThrottle = 800 * time.Millisecond

// journalWatermark 是事件水位的**分域**持有者：一个 `(sessionID, journalEpoch)`
// 一个独立的水位，互不影响。
//
// # 为什么必须是分域的（这是修掉的一个真 bug，不是防御性设计）
//
// 宿主在**每会话各自的 journal** 追加时分配 seq（`docs/transport-contract.md` §3：
// 「每会话一条独立 WS + 一个 Transport journal；seq 在 journal 追加时分配，
// 不能是 v4 seq、全局 turn seq 或连接序号」）。所以：
//
//	A 会话 journal 的 seq = 1..900，B 会话 journal 的 seq = 1..3 —— **两个序号域**。
//
// 早先用一个全局 `eventSeq` 记"已消费到哪"，切到 B 之后 B 的新事件 seq=3
// 永远 `<= 900`，于是被判成"早就消费过"，**再也不触发重读**。表现是
// 「B 里 cordis 激活了，面板不刷新」，既不报错也不丢帧。
//
// # 为什么键里还要带 journalEpoch
//
// 同一个 sessionID 在宿主重启后会换一份 journal（`journalEpoch` 变），
// 新 journal 的 seq **从 1 重新开始**。只按 sessionID 记水位的话，重启前
// 涨到 900 的水位会继续压住重启后的 seq=1..n —— 同一个 bug 换个触发条件。
// 键里带 epoch，换世代就是换序号域，水位从 0 重新开始。
//
// # 为什么不设上限
//
// 键的数量受"本进程见过多少个 (会话, 世代)"约束，一个壳在一次运行里见过的
// 会话是有限且很小的（用户手点的会话数 + 重启次数），不为它引入淘汰逻辑——
// 淘汰逻辑本身要猜"哪个不会再用了"，猜错就退回同一个漏更新。
type journalWatermark struct {
	// seq 是每个 `(sessionID, journalEpoch)` 已消费到的最大 seq。
	seq map[journalKey]int
}

// journalKey 是一个序号域的标识。
type journalKey struct {
	sessionID string
	epoch     int64
}

// consume 报告 frames 里有没有**本域没见过**的 cordis 事件；有就把水位抬上去。
//
// sessionID / epoch 决定"这是哪个序号域"。空 sessionID（壳还没加载会话、
// 或宿主没给 id）时**整段跳过**：那时没有可信的域标识，
// 记一个"空 id 域"会把不同会话的水位混成一份，等于退回全局单值。
//
// 返回 true = 至少有一条新事件，调用方据此触发一次重读。
// 调用方必须持有 m.mu。
func (w *journalWatermark) consume(sessionID string, epoch int64, frames []apiclient.Frame) bool {
	if sessionID == "" {
		return false
	}
	key := journalKey{sessionID: sessionID, epoch: epoch}
	fresh := false
	for i := range frames {
		frame := frames[i]
		if frame.Type != cordisEventPackage && frame.Type != cordisEventRetract {
			continue
		}
		if frame.Seq <= w.seq[key] {
			continue
		}
		if w.seq == nil {
			w.seq = map[journalKey]int{}
		}
		w.seq[key] = frame.Seq
		fresh = true
	}
	return fresh
}

// last 返回某个序号域的水位（0 = 还没见过该域的任何事件）。
// 调用方必须持有 m.mu。
func (w *journalWatermark) last(sessionID string, epoch int64) int {
	return w.seq[journalKey{sessionID: sessionID, epoch: epoch}]
}

// cordisState 是 #12 的壳侧缓存。字段全部由 m.mu 保护（与壳其余状态同一把锁）。
type cordisState struct {
	// known 表示**成功读到过一次**清单（官方 `snapshot.read`）。
	//
	// false 时面板画「读取中…」而不是「还没有定义任何插件」：在知道之前宣称
	// "没有"是编的（OFF:inventory.ts:27-33 的注释写的是同一件事）。
	known bool
	rows  []apiclient.CordisRow
	// removed 是本页**显式移除过**的 pluginId（官方 `snapshot.removed` 的粘性集合）。
	// 目前只影响"移除成功那一刻行立刻消失"，下一次读到的清单里自然也不会再有它。
	removed map[string]bool
	// err 是最近一次读失败的原因（官方 `snapshot.error`）。**非空时不清 rows**：
	// 把暂时失败的读当成"什么都没定义"是这两件事里更坏的一种。
	err string
	// pending 防重入：读在途时不再发第二次请求（每帧、每条事件都可能触发）。
	pending bool
	// watermark 是**按 journal 分域**的事件水位（见 journalWatermark）。
	//
	// 早先这里是一个全局单值 `eventSeq`，那是错的：seq 由宿主在**每会话各自的
	// journal** 追加时分配（`docs/transport-contract.md` §3「seq 在 journal 追加时
	// 分配」），不同会话的 seq **不共享序号域**。用全局单值记水位，切会话时
	// 高 seq 会话（例如 A 的 900）会压住低 seq 会话（B 的 3）的新事件，
	// 表现为「B 里 cordis 明明激活了，面板却不更新」——不报错、不丢帧，
	// 只是少了一次重读，是最难发现的一类错。
	watermark journalWatermark
	// at 是上一次读**完成**的时刻（节流用；零值 = 从没读过）。
	at time.Time
}

// cordisFor 返回本帧的 Cordis 数据面（同步；nil = 壳没接客户端）。
//
// 它在快照组装时被调用，因此**必须**可以每帧调：没有客户端（mockd / 测试）时
// 返回 nil，插件据此整块不画（连触发器都不画）；有客户端但还没读到 → 返回
// `Known:false` 的视图并触发一次后台读。
//
// frames 是当前会话的原始 journal 帧（`Snapshot.Frames` 同一份），这里**只读**
// 它们来找两条 cordis 事件——不复制、不重放、不改水位（水位本文件自持）。
//
// sessionID / epoch 是 frames 所属的**序号域**（`Snapshot.Cur` 与
// `Snapshot.JournalEpoch`）。水位按域分记（见 journalWatermark）：
// 不同会话、同一会话的不同世代，seq 都不共享序号域。
func (m *model) cordisFor(sessionID string, epoch int64, frames []apiclient.Frame) *pluginapi.CordisView {
	if m.cli == nil {
		return nil
	}
	m.mu.Lock()
	events := m.cordisWatchEventsLocked(sessionID, epoch, frames)
	first := !m.cordis.known && !m.cordis.pending
	view := &pluginapi.CordisView{
		Known:    m.cordis.known,
		Rows:     append([]apiclient.CordisRow(nil), m.cordis.rows...),
		Removed:  cordisCopySet(m.cordis.removed),
		Error:    m.cordis.err,
		EventSeq: m.cordis.watermark.last(sessionID, epoch),
	}
	m.mu.Unlock()
	// 事件到达是**事实变化**，不受节流约束（节流只挡"用户连点/面板反复打开"那类
	// 重复动作）；首次读也不受约束（那时还没有基线，at 是零值本来也不会被挡）。
	if events || first {
		m.cordisRefresh(true)
	}
	return view
}

// cordisWatchEventsLocked 扫当前会话的原始帧，吞掉没见过的那几条 cordis 事件。
//
// 返回 true = 至少有一条**新的**事件：调用方据此触发一次重读。
//
// 为什么按 seq 记水位而不是"按最后一条的类型判断"：journal 补页、重连重放都会
// 把同一帧再送一次，靠类型判断会重复触发（官方 single-flight 只能挡住并发，
// 挡不住重放）。seq 在同一 journal 世代内单调，是这里唯一可靠的去重键。
//
// 水位**按 `(sessionID, epoch)` 分域**（journalWatermark）：seq 只在同一个
// journal 世代内可比，跨会话或跨世代直接比大小会把新事件误判成旧帧。
// 调用方必须持有 m.mu。
func (m *model) cordisWatchEventsLocked(sessionID string, epoch int64, frames []apiclient.Frame) bool {
	return m.cordis.watermark.consume(sessionID, epoch, frames)
}

// cordisRefreshAction 触发一次**带节流**的后台清单重读（官方 `onRefresh`）。
//
// 这是插件在"打开面板"这类**重复动作**上调的那一条：绘制线程上不能等 HTTP，
// 所以只置一个 pending 位就返回（官方 same-flight 的等价物，见 inventory.ts:75-77）。
// 距上一次完成不足 throttle 时直接跳过——用户连点十下不该变成十次读。
func (m *model) cordisRefreshAction() { m.cordisRefresh(false) }

// cordisRefresh 是两条入口共用的实现；force=true 时**跳过节流**（事实已经变了，
// 例如收到一条新的 cordis 事件帧），但仍然走 single-flight（不许并发读）。
func (m *model) cordisRefresh(force bool) {
	if m.cli == nil {
		return
	}
	m.mu.Lock()
	if m.cordis.pending {
		m.mu.Unlock()
		return
	}
	if !force && !m.cordis.at.IsZero() && time.Since(m.cordis.at) < cordisEventThrottle {
		m.mu.Unlock()
		return
	}
	m.cordis.pending = true
	m.mu.Unlock()
	go m.cordisLoadInventory()
}

// cordisLoadInventory 是一次真实读：`dynamicCordisRunner.inventory`。
//
// 成功：整份替换 rows + known=true + 清 error。
// 失败：**保留旧 rows**、只写 error（官方注释：drop 掉旧行会把"一次读失败"
// 升级成"什么都没定义"）。
func (m *model) cordisLoadInventory() {
	ctx, cancel := context.WithTimeout(context.Background(), cordisReadBudget)
	defer cancel()
	rows, err := m.cli.CordisInventory(ctx)
	m.mu.Lock()
	m.cordis.pending = false
	m.cordis.at = time.Now()
	if err != nil {
		m.cordis.err = err.Error()
	} else {
		m.cordis.rows = rows
		m.cordis.known = true
		m.cordis.err = ""
		// 清单里又出现 = 它不是"本页移除过"了（重新定义回来的插件要能重新显示）。
		for id := range m.cordis.removed {
			if cordisHasPlugin(rows, id) {
				delete(m.cordis.removed, id)
			}
		}
	}
	m.mu.Unlock()
	if err != nil {
		m.setStatus("读取 Cordis 插件清单失败: " + err.Error())
	}
	m.inv.Invalidate()
}

// cordisReadBudget 是壳侧等一次 inventory 的上限（比客户端自己的 10s 略宽，
// 让客户端先超时并给出它那条更贴近传输层的错误文本）。
const cordisReadBudget = 12 * time.Second

// cordisHasPlugin 判断清单里有没有这个 pluginId。
func cordisHasPlugin(rows []apiclient.CordisRow, pluginID string) bool {
	for i := range rows {
		if rows[i].PluginID == pluginID {
			return true
		}
	}
	return false
}

// cordisCopySet 复制一份字符串集合（空/nil 安全）。
func cordisCopySet(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// ── 三个生命周期动词（官方 CordisPanel 的三个 RowAction）──────────────────

// cordisRunAction 起一个插件的 **Host 半**（官方 `onRun → startUserRun → runHostHalf`）。
//
// 只发 Host 半：Client 半需要一个能在本页跑的 JS 组件宿主，Go+Viva 没有
// （见 `pluginapi/cordis.go` 的 HIDE 依据第 3 条）。所以插件只对**没有 Client 半**
// 的包画这个按钮——有 Client 半的包起了就永远停在 client-pending。
func (m *model) cordisRunAction(sessionID, pluginID, packageID, mode string) error {
	if m.cli == nil {
		return errors.New("没有宿主连接")
	}
	// 官方 `agentId` 可能是空串（没有 define 通道）：不带归属就没有事件帧，
	// 但激活本身仍可发生——照实发，不改判据、不编一个会话名。
	result, err := m.cli.CordisRunHostHalf(context.Background(), sessionID, pluginID, packageID, mode)
	if err == nil && !result.OK {
		err = errors.New(cordisMessageOr(result.Message, "启动 Host 半失败"))
	}
	m.mu.Lock()
	m.cordis.at = time.Time{} // 动作之后必须真的重读，不受节流挡
	m.mu.Unlock()
	m.cordisRefreshAction()
	return err
}

// cordisStopAction 停一个插件的当前运行（官方 `onStop → stopFromPanel`）。
//
// 官方的折算在这里（OFF:index.ts:46-51）：`ok:true` 与 `reason:'not-running'`
// **都算成功**——"本来就没在跑"= 用户想要的结果已经成立。只有 `plugin-missing`
// 和传输失败才是失败。把它放壳这一层，插件就不必各写一遍这条判断。
func (m *model) cordisStopAction(sessionID, pluginID string) error {
	if m.cli == nil {
		return errors.New("没有宿主连接")
	}
	answer, err := m.cli.CordisStopFromPanel(context.Background(), sessionID, pluginID)
	if err != nil {
		return err
	}
	if answer.OK || answer.Reason == "not-running" {
		m.mu.Lock()
		m.cordis.at = time.Time{}
		m.mu.Unlock()
		m.cordisRefreshAction()
		return nil
	}
	return errors.New(cordisMessageOr(answer.Message, "停止失败："+answer.Reason))
}

// cordisRemoveAction 停并移除一个插件及其全部 Package
// （官方 `onRemove → undefineFromPanel`，成功后 `inventory.retire(pluginId)`）。
//
// 官方 `retire()` 的语义（OFF:index.ts:106-110 + inventory.ts:102-106）：
// **立刻**把这一行从本地 rows 里摘掉并记进 removed，不等下一次读回来。
// 这里照同一顺序做，所以点完那一帧行就消失了（而不是转半秒才消失）。
func (m *model) cordisRemoveAction(sessionID, pluginID string) error {
	if m.cli == nil {
		return errors.New("没有宿主连接")
	}
	receipt, err := m.cli.CordisUndefineFromPanel(context.Background(), sessionID, pluginID)
	if err != nil {
		return err
	}
	if !receipt.OK {
		return errors.New(cordisMessageOr(receipt.Message, "移除失败："+receipt.Reason))
	}
	m.mu.Lock()
	if m.cordis.removed == nil {
		m.cordis.removed = map[string]bool{}
	}
	m.cordis.removed[pluginID] = true
	kept := m.cordis.rows[:0]
	for _, row := range m.cordis.rows {
		if row.PluginID != pluginID {
			kept = append(kept, row)
		}
	}
	m.cordis.rows = kept
	m.cordis.at = time.Time{}
	m.mu.Unlock()
	m.inv.Invalidate()
	m.cordisRefreshAction()
	return nil
}

// cordisMessageOr 取宿主原文，空时用兜底文案（不要把空串当"成功"）。
func cordisMessageOr(message, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}
