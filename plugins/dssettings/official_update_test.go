package dssettings

// official_update_test.go —— 官方 `DesktopUpdateIndicator`（设置行第 3 格）的行为断言。
//
// 八组判据（全部对官方 rc.2 源码）：
//
//	① 可见性：`!wide || hidden || (!failed && (state undefined || (idle && 无 configurationError)))`
//	   → 整枚不画；
//	② 几何：28 高 / 左右 8 / 图标 14 + gap4 / 12-18 字 / brand 描边 20% + 底 8% / 错因点 6；
//	③ 文案：官方 zh 的 desktop.update.* 逐字（含 tooltip 的版本号与 14 档错因）；
//	④ 真画：available 相画官方 label + 14px glyph，且 tooltip 带上版本号；
//	⑤ error 相画的是**错因点**（6px 实心圆），不是下载图标；
//	⑥ configurationError 盖过一切（连 idle 都要画，label 是「配置不可用」）；
//	⑦ 点击：真按钮（命中区 + aria-label）+ 真动作（Actions.DesktopUpdateOpen）；
//	   busy / opening 期间 `aria-disabled`，点了不触发；
//	⑧ busy spinner：官方 `.spinner{animation:spin 1s linear infinite}`。

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

// updateHarness 是这一格的最小画布：设置行容器 + 只画「更新指示」那一个画家。
type updateHarness struct {
	kernel *uikernel.Kernel
	tree   *gfx.Tree
	feat   gfx.Scope
	theme  *pluginapi.Theme
	// act 是本帧注入的壳动作（nil 动作 = 老壳，插件不该画命中区）。
	act pluginapi.Actions
	// hit 非空时，这一帧是那个命中区的**点击事件帧**。
	hit string
}

func newUpdateHarness(t *testing.T) *updateHarness {
	t.Helper()
	kernel := uikernel.New("settings-update")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &Plugin{}); err != nil {
		t.Fatalf("装配失败：%v", err)
	}
	tree := &gfx.Tree{}
	return &updateHarness{
		kernel: kernel,
		tree:   tree,
		feat:   tree.Feature("settings-update"),
		theme:  kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme),
	}
}

// frame 画一帧，返回这一帧的画布。
func (h *updateHarness) frame(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	canvas := &gfx.Canvas{}
	canvas.SetTree(h.tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * 0.55
	})
	bounds := gfx.Rect{Width: 320, Height: 60}
	env := &pluginapi.Env{
		Theme: h.theme, Data: data, Act: h.act, Tree: h.tree, Feat: h.feat, Canvas: canvas,
		Bounds: bounds, MainWidth: bounds.Width,
		Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height, MainWidth: bounds.Width},
		Has:      h.kernel.Has,
	}
	if h.hit != "" {
		canvas.BeginEventPass(h.tree.ID(h.feat, h.hit), gfx.ClickInfo{})
		defer canvas.EndEventPass()
	}
	var view ui.Page
	view.Draw(canvas, bounds, func(f ui.C) {
		// 设置行是一个 Row（壳画容器，插件画格子）；这里直接按行画。
		ui.Row(f).Items(ui.Center).Gap(SettingsTriggerRowGapForTest).Children(func() {
			// 只画本单负责的那一格（设置触发器由 dsnotifications 的启动器或
			// fallback 负责，画进来会污染"有没有文字"的断言）。
			for _, painter := range h.kernel.Painters(uikernel.OffSidebarSettings) {
				if painter.ID == "settings-desktop-update" && painter.Draw != nil {
					painter.Draw(f, env)
				}
			}
		})
	})
	return canvas
}

// click 模拟"用户点了一下这枚指示器"：先画一帧让命中区就位（viva 的事件 pass
// 需要上一帧的命中区），再画这一帧事件 pass。
func (h *updateHarness) click(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	h.hit = ""
	h.frame(t, data)
	h.hit = "settings-desktop-update"
	return h.frame(t, data)
}

