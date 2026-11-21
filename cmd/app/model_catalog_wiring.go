// model_catalog_wiring.go —— 模型目录编辑器（宿主 `models.catalog.*`）的同步缓存与写入。
//
// 与 models_page_wiring.go 同一形状，但那一路读的是**三路只读目录**
// （llm.listProviders / llm.listConfigurableProviders / session.modelCatalog），
// 这一路读的是**可写模型目录**本身：`../DSH-GO/cmd/dshd/serve_models.go:28-34` 的
// list/upsert/reorder/test/set-active。两者不能合并成一路——前者是"宿主现在能路由
// 哪些模型"的投影，后者是"目录里存了什么"，同一个 provider 可以注册而没有目录行。
//
// 三条约束：
//
//   - 插件在绘制期只读缓存，**绝不同步等 HTTP**；首读返回 Loading=true 并起一次后台读。
//   - 写口把宿主返回的新文档**直接发布**成缓存（宿主的 upsert/reorder 结果就是整份
//     目录 + 新 revision），不额外再读一次；失败只把错误原文交给调用方，缓存保持原值。
//   - 错误不折叠：`revision_conflict` / `invalid_id` 这类四字段原样传给插件分类。
package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

const modelCatalogTimeout = 8 * time.Second

// modelCatalogTestTimeout 给 `models.catalog.test` 的**真实推理**留足时间：
// 宿主那一次会走目的地栅栏 + 认证 + 上游首字节，8 秒按目录读写算不出来。
const modelCatalogTestTimeout = 60 * time.Second

// modelsCatalogSnapshot 同步返回缓存副本；首读触发一次后台请求。
//
// 返回 nil 表示**壳没有写这条能力**（没有 transport）：插件据此不画目录卡，
// 而不是画一张"0 个模型"的空卡。
func (m *model) modelsCatalogSnapshot() *apiclient.ModelCatalogSnapshot {
	if m.cli == nil {
		return nil
	}
	m.mu.Lock()
	start := !m.modelCatalogLoaded && !m.modelCatalogPending
	if start {
		m.modelCatalogPending = true
	}
	snapshot := m.modelCatalogCache
	if snapshot.Document != nil {
		document := *snapshot.Document
		document.Models = cloneModelsPageSlice(snapshot.Document.Models)
		snapshot.Document = &document
	}
	snapshot.Loading = m.modelCatalogPending
	m.mu.Unlock()

	if start {
		go m.refreshModelCatalog()
	}
	return &snapshot
}

// modelsCatalogRefresh 请求后台重读；在飞请求不叠加，改为排队再读一次
// （与 modelsPageRefresh 同一理由：在飞的请求可能已经读过旧 revision）。
func (m *model) modelsCatalogRefresh() {
	m.mu.Lock()
	if m.modelCatalogPending {
		m.modelCatalogRefreshQueued = true
		m.mu.Unlock()
		return
	}
	if m.cli == nil {
		m.mu.Unlock()
		return
	}
	m.modelCatalogPending = true
	m.mu.Unlock()
	go m.refreshModelCatalog()
}

// refreshModelCatalog 必须在 goroutine 中调用。
func (m *model) refreshModelCatalog() {
	if m.cli == nil {
		m.mu.Lock()
		m.modelCatalogPending = false
		m.modelCatalogRefreshQueued = false
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTimeout)
	document, err := m.cli.ModelCatalogList(ctx)
	cancel()

	m.mu.Lock()
	next := apiclient.ModelCatalogSnapshot{Document: m.modelCatalogCache.Document}
	if err != nil {
		// 读失败保留上一次成功值，只换错误原文：把瞬时失败投成"目录空了"会让
		// 编辑器画出一份不是宿主真实状态的草稿。
		next.Error = err.Error()
	} else {
		next.Document = &document
	}
	m.modelCatalogCache = next
	m.modelCatalogLoaded = true
	rerun := m.modelCatalogRefreshQueued
	m.modelCatalogRefreshQueued = false
	m.modelCatalogPending = rerun
	m.mu.Unlock()
	m.inv.Invalidate()
	if rerun {
		go m.refreshModelCatalog()
	}
}

// publishModelCatalog 把一次写口的返回值发布成缓存（宿主回的就是整份目录）。
func (m *model) publishModelCatalog(document apiclient.ModelCatalogDocument) {
	m.mu.Lock()
	m.modelCatalogCache = apiclient.ModelCatalogSnapshot{Document: &document}
	m.modelCatalogLoaded = true
	m.mu.Unlock()
	m.inv.Invalidate()
}

