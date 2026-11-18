package dsplan

// tab_chip_test.go —— 右栏计划 tab 的**芯片**：图标与资源标题（#28 ⑤）。
//
// 官方契约（唯一权威，OFF 根见 plan.go 文件头）：
//
//	OFF:PlanPreview.tsx:47-53  `PlanTitle`
//	  <FileTypeIcon kind="other" size={16} className={css.titleIcon} />
//	  {plan?.title ?? tab.tab.title}
//	OFF:PlanPreview.module.css 编译产物 `lLdGaq_titleIcon{flex:none}`（**没有**颜色/尺寸覆盖）
//	OFF:ui-primitives/src/FileTypeIcon.tsx:26-33  `clsx(css.icon, css[resolvedType], className)`
//	  `._icon_1wejo_1{color:var(--dsh-file-type-icon-color,var(--dsh-file-type-default-color))}`
//	  `._other_1wejo_32{--dsh-file-type-default-color: var(--dsw-static-neutral-bluish-300)}`
//	FileTypeIcon.tsx:233 + :156-163  other 是 muted 档：底板 currentColor、折角
//	  `var(--dsw-static-neutral-400)`（**两层都是静态色板，不跟主题走**）

import (
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsassets"
	"github.com/vivaui/viva/gfx"
)

// 官方两层颜色的字面值（映射写在 `dsassets`，这里按官方 CSS 声明交叉核对）。
const (
	cssNeutralBluish300 uint32 = 0xcfd3d6 // --dsw-static-neutral-bluish-300
	cssNeutral400       uint32 = 0xa2a4a6 // --dsw-static-neutral-400
)

// TestPlanTabChipUsesOfficialOtherFileIcon 钉住芯片图标就是官方那一枚
// `FileTypeIcon kind="other" size={16}`：素材齐、颜色对、尺寸是 16。
func TestPlanTabChipUsesOfficialOtherFileIcon(t *testing.T) {
	icon := planTabIcon()
	if icon.ViewBox != 28 {
		t.Fatalf("图标 viewBox = %v，官方 FileGlyph 是 28", icon.ViewBox)
	}
	// other = muted：只有底板 + 折角两层，**没有**类型标记。
	if len(icon.Layers) != 2 {
		t.Fatalf("图层数 = %d，官方 other/muted 分支是 2 层（底板 + 折角）", len(icon.Layers))
	}
	if got := icon.Layers[0].Fill; got != gfx.RGB24(cssNeutralBluish300) {
		t.Errorf("底板色 = %v，官方 --dsw-static-neutral-bluish-300 = %#06x", got, cssNeutralBluish300)
	}
	if got := icon.Layers[1].Fill; got != gfx.RGB24(cssNeutral400) {
		t.Errorf("折角色 = %v，官方 --dsw-static-neutral-400 = %#06x", got, cssNeutral400)
	}
	if icon.Stroke != 0 {
		t.Errorf("other 分支是纯填充，不该带描边：stroke = %v", icon.Stroke)
	}

	// 注册处必须把它挂上去，并且尺寸是官方的 `size={16}`（不是 dockkit 默认的 14）。
	var preview *pluginapi.Painter
	plugin := &Plugin{}
	for _, painter := range plugin.Panels() {
		if painter.ID == "plan-preview" {
			copy := painter
			preview = &copy
		}
	}
	if preview == nil {
		t.Fatal("没有注册 plan-preview 画")
	}
	if preview.IconImage.ViewBox != 28 {
		t.Errorf("注册的 IconImage 没带上官方素材：%+v", preview.IconImage)
	}
	if preview.IconSize != 16 {
		t.Errorf("IconSize = %v，官方 PlanPreview.tsx:53 是 size={16}", preview.IconSize)
	}
	// `.titleIcon{flex:none}`：官方只有 flex:none，没有右外边距；图标与文字的距离
	// 由 dockkit 的 `.tabTitle{gap:5px}` 给（DrawRightTabs 的 RightTabChipGap）。
	if preview.IconMarginRight != 0 {
		t.Errorf("IconMarginRight = %v，官方 .titleIcon 不带 margin", preview.IconMarginRight)
	}

	// 素材确实来自 dsassets 的同一个 other 分支（不另画一份）。
	asset, ok := dsassets.FileTypeOther()
	if !ok || asset.Sheet.ViewBox != icon.ViewBox || len(asset.Sheet.Layers) != len(icon.Layers) {
		t.Fatalf("芯片图标与 dsassets.FileTypeOther 不是同一份：%+v vs %+v", asset.Sheet, icon)
	}
	for i := range asset.Sheet.Layers {
		if asset.Sheet.Layers[i].Fill != icon.Layers[i].Fill {
			t.Fatalf("第 %d 层颜色不一致：%v vs %v", i, asset.Sheet.Layers[i].Fill, icon.Layers[i].Fill)
		}
	}
}

