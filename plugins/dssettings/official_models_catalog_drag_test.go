//go:build darwin || windows || linux

// official_models_catalog_drag_test.go —— 模型目录行**拖放排序**的证据（#41 最后一条）。
//
// 判据：
//
//	① 真指针拖放真的换位：按下柄 → 越过阈值移到另一行的上半/下半区 → 松手，
//	   草稿顺序按官方 `onDrop` 的插入位变化（useModelRows.ts:83-88）；
//	② 落点半区真的分上/下（官方 rowHalf，ModelRow.tsx:46-49）：移到上半区插到
//	   那一行**之前**，下半区插到**之后**；
//	③ 拖到列表外（不是任何一行）松手，顺序不变（官方 drop 只落在行上）；
//	④ **拖放与方向键结果一致**：同一份草稿、同一个位移，两条入口得到同一个顺序；
//	⑤ 落点插入线按官方几何画出来（ModelsSection.module.css:589-616），
//	   松手后消失；
//	⑥ 被拖的那一行在拖动中半透明（:558-560）。
//
// 指针走的是真 Viva 窗口：`driver.Press/Move/Release` → 壳的 `cfg.OnPointer`
// → `ui.Page` 的拖放路由（`ui/page.go:586-604`）→ viva 原生 `DragState`。
package dssettings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// catalogHandleLabel 是柄的无障碍名（官方 `aria-label` = `reorderModel` + 位置）。
func catalogHandleLabel(position int) string {
	return catalogCopy("reorderModel") + " " + itoa(position)
}

// catalogDropID 是某一行的落点身份在树里的名字（与 drawCatalogEditorModelRow 同一个前缀）。
func catalogDropID(key string) string { return catalogDropRowPrefix + key }

