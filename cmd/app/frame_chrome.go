package main

import (
	"fmt"

	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const (
	frameCaptionHeight = uikernel.CaptionHeight // 官方 --dsh-windows-titlebar-height，唯一出处 internal/uikernel
	frameMenuHeight    = float32(28)
	frameMenuFontSize  = float32(14)
	frameMenuGap       = float32(2)
	frameMenuPadX      = float32(10)
	frameMenuRadius    = float32(6)
	frameMenuStart     = float32(48)
	frameMenuLayer     = uint16(4096)
)

var (
	chromeFeature          = tree.Feature("frame-chrome")
	chromeApplicationID    = chromeFeature.ID(tree.Name("application"))
	chromeEditID           = chromeFeature.ID(tree.Name("edit"))
	chromeScopeID          = chromeFeature.ID(tree.Name("menu-scope"))
	chromeDismissID        = chromeFeature.ID(tree.Name("menu-dismiss"))
	chromeNewSessionID     = chromeFeature.ID(tree.Name("new-session"))
	chromeToggleRightbarID = chromeFeature.ID(tree.Name("toggle-rightbar"))
	chromeUndoID           = chromeFeature.ID(tree.Name("undo"))
	chromeRedoID           = chromeFeature.ID(tree.Name("redo"))
	chromeCutID            = chromeFeature.ID(tree.Name("cut"))
	chromeCopyID           = chromeFeature.ID(tree.Name("copy"))
	chromePasteID          = chromeFeature.ID(tree.Name("paste"))
	chromeDeleteID         = chromeFeature.ID(tree.Name("delete"))
	chromeSelectAllID      = chromeFeature.ID(tree.Name("select-all"))
)

type frameChromeLayout struct {
	Caption     ui.Rect
	Content     ui.Rect
	Application ui.Rect
	Edit        ui.Rect
}

// layoutFrameChrome 把 caption 留在 Windows 客户区内，并只从三栏内容高度扣一次。
// appTextWidth/editTextWidth 是 14px 字号的实际文本宽度，两侧各加 10px。
func layoutFrameChrome(size ui.Size, enabled bool, trailing, appTextWidth, editTextWidth float32) frameChromeLayout {
	layout := frameChromeLayout{Content: ui.Rect{Width: size.Width, Height: size.Height}}
	if !enabled {
		return layout
	}
	layout.Caption = ui.Rect{Width: size.Width, Height: frameCaptionHeight}
	layout.Content = ui.Rect{Y: frameCaptionHeight, Width: size.Width, Height: max(size.Height-frameCaptionHeight, 0)}
	appWidth := appTextWidth + 2*frameMenuPadX
	editWidth := editTextWidth + 2*frameMenuPadX
	y := (frameCaptionHeight - frameMenuHeight) / 2
	layout.Application = ui.Rect{X: frameMenuStart, Y: y, Width: appWidth, Height: frameMenuHeight}
	layout.Edit = ui.Rect{X: frameMenuStart + appWidth + frameMenuGap, Y: y, Width: editWidth, Height: frameMenuHeight}
	// trailing 是 Viva 窗口控制占用的宽度。极窄窗口不将菜单画到控制按钮下面。
	available := size.Width - trailing - frameMenuStart
	if available < 0 {
		available = 0
	}
	if layout.Application.Width > available {
		layout.Application.Width = available
	}
	available -= layout.Application.Width + frameMenuGap
	if available < 0 {
		available = 0
	}
	if layout.Edit.Width > available {
		layout.Edit.Width = available
	}
	if layout.Application.Width == 0 {
		layout.Edit.X = frameMenuStart
		layout.Edit.Width = 0
	}
	return layout
}

// configureFrameWindow 使用 Viva 已经实现的 Windows 客户区 caption 与真实窗口动作。
func configureFrameWindow(cfg *app.Config, enabled bool, theme ui.Theme) {
	if !enabled {
		cfg.WindowMode = app.WindowTitled
		cfg.TitlebarHeight = 0
		cfg.WindowControls = nil
		return
	}
	cfg.WindowMode = app.WindowCustomTitlebar
	cfg.TitlebarHeight = frameCaptionHeight
	cfg.ClearColor = app.Color{
		Red: float64(theme.Background.Red), Green: float64(theme.Background.Green),
		Blue: float64(theme.Background.Blue), Alpha: float64(theme.Background.Alpha),
	}
	cfg.WindowControls = func() app.WindowControlsStyle {
		return app.WindowControlsStyle{
			Foreground: theme.Foreground, Hover: theme.Muted, Pressed: theme.Accent,
			Minimize: "最小化", Maximize: "最大化", Restore: "还原", CloseName: "关闭",
		}
	}
}

// frameSelection 保存 selection 的方向；只保存 ordered range 会把反向选择的 caret 放错端。
type frameSelection struct {
	Anchor int
	Caret  int
}

func captureFrameSelection(editor *text.Editor) frameSelection {
	ordered := editor.Selection()
	anchor := ordered.Start
	if ordered.Start != ordered.End && editor.Caret() == ordered.Start {
		anchor = ordered.End
	}
	return frameSelection{Anchor: anchor, Caret: editor.Caret()}
}

func (selection frameSelection) Ordered() text.Range {
	if selection.Anchor <= selection.Caret {
		return text.Range{Start: selection.Anchor, End: selection.Caret}
	}
	return text.Range{Start: selection.Caret, End: selection.Anchor}
}

func (selection frameSelection) restore(editor *text.Editor) {
	editor.SetSelection(text.Range{Start: selection.Anchor, End: selection.Caret})
}

type frameEditorSnapshot struct {
	id        ui.ID
	editor    *text.Editor
	selection frameSelection
	secure    bool
	readOnly  bool
}

type frameMenuEntry struct {
	item      components.MenubarItem
	separator bool
}

// frameChrome 是壳自己的 Windows 菜单。它不经过插件 SPI，也不伪造壳尚不具备的动作。
type frameChrome struct {
	open        ui.ID
	last        frameEditorSnapshot
	saved       frameEditorSnapshot
	menuScroll  components.MenuScroll
	lastLayout  frameChromeLayout
	restoreNext bool
	restoreSel  bool

	newSession     func()
	toggleRightbar func()
	rightbarOpen   bool
	readClipboard  func() (string, error)
	writeClipboard func(string) error
}

// wrapTextInput 观察 Viva 真正交给原生文本系统的 editor。菜单编辑的是同一个实例，
// 而不是另建一个 flex.Field 或把文本复制到临时变量里假装动作完成。
func (chrome *frameChrome) wrapTextInput(base app.TextInputProvider) app.TextInputProvider {
	return func(context app.TextInputContext) *app.TextInput {
		if base == nil {
			return nil
		}
		input := base(context)
		if input == nil || input.Editor == nil || context.ID.IsZero() {
			return input
		}
		chrome.last = frameEditorSnapshot{
			id: context.ID, editor: input.Editor, selection: captureFrameSelection(input.Editor),
			secure: context.Secure, readOnly: context.ReadOnly,
		}
		return input
	}
}

func (chrome *frameChrome) openMenu(id ui.ID) {
	if id.IsZero() {
		return
	}
	if chrome.open == id {
		chrome.closeMenu(true)
		return
	}
	if chrome.open.IsZero() {
		chrome.saved = chrome.last
	}
	chrome.open = id
	chrome.menuScroll = components.MenuScroll{}
	chrome.restoreNext = false
}

// closeMenu 在下一帧恢复真实输入焦点。dismiss 恢复原 selection；编辑动作保留动作后的 selection。
func (chrome *frameChrome) closeMenu(restoreSelection bool) {
	if chrome.open.IsZero() {
		return
	}
	chrome.open = ui.ID{}
	chrome.menuScroll = components.MenuScroll{}
	chrome.restoreNext = true
	chrome.restoreSel = restoreSelection
}

func (chrome *frameChrome) handleClick(id ui.ID) error {
	switch id {
	case chromeApplicationID, chromeEditID:
		chrome.openMenu(id)
		return nil
	case chromeDismissID:
		chrome.closeMenu(true)
		return nil
	case chromeNewSessionID:
		if chrome.newSession != nil {
			chrome.newSession()
		}
		chrome.closeMenu(true)
		return nil
	case chromeToggleRightbarID:
		if chrome.toggleRightbar != nil {
			chrome.toggleRightbar()
		}
		chrome.closeMenu(true)
		return nil
	case chromeUndoID, chromeRedoID, chromeCutID, chromeCopyID, chromePasteID, chromeDeleteID, chromeSelectAllID:
		err := chrome.executeEdit(id)
		chrome.closeMenu(false)
		return err
	default:
		return nil
	}
}

func (chrome *frameChrome) handleKey(event ui.KeyEvent) ui.EventResult {
	if event.Release {
		return ui.EventResult{}
	}
	if event.Key == ui.KeyEscape && !chrome.open.IsZero() {
		chrome.closeMenu(true)
		return ui.EventResult{Handled: true, Repaint: true}
	}
	triggers := []components.MenubarTrigger{{ID: chromeApplicationID, Label: "应用"}, {ID: chromeEditID, Label: "编辑"}}
	if event.Target == chromeApplicationID || event.Target == chromeEditID {
		before := chrome.open
		state, focus, ok := components.MenubarKeyDown(event.Target, triggers, components.MenubarState(chrome.open), components.LTR, event.Key)
		if ok {
			next := ui.ID(state)
			if next != before {
				if next.IsZero() {
					chrome.closeMenu(true)
				} else {
					chrome.openMenu(next)
				}
				if !next.IsZero() {
					focus = chrome.firstEnabledItem(chrome.menuEntries())
				}
			}
			return ui.EventResult{Handled: true, Repaint: true, Focus: focus, FocusVisible: true}
		}
	}
	if !chrome.open.IsZero() {
		if target, ok := chrome.menuScroll.Navigate(event.Key, event.Target, true); ok {
			return ui.EventResult{Handled: true, Repaint: true, Focus: target, FocusVisible: true}
		}
	}
	return ui.EventResult{}
}

func (chrome *frameChrome) paintCaption(canvas *ui.Canvas, theme ui.Theme, size ui.Size) frameChromeLayout {
	if chrome.restoreNext {
		if chrome.saved.editor != nil && !chrome.saved.id.IsZero() {
			if chrome.restoreSel {
				chrome.saved.selection.restore(chrome.saved.editor)
			}
			canvas.RequestFocus(chrome.saved.id)
		}
		chrome.restoreNext = false
	}
	_, trailing := app.TitlebarInsets()
	layout := layoutFrameChrome(size, true, trailing,
		canvas.TextWidthWeight("应用", frameMenuFontSize, ui.WeightRegular),
		canvas.TextWidthWeight("编辑", frameMenuFontSize, ui.WeightRegular))
	chrome.lastLayout = layout
	canvas.MoveRegion(layout.Caption)
	canvas.Fill(layout.Caption, theme.Background)
	menubar := ui.Rect{X: frameMenuStart, Width: max(size.Width-trailing-frameMenuStart, 0), Height: frameCaptionHeight}
	canvas.Container(menubar, ui.RoleMenubar, "菜单栏")
	if !chrome.open.IsZero() {
		// popup 的 backdrop 不能盖住另一枚菜单按钮；两枚触发器与菜单同属一个 scope，
		// 才能在菜单已打开时真实切换，而不是先外点关闭。
		canvas.PushLayer(frameMenuLayer + 1)
		canvas.PushFocusScope(chromeScopeID)
	}
	chrome.paintTrigger(canvas, theme, chromeApplicationID, "应用", layout.Application)
	chrome.paintTrigger(canvas, theme, chromeEditID, "编辑", layout.Edit)
	if !chrome.open.IsZero() {
		canvas.PopFocusScope()
		canvas.PopLayer()
	}
	return layout
}

func (chrome *frameChrome) paintTrigger(canvas *ui.Canvas, theme ui.Theme, id ui.ID, label string, bounds ui.Rect) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	open := chrome.open == id
	active := open || canvas.Hovered() == id || canvas.Pressed() == id || canvas.FocusVisible() == id
	foreground := theme.MutedForeground
	if active {
		canvas.Rounded(bounds, frameMenuRadius, theme.Muted)
		foreground = theme.Foreground
	}
	canvas.LabelCenter(label, bounds, ui.TextStyle{Size: frameMenuFontSize, Weight: ui.WeightRegular, Color: foreground})
	canvas.HitWithOptions(id, bounds, label, ui.HitOptions{
		Role: ui.RoleMenuItem, Selected: open, Expanded: open, Expandable: true,
		// 与官方 pointerdown.preventDefault 一致：点击菜单不抢走编辑器的键盘焦点。
		SkipFocus: true,
	})
}

