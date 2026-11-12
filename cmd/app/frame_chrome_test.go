package main

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestFrameChromeLayoutUsesFortyPixelClientCaption(t *testing.T) {
	for _, size := range []ui.Size{{Width: 1920, Height: 1032}, {Width: 1182, Height: 832}} {
		layout := layoutFrameChrome(size, true, 3*app.WindowControlWidth, 28, 28)
		if layout.Caption != (ui.Rect{Width: size.Width, Height: 40}) {
			t.Fatalf("%.0fx%.0f caption = %+v，期望从客户区 (0,0) 起高 40", size.Width, size.Height, layout.Caption)
		}
		wantContent := ui.Rect{Y: 40, Width: size.Width, Height: size.Height - 40}
		if layout.Content != wantContent {
			t.Fatalf("%.0fx%.0f content = %+v，期望 %+v；三栏高度只能扣一次 caption", size.Width, size.Height, layout.Content, wantContent)
		}
		if layout.Application != (ui.Rect{X: 48, Y: 6, Width: 48, Height: 28}) {
			t.Fatalf("应用按钮 = %+v，期望 x48/y6/48x28（14px 字 + 两侧 10px）", layout.Application)
		}
		if layout.Edit.X-layout.Application.X-layout.Application.Width != 2 || layout.Edit.Y != 6 || layout.Edit.Height != 28 || layout.Edit.Width != 48 {
			t.Fatalf("编辑按钮 = %+v，应用按钮 = %+v；menubar gap 必须为 2", layout.Edit, layout.Application)
		}
	}

	disabled := layoutFrameChrome(ui.Size{Width: 1182, Height: 832}, false, 0, 28, 28)
	if disabled.Caption.Height != 0 || disabled.Content.Height != 832 || disabled.Content.Y != 0 {
		t.Fatalf("非 Windows 不应重复造 caption：%+v", disabled)
	}
}

func TestConfigureFrameWindowUsesVivaNativeControls(t *testing.T) {
	cfg := app.Config{}
	configureFrameWindow(&cfg, true, ui.DarkTheme())
	if cfg.WindowMode != app.WindowCustomTitlebar || cfg.TitlebarHeight != 40 {
		t.Fatalf("WindowMode/TitlebarHeight = %v/%v，期望 custom/40", cfg.WindowMode, cfg.TitlebarHeight)
	}
	if cfg.WindowControls == nil {
		t.Fatal("Windows custom caption 没有挂接 Viva 的真实窗口控制")
	}
	style := cfg.WindowControls()
	if style.Minimize == "" || style.Maximize == "" || style.Restore == "" || style.CloseName == "" {
		t.Fatalf("窗口控制缺少可访问名称：%+v", style)
	}
	configureFrameWindow(&cfg, false, ui.DarkTheme())
	if cfg.WindowMode != app.WindowTitled || cfg.TitlebarHeight != 0 || cfg.WindowControls != nil {
		t.Fatalf("禁用客户区 Chrome 后仍残留配置：mode=%v height=%v controls=%v", cfg.WindowMode, cfg.TitlebarHeight, cfg.WindowControls != nil)
	}
}

type frameChromeHarness struct {
	chrome      frameChrome
	field       ui.Field
	view        *ui.Page
	inv         app.Invalidator
	newSessions int
	toggles     int
	clickErr    error
}

func newFrameChromeHarness(t *testing.T) (*frameChromeHarness, *app.Driver) {
	t.Helper()
	h := &frameChromeHarness{}
	h.chrome.newSession = func() { h.newSessions++ }
	h.chrome.toggleRightbar = func() { h.toggles++ }
	theme := ui.DarkTheme()
	// v0.6.0：主题是构造期给的（Draw 不再收 theme）。
	h.view = ui.NewPage(ui.PageConfig{Theme: &theme})
	cfg := app.Config{Tree: &tree, Title: "dsh-ui-frame-test", Width: 1182, Height: 832, Invalidator: &h.inv}
	configureFrameWindow(&cfg, true, theme)
	cfg.TextInput = h.chrome.wrapTextInput(ui.Fields(&h.field))
	cfg.Paint = func(canvas *gfx.Canvas, size ui.Size) {
		if id, ok := canvas.ClickTarget(); ok {
			h.clickErr = h.chrome.handleClick(id)
		}
		canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, theme.Background)
		layout := h.chrome.paintCaption(canvas, theme, size)
		h.view.Draw(canvas, layout.Content, func(f ui.C) {
			ui.Column(f).Fill().Children(func() {
				ui.TextFieldID(f, draftID, &h.field, "输入", "输入消息").Height(32)
			})
		})
		h.chrome.paintPopup(canvas, theme, size)
	}
	cfg.OnKey = h.chrome.handleKey
	driver, err := app.NewDriver(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return h, driver
}

