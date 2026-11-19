// model_catalog_editor.go —— 宿主 `models.catalog.*` 五个方法的客户端。
//
// 为什么不是 profile 的 `models`：官方那两个编辑器（DeepSeekModelsEditor.tsx:1-256、
// ModelListEditor.tsx:1-454）写的是 provider profile 的 `models` 数组，但本仓宿主
// **不发布这个字段**。`../DSH-GO/internal/settingsform/provider_schema.go:167` 的原文是
// 「baseURL/apiKeyEnv/models 等在宿主由模型目录承载」，同一份 schema 的
// `../DSH-GO/internal/llm/provider_settings.go:48` 也只读 `retryPolicy`。
// 所以写 profile `models` 等于**落盘无人消费**；本仓的等价真值是宿主的模型目录，
// 五个方法定义在 `../DSH-GO/cmd/dshd/serve_models.go:28-34` 的白名单里：
//
//	models.catalog.list       {}
//	models.catalog.upsert     {"model","credential","expectedRevision"}
//	models.catalog.reorder    {"ids","expectedRevision"}
//	models.catalog.test       {"id","baseURL","api","credential"}
//	models.catalog.set-active {"id","expectedRevision"}
//
// 三条 wire 事实照宿主实现，缺一条就会得到"看起来成功、其实没写"：
//
//   - **没有 delete**：`../DSH-GO/internal/llm/model_catalog.go:513-546` 的 Reorder 要
//     求 ids 是目录的**全排列**（数量不等即 `bad_request`、目录外 id 即拒），
//     整个方法集里没有任何删除入口。行删除在这条契约下无法表达，见
//     docs/official-parity.md #41。
//   - **显式 null 是拒绝**：`serve_models.go:236-241` 的 decode 对 credential/
//     baseURL/api/ids/expectedRevision 的 `null` 直接 `bad_request`，所以可选参数
//     只能**省略**，不能写成 null——这就是这里全部用 map 逐键拼、而不是发结构体的原因。
//   - **revision 是持久乐观锁**：`model_catalog.go:409-422` 的 checkCatalogRevision
//     在 expectedRevision 与文档 revision 不等时回 `revision_conflict`，调用方必须
//     把冲突原样交给界面，**不准重试覆盖**。nil = 不 fence。
package apiclient

import (
	"context"
	"encoding/json"
)

// ModelCatalogCredentialStatus 是目录里一个凭据引用的状态。宿主只回
// configured/writable，**永不回值**（`../DSH-GO/internal/llm/model_catalog.go:76-84`）。
type ModelCatalogCredentialStatus struct {
	Configured bool `json:"configured"`
	Writable   bool `json:"writable"`
}

// ModelCatalogActive 是目录结果里夹带的"当前默认模型"。
//
// Revision 是**设置文档**（W61 model namespace）的 revision，不是目录的：
// 宿主读它时用的是 settings 存储（`serve_models.go:189-196`），所以它与外层
// Document.Revision 是两个独立的乐观锁，不能互相替换。
type ModelCatalogActive struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

// ModelCatalogDocument 是 list / upsert / reorder 三个方法的**同一个** result
// （`serve_models.go:41-46` 的 modelCatalogListResult）。
//
// Models 保留宿主原始 JSON 对象：行的字段顺序与未识别字段都要原样回写
// （`model_catalog.go:469` 的 Upsert 注释"未知字段与 raw value 原样保留"），
// 客户端若解成结构体再序列化就会丢字段。
type ModelCatalogDocument struct {
	Revision    int64                                   `json:"revision"`
	Models      []json.RawMessage                       `json:"models"`
	Credentials map[string]ModelCatalogCredentialStatus `json:"credentials"`
	Active      ModelCatalogActive                      `json:"active"`
}

// ModelCatalogSetActiveResult 是 `models.catalog.set-active` 的 result
// （`serve_models.go:48-52`）。
//
// 三个字段一起回：ActiveModel/ActiveRevision 是设置文档的新值，
// Revision 是**目录**当时的 revision（宿主在同一次调用里读了一次目录做存在性校验）。
type ModelCatalogSetActiveResult struct {
	ActiveModel    string `json:"activeModel"`
	ActiveRevision int64  `json:"activeRevision"`
	Revision       int64  `json:"revision"`
}

