// official_transcript_paging.go —— 主区 transcript 的**向前分页**（`.older` 单按钮五态）。
//
// # 为什么单独一层
//
// 历史窗口化官方**靠分页、不靠虚拟化**（`ChatView.module.css:100-104` 的注释：
// 虚拟化是未来选项，当前未启用；`ChatView.tsx:253-273` 是那一行按钮的真值）。
// 所以 transcript 顶部有一条**独立于消息节点**的行：它不属于 `ChatFlow` 的
// 任何 kind，画在 `.column` 里、flow 槽之前。
//
// # 官方五态（逐字）
//
//	openState === 'loading'                  → `.hint`「正在加载历史…」     chat.loadingHistory
//	!hasMore                                 → 'start'   「已到会话开头」（disabled）
//	loadingOlder                             → 'loading' 「正在加载」（disabled）
//	historyLoadStatus === 'error'            → 'error'   「重新加载」（可点 = 重试）
//	其余                                      → 'more'    「加载更早」（可点）
//
// 行本身只在 `(hasMore || historyWasPartial)` 时渲染（`ChatView.tsx:265`）——
// **一页装得下的短会话不画这一行**，否则界面上永远挂着一条「已到会话开头」。
//
// # 几何（`ChatView.module.css:325-355`，rc.2 实读）
//
//	.older            { overflow-anchor:none; display:flex; justify-content:center }
//	.older button     { display:grid; line-height:18px; white-space:nowrap;
//	                    border:none; border-radius:var(--dsw-radius-sm);
//	                    padding:4px 12px; font-size:12px;
//	                    color:var(--dsw-alias-label-secondary);
//	                    background:var(--dsw-alias-interactive-bg-hover-solid);
//	                    cursor:pointer }
//	.older button:disabled { cursor:default; opacity:0.6 }
//	.hint             { color:var(--dsw-alias-label-tertiary); font-size:13px;
//	                    line-height:calc(18px + delta-secondary) }
//
// # 一条不编
//
// 五态的**判据全部来自 `pluginapi.HistoryPaging`**（壳从 `session.history`
// 的分页结果算出来），这里不猜"应该还有更多"、不自己数帧。
package dsconversation

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 官方 `.older` / `.hint` 的几何（`ChatView.module.css:212-216` / `:325-355`）。
const (
	// OlderButtonLineHeight 是按钮行高（官方 `line-height:18px`）。
	OlderButtonLineHeight float32 = 18
	// OlderButtonFontSize 是按钮字号（官方 `font-size:12px`）。
	OlderButtonFontSize float32 = 12
	// OlderButtonPadY / OlderButtonPadX 是按钮内边距（官方 `padding:4px 12px`）。
	OlderButtonPadY float32 = 4
	OlderButtonPadX float32 = 12
	// HistoryHintFontSize / HistoryHintLineHeight 是 `.hint` 的字号行高
	// （官方 `13px` / `calc(18px + delta-secondary)`）。
	HistoryHintFontSize   float32 = 13
	HistoryHintLineHeight float32 = 18
)

// HistoryLoadState 是「加载更早」那一行此刻的**五态**。
//
// 为什么要有 HistoryHidden：官方那一行是**条件渲染**的（`(hasMore ||
// historyWasPartial)`），"不画"本身是一个状态——一页装得下的短会话、
// 老宿主没给分页信息、还没打开会话，三种都是它。
type HistoryLoadState int

const (
	// HistoryHidden 这一帧不画（短会话一页装得下 / 老宿主没给分页信息）。
	HistoryHidden HistoryLoadState = iota
	// HistoryFirst 首次：首屏基线还在拉（官方 `.hint`「正在加载历史…」）。
	//
	// 它与「加载中」不是一回事：那一条是**基线**（第一页），这一条是用户点了
	// 「加载更早」之后的**翻页**。官方把它们画成两种东西（`.hint` / `.older`），
	// 因为基线没到时下面连消息都还没有，按钮没有可点的位置。
	HistoryFirst
	// HistoryMore 有更多：还有更早的帧没拉（按钮「加载更早」，可点）。
	HistoryMore
	// HistoryLoading 加载中：正在拉更早一页（按钮「正在加载」，disabled）。
	HistoryLoading
	// HistoryStart 无更多：已到可加载的最早处（按钮「已到会话开头」，disabled）。
	HistoryStart
	// HistoryFailed 失败：最近一次分页失败（按钮「重新加载」，可点重试）。
	HistoryFailed
)

