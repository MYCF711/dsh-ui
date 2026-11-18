//go:build darwin || windows || linux

// official_capture2_test.go —— 第二轮真实渲染审计的薄 capture 层。
//
// 只做一件事：把**已经存在**的官方四区面板逐页画成 PNG，并把「默认态 /
// hover 态」这一对状态也留成像素证据（第一轮只看过 general 一页的默认态）。
//
// 判据仍归 official_panel_visual_test.go / official_geometry_test.go，这里不重复
// 立几何断言，只补三条只有看图才会暴露的东西：
//
//  1. general 页的字号 stepper：官方 `.arrows{opacity:0}`，默认**不出现** ↑↓；
//  2. 指针进入胶囊（`._stepper:hover`）之后两枚 chevron 才显形，且都在胶囊右缘内；
//  3. 显形后点上方 chevron 真的写出去（+1），不是画了个假控件。
//
// 截图落 `$SETTINGS_CAPTURE_DIR/round2`（未设置时用临时目录）。截图只在断言
// 通过之后落盘，和第一轮同一约定。
package dssettings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/vivaui/viva/gfx"
)

// captureRound2Dir 返回本轮截图目录。
func captureRound2Dir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("SETTINGS_CAPTURE_DIR")
	if base == "" {
		return t.TempDir()
	}
	dir := filepath.Join(base, "round2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sectionsCapture 是官方注册点的全量（official_tree.go:65-95 的 officialSections）。
var sectionsCapture = []string{"account", "general", "models", "plugins", "agent-presets"}

// TestOfficialSectionPagesRenderCapture 逐页切过去各画一张。
//
// 切页走的是真实点击（h.click → 真 HitTest → 真 Click），不是直接改 active，
// 所以每张图都是"用户点进去看到的那一页"。
func TestOfficialSectionPagesRenderCapture(t *testing.T) {
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatalf("Shift+F2 没打开官方面板（status=%q）", h.statusText())
	}
	dir := captureRound2Dir(t)
	for _, id := range sectionsCapture {
		h.click("settings-nav-" + id)
		if got := h.p.ActiveSection(); got != id {
			t.Fatalf("点导航 %s 之后 active=%q", id, got)
		}
		// 每页至少要有标题文字：空页在像素上是一整块底色，光看文件大小看不出来。
		section, ok := h.p.PanelModel().Section(id)
		if !ok || section.Title == "" {
			t.Fatalf("section %s 没有标题", id)
		}
		if !h.hasText(section.Title) {
			t.Fatalf("%s 页没画出自己的标题 %q；texts=%v", id, section.Title, h.session.Texts())
		}
		path := filepath.Join(dir, "official-section-"+id+".png")
		if err := h.session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("截图为空：path=%s info=%v err=%v", path, info, err)
		}
		t.Logf("SETTINGS_CAPTURE2 section=%s path=%s", id, path)
	}
}

// themedDescribeFixture 在 describeFixture 上补一个**官方形状**的 ui-theme 命名空间。
//
// 形状逐字来自官方 `ui-theme/src/theme-settings.ts`：`ThemeSettingsSchema` 的
// `preference`（dark/light/system）、`fontSize`（10..22 的整数，默认 14）、
// `codeFontSize`、`terminalFontSize`。补它是因为第一轮的 fixture 没有 ui-theme，
// general 页整行落在"未知"降级上——那是正确的未知处理，但**看不到有值时的样子**，
// 也就看不到 stepper 的箭头显隐。
func themedDescribeFixture() *apiclient.SettingsDescribeResult {
	d := describeFixture()
	d.Namespaces = append(d.Namespaces,
		apiclient.SettingsNamespaceView{NS: "ui-theme", Revision: 4,
			Value: json.RawMessage(`{"preference":"dark","fontSize":14,"codeFontSize":11,"terminalFontSize":13}`)},
		apiclient.SettingsNamespaceView{NS: "ui-settings", Revision: 2,
			Value: json.RawMessage(`{"enabled":false}`)},
	)
	d.Sections = append(d.Sections,
		apiclient.SettingsSectionView{
			NS: "ui-theme", Title: "外观", Writable: true, Revision: 4,
			Fields: []apiclient.SettingsFieldView{
				{Name: "preference", Path: []string{"preference"}, Type: "string", Current: "dark", Writable: true,
					Enum: []string{"light", "dark", "system"}},
				{Name: "fontSize", Path: []string{"fontSize"}, Type: "number", Current: 14, Writable: true},
			},
		},
		apiclient.SettingsSectionView{
			NS: "ui-settings", Title: "设置", Writable: true, Revision: 2,
			Fields: []apiclient.SettingsFieldView{
				{Name: "enabled", Path: []string{"enabled"}, Type: "boolean", Current: false, Writable: true},
			},
		},
	)
	return d
}

