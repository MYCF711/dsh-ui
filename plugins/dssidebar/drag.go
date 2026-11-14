// drag.go —— #62 侧栏会话行的拖拽排序（官方 `workspace.insertSessionBefore` 的消费者）。
//
// # 官方证据（唯一来源）
//
//	OFF:ui-workspace/src/client/rows/Rows.tsx:588-700
//	  会话行整行可拖：`draggable = drag !== undefined && reorderable && !blank && !archived`，
//	  reorderable = 该会话的 `workspace/insertSessionBefore` 访问权。
//	  onDragOver 用 rowHalf 报"落在这行的上半区还是下半区"，onDrop 把半区交给
//	  drag 属主，onDragEnd 提交最后一次悬停的落点。
//	OFF:ui-workspace/src/client/rows/Rows.tsx:215-218
//	  rowHalf：`clientY < rect.top + rect.height / 2 ? 'before' : 'after'`。
//	OFF:ui-workspace/src/client/rows/WorkspaceBrowser.tsx:218-238
//	  sessionDragOrder：抽掉源行、按落点插回、再按"插回后紧跟它的那一条"做锚点。
//	OFF:ui-workspace/src/client/rows/WorkspaceBrowser.tsx:415-428,769-780
//	  提交在 dragend，用的是 `drag.over`（最后一次悬停的落点），不是松手坐标；
//	  同组才允许落（跨组/跨置顶块的落点不成立）。
//	OFF:ui-workspace/src/client/rows/Rows.module.css:370-408
//	  落点视觉：行上/下缘外侧 2px 处的 2px 横线（见 drawDropMarker 的换算）。
//
// # 本仓做不到、如实记下的两处
//
//   - 官方整行是 HTML5 `draggable`，启动阈值由浏览器给；viva 没有原生拖拽，这一档
//     只能自己拿一个 4px 阈值（dragStartThreshold），否则"点一下会话"会全部变成拖拽。
//   - 官方还禁止拖 blank / archived 行（OFF:rows/Rows.tsx:629-634：`!row.blank &&
//     !row.archived`）。`apiclient.Session` 里没有这两个真值（只有 ID/Title/Cwd/
//     UpdatedAt/Mode），所以这一道门没得判——**不猜状态**，所有行一视同仁。
package dssidebar

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// dragStartThreshold 是"按下"变成"拖动"的位移阈值（像素）。
//
// 没有它，点一下会话行就等于开始拖拽，那一下"点击打开会话"会全部失效
// （拖拽一开始我们就 Consume，运行时会把这个按下记成"已处理"，松手时不再产生点击）。
const dragStartThreshold = 4

// sessionDrag 是一次会话行的拖拽状态。
//
// 全是绘制/指针线程自己的状态，不进壳；`sessionDrag{}` 零值 = 没在拖。
type sessionDrag struct {
	pressing bool // 指针已按下、还没越过阈值（也可能就只是一次点击）
	dragging bool // 已经进入拖拽态
	// sessionID 是被拖的会话；workspaceID/order 是**发起组**的名册身份与手动顺序，
	// 拖拽期间不变（落点只在同一组内成立，见 deferDropTarget）。
	sessionID   string
	workspaceID string
	order       []string
	originY     float32 // 按下时的指针 Y：判阈值用
	// over 是最近一次解析出来的落点行；after 表示落点在那一行的下半区。
	// 松手时提交的是**它**（官方 dragend 语义），不是松手坐标。
	over  string
	after bool
}

func (d *sessionDrag) reset() { *d = sessionDrag{} }

// groupDragContext 是"本帧正在画的那条组"在拖拽上需要的全部事实。
//
// drawList 在画每一组的行之前摆好它，drawSession 只读。整组算齐是必须的：
// 落点可能落在**还没画到**的那一行上，而落点解析发生在整帧画完之后
// （见 deferDropTarget），所以命中 id 表不能边画边攒。
type groupDragContext struct {
	// workspaceID 非空 = 这条组背后是 registry 里真实存在的工作区（有写路径）。
	workspaceID string
	order       []string // 这一组的手动顺序（registry sessionIds 的顺序）
	hitIDs      []gfx.ID // 与 order 一一对应的行命中 id
}

// sessionRowHitIDs 列出这一组每一行的命中 id。
//
// 与 drawSession 里给 ListRow 的 id 是同一口径（`s-<会话id>`）：由会话 id 决定，
// 所以可以在画之前把整组算齐。
func sessionRowHitIDs(env *pluginapi.Env, sessions []apiclient.Session) []gfx.ID {
	if env == nil || env.Tree == nil {
		return nil
	}
	out := make([]gfx.ID, 0, len(sessions))
	for i := range sessions {
		out = append(out, env.Tree.ID(env.Feat, "s-"+sessions[i].ID))
	}
	return out
}

