package dssettings

import (
	"strings"
	"testing"
)

// TestSettingsPanelGridIsTwoByTwo 钉住 rc.2 的四区网格（D1）。
func TestSettingsPanelGridIsTwoByTwo(t *testing.T) {
	if len(SettingsPanelGrid.Columns) != 2 || len(SettingsPanelGrid.Rows) != 2 {
		t.Fatalf("网格应为 2×2，实得 %d×%d", len(SettingsPanelGrid.Columns), len(SettingsPanelGrid.Rows))
	}
	if SettingsPanelGrid.Columns[0].Kind != TrackFixed || SettingsPanelGrid.Columns[0].Value != GridNavWidth {
		t.Fatalf("第一列应是 %vpx 固定轨道，实得 %+v", GridNavWidth, SettingsPanelGrid.Columns[0])
	}
	if SettingsPanelGrid.Columns[1].Kind != TrackMinMax || SettingsPanelGrid.Columns[1].Fr != 1 {
		t.Fatalf("第二列应是 minmax(0, 1fr)，实得 %+v", SettingsPanelGrid.Columns[1])
	}
	if SettingsPanelGrid.Rows[0].Kind != TrackAuto {
		t.Fatalf("第一行应是 auto，实得 %+v", SettingsPanelGrid.Rows[0])
	}
	if SettingsPanelGrid.Rows[1].Kind != TrackMinMax || SettingsPanelGrid.Rows[1].Fr != 1 {
		t.Fatalf("第二行应是 minmax(0, 1fr)，实得 %+v", SettingsPanelGrid.Rows[1])
	}
	// 四个区域一个不少，且位置与官方 areas 一致。
	for _, name := range GridAreaNames {
		if _, ok := gridAreaName(name); !ok {
			t.Fatalf("区域 %q 不在模板里", name)
		}
	}
}

// TestSolveGridProducesOfficialRects 是几何断言的核心：800 宽卡片 →
// 导航列 188、内容列 612；第一行 54、第二行 746。
func TestSolveGridProducesOfficialRects(t *testing.T) {
	layout, err := LayoutSettingsPanel(1920, 1032, PanelOverlayTop)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Width != 800 || layout.Height != 800 {
		t.Fatalf("卡片 = %v×%v，应为 800×800", layout.Width, layout.Height)
	}
	// 列：188 + 612 = 800。
	if got := layout.Columns[0]; got != 188 {
		t.Fatalf("导航列 = %v，官方 188", got)
	}
	if got := layout.Columns[1]; got != 612 {
		t.Fatalf("内容列 = %v，官方 1fr = 800-188 = 612", got)
	}
	// 行：auto=54 + 746 = 800。
	if got := layout.Rows[0]; got != GridHeaderRowHeight {
		t.Fatalf("第一行 = %v，官方 auto 内容高 54", got)
	}
	if got := layout.Rows[1]; got != 800-GridHeaderRowHeight {
		t.Fatalf("第二行 = %v，官方 1fr = %v", got, 800-GridHeaderRowHeight)
	}

	// 四区矩形：title/nav 在左列，header/options 在右列。
	title, ok := layout.Area("title")
	if !ok {
		t.Fatal("缺 title 区")
	}
	header, _ := layout.Area("header")
	nav, _ := layout.Area("nav")
	options, _ := layout.Area("options")

	for name, area := range map[string]AreaRect{"title": title, "header": header, "nav": nav, "options": options} {
		if area.Width <= 0 || area.Height <= 0 {
			t.Fatalf("%s 区尺寸非正：%+v", name, area)
		}
	}
	if title.X != nav.X || title.Width != 188 {
		t.Fatalf("左列两区应在同一 X 且宽 188：title=%+v nav=%+v", title, nav)
	}
	if header.X != options.X || header.Width != 612 {
		t.Fatalf("右列两区应在同一 X 且宽 612：header=%+v options=%+v", header, options)
	}
	if title.Y != layout.Y || nav.Y != layout.Y+GridHeaderRowHeight {
		t.Fatalf("纵向：title 应在容器顶 %v、nav 应在 %v（实得 %v/%v）",
			layout.Y, layout.Y+GridHeaderRowHeight, title.Y, nav.Y)
	}
	if header.Y != layout.Y || options.Y != layout.Y+GridHeaderRowHeight {
		t.Fatalf("纵向：header 应在容器顶 %v、options 应在 %v（实得 %v/%v）",
			layout.Y, layout.Y+GridHeaderRowHeight, header.Y, options.Y)
	}
	// 容器偏移要带上（卡片居中）。
	if layout.X != (1920-800)/2 || layout.Y != (1032-800)/2 {
		t.Fatalf("容器偏移 = (%v,%v)，应居中", layout.X, layout.Y)
	}
	if title.X < layout.X || header.X != layout.X+188 {
		t.Fatalf("区域坐标未叠加容器偏移：title=%+v header=%+v layout.X=%v", title, header, layout.X)
	}
	if title.X+title.Width > layout.X+layout.Width+0.01 {
		t.Fatalf("title 溢出卡片右沿：%+v", title)
	}
}