func (chrome *frameChrome) paintPopup(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	if chrome.open.IsZero() {
		return
	}
	anchor := chrome.lastLayout.Application
	if chrome.open == chromeEditID {
		anchor = chrome.lastLayout.Edit
	}
	entries := chrome.menuEntries()
	height := float32(8)
	for _, entry := range entries {
		if entry.separator {
			height += 9
		} else {
			height += 32
		}
	}
	menu := components.BeginMenubarContent(canvas, theme,
		ui.Rect{X: anchor.X, Y: anchor.Y + anchor.Height, Width: 232, Height: height},
		components.MenubarOverlayOptions{
			Layer: frameMenuLayer, Scope: chromeScopeID, DismissID: chromeDismissID, ID: chrome.open,
			Viewport: ui.Rect{Width: size.Width, Height: size.Height}, Scroll: &chrome.menuScroll,
		})
	if first := chrome.firstEnabledItem(entries); !first.IsZero() {
		canvas.SetInitialFocus(first)
	}
	for _, entry := range entries {
		if entry.separator {
			menu.AddSeparator()
		} else {
			menu.AddItem(entry.item)
		}
	}
	menu.End()
}

func (chrome *frameChrome) firstEnabledItem(entries []frameMenuEntry) ui.ID {
	for _, entry := range entries {
		if !entry.separator && !entry.item.Disabled && !entry.item.ID.IsZero() {
			return entry.item.ID
		}
	}
	return ui.ID{}
}

