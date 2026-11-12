//go:build darwin || windows || linux

package dscomposer

// approval_official_test.go —— 审批卡的**官方 composer 座位**（`conversation.composer`）。
//
// 为什么单独一个文件：`approval_test.go` 钉的是**计划层**（该不该画、画什么、
// 回传哪个 id）。这里是**座位**：卡片画在 composer 区域（官方 InputBar 上方），
// 不是消息流里的一段；两个座位同一帧只有一个活；宿主没给的字段显示「未知」。
//
// 三条判据都来自跨线 §12 H13 与官方 `dsh-client-ui-approval`：
//
//	① 座位：官方审批走 **composer 接管**（`ChatView.tsx:293-295` 注释），
//	   transcript 里一条都不画（同屏两条「需要审批」是明确要避免的）。
//	② 三动作：拒绝 / 允许一次 / 始终允许（`ApprovalCard.tsx:87-97` 的顺序），
//	   宿主给几个选项就画几个——**不替用户丢选项**。
//	③ 未知：宿主没给的身份字段显示「未知」，不留空、不编内容。

import (
	"testing"
	"unicode/utf8"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// officialApproval 造一份**宿主给全三项**的审批（真宿主 policy=ask 的形状）：
// allow_once / allow_always / reject_once。
func officialApproval() pluginapi.Approval {
	return pluginapi.Approval{
		RequestID: "perm-official-1",
		Tool:      "bash",
		Title:     "bash: go test ./...",
		Kind:      "execute",
		Args:      `{"command":"go test ./..."}`,
		Options: []pluginapi.ApprovalOption{
			{OptionID: "allow-once", Name: "允许这一次", Kind: "allow_once"},
			{OptionID: "allow-always", Name: "这个会话里以后都允许", Kind: "allow_always"},
			{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
		},
	}
}

// drawSeat 在真 flex 画布上画一次官方座位，返回画布与回传记录。
func drawSeat(t *testing.T, approval *pluginapi.Approval) (*ui.Canvas, *[]string) {
	t.Helper()
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	tree := &ui.Tree{}
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	var approved []string
	data := pluginapi.Snapshot{Cur: "s-approval", Approval: approval}
	env := &pluginapi.Env{
		Theme: theme, Data: data, Tree: tree, Feat: tree.Feature("approval-official"),
		Canvas: canvas, Bounds: ui.Rect{Width: 1180, Height: 832}, MainWidth: 840,
		Geometry: pluginapi.Geometry{MainWidth: 840},
	}
	env.Act.Approve = func(requestID, optionID string) { approved = append(approved, requestID+"|"+optionID) }
	var fv flex.View
	fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 1180, Height: 832}, func(f *flex.Frame) {
		// 官方座位 = 整个 composer 区（审批卡 overlay + InputBar fallback），
		// 容器给官方侧边距 16（与 uikernel.drawComposerSeat 一致）。
		f.Column().Width(840 - 2*composerClearance).PadX(composerClearance).Children(func() {
			p.drawOfficialComposer(f, env)
		})
	})
	return canvas, &approved
}

func seatTexts(canvas *ui.Canvas) []string {
	values := make([]string, 0, len(canvas.Texts()))
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	return values
}

func seatHas(canvas *ui.Canvas, want string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == want || (len(text.Value) > len(want) && text.Value[:len(want)] == want) {
			return true
		}
	}
	return false
}

// TestOfficialSeatDrawsApprovalCard 官方座位真的画出卡片（不是空座位）。
func TestOfficialSeatDrawsApprovalCard(t *testing.T) {
	ap := officialApproval()
	canvas, _ := drawSeat(t, &ap)
	for _, want := range []string{"需要审批", "bash", "bash: go test ./...", "允许这一次", "这个会话里以后都允许", "拒绝"} {
		if !seatHas(canvas, want) {
			t.Fatalf("官方座位没画 %q：%v", want, seatTexts(canvas))
		}
	}
}

