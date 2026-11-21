package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 本文件 = wire 契约**语义**审计的界面侧证据（不是字段形状）。
//
// 镜头：一个 HTTP 状态码 / 一个 `code` 承载了**两种**语义，界面用文本匹配去分，
// 于是"没有更早的了"被当成"journal 换代了"——水位全清、整页重建。
//
// 宿主侧事实（不是猜的，是跑出来的真实回包，见下面的常量）：
//
//	`internal/transport/server.go` 的 handleSessionHistory 对**两种**情况都用
//	`code = "stale_epoch"`：
//	  ① details.reason = "stale_epoch"      journal 换代 / 水位超前 → 该重建基线
//	  ② details.reason = "history_evicted"  sinceSeq 之前已滑出内存 journal
//	                                        → **这就是"没有更早的了"**，不是失败
//
// 界面侧两条判据在 `cmd/app/frames.go`：
//
//	`staleEpochError`（结构优先，`details.reason` 先判）—— 语义正确；
//	`journalEvictedError` 同理。
//
// 但**补洞**那条路（`runSubscription` 的 gap 分支）用的是
// `strings.Contains(err.Error(), "stale_epoch")`，而 `apiclient.APIError.Error()`
// 返回的第一段就是 `code`（`internal/apiclient/client.go` 的 `func (e *APIError) Error()`），
// 于是两种 409 **都会命中**——② 也被当成换代处理。本文件把这个分歧钉住。

// wireStaleEpoch409 是宿主对"水位超前 / 世代对不上"的真实回包。
//
// 采集方式：`internal/transport` 里 `POST /api/session.history
// {"id":"s1","sinceSeq":99,"journalEpoch":<当前>}` 的原始响应体。
const wireStaleEpoch409 = `{"details":{"reason":"stale_epoch"},"error":{"code":"stale_epoch",` +
	`"hint":"丢弃旧水位，重新 session.load 拿 journalEpoch/lastSeq，再从 lastSeq 往后订阅（补洞只覆盖内存里还在的帧）",` +
	`"message":"journal 世代或水位对不上，需要重新 load 基线","retryable":true},"journalEpoch":1,"lastSeq":5}`

// wireHistoryEvicted409 是宿主对"sinceSeq 之前的历史已滑出内存 journal"的真实回包。
//
// 采集方式：journal 容量 3、写 5 帧后 `POST /api/session.history {"id":"s2","sinceSeq":0}`。
//
// **注意 `code` 一样是 `stale_epoch`**——两种语义共用同一个码，只有 details.reason 不同。
const wireHistoryEvicted409 = `{"details":{"reason":"history_evicted"},"error":{"code":"stale_epoch",` +
	`"hint":"丢弃旧水位，重新 session.load 拿 journalEpoch/lastSeq，再从 lastSeq 往后订阅（补洞只覆盖内存里还在的帧）",` +
	`"message":"sinceSeq=0 之前的历史已经滑出内存 journal（只保留最近 3 帧）：继续按 seq 补会缺一段，而且缺口看不出来",` +
	`"retryable":true},"journalEpoch":7,"lastSeq":5}`

// historyConflict 用宿主的**原文回包**跑一次真实的 `apiclient.Client.History`。
func historyConflict(t *testing.T, body string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "session.history") {
			t.Errorf("请求路径不对：%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	_, err := apiclient.New(srv.URL).History(context.Background(), "s1", 0, 200, 7)
	if err == nil {
		t.Fatal("409 必须变成一次失败，拿到 nil")
	}
	return err
}

// TestAuditEvictedGapIsNotEmptyAsEpochRotation 是这条审计的主断言。
//
// 补洞（gap 分支）失败时，界面必须区分"换代了"（丢水位、重建基线）和
// "更早的历史已经滑出宿主内存"（就是到头了，什么都别做）。两种 409 的
// `code` 相同，所以**只能**按 `details.reason` 分。
func TestAuditEvictedGapIsNotEmptyAsEpochRotation(t *testing.T) {
	evicted := historyConflict(t, wireHistoryEvicted409)

	// 结构化判据（frames.go 里的两条，语义正确）：
	if journalEvictedError(evicted) != true {
		t.Fatalf("details.reason=history_evicted 应被认成「滑出内存」，实际没有：%v", evicted)
	}
	if staleEpochError(evicted) != false {
		t.Fatalf("details.reason=history_evicted 被认成了 journal 换代：%v", evicted)
	}

	// 文本判据（补洞分支当前用的那条）：
	if !strings.Contains(evicted.Error(), "stale_epoch") {
		t.Fatalf("前提变了：APIError.Error() 里不再含 code，本审计的结论需要重看：%v", evicted)
	}

	// 补洞分支必须用结构判据，所以这条必须是 false——否则一次「没有更多历史」
	// 就会清掉三份水位、重新 load、重新拉基线（用户看到整屏闪一下，
	// 而且 transcript 的原始帧被丢掉）。
	if gapFillEpochRotated(evicted) {
		t.Fatalf("history_evicted 被判成 journal 换代：补洞会清水位 + 整页重建，" +
			"而它本来只是「没有更早的了」")
	}
}

// TestAuditEpochRotationGapStillRebuilds 是上一条的**反向守卫**：
// 真正的换代必须继续走重建，别为了修 ① 把 ② 一起修没了。
func TestAuditEpochRotationGapStillRebuilds(t *testing.T) {
	stale := historyConflict(t, wireStaleEpoch409)
	if !staleEpochError(stale) {
		t.Fatalf("details.reason=stale_epoch 没被认成换代：%v", stale)
	}
	if journalEvictedError(stale) {
		t.Fatalf("details.reason=stale_epoch 被认成了「滑出内存」：%v", stale)
	}
	if !gapFillEpochRotated(stale) {
		t.Fatalf("换代没被认出来：补洞会继续用旧水位硬补，补回来的是错位数据")
	}
}

// TestAuditOldHostWithoutDetailsKeepsTextFallback 钉住老宿主的兜底路径：
// 没有 `details` 时只能看文本，两条判据不许因此互换。
func TestAuditOldHostWithoutDetailsKeepsTextFallback(t *testing.T) {
	oldEvicted := `{"error":{"code":"stale_epoch","message":"sinceSeq=0 之前的历史已经滑出内存 journal（只保留最近 3 帧）","hint":"","retryable":true}}`
	err := historyConflict(t, oldEvicted)
	var apiErr *apiclient.APIError
	if !errors.As(err, &apiErr) || apiErr.Details != nil {
		t.Fatalf("前提不成立：老宿主不该带 details（%v）", err)
	}
	if !journalEvictedError(err) {
		t.Fatalf("老宿主的「滑出内存」没被认出来：%v", err)
	}
	if gapFillEpochRotated(err) {
		t.Fatalf("老宿主的「滑出内存」被判成换代：%v", err)
	}
}
