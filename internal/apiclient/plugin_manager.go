// plugin_manager.go —— 官方 `dsh-client-ui-plugin-manager` 页面消费的宿主方法
// （`pluginManager.*`）的客户端。
//
// # 契约的唯一来源
//
// 官方是 15 个 typert Remote（`@deepseek-ai/dsh-plugin-manager/lib/typert.host.js`）；
// 本仓宿主在 `internal/transport/handlers_pluginmanager_official.go` 逐个实现，
// 生产装配在 `cmd/dshd/serve_plugins_official.go`。路径沿用本仓 `/`→`.` 的
// 既有形状：`pluginManager/listBundles` → `pluginManager.listBundles`。
//
// 本文件只解**插件页真正消费**的四个方法：
//
//	pluginManager.listBundles       已安装 bundle 清单（零参数）
//	pluginManager.readBundleDetail  一个 bundle 的 README 文档
//	pluginManager.setBundleEnabled  启用/停用（写）
//	pluginManager.removeBundle      卸载（写）
//
// 「添加插件」（installBundle）与「更新」（updateBundle）、版本列表
// （listVersions）、registry 选择在本仓宿主上要么结构化失败、要么根本没有
// 下载源（`cmd/dshd/serve_plugins_official.go` 的 `officialUnsupportedInstall`
// 与 501 点名），所以界面**不画**这些入口，也就不需要客户端方法。少一个
// 方法就少一分"界面以为能做、宿主其实做不了"的漂移。
//
// # 数据形状（逐条对着宿主结构体，不是猜的）
//
//	OfficialBundle            HOST:internal/transport/handlers_pluginmanager_official.go:655
//	OfficialBundleRow         :736          （本仓私有包没有 patch rows → []; 不是占位）
//	OfficialBundleError       :751
//	OfficialChangeResult      :536
//	OfficialBundleDetail      :634
//
// # 三个"缺席"的语义（客户端不许自己补）
//
//   - `version` / `source` / `installTarget` 缺省 = 宿主没有这个事实，界面按
//     官方回退（`sourceBuiltIn` / 不画版本标签），**不编**。
//   - `update` 缺省是官方三态的第三态（"换不了"），与 `updatable:false` 不同；
//     本仓 listBundles 恒不产出 `update`（`cmd/dshd/serve_plugins_official.go:160`
//     的 `Updatable: false`），所以界面永远不画更新入口。
//   - `rows`/`overrides` 在官方 zod 里是**必填数组**：空也必须是 `[]`；宿主
//     `normalizeOfficialBundles` 已经归一，客户端解出来是空 slice（不是 nil 的
//     "未知"）——详情页据此画官方的 `partsEmpty`，而不是画成"没读到"。
package apiclient

import (
	"context"
)

// PluginBundleMeta 是官方 `BundleInfo.meta`（`PluginLocalizedMeta`，
// `dsh-package-manifest/src/types.ts:44` 的 `LocalizedText` 两种形态）。
//
// 本仓宿主在 listBundles 里**不产出** meta（`officialPluginBundles` 没有赋这个
// 字段），所以标题回退到 bundle 名是唯一真实路径；字段仍解出来，是因为
// 官方本地包的清单可以带它，界面不该在宿主将来给出时把标题丢掉。
type PluginBundleMeta struct {
	Title       PluginLocalizedText `json:"title,omitempty"`
	Description PluginLocalizedText `json:"description,omitempty"`
	// Icon 是包根 manifest 图标（base64 data URL），官方当图片渲染。
	Icon string `json:"icon,omitempty"`
	// Error 是本地元数据诊断；**不改变可管理性**（官方 MetadataError 只画一行）。
	Error string `json:"error,omitempty"`
}

// PluginBundleRow 是官方 `BundleInfo.rows[]`（bundle patch 插入的一行）。
//
// 本仓的私有包**没有 patch rows**（`OfficialBundle.Rows` 恒为 `[]`），所以
// 详情页的「包含的组件」段会画官方 `partsEmpty`；这不是缺数据，是真实模型。
type PluginBundleRow struct {
	RowID      string            `json:"rowId"`
	ModuleName string            `json:"moduleName"`
	Meta       *PluginBundleMeta `json:"meta,omitempty"`
	EntryID    string            `json:"entryId,omitempty"`
}

