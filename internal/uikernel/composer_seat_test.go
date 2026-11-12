package uikernel

// composer_seat_test.go —— 官方 composer 座位（`conversation.composer`）的 dispatch 断言。
//
// 背景（GP 报告 `tmp/GO8-RC2/reports/GP-paging-approval.md` §5 第 1 条）：
// 官方座位此前**从来不会被 dispatch**——中栏整列走兼容桥（`main.*` 旧槽），
// 官方子槽没人画。本文件把「座位真的派发了」变成机器断言，四条判据：
//
//	① 官方座位有占用方 → 这一帧 dispatch 它（不是画了没人看）；
//	② 同一帧 `main.composer` **不** dispatch（二选一，不双画）；
//	③ 官方座位没人、旧槽有人 → 退回 `main.composer`（纯旧插件集不黑屏）；
//	④ 兼容桥里（中栏 header/body 走旧槽，`-shell official` 今天的产品路径）
//	   composer 也走官方座位——否则「切到官方档」永远只是换了个壳。

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// seatFrame 画一帧骨架，返回每个槽被 dispatch 的次数与测点矩形。
//
// 矩形必须在 `Draw` 返回**之后**读（`box.Rect()` 的有效期是 End 解算完成）：
// 在 measure 回调里读只会拿到零矩形。
func seatFrame(t *testing.T, opts ShellOptions, occupied map[pluginapi.Slot]bool) (map[pluginapi.Slot]int, map[string]gfx.Rect) {
	t.Helper()
	theme := dscore.Dark()
	env, canvas := newSkeletonEnv(1182, 832, func(slot pluginapi.Slot) bool { return occupied[slot] })
	counts := map[pluginapi.Slot]int{}
	boxes := map[string]ui.Box{}
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		FirstScreenWithMeasure(f, env, opts, func(slot pluginapi.Slot) { counts[slot]++ }, func(name string, box ui.Box) {
			boxes[name] = box
		})
	})
	rects := make(map[string]gfx.Rect, len(boxes))
	for name, box := range boxes {
		rects[name] = box.Rect()
	}
	return counts, rects
}

// officialSeatOpts 是「官方骨架、没有旧部件」的一帧输入。
func officialSeatOpts() ShellOptions {
	return ShellOptions{Tabs: true, SidebarPreferred: 280, RightbarOn: true}
}

// TestOfficialComposerSeatIsDispatched ①：官方座位有占用方时这一帧真的派发。
func TestOfficialComposerSeatIsDispatched(t *testing.T) {
	counts, boxes := seatFrame(t, officialSeatOpts(), map[pluginapi.Slot]bool{OffConvComposer: true})
	if counts[OffConvComposer] != 1 {
		t.Fatalf("官方 composer 座位 dispatch 次数 = %d，want 1（counts=%v）", counts[OffConvComposer], counts)
	}
	if counts[pluginapi.SlotMainComposer] != 0 {
		t.Fatalf("官方座位活着时旧槽 main.composer 也被 dispatch（双画）：%v", counts)
	}
	// 占用方接管整个 composer 区：骨架**不再**画一份 InputBar 占位卡片。
	if rect, ok := boxes["InputBar.card"]; ok {
		t.Fatalf("官方座位有人时骨架还画了占位卡片 %+v（与占用方双画）", rect)
	}
}

// TestComposerSeatFallsBackToLegacySeat ③：官方座位没人、旧槽有人 → 退旧槽。
func TestComposerSeatFallsBackToLegacySeat(t *testing.T) {
	opts := officialSeatOpts()
	// 壳的判据：官方座位没人、旧槽有人 → 兼容桥打开（见 cmd/app/shell.go）。
	opts.BridgeLegacyComposer = true
	counts, _ := seatFrame(t, opts, map[pluginapi.Slot]bool{pluginapi.SlotMainComposer: true})
	if counts[pluginapi.SlotMainComposer] != 1 {
		t.Fatalf("官方座位没人时 main.composer dispatch 次数 = %d，want 1（counts=%v）", counts[pluginapi.SlotMainComposer], counts)
	}
	if counts[OffConvComposer] != 0 {
		t.Fatalf("官方座位没人却 dispatch 了它：%v", counts)
	}
}

// TestComposerSeatDrawsOfficialSkeletonWhenEmpty 官方座位没人时画的是**官方骨架**
// （InputBar 卡片 + 控件行），不是空白——这样没有 composer 插件的宿主也有结构。
func TestComposerSeatDrawsOfficialSkeletonWhenEmpty(t *testing.T) {
	_, boxes := seatFrame(t, officialSeatOpts(), nil)
	card, ok := boxes["InputBar.card"]
	if !ok {
		t.Fatal("官方座位没人时骨架没画 InputBar 卡片")
	}
	// 官方卡片 = padding-top 8 + 输入区 36 + gap 12 + 控件行 42 = 98。
	if card.Height != ComposerSeatHeight-(ComposerDockPadTop+ComposerDockMinH+ComposerPadBottom) {
		t.Fatalf("骨架卡片高 = %v，want %v", card.Height, ComposerSeatHeight-(ComposerDockPadTop+ComposerDockMinH+ComposerPadBottom))
	}
	if card.Height != 98 {
		t.Fatalf("骨架卡片高 = %v，官方 InputBar 卡片是 98（8+36+12+42）", card.Height)
	}
}

