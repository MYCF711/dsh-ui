// official_grid.go —— 设置卡片的 **CSS Grid 等价布局求解器**（rc.2 真变 D1）。
//
// # 为什么要自己解网格
//
// rc.1 台账把卡片写成「内部横向 flex」，rc.2 实测是**四区 CSS Grid**
// （ui-settings-general/src/client/SettingsRoot.module.css:95-118）：
//
//	display: grid;
//	grid-template-columns: 188px minmax(0, 1fr);
//	grid-template-rows: auto minmax(0, 1fr);
//	grid-template-areas:
//	  'title header'
//	  'nav options';
//
// 布局语义变了，实现就不能再用「Row() 里嵌 Column()」拼出来：flex 拼出来的
// 卡片在窄窗口下**两列会平分**（188px 轨道丢失），而且 `auto` 行高的来源
// （navTitle 与 header 取大）无法表达。所以这里实现一个**最小网格求解器**：
// 轨道（fixed / auto / minmax）+ 区域（area 名 → 矩形），绘制代码按区域名取
// 矩形、用 Absolute 定位——这就是本仓对 CSS Grid 的等价物。
//
// 与 viva 的 `flex.Grid` 的区别：viva 的 Grid 是**等宽列**（按 minCell 自动排），
// 表达不了 `188px minmax(0,1fr)` 这种「固定 + 弹性」混合轨道。这里不是绕开
// viva，是 viva 没有这个形状（见报告「未实现项」）。
//
// # 判据
//
// 求解器是纯函数：给模板 + 容器尺寸 + auto 行内容高，出每个区域的矩形。
// 测试直接断言矩形（例如 800 宽卡片 → 导航列 188、内容列 612）。
package dssettings

import "fmt"

// TrackKind 是一条网格轨道的尺寸种类。
type TrackKind string

const (
	// TrackFixed 固定像素（官方的 `188px`）。
	TrackFixed TrackKind = "fixed"
	// TrackAuto 内容自适应（官方的 `auto`）——高度由这一行的内容高决定。
	TrackAuto TrackKind = "auto"
	// TrackMinMax 弹性轨道（官方的 `minmax(0, 1fr)`，以及 `minmax(0, 1fr)` 的行）。
	TrackMinMax TrackKind = "minmax"
)

// Track 是一条轨道。
type Track struct {
	Kind TrackKind
	// Value 是 fixed 轨道的像素值。
	Value float32
	// Min / Max 是 minmax 轨道的边界；Max=0 表示无上限（`1fr`）。
	Min float32
	Max float32
	// Fr 是弹性系数（`1fr` → 1）。0 表示这条轨道不参与剩余空间分配。
	Fr float32
}

// AreaRect 是一个区域在容器内的矩形（相对容器左上角）。
type AreaRect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// GridTemplate 是一个网格模板。
type GridTemplate struct {
	Columns []Track
	Rows    []Track
	// Areas 是行优先的区域名矩阵；空串表示该格不属于任何区域。
	Areas [][]string
}

// GridLayout 是一次求解的结果。
type GridLayout struct {
	// X / Y 是容器在窗口内的左上角。
	X      float32
	Y      float32
	Width  float32
	Height float32
	// Columns / Rows 是求出来的轨道尺寸。
	Columns []float32
	Rows    []float32
	// Areas 是区域名 → 矩形（**绝对坐标**，已加上 X/Y，绘制层直接 Absolute 用）。
	Areas map[string]AreaRect
}

// SettingsPanelGrid 是设置卡片的官方四区网格（rc.2 唯一形状）。
//
// 官方原文见文件头。`title` 与 `nav` 在左列，`header` 与 `options` 在右列；
// 两列的宽度由 `188px + 1fr` 决定，两行由 `auto + 1fr` 决定。
var SettingsPanelGrid = GridTemplate{
	Columns: []Track{
		{Kind: TrackFixed, Value: GridNavWidth},
		{Kind: TrackMinMax, Min: 0, Fr: 1},
	},
	Rows: []Track{
		{Kind: TrackAuto},
		{Kind: TrackMinMax, Min: 0, Fr: 1},
	},
	Areas: [][]string{
		{"title", "header"},
		{"nav", "options"},
	},
}

// GridAreaNames 是官方四区名（DOM 顺序：左标题、右列头、左导航、右内容）。
//
// 顺序不是绘制顺序（绘制按 Areas 名字取），而是官方 JSX 里四个席位的出现顺序，
// 测试用它断言「四席一个都不少」。
var GridAreaNames = [4]string{"title", "header", "nav", "options"}

