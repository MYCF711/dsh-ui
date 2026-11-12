//go:build windows || darwin || linux

// 本文件是「公开槽实画」的可复现证据：用**真 kernel + 真 dsshell + 真 dsconversation**
// 在一个真 viva 帧里跑 dsshortcuts，断言面板不只是"状态变成 shown=true"，
// 而是真的出现在画面像素上。
//
// 为什么必须有它：上一轮把"面板画不出来"记成了 Absolute 布局问题，靠截图猜了三轮。
// 实际上布局一直是好的——缺的是切换之后的那一次重绘（HandleKey 跑在绘制之外）。
// 这个测试把两条路径都钉住：
//
//   - `-keys` 路径：直接调 kernel.HandleKey（不经过 OnKey），只靠插件自己的
//     Actions.Notify 请求重绘 → 面板必须出现；
//   - 真按键路径：session.Keys("f1")/("escape") 经过 OnKey → 面板出现/消失。
package dsshortcuts

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dsconversation"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsshell"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const (
	sheetTestW = 1180
	sheetTestH = 800
	sheetTitle = "键盘快捷键"
)

// TestSheetIsPaintedInOverlaySlot 验证 shell.overlay 槽里的面板真的被画出来。
func TestSheetIsPaintedInOverlaySlot(t *testing.T) {
	dir := os.Getenv("U2_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	kernel := uikernel.New("u2-slot-proof")
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dsshell.Plugin{},
		&dsconversation.Plugin{}, &Plugin{}); err != nil {
		t.Fatal(err)
	}
	theme := kernel.MustGet(pluginapi.KeyTheme).(*pluginapi.Theme)
	tree := &gfx.Tree{}
	feat := tree.Feature("u2-slot-proof")
	pageTheme := dscore.VivaTheme(theme)
	view := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	var invalidator app.Invalidator
	// notify 用带缓冲的 channel 收集：Paint 和测试主 goroutine 不是一个线程，
	// 直接读写变量在 -race 下就是 data race。
	notices := make(chan string, 8)

	config := app.Config{
		Title: "u2-slot-proof", Width: sheetTestW, Height: sheetTestH,
		Tree: tree, Invalidator: &invalidator,
		Paint: func(canvas *gfx.Canvas, size ui.Size) {
			bounds := gfx.Rect{Width: size.Width, Height: size.Height}
			env := &pluginapi.Env{
				Theme: theme, Tree: tree, Feat: feat, Canvas: canvas, Bounds: bounds,
				// Cur 非空 + 无消息：ds-conversation 画官方首屏（hero），
				// 正好用来验证浮层是盖在正文之上的（绘制顺序）。
				Data:      pluginapi.Snapshot{Cur: "u2-slot-proof"},
				MainWidth: size.Width - 280,
				Geometry:  pluginapi.Geometry{Width: size.Width, Height: size.Height, SidebarWidth: 280, BottomHeight: 24},
			}
			env.Has = func(slot pluginapi.Slot) bool { return kernel.Has(slot) }
			// Actions.Notify 照壳的语义实现：写状态 + 请求一帧。
			env.Act.Notify = func(msg string) {
				select {
				case notices <- msg:
				default:
				}
				invalidator.Invalidate()
			}
			view.Draw(canvas, bounds, func(f ui.C) {
				kernel.DrawSlot(f, pluginapi.SlotShellRoot, env)
			})
		},
		// 真按键路径照 cmd/app 的 OnKey 写：HandleKey 成功就显式 Invalidate。
		OnKey: func(ev gfx.KeyEvent) gfx.EventResult {
			if kernel.HandleKey(ev) {
				invalidator.Invalidate()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if session.HasText(sheetTitle) {
		t.Fatalf("面板还没打开，画面上就有 %q 了", sheetTitle)
	}
	if !session.HasText("探索未至之境") {
		t.Fatalf("首屏 hero 没画出来，这个测试的绘制顺序断言就没有意义：texts=%q", session.Texts())
	}

	// ① `-keys` 路径：不经过 OnKey，只有插件自己在 HandleKey 里发出的 Notify。
	if !kernel.HandleKey(gfx.KeyEvent{Key: gfx.KeyF1}) {
		t.Fatal("-keys 路径：F1 没被 dsshortcuts 接住")
	}
	select {
	case msg := <-notices:
		t.Logf("切换后请求重绘：%q", msg)
	case <-time.After(2 * time.Second):
		t.Fatal("F1 切换后没有请求重绘（Actions.Notify 没被调用）：面板只会改状态、不会出现在画面上")
	}
	waitText(t, session, sheetTitle, true)
	if !session.HasText("按 F1 或 Esc 关闭") {
		t.Fatalf("面板标题出来了但内容没画全：texts=%q", session.Texts())
	}

	// ② 画的位置：必须是居中、宽 520 的卡片，不是一个 0 宽的东西。
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	left := (sheetTestW - sheetWidth) / 2
	assertColor(t, picture, left+5, sheetTop+50, theme.InputMajor, "面板内部（左内边距）")
	assertNotColor(t, picture, left-12, sheetTop+50, theme.InputMajor, "面板左边")

	// ③ 绘制顺序：hero 先、面板后（后者盖在前者上面）。
	texts := session.Texts()
	heroAt, sheetAt := -1, -1
	for index, value := range texts {
		if value == "探索未至之境" && heroAt < 0 {
			heroAt = index
		}
		if value == sheetTitle && sheetAt < 0 {
			sheetAt = index
		}
	}
	if heroAt < 0 || sheetAt < 0 || heroAt > sheetAt {
		t.Fatalf("绘制顺序不对（hero=%d sheet=%d）：%q", heroAt, sheetAt, texts)
	}

	path := filepath.Join(dir, "u2-sheet-headless.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	t.Logf("面板已画出：%s（居中 left=%d width=%d）", path, left, sheetWidth)

	// ④ 真按键路径：Esc 关闭（经过 OnKey），画面上就不该再有标题。
	if err := session.Keys("escape"); err != nil {
		t.Fatalf("Keys(escape): %v", err)
	}
	waitText(t, session, sheetTitle, false)
	t.Logf("Esc 后 texts=%q", session.Texts())
}

// waitText 等画面上的文字出现/消失。
//
// headless 是拉模式的：Invalidate 只把这一帧记成"待画"，真正的重绘发生在
// WaitFor 的 Pump 里。所以这里不能自己 sleep 轮询 HasText（那样永远读到旧帧），
// 必须走 WaitFor。注意 Snapshot.Texts 是"这一帧提交过的文字"（裁剪之前），
// 所以它证明"画了"，是否落在可见区域内由下面的像素断言补。
func waitText(t *testing.T, session *headless.Session, value string, want bool) {
	t.Helper()
	err := session.WaitFor(func(snapshot app.Snapshot) bool {
		found := false
		for _, text := range snapshot.Texts {
			if text.Value == value {
				found = true
				break
			}
		}
		return found == want
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("等不到 %q 出现=%v：texts=%q", value, want, session.Texts())
	}
}

func assertColor(t *testing.T, picture image.Image, x, y int, want gfx.Color, where string) {
	t.Helper()
	r, g, b, _ := picture.At(x, y).RGBA()
	wr, wg, wb := uint32(want.Red*255+0.5), uint32(want.Green*255+0.5), uint32(want.Blue*255+0.5)
	if !near(r>>8, wr) || !near(g>>8, wg) || !near(b>>8, wb) {
		t.Fatalf("%s (%d,%d) 颜色 = #%02x%02x%02x，期望 #%02x%02x%02x",
			where, x, y, r>>8, g>>8, b>>8, wr, wg, wb)
	}
}

func assertNotColor(t *testing.T, picture image.Image, x, y int, unwanted gfx.Color, where string) {
	t.Helper()
	r, g, b, _ := picture.At(x, y).RGBA()
	wr, wg, wb := uint32(unwanted.Red*255+0.5), uint32(unwanted.Green*255+0.5), uint32(unwanted.Blue*255+0.5)
	if near(r>>8, wr) && near(g>>8, wg) && near(b>>8, wb) {
		t.Fatalf("%s (%d,%d) 是面板底色 #%02x%02x%02x（说明面板的位置/宽度算错了）",
			where, x, y, wr, wg, wb)
	}
}

// near 允许 ±2 的量化误差：gfx.Color 是 float32，画到 8 位像素会有舍入。
func near(value, want uint32) bool {
	d := int(value) - int(want)
	return d >= -2 && d <= 2
}
