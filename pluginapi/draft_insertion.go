package pluginapi

// KeyDraftInsertion 是「草稿插入」能力的键：由 composer 提供，别人 Inject。
//
// 官方把这两个动词放在**会话输入门面**上（`inputActions`），不是放在平台能力里：
// `OFF:dsh-client-ui-conversation/src/client/input/facade.js` 的
// `SessionInputShell.actions`（编译产物 `tmp/dsh-client-ui-conversation.js:14942-14950`）。
// 消费方是 `dsh-experimental-client-ui-voice-input`
// （`VoiceInput.tsx:122,133,177`：`captureInsertion()` / `insertText(text, span)`）。
//
// # 为什么本仓必须显式开一条通道
//
// 听写要插在**采集时的光标处**，而草稿的编辑器在 composer 插件手里
// （`plugins/dscomposer/composer.go` 的 `ui.Field`）。`pluginapi` 原来只经
// `Fields()`（`composer.go` 的 `Fields`）把 field 交给壳做键盘路由，
// 没有"读光标 / 按位置插字"的入口——所以听写只能追加到末尾
// （`pluginapi.DraftInsert` 的旧注释）。这条通道就是补上那两个动词。
const KeyDraftInsertion = "ui.draft.insertion"

// InsertionSpan 是官方 `captureInsertion()` 的快照。
//
// 官方形状（`facade.js` 的 `captureInsertion`）：
//
//	captureInsertion: () => ({...this.caretSpan(), draftRev: this.rev})
//
// 其中 `caretSpan()` 是**选区非空就用选区、否则光标落在正文末尾**
// （编译产物 `tmp/dsh-client-ui-conversation.js:14745-14752`）。
//
// Start / End 是**rune 下标**（不是字节）：官方是 Lexical 的 UTF-16 offset，
// 本仓草稿编辑器是 viva `text.Editor`，它的 `Selection()` / `Caret()` 都是 rune 下标
// （`viva@0.6.5/text/editor.go:212,218`）。两边都按"字符"数，语义一致。
type InsertionSpan struct {
	// Start / End 是采集时的选区（End==Start 表示只有光标）。
	Start int
	End   int
	// DraftRev 是采集那一刻的草稿版本号。
	//
	// 官方用它做**并发守卫**：`insertText` 里 `if (span.draftRev !== this.rev) return false`
	// （`tmp/dsh-client-ui-conversation.js:14948`）。识别要几秒，这期间用户可能已经
	// 改了草稿——那时把文字硬插到旧位置就会毁掉他刚写的东西。
	DraftRev int64
}

// DraftInsertion 是 composer 提供给其它插件的草稿插入能力（官方 `inputActions` 的两半）。
//
// 实现方：`plugins/dscomposer`（经 `Context.Provide(KeyDraftInsertion, …)`）。
// 取用方：壳（`cmd/app`，听写状态机）或任何需要"往草稿里插一段"的插件。
//
// 两个方法**都必须在 UI（窗口）线程调用**：它们直接动 viva 的 `text.Editor`
// （`viva@0.6.5/text/editor.go:267` 的 `Insert`），而 viva 的编辑器只能在窗口线程改。
// 官方是浏览器里同一线程的 DOM 编辑，没有这个约束，本仓有。
type DraftInsertion interface {
	// CaptureInsertion 返回当前光标 / 选区快照。false = 现在没有可插入的草稿
	// （输入框还没画过、插件没装）。
	CaptureInsertion() (InsertionSpan, bool)
	// InsertText 把 text 插到 span 处（**替换** span 区间），返回是否真的插进去了。
	//
	// false 的三条判据逐条对齐官方 `insertText`
	// （`tmp/dsh-client-ui-conversation.js:14946-14950`）：
	//
	//	① 会话正在提交 / 裁定中 → false（`phase === 'adjudicating' || 'submitting'`；
	//	   本仓对应 `Snapshot.Busy`）；
	//	② `span.DraftRev` 与当前版本不符 → false（草稿被改过）；
	//	③ 没有编辑器 → false。
	//
	// 失败时**调用方负责保留文字**（官方把识别结果存进 `pending` 并画「插入文字」，
	// 见 `VoiceInput.tsx:122,175-178`）——不能丢。
	InsertText(text string, span InsertionSpan) bool
}
