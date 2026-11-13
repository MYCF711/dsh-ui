// official_transcript_wiring.go —— 把 GT2 的 transcript 渲染器接进**产品绘制路径**。
//
// # 为什么这一层单独一个文件
//
// GT2 交付的是三层：节点表（`official_transcript_nodes.go`）、几何表
// （`official_transcript_geometry.go`）、投影 + 绘制
// （`official_transcript_projection.go` / `official_transcript_view.go`）。
// 三层都是纯函数，**没有一处碰快照**——所以它们写完之后，真机上主区依然是
// 旧渲染器画的那一份。本文件只做接线：
//
//	pluginapi.Snapshot.Frames ──增量──> Transcript 投影 ──> OfficialTranscriptView.Draw
//
// # 两个座位（同一帧只有一个活）
//
//	conversation.view   官方 `ChatView.column`（`uikernel.OffConvView`）—— rc.2 终态
//	main.body           dsshell 三栏 / 官方骨架的**旧部件兼容桥** —— 今天的产品路径
//
// 为什么两个都要挂：官方骨架在旧部件还占着 legacy 槽时会整段换成旧三栏排版
// （`uikernel.ShellOptions.BridgeLegacy`），那一帧只 dispatch `main.body`；
// 反过来官方槽全量 dispatch 时 `main.body` 根本没人画。两个座位互斥，
// **不会双画**（同一个判据见 `uikernel.ShellTier.OfficialSlotsLive`）。
//
// # 数据面（为什么是原始帧，不是 messages）
//
// 帧来自 `Env.Data.Frames`（`cmd/app/frames.go` 保留的 journal 原文）：
// `user/message` 的 `surfaceOp` / `source.kind`、`turn/end` 的 `reason.kind`、
// `llm/retry` 的 `attempt/maxRetries` 这些**只有原始帧才有**。从
// `Snapshot.Msgs`（`session.load` 的投影）反推，steering / turn-trigger /
// manual-compaction / question-reply 这几种节点就再也分不出来——界面上不会
// 报错，只是少了几种行。
//
// # 不丢帧、不重复（三条判据）
//
//	① 同 seq 重放            → `Transcript.applyFrames` 按 lastSeq 丢弃
//	② journalEpoch 换代       → `Transcript.ApplyHistory` 丢掉旧基线重建
//	③ 切会话（Cur 变了）       → 这里显式 Reset（两个会话的 seq 从 1 起，
//	                            不 Reset 会把 B 的帧全当成 A 的重放丢掉）
package dsconversation

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// transcriptScrollSelf / transcriptScrollHost 是两个滚动口的交互 id 前缀。
//
// **必须分开**：viva 的 ScrollState 按 id 存，共用一个 id 会让两层拿到同一个
// Offset（滚动位置互相打架，官方 `use-chat-viewport.ts:118-119` 的判据就是
// "外层有宿主滚动口就用它，没有就自己滚"）。
const (
	transcriptScrollHost = "transcript-scroll-host-"
	transcriptScrollSelf = "transcript-scroll-self-"
)

// drawProductTranscript 是**产品绘制入口**：把快照里的原始帧投影成节点后交给
// 官方 transcript 渲染器。返回是否真的画了（false = 没有帧，调用方走旧路径）。
//
// hostScroll 区分两个座位：
//
//	true  → 外层已经有宿主滚动口（官方骨架的 `ConversationRoot.scrollBody`），
//	        自滚动口降级 `overflow-y:visible`（官方 ChatView.module.css:57-64）
//	false → 自滚动口自己滚（旧三栏 / 兼容桥里的 `main.body`）
func (p *Plugin) drawProductTranscript(f ui.C, env *pluginapi.Env, hostScroll bool) bool {
	if f == nil || env == nil {
		return false
	}
	if len(env.Data.Frames) == 0 {
		// 还没有任何原始帧。两种情形**必须分开**：
		//
		//	① 首屏基线还在路上，且这个会话本来就有 load 投影出来的消息
		//	   → 官方此时画 `.hint`「正在加载历史…」（`ChatView.tsx:265`），
		//	     这就是分页五态里的「首次」。接管，让用户看到"在加载"而不是干等。
		//	② 空白会话（Hero）/ 老宿主 / mockd（根本没有分页信息）
		//	   → 回退旧渲染路径：宁可画旧的消息列表，也不给用户一个空白主区。
		//
		// 判据不能只看 Loading：空白会话打开瞬间也在拉基线，那时把 Hero 换成
		// 一行「正在加载历史…」是一次没必要的闪烁。`Msgs` 非空 = 这个会话
		// 本来就有内容，才值得为它显示加载态。
		starting := env.Data.History.Loading && !env.Data.History.Baseline && len(env.Data.Msgs) > 0
		if !starting {
			return false
		}
	}
	entries := p.syncTranscript(env)
	viewport := TranscriptViewport{HostScroll: hostScroll}
	if env.Tree != nil {
		viewport.HostID = env.Tree.ID(env.Feat, transcriptScrollHost+env.Data.Cur)
		viewport.SelfID = env.Tree.ID(env.Feat, transcriptScrollSelf+env.Data.Cur)
	}
	p.OfficialTranscript().Draw(f, env, viewport, entries)
	return true
}

