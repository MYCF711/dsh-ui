// official_models_catalog_drag.go —— 模型目录行的**拖放排序**入口（#41 最后一条）。
//
// # 官方语义（唯一来源，逐条 file:line）
//
//	OFF:ui-settings-models/src/client/ModelRow.tsx:104-127
//	  拖源**只有柄**：`draggable={!disabled}`、`onDragStart` 里
//	  `effectAllowed='move'` + `setData('application/x-dsh-model-order', position)`，
//	  并 `preview.start(event, currentTarget.closest('[data-row-key]'))` —— 预览取的是
//	  **整行**元素（不是柄）。`:126` 的图标是 `IconDragHandleOutlineRegular size={14}`。
//	OFF:ui-settings-models/src/client/ModelRow.tsx:81-101
//	  落点是**整行**（`.modelEntry` 上的 onDragOver/onDrop）：`canDrop` 才 preventDefault，
//	  `onOver(rowHalf(event))`、`onDrop(rowHalf(event))`；`onDragLeave` 在真正离开行时清落点。
//	OFF:ui-settings-models/src/client/ModelRow.tsx:46-49
//	  rowHalf：`clientY < rect.top + rect.height/2 ? 'before' : 'after'` —— 上半区插到本行之前，
//	  下半区插到本行之后。
//	OFF:ui-settings-models/src/client/useModelRows.ts:60-67
//	  `move(from, to)`：**唯一的移动实现**（splice 抽出一项再插回），四个早退条件
//	  （不可编辑 / 原地 / 越界 / 目标越界）。
//	OFF:ui-settings-models/src/client/useModelRows.ts:83-89
//	  `onDrop` 只把 (目标行, 半区) 换算成 (from, to) 再交给 `move`；`onMove(±1)` 也是 `move`。
//	  方向键与拖放在官方就是**同一次** `move`。
//	OFF:ui-settings-models/src/client/ModelsSection.module.css:558-560
//	  拖动中的行：`.modelEntry[data-dragging]{opacity:.5}`。
//	OFF:ui-settings-models/src/client/ModelsSection.module.css:589-616
//	  落点指示器＝行上/下缘外侧的一条插入线：`::before{top:-10px}` / `::after{bottom:-10px}`，
//	  高 12px 的带里画一条 2px 横线（`4px 5px / calc(100% - 4px) 2px`，左内缩 4px），
//	  左端另有两段 5px×7px 的 1px 斜线（55°/125° 两段 gradient，第二段下移 5px）。
//	OFF:ui-settings-models/src/client/ModelsSection.module.css:566-568
//	  拖拽预览底色：`.modelDragPreview{background:var(--dsw-alias-bg-module-platform)}`。
//	OFF:ui-primitives/src/useRowDragPreview.ts:40-66
//	  预览尺寸＝被拖行的 `getBoundingClientRect()`，`setDragImage` 的抓取偏移＝
//	  `(clientX - rect.left, clientY - rect.top)`（即按下的那一点始终在光标下）。
//
// # 本仓怎么接（不自己造拖放）
//
// 全套是 viva 的**原生**拖放（`viva@v0.6.4/ui/drag.go:1-120`）：柄上
// `Box.DragSource`、行上 `Box.DropTarget`；引擎管按下、4px 拖动阈值、落点解析
// 与松手（`drag.go` 的 `press/pointer/targetAt/drop`），应用只回答两件事——
// `accept`（这个落点收不收）与 `drop`（松手了做什么）。
//
// **移动只有一份**：`drop` 只把 (源行, 落点行, 半区) 换成 (from, to)
// （既有 `ModelDropTarget`，官方 `onDrop` 的换算），然后调 `catalogEditor.moveRow`
// —— 与方向键的 `catalogMoveRow` 是同一个函数、同一份 splice、同一组早退条件。
// 落地点本身不做任何数组操作。
//
// 落点半区要在指针路由时算（引擎只把 (源, 目标) 两个身份交给 drop，不交坐标），
// 所以 `accept` 里用 `DragState.At()` 与目标行矩形比中线——正是官方的 rowHalf。
// 行矩形来自**本帧绘制**：viva 的 Box.Rect() 是布局结果（滚动偏移已在布局里加过），
// 因此指针事件发生时读到的是上一帧真实位置。
package dssettings

