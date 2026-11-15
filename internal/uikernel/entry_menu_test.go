package uikernel

// entry_menu_test.go —— 区域 More 菜单面的四件事：开/关、派发、props 生命周期、几何。
//
// 这层是三个区域的**公共一半**（面 + dispatch），所以证据也只钉这一半：
//
//	① 关着 → 一个像素都不画、一次 dispatch 都不发
//	② 开着但壳没接 drawSlot → 同样不画（最小装配/单测环境不能画出空菜单面）
//	③ 开着且接了 → 座位**真的被派发一次**，期间 `env.EntryMenu` 带着 region /
//	   entries / 三个回调；派发之后立刻恢复（同一帧别的画读不到它）
//	④ 面的宽度按官方 `.moreMenu{width:288;max-width:calc(100vw-24px)}` 夹

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func entryMenuFrame(t *testing.T, opts EntryMenuOptions) (called int, seen []*pluginapi.EntryMenuProps, after *pluginapi.EntryMenuProps) {
	t.Helper()
	theme := dscore.Dark()
	env, canvas := newSkeletonEnv(1182, 832, func(pluginapi.Slot) bool { return true })
	env.Theme = theme
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	opts.Theme = theme
	opts.Bounds = env.Bounds
	opts.Anchor = ui.Rect{X: 12, Y: 300, Width: 256, Height: 36}
	opts.Dispatch = func(slot pluginapi.Slot) {
		called++
		if env.EntryMenu == nil {
			t.Fatalf("派发座位 %q 时 env.EntryMenu 是 nil：占用方拿不到 owner props", slot)
		}
		seen = append(seen, env.EntryMenu)
	}
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		DrawEntryMoreMenu(f, env, opts)
	})
	return called, seen, env.EntryMenu
}

func entryMenuEntries() []pluginapi.EntryMenuEntry {
	return []pluginapi.EntryMenuEntry{
		{ID: "plugins", Label: "插件", Pinned: true},
		{ID: "schedules", Label: "自动化任务", Pinned: false, Active: true},
	}
}

func TestEntryMoreMenuClosedDrawsNothingAndDispatchesNothing(t *testing.T) {
	called, _, _ := entryMenuFrame(t, EntryMenuOptions{
		Open: false, Slot: OffSidebarPanelActions,
		Region: pluginapi.EntryRegionSidebar, Entries: entryMenuEntries(),
	})
	if called != 0 {
		t.Fatalf("菜单关着却派发了 %d 次（会画出空菜单面）", called)
	}
}

func TestEntryMoreMenuWithoutDispatchIsNotDrawn(t *testing.T) {
	theme := dscore.Dark()
	env, canvas := newSkeletonEnv(1182, 832, func(pluginapi.Slot) bool { return true })
	env.Theme = theme
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	drawn := false
	view.Draw(canvas, env.Bounds, func(f ui.C) {
		drawn = DrawEntryMoreMenu(f, env, EntryMenuOptions{
			Open: true, Slot: OffSidebarPanelActions,
			Region: pluginapi.EntryRegionSidebar, Entries: entryMenuEntries(),
			Anchor: ui.Rect{X: 12, Y: 300, Width: 256, Height: 36},
			// Dispatch 故意不接：最小装配/单测里内核的 drawSlot 不存在。
		})
	})
	if drawn {
		t.Fatal("没有 drawSlot 却返回了\"画了\"：那层菜单面里一行都画不出来")
	}
}

func TestEntryMoreMenuDispatchesWithPropsAndRestoresThem(t *testing.T) {
	opened, pinned := "", []string{}
	called, seen, after := entryMenuFrame(t, EntryMenuOptions{
		Open:    true,
		Slot:    OffSidebarPanelActions,
		Region:  pluginapi.EntryRegionSidebar,
		Entries: entryMenuEntries(),
		OpenEntry: func(id string) {
			opened = id
		},
		SetPinned: func(id string, pin bool) {
			pinned = append(pinned, id)
		},
		Dismiss: func() {},
	})
	if called != 1 {
		t.Fatalf("座位 dispatch 次数 = %d，want 1（三个区域各派发自己那一个槽）", called)
	}
	if len(seen) != 1 {
		t.Fatalf("props 采样 = %d，want 1", len(seen))
	}
	props := *seen[0]
	if props.Region != pluginapi.EntryRegionSidebar {
		t.Fatalf("region = %q，want %q", props.Region, pluginapi.EntryRegionSidebar)
	}
	if len(props.Entries) != 2 || props.Entries[1].ID != "schedules" || props.Entries[1].Pinned {
		t.Fatalf("entries 没原样交给占用方：%+v", props.Entries)
	}
	// 三个回调必须是宿主给的那三个（不是包一层空实现）。
	props.Open("schedules")
	props.SetPinned("schedules", true)
	if opened != "schedules" || len(pinned) != 1 || pinned[0] != "schedules" {
		t.Fatalf("owner props 回调没接上宿主：opened=%q pinned=%v", opened, pinned)
	}
	if after != nil {
		t.Fatalf("派发结束后 env.EntryMenu 还留着（同一帧别的画会读到菜单里才有的 props）：%+v", after)
	}
}

func TestEntryMoreMenuWidthFollowsOfficialMoreMenu(t *testing.T) {
	// 官方 `.moreMenu{width:288px;max-width:calc(100vw - 24px)}`（SidebarRoot.module.css:726-729）。
	if EntryMoreMenuWidth != 288 {
		t.Fatalf("菜单宽 = %v，官方 .moreMenu 是 288", EntryMoreMenuWidth)
	}
	if EntryMoreMenuEdgeMargin*2 != 24 {
		t.Fatalf("两侧留白合计 = %v，官方 max-width 是 calc(100vw - 24px)", EntryMoreMenuEdgeMargin*2)
	}
	cases := []struct {
		name   string
		bounds float32
		want   float32
	}{
		{name: "宽窗按 288", bounds: 1182, want: 288},
		{name: "窄窗按 100vw-24 夹住", bounds: 200, want: 176},
		{name: "窗口窄到放不下", bounds: 20, want: -4},
	}
	for _, tc := range cases {
		if got := EntryMoreMenuWidthFor(tc.bounds); got != tc.want {
			t.Errorf("%s：宽 %v 的窗口算出菜单宽 %v，want %v", tc.name, tc.bounds, got, tc.want)
		}
	}
}

// 编译期护栏：锚点与菜单面必须落在同一套矩形类型上（换了坐标口径会在这里炸）。
var _ gfx.Rect = ui.Rect{}