// TestOfficialSeatEmptyDrawsOnlyInputCard 没有审批请求时官方座位只画输入卡片。
//
// 官方座位是 **chain** 槽：fallback 是 InputBar（常在），overlay 才是审批卡。
// 所以"没有审批"= 审批那几行一条都不画，但输入卡片必须还在——画一个空壳
// 等于告诉用户"这里本来有个东西"，比没有更糟；反过来连输入卡片都没有，
// 就是 GP 报告 §5 第 2 条那个缺口（官方座位只画审批卡）。
func TestOfficialSeatEmptyDrawsOnlyInputCard(t *testing.T) {
	canvas, _ := drawSeat(t, nil)
	texts := seatTexts(canvas)
	for _, banned := range []string{"需要审批", "允许这一次", "拒绝"} {
		if seatHas(canvas, banned) {
			t.Fatalf("没有审批时官方座位画了审批内容 %q：%v", banned, texts)
		}
	}
	if !seatHas(canvas, placeholderText) {
		t.Fatalf("没有审批时官方座位必须还画输入卡片（占位文案）：%v", texts)
	}
}

// TestOfficialActionOrder 三动作的**左右顺序** = 官方顺序。
//
// 官方 `.actionRow` 是 拒绝 → 允许一次 → 始终允许（`ApprovalCard.tsx:87-97`）。
// 位置本身是风险提示：破坏性最小的放最左，"以后都允许"放最右。
func TestOfficialActionOrder(t *testing.T) {
	ap := officialApproval()
	canvas, _ := drawSeat(t, &ap)
	xOf := func(want string) float32 {
		for _, text := range canvas.Texts() {
			if text.Value == want {
				return text.Bounds.X
			}
		}
		t.Fatalf("画布上没有 %q：%v", want, seatTexts(canvas))
		return 0
	}
	deny, allow, always := xOf("拒绝"), xOf("允许这一次"), xOf("这个会话里以后都允许")
	if !(deny < allow && allow < always) {
		t.Fatalf("动作顺序 = 拒绝(%.0f) / 允许一次(%.0f) / 始终允许(%.0f)，想要从左到右递增",
			deny, allow, always)
	}
}

// TestUnknownFieldsShowUnknown 宿主没给的字段显示「未知」。
//
// 三条：
//   - 工具名解析不出来（title 没有冒号前缀、也没有 Tool）→ 条上显示「未知」；
//   - 标题与参数都空 → 标题行显示「未知」（否则整张卡没有一句在说"批什么"）；
//   - 有参数摘要时**不补**「未知」标题（那不是缺字段，是宿主已经交代了）。
func TestUnknownFieldsShowUnknown(t *testing.T) {
	t.Run("工具名与标题都缺", func(t *testing.T) {
		ap := pluginapi.Approval{
			RequestID: "perm-1",
			Options: []pluginapi.ApprovalOption{
				{OptionID: "allow-once", Name: "允许", Kind: "allow_once"},
				{OptionID: "reject-once", Name: "拒绝", Kind: "reject_once"},
			},
		}
		plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
		if plan.Tool != unknownField {
			t.Fatalf("工具名 = %q，想要 %q", plan.Tool, unknownField)
		}
		if plan.Title != unknownField {
			t.Fatalf("标题 = %q，想要 %q", plan.Title, unknownField)
		}
		canvas, _ := drawSeat(t, &ap)
		if !seatHas(canvas, unknownField) {
			t.Fatalf("画布上没有「未知」：%v", seatTexts(canvas))
		}
	})

	t.Run("有参数摘要就不补未知标题", func(t *testing.T) {
		ap := pluginapi.Approval{RequestID: "perm-1", Args: `{"command":"ls"}`}
		plan := planApproval(pluginapi.Snapshot{Approval: &ap}, true)
		if plan.Title == unknownField {
			t.Fatal("宿主已经给了参数摘要，再补一行「未知」是噪声")
		}
		if plan.Tool != unknownField {
			t.Fatalf("工具名 = %q，想要 %q", plan.Tool, unknownField)
		}
	})
}

