package main

import (
	"encoding/json"
	"os"

	"github.com/vivaui/viva/ui"
)

// frameChromeReport 来自真实原生窗口最后一次 Paint 的布局，而不是重算一份期望值。
type frameChromeReport struct {
	Size                ui.Size `json:"size"`
	CustomTitlebar      bool    `json:"customTitlebar"`
	NativeControls      bool    `json:"nativeControls"`
	Caption             ui.Rect `json:"caption"`
	Content             ui.Rect `json:"content"`
	Application         ui.Rect `json:"application"`
	Edit                ui.Rect `json:"edit"`
	MenuFontSize        float32 `json:"menuFontSize"`
	MenuGap             float32 `json:"menuGap"`
	MenuHorizontalPad   float32 `json:"menuHorizontalPad"`
	MenuRadius          float32 `json:"menuRadius"`
	ContentHeightPlus40 float32 `json:"contentHeightPlus40"`
}

func (m *model) writeFrameChromeReport(path string) error {
	if path == "" {
		return nil
	}
	m.mu.Lock()
	layout := m.chromeLayout
	m.mu.Unlock()
	report := frameChromeReport{
		Size:           ui.Size{Width: layout.Caption.Width, Height: layout.Content.Height + frameCaptionHeight},
		CustomTitlebar: true, NativeControls: true,
		Caption: layout.Caption, Content: layout.Content,
		Application: layout.Application, Edit: layout.Edit,
		MenuFontSize: frameMenuFontSize, MenuGap: frameMenuGap,
		MenuHorizontalPad: frameMenuPadX, MenuRadius: frameMenuRadius,
		ContentHeightPlus40: layout.Content.Height + frameCaptionHeight,
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