// ModelCatalogTestResult 是一次真实推理探测的结果
// （`../DSH-GO/internal/llm/model_catalog.go:95-100`）。
type ModelCatalogTestResult struct {
	ID        string `json:"id"`
	OK        bool   `json:"ok"`
	Model     string `json:"model,omitempty"`
	LatencyMs int64  `json:"latencyMs"`
}

// ModelCatalogTestRequest 是 test 方法的参数。三个 override 指针为 nil = 沿用目录
// （与宿主的 ModelTestOverride 同义，`model_catalog.go:86-92`）。
//
// **不要**用空串表达"没给"：宿主对非 nil 的 override 是直接覆盖
// （`model_catalog.go:861-869`），空串会真的把 baseURL/api 清掉。
type ModelCatalogTestRequest struct {
	ID         string
	BaseURL    *string
	API        *string
	Credential *string
}

// catalogRevisionParams 把"可省略的 expectedRevision"折进参数对象。
//
// 独立成函数是因为这是本组方法唯一的空值陷阱：nil 必须**整个键都不出现**，
// 而不是发 null（`serve_models.go:236-241`）。
func catalogRevisionParams(params map[string]any, expectedRevision *int64) map[string]any {
	if expectedRevision != nil {
		params["expectedRevision"] = *expectedRevision
	}
	return params
}

// ModelCatalogList 是 `models.catalog.list`（零参数）。
func (c *Client) ModelCatalogList(ctx context.Context) (ModelCatalogDocument, error) {
	var out ModelCatalogDocument
	if err := c.Call(ctx, "models.catalog.list", nil, &out); err != nil {
		return ModelCatalogDocument{}, err
	}
	return out, nil
}

// ModelCatalogUpsert 是 `models.catalog.upsert`：同 id 部分更新、新 id 追加。
//
// model 是宿主原样的 JSON 对象（含 id）。可选字段**显式 null = 删除该字段**
// （`model_catalog.go:466-471`、:1095-1109），所以调用方要删一个可选字段就往
// 对象里塞 null，而不是发空串——空串会被 `invalid_field` 拒。
// credential 非 nil 时同一次调用顺带写凭据；空串 = 删掉这个引用。
func (c *Client) ModelCatalogUpsert(ctx context.Context, model json.RawMessage, credential *string, expectedRevision *int64) (ModelCatalogDocument, error) {
	params := map[string]any{"model": model}
	if credential != nil {
		params["credential"] = *credential
	}
	var out ModelCatalogDocument
	if err := c.Call(ctx, "models.catalog.upsert", catalogRevisionParams(params, expectedRevision), &out); err != nil {
		return ModelCatalogDocument{}, err
	}
	return out, nil
}

// ModelCatalogReorder 是 `models.catalog.reorder`：**必须给目录的全排列**。
//
// 这条约束不是客户端的礼貌，是宿主的硬校验（`model_catalog.go:525-546`）：
// 数量不等、含重复、含目录外 id 都会 `bad_request`。空目录回 `empty_catalog`。
func (c *Client) ModelCatalogReorder(ctx context.Context, ids []string, expectedRevision *int64) (ModelCatalogDocument, error) {
	if ids == nil {
		// nil 与 []string{} 在宿主那里后果不同：前者报"缺 ids 数组"，
		// 后者报 `empty_order`。都在说"你没给顺序"，客户端不替调用方选一个。
		ids = []string{}
	}
	params := catalogRevisionParams(map[string]any{"ids": ids}, expectedRevision)
	var out ModelCatalogDocument
	if err := c.Call(ctx, "models.catalog.reorder", params, &out); err != nil {
		return ModelCatalogDocument{}, err
	}
	return out, nil
}

