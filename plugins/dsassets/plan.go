package dsassets

import "github.com/vivaui/viva/gfx"

// 计划面板使用的官方 sprite 原件。
//
// 来源：dsh-web-frontend/dist/assets/index-ghOpC9uV.js:129 的
// plan-outline / close-circle-fill / chevron-right-outline / edit-outline 条目；
// 整文件 sha256=061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8。
// 使用处：dsh-client-ui-plan/src/client/PlanModeControl.tsx:61-64、
// PlanCard.tsx:81-82；dsh-client-ui-user-questions/src/client/PlanReviewPanel.tsx:74-79。
const (
	planOutlineLineA     = `M4.9375 5.90295H11.0625`
	planOutlineLineB     = `M4.9375 9.02991H8.27841`
	planOutlinePage      = `M12.5 1.32617C13.3039 1.32617 14 1.95171 14 2.77637V7.61328L13 8.68164V2.77637C13 2.55186 12.8007 2.32617 12.5 2.32617H3.5C3.1993 2.32617 3 2.55186 3 2.77637V13.2246C3.00044 13.4489 3.19963 13.6738 3.5 13.6738H8.32812L7.39258 14.6738H3.5C2.69637 14.6738 2.00042 14.0489 2 13.2246V2.77637C2 1.95171 2.69613 1.32617 3.5 1.32617H12.5Z`
	planOutlinePencil    = `M8.97212 14.3693C9.17511 14.5723 9.37811 14.7753 9.5811 14.9783C9.67012 14.8953 9.75914 14.8123 9.84815 14.7293C11.4505 13.2352 13.0528 11.7411 14.6551 10.247C14.7441 10.164 14.8331 10.081 14.9221 9.99803C14.5989 9.6748 14.2756 9.35157 13.9524 9.02834C13.8694 9.11736 13.7864 9.20637 13.7034 9.29539C12.2093 10.8977 10.7152 12.5 9.22113 14.1023C9.13813 14.1913 9.05513 14.2803 8.97212 14.3693Z`
	planOutlineUnderline = `M11.6323 13.7841C11.6323 14.0395 11.6323 14.295 11.6323 14.5504C11.6812 14.5523 11.7301 14.5543 11.779 14.5562C12.659 14.5913 13.539 14.6263 14.419 14.6614C14.4679 14.6633 14.5168 14.6653 14.5657 14.6672C14.5657 14.3339 14.5657 14.0006 14.5657 13.6672C14.5168 13.6692 14.4679 13.6711 14.419 13.6731C13.539 13.7081 12.659 13.7432 11.779 13.7783C11.7301 13.7802 11.6812 13.7821 11.6323 13.7841Z`

	planReviewChevronPath = `M6 12L9.29289 8.70711C9.68342 8.31658 9.68342 7.68342 9.29289 7.29289L6 4`
	planReviewEditPage    = `M8.85596 2.69971H4.19971C3.37141 2.69971 2.69992 3.37146 2.69971 4.19971V11.8003C2.69992 12.6285 3.37141 13.3003 4.19971 13.3003H11.8003C12.6283 13.2999 13.3001 12.6283 13.3003 11.8003V7.89893H14.3003V11.8003C14.3001 13.1806 13.1806 14.2999 11.8003 14.3003H4.19971C2.81913 14.3003 1.69992 13.1808 1.69971 11.8003V4.19971C1.69992 2.81918 2.81913 1.69971 4.19971 1.69971H8.85596V2.69971Z`
	planReviewEditSlash   = `M7.7849 8.23878L13.888 2.13574`
)

// PlanModeOutline 返回 IconPlanOutlineRegular（两条描边 + 三层填充）。
func PlanModeOutline(fill gfx.Color) (gfx.Icon, bool) {
	return Build(Asset{
		ID: "plan-outline", Title: "计划模式", ViewBox: 16, Size: 14, StrokeWidth: regularStroke,
		Elements: []Element{
			{Kind: PathElement, D: planOutlineLineA, Stroke: true},
			{Kind: PathElement, D: planOutlineLineB, Stroke: true},
			{Kind: PathElement, D: planOutlinePage, Fill: true},
			{Kind: PathElement, D: planOutlinePencil, Fill: true},
			{Kind: PathElement, D: planOutlineUnderline, Fill: true},
		},
	}, fill)
}

// PlanReviewChevron 返回 IconChevronRightOutlineRegular（14px 显示槽）。
func PlanReviewChevron() (gfx.Icon, bool) {
	return Build(Asset{
		ID: "chevron-right-outline", Title: "查看全文", ViewBox: 16, Size: 14, StrokeWidth: regularStroke,
		Elements: []Element{{Kind: PathElement, D: planReviewChevronPath, Stroke: true}},
	}, gfx.Color{})
}

// PlanReviewEdit 返回 IconEditOutlineRegular（填充页面 + 描边斜线）。
func PlanReviewEdit(fill gfx.Color) (gfx.Icon, bool) {
	return Build(Asset{
		ID: "edit-outline", Title: "要求修改", ViewBox: 16, Size: 16, StrokeWidth: regularStroke,
		Elements: []Element{
			{Kind: PathElement, D: planReviewEditPage, Fill: true},
			{Kind: PathElement, D: planReviewEditSlash, Stroke: true},
		},
	}, fill)
}

// PlanModeCloseCircle 返回 IconCloseCircleFillRegular。
// 官方外圈的 SVG A 命令由 `viva icons` 编译成下面的四次三次曲线；
// 内层 X 的坐标逐字来自同一 sprite，FillRule=evenodd 对应官方 fill-rule。
func PlanModeCloseCircle(fill gfx.Color) gfx.Icon {
	return gfx.Icon{ViewBox: 16, Layers: []gfx.IconLayer{{
		Fill: fill, FillRule: gfx.PathFillEvenOdd,
		Commands: []gfx.PathCommand{
			gfx.MoveTo(15, 8),
			gfx.CubicTo(15, 11.866, 11.866, 15, 8, 15),
			gfx.CubicTo(4.13401, 15, 1, 11.866, 1, 8),
			gfx.CubicTo(1, 4.13401, 4.13401, 1, 8, 1),
			gfx.CubicTo(11.866, 1, 15, 4.13401, 15, 8),
			gfx.ClosePath(),
			gfx.MoveTo(6.409, 10.652),
			gfx.LineTo(5.348, 9.591),
			gfx.LineTo(6.939, 8),
			gfx.LineTo(5.348, 6.409),
			gfx.LineTo(6.409, 5.348),
			gfx.LineTo(8, 6.939),
			gfx.LineTo(9.591, 5.348),
			gfx.LineTo(10.652, 6.409),
			gfx.LineTo(9.061, 8),
			gfx.LineTo(10.652, 9.591),
			gfx.LineTo(9.591, 10.652),
			gfx.LineTo(8, 9.061),
			gfx.ClosePath(),
		},
	}}}
}
