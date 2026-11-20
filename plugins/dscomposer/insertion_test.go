package dscomposer

// insertion_test.go —— 草稿插入能力（官方 `inputActions.captureInsertion/insertText`）。
//
// 判据逐条对着 rc.2 随包的会话输入门面（`tmp/dsh-client-ui-conversation.js`）：
//
//	captureInsertion :14942-14945 → 选区优先，否则光标在正文末尾，再带 draftRev
//	insertText       :14946-14950 → 版本不符 / 无编辑器 → false，否则替换 span 区间
//	insertAsyncText  :14772-14779 → 先清芯片占位码位（REFERENCE_PLACEHOLDER_RE，:14550）
//
// 另外钉住装配面：`Provides()` 必须交出 `KeyDraftInsertion`，且 `Register` 里真的
// `Provide` 得出来（漏了就是"插件编译得动、壳取不到能力"，CordisView 那次的踩法）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/text"
)

// newInsertionTestPlugin 造一个装好的 composer 插件（Register 跑过、editor 已 Bind）。
func newInsertionTestPlugin(t *testing.T) (*Plugin, *uikernel.Kernel, draftInsertion) {
	t.Helper()
	kernel := uikernel.New("composer-insertion-test")
	kernel.Root().Provide(pluginapi.KeyTheme, dscore.Dark())
	kernel.Root().Provide(pluginapi.KeyPrimitives, &dscore.Primitives{Theme: dscore.Dark()})
	plugin := &Plugin{}
	if err := plugin.Register(kernel.Root()); err != nil {
		t.Fatalf("Register 失败：%v", err)
	}
	service, ok := kernel.Get(pluginapi.KeyDraftInsertion).(draftInsertion)
	if !ok {
		t.Fatalf("Register 没有交出 KeyDraftInsertion（实际 %T）", kernel.Get(pluginapi.KeyDraftInsertion))
	}
	return plugin, kernel, service
}

// TestComposerProvidesDraftInsertion 钉住装配面。
func TestComposerProvidesDraftInsertion(t *testing.T) {
	plugin, _, _ := newInsertionTestPlugin(t)
	found := false
	for _, key := range plugin.Provides() {
		if key == pluginapi.KeyDraftInsertion {
			found = true
		}
	}
	if !found {
		t.Fatalf("Provides() = %v，必须含 %q", plugin.Provides(), pluginapi.KeyDraftInsertion)
	}
}

// TestCaptureInsertionOnEmptyDraft：空草稿的光标在 0（官方 caretSpan 的末尾档）。
func TestCaptureInsertionOnEmptyDraft(t *testing.T) {
	_, _, service := newInsertionTestPlugin(t)
	span, ok := service.CaptureInsertion()
	if !ok {
		t.Fatal("装好之后必须有可插入的草稿")
	}
	if span.Start != 0 || span.End != 0 || span.DraftRev != 0 {
		t.Fatalf("空草稿 span = %+v，期望 {0 0 0}", span)
	}
}

// TestCaptureInsertionUsesSelection：有选区就报选区（官方 caretSpan 的第一支）。
func TestCaptureInsertionUsesSelection(t *testing.T) {
	plugin, _, service := newInsertionTestPlugin(t)
	plugin.field.SetText("你好世界")
	plugin.syncDraftRev()
	plugin.editor.SetSelection(textRange(1, 3))

	span, ok := service.CaptureInsertion()
	if !ok {
		t.Fatal("要有 span")
	}
	if span.Start != 1 || span.End != 3 {
		t.Fatalf("span = %+v，期望 {1 3}", span)
	}
	if span.DraftRev != plugin.draftRev || span.DraftRev == 0 {
		t.Fatalf("DraftRev = %d，期望当前版本 %d 且非零", span.DraftRev, plugin.draftRev)
	}
}

