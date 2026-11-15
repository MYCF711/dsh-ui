// plugin_market_discover_test.go —— Discover tab 三个宿主方法（`#30` A/B 块）的客户端契约测试。
//
// 判据对着官方 schema 与宿主 handler 两份原件：
//
//	① listIndexes **无参数**（body 必须是 `{}`），结果是数组；title/description 是
//	   `string | localized` union（HOST:handlers_plugin_market_official.go:337-351）；
//	② search 是**平铺** `url` + `query`，query 里 sort/offset/limit 必填、空的
//	   text/category/labels 整键不发（OFF:market-store.ts:395-403）；
//	   结果字段与官方同名，`nextOffset` 末页是**显式 null**（HOST:同文件 :217-242）；
//	③ setIndexes 读 `urls`，结果无载荷（HOST:同文件 :353-369）；
//	④ 四档 sourceFailure / 列表失败档的投影与官方客户端同判据
//	   （OFF:market-store.ts:140-152）。
package apiclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeBody 解一次请求体，顺手断言没套 params 信封。
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("请求体不是 JSON 对象：%v", err)
	}
	if _, wrapped := body["params"]; wrapped {
		t.Fatalf("不许套 params 信封：%v", body)
	}
	return body
}

func TestPluginMarketListIndexesSendsNoParams(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginMarket.listIndexes" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"result":[
			{"url":"https://m.example/index.json","title":"演示索引","description":"一句话"},
			{"url":"https://n.example/index.json","title":{"en":"Second","zh":"第二"},"error":"读不到"}
		]}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginMarketListIndexes(context.Background())
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if len(body) != 0 {
		t.Fatalf("listIndexes 官方无参数，body 必须是空对象，实得 %v", body)
	}
	if len(got) != 2 {
		t.Fatalf("应当解出 2 条索引源，实得 %d：%+v", len(got), got)
	}
	// 纯字符串与对象两种 union 都要解出来（宿主两种都会下发）。
	if got[0].Title.Pick() != "演示索引" || got[0].Description.Pick() != "一句话" {
		t.Fatalf("字符串形态的 union 解错了：%+v", got[0])
	}
	if got[1].Title.Pick() != "第二" {
		t.Fatalf("对象形态的 union 应优先取中文，实得 %q", got[1].Title.Pick())
	}
	if got[1].Error != "读不到" {
		t.Fatalf("error 字段应如实解出，实得 %q", got[1].Error)
	}
}

func TestPluginMarketListIndexesEmptyIsAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer srv.Close()
	got, err := New(srv.URL).PluginMarketListIndexes(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("空数组是合法回答（一个索引源都没配）：got=%+v err=%v", got, err)
	}
}

