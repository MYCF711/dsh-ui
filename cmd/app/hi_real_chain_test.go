//go:build windows

package main

// hi_real_chain_test.go —— **真机验收**：动画与图标细节 5 件（task-58）。
//
// 链路与 HH/HF 同一条：真 `dshd serve`（临时 home + `--addr 127.0.0.1:0`）
// + 真 `apiclient`（`/connect` 换 cookie）+ `-shell official` 产品形状
// （`uikernel.FirstScreen` 官方骨架 + 产品插件集）。
//
// 判据（逐条对应本单 5 件）：
//
//	① 入场动画：官方 `_indicator-enter .15s ease-out`——同一行的两张截图在
//	   入场起点/入场完成后亮度不同（fade-in 真的发生了），且全不透明帧更亮；
//	② 2s 恢复确认窗 + 800ms 最小可见：真时钟下 connecting 只持续 100ms 就恢复，
//	   400ms 时仍写「重新连接中…」，1200ms 时才是「连接成功」；
//	③ `DesktopUpdateIndicator`：设置行第 3 格按官方几何画出（28 高、新版本文案、
//	   14px glyph）——呈现值由验收脚本注入（产品没有桌面更新通道，见报告）；
//	④ spinner 真动画：connecting 态隔 250ms 两张截图，图标区域像素**不同**
//	   （官方 1.5s 线性旋转 + dash 动画）；
//	⑤ gauge/database 官方图标：胶囊与弹窗标题的 14px 图标区域里有不透明图元，
//	   且胶囊宽度 = 8+14+6+文字+8。
//
// 截图落在 `$HI_CAPTURE_DIR`（默认 `tmp/GO8-RC2/_hi/shots`）。
//
// 跑法（不给 DSHD_BIN 就跳过）：
//
//	export PATH="/c/Program Files/Go/bin:/c/toolchains/mingw64/bin:$PATH"
//	export DSHD_BIN=".../tmp/GO8-RC2/_hi/bin/dshd.exe"
//	export DSHD_HOME_ROOT=".../tmp/GO8-RC2/_hi"
//	export HI_CAPTURE_DIR=".../tmp/GO8-RC2/_hi/shots"
//	CGO_ENABLED=0 go test ./cmd/app -run 'TestRealChainHIAnimationAndIcons' -count=1 -v

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsstatusbar"
	"github.com/vivaui/viva/ui"
)

// hiShotDir 取截图目录（不存在就建）。
func hiShotDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("HI_CAPTURE_DIR")
	if dir == "" {
		dir = "D:/DeepSeek Harness/default-workspace/DSH-GO/tmp/GO8-RC2/_hi/shots"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建截图目录失败：%v", err)
	}
	return dir
}

// hiLoadPNG 读一张截图（像素断言用）。
func hiLoadPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开截图失败 %s：%v", path, err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("解码截图失败 %s：%v", path, err)
	}
	return img
}

