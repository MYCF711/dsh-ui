package main

// shell.go —— L0 壳：首屏结构骨架的接线（rc.2 官方结构树 → 真界面）。
//
// 三档（`-shell`）：
//
//	auto（默认） 内核里**没有** shell.root 占用方时画官方骨架；有就走旧路径。
//	legacy      强制旧路径：`plugins/dsshell` 的三栏（自造 12px 内边距那套）。
//	official    强制官方骨架：`uikernel.FirstScreen`，官方槽名 + 官方 geometry；
//	            旧部件（dssidebar / dstopbar / dsconversation / dscomposer /
//	            dsrightbar）通过骨架里的**兼容桥**继续显示——它们还没迁到
//	            conversation.* / sidebar.* 官方槽位，这一步是过渡，不是终态。
//
// 为什么默认 auto 而不是直接切 official：切官方裸列模型要 dsshell + dstopbar +
// dsconversation + dscomposer 同一次改（`plugins/dsshell/shell.go` 的 columnPad
// 注释已写明，否则 composer 净边距从 16 掉到 4），那是一次独立的跨插件迁移；
// 本任务先把「结构骨架 + 官方槽 + 几何表」做成可断言的地基，并把切换开关接好。
//
// 标题栏只扣一次：Paint 的 contentBounds 已经排掉 Windows 的 40px caption
//（frame_chrome.go），所以传给骨架的 bounds 是**内容区**，ShellOptions.Windows
// 必须为 false；否则会二次扣、整块界面矮 40px。

import (
	"fmt"
	"runtime"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/ui"
)

// shellMode 是壳的三种结构档位。
type shellMode int

const (
	// shellAuto：有 shell.root 占用方（dsshell）就 legacy，否则官方骨架。
	shellAuto shellMode = iota
	// shellLegacy：强制 dsshell 三栏。
	shellLegacy
	// shellOfficial：强制官方骨架。
	shellOfficial
)

// String 实现 flag.Value 的可读输出。
func (m shellMode) String() string {
	switch m {
	case shellLegacy:
		return "legacy"
	case shellOfficial:
		return "official"
	default:
		return "auto"
	}
}

// parseShellMode 解析 -shell 取值；不认识的值直接报错（不许静默回落 auto：
// 那会让「以为切到官方骨架了」变成一次看不出来的测量失败）。
func parseShellMode(s string) (shellMode, error) {
	switch s {
	case "", "auto":
		return shellAuto, nil
	case "legacy":
		return shellLegacy, nil
	case "official":
		return shellOfficial, nil
	default:
		return shellAuto, fmt.Errorf("-shell 只认 auto / legacy / official，收到 %q", s)
	}
}

// resolve 把 auto 落到具体档位：内核里有 shell.root 占用方就走旧路径。
func (m shellMode) resolve(k *uikernel.Kernel) shellMode {
	if m != shellAuto {
		return m
	}
	if k != nil && k.Has(pluginapi.SlotShellRoot) {
		return shellLegacy
	}
	return shellOfficial
}

// linkStateOf 把三枚布尔事实压成 `pluginapi.LinkState`（唯一出处）。
//
// 顺序是有意的：
//
//	① **断线的证据优先于连着的证据**——订阅成功后又失败时两者都真，那时结论
//	   必须是"断"；
//	② 有连着的证据就算连着；
//	③ 都没有、但有一次订阅**正在飞行中**（首连 / 退避到点 / 用户点了立即重连）
//	   → `LinkConnecting`（官方 `data-phase=connecting` 的「重新连接中」）；
//	④ 三个都假 = 还没拿到任何事实 = `LinkUnknown`（界面写「未知」）。
func linkStateOf(up, down, connecting bool) pluginapi.LinkState {
	switch {
	case down:
		return pluginapi.LinkDown
	case up:
		return pluginapi.LinkConnected
	case connecting:
		return pluginapi.LinkConnecting
	default:
		return pluginapi.LinkUnknown
	}
}

