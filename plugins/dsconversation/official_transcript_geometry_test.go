package dsconversation

// official_transcript_geometry_test.go —— 几何表与 z-index 阶梯的机器断言。
//
// 三条硬断言（对应「几何落常量表，三态」这条要求）：
//
//	① 每行必须有官方锚点，且锚点带行号（`src/**.module.css:line`）；
//	② aligned 行实现值 == 官方值；derived 行必须写 Formula（官方 calc 原文）；
//	   deviation 行必须写 Reason（为什么不同）；
//	③ z-index 阶梯必须是 6 < 7 = 7 < 8 < 9 四档（结构树 §5.22 官方注释固化）。

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
)

func TestOfficialTranscriptGeometryAnchors(t *testing.T) {
	rows := OfficialTranscriptGeometry()
	if len(rows) < 100 {
		t.Fatalf("几何表只有 %d 行，rc.2 结构树的几何条目远不止这些（至少 100 行）", len(rows))
	}
	for i, row := range rows {
		if row.Node == "" {
			t.Errorf("第 %d 行缺少官方节点名", i+1)
		}
		if row.Prop == "" {
			t.Errorf("第 %d 行（%s）缺少属性名", i+1, row.Node)
		}
		if row.Anchor == "" {
			t.Errorf("第 %d 行（%s.%s）缺少 rc.2 锚点", i+1, row.Node, row.Prop)
		}
		if !strings.Contains(row.Anchor, ":") {
			t.Errorf("第 %d 行（%s.%s）的锚点 %q 没有行号", i+1, row.Node, row.Prop, row.Anchor)
		}
	}
}

func TestOfficialTranscriptGeometryStatusRules(t *testing.T) {
	for _, row := range OfficialTranscriptGeometry() {
		switch row.Status {
		case uikernel.GeoAligned:
			if row.Official != row.Impl {
				t.Errorf("%s.%s 标 aligned 但官方 %v ≠ 实现 %v", row.Node, row.Prop, row.Official, row.Impl)
			}
			if row.Formula != "" {
				t.Errorf("%s.%s 是 aligned 行，不该有 Formula（%q）", row.Node, row.Prop, row.Formula)
			}
			if row.Reason != "" {
				t.Errorf("%s.%s 是 aligned 行，不该有 Reason（%q）", row.Node, row.Prop, row.Reason)
			}
		case uikernel.GeoDerived:
			if row.Formula == "" {
				t.Errorf("%s.%s 标 derived，必须写官方 calc/clamp 原文（Formula）", row.Node, row.Prop)
			}
		case uikernel.GeoDeviation:
			if row.Reason == "" {
				t.Errorf("%s.%s 标 deviation，必须写清为什么与官方不同", row.Node, row.Prop)
			}
		default:
			t.Errorf("%s.%s 的状态 %q 不在三态里（aligned/derived/deviation）", row.Node, row.Prop, row.Status)
		}
	}
}

// TestGeometryStatusCoverage 报告三态各有多少行（不是"至少一行"这种空断言）。
//
// 现状：deviation 只有三条，且都是渲染器能力边界（容器查询 / 滚动条槽位 /
// 浏览器滚动锚定），不是"还没做"。
func TestGeometryStatusCoverage(t *testing.T) {
	counts := map[uikernel.GeoStatus]int{}
	for _, row := range OfficialTranscriptGeometry() {
		counts[row.Status]++
	}
	if counts[uikernel.GeoAligned] < 70 {
		t.Errorf("aligned 行只有 %d 条，主区纵向节奏/横向宽度轴应基本全部 aligned", counts[uikernel.GeoAligned])
	}
	if counts[uikernel.GeoDerived] < 5 {
		t.Errorf("derived 行只有 %d 条（官方大量 calc/clamp 表达式）", counts[uikernel.GeoDerived])
	}
	if counts[uikernel.GeoDeviation] != 4 {
		t.Errorf("deviation 行 = %d 条，应为 4 条（容器查询 / 滚动条槽位 / 滚动锚定 / z-index 表达）", counts[uikernel.GeoDeviation])
	}
	t.Logf("几何表 %d 行：aligned=%d derived=%d deviation=%d",
		TranscriptGeometryCount(), counts[uikernel.GeoAligned], counts[uikernel.GeoDerived], counts[uikernel.GeoDeviation])
}

