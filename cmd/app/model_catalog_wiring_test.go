// model_catalog_wiring_test.go —— 模型目录编辑器写口/缓存的壳侧接线。
//
// 只钉三件壳负责的事（插件侧的形状在 dssettings 的用例里）：
//
//	① 首读返回 Loading=true 并在后台拉 `models.catalog.list`，拉到后发布缓存；
//	② 写口把宿主返回的**新文档**直接发布（不再多读一次），并返回同一个文档；
//	③ 没有 transport 时读返回 nil（插件据此不画卡）、写返回"未接线"错误。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// catalogHost 是一个只会回答 models.catalog.* 的替身宿主。
type catalogHost struct {
	mu       sync.Mutex
	lists    int
	upserts  int
	lastBody map[string]any
	revision int64
}

func (h *catalogHost) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		h.lastBody = body
		switch r.URL.Path {
		case "/api/models.catalog.list":
			h.lists++
		case "/api/models.catalog.upsert":
			h.upserts++
			h.revision++
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"revision":` + strconv.FormatInt(h.revision, 10) +
			`,"models":[{"id":"alpha"}],"credentials":{},"active":{"id":"alpha","revision":1}}}`))
	}))
}

func TestModelCatalogSnapshotLoadsInBackground(t *testing.T) {
	host := &catalogHost{revision: 7}
	srv := host.serve(t)
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}

	first := m.modelsCatalogSnapshot()
	if first == nil {
		t.Fatalf("有 transport 时首读不该返回 nil（nil 是'壳没接'的意思）")
	}
	if !first.Loading || first.Document != nil {
		t.Fatalf("首读应是 Loading 且还没有文档：%+v", first)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		loaded := m.modelCatalogLoaded
		document := m.modelCatalogCache.Document
		m.mu.Unlock()
		if loaded && document != nil {
			if document.Revision != 7 || len(document.Models) != 1 {
				t.Fatalf("目录内容不对：%+v", document)
			}
			host.mu.Lock()
			lists := host.lists
			host.mu.Unlock()
			if lists == 0 {
				t.Fatalf("应当真的发过 models.catalog.list")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("后台读没有发布缓存")
}

func TestModelCatalogWritePublishesReturnedDocument(t *testing.T) {
	host := &catalogHost{revision: 7}
	srv := host.serve(t)
	defer srv.Close()
	m := &model{cli: apiclient.New(srv.URL)}

	document, err := m.modelsCatalogUpsert(json.RawMessage(`{"id":"alpha","name":"甲"}`), nil, nil)
	if err != nil {
		t.Fatalf("写应当成功：%v", err)
	}
	if document.Revision != 8 {
		t.Fatalf("应发布宿主返回的新 revision（7→8），得到 %d", document.Revision)
	}
	m.mu.Lock()
	cached := m.modelCatalogCache.Document
	m.mu.Unlock()
	if cached == nil || cached.Revision != 8 {
		t.Fatalf("缓存应换成写口返回的那一份：%+v", cached)
	}
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.upserts != 1 {
		t.Fatalf("应只发一次 upsert，得到 %d", host.upserts)
	}
	if _, ok := host.lastBody["credential"]; ok {
		t.Fatalf("credential=nil 必须整键省略（宿主拒绝 null）：%v", host.lastBody)
	}
	if _, ok := host.lastBody["expectedRevision"]; ok {
		t.Fatalf("expectedRevision=nil 必须整键省略：%v", host.lastBody)
	}
}

func TestModelCatalogWithoutTransportIsNotAnEmptyCatalog(t *testing.T) {
	m := &model{}
	if snapshot := m.modelsCatalogSnapshot(); snapshot != nil {
		t.Fatalf("没有 transport 时读必须返回 nil（插件据此不画卡）：%+v", snapshot)
	}
	if _, err := m.modelsCatalogUpsert(json.RawMessage(`{"id":"a"}`), nil, nil); err == nil {
		t.Fatalf("没有 transport 时写必须报错，不能静默成功")
	}
	if _, err := m.modelsCatalogReorder([]string{"a"}, nil); err == nil {
		t.Fatalf("没有 transport 时 reorder 必须报错")
	}
}
