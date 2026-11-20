// insertion.go —— composer 交出的「草稿插入」能力（官方 `inputActions` 的两半，#66 ①）。
//
// # 官方语义（两处原文，都是 rc.2 随包编译产物）
//
//	captureInsertion  tmp/dsh-client-ui-conversation.js:14942-14945
//	                  → {...caretSpan(), draftRev: this.rev}
//	  caretSpan       :14745-14752 → 选区非空就用选区，否则光标落在**正文末尾**（start=end=len）
//	insertText        :14946-14950
//	                  ① phase 是 adjudicating/submitting 或 disposed → false
//	                  ② span.draftRev !== this.rev → false
//	                  ③ 否则 insertAsyncText(span, text)
//	  insertAsyncText :14772-14779 → 清掉芯片占位符（REFERENCE_PLACEHOLDER_RE）后
//	                                 **替换** span 区间，返回是否落进了编辑器
//
// 消费方是听写（`OFF:dsh-experimental-client-ui-voice-input/src/client/VoiceInput.tsx:122`）：
//
//	if (!inputActions.insertText(result.value.text, active.span)) { setPending(...); feedback(conflict) }
//
// 也就是"插不进就留着，别丢"。
//
// # 本仓为什么以前做不了
//
// 听写结果原来直接追加到草稿末尾（`pluginapi.DraftInsert`）。原因不是 Viva 缺东西：
// `ui.Field.Bind(*text.Editor)`（`viva@0.6.5/ui/field.go:71`）+ `Editor.Selection()`
// （`text/editor.go:212`）/ `Caret()`（`:218`）/ `Insert`（`:267`）都在。缺的是**本仓的通道**：
// composer 的 field 只经 `Fields()` 交给壳做键盘路由，光标从来没被暴露过。
// 这个文件就是那条通道的实现端（契约在 `pluginapi/draft_insertion.go`）。
//
// # 线程
//
// 两个方法都直接动 `text.Editor`，只能在窗口（UI）线程调。壳的听写状态机在
// transcribe 回来的 goroutine 里，所以它**排队**、由 `applyPendingDraft` 在
// Paint 里落到这里（与 `writeDraft` 同一条规矩）。
package dscomposer

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/text"
)

