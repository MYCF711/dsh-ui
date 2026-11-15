// market_discover_test.go —— 「发现」tab 壳侧数据面的**真 HTTP**测试（`#30` A/B 块）。
//
// 判据对着官方控制器与宿主 handler 两份原件：
//
//	分页与 fetchedAt 钉住   OFF:market-store.ts:318-360（拼页必须同一份文档）
//	声明表裁剪              OFF:market-store.ts:326-330 + presentation.ts:36-46
//	索引源写回              OFF:market-store.ts:405-415 + :295-311（选中项规则）
//	请求形状                HOST:internal/transport/handlers_plugin_market_official.go:337-392
//
// 这里验的是**壳这一层的行为**：缓存只读一次、分页真的拼起来、失败保留旧值且能重试、
// 索引不声明的过滤条件被剔掉、写回失败给官方四档。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// marketServer 是一个按方法分派的夹具服务器，记下每一次请求体。
type marketServer struct {
	mu       sync.Mutex
	requests map[string][]map[string]any // method → 每次请求体
	srv      *httptest.Server
	handlers map[string]func(body map[string]any) (int, string)
}

func newMarketServer(t *testing.T) *marketServer {
	t.Helper()
	s := &marketServer{requests: map[string][]map[string]any{}, handlers: map[string]func(map[string]any) (int, string){}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/api/")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.requests[method] = append(s.requests[method], body)
		handler := s.handlers[method]
		s.mu.Unlock()
		if handler == nil {
			w.WriteHeader(http.StatusNotImplemented)
			_, _ = w.Write([]byte(`{"error":{"code":"not_implemented","message":"没有这个夹具"}}`))
			return
		}
		status, payload := handler(body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *marketServer) on(method string, handler func(body map[string]any) (int, string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

func (s *marketServer) bodies(method string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.requests[method]...)
}

// marketIndexJSON 造一条 listIndexes 的索引源。
const marketIndexJSON = `{"url":"https://m.example/index.json","title":"演示索引"}`

// marketPage 造一页搜索结果。
func marketPage(t *testing.T, entries string, nextOffset string, fetchedAt int64) string {
	t.Helper()
	if entries == "" {
		entries = "[]"
	}
	if nextOffset == "" {
		nextOffset = "null"
	}
	return `{"result":{"url":"https://m.example/index.json","title":"演示索引",
		"metadata":{"categories":{"tools":{"title":"工具"}},"labels":{"official":{"title":"官方"}},
		"sortKeys":{"stars":{"title":"星标","order":"desc"}}},
		"fetchedAt":` + jsonInt(fetchedAt) + `,"entries":` + entries + `,"total":2,"nextOffset":` + nextOffset + `}}`
}

// marketEntry 造一条官方字段齐全的条目。
func marketEntry(name, version string) string {
	return `{"name":"` + name + `","title":"` + name + ` 标题","author":"Alice",
		"metadata":{"category":"tools","keywords":[],"labels":["official"],"sortKeys":{}},
		"versions":[{"version":"` + version + `","publishTimestamp":1767268800,
		"source":{"type":"npm"},"install":{"spec":"` + name + `@` + version + `"},"engines":{},
		"compatibility":{"compatible":true},"update":false}],
		"defaultVersion":"` + version + `","installation":{"status":"not-installed"}}`
}

func jsonInt(value int64) string {
	b, _ := json.Marshal(value)
	return string(b)
}

// waitMarket 等一个市场状态成立（壳的读取是异步的）。
func waitMarket(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等不到：%s", what)
}

// TestMarketIndexesNoIndexesIsAnEmptyList 钉住"宿主回 503 registry_unavailable +
// 消息说没有配置索引源"要当成**空清单**，而不是故障——否则全新 profile 上
// 永远只能画"无法读取索引源"，而"添加索引源"按钮只在空态出现。
func TestMarketIndexesNoIndexesIsAnEmptyList(t *testing.T) {
	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusServiceUnavailable, `{"error":{"code":"registry_unavailable","message":"pluginMarket.listIndexes 没有配置索引源：设置 plugin-market 索引 URL"}}`
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}

	if _, loaded, _, _ := m.marketIndexes(); loaded {
		t.Fatal("首帧不该已经 loaded（读取是异步的）")
	}
	waitMarket(t, "索引源读取落定", func() bool {
		_, loaded, _, _ := m.marketIndexes()
		return loaded
	})
	indexes, loaded, unavailable, errText := m.marketIndexes()
	if !loaded || unavailable || errText != "" || len(indexes) != 0 {
		t.Fatalf("没有配置索引源应当落成空清单：indexes=%v loaded=%v unavailable=%v err=%q",
			indexes, loaded, unavailable, errText)
	}
}

// TestMarketIndexesUnwiredIsUnavailable 钉住另一半：真没接线（消息写着「未接线」）
// 是 unavailable，界面上要说"没有这个能力"，不能伪装成空清单。
func TestMarketIndexesUnwiredIsUnavailable(t *testing.T) {
	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusServiceUnavailable, `{"error":{"code":"registry_unavailable","message":"pluginMarket.listIndexes 未接线：宿主没有 AttachOfficialPluginMarketAPI"}}`
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "索引源读取落定", func() bool {
		_, loaded, unavailable, _ := m.marketIndexes()
		return loaded || unavailable
	})
	_, loaded, unavailable, _ := m.marketIndexes()
	if loaded || !unavailable {
		t.Fatalf("未接线应当是 unavailable：loaded=%v unavailable=%v", loaded, unavailable)
	}
	if calls := len(srv.bodies("pluginMarket.listIndexes")); calls != 1 {
		t.Fatalf("unavailable 之后不该再自动重试，实际发了 %d 次", calls)
	}
}

// TestMarketLoadPinsOneDocumentAcrossPages 钉住分页：第一页 50 条、第二页钉同一份
// fetchedAt，拼起来的条数与顺序都是宿主的原顺序。
func TestMarketLoadPinsOneDocumentAcrossPages(t *testing.T) {
	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusOK, `{"result":[` + marketIndexJSON + `]}`
	})
	srv.on("pluginMarket.search", func(body map[string]any) (int, string) {
		query, _ := body["query"].(map[string]any)
		offset := int(query["offset"].(float64))
		if offset == 0 {
			return http.StatusOK, marketPage(t, `[`+marketEntry("a", "1.0.0")+`]`, "1", 1767268800000)
		}
		return http.StatusOK, marketPage(t, `[`+marketEntry("b", "2.0.0")+`]`, "null", 1767268800000)
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "索引源到位", func() bool { _, loaded, _, _ := m.marketIndexes(); return loaded })
	// refreshMarketIndexes 选中第一个源时会自动读第一页：等它落定再量这一次的页数。
	waitMarket(t, "自动选中的首屏落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady
	})
	before := len(srv.bodies("pluginMarket.search"))
	m.marketLoad("https://m.example/index.json", marketInitialQuery(), marketPageSize, false)

	waitMarket(t, "两页拼接落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady
	})
	listing, _ := m.marketListingNow()
	if listing.Value == nil || len(listing.Value.Entries) != 2 {
		t.Fatalf("两页应当拼成 2 条：%+v", listing.Value)
	}
	if listing.Value.Entries[0].Name != "a" || listing.Value.Entries[1].Name != "b" {
		t.Fatalf("拼接顺序必须保持宿主的原顺序：%v", listing.Value.Entries)
	}
	if listing.Value.NextOffset != nil {
		t.Fatalf("末页 nextOffset 应为 null：%v", *listing.Value.NextOffset)
	}
	// 第一页 limit=50 且不带 fetchedAt；第二页 offset=1 且钉住第一页的 fetchedAt。
	pages := srv.bodies("pluginMarket.search")
	if len(pages)-before != 2 {
		t.Fatalf("应当正好两页，实际 %d", len(pages)-before)
	}
	first, _ := pages[before]["query"].(map[string]any)
	if first["offset"].(float64) != 0 || first["limit"].(float64) != marketPageSize {
		t.Fatalf("第一页参数错了：%v", first)
	}
	if _, sent := first["fetchedAt"]; sent {
		t.Fatalf("第一页不该带 fetchedAt：%v", first)
	}
	second, _ := pages[before+1]["query"].(map[string]any)
	if second["offset"].(float64) != 1 || second["fetchedAt"].(float64) != 1767268800000 {
		t.Fatalf("后续页必须带 offset 与 first page 的 fetchedAt：%v", second)
	}
	// 索引行的事实（标题）按官方 settle 回写。
	indexes, _, _, _ := m.marketIndexes()
	if len(indexes) != 1 || indexes[0].Title.Pick() != "演示索引" || indexes[0].Error != "" {
		t.Fatalf("索引行事实没回写：%+v", indexes)
	}
}

