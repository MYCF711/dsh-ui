//go:build darwin || windows || linux

package dsopen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
)

// TestOpenButtonHeadlessCapture 给「打开」按钮留一帧真实像素。
//
// 这个部件的全部历史都是"看得见吗"：做出来之后连续三轮在截图里找不到
// （先怀疑槽位没画、再怀疑高度塌陷，最后确认是抓帧时机太早——
// CWD 要等 session.load 回来才填）。几何断言能证明"画在标题行、24×24"，
// 但证明不了它**被真的光栅化出来**（描边、圆角、官方矢量图标）。所以这一帧不能省。
func TestOpenButtonHeadlessCapture(t *testing.T) {
	dir := os.Getenv("OPEN_CAPTURE_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := openHeaderData(`D:\ws\项目`)
	session := openHeaderSession(t, &data, pluginapi.Actions{
		OpenFile:       func(string) error { return nil },
		ToggleRightbar: func() {},
	}, 1182, 832)
	defer session.Close()

	hit, ok := session.Hit("在工作目录里打开")
	if !ok {
		t.Fatalf("真实帧里找不到「打开」按钮：texts=%q", session.Texts())
	}
	// 官方透明底 + 描边在 Viva 里是矢量三角形，不是填充矩形。分别找落在
	// 24×24 命中区内的 BorderL4 描边和 LabelPrimary right-up 图标，避免拿
	// 一颗透明矩形或别的头部图元冒充“按钮已实画”。
	theme := dscore.Dark()
	borderDrawn, iconDrawn := false, false
	for _, primitive := range session.Snapshot().Primitives {
		if primitive.Kind != app.TrianglePrimitive {
			continue
		}
		inside := false
		for _, point := range primitive.Points {
			if point.X >= hit.Bounds.X-1 && point.X <= hit.Bounds.X+hit.Bounds.Width+1 &&
				point.Y >= hit.Bounds.Y-1 && point.Y <= hit.Bounds.Y+hit.Bounds.Height+1 {
				inside = true
				break
			}
		}
		if !inside {
			continue
		}
		switch primitive.Color {
		case theme.BorderL4:
			borderDrawn = true
		case theme.LabelPrimary:
			iconDrawn = true
		}
	}
	if !borderDrawn || !iconDrawn {
		t.Fatalf("按钮实画矢量缺项：BorderL4=%v right-up=%v hit=%+v", borderDrawn, iconDrawn, hit.Bounds)
	}
	path := filepath.Join(dir, "open-in-app-plugin-1182x832.png")
	if err := session.Screenshot(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		t.Fatalf("截图无效 path=%s info=%v err=%v", path, info, err)
	}
	t.Logf("OPEN_VISUAL path=%s bytes=%d button=%+v", path, info.Size(), hit.Bounds)
}
