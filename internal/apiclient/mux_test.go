// mux_test.go —— 自测：真起一个 mockd，用 Mux 订阅，验证 ev / gap / 连续性。
//
// 这是端到端自测（不是 mock 掉网络）：cmd/mockd 的行为就是契约的参考实现。
//
//	go test ./internal/apiclient -run TestMuxLive -v
package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// startMockd 编译并拉起 cmd/mockd，返回 base URL；测试结束自动收尸。
func startMockd(t *testing.T) string {
	t.Helper()

	name := "mockd"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)

	build := exec.Command("go", "build", "-o", bin, "./cmd/mockd")
	build.Dir = "../.." // 模块根
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("编译 mockd 失败：%v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	cmd.Stdout = os.Stderr // mockd 的日志别混进我们的输出
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 mockd 失败：%v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	base := "http://127.0.0.1:7788"
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			resp.Body.Close()
			return base
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("mockd 10 秒内没起来（%s）", base)
	return ""
}

func TestMuxLive(t *testing.T) {
	base := startMockd(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	m := NewMux(base)
	m.PingEvery = 500 * time.Millisecond // 测试里把心跳调密，顺便验证 ping/pong 不炸

	const sinceSeq = 100
	ch, err := m.Subscribe(ctx, "session:s-001", sinceSeq)
	if err != nil {
		t.Fatalf("Subscribe 失败：%v", err)
	}

	var (
		evs      int
		gaps     int
		lastSeq  = sinceSeq
		gapFrame *GapInfo
	)
	fmt.Printf("\n--- 订阅 session:s-001  sinceSeq=%d ---\n", sinceSeq)

	// 读够 12 条或超时；mockd 每 200ms 一条，第 8 条前会给 gap。
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("超时：只收到 %d 条 ev、%d 个 gap（需要 ≥5 ev + ≥1 gap）", evs, gaps)
		case e, ok := <-ch:
			if !ok {
				t.Fatalf("channel 提前关闭：收到 %d 条 ev、%d 个 gap", evs, gaps)
			}
			switch e.Kind {
			case "gap":
				gaps++
				gapFrame = e.Gap
				fmt.Printf("  GAP  from=%d to=%d reason=%q\n", e.Gap.FromSeq, e.Gap.ToSeq, e.Gap.Reason)
				if e.Gap.ToSeq > lastSeq {
					lastSeq = e.Gap.ToSeq
				}
			default:
				evs++
				if e.Seq != lastSeq+1 {
					t.Errorf("客户端收到了不连续的 seq：期望 %d，收到 %d（gap 兜底没生效）", lastSeq+1, e.Seq)
				}
				lastSeq = e.Seq
				fmt.Printf("  EV   seq=%-3d kind=%-18s data=%s\n", e.Seq, e.Kind, string(e.Data))
			}
			if evs >= 10 && gaps >= 1 {
				goto done
			}
		}
	}

done:
	if evs < 5 {
		t.Errorf("ev 条数太少：%d（至少 5）", evs)
	}
	if gaps < 1 {
		t.Errorf("没收到 gap 帧（至少 1）")
	}
	if gapFrame == nil {
		t.Fatal("gap 帧缺 GapInfo")
	}
	fmt.Printf("\n--- 小结：ev=%d  gap=%d  gap区间=[%d,%d]  最后 seq=%d ---\n",
		evs, gaps, gapFrame.FromSeq, gapFrame.ToSeq, lastSeq)
}

// TestMuxBadBase 覆盖配置错误时的报错（不连网）。
func TestMuxBadBase(t *testing.T) {
	m := NewMux("ftp://example.com")
	if _, err := m.Subscribe(context.Background(), "session:x", 0); err == nil {
		t.Fatal("scheme 不认识时应当报错")
	}
	m2 := NewMux("http://127.0.0.1:1") // 必然连不上
	if _, err := m2.Subscribe(context.Background(), "session:x", 0); err == nil {
		t.Fatal("连不上时应当报错")
	}
}
