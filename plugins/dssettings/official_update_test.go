package dssettings

// official_update_test.go —— 官方 `DesktopUpdateIndicator`（设置行第 3 格）的断言。
//
// 五条判据（全部对官方 rc.2 源码）：
//
//	① 可见性：`!wide || hidden || (state undefined || phase idle)` → 整枚不画；
//	② 几何：28 高 / 左右 8 / 图标 14 + gap4 / 12-18 字 / brand 描边 20% + 底 8%；
//	③ 文案：官方 zh 的 desktop.update.* 逐字（含进度百分比）；
//	④ 真画：有呈现时画到画布上（文字 + 14px glyph），没有时画布上什么都没有；
//	⑤ busy spinner：官方 `.spinner{animation:spin 1s linear infinite}`——不同时刻角度不同。

import (
	"math"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// updateFrame 装配 dssettings 并画一帧 `sidebar.settings` 槽（官方设置行）。
func updateFrame(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	kernel := uikernel.New("settings-update")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("settings-update")
	canvas := &gfx.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	bounds := gfx.Rect{Width: 320, Height: 60}
	env := &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
		Bounds: bounds, MainWidth: bounds.Width,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, MainWidth: bounds.Width},
		Has:      kernel.Has,
	}
	var view ui.Page
	view.Draw(canvas, bounds, func(f ui.C) {
		// 设置行是一个 Row（壳画容器，插件画格子）；这里直接按行画三个画家。
		ui.Row(f).Items(ui.Center).Gap(SettingsTriggerRowGapForTest).Children(func() {
			// 只画本单负责的那一格（设置触发器由 dsnotifications 的启动器或
			// fallback 负责，画进来会污染"有没有文字"的断言）。
			for _, painter := range kernel.Painters(uikernel.OffSidebarSettings) {
				if painter.ID == "settings-desktop-update" && painter.Draw != nil {
					painter.Draw(f, env)
				}
			}
		})
	})
	return canvas
}

// SettingsTriggerRowGapForTest 只为测试里的行容器服务（官方 8）。
const SettingsTriggerRowGapForTest float32 = 8

// ① 可见性：官方 `DesktopUpdateIndicator.tsx:66` 的三条判据。
func TestDesktopUpdateVisibilityFollowsOfficialRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		data pluginapi.Snapshot
		want bool
		why  string
	}{
		{
			name: "没有浮现值（nil）",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected},
			want: false,
			why:  "官方 state === undefined → return null",
		},
		{
			name: "idle 相",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateIdle}},
			want: false,
			why:  "官方 state.phase === 'idle' → return null",
		},
		{
			name: "有新版本",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.9"}},
			want: true,
			why:  "available 相要画",
		},
		{
			name: "正在连接时让位",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnecting,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable}},
			want: false,
			why:  "官方 hidden = connectionIndicator !== undefined（connecting 属于非 undefined）",
		},
		{
			name: "断开时让位",
			data: pluginapi.Snapshot{Link: pluginapi.LinkDown,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateChecking}},
			want: false,
			why:  "同上：disconnected 时连接胶囊占位，更新指示让位",
		},
		{
			name: "安装中例外（官方 phase !== installing 才让位）",
			data: pluginapi.Snapshot{Link: pluginapi.LinkDown,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateInstalling}},
			want: true,
			why:  "官方 hidden 里排除了 installing：装的时候必须能看见进度",
		},
		{
			name: "连着时不挡更新指示",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateDownloading, Percent: 42}},
			want: true,
			why:  "官方此时 connectionIndicator 已回到 undefined（2s 确认窗走完）",
		},
	} {
		if got := desktopUpdateVisible(tc.data); got != tc.want {
			t.Errorf("%s：visible = %v，want %v（%s）", tc.name, got, tc.want, tc.why)
		}
	}
}

