//go:build windows || darwin || linux

// official_markdown_bold_visual_test.go —— 整块加粗的**真像素**证据。
//
// 两条判据（本仓成熟做法：不看文字标签的声明，看画出来的像素）：
//
//	① 正判据：同一句话，整块 `**…**` 画出来的**墨迹比常规宽**（官方 `<strong>`
//	   是 700 档字面，不是"画一层描边"也不是"颜色加重"）；
//	② 反向：**常规段落逐像素不变**——把 transcript 里画的常规段落，与"拿同一档
//	   正文样式直接摆一个纯文本 `Wrap`"的参照页在同一坐标上逐像素比对，必须全等。
//	   这条在改动前后都成立（改动前它证明参照页选得对，改动后它证明加粗那条分支
//	   没有碰到常规文本）。
//
// 为什么用参照页而不是钉一张 golden PNG：golden 会随机器字体/DPI 变化，而"常规段落
// == 同一档样式的纯 `Wrap`"是机器无关的等式。
package dsconversation

import (
	"image"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	boldVisualWidth  = 900
	boldVisualHeight = 400
	// boldInkThreshold 是"这一像素算墨迹"的亮度差（背景是深色，字是浅色）。
	boldInkThreshold = 24.0
)

// openTranscriptPixels 用真实 headless 窗口画一条助手正文，返回帧与像素。
func openTranscriptPixels(t *testing.T, markdown string) (*headless.Session, *pluginapi.Theme, image.Image) {
	t.Helper()
	theme := dscore.Dark()
	view := NewOfficialTranscriptView(theme, &dscore.Primitives{Theme: theme})
	tree := &gfx.Tree{}
	feat := tree.Feature("markdown-bold-visual")
	var invalidator app.Invalidator
	data := pluginapi.Snapshot{Cur: "s-markdown-bold-visual"}
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "markdown-bold-visual", Width: boldVisualWidth, Height: boldVisualHeight,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: data, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: size.Width,
		}
		page.Draw(canvas, bounds, func(f ui.C) {
			view.Draw(f, env, TranscriptViewport{
				HostScroll: true, HostID: tree.ID(feat, "host"), SelfID: tree.ID(feat, "self"),
			}, assistantMarkdownText(markdown))
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatalf("取像素失败：%v", err)
	}
	if picture.Bounds().Dx() != boldVisualWidth || picture.Bounds().Dy() != boldVisualHeight {
		t.Fatalf("像素尺寸 = %v，期望 %dx%d", picture.Bounds(), boldVisualWidth, boldVisualHeight)
	}
	return session, theme, picture
}

