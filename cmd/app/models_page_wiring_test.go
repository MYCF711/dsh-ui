// models_page_wiring_test.go —— Models 页同步缓存、后台刷新与独立错误契约。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

func TestModelsPageSnapshotFirstLoadCachesAndDoesNotDuplicate(t *testing.T) {
	var registeredCalls, configurableCalls, catalogCalls atomic.Int32
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 0 {
			t.Errorf("%s 必须无参，实际 %v", r.URL.Path, body)
		}
		switch r.URL.Path {
		case "/api/llm.listProviders":
			registeredCalls.Add(1)
		case "/api/llm.listConfigurableProviders":
			configurableCalls.Add(1)
		case "/api/session.modelCatalog":
			catalogCalls.Add(1)
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		<-release
		switch r.URL.Path {
		case "/api/llm.listProviders":
			_, _ = w.Write([]byte(`{"result":[{"id":"deepseek","name":"DeepSeek"}]}`))
		case "/api/llm.listConfigurableProviders":
			_, _ = w.Write([]byte(`{"result":[{"provider":"deepseek","displayName":"DeepSeek","settingsNs":"llm","settingsPath":["providers","deepseek"]}]}`))
		case "/api/session.modelCatalog":
			_, _ = w.Write([]byte(`{"result":{"default":{"provider":"deepseek","model":"deepseek-chat"},"routableProviders":["deepseek"],"groups":[{"id":"deepseek","name":"DeepSeek","models":[{"id":"deepseek-chat","name":"DeepSeek Chat"}]}],"failures":[]}}`))
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer unblock()

	m := &model{cli: apiclient.New(srv.URL)}
	first := m.modelsPageSnapshot()
	if first == nil || !first.Loading {
		t.Fatalf("首次读取必须返回 Loading=true：%+v", first)
	}
	if first.Registered != nil || first.Configurable != nil || first.Catalog != nil || first.RegisteredError != "" || first.ConfigurableError != "" || first.CatalogError != "" {
		t.Fatalf("首次读取不能伪造数据或错误：%+v", first)
	}
	waitUntil(t, "首路请求发出", func() bool { return registeredCalls.Load() == 1 })

	// 第一路仍在飞时，每次绘制都只读 Loading，不能再触发一轮。
	for i := 0; i < 5; i++ {
		if got := m.modelsPageSnapshot(); !got.Loading {
			t.Fatalf("请求在飞时必须保持 Loading=true：%+v", got)
		}
	}
	if got := registeredCalls.Load(); got != 1 {
		t.Fatalf("首路尚未完成，重复触发了请求：%d", got)
	}
	unblock()
	waitUntil(t, "Models 页三路加载完成", func() bool { return !m.modelsPageSnapshot().Loading })

	got := m.modelsPageSnapshot()
	if len(got.Registered) != 1 || got.Registered[0].ID != "deepseek" || got.Registered[0].Name != "DeepSeek" {
		t.Fatalf("已注册 provider 没有进入缓存：%+v", got.Registered)
	}
	if len(got.Configurable) != 1 || got.Configurable[0].Provider != "deepseek" || len(got.Configurable[0].SettingsPath) != 2 {
		t.Fatalf("可配置 provider 没有进入缓存：%+v", got.Configurable)
	}
	if got.Catalog == nil || got.Catalog.Default.Model != "deepseek-chat" || len(got.Catalog.Groups) != 1 || got.Catalog.Failures == nil {
		t.Fatalf("模型目录没有完整进入缓存：%+v", got.Catalog)
	}
	for i := 0; i < 5; i++ {
		_ = m.modelsPageSnapshot()
	}
	if registeredCalls.Load() != 1 || configurableCalls.Load() != 1 || catalogCalls.Load() != 1 {
		t.Fatalf("缓存读取重复触发请求：registered=%d configurable=%d catalog=%d",
			registeredCalls.Load(), configurableCalls.Load(), catalogCalls.Load())
	}
}

func TestModelsPageRefreshDuringFlightIsQueued(t *testing.T) {
	var catalogCalls atomic.Int32
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(releaseFirst) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/llm.listProviders":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/llm.listConfigurableProviders":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/session.modelCatalog":
			call := catalogCalls.Add(1)
			if call == 1 {
				<-releaseFirst
			}
			model := "old-model"
			if call > 1 {
				model = "new-model"
			}
			_, _ = w.Write([]byte(`{"result":{"default":{"provider":"deepseek","model":"` + model + `"},"routableProviders":[],"groups":[],"failures":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer unblock()

	m := &model{cli: apiclient.New(srv.URL)}
	_ = m.modelsPageSnapshot()
	waitUntil(t, "第一轮目录请求在飞", func() bool { return catalogCalls.Load() == 1 })
	m.modelsPageRefresh() // 不得丢弃：第一轮已经读过 old-model。
	unblock()
	waitUntil(t, "排队刷新发出第二轮", func() bool { return catalogCalls.Load() == 2 })
	waitUntil(t, "排队刷新完成", func() bool {
		got := m.modelsPageSnapshot()
		return !got.Loading && got.Catalog != nil && got.Catalog.Default.Model == "new-model"
	})
}

func TestModelsPageRefreshKeepsLastGoodCatalogOnFailure(t *testing.T) {
	var failCatalog atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/llm.listProviders":
			_, _ = w.Write([]byte(`{"result":[{"id":"deepseek","name":"DeepSeek"}]}`))
		case "/api/llm.listConfigurableProviders":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/session.modelCatalog":
			if failCatalog.Load() {
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"catalog refresh failed","hint":"retry","retryable":true}}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":{"default":{"provider":"deepseek","model":"deepseek-chat"},"routableProviders":["deepseek"],"groups":[{"id":"deepseek","name":"DeepSeek","models":[{"id":"deepseek-chat","name":"DeepSeek Chat"}]}],"failures":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	waitUntil(t, "Models 页首次加载", func() bool { return !m.modelsPageSnapshot().Loading })
	if got := m.modelsPageSnapshot(); got.Catalog == nil {
		t.Fatal("首次加载缺模型目录")
	}
	failCatalog.Store(true)
	m.modelsPageRefresh()
	waitUntil(t, "目录刷新失败发布", func() bool { return m.modelsPageSnapshot().CatalogError != "" })
	got := m.modelsPageSnapshot()
	if got.Catalog == nil || got.Catalog.Default.Model != "deepseek-chat" {
		t.Fatalf("刷新失败清掉了上一份真目录：%+v", got)
	}
	if got.CatalogError == "" || len(got.Registered) != 1 {
		t.Fatalf("错误原文与成功 provider 都必须保留：%+v", got)
	}
}

func TestModelsPageSnapshotKeepsIndependentErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/llm.listProviders":
			_, _ = w.Write([]byte(`{"result":[{"id":"deepseek","name":"DeepSeek"}]}`))
		case "/api/llm.listConfigurableProviders":
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"configurable 未接线","hint":"保留 configurable 提示","retryable":false}}`))
		case "/api/session.modelCatalog":
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"catalog 未接线","hint":"保留 catalog 提示","retryable":false}}`))
		default:
			t.Errorf("意外路径：%s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := &model{cli: apiclient.New(srv.URL)}
	waitUntil(t, "Models 页独立错误发布", func() bool { return !m.modelsPageSnapshot().Loading })
	got := m.modelsPageSnapshot()
	if got.Loading {
		t.Fatalf("全部结束后 Loading 必须为 false：%+v", got)
	}
	if len(got.Registered) != 1 || got.Registered[0].ID != "deepseek" || got.RegisteredError != "" {
		t.Fatalf("成功的一路必须保留真实数据：%+v", got)
	}
	if got.Configurable != nil || got.ConfigurableError != "not_implemented: configurable 未接线（保留 configurable 提示）" {
		t.Fatalf("configurable 的 501 原文被吞或改写：%+v", got)
	}
	if got.Catalog != nil || got.CatalogError != "not_implemented: catalog 未接线（保留 catalog 提示）" {
		t.Fatalf("catalog 的 501 原文被吞或改写：%+v", got)
	}
}