func TestPluginMarketSearchSendsOfficialQuery(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginMarket.search" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"result":{
			"url":"https://m.example/index.json",
			"title":"演示索引",
			"metadata":{
				"categories":{"tools":{"title":"工具"}},
				"labels":{"official":{"title":"官方"}},
				"sortKeys":{"stars":{"title":"星标","order":"desc"}}
			},
			"fetchedAt":1767268800000,
			"entries":[{
				"name":"demo-plugin",
				"title":"演示插件",
				"description":{"en":"Demo","zh":"演示"},
				"icon":"https://x.example/i.png",
				"author":"Alice",
				"homepage":"https://example.com",
				"repository":{"url":"https://github.com/x/y","directory":"packages/demo"},
				"metadata":{"category":"tools","keywords":["dsh"],"labels":["official"],"sortKeys":{"stars":12}},
				"versions":[{
					"version":"1.2.3",
					"publishTimestamp":1767268800,
					"source":{"type":"git","url":"https://github.com/x/y","commit":"abc123"},
					"install":{"spec":"git+https://github.com/x/y#abc123"},
					"engines":{"dsh":"^0.2.0"},
					"compatibility":{"compatible":false,"runtimeVersion":"0.2.2","requirements":{"dsh":"^0.3.0"},"exempted":true},
					"update":true
				},{
					"version":"1.2.2",
					"publishTimestamp":1767182400,
					"source":{"type":"npm","registry":"https://registry.example"},
					"install":{"spec":"demo-plugin@1.2.2","registry":"https://registry.example"},
					"engines":{},
					"compatibility":{"compatible":true},
					"update":false
				}],
				"defaultVersion":"1.2.3",
				"installation":{"status":"installed","installedVersion":"1.2.2"}
			}],
			"total":1,
			"nextOffset":null
		}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginMarketSearch(context.Background(), "https://m.example/index.json", MarketQuery{
		Sort: MarketSort{Kind: MarketSortIndex}, Offset: 0, Limit: 50,
	})
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if body["url"] != "https://m.example/index.json" {
		t.Fatalf("url 参数 = %v", body["url"])
	}
	query, ok := body["query"].(map[string]any)
	if !ok {
		t.Fatalf("query 必须是对象：%v", body["query"])
	}
	// 三个必填键一个不少。
	for _, key := range []string{"sort", "offset", "limit"} {
		if _, sent := query[key]; !sent {
			t.Fatalf("query 缺必填键 %q：%v", key, query)
		}
	}
	// 空的可选键整键不发（官方对象展开写法，空数组/空串根本不出现）。
	for _, key := range []string{"text", "category", "labels", "refresh", "fetchedAt"} {
		if _, sent := query[key]; sent {
			t.Fatalf("空的 %q 不该发（宿主按官方 strictObject 校验）：%v", key, query)
		}
	}
	if sort, ok := query["sort"].(map[string]any); !ok || sort["kind"] != "index" || len(sort) != 1 {
		t.Fatalf("index 排序只该有 kind，实得 %v", query["sort"])
	}

	// 结果：官方字段逐个解出来。
	if got.URL != "https://m.example/index.json" || got.Title.Pick() != "演示索引" {
		t.Fatalf("索引事实解错了：%+v", got)
	}
	if got.Metadata.Categories["tools"].Title.Pick() != "工具" ||
		got.Metadata.Labels["official"].Title.Pick() != "官方" ||
		got.Metadata.SortKeys["stars"].Order != "desc" {
		t.Fatalf("索引的三张声明表解错了：%+v", got.Metadata)
	}
	if got.FetchedAt != 1767268800000 || got.Total != 1 {
		t.Fatalf("fetchedAt/total 解错了：%+v", got)
	}
	if got.NextOffset != nil {
		t.Fatalf("末页 nextOffset 必须是显式 null，实得 %v", *got.NextOffset)
	}
	entry := got.Entries[0]
	if entry.Name != "demo-plugin" || entry.Title.Pick() != "演示插件" || entry.Description.Pick() != "演示" {
		t.Fatalf("条目基本字段解错了：%+v", entry)
	}
	if entry.Repository == nil || entry.Repository.Directory != "packages/demo" {
		t.Fatalf("repository 解错了：%+v", entry.Repository)
	}
	if entry.Metadata.Category != "tools" || len(entry.Metadata.Labels) != 1 || entry.Metadata.SortKeys["stars"] != 12 {
		t.Fatalf("条目 metadata 解错了：%+v", entry.Metadata)
	}
	if entry.DefaultVersion != "1.2.3" || entry.Installation.Status != MarketInstallStatusInstalled ||
		entry.Installation.InstalledVersion != "1.2.2" {
		t.Fatalf("默认版本/安装态解错了：%+v", entry)
	}
	gitVersion := entry.Versions[0]
	if gitVersion.Source.Type != MarketSourceGit || gitVersion.Source.Commit != "abc123" || gitVersion.Source.Registry != "" {
		t.Fatalf("git 来源解错了：%+v", gitVersion.Source)
	}
	if gitVersion.Compatibility.Compatible || !gitVersion.Compatibility.Exempted ||
		gitVersion.Compatibility.Requirements["dsh"] != "^0.3.0" || !gitVersion.Update {
		t.Fatalf("不兼容 union 解错了：%+v", gitVersion)
	}
	npmVersion := entry.Versions[1]
	if npmVersion.Source.Type != MarketSourceNPM || npmVersion.Source.Registry != "https://registry.example" ||
		npmVersion.Source.Commit != "" || !npmVersion.Compatibility.Compatible || npmVersion.Update {
		t.Fatalf("npm 来源解错了：%+v", npmVersion)
	}
	if npmVersion.Install.Spec != "demo-plugin@1.2.2" || npmVersion.PublishTimestamp != 1767182400 {
		t.Fatalf("install/publishTimestamp 解错了：%+v", npmVersion)
	}
}