// PluginIncompatiblePackage 是官方 `incompatible[]` 元素（版本被运行版本拒绝）。
type PluginIncompatiblePackage struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	RuntimeVersion string            `json:"runtimeVersion"`
	Requirements   map[string]string `json:"requirements"`
}

// PluginBundleError 是 bundle 行上的管理性错误（官方 enum code）。
type PluginBundleError struct {
	Code         string                      `json:"code"`
	Diagnostic   string                      `json:"diagnostic,omitempty"`
	Incompatible []PluginIncompatiblePackage `json:"incompatible,omitempty"`
}

// PluginBundleInstallTarget 是官方 `installTarget`（按需安装项的来源声明）。
type PluginBundleInstallTarget struct {
	Spec    string `json:"spec"`
	Version string `json:"version"`
}

// PluginBundle 是 `pluginManager.listBundles()` 的一行（官方 `BundleInfo`）。
//
// 本仓的 "bundle" 就是 `dsh.local-plugin@1` 私有包：`official=false`、
// `availability="profile"`、`installed=true`、`removable=true`、`rows=[]`、
// `overrides=[]`。这些是宿主模型，不是占位值（HOST:cmd/dshd/serve_plugins_official.go:148-176）。
type PluginBundle struct {
	Name          string                     `json:"name"`
	Official      bool                       `json:"official"`
	Availability  string                     `json:"availability"`
	InstallTarget *PluginBundleInstallTarget `json:"installTarget,omitempty"`
	Version       string                     `json:"version,omitempty"`
	Meta          *PluginBundleMeta          `json:"meta,omitempty"`
	Description   string                     `json:"description,omitempty"`
	// ListedByDefault 是官方 optional 清单的显示判据；本仓恒缺省。
	ListedByDefault *bool  `json:"listedByDefault,omitempty"`
	Enabled         bool   `json:"enabled"`
	Installed       bool   `json:"installed"`
	Source          string `json:"source,omitempty"`
	Optional        bool   `json:"optional"`
	Removable       bool   `json:"removable"`
	// Update 是 rc.2 的 `'registry' | 'git'`；**缺省 = 换不了**（三态第三态）。
	Update string `json:"update,omitempty"`
	// Updatable 是 rc.1 的废弃布尔（`true` ⇔ `update == "registry"`）。
	// 界面不读它，保留是为了把宿主真实载荷完整解出来，避免"字段悄悄丢"。
	Updatable      bool               `json:"updatable"`
	ReadOnlyReason string             `json:"readOnlyReason,omitempty"`
	Error          *PluginBundleError `json:"error,omitempty"`
	Rows           []PluginBundleRow  `json:"rows"`
	Overrides      []string           `json:"overrides"`
}

// Title 是官方 `packageText` 的标题回退链：`meta.title` → bundle 名。
//
// 官方 `packageText`（HOST 同仓先例 `presentation.ts:packageText`）在 meta
// 缺席时用包名，**不编**一个人类可读标题。
func (b PluginBundle) Title() string {
	if b.Meta != nil {
		if title := b.Meta.Title.Pick(); title != "" {
			return title
		}
	}
	return b.Name
}

// DescriptionText 是官方 `packageText` 的简介回退链：`meta.description` →
// `description` → 空串（官方没有简介行就不画）。
func (b PluginBundle) DescriptionText() string {
	if b.Meta != nil {
		if text := b.Meta.Description.Pick(); text != "" {
			return text
		}
	}
	return b.Description
}

// PluginChangeError 是官方 `ChangeResult.error`（管理性失败，不是 HTTP 错误）。
type PluginChangeError struct {
	Code       string `json:"code"`
	Diagnostic string `json:"diagnostic,omitempty"`
}