// ② 几何：官方 DesktopUpdateIndicator.module.css 逐条 + 几何表 aligned。
func TestDesktopUpdateGeometryMatchesOfficial(t *testing.T) {
	checks := []struct {
		name string
		got  float32
		want float32
		why  string
	}{
		{"height", UpdateIndicatorHeight, 28, ".indicator{height:28px}"},
		{"padding-inline", UpdateIndicatorPadX, 8, ".indicator{padding:0 8px}"},
		{"column-gap", UpdateIndicatorGap, 4, ".indicator{column-gap:4px}"},
		{"icon", UpdateIndicatorIconSize, 14, ".icon{width:14px;height:14px}"},
		{"border", UpdateIndicatorBorder, 1, ".indicator{border:1px solid …}"},
		{"border-tint", UpdateIndicatorBorderTint, 0.2, "color-mix(… 20%, transparent)"},
		{"bg-tint", UpdateIndicatorBgTint, 0.08, "color-mix(… 8%, transparent)"},
		{"font-size", UpdateIndicatorFontSize, 12, ".indicator{font-size:12px}"},
		{"line-height", UpdateIndicatorLineHeight, 18, ".indicator{line-height:18px}"},
		{"spin", UpdateIndicatorSpinPeriodMS, 1000, ".spinner{animation:spin 1s linear infinite}"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方 %v（%s）", check.name, check.got, check.want, check.why)
		}
	}
	// 几何表里 9 行必须都在册且 aligned（与官方 CSS 一一对应）。
	wantRows := []string{
		"height", "padding-inline", "column-gap", "border-width", "border-tint",
		"background-tint", "font-size", "line-height",
	}
	for _, prop := range wantRows {
		found := false
		for _, row := range OfficialSettingsGeometry {
			if row.Node != "DesktopUpdateIndicator.indicator" || row.Prop != prop {
				continue
			}
			found = true
			if row.Status != GeoAligned {
				t.Errorf("DesktopUpdateIndicator.indicator/%s 状态 = %v，want aligned", prop, row.Status)
			}
			if row.Impl != row.Official {
				t.Errorf("DesktopUpdateIndicator.indicator/%s impl %v != official %v", prop, row.Impl, row.Official)
			}
		}
		if !found {
			t.Errorf("几何表缺 DesktopUpdateIndicator.indicator/%s", prop)
		}
	}
	if row, ok := findGeoRow("DesktopUpdateIndicator.icon", "size"); !ok || row.Impl != 14 {
		t.Errorf("几何表缺 DesktopUpdateIndicator.icon/size=14")
	}
	if row, ok := findGeoRow("DesktopUpdateIndicator.spinner", "cycle-ms"); !ok || row.Impl != 1000 {
		t.Errorf("几何表缺 DesktopUpdateIndicator.spinner/cycle-ms=1000")
	}
}

// findGeoRow 按 node/prop 找一行（dssettings 的几何表没有现成的查询函数）。
func findGeoRow(node, prop string) (GeoRow, bool) {
	for _, row := range OfficialSettingsGeometry {
		if row.Node == node && row.Prop == prop {
			return row, true
		}
	}
	return GeoRow{}, false
}

// ③ 文案：官方 locales.ts 的 desktop.update.* 逐字。
func TestDesktopUpdateLabelsMatchOfficialCopy(t *testing.T) {
	for _, tc := range []struct {
		state *pluginapi.DesktopUpdatePresentation
		want  string
	}{
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateChecking}, "正在检查更新…"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable}, "新版本"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateDownloading, Percent: 42}, "42%"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateVerifying}, "正在校验更新文件…"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateInstalling}, "正在准备重启…"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateReady}, "安装并重启"},
		{&pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateError}, "重试更新"},
	} {
		if got := updatePillLabel(tc.state); got != tc.want {
			t.Errorf("label(%v) = %q，官方 zh 文案 %q", tc.state.Phase, got, tc.want)
		}
	}
	// 错因文案也是逐字（locales.ts:16-25）。
	for _, tc := range []struct {
		kind pluginapi.DesktopUpdateFailureKind
		want string
	}{
		{pluginapi.DesktopUpdateFailureCheck, "检查更新失败，请稍后重试。"},
		{pluginapi.DesktopUpdateFailureCheckNetwork, "检查更新失败，请检查网络连接后重试。"},
		{pluginapi.DesktopUpdateFailureDownload, "下载更新失败，请重试。"},
		{pluginapi.DesktopUpdateFailureDownloadNet, "下载更新失败，请检查网络连接后重试。"},
		{pluginapi.DesktopUpdateFailureInstall, "安装更新失败，请稍后重试。"},
		{pluginapi.DesktopUpdateFailureInstallNetwork, "安装更新失败，请检查网络连接后重试。"},
		{pluginapi.DesktopUpdateFailureStopFailed, "未能安全停止任务，更新尚未安装，请稍后重试。"},
		{pluginapi.DesktopUpdateFailureTasksChanged, "有新任务开始运行，请重新确认是否停止任务并更新。"},
		{pluginapi.DesktopUpdateFailureTasksUnavail, "无法确认任务状态，请在工作区就绪后重试更新。"},
		{pluginapi.DesktopUpdateFailureFullscreen, "无法退出全屏，请退出全屏后重试更新。"},
	} {
		if got := zhUpdateFailure(tc.kind); got != tc.want {
			t.Errorf("failure(%v) = %q，官方 zh %q", tc.kind, got, tc.want)
		}
	}
}