// String 是诊断/测试用的名字（不参与绘制）。
func (s HistoryLoadState) String() string {
	switch s {
	case HistoryFirst:
		return "first"
	case HistoryMore:
		return "more"
	case HistoryLoading:
		return "loading"
	case HistoryStart:
		return "start"
	case HistoryFailed:
		return "error"
	default:
		return "hidden"
	}
}

// historyLoadState 把快照里的分页状态算成五态之一。
//
// 判据顺序与官方 **逐字一致**（`ChatView.tsx:266-273`）：先看有没有更多，
// 再看是不是在加载，再看有没有失败，最后落到「有更多」。
// 注意官方是先判 `!hasMore`（'start'）再判 `loadingOlder`——所以到头的会话
// 即使有个迟到的 loading 也只会显示「已到会话开头」，不会闪一下「正在加载」。
func historyLoadState(p pluginapi.HistoryPaging) HistoryLoadState {
	if !p.Baseline {
		if p.Loading {
			return HistoryFirst
		}
		// 没有任何分页信息（老宿主 / mockd / 还没打开会话）：整行不画。
		return HistoryHidden
	}
	switch {
	case !p.HasMore:
		if !p.Partial {
			// 一页就装下了：官方不画这一行（否则永远挂着「已到会话开头」）。
			return HistoryHidden
		}
		return HistoryStart
	case p.Loading:
		return HistoryLoading
	case p.Error != "":
		return HistoryFailed
	default:
		return HistoryMore
	}
}

// historyLoadLabel 是五态各自的官方文案（`dsh-client-ui-chat/src/client/locale.ts:94-98`）。
func historyLoadLabel(state HistoryLoadState) string {
	switch state {
	case HistoryFirst:
		return "正在加载历史…"
	case HistoryMore:
		return "加载更早"
	case HistoryLoading:
		return "正在加载"
	case HistoryStart:
		return "已到会话开头"
	case HistoryFailed:
		return "重新加载"
	default:
		return ""
	}
}

// historyLoadClickable 报告这一态的按钮**能不能点**（官方 `disabled={!hasMore}`
// 加上失败态可点重试）。
func historyLoadClickable(state HistoryLoadState) bool {
	return state == HistoryMore || state == HistoryFailed
}

// historyButtonID 造一个稳定的按钮交互 id。
//
// 按会话起名而不是自增：同一个会话在两次渲染里必须给**同一个** id，
// 否则 viva 会把按钮当成新节点重挂（焦点/悬停状态每一帧都丢）。
func (v *OfficialTranscriptView) historyButtonID(env *pluginapi.Env) gfx.ID {
	if env == nil || env.Tree == nil {
		return gfx.ID{}
	}
	return env.Tree.ID(env.Feat, "history-older")
}

// drawHistoryRow 画「加载更早」那一行，返回这一行的盒子与五态。
//
// 返回零值盒子 = 这一帧不画（`HistoryHidden`）。
func (v *OfficialTranscriptView) drawHistoryRow(f ui.C, env *pluginapi.Env, p pluginapi.HistoryPaging) (ui.Box, HistoryLoadState) {
	state := historyLoadState(p)
	if state == HistoryHidden {
		return ui.Box{}, state
	}
	t := v.Theme
	if state == HistoryFirst {
		// 官方 `.hint`：13/18、三级色、不是按钮（基线没到时点它没有意义）。
		box := ui.Row(f).FillWidth()
		box.Children(func() {
			ui.Text(f, historyLoadLabel(state)).
				FontSize(HistoryHintFontSize).
				LineHeight(HistoryHintLineHeight).
				Color(t.LabelTertiary)
		})
		return box, state
	}
	// 官方 `.older`：整行居中 + 一个按钮。
	box := ui.Row(f).FillWidth().Justify(ui.Center)
	clickable := historyLoadClickable(state)
	var clicked bool
	box.Children(func() {
		button := ui.Button(f, historyLoadLabel(state)).
			Small().
			Radius(t.Radii.SM).
			Background(t.Hover).
			PadSides(OlderButtonPadY, OlderButtonPadX, OlderButtonPadY, OlderButtonPadX)
		if !clickable {
			button = button.Disabled(true)
		}
		clicked = button.Clicked()
	})
	// 禁用态**再判一次**：viva 的 Disabled 只影响绘制，命中测试不一定认，
	// 而"点了没反应"比"点了发请求"更容易被当成 bug（审批按钮同款写法）。
	if clicked && clickable && env != nil && env.Act.LoadOlder != nil {
		env.Act.LoadOlder()
	}
	return box, state
}