// TestBridgeLegacyKeepsComposerOnOfficialSeat ④：兼容桥里 composer 走官方座位。
//
// 这是 `-shell official` 今天的产品路径：sidebar / header / transcript 还是旧槽
// （dssidebar / dstopbar / dsconversation 未迁完），composer 已经有官方座位占用方。
// 判据必须**分段**：整列一个开关的话 composer 永远只能跟着旧路径走。
func TestBridgeLegacyKeepsComposerOnOfficialSeat(t *testing.T) {
	opts := officialSeatOpts()
	opts.BridgeLegacy = true
	opts.BridgeLegacyComposer = false
	counts, _ := seatFrame(t, opts, map[pluginapi.Slot]bool{
		pluginapi.SlotSidebarList:  true,
		pluginapi.SlotMainHeader:   true,
		pluginapi.SlotMainBody:     true,
		OffConvComposer:            true,
		pluginapi.SlotMainComposer: true, // 旧槽也有人（dsgoal/dsstatusbar 等），但这一帧不该被画
	})
	// 旧槽：header / 正文 / 左栏仍然要出来（兼容桥还在）。
	for _, slot := range []pluginapi.Slot{pluginapi.SlotMainHeader, pluginapi.SlotMainBody, pluginapi.SlotSidebarList} {
		if counts[slot] != 1 {
			t.Fatalf("兼容桥下旧槽 %q dispatch 次数 = %d，want 1（counts=%v）", slot, counts[slot], counts)
		}
	}
	// composer：只有官方座位活。
	if counts[OffConvComposer] != 1 {
		t.Fatalf("兼容桥下官方 composer 座位 dispatch 次数 = %d，want 1（counts=%v）", counts[OffConvComposer], counts)
	}
	if counts[pluginapi.SlotMainComposer] != 0 {
		t.Fatalf("兼容桥下 main.composer 也被 dispatch（双画）：%v", counts)
	}
}

// TestBridgeLegacyComposerUsesLegacySlotWhenOfficialEmpty 纯旧插件集：
// 官方座位没人时 compat 桥仍然画 main.composer（不黑屏）。
func TestBridgeLegacyComposerUsesLegacySlotWhenOfficialEmpty(t *testing.T) {
	opts := officialSeatOpts()
	opts.BridgeLegacy = true
	opts.BridgeLegacyComposer = true
	counts, _ := seatFrame(t, opts, map[pluginapi.Slot]bool{
		pluginapi.SlotMainBody:     true,
		pluginapi.SlotMainComposer: true,
	})
	if counts[pluginapi.SlotMainComposer] != 1 {
		t.Fatalf("纯旧插件集 main.composer dispatch 次数 = %d，want 1（counts=%v）", counts[pluginapi.SlotMainComposer], counts)
	}
	if counts[OffConvComposer] != 0 {
		t.Fatalf("官方座位没人却 dispatch 了它：%v", counts)
	}
}

// TestComposerSeatHeightIsOfficialArithmetic 座位高度 = 官方三段相加。
//
// 卡片 98（8+36+12+42）+ dock 26（4+22）+ 底边距 4 = 128。
// 控件行 42 而不是 36：主按钮 34 比 trigger 28 高，`.row` 被子元素撑开。
func TestComposerSeatHeightIsOfficialArithmetic(t *testing.T) {
	if ComposerRowHeight != ComposerPrimarySize+ComposerRowPadTop+ComposerRowPadBot {
		t.Fatalf("控件行高 = %v，官方 = 主按钮 %v + 上下 padding %v/%v",
			ComposerRowHeight, ComposerPrimarySize, ComposerRowPadTop, ComposerRowPadBot)
	}
	card := ComposerCardPadTop + ComposerInputMinH + ComposerCardGap + ComposerRowHeight
	if card != 98 {
		t.Fatalf("卡片高 = %v，官方 98（8+36+12+42）", card)
	}
	dock := ComposerDockPadTop + ComposerDockMinH
	if dock != 26 {
		t.Fatalf("dock 高 = %v，官方 26（4+22）", dock)
	}
	if ComposerSeatHeight != card+dock+ComposerPadBottom {
		t.Fatalf("座位高 = %v，官方三段相加 = %v", ComposerSeatHeight, card+dock+ComposerPadBottom)
	}
}
