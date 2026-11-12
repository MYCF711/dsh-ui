package dssettings

import (
	"os"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// TestDialogUsesAriaLabelledByNotAriaLabel 钉住 rc.2 D3。
//
// rc.1 台账写面板有 `aria-label`；rc.2 实测是 `aria-labelledby={titleId}`
// （SettingsRoot.tsx:61/73/77-78）。这条真变的意义是：读屏软件要**读到标题
// 文本**，只给一个 label 字符串做不到（官方用 useId 指向 navTitle 节点）。
func TestDialogUsesAriaLabelledByNotAriaLabel(t *testing.T) {
	modal := NewModalLayer(nil)
	modal.Open("settings-panel-title", []string{"a", "b"}, "b", "composer")
	sem := modal.Semantics()

	if sem.Role != "dialog" {
		t.Fatalf("role = %q，官方 role=\"dialog\"", sem.Role)
	}
	if !sem.AriaModal {
		t.Fatal("aria-modal 必须为 true")
	}
	if sem.AriaLabelledBy == "" {
		t.Fatal("aria-labelledby 不能为空（否则是未命名对话框）")
	}
	if sem.AriaLabelledBy != "settings-panel-title" {
		t.Fatalf("aria-labelledby = %q，应指向 navTitle 节点", sem.AriaLabelledBy)
	}
	if sem.AriaLabel != "" {
		t.Fatalf("rc.2 不再用 aria-label（实得 %q）；两者同时出现会让读屏软件读错标题", sem.AriaLabel)
	}
	if sem.TabIndex != -1 {
		t.Fatalf("tabIndex = %d，官方 tabIndex={-1}", sem.TabIndex)
	}
	if sem.PortalTarget != PortalTargetBody {
		t.Fatalf("portal 目标 = %q，官方 createPortal(…, document.body)", sem.PortalTarget)
	}
	if sem.ZIndex != 1000 {
		t.Fatalf("overlay z-index = %v，官方 1000", sem.ZIndex)
	}
}

// TestModalLayerInitialFocusAndRestore 钉住 useModalLayer 的焦点语义。
func TestModalLayerInitialFocusAndRestore(t *testing.T) {
	modal := NewModalLayer(nil)
	if modal.Opened() || modal.Inert() {
		t.Fatal("初始应关闭且不 inert")
	}
	modal.Open("title", []string{"close", "nav-general", "row-1"}, "nav-general", "composer")
	if !modal.Opened() {
		t.Fatal("Open 后应处于打开态")
	}
	if !modal.Inert() {
		t.Fatal("模态打开期间 root 必须 inert（否则 Tab 会跑到面板外）")
	}
	if got := modal.Focused(); got != "nav-general" {
		t.Fatalf("初始焦点 = %q，官方靠 data-modal-autofocus 落在当前 section 的 nav cell", got)
	}
	restore, ok := modal.Close()
	if !ok {
		t.Fatal("Close 应报告确实关闭了")
	}
	if restore != "composer" {
		t.Fatalf("恢复焦点 = %q，应回到打开前的目标", restore)
	}
	if modal.Opened() || modal.Inert() {
		t.Fatal("关闭后应解除 inert")
	}
	// 幂等：再关一次不重复回调。
	opened, closed := modal.Counts()
	if opened != 1 || closed != 1 {
		t.Fatalf("打开/关闭次数 = %d/%d，期望 1/1", opened, closed)
	}
	if _, ok := modal.Close(); ok {
		t.Fatal("重复关闭应返回 false")
	}
}

// TestModalLayerTabCyclesAndWraps 钉住 Tab / Shift+Tab 循环（官方 Tab 陷阱）。
func TestModalLayerTabCyclesAndWraps(t *testing.T) {
	called := 0
	modal := NewModalLayer(func() { called++ })
	modal.Open("title", []string{"a", "b", "c"}, "a", "")

	tab := ui.KeyEvent{Key: ui.KeyTab}
	shiftTab := ui.KeyEvent{Key: ui.KeyTab, Modifiers: ui.ModifierShift}

	steps := []struct {
		ev   ui.KeyEvent
		want string
	}{
		{tab, "b"},
		{tab, "c"},
		{tab, "a"},      // 末尾回到开头
		{shiftTab, "c"}, // 开头回到末尾
		{shiftTab, "b"},
	}
	for i, step := range steps {
		if !modal.HandleKey(step.ev) {
			t.Fatalf("第 %d 步 Tab 没被消费（焦点会跑到面板外）", i)
		}
		if got := modal.Focused(); got != step.want {
			t.Fatalf("第 %d 步焦点 = %q，期望 %q", i, got, step.want)
		}
	}
	if called != 0 {
		t.Fatalf("Tab 不该触发关闭回调（实得 %d 次）", called)
	}
}

// TestModalLayerEscapeCloses 钉住 Escape → 关闭 + 恢复焦点。
func TestModalLayerEscapeCloses(t *testing.T) {
	called := 0
	modal := NewModalLayer(func() { called++ })
	modal.Open("title", []string{"a"}, "a", "trigger")
	if !modal.HandleKey(ui.KeyEvent{Key: ui.KeyEscape}) {
		t.Fatal("Escape 必须被模态层消费")
	}
	if modal.Opened() {
		t.Fatal("Escape 后应关闭")
	}
	if called != 1 {
		t.Fatalf("关闭回调调用 %d 次，期望 1", called)
	}
}

// TestModalLayerIgnoresOtherKeys 钉住「不抢别的键」。
func TestModalLayerIgnoresOtherKeys(t *testing.T) {
	modal := NewModalLayer(nil)
	modal.Open("title", []string{"a"}, "a", "")
	for _, ev := range []ui.KeyEvent{
		{Key: ui.KeyDown}, {Key: ui.KeyUp}, {Key: ui.KeyEnter},
		{Key: ui.KeyF2}, {Key: ui.KeySpace, Text: "x"},
	} {
		if modal.HandleKey(ev) {
			t.Fatalf("键 %v 不该被模态层消费（面板自己的行导航要用）", ev.Key)
		}
	}
	// 关着的时候连 Escape 都不吃。
	modal.Close()
	if modal.HandleKey(ui.KeyEvent{Key: ui.KeyEscape}) {
		t.Fatal("关着时不应消费 Escape")
	}
}

// TestModalLayerSetFocusablesFollowsSectionSwitch 钉住 section 切换后的焦点迁移。
func TestModalLayerSetFocusablesFollowsSectionSwitch(t *testing.T) {
	modal := NewModalLayer(nil)
	modal.Open("title", []string{"close", "nav-general"}, "nav-general", "")
	// 新 section 的行列表里没有旧焦点 → 回退到 autofocus（官方 data-modal-autofocus）。
	modal.SetFocusables([]string{"close", "nav-models", "row-a"}, "nav-models")
	if got := modal.Focused(); got != "nav-models" {
		t.Fatalf("切换后焦点 = %q，应回到新 section 的 autofocus", got)
	}
	// 当前焦点还在新列表里时**不跳**。
	modal.SetFocusables([]string{"close", "nav-models", "row-b"}, "row-b")
	if got := modal.Focused(); got != "nav-models" {
		t.Fatalf("焦点应保持 %q，实得 %q", "nav-models", got)
	}
}

// TestOfficialModalDoesNotUsePrimitivesModal 是**源码守卫**：rc.2 明确弃用
// primitives 的 Modal（`flex.Dialog` / `components.Dialog` / `primitives.Modal`），
// 改成 createPortal + useModalLayer。本仓的等价物必须是自己的 ModalLayer。
//
// 为什么用源码守卫而不是行为测试：引入 primitives Modal 也能画出「看起来一样」
// 的对话框，但它不带官方的 initial focus / Tab 循环 / inert 语义——那正是 D4。
func TestOfficialModalDoesNotUsePrimitivesModal(t *testing.T) {
	for _, file := range []string{"official_modal.go", "official_panel.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", file, err)
		}
		src := string(raw)
		for _, forbidden := range []string{
			"flex.Dialog", "components.Dialog", "primitives.Modal", "flex.Modal",
			"components.Modal", "ui.Modal",
		} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s 用了 %s：rc.2 已弃用 primitives Modal（D4），面板必须自建 portal + modal layer", file, forbidden)
			}
		}
	}
	// 反向：必须真的自己实现了这三件事。
	raw, err := os.ReadFile("official_modal.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, want := range []string{"PortalTargetBody", "ModalLayer", "handleOfficialKey", "Inert"} {
		if !strings.Contains(src, want) && !strings.Contains(string(mustRead(t, "official_panel.go")), want) {
			t.Fatalf("缺少 %q（portal/modal layer 的实现痕迹）", want)
		}
	}
}

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", name, err)
	}
	return raw
}