func TestPluginMarketSearchSendsFiltersAndPaging(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"result":{"url":"u","title":"t","metadata":{"categories":{},"labels":{},"sortKeys":{}},"fetchedAt":7,"entries":[],"total":0,"nextOffset":50}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).PluginMarketSearch(context.Background(), "u", MarketQuery{
		Text: "demo", Category: "tools", Labels: []string{"official"},
		Sort:   MarketSort{Kind: MarketSortKey, ID: "stars", Direction: "asc"},
		Offset: 50, Limit: 100, Refresh: true, FetchedAt: 19,
	})
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	query, _ := body["query"].(map[string]any)
	want := map[string]any{"text": "demo", "category": "tools", "refresh": true, "offset": float64(50), "limit": float64(100), "fetchedAt": float64(19)}
	for key, value := range want {
		if query[key] != value {
			t.Fatalf("query.%s = %v，想要 %v（全量 %v）", key, query[key], value, query)
		}
	}
	labels, _ := query["labels"].([]any)
	if len(labels) != 1 || labels[0] != "official" {
		t.Fatalf("labels 解错了：%v", query["labels"])
	}
	sort, _ := query["sort"].(map[string]any)
	if sort["kind"] != MarketSortKey || sort["id"] != "stars" || sort["direction"] != "asc" {
		t.Fatalf("key 排序必须带 id + direction：%v", sort)
	}
	if got.NextOffset == nil || *got.NextOffset != 50 {
		t.Fatalf("非末页 nextOffset 必须解成数字，实得 %v", got.NextOffset)
	}
}

func TestPluginMarketSetIndexesSendsURLs(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pluginMarket.setIndexes" {
			t.Errorf("路径 = %q", r.URL.Path)
		}
		body = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"result":{}}`))
	}))
	defer srv.Close()

	if err := New(srv.URL).PluginMarketSetIndexes(context.Background(), []string{"https://m.example/index.json"}); err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	urls, _ := body["urls"].([]any)
	if len(urls) != 1 || urls[0] != "https://m.example/index.json" {
		t.Fatalf("urls 参数解错了：%v", body)
	}
	// 空列表也必须是**空数组**，不能是 null（宿主按字符串数组解）。
	if err := New(srv.URL).PluginMarketSetIndexes(context.Background(), nil); err != nil {
		t.Fatalf("空列表应当成功：%v", err)
	}
	if urls, ok := body["urls"].([]any); !ok || len(urls) != 0 {
		t.Fatalf("nil 要发成空数组，实得 %v", body["urls"])
	}
}

func TestMarketSourceFailureOfFollowsOfficialKinds(t *testing.T) {
	if kind, reason := MarketSourceFailureOf(nil); kind != "" || reason != "" {
		t.Fatalf("没有错误就不该有失败档：%q %q", kind, reason)
	}
	invalid := &APIError{Code: "bad_request", Message: "索引 URL 必须是 http(s) 绝对 URL: \"x\""}
	if kind, reason := MarketSourceFailureOf(invalid); kind != MarketSourceInvalid || reason == "" {
		t.Fatalf("400 bad_request 应投影成 invalid 并带宿主原文，实得 %q %q", kind, reason)
	}
	unavailable := &APIError{Code: "registry_unavailable", Message: "没有配置索引源"}
	if kind, reason := MarketSourceFailureOf(unavailable); kind != MarketSourceOther || reason != "没有配置索引源" {
		t.Fatalf("其余错误码一律 other + 宿主原文，实得 %q %q", kind, reason)
	}
	if kind, _ := MarketSourceFailureOf(context.DeadlineExceeded); kind != MarketSourceOther {
		t.Fatalf("非 API 错误也必须是 other，实得 %q", kind)
	}
}

func TestMarketListingFailureOfIsFetch(t *testing.T) {
	if kind := MarketListingFailureOf(nil); kind != "" {
		t.Fatalf("没有错误就没有失败档，实得 %q", kind)
	}
	// 宿主把"索引文档非法"和"取不到"都回 index_unavailable（serve_plugin_market.go:146-149），
	// 所以两者都按 fetch 画「无法读取此索引」——不假装能分出 invalid。
	for _, err := range []error{&APIError{Code: "index_unavailable"}, context.DeadlineExceeded} {
		if kind := MarketListingFailureOf(err); kind != MarketFailureFetch {
			t.Fatalf("列表读失败应投影成 fetch，实得 %q（err=%v）", kind, err)
		}
	}
}

func TestMarketIndexSummaryPickLabel(t *testing.T) {
	cases := []struct {
		summary MarketIndexSummary
		want    string
	}{
		{MarketIndexSummary{URL: "https://m.example/index.json", Title: PluginLocalizedText{Text: "演示"}}, "演示"},
		{MarketIndexSummary{URL: "https://m.example:8443/index.json"}, "m.example:8443"},
		{MarketIndexSummary{URL: "https://user:pass@m.example/x.json"}, "m.example"},
		{MarketIndexSummary{URL: "不是 URL"}, "不是 URL"},
	}
	for _, tc := range cases {
		if got := tc.summary.PickLabel(); got != tc.want {
			t.Fatalf("PickLabel(%q) = %q，想要 %q", tc.summary.URL, got, tc.want)
		}
	}
}
