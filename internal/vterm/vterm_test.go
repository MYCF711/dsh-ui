package vterm

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/ansi"
)

// TestScreenDiffersFromStrippedStream 钉住这个包存在的**理由**：
// 「当前屏幕」和「剥掉控制序列后按写入顺序拼起来」不是一回事。
//
// 用 `\r` 原地重画演示：真实终端上只应该看到最后那一帧，而剥序列的文本里
// 所有中间帧都留着。agent 读到前者才知道现在是什么状态。
func TestScreenDiffersFromStrippedStream(t *testing.T) {
	e := New(40, 5)

	// 下载进度条：每一帧都用 \r 回车重画同一行。
	raw := "progress  0%\rprogress 50%\rprogress 100%\ndone\n"
	if _, err := e.Write([]byte(raw)); err != nil {
		t.Fatalf("写入失败：%v", err)
	}

	screen := e.Lines()
	if len(screen) < 2 {
		t.Fatalf("屏幕行数不对：%q", screen)
	}
	if got := strings.TrimSpace(screen[0]); got != "progress 100%" {
		t.Fatalf("第一行应当是最后一帧 %q，实际 %q", "progress 100%", got)
	}
	if got := strings.TrimSpace(screen[1]); got != "done" {
		t.Fatalf("第二行应当是 %q，实际 %q", "done", got)
	}

	// 对照组：剥序列那条路会把三帧全留着（这正是我们要摆脱的行为）。
	stripped := ansi.Strip(raw)
	if !strings.Contains(stripped, "progress  0%") {
		t.Fatalf("对照组前提不成立，剥序列后应当仍含第一帧：%q", stripped)
	}
	if strings.Contains(strings.Join(screen, "\n"), "progress  0%") {
		t.Fatal("屏幕里不该出现已被重画掉的中间帧")
	}
}

// TestCursorAddressingRendersAsScreen 钉住「靠光标定位绘制」这条路径。
//
// `vim` / `htop` 这类程序不按行顺序写，而是先跳到某一行某一列再写。
// 剥序列的文本拿不到这个结果，屏幕模型可以。
func TestCursorAddressingRendersAsScreen(t *testing.T) {
	e := New(20, 4)

	// 先写满四行，再用 CSI 跳回第 2 行第 3 列覆盖两个字符。
	raw := "aaaaaaaaaa\r\nbbbbbbbbbb\r\ncccccccccc\r\ndddddddddd" +
		"\x1b[2;3Hxy"
	if _, err := e.Write([]byte(raw)); err != nil {
		t.Fatalf("写入失败：%v", err)
	}

	lines := e.Lines()
	if len(lines) < 2 {
		t.Fatalf("屏幕行数不对：%q", lines)
	}
	if got := strings.TrimRight(lines[1], " "); got != "bbxybbbbbb" {
		t.Fatalf("第 2 行应是光标定位后的 %q，实际 %q", "bbxybbbbbb", got)
	}
	// 其它行不受影响。
	if got := strings.TrimRight(lines[0], " "); got != "aaaaaaaaaa" {
		t.Fatalf("第 1 行被误改：%q", got)
	}
}

// TestCJKIsIntactAcrossWrites 钉住**我们真实会遇到的场景**：中文路径/文件名。
//
// 上游 charmbracelet/x#1039 是「grapheme cluster 跨 Write 被拆坏」，实测
// CJK 与区域指示符旗帜**不受影响**；这里把它钉住，防止将来升级后回归。
func TestCJKIsIntactAcrossWrites(t *testing.T) {
	const s = "a 中文 b"
	whole := New(40, 2)
	if _, err := whole.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(whole.Lines()[0])

	// 逐字节切开喂（pty 的读边界本来就不保证）。
	split := New(40, 2)
	for i := 0; i < len(s); i++ {
		if _, err := split.Write([]byte{s[i]}); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.TrimSpace(split.Lines()[0])

	if got != want {
		t.Fatalf("中文跨 Write 被拆坏：整段=%q 逐字节=%q", want, got)
	}
}

// TestResizeKeepsContent 钉住 resize 不丢已有内容。
func TestResizeKeepsContent(t *testing.T) {
	e := New(20, 3)
	if _, err := e.Write([]byte("hello\r\nworld")); err != nil {
		t.Fatal(err)
	}
	if err := e.Resize(40, 6); err != nil {
		t.Fatalf("resize 失败：%v", err)
	}
	w, h := e.Size()
	if w != 40 || h != 6 {
		t.Fatalf("尺寸没变：%dx%d", w, h)
	}
	joined := strings.Join(e.Lines(), "\n")
	if !strings.Contains(joined, "hello") || !strings.Contains(joined, "world") {
		t.Fatalf("resize 后内容丢了：%q", joined)
	}
}