// shellOptionsFor 把壳的真实状态压成骨架的一帧输入。
//
// 这里只许读真值：会话、右栏开关、侧栏宽度偏好都来自 model；未知一律按
// 官方默认（空 bottom、单 view、无 tabs），不编一个假值出来。
func shellOptionsFor(m *model, kernel *uikernel.Kernel, width, height float32) uikernel.ShellOptions {
	opts := uikernel.ShellOptions{
		// caption 由 L0 的 frame_chrome 扣过一次，这里的 bounds 已是内容区。
		Windows: false,
		// 目前只有 Chat 一个 view（Trajectory 未接），所以 header 是「无 tabs」态。
		Tabs:  false,
		Blank: m.cur == "",
		Phase: uikernel.PhaseActive,
		// Windows 下官方侧栏收起后整列消失（rail 0）；Web 保留 56px 图标轨。
		CollapsedWidth: uikernel.SidebarCollapsed,
		RightbarOn:     m.showRightbar,
		// shell.bottom 当前没有占用方（旧 dsstatusbar 已由 shell.bottom 取代且未挂），
		// 官方默认不预留高度。
		BottomHeight: 0,
	}
	if runtime.GOOS == "windows" {
		opts.CollapsedWidth = 0
	}
	if m.cur == "" {
		opts.Phase = uikernel.PhaseHero
	}
	// 侧栏宽度偏好：优先用 dssidebar 的真实宽度；它还没 Register（宽度未知）时
	// 落官方默认 280，**不要**退化成 0——0 在 official 档位等于「整列消失」，
	// 那是「宽度没拿到」被错当成「用户收起了侧栏」。
	opts.SidebarPreferred = uikernel.SidebarDefault
	if m.side != nil && m.side.Width() > 0 {
		opts.SidebarPreferred = m.side.Width()
	}
	// 旧部件还在时打开兼容桥：它们挂在 sidebar.* / main.* 旧槽上。
	if kernel != nil {
		opts.BridgeLegacy = kernel.Has(pluginapi.SlotSidebarList) || kernel.Has(pluginapi.SlotMainBody)
		// composer 那一段单独判：dscomposer 同时注册了 `main.composer`（旧三栏）
		// 与 `conversation.composer`（官方 chain）。官方座位有人时**走官方座位**，
		// 旧槽这一帧不 dispatch —— 这就是「同帧只有一个活」的判据。
		//
		// 为什么不用 BridgeLegacy 那一条：中栏其余部件（dstopbar 的 main.header、
		// dsconversation 的 main.body）今天仍是旧槽，整列一个开关会把 composer
		// 永远按在兼容桥上，官方座位永远 dispatch 不到（GP 报告 §5 第 1 条）。
		opts.BridgeLegacyComposer = kernel.Has(pluginapi.SlotMainComposer) &&
			!kernel.Has(uikernel.OffConvComposer)
	}
	return opts
}

// officialGeometry 用骨架的同一份三列求解填 pluginapi.Geometry（插件读它排版）。
//
// 不允许另算一遍：这里调的是 uikernel.ComputeShellLayout，与真正画出来的
// 是同一份结果（旧 buildGeometry 与 dsshell 各算一遍那次分叉过）。
func officialGeometry(width, height float32, opts uikernel.ShellOptions) pluginapi.Geometry {
	layout := uikernel.ComputeShellLayout(width, height, opts)
	return pluginapi.Geometry{
		Width:        layout.Frame.Width,
		Height:       layout.Frame.Height,
		SidebarWidth: layout.Columns.Sidebar,
		RightWidth:   layout.Columns.Rightbar,
		MainWidth:    layout.Columns.Center,
		BottomHeight: layout.BottomRow.Height,
	}
}

// drawOfficialFirstScreen 画官方骨架：容器由 uikernel 建，内容仍由插件画。
func drawOfficialFirstScreen(f ui.C, m *model, env *pluginapi.Env, opts uikernel.ShellOptions) {
	uikernel.FirstScreen(f, env, opts, func(slot pluginapi.Slot) {
		drawSlot(f, m.kernel, slot, env)
	})
}
