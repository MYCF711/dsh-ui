//go:build darwin || windows || linux

package dsdirpicker

// 真实光栅化证据：几何断言能证明"盒子是 22px 高"，但证明不了
// **官方图标与官方 elevation 真的被画出来了**。这一帧走真实装配
// （uikernel + dscore + 本插件 + headless 驱动），量的是 drawn primitive。
//
// 官方出处见 browse.go 的常量注释；这里只钉"画的是不是那个东西"。

import (
	"context"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// visualSource 是够画一帧的最小数据面（home 一层 + 两个普通目录）。
type visualSource struct{}

func (visualSource) List(_ context.Context, path string) (*apiclient.DirectoryListing, error) {
	listing := homeListing()
	listing.Path = path
	return listing, nil
}

func (visualSource) Create(_ context.Context, parent, name string) (string, error) {
	return parent + "/" + name, nil
}

func (visualSource) Register(_ context.Context, path string) (apiclient.WorkspaceView, error) {
	return apiclient.WorkspaceView{WorkspaceID: "ws-home", Path: path}, nil
}

// pickerSession 起一个真实装配的无头窗口，只画 `shell.overlay` 这一槽。
func pickerSession(t *testing.T, width, height int) (*headless.Session, *Plugin) {
	t.Helper()
	kernel := uikernel.New("dirpicker-visual")
	p := &Plugin{run: func(fn func()) { fn() }}
	if err := kernel.Load(&dscore.Plugin{Dark: true}, p); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("dirpicker-visual")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	config := app.Config{
		Title: "dirpicker-visual", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
			Geometry: pluginapi.Geometry{Width: bounds.Width, Height: bounds.Height},
			Has:      func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
		}
		view.Draw(canvas, bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellOverlay, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)

	SetSource(visualSource{})
	SetOnPicked(nil)
	t.Cleanup(func() {
		SetSource(nil)
		SetOnPicked(nil)
	})
	if !p.Open() {
		t.Fatal("有 source 时 Open 应该打开对话框")
	}
	// 窗口是**先开、后置 open** 的：headless 驱动只在尺寸变化时重画，
	// 所以这里撑一次再还原，逼出一帧"对话框已经打开"的画面。
	if err := session.Resize(width+1, height); err != nil {
		t.Fatal(err)
	}
	if err := session.Resize(width, height); err != nil {
		t.Fatal(err)
	}
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session, p
}

// TestCardUsesOfficialModalChrome 钉住 Modal 卡片本体的官方值：官方
// `._dialog_1ytzq_33`（dsh-web-frontend/dist/assets/index-CSAdOBx6.css，偏移 32831）
// 是 `border:0; border-radius:var(--dsw-radius-panel); background:var(--dsw-alias-bg-layer-2);
// box-shadow:var(--dsw-elevation-prominent)`——**radius-panel（28），不是 XL（20）**。
func TestCardUsesOfficialModalChrome(t *testing.T) {
	session, _ := pickerSession(t, 1200, 800)
	theme := dscore.Dark()
	snapshot := session.Snapshot()

	var card *app.Primitive
	for i := range snapshot.Primitives {
		primitive := snapshot.Primitives[i]
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		if primitive.Color != theme.BgLayer2 {
			continue
		}
		if primitive.Bounds.Width != dialogMaxWidth || primitive.Bounds.Height != dialogMaxHeight {
			continue
		}
		card = &snapshot.Primitives[i]
		break
	}
	if card == nil {
		t.Fatalf("找不到 %vx%v 的 bg-layer-2 卡片（画出来的矩形见 snapshot.Primitives）",
			dialogMaxWidth, dialogMaxHeight)
	}
	if card.Radius != theme.Radii.Panel {
		t.Errorf("卡片圆角=%v，want %v（radius-panel；官方 Modal 不是 XL 的 %v）",
			card.Radius, theme.Radii.Panel, theme.Radii.XL)
	}
}

// TestCardCastsOfficialProminentElevation 钉住 `--dsw-elevation-prominent` 两层柔光：
//
//	0 3px 8px 0 rgba(0,0,0,.04) + 0 0 20px 0 rgba(0,0,0,.05)
//	（dsh-client-ui-theme/src/styles/gradient-shadow-text.css:36-37）
//
// 判据是**原始层数与偏移/模糊/透明度**，不是"有一团阴影"：Viva 的 ShadowXS/MD
// 是另一套 shadcn 档位，用它会画出 0/1/2/5% 那种值，不是官方的这四层。
//
// 只数**卡片那一块矩形**（680×500）上的柔光：footer 的 outline 按钮自带
// ShadowXS（0/1/2/5%），那是按钮 recipe 的，不是卡片的。
func TestCardCastsOfficialProminentElevation(t *testing.T) {
	session, _ := pickerSession(t, 1200, 800)
	snapshot := session.Snapshot()

	var shadows []app.Primitive
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.ShadowPrimitive {
			continue
		}
		if primitive.Bounds.Width != dialogMaxWidth || primitive.Bounds.Height != dialogMaxHeight {
			continue
		}
		shadows = append(shadows, primitive)
	}
	if len(shadows) != 2 {
		t.Fatalf("卡片柔光=%d 层，want 2（elevation-prominent 的两层）", len(shadows))
	}
	want := []struct {
		offsetY, blur, alpha float32
	}{
		{modalElevationShadowY, modalElevationShadowB, modalElevationShadowA},
		{0, modalElevationGlowB, modalElevationGlowA},
	}
	for i, layer := range want {
		got := shadows[i]
		if got.Shadow.OffsetY != layer.offsetY || got.Shadow.Blur != layer.blur {
			t.Errorf("第 %d 层柔光 offset/blur=%v/%v，want %v/%v（gradient-shadow-text.css:36-37）",
				i, got.Shadow.OffsetY, got.Shadow.Blur, layer.offsetY, layer.blur)
		}
		if d := got.Color.Alpha - layer.alpha; d > 0.001 || d < -0.001 {
			t.Errorf("第 %d 层柔光 alpha=%v，want %v", i, got.Color.Alpha, layer.alpha)
		}
	}
}

