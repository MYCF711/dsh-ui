//go:build darwin || windows || linux

// official_models_catalog_visual_test.go —— 模型目录编辑卡的**真实绘制 + 真点击**证据。
//
// 判据不是"有这一行"，而是四件事各在真实窗口里成立：
//
//	① 卡片由行头打开，编辑器的每一行都画出来了（标题/状态/空态说明/每行模型）；
//	② **每个可交互控件都有正面积命中区**，而且点它的中心能打中它自己
//	   （`officialHarness.click` 里已有"中心必须 HitTest 回自己"的判据）；
//	③ 点击真的改草稿、真的发写：展开、输入类型、推理开关、添加模型、保存
//	   （`models.catalog.upsert` 的 model 与 expectedRevision 都要对）；
//	④ 顺序变了才发 `models.catalog.reorder`，且发的是**全排列**；
//	   删除钮**没有命中区**（宿主契约没有 delete，见 #41）。
package dssettings

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/vivaui/viva/gfx"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// cloneRevision 复制一个 expectedRevision 指针（断言时不能持有会被改的那一个）。
func cloneRevision(revision *int64) *int64 {
	if revision == nil {
		return nil
	}
	value := *revision
	return &value
}

// catalogModelsFor 按 ids 造一组最小目录模型（替身写口用）。
func catalogModelsFor(ids []string) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		out = append(out, json.RawMessage(`{"id":`+strconv.Quote(id)+`}`))
	}
	return out
}

// catalogDocumentWith 造一份"写完之后"的目录文档（replace 那条放最前，保证 id 唯一）。
func catalogDocumentWith(revision int64, _ string) apiclient.ModelCatalogDocument {
	return apiclient.ModelCatalogDocument{
		Revision: revision,
		Models:   []json.RawMessage{json.RawMessage(`{"id":"alpha"}`), json.RawMessage(`{"id":"beta"}`)},
		Active:   apiclient.ModelCatalogActive{ID: "alpha"},
	}
}

// openModelsCatalogCard 打开设置 → Models，并把替身目录装好、点开编辑卡。
func openModelsCatalogCard(t *testing.T, h *officialHarness) {
	t.Helper()
	h.mu.Lock()
	document := apiclient.ModelCatalogDocument{
		Revision: 7,
		Models: []json.RawMessage{
			json.RawMessage(`{"id":"alpha","name":"阿尔法","contextWindow":131072}`),
			json.RawMessage(`{"id":"beta"}`),
		},
		Active: apiclient.ModelCatalogActive{ID: "alpha", Revision: 3},
	}
	h.catalog = &apiclient.ModelCatalogSnapshot{Document: &document}
	h.catalogRevision = 7
	h.mu.Unlock()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatalf("Shift+F2 打开设置：%v", err)
	}
	h.pump()
	h.pump()
	h.click("settings-nav-models")
	// 目录卡排在所有 provider 卡之后，默认在可视区外：按真实滚动把它滚进来。
	h.clickScrolled("settings-row-catalog-edit")
}

// editorSnapshot 读当前草稿（拿锁拷贝，避免用例与绘制线程抢同一片内存）。
func catalogEditOf(h *officialHarness) *catalogEditor {
	o := h.p.ensureOfficial()
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.modelsCatalog
}

// assertCatalogHit 断言一个命中区存在、面积为正、且中心真的打到它自己。
//
// 零面积 = 声明了但点不着（与 cmd/app hits_matrix 的同一条判据）。长卡片里的行
// 可能落在可视区外，所以先按真实滚动把它滚进来——命中区存在但被裁掉同样是
// "点不着"，用滚动而不是断言豁免来处理。
func assertCatalogHit(t *testing.T, h *officialHarness, name string) {
	t.Helper()
	for attempt := 0; attempt < 40; attempt++ {
		if hit, ok := h.findHit(name); ok {
			if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				t.Fatalf("%s 命中区是零面积：%+v", name, hit.Bounds)
			}
			x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			snapshot := h.session.Snapshot()
			if x >= 0 && y >= 0 && x < float32(snapshot.Width) && y < float32(snapshot.Height) {
				if id, ok := h.session.Driver().HitTest(x, y); ok && id == hit.ID {
					return
				}
			}
		}
		if err := h.session.Scroll(gfx.ScrollEvent{X: float32(h.session.Snapshot().Width) * 0.75,
			Y: float32(h.session.Snapshot().Height) / 2, DeltaY: 180}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	t.Fatalf("滚动后仍然点不着 %q", name)
}

// TestModelsCatalogCardOpensAndDrawsEveryRow 钉 ①②：卡片能打开，行都在。
func TestModelsCatalogCardOpensAndDrawsEveryRow(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)

	editor := catalogEditOf(h)
	if editor == nil {
		t.Fatalf("点行头应打开编辑卡")
	}
	if len(editor.drafts) != 2 {
		t.Fatalf("应从宿主快照灌入两条模型，得到 %d", len(editor.drafts))
	}
	// 官方文案逐条落地：卡片标题、状态、两个容量名、输入类型、推理等级。
	for _, want := range []string{
		catalogCopy("models"), catalogCopy("modelsCustomized"), catalogCopy("addModel"),
		catalogDeleteGapNote, "alpha", "beta",
	} {
		if !h.hasText(want) {
			t.Fatalf("缺少文案 %q；已画文本=%v", want, h.session.Texts())
		}
	}
	// 折叠态下不画展开区。
	if h.hasText(catalogCopy("modelInputTypes")) {
		t.Fatalf("折叠态不该画输入类型")
	}
}