import (
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 落点指示器几何（逐个对上 ModelsSection.module.css:589-616 的声明，见文件头）。
const (
	catalogDropBandHeight float32 = 12  // [data-drop-target]::before|after{height:12px}
	catalogDropBeforeTop  float32 = -10 // [data-drop-target='before']::before{top:-10px}
	catalogDropAfterBand  float32 = 10  // [data-drop-target='after']::after{bottom:-10px} 的绝对值
	catalogDropRuleTop    float32 = 5   // gradient 的 `5px`（带内纵向偏移）
	catalogDropRuleHeight float32 = 2   // `calc(100% - 4px) 2px` 的 2px
	catalogDropRuleInset  float32 = 4   // `4px 5px` 的 4px（左内缩）
	catalogChevronWidth   float32 = 5   // 两段 gradient 的 `5px 7px` 的 5px
	catalogChevronHeight  float32 = 7   // 同上 7px
	catalogChevronShift   float32 = 5   // 第二段 gradient 的 `0 5px`（下移 5px）
	catalogDraggedFade    float32 = 0.5 // :558-560 [data-dragging]{opacity:.5}
)

// catalogDropRowPrefix 是落点行的命中身份前缀。与行自身的 Path 标记不同：
// 它只是 viva 落点要的那个 gfx.ID 名字，不参与设置路径。
const catalogDropRowPrefix = "settings-catalog-drop-"

// catalogDragTarget 是本帧声明过的一个落点行：身份 + 画它的 Box。
//
// 存 Box 而不是矩形：viva 的矩形要等本帧布局完成才有（`ui/box.go:45` 的 Box.Rect），
// 而落点判定发生在**下一次指针事件**里，那时读到的正好是上一帧的真实几何。
// 面板的 AfterDraw 读同一份（`o.card` 也是这么用的）。
type catalogDragTarget struct {
	id  gfx.ID
	key string
	row ui.Box
}

// catalogDragState 是模型目录行的一次拖放的全部状态。
//
// 只活在一个窗口线程里：绘制、指针路由与 AfterDraw 都在那条线程上，
// 所以下面这些字段一个都不套 `officialPanel.mu`（和 `card`/`cardShown` 同一档）。
type catalogDragState struct {
	// state 是 viva 的原生拖拽状态；零值 = 没在拖。
	state ui.DragState
	env   *pluginapi.Env

	// targets 是本帧声明过的落点行，下标即绘制顺序。
	targets []catalogDragTarget
	// writable 是本帧编辑卡是否可写（官方 `disabled` 取反，useModelRows.ts:76）。
	writable bool

	// pressed 表示指针按在了某个柄上；pressedKey 是那一行，pressedRow 是行矩形，
	// grab 是按下点相对行左上角的偏移（官方的 setDragImage 偏移，
	// useRowDragPreview.ts:65）。
	pressed    bool
	pressedKey string
	pressedRow gfx.Rect
	grab       gfx.Point

	// after 记每个落点行最近一次判定出的半区（accept 里写，drop 里读）。
	after map[gfx.ID]bool

	// accept/drop 只做一次：viva 明确写了「每次绘制造方法值会分配」
	// （ui/drag.go 的 DropTarget 注释），而且要能被所有落点行共用。
	accept func(source, target gfx.ID) bool
	drop   func(source, target gfx.ID)
}

// init 把两个回调挂上（零值可用，第一次用之前调）。
func (d *catalogDragState) init(p *Plugin) {
	if d.accept != nil {
		return
	}
	if d.after == nil {
		d.after = map[gfx.ID]bool{}
	}
	d.accept = func(source, target gfx.ID) bool { return p.catalogDragAccepts(d, source, target) }
	d.drop = func(source, target gfx.ID) { p.catalogDragDrops(d, source, target) }
}

// catalogDrag 取本插件的拖放状态（零值可用；回调只在第一次挂上）。
func (p *Plugin) catalogDrag() *catalogDragState {
	d := &p.ensureOfficial().catalogDrag
	d.init(p)
	return d
}

// beginPaint 清掉上一帧的落点声明。每帧绘制开头调一次：viva 的落点只对**本帧**
// 声明过的元素成立（`ui/drag.go` 的 begin/stale），本地这两张表要跟着清。
// writable 由本帧的行绘制填（drawCatalogEditorModelRow）：只有真画出目录行的那一帧
// 才可能有"收不放"的落点，别把上一帧的可写性带过来。
func (d *catalogDragState) beginPaint(env *pluginapi.Env) {
	d.env = env
	d.writable = false
	d.targets = d.targets[:0]
}

// markWritable 由行绘制调一次：本帧这张卡可写（官方 `!disabled`）。
func (d *catalogDragState) markWritable(writable bool) {
	if writable {
		d.writable = true
	}
}

// declare 记一个本帧声明过的落点行。
func (d *catalogDragState) declare(key string, id gfx.ID, row ui.Box) {
	d.targets = append(d.targets, catalogDragTarget{id: id, key: key, row: row})
}

// press 记下"按在了哪一行的柄上、按在行里的哪个位置"。
//
// 为什么要记身份：viva 的拖源身份是 Control 自己的 id（`ui/control.go:69` 的
// `p.controlID`），Box 上没有读回它的入口，所以按下这一下是身份与行唯一能对上的
// 地方；drop 回调只给身份，`pressedKey` 是它还原成行的依据。
func (d *catalogDragState) press(key string, x, y float32) {
	d.pressed, d.pressedKey = true, key
	rect, ok := d.rowRect(key)
	if !ok {
		d.pressedRow, d.grab = gfx.Rect{}, gfx.Point{}
		return
	}
	d.pressedRow = rect
	d.grab = gfx.Point{X: x - rect.X, Y: y - rect.Y}
}

// rowRect 读本帧某个落点行的矩形。
func (d *catalogDragState) rowRect(key string) (gfx.Rect, bool) {
	for i := range d.targets {
		if d.targets[i].key != key {
			continue
		}
		if !d.targets[i].row.Valid() {
			return gfx.Rect{}, false
		}
		rect := d.targets[i].row.Rect()
		if rect.Width <= 0 || rect.Height <= 0 {
			return gfx.Rect{}, false
		}
		return rect, true
	}
	return gfx.Rect{}, false
}

// targetKey 把落点身份还原成行身份。
func (d *catalogDragState) targetKey(id gfx.ID) (string, bool) {
	for i := range d.targets {
		if d.targets[i].id == id {
			return d.targets[i].key, true
		}
	}
	return "", false
}

// draggingKey 报告某一行是不是正在被拖（官方 `dragging: dragged === state.key`，
// useModelRows.ts:74）。
func (d *catalogDragState) draggingKey(key string) bool {
	return d.state.Dragging() && d.pressed && d.pressedKey == key
}

// catalogDragAccepts 回答"这个落点收不收"（官方 `canDrop`，useModelRows.ts:76）。
//
// 收的条件只有两条：这颗卡可写，而且这次拖拽真的是本卡某个柄发起的。
// 顺带把**半区**算出来：viva 的 drop 只给 (源, 目标) 两个身份，不给坐标，
// 而官方 onDrop 要 `rowHalf`（上半区 before / 下半区 after）——所以在这里用
// `DragState.At()`（指针位置）与目标行矩形比中线记下来，drop 时直接用。
func (p *Plugin) catalogDragAccepts(d *catalogDragState, source, target gfx.ID) bool {
	if !d.writable || !d.pressed || d.pressedKey == "" {
		return false
	}
	rect, ok := d.rowRectByID(target)
	if !ok {
		return false
	}
	// 半区就是官方的 rowHalf（既有 ModelRowAfterHalf，ModelRow.tsx:46-49）：
	// 指针在目标行中线以上 → 插到它前面，中线以下 → 插到它后面。
	d.after[target] = ModelRowAfterHalf(float64(d.state.At().Y), float64(rect.Y), float64(rect.Height))
	return true
}

// rowRectByID 读落点身份对应的行矩形。
func (d *catalogDragState) rowRectByID(id gfx.ID) (gfx.Rect, bool) {
	if key, ok := d.targetKey(id); ok {
		return d.rowRect(key)
	}
	return gfx.Rect{}, false
}

// catalogDragDrops 是松手：把 (源行, 落点行, 半区) 换成 (from, to) 交给**同一个**移动实现。
//
// 官方 `onDrop`（useModelRows.ts:83-88）：
//
//	const insertion = index + (half === 'after' ? 1 : 0)
//	move(from, insertion > from ? insertion - 1 : insertion)
//
// 这段换算就是既有的 `ModelDropTarget`；`move` 就是既有的 `catalogEditor.moveRow`
// （方向键也走它）。这里不做任何数组操作——落地点一旦自己挪数组，就有了第二份
// 移动逻辑。
func (p *Plugin) catalogDragDrops(d *catalogDragState, source, target gfx.ID) {
	fromKey := d.pressedKey
	toKey, ok := d.targetKey(target)
	if !ok || fromKey == "" {
		return
	}
	p.catalogDropRow(d.env, fromKey, toKey, d.after[target])
}

// catalogDropRow 是落点落到草稿上的那一步：换算 + 调 moveRow。
//
// 与 catalogMoveRow（方向键）同形：同样是"拿锁 → 找到行 → 走 moveRow → 挪动了就
// 要一帧"，只是 index 由 ModelDropTarget 算而不是 ±1。
func (p *Plugin) catalogDropRow(env *pluginapi.Env, fromKey, toKey string, after bool) {
	o := p.ensureOfficial()
	o.mu.Lock()
	moved := false
	if edit := o.modelsCatalog; edit != nil {
		from := edit.indexOfKey(fromKey)
		target := edit.indexOfKey(toKey)
		if from >= 0 && target >= 0 {
			if to, ok := ModelDropTarget(from, target, after); ok {
				moved = edit.moveRow(from, to)
			}
		}
	}
	o.mu.Unlock()
	if moved {
		catalogInvalidate(env)
	}
}

// paintCatalogDragOverlay 在布局之后画拖拽的两处视觉（官方 `data-dragging` 的
// 半透明由绘制期 `.Opacity` 落地，这里只管**跟随指针**的两件）：
//
//	① 落点插入线（ModelsSection.module.css:589-616）：目标行上缘外（before）或
//	   下缘外（after）的 2px 横线 + 左端两段 1px 斜线；
//	② 拖拽预览（:566-568 + useRowDragPreview.ts:40-66）：被拖行大小的一块底色，
//	   按抓取偏移跟着指针。
//
// 为什么必须在 AfterDraw：viva 对每个元素是**先推 Clip、再跑 Draw 钩子**
// （`ui/paint.go:50-52`、`:132`），而这两件都画在自己那个盒子**外面**
// （插入线在行框外的 -10px 带上，预览跟着指针跑）——在 Draw 里画会被裁掉。
// 与卡片阴影同一个理由（official_panel_elevation_test.go:15）。
func (p *Plugin) paintCatalogDragOverlay(canvas *gfx.Canvas, env *pluginapi.Env, t *pluginapi.Theme, card gfx.Rect) {
	if canvas == nil {
		return
	}
	d := &p.ensureOfficial().catalogDrag
	if !d.state.Dragging() {
		return
	}
	// 官方卡片 `.AnXs9G_panel{overflow:hidden}` 的等价物：两张图都在卡片外缘之前收住。
	if card.Width <= 0 || card.Height <= 0 {
		return
	}
	canvas.Clip(card)
	if id := d.state.Target(); !id.IsZero() {
		if rect, ok := d.rowRectByID(id); ok {
			paintCatalogDropIndicator(canvas, rect, d.after[id], t.Info)
		}
	}
	if d.pressed && d.pressedRow.Width > 0 && d.pressedRow.Height > 0 {
		at := d.state.At()
		canvas.Rounded(gfx.Rect{
			X: at.X - d.grab.X, Y: at.Y - d.grab.Y,
			Width: d.pressedRow.Width, Height: d.pressedRow.Height,
		}, t.Radii.LG, t.BgModulePlatform)
	}
	canvas.Unclip()
}

// paintCatalogDropIndicator 画一条官方落点插入线。
//
// 几何逐条来自 ModelsSection.module.css:589-616（文件头已列）：
//
//	带高 12px；before 的带顶在行顶 -10px，after 的带底在行底 +10px
//	横线：带顶 +5px，高 2px，左内缩 4px，宽 = 行宽 - 4px
//	斜线：左端 5px×7px，第一段 (0,0)→(5,7)、第二段 (0,12)→(5,5)
func paintCatalogDropIndicator(canvas *gfx.Canvas, row gfx.Rect, after bool, color ui.Color) {
	// before：带顶在行顶外 10px；after：`bottom:-10px` 表示带**底**在行底外 10px，
	// 所以带顶 = 行底 + 10 - 12（= 行底内 2px）。
	bandTop := row.Y + catalogDropBeforeTop
	if after {
		bandTop = row.Y + row.Height + catalogDropAfterBand - catalogDropBandHeight
	}
	canvas.Fill(gfx.Rect{
		X:      row.X + catalogDropRuleInset,
		Y:      bandTop + catalogDropRuleTop,
		Width:  row.Width - catalogDropRuleInset,
		Height: catalogDropRuleHeight,
	}, color)
	// 左端那两段斜线：两段 5×7 的 gradient 在带顶与带顶+5px 处各画一条 1px 斜线，
	// 合起来是一个「＞」形的标记（55° 与 125° 两条，见 CSS 里两个 linear-gradient）。
	chevron := gfx.NewPath(catalogChevronWidth, catalogDropBandHeight,
		gfx.MoveTo(0, 0), gfx.LineTo(catalogChevronWidth, catalogChevronHeight),
		gfx.MoveTo(0, catalogChevronHeight+catalogChevronShift),
		gfx.LineTo(catalogChevronWidth, catalogChevronShift),
	)
	canvas.DrawPath(chevron, gfx.Rect{
		X: row.X, Y: bandTop, Width: catalogChevronWidth, Height: catalogDropBandHeight,
	}, gfx.PathStyle{Stroke: color, StrokeWidth: 1})
}
