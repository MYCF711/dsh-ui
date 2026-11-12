//go:build darwin || windows || linux

package dsopen

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsrightbar"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/MYCF711/dsh-ui/plugins/dstopbar"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// openHeaderSession 起一个**真实装配**的无头窗口：ds-core + ds-shell +
// ds-rightbar + ds-topbar + ds-open，和 cmd/app 的槽位树一样。
//
// 这里不用"模拟一个头部行"的裸 canvas：旧实现正是死在真实槽位上——
// 在槽位列里再声明一行会**另起一行**，按钮落到 header 下面（y=96.5），
// 标题行却在 22…52。裸 canvas 测不出这件事。
func openHeaderSession(t *testing.T, data *pluginapi.Snapshot, act pluginapi.Actions, width, height int) *headless.Session {
	t.Helper()
	kernel := uikernel.New("open-header")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{}, &dsrightbar.Plugin{},
		&dstopbar.Plugin{}, &Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("open-header")
	var (
		view        ui.Page
		invalidator app.Invalidator
	)
	config := app.Config{
		Title: "open-in-app-header", Width: width, Height: height, Tree: tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
	}
	config.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.BgBase)
		env := &pluginapi.Env{
			Theme: theme, Data: *data, Act: act, Tree: tree, Feat: feat, Canvas: canvas,
			Bounds: bounds, MainWidth: bounds.Width - 280,
			Geometry: pluginapi.Geometry{
				Width: bounds.Width, Height: bounds.Height, SidebarWidth: 280,
				MainWidth: bounds.Width - 280,
			},
			Has: func(slot pluginapi.Slot) bool { return kernel.Has(slot) },
		}
		view.Draw(canvas, dscore.VivaTheme(theme), bounds, func(f ui.C) {
			kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
		})
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 一帧就够：CWD 与动作都在 Snapshot 里，不需要等 session.load。
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session
}

func openHeaderData(cwd string) pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Cur:      "s-open",
		CWD:      cwd,
		Sessions: []apiclient.Session{{ID: "s-open", Title: "会话一", Cwd: cwd}},
		Msgs:     []apiclient.Message{{Role: "user", Text: "hello", Seq: 1}},
		Total:    1,
	}
}

// TestButtonLivesInTheHeaderTitleRowAtBothSizes 是"可见、非 0 高、不挤压页签"
// 的主证据，两个产品尺寸都跑。
//
// 判据不写死坐标：标题行底 = 页签顶 - tabsMarginTop(10)，标题行高 = 30。
// 按钮必须整颗落在这一带里，且垂直居中；页签自己的矩形在有/无按钮两帧里
// 必须逐像素相同——这就是"不挤压页签"。
func TestButtonLivesInTheHeaderTitleRowAtBothSizes(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{name: "1182x832", width: 1182, height: 832}, {name: "1920x1032", width: 1920, height: 1032}} {
		t.Run(size.name, func(t *testing.T) {
			data := openHeaderData(`D:\ws\项目`)
			act := pluginapi.Actions{
				OpenFile:       func(string) error { return nil },
				OpenExternal:   func(string) error { return nil },
				ToggleRightbar: func() {},
			}
			session := openHeaderSession(t, &data, act, size.width, size.height)
			defer session.Close()

			forcedRepaint := func() {
				t.Helper()
				if err := session.Resize(size.width+1, size.height); err != nil {
					t.Fatal(err)
				}
				if err := session.Resize(size.width, size.height); err != nil {
					t.Fatal(err)
				}
			}
			forcedRepaint()

			hit, ok := session.Hit("在工作目录里打开")
			if !ok {
				t.Fatalf("%dx%d 找不到「在工作目录里打开」按钮：texts=%q", size.width, size.height, session.Texts())
			}
			if hit.Bounds.Width != 56 || hit.Bounds.Height != 24 {
				t.Fatalf("%dx%d 按钮 %vx%v，want 56x24（0 高就是「看不见」）",
					size.width, size.height, hit.Bounds.Width, hit.Bounds.Height)
			}
			tabs, ok := session.Hit("对话")
			if !ok {
				t.Fatalf("%dx%d 页签不见了：texts=%q", size.width, size.height, session.Texts())
			}
			const (
				tabsMarginTop  = 10
				titleRowHeight = 30
			)
			rowBottom := tabs.Bounds.Y - tabsMarginTop
			rowTop := rowBottom - titleRowHeight
			if hit.Bounds.Y < rowTop || hit.Bounds.Y+hit.Bounds.Height > rowBottom {
				t.Fatalf("%dx%d 按钮 y=%v..%v 不在标题行 %v..%v 里——它是另起了一行、落到正文区",
					size.width, size.height, hit.Bounds.Y, hit.Bounds.Y+hit.Bounds.Height, rowTop, rowBottom)
			}
			if center, want := hit.Bounds.Y+hit.Bounds.Height/2, rowTop+titleRowHeight/2; center != want {
				t.Errorf("%dx%d 按钮垂直中心 %v，want 标题行中心 %v", size.width, size.height, center, want)
			}

			// 有按钮 vs 没按钮：页签几何必须一模一样。
			data.CWD = ""
			forcedRepaint()
			if _, stillThere := session.Hit("在工作目录里打开"); stillThere {
				t.Fatalf("%dx%d 没有 CWD 仍然画着按钮", size.width, size.height)
			}
			tabsWithout, ok := session.Hit("对话")
			if !ok || tabsWithout.Bounds != tabs.Bounds {
				t.Fatalf("%dx%d 按钮消失后页签动了：%+v → %+v（按钮不许挤压页签）",
					size.width, size.height, tabs.Bounds, tabsWithout.Bounds)
			}
			data.CWD = `D:\ws\项目`
			forcedRepaint()
			again, ok := session.Hit("在工作目录里打开")
			if !ok || again.Bounds != hit.Bounds {
				t.Fatalf("%dx%d 恢复 CWD 后按钮位置不稳定：%+v → %+v", size.width, size.height, hit.Bounds, again.Bounds)
			}
			if !session.HasText("打开") {
				t.Errorf("%dx%d 按钮只有底色没有文案：%q", size.width, size.height, session.Texts())
			}
		})
	}
}

// TestButtonDoesNotCoverTheRightbarCorner 钉住 utilities 与 headerCorner 的关系：
// 官方 headerUtilities 在 corner 左边 8px。旧实现把按钮放在 header 下面，
// 这一条对它是"按钮不存在"，同样是红。
func TestButtonDoesNotCoverTheRightbarCorner(t *testing.T) {
	data := openHeaderData(`D:\ws\项目`)
	act := pluginapi.Actions{
		OpenFile:       func(string) error { return nil },
		ToggleRightbar: func() {},
	}
	session := openHeaderSession(t, &data, act, 1182, 832)
	defer session.Close()

	button, ok := session.Hit("在工作目录里打开")
	if !ok {
		t.Fatalf("找不到按钮：texts=%q", session.Texts())
	}
	corner, ok := session.Hit("打开右侧边栏")
	if !ok {
		t.Fatalf("找不到 28px corner：texts=%q", session.Texts())
	}
	if gap := corner.Bounds.X - (button.Bounds.X + button.Bounds.Width); gap != 8 {
		t.Errorf("按钮右缘到 corner 左缘 = %v，want 8（官方 headerUtilities gap）", gap)
	}
	if overlaps(button.Bounds, corner.Bounds) {
		t.Errorf("按钮盖住了 corner：button=%+v corner=%+v", button.Bounds, corner.Bounds)
	}
}

func overlaps(a, b ui.Rect) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width &&
		a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}