// TestInsertTextReplacesSpan：插到 span 处（**替换**区间），文本顺序正确。
func TestInsertTextReplacesSpan(t *testing.T) {
	plugin, _, service := newInsertionTestPlugin(t)
	plugin.field.SetText("你好世界")
	plugin.syncDraftRev()

	span := pluginapi.InsertionSpan{Start: 2, End: 2, DraftRev: plugin.draftRev}
	if !service.InsertText("，", span) {
		t.Fatal("版本对得上，插入必须成功")
	}
	if got := plugin.draftText(); got != "你好，世界" {
		t.Fatalf("草稿 = %q，期望 %q", got, "你好，世界")
	}

	// 替换选区：把「世界」换成「DSH」。
	plugin.syncDraftRev()
	replace := pluginapi.InsertionSpan{Start: 3, End: 5, DraftRev: plugin.draftRev}
	if !service.InsertText("DSH", replace) {
		t.Fatal("替换选区必须成功")
	}
	if got := plugin.draftText(); got != "你好，DSH" {
		t.Fatalf("草稿 = %q，期望 %q", got, "你好，DSH")
	}
}

// TestInsertTextRejectsStaleRevision：草稿变过 → false，且**一个字都不许动**。
//
// 这是官方那条并发守卫（:14948），也是听写"几秒后回来"最容易出事的地方。
func TestInsertTextRejectsStaleRevision(t *testing.T) {
	plugin, _, service := newInsertionTestPlugin(t)
	plugin.field.SetText("用户正在写的东西")
	plugin.syncDraftRev()

	stale := pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: plugin.draftRev}
	// 采集之后用户又打了字。
	plugin.field.SetText("用户正在写的东西，又加了点")
	plugin.syncDraftRev()

	before := plugin.draftText()
	if service.InsertText("识别结果", stale) {
		t.Fatal("版本不符必须拒绝")
	}
	if got := plugin.draftText(); got != before {
		t.Fatalf("拒绝之后草稿被改了：%q → %q", before, got)
	}
}

// TestInsertTextRejectsEmptyAndNoEditor：两条防御。
func TestInsertTextRejectsEmptyAndNoEditor(t *testing.T) {
	// 没有 Register 过：editor 为 nil。
	raw := &Plugin{}
	if (draftInsertion{p: raw}).InsertText("x", pluginapi.InsertionSpan{}) {
		t.Fatal("没有编辑器时必须拒绝")
	}
	if _, ok := (draftInsertion{p: raw}).CaptureInsertion(); ok {
		t.Fatal("没有编辑器时不该报有 span")
	}

	plugin, _, service := newInsertionTestPlugin(t)
	plugin.field.SetText("abc")
	plugin.syncDraftRev()
	span := pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: plugin.draftRev}
	if service.InsertText("", span) {
		t.Fatal("空内容必须拒绝（否则等于静默删掉选区）")
	}
	if got := plugin.draftText(); got != "abc" {
		t.Fatalf("草稿 = %q", got)
	}
}

// TestInsertTextStripsChipPlaceholders：识别结果里的芯片占位码位要清掉
// （官方 REFERENCE_PLACEHOLDER_RE `/[\uE100-\uE11D\uFFFC]/gu`）。
func TestInsertTextStripsChipPlaceholders(t *testing.T) {
	plugin, _, service := newInsertionTestPlugin(t)
	plugin.syncDraftRev()
	span := pluginapi.InsertionSpan{Start: 0, End: 0, DraftRev: plugin.draftRev}

	if !service.InsertText("见\ufffc文件\ue101目录", span) {
		t.Fatal("清掉占位符之后还有正文，必须插入")
	}
	if got := plugin.draftText(); got != "见文件目录" {
		t.Fatalf("草稿 = %q，期望 %q（占位码位已清）", got, "见文件目录")
	}
}

// TestSyncDraftRevBumpsOnContentChange：版本号只在内容真的变了时 +1。
func TestSyncDraftRevBumpsOnContentChange(t *testing.T) {
	plugin, _, _ := newInsertionTestPlugin(t)
	plugin.syncDraftRev() // 空 → 空，不动
	if plugin.draftRev != 0 {
		t.Fatalf("内容没变不该涨版本：%d", plugin.draftRev)
	}
	plugin.field.SetText("a")
	plugin.syncDraftRev()
	if plugin.draftRev != 1 {
		t.Fatalf("内容变了要 +1，实际 %d", plugin.draftRev)
	}
	plugin.syncDraftRev() // 同一内容
	if plugin.draftRev != 1 {
		t.Fatalf("同一内容不该再涨：%d", plugin.draftRev)
	}
}

// textRange 造一个 viva 选区（局部包装，避免测试文件重复 import text 包）。
func textRange(start, end int) text.Range { return text.Range{Start: start, End: end} }