// handleSessionDragPointer 是会话行上的指针事件入口。
//
// 只有**可拖的组**里的会话行才走到这里（drawSession 的 draggable 判据），
// 不可拖的行连 `PointerOn` 都不问：运行时就收不到这条行的事件，于是"不能拖"
// 和"拖了没反应"在实现上也是两件事（契约：壳没接写动作时侧栏不进入拖拽态）。
//
// viva 指针模型与 plugins/dsshell 的把手同一条路（shell.go:426-460）：
// PointerDown 只送到指针正下方的命中区；按下之后指针被**捕获**，PointerMove/Up
// 一直送到按下的那个 id，指针离开这一行也照样送。
func (p *Plugin) handleSessionDragPointer(env *pluginapi.Env, rowID gfx.ID, sessionID string) {
	c := env.Canvas
	if c == nil {
		return
	}
	ev, ok := c.PointerOn(rowID)
	if !ok {
		return
	}
	switch ev.Kind {
	case gfx.PointerDown:
		// 只认主键：右键那一下是上下文菜单，不该开始拖。
		if ev.Button != gfx.PointerButtonPrimary && ev.Button != gfx.PointerButtonNone {
			return
		}
		// 已经在拖（或已按下）另一条：不抢。真机上"按住左键再按右键"会送来第二个
		// PointerDown，放过去的话正在拖的那条会突然换目标。
		if p.drag.pressing || p.drag.dragging {
			return
		}
		p.drag = sessionDrag{
			pressing:    true,
			sessionID:   sessionID,
			workspaceID: p.rowGroup.workspaceID,
			order:       p.rowGroup.order,
			originY:     ev.Y,
		}
		// 这里**故意不 Consume**：这一下可能只是"点开这个会话"。Consume 会让运行时
		// 把这次按下记成已处理，松手时那个点击就没了（viva
		// internal/interaction/pointer.go:52-57 的 NoteHandled）。
	case gfx.PointerMove:
		if p.drag.sessionID != sessionID || (!p.drag.pressing && !p.drag.dragging) {
			return
		}
		if !p.drag.dragging {
			if absFloat(ev.Y-p.drag.originY) < dragStartThreshold {
				return // 还没越过阈值：仍然可能是一次点击
			}
			p.drag.dragging, p.drag.pressing = true, false
		}
		c.Consume() // 真的在拖了：这一下不是点击（也让它立刻重绘，落点跟手）
		p.deferDropTarget(env, ev)
	case gfx.PointerUp:
		if p.drag.sessionID != sessionID {
			return
		}
		if !p.drag.dragging {
			p.drag.reset() // 只是点了一下：交回正常的点击路径
			return
		}
		c.Consume()
		drag := p.drag
		p.drag.reset()
		p.startSessionDrop(env.Act, drag)
	}
}

// rowBounds 报一条命中 id 在本帧的可见矩形（`gfx.Canvas.HitBounds` 的形状）。
type rowBounds func(gfx.ID) (gfx.Rect, bool)

// resolveDropRow 用刚画完那一帧的行矩形把指针 Y 折成落点：命中哪一行、落在
// 该行的上半区还是下半区。
//
// 半区判据照官方 rowHalf（OFF:rows/Rows.tsx:215-218）。**按几何找行**而不是看
// `Canvas.Hovered()`：悬停的命中可能是行**内部**的小控件（删除 ×、菜单按钮），
// 那时 hovered 不是行 id，落点会莫名其妙消失。
//
// 返回 (落点行 id, 是否下半区, 有没有落点)。落点不在这一组任何一行上 = 没有落点
// （跨组拖动因此自然不会落——命中表里只有发起组的行）。
//
// 一处保守的近似：`HitBounds` 报的是**可见部分**（被滚动裁掉的部分不在里面，
// gfx/canvas.go:1660-1670），所以一行只露出一半时，半区用的是那半行的中点。
// 落点仍然落在这一行上，只是"上/下半区"的界线跟着可见区走。
func resolveDropRow(hitIDs []gfx.ID, order []string, y float32,
	bounds rowBounds) (string, bool, bool) {
	for i, id := range hitIDs {
		if i >= len(order) {
			break
		}
		rect, ok := bounds(id)
		if !ok || rect.Width <= 0 || rect.Height <= 0 {
			continue // 这一行整行滚出裁剪区：不是可落点
		}
		if y < rect.Y || y >= rect.Y+rect.Height {
			continue
		}
		return order[i], y >= rect.Y+rect.Height/2, true
	}
	return "", false, false
}

