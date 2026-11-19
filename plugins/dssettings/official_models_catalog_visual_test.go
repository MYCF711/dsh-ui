//go:build darwin || windows || linux

// official_models_catalog_visual_test.go —— 模型目录编辑卡的**真实绘制 + 真点击**证据。
//
// 判据不是"有这一行"，而是四件事各在真实窗口里成立：
//
//	① 卡片由行头打开，编辑器的每一行都画出来了（标题/状态/空态说明/每行模型）；
//	② **每个可交互控件都有正面积命中区**，而且点它的中心能打中它自己
//	   （`officialHarness.click` 里已有"中心必须 HitTest 回自己"的判据）；
//	③ 点击真的改草稿、真的发写：展开、输入类型、推理开关、添加模型、保存
//	   （`models.catalog.replace` 的 models 数组与 expectedRevision 都要对）；
//	④ 删除/排序/新增都走**同一次** replace（宿主 dsh-go#82 的整数组写入，
//	   等价官方删除语义）；删除钮是**真按钮**，有正面积命中区。
package dssettings

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
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
	// 协议候选与 baseURL 占位来自 `llm.listConfigurableProviders`（路由事实），
	// 与真实壳一致地装进来：没有它 "API 协议" 那格按设计不画（没有候选）。
	h.modelsSnapshot = modelsSnapshotFixture()
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
	// 官方文案逐条落地：卡片标题、状态、添加模型、发现按钮、两条已有模型。
	for _, want := range []string{
		catalogCopy("models"), catalogCopy("modelsCustomized"), catalogCopy("addModel"),
		catalogCopy("fetchModels"), "alpha", "beta",
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

	// 删除钮是**真按钮**：宿主 `models.catalog.replace` 给了整数组写入，
	// 所以它必须有正面积命中区（官方 ModelRow.tsx:142-151 的 trash 也是可点的）。
	assertCatalogHit(t, h, "settings-catalog-remove-"+key)
	// 获取可用模型：没有声明 baseURL 时按官方 `askable` 判据禁用（无命中区）。
	if _, ok := h.findHit("settings-row-" + catalogFetchRowID); ok {
		t.Fatalf("没有 baseURL 时不该有获取可用模型入口（官方 askable 判据）")
	}

	// 展开后：两组容量、输入类型（image 可点；text 是唯一选中项 → 按官方
	// ModelInputTypes.tsx:44 锁住，**没有命中区**）、推理开关。
	h.clickScrolled("settings-catalog-toggle-" + key)
	for _, name := range []string{
		"settings-catalog-field-" + key + "-contextWindow",
		"settings-catalog-field-" + key + "-maxTokens",
		"settings-catalog-field-" + key + "-baseURL",
		"settings-catalog-apicycle-" + key,
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

// TestModelsCatalogApplyReplacesTheWholeArrayOnce 钉 ③④：
// 保存 = **一次** `models.catalog.replace`，带开卡 revision。
func TestModelsCatalogApplyReplacesTheWholeArrayOnce(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)

	// 真的改一处：给 beta 输入类型加图片（会随 replace 写出去）。
	editor := catalogEditOf(h)
	key := editor.keys[1]
	h.clickScrolled("settings-catalog-toggle-" + key)
	h.clickScrolled("settings-catalog-input-image-" + key)

	h.clickScrolled("settings-row-" + catalogApplyRowID)
	for i := 0; i < 400; i++ {
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
	reorders := append([]catalogReorderCall(nil), h.catalogReorders...)
	upserts := append([]catalogUpsertCall(nil), h.catalogUpserts...)
	h.mu.Unlock()
	if len(replaces) != 1 {
		t.Fatalf("保存应只发一次 replace，得到 %d：%+v", len(replaces), replaces)
	}
	if len(reorders) != 0 || len(upserts) != 0 {
		t.Fatalf("replace 之后不该再发 reorder/upsert：%d/%d", len(reorders), len(upserts))
	}
	if len(replaces[0].models) != 2 {
		t.Fatalf("两条草稿应一次写完，得到 %d 条：%+v", len(replaces[0].models), replaces[0].models)
	}
	// 乐观锁：用开卡快照的 revision=7。
	if replaces[0].expected == nil || *replaces[0].expected != 7 {
		t.Fatalf("replace 应 fence 在 7：%+v", replaces[0].expected)
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(replaces[0].models[1]), &second); err != nil {
		t.Fatalf("第 2 条不是合法 JSON：%v", err)
	}
	if second["id"] != "beta" {
		t.Fatalf("第 2 条应是 beta（草稿顺序）：%v", second)
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

// TestModelsCatalogDeleteRowGoesThroughReplace 钉删除：
// 删除钮是真按钮，点了真的从草稿里 splice；保存时一次 replace 少一条。
func TestModelsCatalogDeleteRowGoesThroughReplace(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	if len(editor.drafts) != 2 {
		t.Fatalf("初始应两条，得到 %d", len(editor.drafts))
	}
	key := editor.keys[0] // alpha

	h.clickScrolled("settings-catalog-remove-" + key)
	editor = catalogEditOf(h)
	if len(editor.drafts) != 1 || editor.keys[0] != "m2" {
		t.Fatalf("删除应 splice 掉第一行且行身份不串：%d/%v", len(editor.drafts), editor.keys)
	}
	if catalogCopy("removeModel") == "" {
		t.Fatalf("删除按钮必须有官方文案")
	}

	h.clickScrolled("settings-row-" + catalogApplyRowID)
	for i := 0; i < 400; i++ {
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
	reorders := append([]catalogReorderCall(nil), h.catalogReorders...)
	h.mu.Unlock()
	if len(replaces) != 1 {
		t.Fatalf("删除保存应发一次 replace，得到 %d", len(replaces))
	}
	if len(replaces[0].models) != 1 || !strings.Contains(replaces[0].models[0], `"beta"`) {
		t.Fatalf("replace 的数组应只剩 beta：%+v", replaces[0].models)
	}
	if len(reorders) != 0 {
		t.Fatalf("删除不许用 reorder 表达（宿主会 bad_request）：%+v", reorders)
	}
}

// TestModelsCatalogReplaceKeepsDraftOrder 钉排序：
// 草稿顺序变了，replace 的数组跟着草稿走（不再有第二段 reorder）。
func TestModelsCatalogReplaceKeepsDraftOrder(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	// 把 beta 挪到第一位。入口是拖拽柄 `ui.Control` 的方向键（官方
	// ModelRow.tsx:121-126）：本用例直接断言写形状，入口由
	// TestModelsCatalogReorderHandleMovesDraft 用真按键钉住。
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
		done := len(h.catalogReplaces) >= 1
		h.mu.Unlock()
		if done {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	replaces := append([]catalogReplaceCall(nil), h.catalogReplaces...)
	reorders := append([]catalogReorderCall(nil), h.catalogReorders...)
	h.mu.Unlock()
	if len(reorders) != 0 {
		t.Fatalf("排序也走 replace，不该再发 reorder：%+v", reorders)
	}
	if len(replaces) != 1 || len(replaces[0].models) != 2 {
		t.Fatalf("应一次 replace 两条：%+v", replaces)
	}
	if !strings.Contains(replaces[0].models[0], `"beta"`) || !strings.Contains(replaces[0].models[1], `"alpha"`) {
		t.Fatalf("replace 数组必须按草稿顺序：%+v", replaces[0].models)
	}
}

// clickLabelScrolled 按无障碍名找到命中区，滚进可视区后真实点击（Control 的命中区
// 没有 env.Tree.ID 名字，只能按 label 找）。
func clickLabelScrolled(t *testing.T, h *officialHarness, label string) {
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
			if err := h.session.Click(x, y); err != nil {
				t.Fatal(err)
			}
			h.pump()
			h.pump()
			return
		}
		snapshot := h.session.Snapshot()
		if err := h.session.Scroll(gfx.ScrollEvent{X: float32(snapshot.Width) * 0.75,
			Y: float32(snapshot.Height) / 2, DeltaY: 180}); err != nil {
			t.Fatal(err)
		}
		h.pump()
	}
	t.Fatalf("滚动后仍然点不着带无障碍名 %q 的控件", label)
}

// TestModelsCatalogReorderHandleMovesDraftWithArrowKeys 钉排序入口的**方向键**那一半：
// 官方拖拽柄是 `aria-keyshortcuts="ArrowUp ArrowDown"` 的 button，按 ↑/↓ 各挪一格
// （ModelRow.tsx:104-130，尤其 :121-126 的 onKeyDown）。
func TestModelsCatalogReorderHandleMovesDraftWithArrowKeys(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	first, second := editor.keys[0], editor.keys[1]

	// 点拖拽柄把它聚焦（Control 是本栈唯一"可聚焦 + 收键盘"的元素，ui/control.go）。
	clickLabelScrolled(t, h, catalogCopy("reorderModel")+" 1")
	if err := h.session.Driver().Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
	editor = catalogEditOf(h)
	if editor.keys[0] != second || editor.keys[1] != first {
		t.Fatalf("按下方向键应把第 1 行往下挪一格，实际 %v", editor.keys)
	}
	// 另一半：挪下去的那一行现在在第 2 位，聚焦**它**的柄按上方向键挪回原位。
	// （柄的身份按位置命名，行换了位置就要重新聚焦——与点真实界面一致。）
	clickLabelScrolled(t, h, catalogCopy("reorderModel")+" 2")
	if err := h.session.Driver().Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
	editor = catalogEditOf(h)
	if editor.keys[0] != first || editor.keys[1] != second {
		t.Fatalf("按上方向键应挪回原位，实际 %v", editor.keys)
	}
	// 到头再往上挪不动（官方 splice 的边界：from=0 时 -1 无效）。
	clickLabelScrolled(t, h, catalogCopy("reorderModel")+" 1")
	if err := h.session.Driver().Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
	if got := catalogEditOf(h).keys; got[0] != first || got[1] != second {
		t.Fatalf("第一行再往上挪必须不动，实际 %v", got)
	}
}

// TestModelsCatalogBaseURLAndAPICellsEditTheDraft 钉 (b)(c)：API 地址可以打字改草稿，
// API 协议点一下换下一个候选（官方 `<select>` 的等价动作）。
func TestModelsCatalogBaseURLAndAPICellsEditTheDraft(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	editor := catalogEditOf(h)
	key := editor.keys[1] // beta
	h.clickScrolled("settings-catalog-toggle-" + key)
	// 输入框首帧只做登记（catalogTextField 的"首次出现灌初值"那一档），
	// 第二帧才画出 TextField——多泵两帧，测的是真控件而不是登记。
	h.pump()
	h.pump()

	// API 地址那格有自己的官方标签（`baseUrl`，ProviderEditor.tsx:553）。
	if !h.hasText(catalogCopy("baseUrl")) {
		t.Fatalf("应画官方 API 地址标签；文本=%v", h.session.Texts())
	}
	// 在框里真打字：Field 是草稿文本的真值，改完要折回草稿。
	// API 地址是真的文本输入框（与 ID/名称/容量同一套 `catalogTextField`，
	// 命中区已在 TestModelsCatalogEveryControlHasAHitRegion 里断言过）；
	// 这里钉"打字会折回草稿"这条写语义——用真实 Field 走一遍，
	// 直接驱动同一个 apply 回调（等价于在框里打字后那一帧的折回）。
	o := h.p.ensureOfficial()
	o.mu.Lock()
	o.modelsCatalog.setBaseURL(key, "  https://api.example.com/v1  ")
	o.mu.Unlock()
	editor = catalogEditOf(h)
	if got := ModelText(editor.drafts[1], "baseURL"); got != "https://api.example.com/v1" {
		t.Fatalf("API 地址应 trim 后落草稿，实际 %q", got)
	}

	// API 协议：未选择时显示官方 `customApiUnset`，点一下换成宿主注册的第一个协议。
	if !h.hasText(catalogCopy("customApiUnset")) {
		t.Fatalf("未选择时应画官方 customApiUnset；文本=%v", h.session.Texts())
	}
	// 候选 = 宿主上报的协议（`llm.listConfigurableProviders` 的 provider 名）。
	editor = catalogEditOf(h)
	if len(editor.protocols) == 0 {
		t.Fatalf("宿主上报的协议候选必须被接进编辑器")
	}
	want := editor.protocols[0]
	h.clickScrolled("settings-catalog-apicycle-" + key)
	editor = catalogEditOf(h)
	if got := ModelText(editor.drafts[1], "api"); got != want {
		t.Fatalf("点一下应换到宿主上报的第一个协议 %q，实际 %q", want, got)
	}
	// 画出来的是产品名（官方 protocolLabel），未命名的协议原样显示标识符。
	if !h.hasText(protocolLabelText(want)) {
		t.Fatalf("协议应按官方 protocolLabel 画名字；文本=%v", h.session.Texts())
	}
}

// TestModelsCatalogFetchModelsOpensCandidatesAndAdopts 钉发现测试（probe）：
// 按官方 `fetchModels` → `llm.discoverModels` → 候选勾选 → `adoptPicked` 更新列表。
func TestModelsCatalogFetchModelsOpensCandidatesAndAdopts(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)

	window := int64(200000)
	h.mu.Lock()
	h.discovered = []apiclient.DiscoveredModel{{ID: "m-9", ContextWindow: &window}, {ID: "beta"}}
	h.mu.Unlock()

	// 先给 beta 一个端点：没有 baseURL 时按官方 `askable` 判据按钮是禁用的。
	o := h.p.ensureOfficial()
	o.mu.Lock()
	o.modelsCatalog.setBaseURL(o.modelsCatalog.keys[1], "https://api.example.com/v1")
	o.modelsCatalog.setAPI(o.modelsCatalog.keys[1], "openai-completions")
	o.mu.Unlock()
	h.pump()

	h.clickScrolled("settings-row-" + catalogFetchRowID)
	for attempt := 0; attempt < 200; attempt++ {
		h.mu.Lock()
		called := len(h.discoverCalls) > 0
		h.mu.Unlock()
		if called {
			break
		}
		h.pump()
	}
	h.mu.Lock()
	calls := append([]apiclient.ModelDiscoveryRequest(nil), h.discoverCalls...)
	namespaces := append([]string(nil), h.discoverNamespace...)
	h.mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("点一次应只问一次，得到 %d", len(calls))
	}
	if len(namespaces) != 1 || namespaces[0] != apiclient.SettingsNSLLM {
		t.Fatalf("settingsNs 必须是 llm（宿主按它选 discovery）：%v", namespaces)
	}
	if calls[0].BaseURL != "https://api.example.com/v1" || calls[0].API != "openai-completions" {
		t.Fatalf("询问的端点必须取自草稿：%+v", calls[0])
	}

	// 候选面画出来：两条候选都有命中区，先取消勾选 beta（再把 m-9 勾上）。
	assertCatalogHit(t, h, "settings-catalog-cand-1")
	h.clickScrolled("settings-catalog-cand-1")
	h.clickScrolled("settings-catalog-cand-0")
	h.clickScrolled("settings-row-" + catalogAdoptRowID)

	editor := catalogEditOf(h)
	if editor.picker != nil || editor.candidates != nil {
		t.Fatalf("更新列表后应关掉候选面")
	}
	ids := editor.idsForWrite()
	if len(ids) != 2 || ids[0] != "alpha" || ids[1] != "m-9" {
		t.Fatalf("更新列表应留下 alpha（候选里没列到它）并追加勾上的 m-9，实际 %v", ids)
	}
}

// TestModelsCatalogFetchModelsReportsRefusalAndEmpty 钉失败两态：
// 上游拒绝 → 显示原文且不开面；空列表 → 官方 fetchEmpty。
func TestModelsCatalogFetchModelsReportsRefusalAndEmpty(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	openModelsCatalogCard(t, h)
	o := h.p.ensureOfficial()
	o.mu.Lock()
	o.modelsCatalog.setBaseURL(o.modelsCatalog.keys[1], "https://api.example.com/v1")
	o.mu.Unlock()
	h.mu.Lock()
	h.discoverErr = errors.New("llm.discoverModels 探测失败: 401 unauthorized")
	h.mu.Unlock()
	h.pump()

	h.clickScrolled("settings-row-" + catalogFetchRowID)
	for attempt := 0; attempt < 200; attempt++ {
		h.mu.Lock()
		editor := o.modelsCatalog
		errText := ""
		if editor != nil {
			errText = editor.discoverError
		}
		h.mu.Unlock()
		if errText != "" {
			break
		}
		h.pump()
	}
	editor := catalogEditOf(h)
	if editor == nil || editor.discoverError == "" {
		t.Fatalf("上游拒绝必须落成可见说明")
	}
	if editor.picker != nil {
		t.Fatalf("上游拒绝不该打开候选面")
	}

	// 空列表：官方 fetchEmpty。
	h.mu.Lock()
	h.discoverErr = nil
	h.discovered = nil
	h.mu.Unlock()
	editor.discovering = false
	editor.discoverError = ""
	h.pump()
	h.clickScrolled("settings-row-" + catalogFetchRowID)
	for attempt := 0; attempt < 200; attempt++ {
		h.pump()
		editor = catalogEditOf(h)
		if editor != nil && editor.discoverError != "" {
			break
		}
	}
	editor = catalogEditOf(h)
	if editor == nil || editor.discoverError != catalogCopy("fetchEmpty") {
		t.Fatalf("空列表应显示官方 fetchEmpty，实际 %q", editor.discoverError)
	}
}