// updatePillHit 取这一帧的指示器命中区（没有就返回 false）。
func updatePillHit(canvas *gfx.Canvas, tree *gfx.Tree, feat gfx.Scope) (gfx.Hit, bool) {
	id := tree.ID(feat, "settings-desktop-update")
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// updateFrame 装配 dssettings 并画一帧（旧入口，保持既有断言可读）。
func updateFrame(t *testing.T, data pluginapi.Snapshot) *gfx.Canvas {
	t.Helper()
	return newUpdateHarness(t).frame(t, data)
}

// SettingsTriggerRowGapForTest 只为测试里的行容器服务（官方 8）。
const SettingsTriggerRowGapForTest float32 = 8

// ① 可见性：官方 `DesktopUpdateIndicator.tsx:70` 的那四条判据。
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
		{
			name: "壳说桥坏了（failed）也要画",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Failed: true}},
			want: true,
			why:  "官方 `!failed && …` —— failed 时整条 copy 换成「重试更新」，不能因为 state undefined 就不画",
		},
		{
			name: "配置坏掉的 idle 相要画",
			data: pluginapi.Snapshot{Link: pluginapi.LinkConnected,
				DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
					Phase: pluginapi.DesktopUpdateIdle, ConfigurationError: "channel"}},
			want: true,
			why:  "官方 :70 的 `&& state.configurationError === undefined`：配置坏了正是最该看见提示的时候",
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
		{"error-dot", UpdateErrorDotSize, 6, ".errorDot{width:6px;height:6px}"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v，官方 %v（%s）", check.name, check.got, check.want, check.why)
		}
	}
	// 几何表里这些行必须都在册且 aligned（与官方 CSS 一一对应）。
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
	for _, tc := range []struct {
		node, prop string
		want       float32
	}{
		{"DesktopUpdateIndicator.icon", "size", 14},
		{"DesktopUpdateIndicator.spinner", "cycle-ms", 1000},
		{"DesktopUpdateIndicator.errorDot", "size", 6},
	} {
		row, ok := findGeoRow(tc.node, tc.prop)
		if !ok {
			t.Errorf("几何表缺 %s/%s", tc.node, tc.prop)
			continue
		}
		if row.Status != GeoAligned || row.Impl != tc.want {
			t.Errorf("%s/%s = %v（%v），want aligned %v", tc.node, tc.prop, row.Impl, row.Status, tc.want)
		}
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
	// 错因文案也是逐字（locales.ts:36-48 全 14 档）。
	for _, tc := range []struct {
		kind pluginapi.DesktopUpdateFailureKind
		want string
	}{
		{pluginapi.DesktopUpdateFailureConfiguration, "未能应用更新配置。请点击桌面更新入口查看原因。"},
		{pluginapi.DesktopUpdateFailureCheck, "检查更新失败，请稍后重试。"},
		{pluginapi.DesktopUpdateFailureCheckNetwork, "检查更新失败，请检查网络连接后重试。"},
		{pluginapi.DesktopUpdateFailureDownload, "下载更新失败，请重试。"},
		{pluginapi.DesktopUpdateFailureDownloadSpace, "磁盘空间不足，无法下载更新。请释放空间后重试。"},
		{pluginapi.DesktopUpdateFailureDownloadPermission, "无法写入更新文件，请检查目录权限后重试。"},
		{pluginapi.DesktopUpdateFailureDownloadBusy, "更新文件正被占用，请关闭占用它的程序后重试。"},
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
	// `?? 'install'` 的兜底只覆盖"没给 failure"；给了但不在词表里时官方是
	// `undefined`（tooltip 什么都不显示），我们回到空串。
	if got := zhUpdateFailure(""); got != "安装更新失败，请稍后重试。" {
		t.Errorf("failure 为空应兜底 install 文案，实得 %q", got)
	}
	if got := zhUpdateFailure("no-such-kind"); got != "" {
		t.Errorf("词表外的 failure 应回到空串（官方 undefined），实得 %q", got)
	}
	// 配置错四句（locales.ts:21-24）。未知码同样是空串。
	for _, tc := range []struct {
		code, want string
	}{
		{"read", "更新已暂停：无法读取配置文件"},
		{"format", "更新已暂停：配置文件格式有误"},
		{"channel", "更新已暂停：channel 只能是 nightly 或 stable"},
		{"cleanup", "更新已暂停：无法清理旧更新，请重启应用"},
		{"no-such-code", ""},
	} {
		if got := zhUpdateConfiguration(tc.code); got != tc.want {
			t.Errorf("configuration(%q) = %q，want %q", tc.code, got, tc.want)
		}
	}
}

// ③b `updateCopy()` 整条分派：label/detail 一对，逐条对官方 `DesktopUpdateIndicator.tsx:14-55`。
func TestDesktopUpdateCopyMatchesOfficialDispatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		state  *pluginapi.DesktopUpdatePresentation
		label  string
		detail string
	}{
		{
			name:   "available 带版本：版本号在 detail（官方 versionDetail 全角冒号）",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.3"},
			label:  "新版本",
			detail: "新版本：0.2.2-go.3",
		},
		{
			name:   "available 没版本：detail 退回 label",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable},
			label:  "新版本",
			detail: "新版本",
		},
		{
			name:   "checking 的 detail 是 version ?? label",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateChecking},
			label:  "正在检查更新…",
			detail: "正在检查更新…",
		},
		{
			name:   "checking 带 version 时 detail 就是 version",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateChecking, Version: "0.2.2-go.3"},
			label:  "正在检查更新…",
			detail: "0.2.2-go.3",
		},
		{
			name:   "error：label 是重试，detail 是错因分类文案",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateError, Failure: pluginapi.DesktopUpdateFailureCheckNetwork},
			label:  "重试更新",
			detail: "检查更新失败，请检查网络连接后重试。",
		},
		{
			name:   "error 没给 failure：按 install 兜底",
			state:  &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateError},
			label:  "重试更新",
			detail: "安装更新失败，请稍后重试。",
		},
		{
			name: "configurationError 盖过 phase 与 failure",
			state: &pluginapi.DesktopUpdatePresentation{
				Phase: pluginapi.DesktopUpdateError, Failure: pluginapi.DesktopUpdateFailureCheck,
				ConfigurationError: "format"},
			label:  "配置不可用",
			detail: "更新已暂停：配置文件格式有误",
		},
		{
			name: "configurationError 在 idle 上也生效",
			state: &pluginapi.DesktopUpdatePresentation{
				Phase: pluginapi.DesktopUpdateIdle, ConfigurationError: "read"},
			label:  "配置不可用",
			detail: "更新已暂停：无法读取配置文件",
		},
		{
			name: "downloading 的 detail 是 downloadDetail（两行，percent 在前）",
			state: &pluginapi.DesktopUpdatePresentation{
				Phase: pluginapi.DesktopUpdateDownloading, Percent: 42, Version: "0.2.2-go.3"},
			label:  "42%",
			detail: "正在下载更新：42%\n目标版本：0.2.2-go.3",
		},
		{
			name:   "nil（桥失败）：label 与 detail 都是重试更新",
			state:  nil,
			label:  "重试更新",
			detail: "重试更新",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := updateCopy(tc.state)
			if got.Label != tc.label || got.Detail != tc.detail {
				t.Fatalf("updateCopy = {%q, %q}，官方 = {%q, %q}",
					got.Label, got.Detail, tc.label, tc.detail)
			}
		})
	}
}