// TestSolveGridRejectsBadTemplates 钉住模板错误 fail-loud（不静默画错）。
func TestSolveGridRejectsBadTemplates(t *testing.T) {
	cases := []struct {
		name string
		tmpl GridTemplate
	}{
		{"空模板", GridTemplate{}},
		{"区域行数与轨道行数不一致", GridTemplate{
			Columns: []Track{{Kind: TrackFixed, Value: 10}},
			Rows:    []Track{{Kind: TrackAuto}},
			Areas:   [][]string{{"a"}, {"b"}},
		}},
		{"区域列数与轨道列数不一致", GridTemplate{
			Columns: []Track{{Kind: TrackFixed, Value: 10}, {Kind: TrackMinMax, Fr: 1}},
			Rows:    []Track{{Kind: TrackAuto}},
			Areas:   [][]string{{"a"}},
		}},
		{"区域名重复", GridTemplate{
			Columns: []Track{{Kind: TrackFixed, Value: 10}, {Kind: TrackFixed, Value: 10}},
			Rows:    []Track{{Kind: TrackAuto}},
			Areas:   [][]string{{"a", "a"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SolveGrid(tc.tmpl, 0, 0, 100, 100, nil); err == nil {
				t.Fatal("坏模板必须报错（fail-loud），不能静默布局")
			}
		})
	}
}

// TestSolveGridAutoRowUsesContentHeight 钉住 auto 行取内容高。
func TestSolveGridAutoRowUsesContentHeight(t *testing.T) {
	tmpl := GridTemplate{
		Columns: []Track{{Kind: TrackFixed, Value: 100}},
		Rows:    []Track{{Kind: TrackAuto}, {Kind: TrackMinMax, Fr: 1}},
		Areas:   [][]string{{"a"}, {"b"}},
	}
	layout, err := SolveGrid(tmpl, 0, 0, 100, 500, []float32{54})
	if err != nil {
		t.Fatal(err)
	}
	if layout.Rows[0] != 54 || layout.Rows[1] != 446 {
		t.Fatalf("auto 行 = %v / %v，期望 54 / 446", layout.Rows[0], layout.Rows[1])
	}
	// 没给内容高时 auto 行是 0（调用方必须传；测试钉住这个「塌陷」可见）。
	layout, err = SolveGrid(tmpl, 0, 0, 100, 500, nil)
	if err != nil {
		t.Fatal(err)
	}
	if layout.Rows[0] != 0 {
		t.Fatalf("未给 auto 内容高时第一行 = %v，应塌成 0（调用方 bug 会立刻可见）", layout.Rows[0])
	}
}

// TestGridAreasAreDistinct 钉住四区互不重叠（同一格不会被两个区域占用）。
func TestGridAreasAreDistinct(t *testing.T) {
	layout, err := LayoutSettingsPanel(1182, 832, PanelOverlayTop)
	if err != nil {
		t.Fatal(err)
	}
	rects := make([]AreaRect, 0, len(GridAreaNames))
	for _, name := range GridAreaNames {
		area, ok := layout.Area(name)
		if !ok {
			t.Fatalf("缺区域 %q", name)
		}
		rects = append(rects, area)
	}
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			if overlaps(rects[i], rects[j]) {
				t.Fatalf("区域 %q 与 %q 重叠：%+v / %+v", GridAreaNames[i], GridAreaNames[j], rects[i], rects[j])
			}
		}
	}
}

// TestGridTemplateMatchesOfficialCSS 是「实现值 ↔ 官方 CSS」的文本级对照。
//
// 官方原文（SettingsRoot.module.css:98-103）必须与模板一致；任一改一边就红。
func TestGridTemplateMatchesOfficialCSS(t *testing.T) {
	official := "grid-template-columns: 188px minmax(0, 1fr);\n" +
		"grid-template-rows: auto minmax(0, 1fr);\n" +
		"grid-template-areas: 'title header' 'nav options';"
	got := renderTemplate(SettingsPanelGrid)
	if got != official {
		t.Fatalf("模板渲染与官方 CSS 不一致：\n--- got ---\n%s\n--- want ---\n%s", got, official)
	}
}

func renderTemplate(tmpl GridTemplate) string {
	var b strings.Builder
	b.WriteString("grid-template-columns: ")
	for i, c := range tmpl.Columns {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(renderTrack(c))
	}
	b.WriteString(";\ngrid-template-rows: ")
	for i, r := range tmpl.Rows {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(renderTrack(r))
	}
	b.WriteString(";\ngrid-template-areas:")
	for _, row := range tmpl.Areas {
		b.WriteString(" '")
		b.WriteString(strings.Join(row, " "))
		b.WriteString("'")
	}
	b.WriteString(";")
	return b.String()
}

func renderTrack(t Track) string {
	switch t.Kind {
	case TrackFixed:
		return itoa(int(t.Value)) + "px"
	case TrackAuto:
		return "auto"
	case TrackMinMax:
		return "minmax(" + itoa(int(t.Min)) + ", " + itoa(int(t.Fr)) + "fr)"
	default:
		return "?"
	}
}

func gridAreaName(name string) (AreaRect, bool) {
	for _, row := range SettingsPanelGrid.Areas {
		for _, cell := range row {
			if cell == name {
				return AreaRect{}, true
			}
		}
	}
	return AreaRect{}, false
}

func overlaps(a, b AreaRect) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}