// TestMarketDeclaredQueryDropsUndeclaredFilters 钉住"索引不再声明的 id 从 query 剔掉"，
// 并按剔过的 query 重读——这是官方 declaredQuery 的整条行为。
func TestMarketDeclaredQueryDropsUndeclaredFilters(t *testing.T) {
	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusOK, `{"result":[` + marketIndexJSON + `]}`
	})
	srv.on("pluginMarket.search", func(map[string]any) (int, string) {
		return http.StatusOK, marketPage(t, `[`+marketEntry("a", "1.0.0")+`]`, "null", 1767268800000)
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "索引源到位", func() bool { _, loaded, _, _ := m.marketIndexes(); return loaded })
	waitMarket(t, "自动选中的首屏落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady
	})
	m.marketLoad("https://m.example/index.json", apiclient.MarketQuery{
		Category: "gone", Labels: []string{"official", "gone"},
		Sort: apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "gone", Direction: "asc"},
	}, marketPageSize, false)

	waitMarket(t, "剔过之后落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady && listing.Answered != nil
	})
	listing, _ := m.marketListingNow()
	if listing.Query.Category != "" {
		t.Fatalf("索引没声明的分类必须剔掉，实得 %q", listing.Query.Category)
	}
	if len(listing.Query.Labels) != 1 || listing.Query.Labels[0] != "official" {
		t.Fatalf("索引没声明的标签必须剔掉，实得 %v", listing.Query.Labels)
	}
	if listing.Query.Sort.Kind != apiclient.MarketSortIndex {
		t.Fatalf("索引没声明的排序键必须退回默认顺序，实得 %+v", listing.Query.Sort)
	}
	// 两页请求：第一页带原始 query（发出去的就该是用户选的），第二页是剔过的。
	pages := srv.bodies("pluginMarket.search")
	last, _ := pages[len(pages)-1]["query"].(map[string]any)
	if _, sent := last["category"]; sent {
		t.Fatalf("剔过之后的请求不该再带 category：%v", last)
	}
	if sort, _ := last["sort"].(map[string]any); sort["kind"] != apiclient.MarketSortIndex {
		t.Fatalf("剔过之后的请求应退回 index 排序：%v", sort)
	}
}