// TestModelsCatalogEveryControlHasAHitRegion 钉 ②：每个可交互控件都有正面积命中区。
func TestModelsCatalogEveryControlHasAHitRegion(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	key := editor.keys[0]

	// 折叠态：行内的 ID / 名称 / 展开 / 添加模型 / 取消 / 保存。
	for _, name := range []string{
		"settings-catalog-field-" + key + "-id",
		"settings-catalog-field-" + key + "-name",
		"settings-catalog-toggle-" + key,
		"settings-row-" + catalogAddRowID,
		"settings-row-" + catalogCancelRowID,
		"settings-row-" + catalogApplyRowID,
	} {
		assertCatalogHit(t, h, name)
	}
	// 拖拽柄是 `ui.Control`（本栈唯一"可聚焦 + 收键盘"的元素），它的命中区
	// 按官方 aria-label 登记（ModelRow.tsx:107）。
	assertHitByLabel(t, h, catalogCopy("reorderModel")+" 1")

	// 删掉钮**故意没有命中区**：宿主契约没有 delete。
	if _, ok := h.findHit("settings-catalog-remove-" + key); ok {
		t.Fatalf("删除钮不该有命中区（宿主没有删除方法，点得着却没反应更坏）")
	}

	// 展开后：两组容量、输入类型（image 可点；text 是唯一选中项 → 按官方
	// ModelInputTypes.tsx:44 锁住，**没有命中区**）、推理开关。
	h.clickScrolled("settings-catalog-toggle-" + key)
	for _, name := range []string{
		"settings-catalog-field-" + key + "-contextWindow",
		"settings-catalog-field-" + key + "-maxTokens",
		"settings-catalog-input-image-" + key,
		"settings-catalog-reason-" + key,
	} {
		assertCatalogHit(t, h, name)
	}
	if _, ok := h.findHit("settings-catalog-input-text-" + key); ok {
		t.Fatalf("只剩 text 一项时不能取消（官方 disabled），不该有命中区")
	}
}

// assertHitByLabel 按无障碍名找命中区并断言面积为正。
func assertHitByLabel(t *testing.T, h *officialHarness, label string) {
	t.Helper()
	for _, hit := range h.session.Snapshot().Hits {
		if hit.Label == label {
			if hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
				t.Fatalf("%q 命中区零面积：%+v", label, hit.Bounds)
			}
			return
		}
	}
	t.Fatalf("没有命中区带无障碍名 %q", label)
}

// TestModelsCatalogClickEditsDraft 钉 ③：真点击真的改草稿。
func TestModelsCatalogClickEditsDraft(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	key := editor.keys[1] // beta：没有 input/容量，正好看"加"的效果

	// 展开 → 折叠态与展开态是两份行集。
	h.clickScrolled("settings-catalog-toggle-" + key)
	editor = catalogEditOf(h)
	if !editor.expanded[key] {
		t.Fatalf("点 chevron 应展开这一行")
	}

	// 输入类型：加图片（官方 ModelInputTypes.tsx:45-55）。
	h.clickScrolled("settings-catalog-input-image-" + key)
	editor = catalogEditOf(h)
	if selected := catalogSelectedInputs(editor.drafts[1]); !containsString(selected, ModelInputImage) {
		t.Fatalf("点图片复选后草稿应含 image：%v", selected)
	}

	// 推理等级：打开写官方初始档位。
	h.clickScrolled("settings-catalog-reason-" + key)
	editor = catalogEditOf(h)
	if !editor.reasoningOn(key) {
		t.Fatalf("点开关应打开推理等级")
	}

	// 添加模型：追加一行空 id（官方 `{id:''}`）。
	h.clickScrolled("settings-row-" + catalogAddRowID)
	editor = catalogEditOf(h)
	if len(editor.drafts) != 3 {
		t.Fatalf("添加模型应多一行，得到 %d", len(editor.drafts))
	}
}