// catalogHitByLabel 找带这个无障碍名的命中区（与 clickLabelScrolled 同一口径，
// 但只找、不点）。找不到就把这个面板行的可见情况滚进来再试。
func catalogHitByLabel(t *testing.T, h *officialHarness, label string) gfx.Hit {
	t.Helper()
	for attempt := 0; attempt < 40; attempt++ {
		for _, hit := range h.session.Snapshot().Hits {
			if hit.Label != label || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				continue
			}
			x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			snapshot := h.session.Snapshot()
			if x < 0 || y < 0 || x >= float32(snapshot.Width) || y >= float32(snapshot.Height) {
				continue
			}
			if id, ok := h.session.Driver().HitTest(x, y); !ok || id != hit.ID {
				continue
			}
			return hit
		}
		snapshot := h.session.Snapshot()
		if err := h.session.Scroll(gfx.ScrollEvent{X: float32(snapshot.Width) * 0.75,
			Y: float32(snapshot.Height) / 2, DeltaY: 180}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	t.Fatalf("滚动后仍然找不到带无障碍名 %q 的控件", label)
	var zero gfx.Hit
	return zero
}

// catalogRowRect 读某一行的**布局矩形**。
//
// 用 `Box.Rect()` 而不是快照里的命中区：行在滚动区里，命中区是**裁剪后**的那块
// （滚出视口就只剩一条边），而落点判定用的正是未裁剪的布局矩形。测试要点到
// 「下半区」，就得按布局矩形算——所以先把它整行滚进视口（catalogScrollRowIntoView），
// 这样两个矩形一致，点也落在真能看见、真能打中的地方。
func catalogRowRect(t *testing.T, h *officialHarness, key string) gfx.Rect {
	t.Helper()
	rect, ok := h.p.catalogDrag().rowRect(key)
	if !ok {
		t.Fatalf("行 %q 没有落点矩形（本帧没声明？）", key)
	}
	return rect
}

// catalogScrollRowIntoView 把一行整行滚进视口，并断言它真能被打中。
func catalogScrollRowIntoView(t *testing.T, h *officialHarness, key string) gfx.Rect {
	t.Helper()
	for attempt := 0; attempt < 40; attempt++ {
		rect, ok := h.p.catalogDrag().rowRect(key)
		if ok {
			hit, found := h.findHit(catalogDropID(key))
			if found && hit.Bounds.Height >= rect.Height-0.5 && hit.Bounds.Width >= rect.Width-0.5 {
				// 整行都在视口里了。再从行内边距上的一点确认它真能被打中
				// （行中线那一带是 ID/名称输入框，HitTest 会打中控件，不是行本身）。
				if id, hittable := h.session.Driver().HitTest(rect.X+2, rect.Y+rect.Height/2); hittable && id == hit.ID {
					return rect
				}
			}
		}
		snapshot := h.session.Snapshot()
		if err := h.session.Scroll(gfx.ScrollEvent{X: float32(snapshot.Width) * 0.75,
			Y: float32(snapshot.Height) / 2, DeltaY: 200}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	t.Fatalf("行 %q 滚了 40 次仍没有整行进入视口", key)
	var zero gfx.Rect
	return zero
}

// dragCatalogRow 在真窗口里把 position 那一行的柄拖到 targetKey 那一行的
// 上半区（after=false）或下半区（after=true），然后松手。
func dragCatalogRow(t *testing.T, h *officialHarness, position int, targetKey string, after bool) {
	t.Helper()
	// 先把落点行整行滚进视口：下半区必须真能点到（剪掉的那半边不该拿来测）。
	row := catalogScrollRowIntoView(t, h, targetKey)
	handle := catalogHitByLabel(t, h, catalogHandleLabel(position))
	sx, sy := handle.Bounds.X+handle.Bounds.Width/2, handle.Bounds.Y+handle.Bounds.Height/2
	tx := row.X + row.Width/2
	ty := row.Y + row.Height/4
	if after {
		ty = row.Y + row.Height*3/4
	}
	if err := h.session.Driver().Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	if err := h.session.Driver().Move(tx, ty); err != nil {
		t.Fatal(err)
	}
	h.pump()
	if !h.p.catalogDrag().state.Dragging() {
		t.Fatalf("从第 %d 行的柄拖到 (%v,%v) 之后 DragState 没进入拖动态", position, tx, ty)
	}
	if err := h.session.Driver().Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// dropCatalogRowOutside 按下柄、拖到面板里的空白处（不是任何一行）再松手。
func dropCatalogRowOutside(t *testing.T, h *officialHarness, position int) {
	t.Helper()
	handle := catalogHitByLabel(t, h, catalogHandleLabel(position))
	sx, sy := handle.Bounds.X+handle.Bounds.Width/2, handle.Bounds.Y+handle.Bounds.Height/2
	if err := h.session.Driver().Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	// 面板标题那一带（卡片顶部的内边距里）：既不是行，也不是任何落点。
	layout := h.p.PanelLayout()
	if err := h.session.Driver().Move(layout.X+layout.Width/2, layout.Y+6); err != nil {
		t.Fatal(err)
	}
	h.pump()
	if !h.p.catalogDrag().state.Dragging() {
		t.Fatal("越过阈值后应处于拖动态（这条要测的是拖到列表外）")
	}
	if !h.p.catalogDrag().state.Target().IsZero() {
		t.Fatalf("拖到面板标题带不该解析出落点行，实际 %v", h.p.catalogDrag().state.Target())
	}
	if err := h.session.Driver().Release(layout.X+layout.Width/2, layout.Y+6, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
}

// catalogPayloadIDs 从 replace 的 JSON 数组里按顺序读 id（写出去的形状是每条一个对象）。
func catalogPayloadIDs(t *testing.T, models []string) []string {
	t.Helper()
	out := make([]string, 0, len(models))
	for index, model := range models {
		var payload map[string]any
		if err := json.Unmarshal([]byte(model), &payload); err != nil {
			t.Fatalf("第 %d 条不是合法 JSON：%v", index+1, err)
		}
		id, _ := payload["id"].(string)
		out = append(out, id)
	}
	return out
}

// catalogDraftIDs 读当前草稿的 id 顺序（判据用 id 而不是内部行 key，读起来是人话）。
func catalogDraftIDs(h *officialHarness) []string {
	editor := catalogEditOf(h)
	out := make([]string, 0, len(editor.drafts))
	for index := range editor.drafts {
		out = append(out, ModelText(editor.drafts[index], "id"))
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

// fakeColor Fade：官方 `opacity:.5` 落到颜色上就是 alpha 乘 0.5（viva ui/style.go:361-364）。
func fadedColor(color gfx.Color, opacity float32) gfx.Color {
	color.Alpha *= opacity
	return color
}

// TestModelsCatalogDragHandleReordersDraft 钉 ①：柄上按下、拖到另一行、松手，
// 草稿顺序真的变，而且变化在**同一次** replace 里写出去（写出口没变）。
func TestModelsCatalogDragHandleReordersDraft(t *testing.T) {
	h := openOfficialHarness(t, 1182, 1000)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	if len(editor.keys) < 2 {
		t.Fatalf("这条用例要两条模型，实际 %d 条", len(editor.keys))
	}
	first, second := editor.keys[0], editor.keys[1]
	if got := catalogDraftIDs(h); !equalStrings(got, []string{"alpha", "beta"}) {
		t.Fatalf("起手草稿应是 alpha,beta，实际 %v", got)
	}

	// 第 1 行拖到第 2 行的**下半区** → 插到它后面。
	dragCatalogRow(t, h, 1, second, true)
	if got := catalogDraftIDs(h); !equalStrings(got, []string{"beta", "alpha"}) {
		t.Fatalf("拖到第 2 行下半区松手后应是 beta,alpha，实际 %v", got)
	}
	editor = catalogEditOf(h)
	if editor.keys[0] != second || editor.keys[1] != first {
		t.Fatalf("行身份应跟着草稿一起换位，实际 %v", editor.keys)
	}

	// 保存仍走唯一写出口：一次 `models.catalog.replace`，数组顺序跟草稿走。
	h.clickScrolled("settings-row-" + catalogApplyRowID)
	// 写请求在 goroutine 里发（绘制线程不能等 HTTP）：pump 到它落地，超时才失败。
	for attempt := 0; attempt < 400; attempt++ {
		h.mu.Lock()
		done := len(h.catalogReplaces) >= 1
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	replaces := append([]catalogReplaceCall(nil), h.catalogReplaces...)
	h.mu.Unlock()
	if len(replaces) != 1 {
		t.Fatalf("排序保存应恰好一次 replace，实际 %d 次：%+v", len(replaces), replaces)
	}
	if got := catalogPayloadIDs(t, replaces[0].models); !equalStrings(got, []string{"beta", "alpha"}) {
		t.Fatalf("replace 的数组顺序应跟拖放后的草稿：%v", replaces[0].models)
	}
}

// TestModelsCatalogDropHalfDecidesInsertion 钉 ②：同一次拖拽，落点在目标行的
// 上半区与下半区得到不同结果（官方 rowHalf，ModelRow.tsx:46-49）。
func TestModelsCatalogDropHalfDecidesInsertion(t *testing.T) {
	h := openOfficialHarness(t, 1182, 1000)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	third := editor.indexOfKey(editor.keys[1])
	if third < 0 {
		t.Fatal("第二条行身份不见了")
	}
	target := editor.keys[1]

	// 上半区：第 1 行插到第 2 行**之前** = 原地不动。
	dragCatalogRow(t, h, 1, target, false)
	if got := catalogDraftIDs(h); !equalStrings(got, []string{"alpha", "beta"}) {
		t.Fatalf("拖到自己后面那一行的上半区应保持原序，实际 %v", got)
	}

	// 下半区：插到第 2 行**之后** = 换位。
	dragCatalogRow(t, h, 1, target, true)
	if got := catalogDraftIDs(h); !equalStrings(got, []string{"beta", "alpha"}) {
		t.Fatalf("拖到下半区应换位成 beta,alpha，实际 %v", got)
	}
}

// TestModelsCatalogDropOutsideLeavesDraftAlone 钉 ③：拖到列表之外松手不变。
func TestModelsCatalogDropOutsideLeavesDraftAlone(t *testing.T) {
	h := openOfficialHarness(t, 1182, 1000)
	defer h.close()
	openModelsCatalogCard(t, h)
	dropCatalogRowOutside(t, h, 1)
	if got := catalogDraftIDs(h); !equalStrings(got, []string{"alpha", "beta"}) {
		t.Fatalf("拖到列表外松手不该改顺序，实际 %v", got)
	}
	// 也不能留下"半截拖拽"：状态必须是没在拖。
	if h.p.catalogDrag().state.Dragging() {
		t.Fatal("松手后 DragState 还认为在拖")
	}
}

// TestModelsCatalogDragAndArrowKeysAgree 钉 ④（本单最重要的一条）：
// **同一个位移，拖放与方向键得到同一个顺序**。
//
// 两条入口在官方就是同一次 `move`（useModelRows.ts:60-67 被 :83-89 的 onDrop 与
// :89 的 onMove 共用）；在本仓是同一个 `catalogEditor.moveRow`。这条用真窗口跑
// 两遍：一遍拖放，一遍在一棵新窗口上按方向键走到同一个落点。
func TestModelsCatalogDragAndArrowKeysAgree(t *testing.T) {
	// 拖放：第 1 行拖到第 2 行下半区。
	dragged := openOfficialHarness(t, 1182, 1000)
	openModelsCatalogCard(t, dragged)
	dragCatalogRow(t, dragged, 1, catalogEditOf(dragged).keys[1], true)
	dragOrder := catalogDraftIDs(dragged)
	dragged.close()

	// 方向键：聚焦第 1 行的柄，按一次 ↓（等价"往后挪一格"）。
	keys := openOfficialHarness(t, 1182, 1000)
	defer keys.close()
	openModelsCatalogCard(t, keys)
	clickLabelScrolled(t, keys, catalogHandleLabel(1))
	if err := keys.session.Driver().Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	keys.pump()
	arrowOrder := catalogDraftIDs(keys)

	if !equalStrings(dragOrder, arrowOrder) {
		t.Fatalf("拖放与方向键必须得到同一个顺序：拖放 %v，方向键 %v（两条入口应共用 "+
			"catalogEditor.moveRow，useModelRows.ts:60-67/:83-89）", dragOrder, arrowOrder)
	}
	if !equalStrings(dragOrder, []string{"beta", "alpha"}) {
		t.Fatalf("这个位移应得到 beta,alpha，实际 %v", dragOrder)
	}
}

// TestModelsCatalogDropIndicatorFollowsPointer 钉 ⑤：落点插入线按官方几何画出来。
//
// 官方（ModelsSection.module.css:589-616）：`before` 的带顶在目标行顶外 10px，
// 带里第 5px 处一条 2px 横线、左内缩 4px、宽 = 行宽 - 4px。
func TestModelsCatalogDropIndicatorFollowsPointer(t *testing.T) {
	h := openOfficialHarness(t, 1182, 1000)
	defer h.close()
	openModelsCatalogCard(t, h)
	theme := h.theme
	target := catalogEditOf(h).keys[1]
	row := catalogScrollRowIntoView(t, h, target)

	// 拖动中但还没松手：不 pump 出松手，先看画出来的东西。
	handle := catalogHitByLabel(t, h, catalogHandleLabel(1))
	sx, sy := handle.Bounds.X+handle.Bounds.Width/2, handle.Bounds.Y+handle.Bounds.Height/2
	if err := h.session.Driver().Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	ty := row.Y + row.Height/4 // 上半区 → before
	if err := h.session.Driver().Move(row.X+row.Width/2, ty); err != nil {
		t.Fatal(err)
	}
	h.pump()

	want := gfx.Rect{
		X:      row.X + catalogDropRuleInset,
		Y:      row.Y + catalogDropBeforeTop + catalogDropRuleTop,
		Width:  row.Width - catalogDropRuleInset,
		Height: catalogDropRuleHeight,
	}
	if !hasRectangle(h, want, theme.Info) {
		t.Fatalf("拖动中应画官方落点插入线 %+v（颜色 = --dsw-alias-state-business-primary），"+
			"实际矩形=%v", want, canvasRectangles(h))
	}
	if !h.p.catalogDrag().state.Dragging() {
		t.Fatal("越过阈值后 DragState 应处于拖动态")
	}

	if err := h.session.Driver().Release(row.X+row.Width/2, ty, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if hasRectangle(h, want, theme.Info) {
		t.Fatal("松手后插入线必须消失")
	}
}

// TestModelsCatalogDraggedRowFadesWhileDragging 钉 ⑥：被拖的那一行在拖动中半透明
// （官方 `.modelEntry[data-dragging]{opacity:.5}`，ModelsSection.module.css:558-560）。
//
// 判据取行框的描边色：viva 的 Opacity 是把**里面每个颜色**的 alpha 乘上去
// （ui/style.go:207-214 的注释与 :361-364 的 fade），所以拖动中行描边应是
// border-l4 减半的那一个色。
func TestModelsCatalogDraggedRowFadesWhileDragging(t *testing.T) {
	h := openOfficialHarness(t, 1182, 1000)
	defer h.close()
	openModelsCatalogCard(t, h)
	theme := h.theme
	faded := fadedColor(theme.BorderL4, catalogDraggedFade)

	handle := catalogHitByLabel(t, h, catalogHandleLabel(1))
	sx, sy := handle.Bounds.X+handle.Bounds.Width/2, handle.Bounds.Y+handle.Bounds.Height/2
	if err := h.session.Driver().Press(sx, sy, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	target := catalogEditOf(h).keys[1]
	row := catalogRowRect(t, h, target)
	if err := h.session.Driver().Move(row.X+row.Width/2, row.Y+row.Height*3/4); err != nil {
		t.Fatal(err)
	}
	h.pump()

	if !hasColor(h, faded) {
		t.Fatalf("拖动中应该有 opacity .5 的描边（%+v），实际三角/矩形颜色里没有",
			faded)
	}
	if err := h.session.Driver().Release(row.X+row.Width/2, row.Y+row.Height*3/4, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if hasColor(h, faded) {
		t.Fatal("松手后不该再有半透明描边")
	}
}

// hasRectangle 报告画布里有没有一个与 want 同位置同尺寸、颜色为 color 的矩形。
func hasRectangle(h *officialHarness, want gfx.Rect, color gfx.Color) bool {
	if h.canvas == nil {
		return false
	}
	for _, rect := range h.canvas.Rectangles() {
		if rect.Color != color {
			continue
		}
		if near(rect.Bounds.X, want.X) && near(rect.Bounds.Y, want.Y) &&
			near(rect.Bounds.Width, want.Width) && near(rect.Bounds.Height, want.Height) {
			return true
		}
	}
	return false
}

// hasColor 报告画布里有没有任何图元用了这个颜色（矩形或三角）。
func hasColor(h *officialHarness, color gfx.Color) bool {
	if h.canvas == nil {
		return false
	}
	for _, rect := range h.canvas.Rectangles() {
		if rect.Color == color {
			return true
		}
	}
	for _, tri := range h.canvas.Triangles() {
		if tri.Color == color {
			return true
		}
	}
	return false
}

// canvasRectangles 是失败时用的现场（只留位置与颜色，够定位）。
func canvasRectangles(h *officialHarness) []string {
	if h.canvas == nil {
		return nil
	}
	out := make([]string, 0, len(h.canvas.Rectangles()))
	for _, rect := range h.canvas.Rectangles() {
		out = append(out, fmtRect(rect.Bounds))
	}
	return out
}

// fmtRect 把一个矩形打印成 `x,y wxh`。
func fmtRect(rect gfx.Rect) string {
	return itoa(int(rect.X)) + "," + itoa(int(rect.Y)) + " " + itoa(int(rect.Width)) + "x" + itoa(int(rect.Height))
}

// near 判两个坐标是否同一个像素（画布会按设备像素吸附，留一像素的余量）。
func near(a, b float32) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1
}

// TestModelsCatalogHasASingleMoveImplementation 钉**结构**：排序移动只有一份。
//
// 为什么要有这条：方向键那半条早已写好并有测试（TestModelsCatalogReorderHandle...
// MovesDraftWithArrowKeys），拖放若自己再挪一次数组，就有两份移动逻辑——两份迟早
// 漂，而且各自的测试都会绿、行为却不一致。判据是**源码里的调用点数量**：
//
//	MoveModelDraft(   只在 catalogEditor.moveRow 里调一次（定义处一次）
//	.moveRow(         只有方向键 catalogMoveRow 与落点 catalogDropRow 两处
//	拖放文件里不许出现 drafts（落地点碰草稿数组就等于第二份实现）
func TestModelsCatalogHasASingleMoveImplementation(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读 %s：%v", name, err)
		}
		text := string(source)
		counts["MoveModelDraft("] += strings.Count(text, "MoveModelDraft(")
		counts[".moveRow("] += strings.Count(text, ".moveRow(")
		if name == "official_models_catalog_drag.go" && strings.Contains(text, "drafts") {
			t.Errorf("%s 里出现了 drafts：落地点不许直接改草稿数组（那就是第二份移动逻辑），"+
				"要移动就调 catalogEditor.moveRow", name)
		}
	}
	// MoveModelDraft：定义 1 次 + moveRow 里调用 1 次。
	if counts["MoveModelDraft("] != 2 {
		t.Errorf("MoveModelDraft( 应只有定义 + moveRow 一处调用（2 次），实际 %d 次："+
			"排序移动必须只有一份（useModelRows.ts:60-67）", counts["MoveModelDraft("])
	}
	// moveRow：方向键一处 + 拖放一处。
	if counts[".moveRow("] != 2 {
		t.Errorf(".moveRow( 应只有方向键与拖放两处（2 次），实际 %d 次："+
			"两条入口共用同一个 move（useModelRows.ts:83-89）", counts[".moveRow("])
	}
}