func (chrome *frameChrome) menuEntries() []frameMenuEntry {
	if chrome.open == chromeApplicationID {
		rightbarLabel := "显示右栏"
		if chrome.rightbarOpen {
			rightbarLabel = "隐藏右栏"
		}
		return []frameMenuEntry{
			{item: components.MenubarItem{ID: chromeNewSessionID, Label: "新建会话"}},
			{item: components.MenubarItem{ID: chromeToggleRightbarID, Label: rightbarLabel}},
		}
	}
	editor := chrome.saved.editor
	if chrome.open.IsZero() {
		editor = chrome.last.editor
	}
	hasEditor := editor != nil
	readOnly := chrome.saved.readOnly
	secure := chrome.saved.secure
	selection := frameSelection{}
	canUndo, canRedo, atEnd := false, false, true
	if hasEditor {
		selection = captureFrameSelection(editor)
		canUndo, canRedo = editor.CanUndo(), editor.CanRedo()
		atEnd = editor.Caret() == editor.RuneCount()
	}
	hasSelection := selection.Ordered().Start != selection.Ordered().End
	mutable := hasEditor && !readOnly
	item := func(id ui.ID, label, shortcut string, enabled bool) frameMenuEntry {
		return frameMenuEntry{item: components.MenubarItem{ID: id, Label: label, Shortcut: shortcut, Disabled: !enabled}}
	}
	return []frameMenuEntry{
		item(chromeUndoID, "撤销", "Ctrl+Z", mutable && canUndo),
		item(chromeRedoID, "重做", "Ctrl+Y", mutable && canRedo),
		{separator: true},
		item(chromeCutID, "剪切", "Ctrl+X", mutable && !secure && hasSelection),
		item(chromeCopyID, "复制", "Ctrl+C", hasEditor && !secure && hasSelection),
		item(chromePasteID, "粘贴", "Ctrl+V", mutable),
		item(chromeDeleteID, "删除", "Delete", mutable && (hasSelection || !atEnd)),
		{separator: true},
		item(chromeSelectAllID, "全选", "Ctrl+A", hasEditor && editor.RuneCount() > 0),
	}
}