// TestFontStepperArrowsOnlyOnHover 钉住"箭头默认不显形、进入胶囊才显形"。
//
// 官方依据（两条一起才成立）：
//
//	FontSizeRow.module.css  `._arrows{…opacity:0;position:absolute;right:8px}`
//	FontSizeRow.module.css  `._stepper:hover ._arrows,._stepper:focus-within ._arrows{opacity:1}`
//
// 第一轮之前这里是常数：两枚 17×12 的 chevron 一直画着（label-primary），
// 于是"默认态"和"hover 态"在像素上完全一样——正是数值全对、画出来不对的那一类。
func TestFontStepperArrowsOnlyOnHover(t *testing.T) {
	dir := captureRound2Dir(t)
	h := openOfficialHarness(t, 1182, 832)
	defer h.close()
	h.mu.Lock()
	h.describe = themedDescribeFixture()
	h.mu.Unlock()
	h.pump()
	h.pump()
	if err := h.session.Driver().Key(gfx.KeyF2, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.p.OfficialShown() {
		t.Fatalf("Shift+F2 没打开官方面板（status=%q）", h.statusText())
	}
	h.click("settings-nav-general")
	section, ok := h.p.PanelModel().Section("general")
	if !ok {
		t.Fatal("general 页不存在")
	}
	var fontSize PanelRow
	for _, row := range section.Rows {
		if row.ID == "font-size" {
			fontSize = row
		}
	}
	if fontSize.ID != "font-size" {
		t.Fatalf("general 页没有字号行；rows=%+v", section.Rows)
	}
	if !fontSize.Known {
		t.Fatalf("补了 ui-theme 之后字号行仍是未知：value=%q reason=%q", fontSize.Value, fontSize.Reason)
	}

	stepper, found := h.findHit("settings-stepper-font-size")
	if !found {
		t.Fatalf("没有胶囊命中区；hits=%v", h.hitIDs())
	}
	// 官方 `._stepper{min-width:72px;height:36px}`：命中区必须就是那枚 72×36 的胶囊，
	// 不是整行（整行会把标题文字也算进 hover 范围，箭头在标题上就会冒出来）。
	if stepper.Bounds.Width != StepperMinWidth || stepper.Bounds.Height != SelectorHeight {
		t.Fatalf("胶囊命中区=%+v，官方 %v×%v", stepper.Bounds, StepperMinWidth, SelectorHeight)
	}

	// —— 默认态：指针在别处（打开面板后从未移动过）。
	if hit, ok := h.findHit("settings-stepper-font-size"); ok && h.hoveredIs(hit.ID) {
		t.Fatalf("默认态 hover 目标已是胶囊：%v", hit.ID)
	}
	if arrows := h.arrowHits(); len(arrows) != 0 {
		t.Fatalf("默认态画出了箭头命中区 %+v——官方 ._arrows{opacity:0} 不该显形", arrows)
	}
	defaultPath := filepath.Join(dir, "general-stepper-default.png")
	if err := h.session.Screenshot(defaultPath); err != nil {
		t.Fatal(err)
	}
	t.Logf("SETTINGS_CAPTURE2 stepper-default path=%s", defaultPath)

	// —— hover 胶囊：指针真的移进去（不是改内部状态）。
	x, y := stepper.Bounds.X+stepper.Bounds.Width/2, stepper.Bounds.Y+stepper.Bounds.Height/2
	if err := h.session.Move(x, y); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if !h.hoveredIs(stepper.ID) {
		t.Fatalf("指针移到胶囊 (%.0f,%.0f) 后 hover 目标不是它：%v", x, y, h.session.Snapshot().Hovered)
	}
	arrows := h.arrowHits()
	if len(arrows) != 2 {
		t.Fatalf("hover 胶囊后应有 ↑↓ 两枚命中区，得到 %+v", arrows)
	}
	for _, arrow := range arrows {
		if arrow.Bounds.Width != StepperArrowWidth || arrow.Bounds.Height != StepperArrowHeight {
			t.Fatalf("箭头命中区=%+v，官方 17×12（FontSizeRow.module.css 的 ._arrow）", arrow.Bounds)
		}
		// `._arrows{position:absolute;right:8px}`：两枚都必须落在胶囊右缘 8px 起、
		// 宽 17 的列里，超出胶囊就是画到外面去了。
		if right := arrow.Bounds.X + arrow.Bounds.Width; right > stepper.Bounds.X+stepper.Bounds.Width {
			t.Fatalf("箭头右缘 %.1f 超出胶囊右缘 %.1f", right, stepper.Bounds.X+stepper.Bounds.Width)
		}
		if arrow.Bounds.X != stepper.Bounds.X+StepperMinWidth-StepperArrowRight-StepperArrowWidth {
			t.Fatalf("箭头左缘 %.1f，官方 right:8 → %.1f",
				arrow.Bounds.X, stepper.Bounds.X+StepperMinWidth-StepperArrowRight-StepperArrowWidth)
		}
	}
	hoverPath := filepath.Join(dir, "general-stepper-hover.png")
	if err := h.session.Screenshot(hoverPath); err != nil {
		t.Fatal(err)
	}
	t.Logf("SETTINGS_CAPTURE2 stepper-hover path=%s", hoverPath)

	// —— 点上方 chevron：官方 FontSizeRow.tsx 的 onClick 是 setFontSize(+1)。
	before := h.writer.callCount()
	var up gfx.Hit
	for _, arrow := range arrows {
		if arrow.Label == "字号大小 增大" {
			up = arrow
		}
	}
	if up.ID.IsZero() {
		t.Fatalf("找不到「增大」箭头：%+v", arrows)
	}
	ux, uy := up.Bounds.X+up.Bounds.Width/2, up.Bounds.Y+up.Bounds.Height/2
	if err := h.session.Click(ux, uy); err != nil {
		t.Fatal(err)
	}
	h.pump()
	deadline := time.Now().Add(2 * time.Second)
	for h.writer.callCount() == before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.writer.callCount() == before {
		t.Fatal("点了「增大」什么都没写出去")
	}
	call, _ := h.writer.lastCall()
	if call.ns != "ui-theme" {
		t.Fatalf("写到了 %q，官方字号属于 ui-theme", call.ns)
	}
	if got, want := len(call.path), 1; got != want || call.path[0] != "fontSize" {
		t.Fatalf("写入路径=%v，官方 ThemeSettings.fontSize", call.path)
	}
	if px, ok := call.value.(int); !ok || px != 15 {
		t.Fatalf("写入值=%v（%T），官方 14+1=15", call.value, call.value)
	}

	// —— 指针离开胶囊：箭头重新隐去（否则"hover 态"只是画上去的装饰）。
	if err := h.session.Move(12, 12); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.pump()
	if arrows := h.arrowHits(); len(arrows) != 0 {
		t.Fatalf("指针离开胶囊后箭头仍显形：%+v", arrows)
	}
}

// hoveredIs 读本帧 hover 目标（gfx.Canvas.Hovered 的快照面）。
func (h *officialHarness) hoveredIs(id gfx.ID) bool {
	return h.session.Snapshot().Hovered == id
}

// arrowHits 返回本帧两枚 stepper 箭头的命中区。
func (h *officialHarness) arrowHits() []gfx.Hit {
	var out []gfx.Hit
	for _, hit := range h.session.Snapshot().Hits {
		if hit.Label == "字号大小 增大" || hit.Label == "字号大小 减小" {
			out = append(out, hit)
		}
	}
	return out
}

// hitIDs 只用于失败信息：把命中区 id 全列出来比 "hits=[]" 好查。
func (h *officialHarness) hitIDs() []gfx.ID {
	ids := make([]gfx.ID, 0, len(h.session.Snapshot().Hits))
	for _, hit := range h.session.Snapshot().Hits {
		ids = append(ids, hit.ID)
	}
	return ids
}