// PluginChangeResult 是官方 `ChangeResult`：所有写方法的统一结果
// （HOST:internal/transport/handlers_pluginmanager_official.go:536）。
//
// `Application` 只有 applied / cancelled / failed 三个值（本仓不跑 pnpm，
// 不报 restart-required / overridden）。管理性失败在 `Error` 里，**不是**
// HTTP 错误：调用方必须解它，不能只看 err == nil。
type PluginChangeResult struct {
	Changed       bool               `json:"changed"`
	Application   string             `json:"application"`
	RestartReason string             `json:"restartReason,omitempty"`
	Stage         string             `json:"stage"`
	Target        string             `json:"target"`
	Enabled       *bool              `json:"enabled,omitempty"`
	Error         *PluginChangeError `json:"error,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
	Bundle        string             `json:"bundle,omitempty"`
	Version       string             `json:"version,omitempty"`
}

// Failed 报告这次写是否以管理性失败结束（官方 `application === 'failed'`）。
func (r PluginChangeResult) Failed() bool { return r.Application == "failed" }

// ErrorCode 是管理性失败的官方 code（没有 error 时是空串）。
func (r PluginChangeResult) ErrorCode() string {
	if r.Error == nil {
		return ""
	}
	return r.Error.Code
}

// ErrorDiagnostic 是管理性失败的宿主诊断原文（没有 error 时是空串）。
func (r PluginChangeResult) ErrorDiagnostic() string {
	if r.Error == nil {
		return ""
	}
	return r.Error.Diagnostic
}

// PluginBundleDetail 是官方 `BundleDetail` 的联合（found | none）。
//
// `Status == "none"` 是官方回答"装上了但没有 README"，不是错误；界面这一版
// 整段不画（官方 DetailDocument 在 markdown 缺失时也不画文档）。
type PluginBundleDetail struct {
	Status   string `json:"status"`
	Origin   string `json:"origin,omitempty"`
	Path     string `json:"path,omitempty"`
	Markdown string `json:"markdown,omitempty"`
}

// Found 报告是否读到了真实文档（`status === "found"` 且有 markdown）。
func (d PluginBundleDetail) Found() bool {
	return d.Status == "found" && d.Markdown != ""
}

// PluginBundles 读已安装 bundle 清单（官方 `pluginManager.listBundles`，零参数）。
//
// 宿主没接线时回四字段 501 `not_implemented`（*APIError），**不是空数组**：
// 调用方必须区分"没有这个能力"与"一个 bundle 都没有"。
func (c *Client) PluginBundles(ctx context.Context) ([]PluginBundle, error) {
	var out []PluginBundle
	if err := c.Call(ctx, "pluginManager.listBundles", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []PluginBundle{}
	}
	return out, nil
}

// PluginReadBundleDetail 读一个 bundle 的 README（官方 `pluginManager.readBundleDetail`）。
//
// `language` 是官方必填的 BCP-47 语言标签（如 `zh-CN`）；宿主按
// `pmLanguagePattern` 校验，空串/怪形状回 400。
func (c *Client) PluginReadBundleDetail(ctx context.Context, name, language string) (PluginBundleDetail, error) {
	var out PluginBundleDetail
	params := map[string]any{"name": name, "language": language}
	if err := c.Call(ctx, "pluginManager.readBundleDetail", params, &out); err != nil {
		return PluginBundleDetail{}, err
	}
	return out, nil
}

// PluginSetBundleEnabled 启用/停用一个 bundle（官方 `pluginManager.setBundleEnabled`）。
//
// 结果是官方 `ChangeResult`：`application == "failed"` 是**管理性失败**，
// 不是 HTTP 错误，调用方要看 `Error.Code`。
func (c *Client) PluginSetBundleEnabled(ctx context.Context, name string, enabled bool) (PluginChangeResult, error) {
	var out PluginChangeResult
	params := map[string]any{"name": name, "enabled": enabled}
	if err := c.Call(ctx, "pluginManager.setBundleEnabled", params, &out); err != nil {
		return PluginChangeResult{}, err
	}
	return out, nil
}

// PluginRemoveBundle 卸载一个 bundle（官方 `pluginManager.removeBundle`）。
func (c *Client) PluginRemoveBundle(ctx context.Context, name string) (PluginChangeResult, error) {
	var out PluginChangeResult
	if err := c.Call(ctx, "pluginManager.removeBundle", map[string]any{"name": name}, &out); err != nil {
		return PluginChangeResult{}, err
	}
	return out, nil
}

// pluginManagerMethodUnavailable 报告某方法是否"宿主没这个能力"（501）。
//
// 界面据此把整页画成官方 `unavailable`（本部署没有可管理的 profile），
// 而不是错误 + 重试：重试一个不存在的方法永远是同一个 501。
func pluginManagerMethodUnavailable(err error) bool {
	apiErr, ok := err.(*APIError)
	if !ok || apiErr == nil {
		return false
	}
	return apiErr.Code == "not_implemented"
}

// PluginManagerUnavailable 报告一个 pluginManager 错误是否是"宿主没这个能力"。
func PluginManagerUnavailable(err error) bool { return pluginManagerMethodUnavailable(err) }