// TestRightTabTitleFollowsResource 钉住官方 `plan?.title ?? tab.tab.title`：
// 拿到计划标题就用它，拿不到就让壳保留类型标题。
func TestRightTabTitleFollowsResource(t *testing.T) {
	plugin := &Plugin{}
	address := planAddress("session-1", "call-a")
	if got := plugin.RightTabTitleFor("session-1", "tab1", address); got != "" {
		t.Fatalf("还没有任何计划帧时不该编标题，得到 %q", got)
	}

	// 计划正文的标题来自 `# 标题`（OFF:plan.ts:45-48），这里直接喂解析结果。
	plugin.rememberPlanTitles([]Document{
		{CallID: "call-a", Title: "把 GUI 迁到 Go + Viva"},
		{CallID: "call-b", Title: "另一份方案"},
	})
	if got := plugin.RightTabTitleFor("session-1", "tab1", address); got != "把 GUI 迁到 Go + Viva" {
		t.Fatalf("资源标题 = %q，want「把 GUI 迁到 Go + Viva」", got)
	}
	// 地址对不上（别的 callId）不许借别人的标题。
	if got := plugin.RightTabTitleFor("session-1", "tab2", planAddress("session-1", "call-zzz")); got != "" {
		t.Fatalf("不存在的 callId 借到了标题 %q", got)
	}
	// 别的会话的地址：本仓只有当前会话的帧，不许跨会话安标题。
	if got := plugin.RightTabTitleFor("session-2", "tab1", address); got != "" {
		t.Fatalf("跨会话地址拿到了标题 %q", got)
	}
	// 地址不是计划地址 / 是空串：不编。
	if got := plugin.RightTabTitleFor("session-1", "tab1", "dsh-resource://file/session-1/a.md"); got != "" {
		t.Fatalf("非计划地址拿到了标题 %q", got)
	}
	if got := plugin.RightTabTitleFor("session-1", "tab1", ""); got != "" {
		t.Fatalf("空地址拿到了标题 %q", got)
	}
}

// TestRightTabTitleForTemporaryReview 钉住官方那条临时审阅分支：
// 地址是 `dsh-resource://plan-review/…` 时标题取在内存文档的标题
// （OFF:PlanPreview.tsx:50-52 的 `params.planReview.title`）。
func TestRightTabTitleForTemporaryReview(t *testing.T) {
	plugin := &Plugin{temporaryReviews: map[string]Document{
		reviewPreviewAddress("session-1", "win:req"): {Title: "等待审批的草稿"},
	}}
	address := reviewPreviewAddress("session-1", "win:req")
	if got := plugin.RightTabTitleFor("session-1", "tab1", address); got != "等待审批的草稿" {
		t.Fatalf("临时审阅标题 = %q，want「等待审批的草稿」", got)
	}
	// 文档已经不在（地址还在）→ 不编标题，壳保留类型标题。
	if got := plugin.RightTabTitleFor("session-1", "tab1", reviewPreviewAddress("session-1", "win:other")); got != "" {
		t.Fatalf("失效的临时审阅拿到了标题 %q", got)
	}
}

// TestRememberPlanTitlesIgnoresEmpty 钉住缓存只记真标题：没有一级标题的计划
// 官方根本不算计划（OFF:plan.ts:47-48），所以 `Title` 空就不进缓存。
func TestRememberPlanTitlesIgnoresEmpty(t *testing.T) {
	plugin := &Plugin{}
	plugin.rememberPlanTitles([]Document{
		{CallID: "call-a", Title: ""},
		{CallID: "", Title: "没有 callId"},
		{CallID: "call-b", Title: "有标题"},
	})
	if len(plugin.planTitles) != 1 {
		t.Fatalf("缓存了 %d 条，want 1：%+v", len(plugin.planTitles), plugin.planTitles)
	}
	if plugin.planTitles["call-b"] != "有标题" {
		t.Fatalf("缓存内容不对：%+v", plugin.planTitles)
	}
}