// ModelCatalogReplace 是 `models.catalog.replace`：**一次写入整份模型数组**。
//
// 它才是官方编辑器语义的等价物：官方删一行是本地数组 `splice` 后把整个 `models`
// 数组写回去（`DeepSeekModelsEditor.tsx:157-160` 的 `onRemove` → `remove(index)`，
// 落盘走一次 `settings.mutate` 的整数组重写，`operations.ts:86-91`）。宿主
// `../DSH-GO/internal/llm/model_catalog.go` 的 `Replace` 就是同一条语义：
// 逐行规范化 + 重复 id 拒绝 + 乐观锁，然后**整数组替换**（`pruneOrphanCredentials`
// 顺带清掉没人引用的凭据）。空数组是合法目标（删到空）。
//
// 与 `ModelCatalogReorder` 的区别是决定性的：reorder 要求 ids 是目录的**全排列**
// （少一条即 `bad_request`，`model_catalog.go:525-546`），所以它表达不了删除；
// replace 直接给出目标数组，删除/新增/排序一次说完，也不再需要"先 upsert 再 reorder"
// 的两段式（那两段之间失败会留下半份目录）。
func (c *Client) ModelCatalogReplace(ctx context.Context, models []json.RawMessage, expectedRevision *int64) (ModelCatalogDocument, error) {
	if models == nil {
		// 宿主把 `models=null` 当拒绝（`serve_models.go:236-241` 的 null 白名单），
		// 而 `models=[]` 是"删到空"的合法目标。nil 切片在这里就是后者。
		models = []json.RawMessage{}
	}
	params := catalogRevisionParams(map[string]any{"models": models}, expectedRevision)
	var out ModelCatalogDocument
	if err := c.Call(ctx, "models.catalog.replace", params, &out); err != nil {
		return ModelCatalogDocument{}, err
	}
	return out, nil
}

// ModelCatalogTest 是 `models.catalog.test`：对一条目录模型发一次真实探测请求。
//
// 它**不是**"列端点上的模型"（那是官方 `llm.discoverModels`），宿主对它的注释是
// "测试是一次真实推理请求"（`model_catalog.go:884-886`），所以它会真的走认证、
// 目的地栅栏与上游。
func (c *Client) ModelCatalogTest(ctx context.Context, req ModelCatalogTestRequest) (ModelCatalogTestResult, error) {
	params := map[string]any{"id": req.ID}
	if req.BaseURL != nil {
		params["baseURL"] = *req.BaseURL
	}
	if req.API != nil {
		params["api"] = *req.API
	}
	if req.Credential != nil {
		params["credential"] = *req.Credential
	}
	var out ModelCatalogTestResult
	if err := c.Call(ctx, "models.catalog.test", params, &out); err != nil {
		return ModelCatalogTestResult{}, err
	}
	return out, nil
}

// ModelCatalogSetActive 是 `models.catalog.set-active`：把一条目录模型设成默认。
//
// 与直接写 `model.defaultModel` 的区别是宿主**先校验 id 在目录里**
// （`serve_models.go:137-152`，不在就 `not_found`）：这是同一次调用就能拿到的
// 真实校验，界面据此不会把默认值设成一个目录里根本不存在的模型。
func (c *Client) ModelCatalogSetActive(ctx context.Context, id string, expectedRevision *int64) (ModelCatalogSetActiveResult, error) {
	params := catalogRevisionParams(map[string]any{"id": id}, expectedRevision)
	var out ModelCatalogSetActiveResult
	if err := c.Call(ctx, "models.catalog.set-active", params, &out); err != nil {
		return ModelCatalogSetActiveResult{}, err
	}
	return out, nil
}

// ModelCatalogSnapshot 是壳同步发布给编辑器的那一份缓存。**不参与 Transport
// JSON**：它是壳内投影，与 ModelsPageSnapshot 同一类（见 model_catalog.go:69-82）。
//
// 为什么需要它而不是把 Document 直接给界面：`Loading` 与 `Error` 必须能和
// 「宿主真的返回了空目录」区分开——空目录是事实（`modelsEmpty` 那句话），
// 读取失败或还在飞都不是。Document 为 nil 且 Loading=false 且 Error=="" 只
// 出现在"壳还没开始读"的那一帧，界面照样画加载态。
type ModelCatalogSnapshot struct {
	Document *ModelCatalogDocument
	Error    string
	Loading  bool
}
