// entrypins_test.go —— 官方三份 `*-pins` 的语义逐条钉住。
//
// 用例结构照抄官方 spec（OFF:ui-sidebar/tests/panel-pins.client.spec.ts 的四个
// `it`、ui-conversation/tests/view-pins.client.spec.ts、ui-sidebar-right/tests/
// guide-pins.client.spec.ts），断言的是**同一条行为**，只是换了 Go 的 api：
//
//	① 空表起步、同一份快照反复读不重建（这里用"值不变"表达；Go 侧没有
//	   React 的对象身份语义，快照身份由调用方按需承担）
//	② 取消固定按先后追加、固定按 filter 删除（顺序保持）
//	③ 固定条目不受理；重复设置同一态不算改、不发通知
//	④ 未注册的 id 留在表里
//	⑤ 没有持久化：新实例一律空表（官方 spec 显式断言 localStorage 里没有那条老键）
package entrypins

import (
	"reflect"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

func TestPanelsKeepUnpinnedInMemoryAndInOrder(t *testing.T) {
	pins := New()
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); got != nil {
		t.Fatalf("新实例必须是空表，实际 %v", got)
	}
	if !pins.SetPinned(pluginapi.EntryRegionSidebar, "notes", false) {
		t.Fatal("取消固定 notes 应真的改表")
	}
	if !pins.SetPinned(pluginapi.EntryRegionSidebar, "tasks", false) {
		t.Fatal("取消固定 tasks 应真的改表")
	}
	want := []string{"notes", "tasks"}
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); !reflect.DeepEqual(got, want) {
		t.Fatalf("未固定表 = %v，want %v（追加序）", got, want)
	}
	if pins.Pinned(pluginapi.EntryRegionSidebar, "notes") {
		t.Fatal("notes 已被取消固定，Pinned 应为 false")
	}
	if !pins.Pinned(pluginapi.EntryRegionSidebar, "plugins") {
		t.Fatal("固定条目 plugins 恒为固定")
	}
	// 固定 notes：从表里删掉，剩余顺序不变。
	if !pins.SetPinned(pluginapi.EntryRegionSidebar, "notes", true) {
		t.Fatal("固定 notes 应真的改表")
	}
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); !reflect.DeepEqual(got, []string{"tasks"}) {
		t.Fatalf("固定 notes 之后 = %v，want [tasks]", got)
	}
	// 没有持久化：换一个实例就是空表（官方 spec 的 "a new instance starts empty"）。
	if got := New().Unpinned(pluginapi.EntryRegionSidebar); len(got) != 0 {
		t.Fatalf("新实例继承了旧表：%v（固定态只存在内存里）", got)
	}
}

func TestPanelsIgnoreFixedAndRepeatedChoices(t *testing.T) {
	pins := New()
	pins.SetPinned(pluginapi.EntryRegionSidebar, "notes", false)
	cases := []struct {
		name string
		id   string
		pin  bool
	}{
		{name: "固定条目不受理", id: FixedPanelID, pin: false},
		{name: "已是未固定，再取消固定", id: "notes", pin: false},
		{name: "表里没有而且要求固定", id: "absent", pin: true},
	}
	for _, tc := range cases {
		if pins.SetPinned(pluginapi.EntryRegionSidebar, tc.id, tc.pin) {
			t.Errorf("%s：不该算改", tc.name)
		}
	}
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); !reflect.DeepEqual(got, []string{"notes"}) {
		t.Fatalf("表被不该改的调用改动了：%v", got)
	}
}

func TestPanelsKeepIdsOfUnregisteredEntries(t *testing.T) {
	pins := New()
	pins.SetPinned(pluginapi.EntryRegionSidebar, "removed-plugin", false)
	pins.SetPinned(pluginapi.EntryRegionSidebar, "notes", false)
	want := []string{"removed-plugin", "notes"}
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); !reflect.DeepEqual(got, want) {
		t.Fatalf("未注册的 id 必须留在表里：%v，want %v", got, want)
	}
}

func TestViewPinsUseFixedViewChat(t *testing.T) {
	pins := New()
	if pins.SetPinned(pluginapi.EntryRegionConversation, FixedViewID, false) {
		t.Fatal("固定 view（chat）不受理")
	}
	if !pins.Pinned(pluginapi.EntryRegionConversation, FixedViewID) {
		t.Fatal("chat 恒为固定")
	}
	pins.SetPinned(pluginapi.EntryRegionConversation, "trajectory", false)
	if got := pins.Unpinned(pluginapi.EntryRegionConversation); !reflect.DeepEqual(got, []string{"trajectory"}) {
		t.Fatalf("view 未固定表 = %v", got)
	}
	// 三个区域的表互不相干。
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); len(got) != 0 {
		t.Fatalf("会话头的固定态串到了左栏：%v", got)
	}
}

func TestGuidePinsUseFixedKind(t *testing.T) {
	pins := New()
	// 右栏 guide 的固定判据是 kind 前缀（`kind:id`），不是整串相等。
	if pins.SetPinned(pluginapi.EntryRegionRightbar, FixedGuideKind+":worktree", false) {
		t.Fatal("kind=files 的卡片不受理（guide-pins.ts:46 的 startsWith 判据）")
	}
	if !pins.SetPinned(pluginapi.EntryRegionRightbar, "terminal:shell-1", false) {
		t.Fatal("非 files 类的卡片可以取消固定")
	}
	if got := pins.Unpinned(pluginapi.EntryRegionRightbar); !reflect.DeepEqual(got, []string{"terminal:shell-1"}) {
		t.Fatalf("guide 未固定表 = %v", got)
	}
	if !pins.Pinned(pluginapi.EntryRegionRightbar, "files:anything") {
		t.Fatal("files:* 恒为固定")
	}
}

func TestNilStoreBehavesAsAllPinned(t *testing.T) {
	var pins *Store
	if !pins.Pinned(pluginapi.EntryRegionSidebar, "notes") {
		t.Fatal("nil 表按\"全部固定\"回答（默认值）")
	}
	if pins.SetPinned(pluginapi.EntryRegionSidebar, "notes", false) {
		t.Fatal("nil 表不写任何东西")
	}
	if got := pins.Unpinned(pluginapi.EntryRegionSidebar); got != nil {
		t.Fatalf("nil 表的未固定表必须是空：%v", got)
	}
}
