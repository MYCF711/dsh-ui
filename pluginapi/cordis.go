package pluginapi

import "github.com/MYCF711/dsh-ui/internal/apiclient"

// cordis.go —— 官方 `dsh-client-ui-cordis` 的**只读数据面**（GUI 账本 #12）。
//
// # 官方这条包的两个座位
//
//	sidebar.footer.action       面板触发器 + 面板本体（frame-wide，不按会话过滤）
//	                             OFF:dsh-client-ui-cordis/src/client/index.ts:86-114
//	tool.call.toolview ×4       4 张历史工具卡（cordis_define / cordis_run /
//	                             cordis_stop / cordis_undefine）
//	                             OFF:…/src/client/index.ts:117-145
//
// 本仓**只落第一个**。第二组四个座位**HIDE**，依据不是"没做完"：
//
//   - 这四张卡的**唯一数据来源是已记录的 tool/call + tool/result 帧**
//     （OFF:card-model.ts:95-160 全部从 `block.argsRaw` / `block.result` 派生），
//     而本宿主注册 **0 个 `cordis_*` 工具**（HOST:docs/host-stable-gap-register.md:412：
//     「4 个 cordis_* 工具：注册 0 个」），官方 rc.2 自己也不注册
//     （`dsh-cordis-host-runner/README.md:119`「This package registers no tool or
//     prompt」）。没有生产者 = 这些帧永远不会出现，画出来是一块永远空着的座位。
//   - 本宿主也没有 define 通道（没有 Package/版本存储、没有审批），
//     `inventory.agentId` 因此可能为空串（见 Actions.CordisStopFromPanel 的注释）。
//
// 缺生产者就 HIDE，不自造"没有调用也画一张卡"的形态——那正是本仓红线
// （「缺官方证据或真实能力就 HIDE」）。
//
// # 面板的数据从哪来
//
// 面板是**帧内全局**的：它不按会话过滤（官方注释 index.ts:5-10 说得很清楚，
// 面板是 frame-wide surface），读的是一次全局 `dynamicCordisRunner.inventory`。
// 所以这份视图挂在 `Snapshot.Cordis` 上（与 `Snapshot.Forge` 同一位置），由壳在
// 每帧组装快照时**同步给出缓存**——插件在 draw 里不许发请求。
//
// **事件驱动刷新**：宿主只发两条真事件帧（`cordis/dynamic-package` 激活成功、
// `cordis/dynamic-retract` stop/undefine 撤下），它们落在**归属会话的 journal** 里
// （HOST:cmd/dshd/serve_cordis_api.go:420-455 `emitFrameAt`），所以壳是在
// 会话原始帧流（`Snapshot.Frames`）里看到它们的。看到新的一条就触发一次后台
// inventory 重读——这就是官方 `ctx.remote.$on('cordis/dynamic-package', …)` 的
// 等价路径（OFF:index.ts:75-80：事件里**不带标签**，只能整份重读）。
type CordisView struct {
	// Known 表示这份清单**真的读到过**（成功过一次）。
	//
	// 官方 `CordisInventorySnapshot.read` 同义（OFF:inventory.ts:27-33）：
	// false 时面板画「读取中…」而不是「还没有定义任何插件」——在知道之前宣称
	// "没有"是编的。
	Known bool
	// Rows 是宿主当前的清单（官方 `DynamicCordisInventoryRow[]` 的投影）。
	//
	// **顺序由宿主给**（它按 pluginId 排好）；界面按 `AgentID` 分组，
	// 不重排整个清单——分组不能改变组内顺序，否则"每次刷新行都在跳"。
	Rows []apiclient.CordisRow
	// Removed 是本页**显式移除过**的插件（官方 `snapshot.removed`，
	// OFF:inventory.ts:28-30）。
	//
	// 为什么界面自己记：undefine 成功后宿主清单还没重读回来，那段时间里历史卡片
	// 与面板都不该再显示它（官方 `retire()` 就是干这件事）。下一次成功读到的
	// 清单里没有它 → 继续留在集合里（粘性），重新定义回来 → 由壳清掉。
	Removed map[string]bool
	// Error 是最近一次读取失败的原因（官方 `snapshot.error`，OFF:inventory.ts:34-35）。
	//
	// **非空时不清空 Rows**：官方那句注释写得很清楚——把暂时失败的读当成
	// "什么都没定义"是这两件事里更坏的一种。面板据此画一行红色说明 + 保留旧行。
	Error string
	// EventSeq 是壳已经消费掉的最后一条 cordis 事件帧的 seq（0 = 还没见过）。
	//
	// 它有两个用处，都不是装饰：
	//   - 壳用它做**去重**：同一帧在 history 补页/重放里会再来一次，不重复触发读；
	//   - 插件/测试用它证明"事件真的走了 WS + journal 这条路"，而不是轮询。
	EventSeq int
}

// CordisRows 返回清单行（nil 安全），供插件在 draw 里直接遍历。
func (v *CordisView) CordisRows() []apiclient.CordisRow {
	if v == nil {
		return nil
	}
	return v.Rows
}

// CordisRemoved 判断一个插件是否被本页显式移除过（nil 安全）。
func (v *CordisView) CordisRemoved(pluginID string) bool {
	if v == nil {
		return false
	}
	return v.Removed[pluginID]
}