// openPlainWrapPixels 画一个"只有纯文本 Wrap"的参照页：样式与 transcript 正文同一档，
// 摆在同一个坐标上。返回的标签矩形用来断言两页真的同位。
func openPlainWrapPixels(t *testing.T, value string, x, y float32) (*headless.Session, image.Image) {
	t.Helper()
	theme := dscore.Dark()
	tree := &gfx.Tree{}
	var invalidator app.Invalidator
	pageTheme := dscore.VivaTheme(theme)
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	config := app.Config{
		Title: "plain-wrap-reference", Width: boldVisualWidth, Height: boldVisualHeight,
		Tree: tree, WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		page.Draw(canvas, bounds, func(f ui.C) {
			ui.Column(f).FillWidth().PadSides(y, 0, 0, x).Children(func() {
				ui.Wrap(f, value).FontSize(theme.Fonts.Body).LineHeight(theme.Fonts.BodyLine).
					Color(theme.LabelPrimary)
			})
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("参照页 headless 打不开：%v", err)
	}
	t.Cleanup(func() { session.Close() })
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	picture, err := session.Image()
	if err != nil {
		t.Fatalf("参照页取像素失败：%v", err)
	}
	return session, picture
}

// paintedText 在帧里按值找一段文字，连它这一帧量出来的宽度一起返回。
func paintedText(t *testing.T, session *headless.Session, value string) (gfx.Text, float32) {
	t.Helper()
	snapshot := session.Snapshot()
	for i, text := range snapshot.Texts {
		if text.Value != value {
			continue
		}
		width := float32(0)
		if i < len(snapshot.TextWidths) {
			width = snapshot.TextWidths[i]
		}
		return text, width
	}
	var got []string
	for _, text := range snapshot.Texts {
		got = append(got, text.Value)
	}
	t.Fatalf("帧里没有文字 %q；实际文本：%v", value, got)
	return gfx.Text{}, 0
}

// luma 是一个像素的感知亮度（0–255）。
func luma(r, g, b uint32) float64 {
	return 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(b>>8)
}

// backgroundLuma 是页面底色（`theme.BgBase`）的亮度。
func backgroundLuma(theme *pluginapi.Theme) float64 {
	color := theme.BgBase
	return luma(uint32(color.Red*255+0.5), uint32(color.Green*255+0.5), uint32(color.Blue*255+0.5))
}

// inkBounds 在一个矩形范围内找墨迹（与底色差得足够远的像素）的包围框。
//
// used=false 表示这块区域全是底色（一个字都没画出来）。
func inkBounds(picture image.Image, rect gfx.Rect, background float64) (ink gfx.Rect, used bool) {
	x0, y0 := int(rect.X), int(rect.Y)
	x1, y1 := int(rect.X+rect.Width), int(rect.Y+rect.Height)
	bounds := picture.Bounds()
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
	minX, minY, maxX, maxY := x1, y1, x0-1, y0-1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := picture.At(x, y).RGBA()
			if diff := luma(r, g, b) - background; diff < boldInkThreshold && diff > -boldInkThreshold {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return gfx.Rect{}, false
	}
	return gfx.Rect{X: float32(minX), Y: float32(minY), Width: float32(maxX - minX + 1), Height: float32(maxY - minY + 1)}, true
}

// TestBoldParagraphInkIsWiderThanRegular 是正判据：**粗体真的比常规宽**。
//
// 三份证据一起看，缺一不可：
//
//	① 字重是 700 档（不是"看起来像"）；
//	② 这一帧量出来的宽度更大（布局用的是加粗那一档进宽，不是常规字面撑出来的）；
//	③ 画到屏幕上的墨迹更宽（像素层的结论，排除"只改了量宽没改字形"）。
func TestBoldParagraphInkIsWiderThanRegular(t *testing.T) {
	value := "bravo charlie"
	plainSession, theme, plainPicture := openTranscriptPixels(t, value)
	boldSession, _, boldPicture := openTranscriptPixels(t, "**"+value+"**")
	background := backgroundLuma(theme)

	plainText, plainWidth := paintedText(t, plainSession, value)
	boldText, boldWidth := paintedText(t, boldSession, value)

	if plainText.Style.Weight != gfx.WeightRegular {
		t.Fatalf("常规段落字重 = %v，应为 gfx.WeightRegular", plainText.Style.Weight)
	}
	if boldText.Style.Weight != gfx.WeightBold {
		t.Fatalf("整块加粗段落字重 = %v，应为 gfx.WeightBold", boldText.Style.Weight)
	}
	if boldWidth <= plainWidth {
		t.Fatalf("加粗量宽 = %v，常规 = %v；加粗必须更宽", boldWidth, plainWidth)
	}
	if boldText.Bounds.Height != plainText.Bounds.Height {
		t.Fatalf("加粗行盒高 = %v，常规 = %v；一句话不该因此换行", boldText.Bounds.Height, plainText.Bounds.Height)
	}

	plainInk, ok := inkBounds(plainPicture, plainText.Bounds, background)
	if !ok {
		t.Fatal("常规段落一个墨迹像素都没有")
	}
	boldInk, ok := inkBounds(boldPicture, boldText.Bounds, background)
	if !ok {
		t.Fatal("加粗段落一个墨迹像素都没有")
	}
	if boldInk.Width <= plainInk.Width {
		t.Fatalf("像素层：加粗墨迹宽 %v（x=%v..%v），常规 %v（x=%v..%v）；粗体必须更宽",
			boldInk.Width, boldInk.X, boldInk.X+boldInk.Width, plainInk.Width, plainInk.X, plainInk.X+plainInk.Width)
	}
	t.Logf("BOLD_INK 常规量宽=%.3f 墨迹=%.0f | 加粗量宽=%.3f 墨迹=%.0f", plainWidth, plainInk.Width, boldWidth, boldInk.Width)
}

// TestPlainParagraphPixelsEqualAPlainWrap 是反向判据：**常规文本逐像素不变**。
//
// 等式：transcript 画出来的段落 == 用同一档正文样式（字号/行高/颜色/同一列宽）
// 直接摆的纯文本 `Wrap`。任何"顺手给常规段落也加了点什么"的改动都会让这条红。
//
// 两种常规文本一起比：**完全没加粗的段落**，以及**混排段落**（`**alpha bravo** charlie`：
// 盒级没有行内 run，按设计整段仍是常规字重，必须与纯文本逐像素相同——不是"差不
// 多"，是同一个像素）。
func TestPlainParagraphPixelsEqualAPlainWrap(t *testing.T) {
	for _, tc := range []struct{ name, markdown, value string }{
		{"plain", "alpha bravo charlie", "alpha bravo charlie"},
		{"mixed", "**alpha bravo** charlie", "alpha bravo charlie"},
	} {
		t.Run(tc.name, func(t *testing.T) { assertPlainPixels(t, tc.markdown, tc.value) })
	}
}

// assertPlainPixels 断言 markdown 画出来的那段文字与"纯文本 Wrap"逐像素相同。
func assertPlainPixels(t *testing.T, markdown, value string) {
	t.Helper()
	session, _, picture := openTranscriptPixels(t, markdown)
	text, _ := paintedText(t, session, value)
	if text.Style.Weight != gfx.WeightRegular {
		t.Fatalf("这段文字的字重 = %v，应为常规（加粗只该落在整块 `**…**` 上）", text.Style.Weight)
	}

	reference, referencePicture := openPlainWrapPixels(t, value, text.Bounds.X, text.Bounds.Y)
	referenceText, _ := paintedText(t, reference, value)
	if referenceText.Bounds.X != text.Bounds.X || referenceText.Bounds.Y != text.Bounds.Y {
		t.Fatalf("参照页文字位置 = (%.1f,%.1f)，transcript = (%.1f,%.1f)；参照页必须同位",
			referenceText.Bounds.X, referenceText.Bounds.Y, text.Bounds.X, text.Bounds.Y)
	}
	if referenceText.Style != text.Style {
		t.Fatalf("参照页字样 = %+v，transcript = %+v；两边必须同一档样式", referenceText.Style, text.Style)
	}

	x0, y0 := int(text.Bounds.X), int(text.Bounds.Y)
	x1, y1 := int(text.Bounds.X+text.Bounds.Width), int(text.Bounds.Y+text.Bounds.Height)
	compared := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ar, ag, ab, aa := picture.At(x, y).RGBA()
			br, bg, bb, ba := referencePicture.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				t.Fatalf("常规段落像素在 (%d,%d) 变了：transcript=%v，参照页=%v；常规文本必须逐像素不变",
					x, y, []uint32{ar, ag, ab, aa}, []uint32{br, bg, bb, ba})
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("没有比较任何像素：矩形是空的")
	}
	t.Logf("PLAIN_PIXELS_IDENTICAL 比较了 %d 个像素（矩形 %v）", compared, text.Bounds)
}