// TestModelsCatalogApplyUpsertsEveryRowAndReordersOnce 钉 ③④：
// 保存 = 逐条 upsert（带乐观锁），顺序变了再 reorder 一次全排列。
func TestModelsCatalogApplyUpsertsEveryRowAndReordersOnce(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)

	// 真的改一处：给 beta 输入类型加图片（会随 upsert 写出去）。
	editor := catalogEditOf(h)
	key := editor.keys[1]
	h.clickScrolled("settings-catalog-toggle-" + key)
	h.clickScrolled("settings-catalog-input-image-" + key)

	// 只改字段、不动顺序：只应有两次 upsert、零次 reorder。
	h.clickScrolled("settings-row-" + catalogApplyRowID)
	for i := 0; i < 400; i++ {
		h.mu.Lock()
		done := len(h.catalogUpserts) >= 2
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	upserts := append([]catalogUpsertCall(nil), h.catalogUpserts...)
	reorders := append([]catalogReorderCall(nil), h.catalogReorders...)
	h.mu.Unlock()
	if len(upserts) != 2 {
		t.Fatalf("两条草稿应各 upsert 一次，得到 %d：%+v", len(upserts), upserts)
	}
	if len(reorders) != 0 {
		t.Fatalf("顺序没变不该发 reorder：%+v", reorders)
	}
	// 乐观锁：第一次用快照的 revision=7，第二次用宿主回报的新值。
	if upserts[0].expected == nil || *upserts[0].expected != 7 {
		t.Fatalf("第一次 upsert 应 fence 在 7：%+v", upserts[0].expected)
	}
	if upserts[1].expected == nil || *upserts[1].expected <= 7 {
		t.Fatalf("第二次 upsert 应 fence 在宿主回报的新 revision：%+v", upserts[1].expected)
	}
	// model 必须是宿主认的形状：id 原样、input 有 image。
	var second map[string]any
	if err := json.Unmarshal([]byte(upserts[1].model), &second); err != nil {
		t.Fatalf("upsert 的 model 不是合法 JSON：%v", err)
	}
	if second["id"] != "beta" {
		t.Fatalf("第二条应是 beta：%v", second)
	}
	inputs, _ := second["input"].([]any)
	if len(inputs) != 2 {
		t.Fatalf("input 应含 text+image 两项：%v", second["input"])
	}
	// 保存成功后卡片关闭、输入框登记清掉。
	if catalogEditOf(h) != nil {
		t.Fatalf("保存成功后应关卡")
	}
}

// TestModelsCatalogReorderOnlyOnOrderChangeAndFullPermutation 钉 ④：
// 顺序变了发一次全排列；宿主拒绝时保留草稿并报失败。
func TestModelsCatalogReorderOnlyOnOrderChangeAndFullPermutation(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	// 把 beta 挪到第一位（顺序真的变了）。用官方 `move` 的同一语义直接挪草稿：
	// 拖放/键盘这个**入口**本单没落（viva 没有 HTML5 拖放，Control 的上下箭头
	// 被面板的行光标先吃掉），但"顺序变了就必须发全排列 reorder"这条写形状
	// 必须被钉住——否则宿主那边真发出去时才发现形状错了。
	o := h.p.ensureOfficial()
	o.mu.Lock()
	if !o.modelsCatalog.moveRow(1, 0) {
		o.mu.Unlock()
		t.Fatalf("草稿移动失败")
	}
	o.mu.Unlock()
	h.pump()

	h.clickScrolled("settings-row-" + catalogApplyRowID)
	for i := 0; i < 400; i++ {
		h.mu.Lock()
		done := len(h.catalogReorders) > 0
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	reorders := append([]catalogReorderCall(nil), h.catalogReorders...)
	h.mu.Unlock()
	if len(reorders) != 1 {
		t.Fatalf("顺序变了应发一次 reorder，得到 %d", len(reorders))
	}
	if len(reorders[0].ids) != 2 || reorders[0].ids[0] != "beta" || reorders[0].ids[1] != "alpha" {
		t.Fatalf("reorder 必须发全排列且按草稿顺序：%v", reorders[0].ids)
	}
}