// TestMarketLoadFailureKeepsOldValueAndRetries 钉住失败的两条：旧结果**保留**、
// 失败档是官方档、重试真的再发一次。
func TestMarketLoadFailureKeepsOldValueAndRetries(t *testing.T) {
	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusOK, `{"result":[` + marketIndexJSON + `]}`
	})
	var mu sync.Mutex
	fail := false
	srv.on("pluginMarket.search", func(map[string]any) (int, string) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return http.StatusBadGateway, `{"error":{"code":"index_unavailable","message":"取不到索引"}}`
		}
		return http.StatusOK, marketPage(t, `[`+marketEntry("a", "1.0.0")+`]`, "null", 1767268800000)
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "索引源到位", func() bool { _, loaded, _, _ := m.marketIndexes(); return loaded })
	waitMarket(t, "首屏落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady && listing.Value != nil
	})

	mu.Lock()
	fail = true
	mu.Unlock()
	m.marketRefresh()
	waitMarket(t, "失败落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingError
	})
	listing, _ := m.marketListingNow()
	if listing.Failure != apiclient.MarketFailureFetch {
		t.Fatalf("失败档应是 fetch（宿主把非法/取不到都回 index_unavailable）：%q", listing.Failure)
	}
	if listing.Value == nil || len(listing.Value.Entries) != 1 {
		t.Fatalf("失败时已读到的结果必须保留：%+v", listing.Value)
	}
	// 失败的索引行带宿主原文（官方 settle 同样把 error 回写进索引行）。
	indexes, _, _, _ := m.marketIndexes()
	if len(indexes) != 1 || !strings.Contains(indexes[0].Error, "index_unavailable") {
		t.Fatalf("失败原因要回写进索引行：%+v", indexes)
	}

	mu.Lock()
	fail = false
	mu.Unlock()
	m.marketRetry()
	waitMarket(t, "重试后回到 ready", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady
	})
}

// TestMarketSetIndexesReportsOfficialKinds 钉住写回索引源：成功重读清单、选中新加的
// 那个；失败按官方四档报（400 → invalid）。
func TestMarketSetIndexesReportsOfficialKinds(t *testing.T) {
	srv := newMarketServer(t)
	var mu sync.Mutex
	urls := []string{}
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		mu.Lock()
		defer mu.Unlock()
		if len(urls) == 0 {
			return http.StatusServiceUnavailable, `{"error":{"code":"registry_unavailable","message":"pluginMarket.listIndexes 没有配置索引源"}}`
		}
		payload, _ := json.Marshal(map[string]any{})
		_ = payload
		body, _ := json.Marshal(struct {
			Result []map[string]string `json:"result"`
		}{Result: []map[string]string{{"url": urls[0], "title": "演示索引"}}})
		return http.StatusOK, string(body)
	})
	srv.on("pluginMarket.setIndexes", func(body map[string]any) (int, string) {
		raw, _ := body["urls"].([]any)
		for _, item := range raw {
			value, _ := item.(string)
			if !strings.HasPrefix(value, "https://") {
				return http.StatusBadRequest, `{"error":{"code":"bad_request","message":"索引 URL 必须是 http(s) 绝对 URL"}}`
			}
		}
		if len(raw) != 1 {
			return http.StatusBadRequest, `{"error":{"code":"bad_request","message":"只放一个"}}`
		}
		mu.Lock()
		urls = []string{raw[0].(string)}
		mu.Unlock()
		return http.StatusOK, `{"result":{}}`
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "空清单到位", func() bool { _, loaded, _, _ := m.marketIndexes(); return loaded })

	// 400 → 官方 invalid 档，且 saving 复位。
	m.marketSetIndexes([]string{"不是 URL"})
	waitMarket(t, "写回失败落定", func() bool {
		saving, kind, _ := m.marketSourcesState()
		return !saving && kind != ""
	})
	if _, kind, _ := m.marketSourcesState(); kind != apiclient.MarketSourceInvalid {
		t.Fatalf("400 应投影成 invalid，实得 %q", kind)
	}

	// 成功 → 重读清单并选中新加的源。
	m.marketSetIndexes([]string{"https://m.example/index.json"})
	waitMarket(t, "写回成功后重读清单", func() bool {
		indexes, _, _, _ := m.marketIndexes()
		return len(indexes) == 1
	})
	if saving, kind, _ := m.marketSourcesState(); saving || kind != "" {
		t.Fatalf("写回成功后不该留失败档：saving=%v kind=%q", saving, kind)
	}
	waitMarket(t, "新加的源被选中并读出第一页", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.URL == "https://m.example/index.json"
	})
}