// ④ 真画：available 相画官方 label，tooltip（= 命中区 Hint）带上版本号；
// 无呈现时画布空白。
func TestDesktopUpdatePaintsOnlyWithRealPresentation(t *testing.T) {
	canvas := updateFrame(t, pluginapi.Snapshot{Link: pluginapi.LinkConnected})
	if texts := canvas.Texts(); len(texts) != 0 {
		t.Fatalf("没有呈现时不该画任何文字，实得 %v", texts)
	}

	h := newUpdateHarness(t)
	h.act = pluginapi.Actions{DesktopUpdateOpen: func() {}}
	data := pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
			Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.3",
		}}
	canvas = h.frame(t, data)
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
	// 版本号在 tooltip 里（官方 `desktop.update.versionDetail` = "{label}：{version}"）。
	hit, ok := updatePillHit(canvas, h.tree, h.feat)
	if !ok {
		t.Fatal("available 相没有命中区（官方是 <button>）")
	}
	if want := "新版本：0.2.2-go.3"; hit.Hint != want {
		t.Fatalf("tooltip = %q，官方 versionDetail 文案 %q", hit.Hint, want)
	}
	if hit.Label != zhUpdateAvailable {
		t.Fatalf("aria-label = %q，官方 `aria-label={label}` = %q", hit.Label, zhUpdateAvailable)
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

// ⑤ error 相画的是**错因点**：官方 `.errorDot{width:6px;height:6px;border-radius:50%}`，
// 长在 14×14 的 `.icon` 里；此时**不画**下载图标。
func TestDesktopUpdateErrorPaintsErrorDotNotIcon(t *testing.T) {
	h := newUpdateHarness(t)
	h.act = pluginapi.Actions{DesktopUpdateOpen: func() {}}
	data := pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
			Phase: pluginapi.DesktopUpdateError, Failure: pluginapi.DesktopUpdateFailureCheck,
		}}
	canvas := h.frame(t, data)

	var label gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == zhUpdateRetry {
			label = text
		}
	}
	if label.Value == "" {
		t.Fatalf("error 相的 label 应是官方 %q，实得 %v", zhUpdateRetry, textValues(canvas))
	}
	// 错因点在图标格里：找到一枚 6×6 的矩形。
	iconX := label.Bounds.X - UpdateIndicatorGap - UpdateIndicatorIconSize
	dotOK := false
	for _, rect := range canvas.Rectangles() {
		if math.Abs(float64(rect.Bounds.Width-UpdateErrorDotSize)) <= 0.5 &&
			math.Abs(float64(rect.Bounds.Height-UpdateErrorDotSize)) <= 0.5 &&
			rect.Bounds.X >= iconX-0.5 && rect.Bounds.X <= label.Bounds.X {
			dotOK = true
		}
	}
	if !dotOK {
		t.Fatalf("error 相没画 %vpx 的错因点（官方 .errorDot）；rects=%+v",
			UpdateErrorDotSize, canvas.Rectangles())
	}
	// 官方的三选一里 error 排最前：这时**不该**有下载 glyph（三角形）。
	if n := opaqueTrianglesIn(canvas, gfx.Rect{X: iconX, Y: 0, Width: UpdateIndicatorIconSize, Height: 60}); n != 0 {
		t.Fatalf("error 相还画了 %d 个 glyph 三角形——官方是 error → busy → download 三选一", n)
	}
	// 错因文案进 tooltip。
	hit, ok := updatePillHit(canvas, h.tree, h.feat)
	if !ok {
		t.Fatal("error 相没有命中区")
	}
	if want := "检查更新失败，请稍后重试。"; hit.Hint != want {
		t.Fatalf("tooltip = %q，官方 checkFailed 文案 %q", hit.Hint, want)
	}
}