// deferDropTarget 把"指针 Y → 落点行 + 半区"推迟到这一帧画完之后再算。
//
// 为什么必须推迟：viva 的矩形要等布局**和绘制**结束才有效（ui.Box.Rect 的注释
// "valid once the page has been laid out"），`gfx.Canvas.HitBounds` 报的是本帧
// **已经画过的**命中区（gfx/canvas.go:1660-1670）。在声明期算落点只能拿到一张
// 空的命中表。`Canvas.AfterPaint` 的注释正好写着这个用途——"changes what the
// paint is declaring (a drop that moves a row)"。
//
// 算出来的是**用户此刻看到的那一帧**的几何，不是上一帧的近似；结果存在
// p.drag.over / p.drag.after 上，供下一帧画落点指示、以及松手时提交。
func (p *Plugin) deferDropTarget(env *pluginapi.Env, ev gfx.PointerEvent) {
	group := p.rowGroup
	y, sessionID := ev.Y, p.drag.sessionID
	canvas := env.Canvas
	canvas.AfterPaint(func() {
		if !p.drag.dragging || p.drag.sessionID != sessionID {
			return // 这一帧里拖拽已经结束：不要再改动状态
		}
		target, after, ok := resolveDropRow(group.hitIDs, group.order, y, canvas.HitBounds)
		if !ok {
			// 落点没落在本组任何一行上（行间那 2px 空隙、列表外、别的组）：**保留上一次
			// 的落点**。官方 dragend 提交的就是 `drag.over` 这最后一次"落在行上"的落点，
			// 不是松手坐标，而且整份文件头写明"指针滑出列表不该被当成一次被拒绝的落点"
			// （OFF:rows/WorkspaceBrowser.tsx:121-123,769-772）。清掉反而会让指示线在
			// 行间空隙里一闪一闪。
			return
		}
		p.drag.over, p.drag.after = target, after
	})
}

// startSessionDrop 是松手那一下的入口。
//
// 写是**同步** HTTP（壳要等宿主回执，见 `Actions.WorkspaceInsertSessionBefore`
// 的契约），所以另起 goroutine，别把绘制线程堵在一次往返上——与工作区改名/删除
// 同一条规矩（workspace_actions.go 的 startWorkspaceRename）。
func (p *Plugin) startSessionDrop(act pluginapi.Actions, drag sessionDrag) {
	go p.commitSessionDrop(act, drag)
}

// commitSessionDrop 发**唯一**一次写请求：松手才写，不是每个 move 都写。
//
// 锚点用 `drag.over`（最后一次悬停的落点），照官方 dragend 语义
// （OFF:rows/WorkspaceBrowser.tsx:769-772；文件头注释写明"releasing outside the
// list must not be rendered as a rejected drop before dragend commits that last
// marker"）。
//
// 返回 false = **一次请求都没发**：没有落点、落点不产生顺序变化（含拖回原位）、
// 或者壳没接写面。返回 true = 请求发出去了（宿主接受还是拒绝看 err：拒绝时
// 行留在宿主真序上，并把错误交给壳的状态栏通道 Notify，**不翻成成功**）。
func (p *Plugin) commitSessionDrop(act pluginapi.Actions, drag sessionDrag) bool {
	if drag.workspaceID == "" || drag.over == "" {
		return false
	}
	before, changed := dropBeforeID(drag.order, drag.sessionID, drag.over, drag.after)
	if !changed {
		return false
	}
	insert := act.WorkspaceInsertSessionBefore
	if insert == nil {
		return false
	}
	if err := insert(drag.workspaceID, drag.sessionID, before); err != nil {
		if act.Notify != nil {
			act.Notify("会话排序失败：" + err.Error())
		}
		return true
	}
	// 成功后的重绘由壳负责：写动作的契约就是"同步写宿主 + 成功后壳刷新缓存"
	// （cmd/app/workspace_registry_wiring.go 的 insertSessionBefore 会 Invalidate）。
	return true
}

// drawDropMarker 画官方那根落点插入线——拖拽期间唯一新增的视觉。
//
// 几何照官方 `.sessionRow.dropBefore::before{top:-7px}` /
// `.dropAfter::after{bottom:-7px}`（OFF:rows/Rows.module.css:370-408）：
// 元素高 12px、里面那根线高 2px 且位于元素内 y=5..7，折到行盒上就是
// 「行上缘往上 2px」/「行下缘往下 2px」。横向：元素两侧各伸出
// `--dsh-sidebar-paint-inset`（=3px，SidebarRoot.module.css:13）再各内缩 4px，
// 净效果与行盒整宽相差 1px——viva 没有这层 3px 透明描边，所以按行盒整宽画。
// 颜色 `--dsw-alias-state-business-primary`，本主题里就是 Info（活动点同一个色）。
//
// 官方线左端还有一个 5×7 的箭头（两条 55°/125° 斜线拼的渐变图元）。viva 没有斜线
// 渐变图元，按"官方没有对应图元就不自造"的规矩**不画**，只留这根有官方出处的横线。
//
// 被拖的源行**不做任何特殊样式**：官方跟手的是浏览器原生 drag image
// （`.dragPreview` 是给 drag image 用的），源行本身没有第二套视觉。
func (p *Plugin) drawDropMarker(f ui.C, after bool) {
	line := ui.Row(f).Absolute().Left(0).Right(0).Height(2).Background(p.theme.Info)
	if after {
		line.Bottom(-2)
		return
	}
	line.Top(-2)
}

func absFloat(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
