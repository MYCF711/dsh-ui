// skills_test.go —— 官方 `skills.list` 的客户端契约测试。
//
// 钉三件事（都不是猜的）：
//   - wire 路径 `/api/skills.list` 与参数名 `sessionId`（官方 source=lookup）；
//   - 可选字段（`path` / `whenToUse`）**缺席就是缺席**，不往空串上凑；
//   - `/name` 解析是**整串精确匹配**（官方 `'/' + skill.name === ref`），
//     找不到返回空串 —— 不做模糊匹配、不去大小写、不剥后缀。
package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillsListSendsWireNamesAndDecodesEntries(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"result":{"skills":[
			{"path":"/home/skills/review/SKILL.md","name":"review","description":"审阅改动","whenToUse":"要审代码时","modelInvocable":true},
			{"path":"/home/skills/quiet/SKILL.md","name":"quiet","description":"安静模式","modelInvocable":false}
		]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).SkillsList(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("应当成功：%v", err)
	}
	if gotPath != "/api/skills.list" {
		t.Fatalf("路径应为 /api/skills.list，得到 %q", gotPath)
	}
	if body["sessionId"] != "sess-1" {
		t.Fatalf("wire 名必须是 sessionId（官方 source=lookup），得到 %v", body)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("应有 2 条 skill，得到 %d", len(got.Skills))
	}
	first := got.Skills[0]
	if first.Name != "review" || first.Description != "审阅改动" || first.WhenToUse != "要审代码时" {
		t.Fatalf("第一条解码不对：%+v", first)
	}
	if !first.ModelInvocable {
		t.Fatalf("review 的 modelInvocable 应为 true：%+v", first)
	}
	// 官方 `whenToUse` 是可选的：没写就是空串，不是"未知"之类的占位。
	if second := got.Skills[1]; second.WhenToUse != "" {
		t.Fatalf("没给 whenToUse 时应当是空串（omitempty），得到 %q", second.WhenToUse)
	}
	if got.Skills[1].ModelInvocable {
		t.Fatalf("quiet 的 modelInvocable 应为 false：%+v", got.Skills[1])
	}
}

// 空目录是**正常结果**，不是错误：官方宿主对"真的没有 skill"回 `skills: []`。
func TestSkillsListEmptyCatalogIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"skills":[]}}`))
	}))
	defer srv.Close()

	got, err := New(srv.URL).SkillsList(context.Background(), "sess-empty")
	if err != nil {
		t.Fatalf("空目录不该报错：%v", err)
	}
	if len(got.Skills) != 0 {
		t.Fatalf("空目录应当是 0 条，得到 %d", len(got.Skills))
	}
}

// 会话不存在时宿主回四字段错误（稳定码 `not_found`），客户端必须原样带出来。
func TestSkillsListSurfacesSessionNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"session \"nope\" not found","hint":"先建会话","retryable":false}}`))
	}))
	defer srv.Close()

	_, err := New(srv.URL).SkillsList(context.Background(), "nope")
	if err == nil {
		t.Fatal("会话不存在必须报错，不能塌成空目录")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("应当是 *APIError，得到 %T：%v", err, err)
	}
	if apiErr.Code != "not_found" {
		t.Fatalf("稳定码应为 not_found，得到 %q", apiErr.Code)
	}
}

// 找不到的引用返回空串（官方 `?.path` 是 undefined），不做任何模糊匹配。
func TestResolveSkillPathIsExactSlashMatch(t *testing.T) {
	catalog := []SkillEntry{
		{Name: "review", Path: "/skills/review/SKILL.md"},
		{Name: "code-review", Path: "/skills/code-review/SKILL.md"},
		{Name: "Review", Path: "/skills/Review/SKILL.md"}, // 大小写不同的另一个 skill
	}
	cases := []struct {
		ref  string
		want string
	}{
		{"/review", "/skills/review/SKILL.md"},
		{"/code-review", "/skills/code-review/SKILL.md"},
		{"/Review", "/skills/Review/SKILL.md"}, // 大小写敏感：必须命中它自己那条
		{"review", ""},                         // 少了前导 `/`：不是官方形状
		{"/revi", ""},                          // 前缀不算命中
		{"/review/", ""},                       // 后缀不算命中
		{"/nope", ""},                          // 找不到
		{"", ""},                               // 空引用
	}
	for _, tc := range cases {
		if got := ResolveSkillPath(catalog, tc.ref); got != tc.want {
			t.Fatalf("ResolveSkillPath(%q) = %q，want %q", tc.ref, got, tc.want)
		}
	}
}
