// model_catalog_singleflight_test.go —— 目录写口**不许绕过** single-flight。
//
// `models.catalog.set-active` 只回 revision，目录本体还得重读一次。这次重读如果
// 不经过 `modelsCatalogRefresh`（在飞就排队）而是直接起一个 goroutine，就会和
// 首读/其它写口的重读同时在飞：两条 list 各自把整份目录发布一次，**后回来的
// 那份可能是更早的快照**——界面上"设了默认模型，过一下又跳回原来那个"，
// 不报错、也不崩，只是结果错。
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestSetActiveDoesNotBypassCatalogSingleFlight(t *testing.T) {
	var (
		mu       sync.Mutex
		inflight int
		peak     int
		lists    atomic.Int32
	)
	release := make(chan struct{})
	var once sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models.catalog.list") {
			lists.Add(1)
			mu.Lock()
			inflight++
			if inflight > peak {
				peak = inflight
			}
			mu.Unlock()
			<-release // 卡住第一次 list，让"第二次 list 并发在飞"这件事可观测
			mu.Lock()
			inflight--
			mu.Unlock()
			_, _ = fmt.Fprint(w, `{"result":{"revision":7,"models":[],"active":{}}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/models.catalog.set-active") {
			_, _ = fmt.Fprint(w, `{"result":{"activeModel":"m-2","activeRevision":8,"revision":7}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"result":{}}`)
	}))
	defer srv.Close()
	// **必须排在上面的 defer 之后注册**（defer 是 LIFO）：用例失败（t.Fatalf）时
	// 先放开 handler 再关 server，否则 httptest.Server.Close 会等那两条卡住的
	// list 一直到用例超时——真正的失败信息就永远打不出来。
	defer func() { once.Do(func() { close(release) }) }()

	m := &model{cli: apiclient.New(srv.URL)}

	// 首读：起一条 list（会卡在 release 上）。
	snap := m.modelsCatalogSnapshot()
	if snap == nil {
		t.Fatal("modelsCatalogSnapshot 返回 nil（壳没接目录）")
	}
	waitUntil(t, "首读 list 进飞", func() bool { return lists.Load() >= 1 })

	// 在飞期间设默认模型：它的重读必须**排队**，不能再起一条 list。
	if _, err := m.modelsCatalogSetActive("m-2", nil); err != nil {
		t.Fatalf("set-active 失败：%v", err)
	}

	// 给"绕过 single-flight 的那条 list"足够时间进飞（本地 HTTP，几十毫秒足够）。
	// 正确实现下这段时间里不会有第二条 list：它被 pending 位挡住、排队等首读。
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	gotPeak := peak
	mu.Unlock()
	if gotPeak > 1 {
		t.Fatalf("同时有 %d 条 models.catalog.list 在飞，最多 1（写口绕过了 single-flight，两条快照会互相覆盖）", gotPeak)
	}

	// 放开：首读落地，排队的重读接着跑（这正是"排队再读"存在的理由）。
	once.Do(func() { close(release) })
	waitUntil(t, "排队的重读发出第二条 list", func() bool { return lists.Load() >= 2 })
}