// ④ 真画：有呈现 → 文字 + 14px glyph；无呈现 → 画布空白。
func TestDesktopUpdatePaintsOnlyWithRealPresentation(t *testing.T) {
	canvas := updateFrame(t, pluginapi.Snapshot{Link: pluginapi.LinkConnected})
	if texts := canvas.Texts(); len(texts) != 0 {
		t.Fatalf("没有呈现时不该画任何文字，实得 %v", texts)
	}

	data := pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
			Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.9",
		}}
	canvas = updateFrame(t, data)
	var label gfx.Text
	found := false
	for _, text := range canvas.Texts() {
		if text.Value == zhUpdateAvailable {
			label, found = text, true
		}
	}
	if !found {
		t.Fatalf("新版本相没画官方文案 %q，实得 %v", zhUpdateAvailable, textValues(canvas))
	}
	// 图标在文字左侧 4px（官方 column-gap:4px）+ 14px。
	iconX := label.Bounds.X - UpdateIndicatorGap - UpdateIndicatorIconSize
	if n := opaqueTrianglesIn(canvas, gfx.Rect{X: iconX, Y: 0, Width: UpdateIndicatorIconSize, Height: 60}); n == 0 {
		t.Fatal("更新指示的 14px glyph 没画（文字左侧 18px 内没有不透明图元）")
	}
	// 整枚高度 28：胶囊底色/描边是矩形图元（viva 的 Background/Border 走
	// Rectangles，不走三角形），所以按矩形找"高 28"的那一格。
	pillHeightOK := false
	for _, rect := range canvas.Rectangles() {
		if math.Abs(float64(rect.Bounds.Height-UpdateIndicatorHeight)) <= 0.5 && rect.Bounds.Width > 0 {
			pillHeightOK = true
		}
	}
	if !pillHeightOK {
		t.Fatalf("画布里没有高 %v 的胶囊图元，官方 .indicator{height:28px}；实得 %+v",
			UpdateIndicatorHeight, canvas.Rectangles())
	}
}

// ⑤ busy spinner：官方 `.spinner{animation:spin 1s linear infinite}`。
func TestDesktopUpdateBusySpinnerRotates(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := updateSpinRadians(t0)
	b := updateSpinRadians(t0.Add(250 * time.Millisecond))
	if math.Abs(float64(b-a-float32(math.Pi/2))) > 0.01 {
		t.Fatalf("250ms 旋转 = %v rad，官方 1s 线性应为 π/2 (%v)", b-a, math.Pi/2)
	}
	if a == b {
		t.Fatal("两个时间点角度相同——spinner 没在转")
	}
	// 一个周期后回到起点。
	if c := updateSpinRadians(t0.Add(time.Second)); math.Abs(float64(c-a)) > 0.01 {
		t.Fatalf("一个周期后角度 = %v，应回到 %v", c, a)
	}
	// busy 判据与官方 BUSY_PHASES 一致。
	busy := []pluginapi.DesktopUpdatePhase{
		pluginapi.DesktopUpdateChecking, pluginapi.DesktopUpdateDownloading,
		pluginapi.DesktopUpdateVerifying, pluginapi.DesktopUpdateInstalling,
	}
	for _, phase := range busy {
		state := &pluginapi.DesktopUpdatePresentation{Phase: phase}
		if !state.Busy() {
			t.Errorf("%v 应是 busy（官方 BUSY_PHASES）", phase)
		}
	}
	for _, phase := range []pluginapi.DesktopUpdatePhase{
		pluginapi.DesktopUpdateAvailable, pluginapi.DesktopUpdateReady, pluginapi.DesktopUpdateError,
	} {
		state := &pluginapi.DesktopUpdatePresentation{Phase: phase}
		if state.Busy() {
			t.Errorf("%v 不该是 busy", phase)
		}
	}
}

// textValues 取画布上的文字（失败信息用）。
func textValues(canvas *gfx.Canvas) []string {
	out := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		out = append(out, text.Value)
	}
	return out
}

// opaqueTrianglesIn 数落在一个矩形里的不透明三角形（glyph 是否画了的判据）。
func opaqueTrianglesIn(canvas *gfx.Canvas, rect gfx.Rect) int {
	const opaque = 0.5
	n := 0
	for _, triangle := range canvas.Triangles() {
		if triangle.Color.Alpha < opaque {
			continue
		}
		for _, point := range triangle.Points {
			if point.X >= rect.X && point.X <= rect.X+rect.Width &&
				point.Y >= rect.Y && point.Y <= rect.Y+rect.Height {
				n++
				break
			}
		}
	}
	return n
}