// modelsCatalogUpsert 写一条模型（宿主 `models.catalog.upsert`）。
//
// 调用方必须在 goroutine 里调；错误原样返回，由插件分类成失败态。
func (m *model) modelsCatalogUpsert(model json.RawMessage, credential *string, expectedRevision *int64) (apiclient.ModelCatalogDocument, error) {
	if m.cli == nil {
		return apiclient.ModelCatalogDocument{}, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTimeout)
	defer cancel()
	document, err := m.cli.ModelCatalogUpsert(ctx, model, credential, expectedRevision)
	if err != nil {
		return apiclient.ModelCatalogDocument{}, err
	}
	m.publishModelCatalog(document)
	return document, nil
}

// modelsCatalogReorder 提交一份全排列（宿主 `models.catalog.reorder`）。
func (m *model) modelsCatalogReorder(ids []string, expectedRevision *int64) (apiclient.ModelCatalogDocument, error) {
	if m.cli == nil {
		return apiclient.ModelCatalogDocument{}, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTimeout)
	defer cancel()
	document, err := m.cli.ModelCatalogReorder(ctx, ids, expectedRevision)
	if err != nil {
		return apiclient.ModelCatalogDocument{}, err
	}
	m.publishModelCatalog(document)
	return document, nil
}

// modelsCatalogReplace 一次写入整份模型数组（宿主 `models.catalog.replace`）。
//
// 这是编辑卡唯一的写出口：官方删一行就是本地 splice 后整数组重写
// （`DeepSeekModelsEditor.tsx:157-160` + `operations.ts:86-91`），宿主 Replace 是
// 同一条语义（逐行规范化 + 重复 id 拒绝 + 乐观锁 + 整数组替换）。调用方必须在
// goroutine 里调；错误原样返回，由插件分类成失败态。
func (m *model) modelsCatalogReplace(models []json.RawMessage, expectedRevision *int64) (apiclient.ModelCatalogDocument, error) {
	if m.cli == nil {
		return apiclient.ModelCatalogDocument{}, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTimeout)
	defer cancel()
	document, err := m.cli.ModelCatalogReplace(ctx, models, expectedRevision)
	if err != nil {
		return apiclient.ModelCatalogDocument{}, err
	}
	m.publishModelCatalog(document)
	return document, nil
}

// llmDiscoverModels 问端点上有哪些模型（官方 `llm.discoverModels`）。
//
// 它是一次**真实上游请求**（宿主真的 `GET <baseURL>/models`，
// `../DSH-GO/cmd/dshd/serve_account_api.go:492-538`），所以用测试那一档超时；
// 8 秒按目录读写算不出来。结果不进缓存：这是"用户按了一次"的动作。
func (m *model) llmDiscoverModels(settingsNS string, request apiclient.ModelDiscoveryRequest) ([]apiclient.DiscoveredModel, error) {
	if m.cli == nil {
		return nil, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTestTimeout)
	defer cancel()
	return m.cli.LLMDiscoverModels(ctx, settingsNS, request)
}

// modelsCatalogTest 发一次真实探测（宿主 `models.catalog.test`）。
//
// 它是**一次真实推理请求**（宿主 `model_catalog.go:884-886` 的注释），所以超时
// 比目录读写宽：上游慢不等于宿主没这个能力。
func (m *model) modelsCatalogTest(id string) (apiclient.ModelCatalogTestResult, error) {
	if m.cli == nil {
		return apiclient.ModelCatalogTestResult{}, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTestTimeout)
	defer cancel()
	return m.cli.ModelCatalogTest(ctx, apiclient.ModelCatalogTestRequest{ID: id})
}

// modelsCatalogSetActive 设默认模型（宿主 `models.catalog.set-active`，顺带校验 id 在目录里）。
func (m *model) modelsCatalogSetActive(id string, expectedRevision *int64) (apiclient.ModelCatalogSetActiveResult, error) {
	if m.cli == nil {
		return apiclient.ModelCatalogSetActiveResult{}, errModelCatalogNotWired
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogTimeout)
	defer cancel()
	result, err := m.cli.ModelCatalogSetActive(ctx, id, expectedRevision)
	if err != nil {
		return apiclient.ModelCatalogSetActiveResult{}, err
	}
	// **走 single-flight 入口，不要直接 go refreshModelCatalog()**：
	// set-active 只回 revision，目录本体还得重读一次，而这次重读必须和
	// `modelsCatalogSnapshot` / 其它写口共用一份"在飞就排队"的协议。
	// 绕过它的后果是两条 list 同时在飞，**后回来的那份可能是更早的快照**——
	// 缓存被旧目录覆盖，界面上"设了默认模型又跳回原来那个"，且不报错。
	m.modelsCatalogRefresh()
	return result, nil
}

// errModelCatalogNotWired 是壳没有 transport 时写口的错误（不冒充宿主四字段）。
var errModelCatalogNotWired = notWiredError("宿主未接 models.catalog.*")