func frameHit(t *testing.T, driver *app.Driver, id gfx.ID) ui.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds
		}
	}
	t.Fatalf("当前帧没有 hit %v；hits=%+v", id, driver.Snapshot().Hits)
	return ui.Rect{}
}

func clickFrameID(t *testing.T, driver *app.Driver, id gfx.ID) {
	t.Helper()
	bounds := frameHit(t, driver, id)
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatalf("点击 %v: %v", id, err)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
}

func focusFrameDraft(t *testing.T, driver *app.Driver) {
	t.Helper()
	clickFrameID(t, driver, draftID)
}

func TestFrameChromeCaptionDragAndMenuControls(t *testing.T) {
	h, driver := newFrameChromeHarness(t)
	_ = h
	if got := driver.PressAt(12, 20); got != app.PressMove {
		t.Fatalf("空白 caption PressAt = %v，期望 PressMove", got)
	}
	for _, id := range []gfx.ID{chromeApplicationID, chromeEditID} {
		bounds := frameHit(t, driver, id)
		if got := driver.PressAt(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); got != app.PressControl {
			t.Fatalf("菜单 %v PressAt = %v，期望 PressControl，不能拖动窗口", id, got)
		}
	}
	clickFrameID(t, driver, chromeApplicationID)
	if h.chrome.open != chromeApplicationID {
		t.Fatal("点击应用按钮没有打开真实菜单")
	}
	clickFrameID(t, driver, chromeEditID)
	if h.chrome.open != chromeEditID {
		t.Fatal("菜单打开时点击另一按钮没有切换到编辑菜单")
	}
}

func TestFrameChromeMenuDismissRestoresEditorFocusAndSelection(t *testing.T) {
	for _, outside := range []bool{false, true} {
		t.Run(map[bool]string{false: "Escape", true: "外点"}[outside], func(t *testing.T) {
			h, driver := newFrameChromeHarness(t)
			focusFrameDraft(t, driver)
			if err := driver.Type("hello 世界"); err != nil {
				t.Fatal(err)
			}
			if err := driver.SelectText(1, 7); err != nil {
				t.Fatal(err)
			}
			before := h.chrome.last.selection
			clickFrameID(t, driver, chromeEditID)
			if !h.chrome.open.IsZero() && h.chrome.open == chromeEditID {
				// open 是 ui.ID；这个分支只让失败信息更直观。
			} else {
				t.Fatal("编辑菜单未打开")
			}
			if outside {
				if err := driver.Click(1100, 700); err != nil {
					t.Fatal(err)
				}
			} else if err := driver.Key(gfx.KeyEscape, 0); err != nil {
				t.Fatal(err)
			}
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if !h.chrome.open.IsZero() {
				t.Fatal("菜单关闭后 open 状态仍非零")
			}
			if got := driver.Snapshot().Focused; got != draftID {
				t.Fatalf("关闭菜单后焦点 = %v(%s)，期望原输入框 %v(%s)；saved=%v restore=%v", got, driver.Snapshot().Tree.Text(got), draftID, driver.Snapshot().Tree.Text(draftID), h.chrome.saved.id, h.chrome.restoreNext)
			}
			if got := h.fieldEditor(h.chrome).Selection(); got != before.Ordered() {
				t.Fatalf("关闭菜单后 selection = %+v，期望 %+v", got, before.Ordered())
			}
			if got := h.fieldEditor(h.chrome).Caret(); got != before.Caret {
				t.Fatalf("关闭菜单后 caret = %d，期望 %d", got, before.Caret)
			}
		})
	}
}