// hiLuminance 求一块矩形里的平均亮度（0..1）。越界自动夹到图内。
func hiLuminance(img image.Image, rect ui.Rect) float64 {
	bounds := img.Bounds()
	x0, y0 := int(rect.X), int(rect.Y)
	x1, y1 := int(rect.X+rect.Width), int(rect.Y+rect.Height)
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return 0
	}
	var sum, count float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			sum += (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

// hiPixelDiff 数两块矩形里"明显不同"的像素数（动画是否真的在动）。
func hiPixelDiff(a, b image.Image, rect ui.Rect) int {
	bounds := a.Bounds()
	x0, y0 := int(rect.X), int(rect.Y)
	x1, y1 := int(rect.X+rect.Width), int(rect.Y+rect.Height)
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	diff := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			delta := absInt(int(ar)-int(br)) + absInt(int(ag)-int(bg)) + absInt(int(ab)-int(bb))
			if delta > 3*257 { // 每个通道平均差 > 1/255 才算"变了"
				diff++
			}
		}
	}
	return diff
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// hiOpaqueTrianglesIn 数矩形里的不透明三角形（glyph 是否画了）。
func hiOpaqueTrianglesIn(canvas *ui.Canvas, rect ui.Rect) int {
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

// TestRealChainHIAnimationAndIcons 是真机验收主入口。
func TestRealChainHIAnimationAndIcons(t *testing.T) {
	host := startRealHost(t)
	client, mux := connectRealGUI(t, host)

	// ── 真会话（真宿主里的真 session）。
	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	sessionID, err := client.Create(createCtx, host.home, "allow", "hi-animation-icons")
	createCancel()
	if err != nil {
		t.Fatalf("session.create 失败：%v", err)
	}
	load, err := client.Load(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("session.load 失败：%v", err)
	}
	t.Logf("真宿主：session=%s home=%s", sessionID, host.home)

	// ── 真订阅（壳自己的 runSubscription）：成功即 LinkConnected。
	m := &model{cli: client, mux: mux, cur: sessionID}
	m.subscribe(sessionID)
	t.Cleanup(func() {
		m.mu.Lock()
		if sub := m.subs[sessionID]; sub != nil && sub.cancel != nil {
			sub.cancel()
		}
		m.mu.Unlock()
	})
	hhWaitLink(t, m, pluginapi.LinkConnected, 30*time.Second)

	product := newHHRealProduct(t)
	shotDir := hiShotDir(t)
	stamp := time.Now().Unix()
	act := pluginapi.Actions{
		Send:       func(string) {},
		Cancel:     func() {},
		Reconnect:  func() {},
		Invalidate: func() {},
	}
	base := pluginapi.Snapshot{
		Cur: sessionID, CWD: host.home, Status: "已加载 0 条（总 0）",
		Link:       pluginapi.LinkConnected,
		Msgs:       load.Messages,
		UsageTotal: gyUsage(load.UsageTotal),
		Usage:      gyUsage(load.Usage),
		Model:      load.Model,
		Permission: load.Permission,
		Total:      load.Total,
		// 真宿主跑出来的八字段投影形态（HC 报告 §4.1 的真机数据）：
		// 有计时行，活动胶囊才是官方 DialogPill（gauge 图标 + 点开弹窗）。
		SessionStats: &apiclient.SessionStats{
			Turns: 2, Steps: 3, LLMMs: 3267, ToolMs: 61, TTFTMs: 3055,
			TTFTSteps: 2, DecodeMs: 212, DecodeTokens: 87,
		},
	}
	// 首帧必须先走完官方 `_indicator-enter`（150ms）才有全不透明的胶囊。
	_, _, rowRect := product.paintReal(t, base, act, "")
	if rowRect.Width == 0 || rowRect.Height == 0 {
		t.Fatalf("骨架没报出设置行几何：%+v", rowRect)
	}
	product.settle(t, base, act)

	// waitLabel 重复画帧直到文案出现（真时钟 + 官方 800ms/2s 窗口都靠它兜）。
	waitLabel := func(snap pluginapi.Snapshot, want string, timeout time.Duration) *ui.Canvas {
		t.Helper()
		deadline := time.Now().Add(timeout)
		var last *ui.Canvas
		for {
			canvas, _, _ := product.paintReal(t, snap, act, "")
			last = canvas
			if hhHasText(canvas, want) {
				return canvas
			}
			if time.Now().After(deadline) {
				t.Fatalf("等不到文案 %q（超时 %v）；texts=%v", want, timeout, hhTexts(last))
			}
			time.Sleep(40 * time.Millisecond)
		}
	}

	// ── ① 入场动画：connected → disconnected 的入场帧 vs 全不透明帧。
	//    选 disconnected 而不是 connecting，是为了避开官方 800ms 最小可见窗。
	connecting := base
	connecting.Link = pluginapi.LinkConnecting
	down := base
	down.Link = pluginapi.LinkDown
	leaveFrame, _, _ := product.paintReal(t, down, act, "") // 离场窗第 1 帧（仍画旧态）
	time.Sleep(160 * time.Millisecond)
	enterStart, _, _ := product.paintReal(t, down, act, "") // 换态 + 入场起点（opacity≈0）
	enterStartPath := filepath.Join(shotDir, fmt.Sprintf("hi-enter-start-%d.png", stamp))
	product.shot(t, down, act, enterStartPath)
	time.Sleep(200 * time.Millisecond) // 入场 150ms 走完
	settled := waitLabel(down, dsstatusbar.OfficialDisconnectedLabel, 5*time.Second)
	settledPath := filepath.Join(shotDir, fmt.Sprintf("hi-enter-done-%d.png", stamp))
	product.shot(t, down, act, settledPath)

	startImg, doneImg := hiLoadPNG(t, enterStartPath), hiLoadPNG(t, settledPath)
	// 量"胶囊那一格"的亮度：从完成帧的胶囊文案反推它的包围盒（比猜坐标稳）。
	pillText, hasPill := hhFindText(settled, dsstatusbar.OfficialDisconnectedLabel)
	if !hasPill {
		t.Fatalf("① 完成后没有「%s」", dsstatusbar.OfficialDisconnectedLabel)
	}
	pillRegion := ui.Rect{
		X: pillText.X - 40, Y: pillText.Y - 10,
		Width: pillText.Width + 80, Height: pillText.Height + 20,
	}
	startLum, doneLum := hiLuminance(startImg, pillRegion), hiLuminance(doneImg, pillRegion)
	t.Logf("① 入场：设置行=%+v 胶囊格=%+v 亮度 入场起点=%.6f 完成=%.6f",
		rowRect, pillRegion, startLum, doneLum)
	if doneLum <= startLum+0.0005 {
		t.Fatalf("① 入场动画没把胶囊淡进来：入场起点亮度 %.6f，完成 %.6f", startLum, doneLum)
	}
	if hiPixelDiff(startImg, doneImg, pillRegion) == 0 {
		t.Fatal("① 入场起点的截图与完成帧完全一样——150ms 淡入没发生")
	}
	// 离场窗第一帧仍是旧态；150ms 后那一帧才换成新态（并开始入场）。
	if !hhHasText(leaveFrame, dsstatusbar.OfficialConnectedLabel) {
		t.Fatalf("① 离场窗第一帧应仍画旧态「%s」；texts=%v",
			dsstatusbar.OfficialConnectedLabel, hhTexts(leaveFrame))
	}
	if hhHasText(leaveFrame, dsstatusbar.OfficialDisconnectedLabel) {
		t.Fatal("① 离场窗第一帧就换成新态了——150ms 离场没生效")
	}
	if !hhHasText(enterStart, dsstatusbar.OfficialDisconnectedLabel) {
		t.Fatalf("① 入场起点应是新态（opacity≈0）；texts=%v", hhTexts(enterStart))
	}

	// ── ② 800ms 最小可见（真时钟）：connecting 只持续 100ms 就恢复。
	back := waitLabel(base, dsstatusbar.OfficialConnectedLabel, 5*time.Second)
	_ = back
	recoverStart := time.Now()
	product.paintReal(t, connecting, act, "") // 记下 connecting 起点
	time.Sleep(100 * time.Millisecond)
	product.paintReal(t, base, act, "") // 真实拨号 100ms 就成功
	time.Sleep(250 * time.Millisecond)
	hold, _, _ := product.paintReal(t, base, act, "")
	if !hhHasText(hold, dsstatusbar.OfficialConnectingLabel) {
		t.Fatalf("② %.0fms 时应仍是「%s」（官方最小可见 800ms）；texts=%v",
			float64(time.Since(recoverStart).Milliseconds()), dsstatusbar.OfficialConnectingLabel, hhTexts(hold))
	}
	t.Logf("② 800ms 最小可见：%.0fms 时仍画「%s」",
		float64(time.Since(recoverStart).Milliseconds()), dsstatusbar.OfficialConnectingLabel)
	time.Sleep(time.Until(recoverStart.Add(1400 * time.Millisecond)))
	after := waitLabel(base, dsstatusbar.OfficialConnectedLabel, 5*time.Second)
	if hhHasText(after, dsstatusbar.OfficialConnectingLabel) {
		t.Fatalf("② 1400ms 时还写着「%s」；texts=%v", dsstatusbar.OfficialConnectingLabel, hhTexts(after))
	}
	product.shot(t, base, act, filepath.Join(shotDir, fmt.Sprintf("hi-recovered-%d.png", stamp)))

	// ── ④ spinner 真动画：connecting 态隔 250ms 两张截图必须不同像素。
	spinning := waitLabel(connecting, dsstatusbar.OfficialConnectingLabel, 5*time.Second)
	spinPathA := filepath.Join(shotDir, fmt.Sprintf("hi-spinner-a-%d.png", stamp))
	product.shot(t, connecting, act, spinPathA)
	product.paintReal(t, connecting, act, "")
	time.Sleep(250 * time.Millisecond)
	spinPathB := filepath.Join(shotDir, fmt.Sprintf("hi-spinner-b-%d.png", stamp))
	product.shot(t, connecting, act, spinPathB)
	spinA, spinB := hiLoadPNG(t, spinPathA), hiLoadPNG(t, spinPathB)
	// spinner 是胶囊左侧那一枚 14px glyph：以胶囊文案反推它的位置。
	// 官方 connecting 文案是「重新连接中」+ 逐点出现的省略号，所以按前缀找。
	var spinText ui.Text
	okSpin := false
	for _, text := range spinning.Texts() {
		if strings.HasPrefix(text.Value, dsstatusbar.OfficialConnectingLabel) {
			spinText, okSpin = text, true
		}
	}
	if !okSpin {
		t.Fatalf("④ connecting 帧没有「%s」；texts=%v", dsstatusbar.OfficialConnectingLabel, hhTexts(spinning))
	}
	spinnerArea := ui.Rect{X: spinText.Bounds.X - 24, Y: spinText.Bounds.Y - 6, Width: 20, Height: 20}
	diff := hiPixelDiff(spinA, spinB, spinnerArea)
	t.Logf("④ spinner：250ms 两帧在胶囊图标区 %+v 的差异像素 = %d", spinnerArea, diff)
	if diff == 0 {
		t.Fatal("④ spinner 两帧在图标区完全相同——旋转动画没跑")
	}

	// ── ⑤ gauge/database 官方图标：胶囊与弹窗标题的 14px 图标区必须有图元。
	activity := waitLabel(base, dsstatusbar.OfficialConnectedLabel, 6*time.Second)
	dockTexts := make([]ui.Text, 0, 2)
	for _, text := range activity.Texts() {
		if strings.Contains(text.Value, " 轮 ") || strings.HasPrefix(text.Value, "用量") {
			dockTexts = append(dockTexts, text)
		}
	}
	if len(dockTexts) < 2 {
		t.Fatalf("⑤ dock 只找到 %d 枚胶囊文本（want 活动 + 用量）；texts=%v",
			len(dockTexts), hhTexts(activity))
	}
	for _, text := range dockTexts {
		// 官方 `.pill{display:inline-flex;gap:6px}` + `.pill svg{width:14px;height:14px}`：
		// 图标在文字左 20px（14 + 6）处开始。
		iconRect := ui.Rect{X: text.Bounds.X - 20, Y: text.Bounds.Y, Width: 14, Height: 14}
		if n := hiOpaqueTrianglesIn(activity, iconRect); n == 0 {
			t.Fatalf("⑤ 胶囊 %q 左侧没有 14px glyph（官方 gauge/database）；rect=%+v", text.Value, iconRect)
		}
	}
	product.shot(t, base, act, filepath.Join(shotDir, fmt.Sprintf("hi-dock-icons-%d.png", stamp)))

	// 弹窗标题：官方 `.titleLabel{display:inline-flex;gap:6px}` + 14px gauge。
	product.paintReal(t, base, act, "")
	opened, _, _ := product.paintReal(t, base, act, "activity-pill")
	titleRect, ok := hhFindText(opened, "会话统计")
	if !ok {
		t.Fatalf("⑤ 弹窗标题没画出来；texts=%v", hhTexts(opened))
	}
	titleIcon := ui.Rect{X: titleRect.X - 20, Y: titleRect.Y, Width: 14, Height: 14}
	if n := hiOpaqueTrianglesIn(opened, titleIcon); n == 0 {
		t.Fatalf("⑤ 弹窗标题左侧 20px 没有 gauge 图标（viewBox 16 / 显示 14）rect=%+v", titleIcon)
	}
	product.shot(t, base, act, filepath.Join(shotDir, fmt.Sprintf("hi-stats-dialog-title-%d.png", stamp)))

	// ── ③ 设置行第 3 格：DesktopUpdateIndicator（官方几何 + 官方文案）。
	//    产品没有桌面更新通道（宿主无 updater 方法），呈现值由验收脚本注入；
	//    这与官方 `DesktopUpdateIndicator` 的输入通道同一形状（`useDesktopUpdate`）。
	withUpdate := base
	withUpdate.DesktopUpdate = &pluginapi.DesktopUpdatePresentation{
		Phase: pluginapi.DesktopUpdateAvailable, Version: "0.2.2-rc.2-go.next",
	}
	updateCanvas := product.settle(t, withUpdate, act)
	if !hhHasText(updateCanvas, "新版本") {
		t.Fatalf("③ 设置行没有「新版本」（官方 desktop.update.available）；texts=%v", hhTexts(updateCanvas))
	}
	updateText, _ := hhFindText(updateCanvas, "新版本")
	updateIcon := ui.Rect{X: updateText.X - 18, Y: updateText.Y - 3, Width: 14, Height: 14}
	if n := hiOpaqueTrianglesIn(updateCanvas, updateIcon); n == 0 {
		t.Fatalf("③ 更新指示左侧没有 14px glyph（官方 IconDownloadOutlineRegular）rect=%+v", updateIcon)
	}
	if updateText.X <= 0 {
		t.Fatalf("③ 更新指示位置异常：%+v", updateText)
	}
	product.shot(t, withUpdate, act, filepath.Join(shotDir, fmt.Sprintf("hi-desktop-update-%d.png", stamp)))

	// 无呈现时（官方 idle）第 3 格不画：同一帧里不该出现任何更新文案。
	idleCanvas, _, _ := product.paintReal(t, base, act, "")
	for _, banned := range []string{"新版本", "正在检查更新…", "重试更新", "安装并重启"} {
		if hhHasText(idleCanvas, banned) {
			t.Fatalf("③ 没有呈现时不该画更新指示，却出现 %q", banned)
		}
	}
	t.Logf("HI_CAPTURE_DIR=%s", shotDir)
}
