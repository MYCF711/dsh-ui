package dsfiles

import (
	"math"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// drag_files.go —— 官方 `dsh-client-ui-file-drag` 的**拖出**（把文件从我们窗口拖到
// Explorer / 别的应用），接在 Viva 的 `app.StartFileDrag` 上。
//
// # 官方两条通道，本仓只有一条能落地
//
// OFF:fd/src/client/transfers.ts:32-118 是桌面桥：先 `prepare` 记住要拖什么，用户
// 越过拖拽阈值时 `start` 才真正发起 OS 拖拽，松手 `release`、取消 `cancel`。
// OFF:fd/src/client/transfers.ts:120-155 是浏览器通道：`DataTransfer.setData('DownloadURL', …)`
// 加应用内 token，由 Chromium 落到资源管理器。
//
// Go 原生界面没有 DOM `DataTransfer`，第二条**不适用**（如实登记，不做假的）。
// 第一条直接对应 Viva 的 OLE 拖出：`app.StartFileDrag(paths, effects)` 在窗口线程上
// 跑 `DoDragDrop`，返回目标真正接受的 effect。所以这里按桌面桥的三个阶段接线：
//
//	按下文件行           = prepare（记住被拖的绝对路径）
//	按住移动超过 4px      = start（调用 app.StartFileDrag，阻塞到拖拽结束）
//	松手 / 指针离开窗口   = release/cancel（清状态）
//
// # 为什么状态在插件里、发起在壳的 OnPointer 里
//
// 只有画文件行的这一段知道"这一行的命中身份 ↔ 绝对路径"（`dirPath` 那一处拼出来的
// 同一份路径）。而在页面绘制里直接调 `DoDragDrop` 会让绘制本身嵌进拖拽的消息循环，
// 所以插件只把按下记下来，由壳的 `app.Config.OnPointer`（与 OLE 拖拽循环同一条
// 窗口线程、且在页面绘制之外）在指针移动时发起。壳不认识文件行，插件认识。
//
// # 阈值
//
// 官方阈值由浏览器给（`draggable`）；这里得自己拿一个 4px，否则"点一下文件看预览"
// 会全部变成拖拽——按下不越过阈值就什么都不做，点击照旧。

// fileDragThreshold 是"按下"变成"拖出"的位移阈值（像素，逻辑坐标）。
//
// 与 dssidebar 的会话行拖拽同一个数、同一个理由：没有阈值，点一下文件行就等于开始
// 拖拽，"点开预览"会全部失效。
const fileDragThreshold = 4

// fileDragEffects 是文件拖出提供的下场效果：复制与移动。
//
// 官方的桌面桥按 `effectAllowed` 给，浏览器通道写死 `copy`（transfers.ts:148）。
// 这里给"复制 + 移动"由目标选：资源管理器默认按 Ctrl/Shift 与源卷决定，用户不按
// 修饰键时同卷是移动——和从资源管理器自己拖出来一致。链接不给：CF_HDROP 里没有
// shell link，给了也是空承诺。
const fileDragEffects = app.FileDragCopy | app.FileDragMove

// startFileDrag 是发起 OS 拖出的唯一出口。
//
// 测试替换它（真实拖拽没法在单测里按鼠标），真机走 `app.StartFileDrag`。
var startFileDrag = app.StartFileDrag

// filePress 是一次按下、还没变成拖出的状态。零值 = 没在按。
type filePress struct {
	pressing bool
	path     string  // 被按下的文件（绝对路径）
	x, y     float32 // 按下时的逻辑坐标，判阈值用
}

// moved 报告指针是否已经离开按下点足够远，也就是"按下"该变成"拖出"了。
func (press filePress) moved(x, y float32) bool {
	return math.Hypot(float64(x-press.x), float64(y-press.y)) >= fileDragThreshold
}

// dragRows 把本帧每个文件行的命中身份映射到它的绝对路径。
//
// 每帧重写（目录换了、文件列表变了，身份就变了），只由绘制的那个线程读写。
func (p *Plugin) dragRows(rows map[gfx.ID]string) { p.draggable = rows }

// OnPointer 是壳的 `app.Config.OnPointer` 转进来的指针事件，返回是否消费（消费 = 要求重画）。
//
// 它必须跑在窗口线程上、且在页面绘制之外：拖出会在这里阻塞到用户松手。
// 不认识的身份（不是文件行）、非主键、以及没按下的移动都直接放行，页面的点击/
// 悬停/预览照旧走运行时那条路。
func (p *Plugin) OnPointer(event gfx.PointerEvent) bool {
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary {
			return false
		}
		path, ok := p.draggable[event.Target]
		if !ok || path == "" {
			return false
		}
		p.press = filePress{pressing: true, path: path, x: event.X, y: event.Y}
		p.dragErr = ""
		return false // 按下还要照常选中/预览这一行
	case gfx.PointerMove:
		if !p.press.pressing || !p.press.moved(event.X, event.Y) {
			return false
		}
		path := p.press.path
		p.press = filePress{}
		// 拖出期间窗口线程在这里面（Viva 的 DoDragDrop 自己跑模态循环）。
		effect, err := startFileDrag([]string{path}, fileDragEffects)
		if err != nil {
			// 平台不支持拖出，或者拿不到窗口：写清楚，不假装成功。
			p.dragErr = err.Error()
			return true
		}
		_ = effect
		return true
	case gfx.PointerUp, gfx.PointerExit:
		p.press = filePress{}
	}
	return false
}

// dragging reports whether a press is waiting to become a drag. Tests read it;
// the paint path does not.
func (p *Plugin) dragging() bool { return p.press.pressing }