func TestFrameChromeApplicationMenuRunsOnlyRealActions(t *testing.T) {
	h, driver := newFrameChromeHarness(t)
	clickFrameID(t, driver, chromeApplicationID)
	for _, text := range driver.Snapshot().Texts {
		switch text.Value {
		case "关于", "检查更新", "CLI 命令", "退出确认":
			t.Fatalf("没有真实 handler 的菜单项被伪造：%q", text.Value)
		}
	}
	clickFrameID(t, driver, chromeNewSessionID)
	if h.newSessions != 1 || h.toggles != 0 {
		t.Fatalf("新建会话回调 = %d，右栏回调 = %d；期望 1/0", h.newSessions, h.toggles)
	}
	clickFrameID(t, driver, chromeApplicationID)
	clickFrameID(t, driver, chromeToggleRightbarID)
	if h.newSessions != 1 || h.toggles != 1 {
		t.Fatalf("应用菜单动作计数 = %d/%d；期望各一次且不重复", h.newSessions, h.toggles)
	}
}

func TestFrameChromeEditMenuUsesRealSelectionAndClipboard(t *testing.T) {
	t.Run("复制保持正文并写真实剪贴板", func(t *testing.T) {
		h, driver := newFrameChromeHarness(t)
		focusFrameDraft(t, driver)
		if err := driver.Type("alpha 世界"); err != nil {
			t.Fatal(err)
		}
		if err := driver.SelectText(0, 5); err != nil {
			t.Fatal(err)
		}
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeCopyID)
		if got := driver.Clipboard(); got != "alpha" {
			t.Fatalf("clipboard = %q，期望 alpha", got)
		}
		if got := h.field.Text(); got != "alpha 世界" {
			t.Fatalf("复制修改了正文：%q", got)
		}
		if got := driver.Snapshot().Focused; got != draftID {
			t.Fatalf("复制后焦点 = %v，期望编辑器", got)
		}
	})

	t.Run("剪切写剪贴板并删除所选", func(t *testing.T) {
		h, driver := newFrameChromeHarness(t)
		focusFrameDraft(t, driver)
		_ = driver.Type("alpha 世界")
		_ = driver.SelectText(0, 5)
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeCutID)
		if driver.Clipboard() != "alpha" || h.field.Text() != " 世界" {
			t.Fatalf("剪切后 clipboard/text = %q/%q", driver.Clipboard(), h.field.Text())
		}
	})

	t.Run("粘贴读取真实剪贴板并替换所选", func(t *testing.T) {
		h, driver := newFrameChromeHarness(t)
		focusFrameDraft(t, driver)
		_ = driver.Type("abc")
		_ = driver.SelectText(1, 2)
		driver.SetClipboard("粘贴")
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromePasteID)
		if got := h.field.Text(); got != "a粘贴c" {
			t.Fatalf("粘贴后正文 = %q，期望 a粘贴c", got)
		}
	})

	t.Run("删除全选撤销重做都是编辑器动作", func(t *testing.T) {
		h, driver := newFrameChromeHarness(t)
		focusFrameDraft(t, driver)
		_ = driver.Type("abc")
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeSelectAllID)
		if got := h.fieldEditor(h.chrome).Selection(); got != (text.Range{Start: 0, End: 3}) {
			t.Fatalf("全选 selection = %+v", got)
		}
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeDeleteID)
		if got := h.field.Text(); got != "" {
			t.Fatalf("删除所选后正文 = %q", got)
		}
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeUndoID)
		if got := h.field.Text(); got != "abc" {
			t.Fatalf("撤销后正文 = %q，期望 abc", got)
		}
		clickFrameID(t, driver, chromeEditID)
		clickFrameID(t, driver, chromeRedoID)
		if got := h.field.Text(); got != "" {
			t.Fatalf("重做后正文 = %q，期望空", got)
		}
	})
}

func TestFrameChromeClipboardFailureDoesNotMutateEditor(t *testing.T) {
	h, driver := newFrameChromeHarness(t)
	focusFrameDraft(t, driver)
	_ = driver.Type("keep")
	_ = driver.SelectText(0, 4)
	h.chrome.writeClipboard = func(string) error { return errors.New("clipboard busy") }
	clickFrameID(t, driver, chromeEditID)
	clickFrameID(t, driver, chromeCutID)
	if h.clickErr == nil || h.field.Text() != "keep" || driver.Clipboard() != "" {
		t.Fatalf("剪贴板失败后 err/text/clip = %v/%q/%q；正文不得删除", h.clickErr, h.field.Text(), driver.Clipboard())
	}
}

// fieldEditor 只从 Chrome 捕获的真实 TextInputProvider 取编辑器，绝不另建替身。
func (h *frameChromeHarness) fieldEditor(chrome frameChrome) *text.Editor {
	if chrome.last.editor == nil {
		return nil
	}
	return chrome.last.editor
}