// TestOfficialSeatRealClick 真 Paint → 真 HitTest → 真 Press/Release：
// 点「拒绝」必须回传**宿主给的那个 optionId**。
//
// 只断言"按钮画出来了"是不够的：`Clicked()` 没接 Action 时界面完全正常，
// 点下去什么都不发生——审批卡最坏的失败形态就是"点了没反应"。
func TestOfficialSeatRealClick(t *testing.T) {
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	tree := &ui.Tree{}
	feat := tree.Feature("approval-official-click")
	ap := officialApproval()
	var (
		invalidator app.Invalidator
		approved    []string
	)
	data := pluginapi.Snapshot{Cur: "s-approval", Approval: &ap}
	config := app.Config{
		Title: "approval-official", Width: 1180, Height: 832, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	var fv flex.View
	config.Paint = func(canvas *ui.Canvas, current ui.Size) {
		bounds := ui.Rect{Width: current.Width, Height: current.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: 840, Scroll: fv.ScrollState,
			Geometry: pluginapi.Geometry{MainWidth: 840},
		}
		env.Act.Approve = func(requestID, optionID string) { approved = append(approved, requestID+"|"+optionID) }
		fv.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f *flex.Frame) {
			p.drawOfficialComposer(f, env)
		})
	}
	config.OnScroll = func(event ui.ScrollEvent) ui.EventResult { return fv.Wheel(event) }
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	defer session.Close()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}

	id := tree.ID(feat, "approval-deny")
	var hit ui.Hit
	found := false
	for _, candidate := range session.Snapshot().Hits {
		if candidate.ID == id {
			hit, found = candidate, true
			break
		}
	}
	if !found {
		t.Fatalf("「拒绝」按钮没有命中区 id=%v；hits=%+v", id, session.Snapshot().Hits)
	}
	x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
	if err := session.Press(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Release(x, y); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if len(approved) != 1 || approved[0] != "perm-official-1|reject-once" {
		t.Fatalf("回传 = %v，想要 [perm-official-1|reject-once]（原样带回宿主给的 id）", approved)
	}
}

// TestBothSeatsSameCard 两个座位画的是**同一张卡**（不会各画一套）。
//
// 座位只是壳的挂载点不同；卡片几何与计划只有一份。两份迟早会漂——
// 漂了的表现是"旧壳上三个按钮、官方壳上两个"。
func TestBothSeatsSameCard(t *testing.T) {
	ap := officialApproval()
	official, _ := drawSeat(t, &ap)

	// 旧座位（main.composer 的 draw）走完整 composer（审批条 + 输入卡片），
	// 只在画布上取**审批卡那几行**做对比。
	theme := dscore.Dark()
	p := &Plugin{theme: theme}
	tree := &ui.Tree{}
	canvas := &ui.Canvas{}
	canvas.SetTree(tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size
	})
	env := &pluginapi.Env{
		Theme: theme, Data: pluginapi.Snapshot{Cur: "s-approval", Approval: &ap},
		Tree: tree, Feat: tree.Feature("approval-legacy"), Canvas: canvas,
		Bounds: ui.Rect{Width: 1180, Height: 832}, MainWidth: 840,
		Geometry: pluginapi.Geometry{MainWidth: 840},
	}
	env.Act.Approve = func(string, string) {}
	var fv flex.View
	fv.Draw(canvas, dscore.VivaTheme(theme), ui.Rect{Width: 1180, Height: 832}, func(f *flex.Frame) {
		p.draw(f, env)
	})
	for _, want := range []string{"需要审批", "允许这一次", "这个会话里以后都允许", "拒绝"} {
		if !seatHas(official, want) {
			t.Fatalf("官方座位缺 %q", want)
		}
		if !seatHas(canvas, want) {
			t.Fatalf("旧座位缺 %q（两座位的卡片漂了）", want)
		}
	}
}
