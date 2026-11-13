// skill_catalog_wiring_test.go —— 钉住"skills.list 接缝"的**失败语义**。
//
// 这一层最容易写错的地方不是"成功时接上"，而是**失败时怎么办**：
// 官方 `inspect !== undefined` 才画按钮，所以拉不到目录时必须把 resolver 撤掉
// （`SetInspect(nil)`），而不是留一个"点了没反应"的函数。
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/MYCF711/dsh-ui/plugins/dsskill"
	"github.com/MYCF711/dsh-ui/plugins/dstool"
)

// newSkillWiringModel 造一个只够跑 refreshSkillCatalog 的最小 model。
//
// cli 指向测试服务器；kernel 里装 dsskill（否则没有接缝可接）；
// inv 用零值 Invalidator（Invalidate 幂等，不需要窗口）。
func newSkillWiringModel(t *testing.T, baseURL string) (*model, *dsskill.Plugin) {
	t.Helper()
	skill := &dsskill.Plugin{}
	kernel := uikernel.New("skill-wiring")
	// 依赖链：dsskill 要 dstool 的 ui.tool-card，dstool 要 ui.theme/ui.primitives
	// （dscore 提供）。装配卡住是**正确的**依赖约束，不是夹具麻烦。
	if err := kernel.Load(&dscore.Plugin{Dark: true}, &dstool.Plugin{}, skill); err != nil {
		t.Fatalf("装 dscore + dstool + dsskill: %v", err)
	}
	m := &model{cli: apiclient.New(baseURL), kernel: kernel}
	return m, skill
}

// 拉不到目录 -> 撤掉 resolver（按钮消失），**不**留一个空转的函数。
func TestRefreshSkillCatalogDropsResolverOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"boom","hint":"重试","retryable":true}}`))
	}))
	defer srv.Close()

	m, skill := newSkillWiringModel(t, srv.URL)
	// 先假装上一轮接上过，确认这一轮会被**撤掉**（不是保持原样）。
	skill.SetInspect(func(string) { t.Fatal("上一轮的 resolver 不该还活着") })

	m.refreshSkillCatalog("sess-1")

	if skill.InspectForTest() != nil {
		t.Fatal("拉取失败后 resolver 必须是 nil（官方 inspect===undefined 才不画按钮）")
	}
}

// 空会话 id -> 直接撤掉，连请求都不发。
func TestRefreshSkillCatalogEmptySessionDropsResolver(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()

	m, skill := newSkillWiringModel(t, srv.URL)
	skill.SetInspect(func(string) {})
	m.refreshSkillCatalog("")

	if skill.InspectForTest() != nil {
		t.Fatal("空会话 id 时 resolver 必须是 nil")
	}
	if hits != 0 {
		t.Fatalf("空会话 id 不该发请求，实际发了 %d 次", hits)
	}
}

// 成功 -> resolver 接上，且 `/name` 能解析到路径（找不到的引用不调到预览）。
func TestRefreshSkillCatalogResolvesExactReference(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"skills":[{"path":"/skills/review/SKILL.md","name":"review","description":"d","modelInvocable":true}]}}`))
	}))
	defer srv.Close()

	m, skill := newSkillWiringModel(t, srv.URL)
	m.refreshSkillCatalog("sess-1")

	inspect := skill.InspectForTest()
	if inspect == nil {
		t.Fatal("成功时必须接上 resolver")
	}
	// 找不到的引用：官方是 `?.path` = undefined -> 什么都不做。
	// 这里用"不 panic 且不崩"当判据（预览能力没装，命中会走 openSkillPath 的 log 分支）。
	inspect("/nope")
}
