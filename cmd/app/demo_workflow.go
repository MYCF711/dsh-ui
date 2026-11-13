package main

import (
	"encoding/json"
	"flag"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 这个文件是**调试用的假数据**，只为一件事：在没有宿主推 `tool-workflow/*`
// 帧的情况下，也能给 §5.20 工作流运行面板截一张真实的图。
//
// # 为什么需要它（数据面的事实）
//
// 官方 `@deepseek-ai/dsh-tool-workflow` 与官方 `dsh-workflow` 是两个包：
//
//	官方 workflow 工具  Lua/JS 编排脚本 + agent() 扇出 + phase() 分组
//	                    → 往父会话写四类耐久事件（run-start / agent-start /
//	                      agent-end / run-end）
//	本仓 modules/tool/workflow  声明式 JSON 步骤表（`script` + `meta`）
//	                    → **只回一个 spi.Result，一条耐久事件都不写**
//
// 所以本仓宿主今天**没有任何** `tool-workflow/*` 生产者；界面要验这个面板，
// 只能自己造帧——这正是本文件存在的理由，也是报告里那条"宿主数据面缺口"。
//
// 它不是产品路径的一部分：开关关着（默认）时一帧都不注入，快照里的
// `Frames` 完全来自宿主 journal。正式行为不依赖它，也不能依赖它。

// demoWorkflowFlag 是**纯调试**开关（见上）。
var demoWorkflowFlag = flag.Bool("demo-workflow", false,
	"调试：往 journal 塞一组假的 tool-workflow/* 帧（只用于截图/验收；正式路径不看它）")

// demoWorkflowBaseTime 是固定基准毫秒。
//
// **必须固定**：状态栏渲染 `186 tok/s`（= outputTokens ÷ 相邻帧墙钟差），
// 消息行渲染 `01:07`。任何一处拿实时钟都会让两次截图差出像素。
const demoWorkflowBaseTime int64 = 1_700_000_000_000

// demoWorkflowSpec 是一帧的字面量（kind + data 原文）。
type demoWorkflowSpec struct {
	kind string
	data string
}

// demoWorkflowSpecs 是这一组帧的**全部内容**（可读可审）。
//
// 覆盖的形态（截图要能一眼看出三级嵌套 + StateDot 四色全部画出来）：
//
//	run    review-and-fix：completed（run-end 说 completed）
//	├─ review      abnormal（有一个 failed 成员 → 强制展开）
//	│  ├─ seq1 读改动并列出风险点  completed  → StateDot done   （绿）
//	│  └─ seq2 跑一遍全量测试      failed     → StateDot error  （红）
//	└─ implement   abnormal（有一个 cancelled 成员 → 强制展开）
//	   ├─ seq3 按风险点改代码      completed  → StateDot done   （绿）
//	   └─ seq4 补回归测试          cancelled  → StateDot warning（黄）
//
// "ongoing（蓝色转圈）"那一档由"没有 agent-end 的 run"覆盖，不在这一张图里：
// 静态截图画不出旋转，它的几何由单测（TestWorkflowRunAllStatusesRender）钉住。
var demoWorkflowSpecs = []demoWorkflowSpec{
	{"turn/start", `{"turn":1}`},
	{"step/start", `{"turn":1,"step":1}`},
	{"user/message", `{"surfaceOp":"append","turn":1,"id":"demo-wf-user","source":{"kind":"user"},"content":[{"type":"text","text":"用工作流把这次改动的评审和修复都跑一遍。"}]}`},
	{"assistant/message", `{"turn":1,"step":1,"content":[{"type":"text","text":"我把这次改动拆成 review 与 implement 两个阶段并行跑。"}]}`},
	{"tool-workflow/run-start", `{"runId":"run-demo-1","name":"review-and-fix"}`},
	{"tool-workflow/agent-start", `{"runId":"run-demo-1","seq":1,"label":"读改动并列出风险点","phase":"review","childId":"child-demo-1"}`},
	{"tool-workflow/agent-start", `{"runId":"run-demo-1","seq":2,"label":"跑一遍全量测试","phase":"review","childId":"child-demo-2"}`},
	{"tool-workflow/agent-start", `{"runId":"run-demo-1","seq":3,"label":"按风险点改代码","phase":"implement","childId":"child-demo-3"}`},
	{"tool-workflow/agent-start", `{"runId":"run-demo-1","seq":4,"label":"补回归测试","phase":"implement","childId":"child-demo-4"}`},
	{"tool-workflow/agent-end", `{"runId":"run-demo-1","seq":1,"outcome":"completed"}`},
	{"tool-workflow/agent-end", `{"runId":"run-demo-1","seq":2,"outcome":"failed"}`},
	{"tool-workflow/agent-end", `{"runId":"run-demo-1","seq":3,"outcome":"completed"}`},
	{"tool-workflow/agent-end", `{"runId":"run-demo-1","seq":4,"outcome":"cancelled"}`},
	{"tool-workflow/run-end", `{"runId":"run-demo-1","stopReason":"completed"}`},
	{"step/end", `{"turn":1,"step":1}`},
	{"turn/end", `{"turn":1,"reason":{"kind":"completed"}}`},
}

// demoWorkflowFrames 把 demoWorkflowSpecs 展开成 journal 帧（seq 从 1 起）。
//
// 时间轴固定：第 i 帧 = baseTime + i×1000。**不用实时钟**（见 baseTime 注释）。
func demoWorkflowFrames() []apiclient.Frame {
	frames := make([]apiclient.Frame, 0, len(demoWorkflowSpecs))
	for i, spec := range demoWorkflowSpecs {
		frames = append(frames, apiclient.Frame{
			Seq:  i + 1,
			Type: spec.kind,
			Data: json.RawMessage(spec.data),
			Time: demoWorkflowBaseTime + int64(i+1)*1000,
		})
	}
	return frames
}

// injectDemoWorkflowLocked 把 demo 帧并入会话 journal。调用方必须持有 m.mu。
//
// 幂等：`frameJournal.insert` 按 seq 去重，重复注入不会多出帧。
// seq 必须 > 0 且严格递增（journal 是 append-only 的有序表）。
func (m *model) injectDemoWorkflowLocked(id string) {
	if !*demoWorkflowFlag || id == "" {
		return
	}
	for _, frame := range demoWorkflowFrames() {
		m.appendFrameLocked(id, frame)
	}
	// 水位跟着走：`session.history` 补洞与"已加载 N 条"都读它。
	if journal := m.frames[id]; journal != nil {
		if journal.lastSeq > m.seqs[id] {
			m.seqs[id] = journal.lastSeq
			if id == m.cur {
				m.lastSeq = journal.lastSeq
			}
		}
	}
}