// chipPlaceholder 是被清掉的芯片占位码位。
//
// 官方 `REFERENCE_PLACEHOLDER_RE = /[\uE100-\uE11D\uFFFC]/gu`
// （`tmp/dsh-client-ui-conversation.js:14550`）：`\uFFFC` 是 Object Replacement
// Character（Lexical 里代表一个芯片节点），`\uE100-\uE11D` 是官方私有区的引用芯片字形。
// 本仓草稿是纯文本、没有芯片节点，但识别结果里若混进这些码位（模型抄了带芯片的
// 草稿片段），官方会清掉——这里照做，语义逐字相同。
func stripChipPlaceholders(value string) string {
	if !strings.ContainsFunc(value, isChipPlaceholder) {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if !isChipPlaceholder(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isChipPlaceholder 报告这个码位是不是芯片占位符（同一个正则的逐个码位版本）。
func isChipPlaceholder(r rune) bool {
	return r == '\uFFFC' || (r >= '\uE100' && r <= '\uE11D')
}

// draftInsertion 是 `pluginapi.DraftInsertion` 的实现端（值类型，持有 composer 指针）。
type draftInsertion struct{ p *Plugin }

// CaptureInsertion 实现 pluginapi.DraftInsertion：返回当前光标 / 选区快照。
//
// 官方 `caretSpan()` 的两支：选区非空 → 选区；否则 → 光标在正文末尾
// （文本编辑器里"没有选区"就等于 `{start: caret, end: caret}`，两个分支合并成一支）。
func (d draftInsertion) CaptureInsertion() (pluginapi.InsertionSpan, bool) {
	editor := d.p.editor
	if editor == nil {
		// 还没 Bind（插件未装配）：没有可插入的草稿，如实说没有。
		return pluginapi.InsertionSpan{}, false
	}
	selection := editor.Selection()
	return pluginapi.InsertionSpan{
		Start:    selection.Start,
		End:      selection.End,
		DraftRev: d.p.draftRev,
	}, true
}

// InsertText 实现 pluginapi.DraftInsertion：把 text 插到 span 处（替换区间）。
//
// 守卫逐条对齐官方，**按本仓能观测到的状态**：
//
//	② span.DraftRev 与当前版本不符 → false（这条是真正防事故的：识别要几秒，
//	  这期间用户可能已经改了草稿，硬插到旧位置会毁掉他刚写的东西）
//	③ 没有编辑器 → false
//
// ① 官方还有一条"phase 是 adjudicating/submitting → false"。本仓没有那个提交
// 相位机：`Snapshot.Busy` 表示"整个 turn 在跑"，窗口比官方的瞬时相位长几个数量级，
// 拿它顶替会让**整个回合期间**都插不进去（官方只在提交那一瞬挡）。
// 所以这条不实现，只登记（`docs/official-parity.md` #66 行）。真正的风险由 ② 承担。
func (d draftInsertion) InsertText(value string, span pluginapi.InsertionSpan) bool {
	editor := d.p.editor
	if editor == nil {
		return false
	}
	if span.DraftRev != d.p.draftRev {
		return false
	}
	clean := stripChipPlaceholders(value)
	if clean == "" {
		// 官方这里会调 insertAsyncText("")：区间被替换成空串 = 删掉选区。
		// 听写结果为空串官方在上游就拦掉了（`text === ''` → `empty` 档），
		// 走到这里说明调用方传了空内容——按"没插进去"返回，避免静默删掉用户选区。
		return false
	}
	length := editor.RuneCount()
	start, end := span.Start, span.End
	if start > end {
		start, end = end, start
	}
	start = clampRune(start, length)
	end = clampRune(end, length)
	editor.SetSelection(text.Range{Start: start, End: end})
	editor.Insert(clean)
	return true
}

// clampRune 把下标夹进 [0, length]（官方 Lexical 的 range 映射失败时也是"不应用"，
// 本仓退化成一次夹取：越界说明调用方手里的 span 已经过期，夹到边界仍然安全）。
func clampRune(value, length int) int {
	if value < 0 {
		return 0
	}
	if value > length {
		return length
	}
	return value
}

// bindDraftEditor 造一个 editor 并 Bind 给 field（只在 Register 里调一次）。
//
// 为什么用空串起步而不是拿 field 现有内容：Register 在第一次 Text()/SetText 之前，
// 那时 field 里本来就是空的。真要有内容（会话切换灌草稿）都是后来的 SetText，
// 走的是同一个 editor，不会丢。
func (p *Plugin) bindDraftEditor() {
	p.editor = text.NewEditor("")
	p.lastText = p.editor.String()
	(&p.field).Bind(p.editor)
}

// syncDraftRev 在**每帧绘制开头**同步一次草稿版本号（官方 `this.rev`）。
//
// 为什么在绘制帧里数：viva 的 Field 没有"内容变了"的回调，草稿变化的唯一
// 可观测点就是每帧读一次内容。放在绘制开头（UI 线程）保证：
//
//	① 用户刚打的字在这一帧就被记进版本号；
//	② 壳/插件调 `SetText`（灌草稿、清空）同样算一次变化；
//	③ 本插件自己的 `InsertText` 也是变化——官方 applyEdit 之后同样会 +1。
func (p *Plugin) syncDraftRev() {
	if p.editor == nil {
		return
	}
	current := p.editor.String()
	if current == p.lastText {
		return
	}
	p.lastText = current
	p.draftRev++
}

// draftText 是当前草稿内容（只给本包用；壳走 Fields()）。
func (p *Plugin) draftText() string {
	if p.editor == nil {
		return ""
	}
	return p.editor.String()
}