// ⑥ 壳报 failed（桥存在但调用失败）：官方整条 copy 换成「重试更新」。
func TestDesktopUpdateFailedBridgeShowsRetryCopy(t *testing.T) {
	h := newUpdateHarness(t)
	h.act = pluginapi.Actions{DesktopUpdateOpen: func() {}}
	canvas := h.frame(t, pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{Failed: true}})
	if got := textValues(canvas); len(got) == 0 || got[0] != zhUpdateRetry {
		t.Fatalf("failed 相文案 = %v，官方 %q", got, zhUpdateRetry)
	}
	hit, ok := updatePillHit(canvas, h.tree, h.feat)
	if !ok {
		t.Fatal("failed 相没有命中区")
	}
	if hit.Hint != zhUpdateRetry {
		t.Fatalf("failed 相 tooltip = %q，官方用 retryLabel 顶替 detail", hit.Hint)
	}
}

// ⑦ 点击：真按钮 + 真动作（官方 `<button onClick={onOpen}>`）。
//
// 只断言"画出来了"不够——`Clicked()` 没接动作时界面完全正常、点了没反应，
// 那正是任务书要拆掉的"假动作"。
func TestDesktopUpdateClickInvokesOpen(t *testing.T) {
	h := newUpdateHarness(t)
	opens := 0
	h.act = pluginapi.Actions{DesktopUpdateOpen: func() { opens++ }}
	data := pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
			Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.3"}}
	canvas := h.click(t, data)
	if opens != 1 {
		t.Fatalf("点一下指示器应发起 1 次真实 open，实际 %d 次", opens)
	}
	hit, ok := updatePillHit(canvas, h.tree, h.feat)
	if !ok {
		t.Fatal("available 相没有命中区")
	}
	if hit.Disabled {
		t.Fatal("available 相不该是 disabled（官方只在 opening / busy 时禁用）")
	}
	if hit.Cursor != gfx.PointerCursorHand {
		t.Fatalf("cursor = %v，官方 `.indicator{cursor:pointer}` 是 Hand", hit.Cursor)
	}
}

