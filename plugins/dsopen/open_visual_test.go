//go:build darwin || windows || linux

package dsopen

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// TestOpenButtonHeadlessCapture 给「打开」按钮留一帧真实像素。
//
// 这个部件的全部历史都是"看得见吗"：做出来之后连续三轮在截图里找不到
// （先怀疑槽位没画、再怀疑高度塌陷，最后确认是抓帧时机太早——
// CWD 要等 session.load 回来才填）。几何断言能证明"画在标题行、56×24"，
// 但证明不了它**被真的光栅化出来**（底色、圆角、文案）。所以这一帧不能省。
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
	// 命中矩形是布局值，可能带 0.5 的子像素；找一颗中心/尺寸都在 1px 内的实画矩形。
	drawn := false
	for _, primitive := range session.Snapshot().Primitives {
		b := primitive.Bounds
		if math.Abs(float64(b.Width-hit.Bounds.Width)) <= 1 && math.Abs(float64(b.Height-hit.Bounds.Height)) <= 1 &&
			math.Abs(float64(b.X-hit.Bounds.X)) <= 1 && math.Abs(float64(b.Y-hit.Bounds.Y)) <= 1 {
			drawn = true
			break
		}
	}
	if !drawn {
		t.Fatalf("按钮只有命中区、没有画出来的像素：hit=%+v", hit.Bounds)
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