// triangleColorsInside 收集落在 rect 里的矢量三角形的颜色集合（图标是矢量三角形）。
func triangleColorsInside(snapshot app.Snapshot, rect gfx.Rect) map[gfx.Color]int {
	out := map[gfx.Color]int{}
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.TrianglePrimitive {
			continue
		}
		inside := true
		for _, point := range primitive.Points {
			if point.X < rect.X || point.X > rect.X+rect.Width ||
				point.Y < rect.Y || point.Y > rect.Y+rect.Height {
				inside = false
				break
			}
		}
		if inside {
			out[primitive.Color]++
		}
	}
	return out
}

// TestRowDrawsOfficialFolderAndChevronIcons 一行里必须真的有官方那两枚图标：
// 行首 16px folder（`label-secondary`，TSX:256-257 / module.css:283-286）与
// 行尾 12px chevron（`label-tertiary`，TSX:259 / module.css:305-308）。
func TestRowDrawsOfficialFolderAndChevronIcons(t *testing.T) {
	session, _ := pickerSession(t, 1200, 800)
	theme := dscore.Dark()
	snapshot := session.Snapshot()

	row, ok := session.Hit("projects")
	if !ok {
		t.Fatalf("找不到行 projects 的命中区：texts=%q", session.Texts())
	}
	colors := triangleColorsInside(snapshot, row.Bounds)
	if colors[theme.LabelSecondary] == 0 {
		t.Errorf("行首没有 label-secondary 的 folder 图标：三角形颜色=%v", colors)
	}
	if colors[theme.LabelTertiary] == 0 {
		t.Errorf("行尾没有 label-tertiary 的 chevron 图标：三角形颜色=%v", colors)
	}
}

// TestCrumbBarDrawsOfficialChevronAndPencil 面包屑条里必须真的有两枚官方矢量：
// 分隔用的 12px chevron（TSX:827）与右端 14px 铅笔（TSX:875，本版按官方
// 禁用态取 `label-caption`，module.css:179-185）。
func TestCrumbBarDrawsOfficialChevronAndPencil(t *testing.T) {
	session, _ := pickerSession(t, 1200, 800)
	theme := dscore.Dark()
	snapshot := session.Snapshot()

	home, ok := session.Hit("主目录")
	if !ok {
		t.Fatalf("找不到面包屑「主目录」：texts=%q", session.Texts())
	}
	// 面包屑条：从 home 那段往上到标题行下沿都算这一带。
	band := gfx.Rect{X: 0, Y: home.Bounds.Y - 8, Width: 1200, Height: 40}
	colors := triangleColorsInside(snapshot, band)
	if colors[theme.LabelCaption] == 0 {
		t.Errorf("面包屑右端没有 label-caption 的铅笔图标：三角形颜色=%v", colors)
	}
	// 铅笔必须落在条的右侧（`.crumbEditZone{justify-content:flex-end}`）。
	pencilRightHalf := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.TrianglePrimitive || primitive.Color != theme.LabelCaption {
			continue
		}
		inside := true
		for _, point := range primitive.Points {
			if point.X < band.X || point.X > band.X+band.Width ||
				point.Y < band.Y || point.Y > band.Y+band.Height {
				inside = false
				break
			}
		}
		if inside && primitive.Points[0].X > 600 {
			pencilRightHalf = true
			break
		}
	}
	if !pencilRightHalf {
		t.Error("铅笔没有落在面包屑条右半边（官方 justify-content:flex-end）")
	}
}