// SolveGrid 解一个网格模板。
//
// autoSizes 是每个 auto 行的内容高（长度必须等于 Rows 里 auto 轨道的数量；
// 少给按 0 处理，多给忽略——但**缺 auto 行高**会让卡片第一行塌成 0，所以
// 测试必须同时钉住「navTitle 与 header 都算进来」）。
func SolveGrid(tmpl GridTemplate, x, y, width, height float32, autoSizes []float32) (GridLayout, error) {
	if len(tmpl.Columns) == 0 || len(tmpl.Rows) == 0 {
		return GridLayout{}, fmt.Errorf("网格模板不能为空（columns=%d rows=%d）", len(tmpl.Columns), len(tmpl.Rows))
	}
	if len(tmpl.Areas) != len(tmpl.Rows) {
		return GridLayout{}, fmt.Errorf("区域矩阵行数 %d 与轨道行数 %d 不一致", len(tmpl.Areas), len(tmpl.Rows))
	}
	seen := make(map[string]bool, len(tmpl.Areas)*len(tmpl.Columns))
	for r, row := range tmpl.Areas {
		if len(row) != len(tmpl.Columns) {
			return GridLayout{}, fmt.Errorf("区域矩阵第 %d 行有 %d 列，与轨道列数 %d 不一致", r, len(row), len(tmpl.Columns))
		}
		for _, name := range row {
			if name == "" {
				continue
			}
			if seen[name] {
				return GridLayout{}, fmt.Errorf("区域名 %q 重复（同名区域会让两个席位画在同一个矩形上）", name)
			}
			seen[name] = true
		}
	}

	cols := solveTracks(tmpl.Columns, width, nil)
	rows := solveTracks(tmpl.Rows, height, autoSizes)

	out := GridLayout{
		X: x, Y: y, Width: width, Height: height,
		Columns: cols, Rows: rows,
		Areas: make(map[string]AreaRect, len(seen)),
	}
	colAt := offsets(cols)
	rowAt := offsets(rows)
	for r, row := range tmpl.Areas {
		for c, name := range row {
			if name == "" {
				continue
			}
			if _, ok := out.Areas[name]; ok {
				continue
			}
			out.Areas[name] = AreaRect{
				X:      x + colAt[c],
				Y:      y + rowAt[r],
				Width:  cols[c],
				Height: rows[r],
			}
		}
	}
	return out, nil
}

// solveTracks 解一维轨道：先固定，再按 fr 分剩余（clamp 到 min/max）。
//
// autoSizes 只对 auto 轨道有意义（行方向）；列方向传 nil。
func solveTracks(tracks []Track, total float32, autoSizes []float32) []float32 {
	out := make([]float32, len(tracks))
	var fixed, frTotal float32
	autoIdx := 0
	for i, t := range tracks {
		switch t.Kind {
		case TrackFixed:
			out[i] = t.Value
			fixed += t.Value
		case TrackAuto:
			var h float32
			if autoSizes != nil && autoIdx < len(autoSizes) {
				h = autoSizes[autoIdx]
			}
			autoIdx++
			out[i] = h
			fixed += h
		case TrackMinMax:
			frTotal += t.Fr
		}
	}
	remaining := total - fixed
	if remaining < 0 {
		remaining = 0
	}
	for i, t := range tracks {
		if t.Kind != TrackMinMax {
			continue
		}
		var size float32
		if frTotal > 0 && t.Fr > 0 {
			size = remaining * t.Fr / frTotal
		}
		if size < t.Min {
			size = t.Min
		}
		if t.Max > 0 && size > t.Max {
			size = t.Max
		}
		out[i] = size
	}
	return out
}

// offsets 把轨道尺寸转成累计偏移（第 0 条在 0）。
func offsets(sizes []float32) []float32 {
	out := make([]float32, len(sizes))
	var at float32
	for i, s := range sizes {
		out[i] = at
		at += s
	}
	return out
}

// LayoutSettingsPanel 按视口尺寸求卡片四区布局。
//
// 这是绘制层唯一的入口：卡片尺寸、居中位置、四区矩形都从这里出，
// 绘制代码不自己算（也就不会出现「两处各算一次、一处改了另一处没改」）。
func LayoutSettingsPanel(viewportW, viewportH, overlayTop float32) (GridLayout, error) {
	w, h := PanelSize(viewportW, viewportH, overlayTop)
	left := PanelLeft(viewportW, w)
	top := PanelTop(viewportH, h)
	// auto 行高 = 这一行两个席位的内容高取大：navTitle 54、header 54。
	// 两者都是官方常量，所以这里直接给 GridHeaderRowHeight。
	return SolveGrid(SettingsPanelGrid, left, top, w, h, []float32{GridHeaderRowHeight})
}

// Area 取一个区域矩形；不存在时返回零值 + false（绘制层据此不画，而不是画在 0,0）。
func (l GridLayout) Area(name string) (AreaRect, bool) {
	r, ok := l.Areas[name]
	return r, ok
}
