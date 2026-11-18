package dsassets

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestPluginArtworkTerminal 钉住官方 `PluginArtworkTerminal` 的几何：
// 36×36 viewBox、stroke 3.5、两条 path（`dsh-web-frontend/dist/assets/
// index-ghOpC9uV.js` 的 xK 组件）。
func TestPluginArtworkTerminal(t *testing.T) {
	icon, ok := PluginArtworkTerminal()
	if !ok {
		t.Fatal("PluginArtworkTerminal 解析失败")
	}
	if icon.ViewBox != 36 || icon.Stroke != 3.5 {
		t.Fatalf("几何 = viewBox %v / stroke %v，want 36 / 3.5", icon.ViewBox, icon.Stroke)
	}
	if len(icon.Commands) == 0 {
		t.Fatal("两条 path 没有编译出命令")
	}
	if got := TerminalArtworkStroke(); got != gfx.RGB24(0x679efe) {
		t.Fatalf("stroke 色 = %v，want #679EFE", got)
	}
}
