//go:build darwin || windows || linux

package dsentrycustom

// entry_focus_test.go —— #17 ③ 键盘模态那半边（2026-11-21 落地）。
//
// 官方在 `@media (hover:hover)` 里给行尾那一格写了**两条并列**规则
// （OFF:EntryMenu.module.css:28-31）：
//
//	.sidebarEntry:hover .pinAnchor            { opacity: 1 }   ← 指针
//	html[data-input-modality='keyboard']
//	  .sidebarEntry:focus-within .pinAnchor   { opacity: 1 }   ← 键盘
//	.sidebarEntry:hover .activeMark           { opacity: 0 }
//	html[data-input-modality='keyboard']
//	  .sidebarEntry:focus-within .activeMark  { opacity: 0 }
//
// 判据拆成两件事：①「最近一次输入是键盘」②「焦点在这一行**内部**」。
// viva 的 `Canvas.FocusVisible()` 已经把①合进去了（只在键盘模态下非零），
// 所以本仓只需要比对「焦点身份 == 这一行的任一个可停靠命中」。
//
// 这里用**真 Tab**走 viva 的焦点遍历去拿焦点——不是在测试里塞一个假 canvas，
// 因为要证明的恰恰是"这些行真的是键盘停靠点"（否则规则永远触发不到，等于没画）。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// tabUntil 按 Tab 直到焦点落在 want 里任一身份上（最多 16 次），回最后的焦点身份。
//
// 16 是"这一小片区域里可停靠身份"的宽松上限：本 fixture 只有一两条条目 + 两行 footer。
func (f *entryFixture) tabUntil(want ...gfx.ID) gfx.ID {
	f.t.Helper()
	var focused gfx.ID
	for i := 0; i < 16; i++ {
		if err := f.session.Keys("tab"); err != nil {
			f.t.Fatalf("第 %d 次 Tab 失败：%v", i+1, err)
		}
		f.pump()
		focused = f.session.Snapshot().Focused
		for _, target := range want {
			if focused == target {
				return focused
			}
		}
	}
	return focused
}

// TestKeyboardFocusInsideShowsPin 钉住 `:focus-within` 那两条：
// 指针**不在**行上（所以 hover 不成立），只靠键盘焦点走进这一行，固定按钮要显形、对勾要隐去。
func TestKeyboardFocusInsideShowsPin(t *testing.T) {
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, hoverActiveEntries(), &fakeNavigation{})
	defer f.close()

	row := f.mustHit("entry-open-schedules").Bounds
	rowID := f.tree.ID(f.feat, "entry-open-schedules")
	pinID := f.tree.ID(f.feat, "entry-pin-schedules")

	// 指针挪到角落：这一档**不许**靠 hover 冒充（官方 :28 与 :29 是两条并列规则）。
	f.moveTo(2, 2)
	if got := countVisible(f.rowPrims(row, f.theme.LabelTertiary)); got != 0 {
		t.Fatalf("静止态（指针在角落）画出了 %d 枚固定按钮图元：`.pinAnchor{opacity:0}`（:26）", got)
	}
	if got := countVisible(f.rowPrims(row, f.theme.LabelPrimary)); got == 0 {
		t.Fatal("静止态没画出对勾：`.sidebarEntry .activeMark{opacity:1}`（:27）")
	}

	focused := f.tabUntil(rowID, pinID)
	if focused != rowID && focused != pinID {
		t.Fatalf("16 次 Tab 焦点没走到这一行（最后焦点=%v）——行/固定按钮不是键盘停靠点，"+
			"官方那条 :focus-within 规则永远触发不到", focused)
	}
	if got := countVisible(f.rowPrims(row, f.theme.LabelTertiary)); got == 0 {
		t.Fatalf("键盘焦点在这一行内部（%v）、指针不在行上，固定按钮却没显形："+
			"官方 `html[data-input-modality='keyboard'] .sidebarEntry:focus-within .pinAnchor{opacity:1}`（:29）", focused)
	}
	if got := countVisible(f.rowPrims(row, f.theme.LabelPrimary)); got != 0 {
		t.Fatalf("键盘焦点态还画着 %d 枚对勾：官方 :31 在这一档把 .activeMark 隐去", got)
	}
}

// TestKeyboardFocusIsScopedToItsOwnRow 钉住 `:focus-within` 的**范围**：
// 焦点落在 B 行时，A 行不能跟着显形（那是"整个面板:focus-within"的错实现）。
func TestKeyboardFocusIsScopedToItsOwnRow(t *testing.T) {
	entries := []pluginapi.EntryMenuEntry{
		{ID: "alpha", Label: "甲", Pinned: false},
		{ID: "beta", Label: "乙", Pinned: false},
	}
	f := newEntryFixture(t, pluginapi.EntryRegionSidebar, entries, &fakeNavigation{})
	defer f.close()

	alphaRow := f.mustHit("entry-open-alpha").Bounds
	alphaID := f.tree.ID(f.feat, "entry-open-alpha")
	alphaPinID := f.tree.ID(f.feat, "entry-pin-alpha")
	betaID := f.tree.ID(f.feat, "entry-open-beta")

	f.moveTo(2, 2)
	focused := f.tabUntil(betaID)
	if focused != betaID {
		t.Fatalf("16 次 Tab 焦点没走到乙行（最后焦点=%v）", focused)
	}
	if got := countVisible(f.rowPrims(alphaRow, f.theme.LabelTertiary)); got != 0 {
		t.Fatalf("焦点在另一行时甲行画出了 %d 枚固定按钮图元：`:focus-within` 只管自己这一行", got)
	}
	if focused == alphaID || focused == alphaPinID {
		t.Fatalf("焦点身份应是乙行，实际 %v", focused)
	}
}