// drawOfficialTranscriptSeat 是官方 `conversation.view` 座位（rc.2 终态）。
//
// 这一帧外层已经有宿主滚动口（骨架的 `.scrollBody[data-conversation-scroll]`），
// 所以自滚动口降级——两层都滚会让滚动位置打架。
func (p *Plugin) drawOfficialTranscriptSeat(f ui.C, env *pluginapi.Env) {
	p.drawProductTranscript(f, env, true)
}

// drawLegacyTranscriptSeat 是旧三栏 / 兼容桥的 `main.body` 座位。
//
// 没有宿主滚动口，自滚动口自己滚（官方"独立挂载 Chat"的情形）。
func (p *Plugin) drawLegacyTranscriptSeat(f ui.C, env *pluginapi.Env) {
	p.drawProductTranscript(f, env, false)
}

// afterDrawOfficialTranscriptSeat 是官方 `conversation.view` 座位的**覆画阶段**。
//
// 只做一件事：把布局阶段占好位的代码块按真实矩形画出来（banner 语言标签 /
// 语法高亮 / 横滚视口 / 复制按钮）。为什么不在 Draw 里画：viva 的布局是
// 「先声明整棵树、再一次性 End() 解算」，Draw 回调里 `Box.Rect()` 还是零值，
// 而 `mdlite.CodeBlock` 要的正是真实矩形（`pluginapi.Painter.AfterDraw` 的
// 注释写的就是这件事）。
func (p *Plugin) afterDrawOfficialTranscriptSeat(canvas *gfx.Canvas, env *pluginapi.Env) {
	p.paintOfficialTranscriptCodeBlocks(canvas, env)
}

// paintOfficialTranscriptCodeBlocks 覆画官方 transcript 的代码块。
//
// 两个座位共用：`conversation.view`（rc.2 终态）与 `main.body`（三栏/兼容桥）
// 都可能承载官方 transcript（判据是"这一帧有没有原始帧"）。渲染器**自己按
// 帧号去重**（`OfficialTranscriptView.PaintCodeBlocks`），所以两边都调不会双画。
func (p *Plugin) paintOfficialTranscriptCodeBlocks(canvas *gfx.Canvas, env *pluginapi.Env) {
	if p.officialTranscript == nil {
		return
	}
	p.officialTranscript.PaintCodeBlocks(canvas, env)
}

// syncTranscript 把快照里的原始帧**增量**并入投影，返回当前节点。
//
// 三条判据按顺序生效（详见文件头）：
//   - 切会话 → Reset（两个会话的 seq 都从 1 起，不 Reset 会把新会话的帧全丢掉）
//   - journalEpoch 换代 → `ApplyHistory` 内部丢旧基线重建
//   - 同 seq 重放 → `applyFrames` 按 lastSeq 丢弃
func (p *Plugin) syncTranscript(env *pluginapi.Env) []TranscriptEntry {
	if p.transcript == nil {
		p.transcript = NewTranscript()
	}
	if p.transcriptSession != env.Data.Cur {
		// 切会话：投影是**会话内**的节点序列，跨会话复用等于把 A 的行贴到 B 上。
		p.transcript.Reset()
		p.transcriptSession = env.Data.Cur
	}
	frames := env.Data.Frames
	// 交给 `ApplyHistory` 而不是自己判世代：投影层的判据（两处都非 0 且不等）
	// 是**唯一**出处，这里再写一遍迟早会漂。
	res := &apiclient.HistoryResult{Frames: frames, JournalEpoch: int(env.Data.JournalEpoch)}
	if n := len(frames); n > 0 {
		res.LastSeq = frames[n-1].Seq
	}
	p.transcript.ApplyHistory(res)
	return p.transcript.Entries()
}

// TranscriptEntries 返回当前已投影的节点（诊断 / 测试用）。
func (p *Plugin) TranscriptEntries() []TranscriptEntry {
	if p.transcript == nil {
		return nil
	}
	return p.transcript.Entries()
}

// transcriptRowKeys 是这一帧真实挂载的行键（顺序 = journal 顺序）。
//
// 单独暴露是为了让验收能断言"**这一帧真画了**哪些节点"，而不是只断言
// "投影里有几条"——投影对、绘制没接上，正是本任务要干掉的那种假通过。
func (p *Plugin) transcriptRowKeys() []string {
	if p.officialTranscript == nil {
		return nil
	}
	rows := p.officialTranscript.Boxes.Rows
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}

// transcriptRowText 是这一帧挂载行的 kind 表（诊断 / 测试用）。
func (p *Plugin) transcriptRowKinds() []ChatNodeKind {
	if p.officialTranscript == nil {
		return nil
	}
	rows := p.officialTranscript.Boxes.Rows
	kinds := make([]ChatNodeKind, 0, len(rows))
	for _, row := range rows {
		kinds = append(kinds, row.Kind)
	}
	return kinds
}

// 编译期断言：两个座位都是 `pluginapi.Painter.Draw` 的形状。
var (
	_ func(ui.C, *pluginapi.Env) = (*Plugin)(nil).drawOfficialTranscriptSeat
	_ func(ui.C, *pluginapi.Env) = (*Plugin)(nil).drawLegacyTranscriptSeat
	_                            = gfx.ID{}
)
