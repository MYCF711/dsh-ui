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

// ── 重连相关的两条：订阅取消后的收尾、以及 error 帧 ────────────────────
//
// 这两条和上面的 mockd 端到端自测不同：它们用 multisession_test.go 里的假宿主
// （httptest），因为要控制"什么时候取消""什么时候插一帧 error"。

// TestReconnectCtxCancelNoLeak：取消 ctx 之后，channel 必须在**合理时间**内关闭。
//
// 为什么这不是洁癖：壳每切一次会话就取消一次订阅（cmd/app/main.go 的 subscribe）。
// 如果读循环迟迟不退出，每切一次就留下一个 goroutine + 一个 ticker，
// 一天下来是几千个；而 channel 不关，调用方的 `for ev := range ch` 也永远不结束。
//
// 故意**不**设 PingEvery：默认 20s 才是线上最常见的情况，
// 也是这类收尾缺陷最容易藏身的地方（测试里习惯性调成 500ms 就把它盖住了）。
func TestReconnectCtxCancelNoLeak(t *testing.T) {
	h := newFakeHost(t)
	m := NewMux(h.URL())

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := m.Subscribe(ctx, "session:leak", 0)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	h.waitSub(t, "session:leak", 1)
	conn := h.conn(t, "session:leak", 1)
	conn.push(t, msFrame(1, "user/message", `{}`))
	if e := msNextEvent(t, ch, 3*time.Second); e.Seq != 1 {
		t.Fatalf("订阅没通：%+v", e)
	}

	cancel()
	msWaitClosed(t, ch, 2*time.Second)
}

// TestReconnectDropClosesChannelNoLeak：宿主把连接掐了（用户没切走、ctx 也没取消）时，
// channel 同样必须很快关闭。
//
// 为什么单列一条：ctx 取消会顺带取消 pingCtx，读循环能立刻收尾；
// 而"连接被掐"没有 ctx 取消，`defer pingWG.Wait()` 会一直等到下一次心跳 tick
// 才返回——默认 20s。这 20 秒里壳既不知道流已经死了，也不会去重连：
// 用户看到的是"回复永远停在一半"，而日志里一条错误都没有。
//
// 这里**故意用默认心跳**（20s），因为它就是这个缺陷的时间常数。
func TestReconnectDropClosesChannelNoLeak(t *testing.T) {
	h := newFakeHost(t)
	m := NewMux(h.URL())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := m.Subscribe(ctx, "session:drop", 0)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	h.waitSub(t, "session:drop", 1)
	conn := h.conn(t, "session:drop", 1)
	conn.push(t, msFrame(1, "user/message", `{}`))
	if e := msNextEvent(t, ch, 3*time.Second); e.Seq != 1 {
		t.Fatalf("订阅没通：%+v", e)
	}

	conn.hangup() // 服务端掐断，客户端 ctx 不动
	msWaitClosed(t, ch, 2*time.Second)
}

// TestReconnectErrorFrameNotContent：服务端推 `{type:"error",data:{reason:…}}`。
//
// 断言两件必须成立的事：
//   - 不 panic；
//   - error 帧不会被当成**内容**（不产出 content 块、不被当成回合结束），
//     后面的正常事件照常到达。
//
// "error 帧要不要单独交给调用方"目前没有定论，所以这条用例**记录**当前行为
// （t.Logf），不靠它判红；如果实现改成"error 即关流"或"专用回调"，
// 这条用例也应当相应改写，而不是删掉。
func TestReconnectErrorFrameNotContent(t *testing.T) {
	h := newFakeHost(t)
	m := NewMux(h.URL())
	m.PingEvery = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := m.Subscribe(ctx, "session:err", 0)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	h.waitSub(t, "session:err", 1)
	conn := h.conn(t, "session:err", 1)
	conn.push(t,
		msFrame(1, "user/message", `{"content":[{"type":"text","text":"一"}]}`),
		`{"type":"error","data":{"reason":"宿主内部错误"}}`,
		msFrame(2, "assistant/text", `{"delta":"二"}`),
	)

	evs := msCollectUntil(t, ch, 3*time.Second, "seq=2", func(e Event) bool {
		return e.Kind != "gap" && e.Seq == 2
	})

	var (
		others []Event
		seqs   []int
	)
	for _, e := range evs {
		switch e.Kind {
		case "gap":
			t.Fatalf("seq 1 → 2 是连续的，error 帧不该被算成断号：%+v", e.Gap)
		case "user/message", "assistant/text":
			seqs = append(seqs, e.Seq)
		default:
			others = append(others, e)
			if ch := e.Chunks(); len(ch) != 0 {
				t.Fatalf("error 帧被解析成了内容块：%+v（帧：%+v）", ch, e)
			}
			if e.End() {
				t.Fatalf("error 帧被当成了回合结束：%+v", e)
			}
		}
	}
	if len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("error 帧前后的事件必须照常到达，收到序号 %v", seqs)
	}
	t.Logf("当前行为：error 帧作为普通事件交给了调用方 %d 次，内容为空（不 panic、不造内容）：%+v",
		len(others), others)
}

// TestReconnectErrorFrameKeepsWatermark：error 帧不能把连续性水位打回 0。
//
// readLoop 目前对每一帧都做 `last = f.Seq`。服务端的 error 帧不带 seq（=0），
// 于是 last 被重置成 0，而断号检查又被 `last != 0` 挡住——
// **error 帧之后的第一段断号会被永久漏报**（界面表现为安静地少几条消息）。
func TestReconnectErrorFrameKeepsWatermark(t *testing.T) {
	h := newFakeHost(t)
	m := NewMux(h.URL())
	m.PingEvery = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := m.Subscribe(ctx, "session:err", 0)
	if err != nil {
		t.Fatalf("订阅失败：%v", err)
	}
	h.waitSub(t, "session:err", 1)
	conn := h.conn(t, "session:err", 1)
	conn.push(t,
		msFrame(1, "user/message", `{}`),
		`{"type":"error","data":{"reason":"宿主内部错误"}}`,
		msFrame(5, "assistant/text", `{"delta":"五"}`), // 中间 2、3、4 没到
	)

	evs := msCollectUntil(t, ch, 3*time.Second, "seq=5", func(e Event) bool {
		return e.Kind != "gap" && e.Seq == 5
	})
	gaps, _ := msSplit(evs)

	if len(gaps) != 1 {
		t.Fatalf("seq 1 → 5 中间缺 2、3、4，应当报 1 个 gap，收到 %d 个：%+v（水位被 error 帧重置了？）",
			len(gaps), evs)
	}
	if gaps[0] == nil || gaps[0].FromSeq != 2 || gaps[0].ToSeq != 4 {
		t.Fatalf("gap 区间应当是 [2,4]，收到 %+v", gaps[0])
	}
}