// TestMarketSearchDebouncesTyping 钉住 250ms 去抖：连打三下只发一次请求。
func TestMarketSearchDebouncesTyping(t *testing.T) {
	old := marketSearchDebounce
	marketSearchDebounce = 20 * time.Millisecond
	t.Cleanup(func() { marketSearchDebounce = old })

	srv := newMarketServer(t)
	srv.on("pluginMarket.listIndexes", func(map[string]any) (int, string) {
		return http.StatusOK, `{"result":[` + marketIndexJSON + `]}`
	})
	srv.on("pluginMarket.search", func(map[string]any) (int, string) {
		return http.StatusOK, marketPage(t, `[]`, "null", 1767268800000)
	})
	m := &model{cli: apiclient.New(srv.srv.URL)}
	waitMarket(t, "索引源到位", func() bool { _, loaded, _, _ := m.marketIndexes(); return loaded })
	m.marketSelectIndex("https://m.example/index.json")
	waitMarket(t, "首屏落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady
	})
	before := len(srv.bodies("pluginMarket.search"))

	for _, text := range []string{"d", "de", "dem"} {
		query := marketInitialQuery()
		query.Text = text
		m.marketSearch(query, text)
	}
	waitMarket(t, "去抖后的那一次落定", func() bool {
		listing, ok := m.marketListingNow()
		return ok && listing.Status == pluginapi.MarketListingReady && listing.Query.Text == "dem"
	})
	after := srv.bodies("pluginMarket.search")
	if len(after)-before != 1 {
		t.Fatalf("连打三下只该发一次请求，实际 %d 次", len(after)-before)
	}
	last, _ := after[len(after)-1]["query"].(map[string]any)
	if last["text"] != "dem" {
		t.Fatalf("最后发出去的必须是最后一次输入：%v", last)
	}
}

// TestMarketDeclaredQueryPure 是 declaredQuery 的纯函数级判据（官方 presentation.ts:36-46）。
func TestMarketDeclaredQueryPure(t *testing.T) {
	metadata := apiclient.MarketIndexMetadata{
		Categories: map[string]apiclient.MarketDeclaration{"tools": {}},
		Labels:     map[string]apiclient.MarketDeclaration{"official": {}},
		SortKeys:   map[string]apiclient.MarketSortKeyDeclaration{"stars": {Order: "desc"}},
	}
	// 全部声明过 → 原样返回，且 changed=false（不许因为"看起来一样"就多发一次请求）。
	declared, changed := marketDeclaredQuery(apiclient.MarketQuery{
		Category: "tools", Labels: []string{"official"},
		Sort: apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "stars", Direction: "desc"},
	}, metadata)
	if changed || declared.Category != "tools" || len(declared.Labels) != 1 || declared.Sort.ID != "stars" {
		t.Fatalf("全部声明的 query 不该被改：%+v changed=%v", declared, changed)
	}
	// 一个都没声明 → 三样都清掉。
	declared, changed = marketDeclaredQuery(apiclient.MarketQuery{
		Category: "gone", Labels: []string{"gone"},
		Sort: apiclient.MarketSort{Kind: apiclient.MarketSortKey, ID: "gone"},
	}, apiclient.MarketIndexMetadata{})
	if !changed || declared.Category != "" || len(declared.Labels) != 0 || declared.Sort.Kind != apiclient.MarketSortIndex {
		t.Fatalf("未声明的 id 必须全剔掉：%+v changed=%v", declared, changed)
	}
}