func (chrome *frameChrome) executeEdit(id ui.ID) error {
	saved := chrome.saved
	editor := saved.editor
	if editor == nil {
		return fmt.Errorf("没有可接收编辑命令的输入框")
	}
	mutable := !saved.readOnly
	selection := editor.Selection()
	selected := func() string {
		runes := []rune(editor.String())
		start := max(0, min(selection.Start, len(runes)))
		end := max(start, min(selection.End, len(runes)))
		return string(runes[start:end])
	}
	write := chrome.writeClipboard
	if write == nil {
		write = app.SetClipboardText
	}
	read := chrome.readClipboard
	if read == nil {
		read = app.ClipboardText
	}
	apply := func(event app.TextEvent) { event.Apply(editor) }

	switch id {
	case chromeUndoID:
		if !mutable || !editor.CanUndo() {
			return fmt.Errorf("当前不能撤销")
		}
		apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextUndo})
	case chromeRedoID:
		if !mutable || !editor.CanRedo() {
			return fmt.Errorf("当前不能重做")
		}
		apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextRedo})
	case chromeCopyID, chromeCutID:
		if saved.secure || selection.Start == selection.End || id == chromeCutID && !mutable {
			return fmt.Errorf("当前没有可复制或剪切的选择")
		}
		if err := write(selected()); err != nil {
			return err // 写剪贴板失败时绝不能先删正文。
		}
		if id == chromeCutID {
			apply(app.TextEvent{Kind: app.TextPaste})
		}
	case chromePasteID:
		if !mutable {
			return fmt.Errorf("当前输入框为只读")
		}
		value, err := read()
		if err != nil {
			return err
		}
		if value != "" {
			apply(app.TextEvent{Kind: app.TextPaste, Text: value})
		}
	case chromeDeleteID:
		if !mutable {
			return fmt.Errorf("当前输入框为只读")
		}
		apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextDeleteForward})
	case chromeSelectAllID:
		apply(app.TextEvent{Kind: app.TextCommandEvent, Command: app.TextSelectAll})
	default:
		return fmt.Errorf("未知编辑命令 %v", id)
	}
	return nil
}