// ⑦b busy / opening 期间 `aria-disabled`：官方点了不触发。
func TestDesktopUpdateBusyAndOpeningAreDisabled(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state *pluginapi.DesktopUpdatePresentation
	}{
		{"checking（BUSY_PHASES）", &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateChecking}},
		{"opening（动作在飞）", &pluginapi.DesktopUpdatePresentation{Phase: pluginapi.DesktopUpdateAvailable, Opening: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newUpdateHarness(t)
			opens := 0
			h.act = pluginapi.Actions{DesktopUpdateOpen: func() { opens++ }}
			canvas := h.click(t, pluginapi.Snapshot{Link: pluginapi.LinkConnected, DesktopUpdate: tc.state})
			if opens != 0 {
				t.Fatalf("disabled 期间点了应无动作，实际 %d 次", opens)
			}
			hit, ok := updatePillHit(canvas, h.tree, h.feat)
			if !ok {
				t.Fatal("这一相仍要画命中区（官方画的是 aria-disabled 的按钮，不是不画）")
			}
			if !hit.Disabled {
				t.Fatal("官方 disabled 时应带 aria-disabled=true")
			}
			// 官方 `.indicator[aria-disabled='true']{cursor:default}`。
			if hit.Cursor != gfx.PointerCursorDefault {
				t.Fatalf("disabled 的 cursor = %v，官方是 default", hit.Cursor)
			}
		})
	}
}

// ⑦c 壳没接 open：像素照画，但**不画命中区**（不留假按钮）。
func TestDesktopUpdateWithoutActionDrawsNoHit(t *testing.T) {
	h := newUpdateHarness(t)
	canvas := h.frame(t, pluginapi.Snapshot{Link: pluginapi.LinkConnected,
		DesktopUpdate: &pluginapi.DesktopUpdatePresentation{
			Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-go.3"}})
	if got := textValues(canvas); len(got) == 0 || got[0] != zhUpdateAvailable {
		t.Fatalf("壳没接动作时仍应画官方像素，实得 %v", got)
	}
	if _, ok := updatePillHit(canvas, h.tree, h.feat); ok {
		t.Fatal("壳没接 DesktopUpdateOpen 却画了命中区——点了没反应比不画更坏")
	}
}

// ⑧ busy spinner：官方 `.spinner{animation:spin 1s linear infinite}`。
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
	// 官方 `busy` 里 `configurationError === undefined` 是并列条件：配置坏了不禁用。
	configured := &pluginapi.DesktopUpdatePresentation{
		Phase: pluginapi.DesktopUpdateChecking, ConfigurationError: "read"}
	if desktopUpdateBusy(configured) {
		t.Error("configurationError 非空时官方不按 busy 处理（要点它可以去看配置错在哪）")
	}
	if desktopUpdateDisabled(configured) {
		t.Error("configurationError 非空时官方不禁用按钮")
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