// TestZIndexLadderFixed 钉住 6 < 7 = 7 < 8 < 9 四档。
//
// 官方把这条写成注释固化在 `ConversationRoot.module.css:522-527`：
// 代码块 banner 6 → 钉住的压缩头与 turn rail 7 → 回到底部 8 → composer seat 9。
// **实现不得改**：改一档就会让钉住头压不过 banner（滚动时露出双份标题）。
func TestZIndexLadderFixed(t *testing.T) {
	ladder := OfficialZIndexLadder()
	if len(ladder) != 5 {
		t.Fatalf("z-index 阶梯 %d 档，官方是 5 个节点（6/7/7/8/9）", len(ladder))
	}
	want := []float32{ZChatCodeBanner, ZChatStickyHeader, ZChatTurnRail, ZChatToBottom, ZChatComposerSeat}
	for i := range ladder {
		if ladder[i].Z != want[i] {
			t.Errorf("第 %d 档 z = %v，应为 %v", i+1, ladder[i].Z, want[i])
		}
		if ladder[i].Anchor == "" {
			t.Errorf("第 %d 档（%s）缺少官方锚点", i+1, ladder[i].Node)
		}
	}
	// 四档字面值：6 / 7 / 7 / 8 / 9（**不是** 5/10/15/20 那种任意值）。
	if ZChatCodeBanner != 6 || ZChatStickyHeader != 7 || ZChatTurnRail != 7 || ZChatToBottom != 8 || ZChatComposerSeat != 9 {
		t.Fatalf("z-index 阶梯字面值错了：%v/%v/%v/%v/%v",
			ZChatCodeBanner, ZChatStickyHeader, ZChatTurnRail, ZChatToBottom, ZChatComposerSeat)
	}
	// 非递减 + 首尾必须压过对方。
	for i := 1; i < len(ladder); i++ {
		if ladder[i].Z < ladder[i-1].Z {
			t.Errorf("第 %d 档 %v 低于第 %d 档 %v（阶梯必须从低到高）", i+1, ladder[i].Z, i, ladder[i-1].Z)
		}
	}
	if !(ZChatCodeBanner < ZChatStickyHeader && ZChatToBottom < ZChatComposerSeat) {
		t.Error("阶梯关系错：banner 必须低于钉住头，回到底部必须低于 composer seat")
	}
}

// TestScrollPortsDoubleLayer 钉住**双层滚动口**：宿主口 → 自滚口 → 降级态。
func TestScrollPortsDoubleLayer(t *testing.T) {
	ports := OfficialScrollPorts()
	if len(ports) != 3 {
		t.Fatalf("滚动口声明 %d 层，官方是 3 层（host / self / self-downgraded）", len(ports))
	}
	host, self, down := ports[0], ports[1], ports[2]
	if host.Name != "host" || !strings.Contains(host.Selector, "data-conversation-scroll") {
		t.Errorf("第一层必须是宿主滚动口 .scrollBody[data-conversation-scroll]，实际 %+v", host)
	}
	if host.OverflowY != "auto" {
		t.Errorf("宿主滚动口 overflow-y = %q，官方是 auto", host.OverflowY)
	}
	if self.Name != "self" || self.Selector != ".scroll" {
		t.Errorf("第二层必须是自滚动口 .scroll，实际 %+v", self)
	}
	if self.OverflowY != "auto" {
		t.Errorf("自滚动口 overflow-y = %q，官方是 auto（独立挂载时自己滚）", self.OverflowY)
	}
	if down.OverflowY != "visible" {
		t.Errorf("降级态 overflow-y = %q，官方是 visible（宿主口存在时让出纵向滚动）", down.OverflowY)
	}
	if !strings.Contains(down.Selector, "data-conversation-scroll") {
		t.Errorf("降级态选择器必须命中宿主滚动口祖先，实际 %q", down.Selector)
	}
	for _, port := range ports {
		if port.Anchor == "" {
			t.Errorf("滚动口 %q 缺少 rc.2 锚点", port.Name)
		}
	}
}

// TestGeometryUsesSingleSourceOfTruth 检查绘制代码没有另写魔法数：
// 凡是几何表里有的语义，常量必须与表里的实现值一致。
func TestGeometryUsesSingleSourceOfTruth(t *testing.T) {
	byNodeProp := map[string]float32{}
	for _, row := range OfficialTranscriptGeometry() {
		byNodeProp[row.Node+"."+row.Prop] = row.Impl
	}
	check := func(key string, impl float32) {
		t.Helper()
		want, ok := byNodeProp[key]
		if !ok {
			t.Errorf("几何表缺少 %s（绘制代码引用的常量必须在表里有出处）", key)
			return
		}
		if want != impl {
			t.Errorf("%s 表里实现值 %v，代码常量 %v（两处漂了）", key, want, impl)
		}
	}
	check("UserMessageNodeView.userRow.gap", UserRowGap)
	check("AssistantMarkdown.body.gap", AssistantBlockGap)
	check("MessageIconActions.actions.height", ActionsRowHeight)
	check("ToolRow.bodyScroll.max-height", ToolBodyScrollMaxHeight)
	check("ChatView.toBottom.size", ToBottomSize)
	check("TurnNavigator.frame.width", TurnRailWidth)
	check("ChatView.flowItem.min-width", 0)
}
