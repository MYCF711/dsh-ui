// plugin_market.go —— 官方 `dsh-client-ui-plugin-market` 详情页消费的宿主方法
// （`pluginMarket.readEntryDetail`）的客户端。
//
// # 官方契约（唯一权威）
//
//	OFF:../dsh-plugin-market/lib/typert.host.js:30-38
//	     结果是一个**判别联合**：
//	       {status:'found', origin:'locale'|'readme'|'repository', markdown, document}
//	     | {status:'none'}
//	OFF:../dsh-plugin-market/lib/typert.host.js:17-29
//	     参数是四个位置参数 (url, name, version, language)，外加可选的第五个 options 对象
//	     `{refresh?: boolean, npmFileSources?: ('jsdelivr'|'unpkg'|'npmmirror')[]}`
//	OFF:../dsh-plugin-market/src/index.ts:283-345
//	     实现与错误码：`plugin-market/unknown-plugin`、`plugin-market/invalid-query`、
//	     `plugin-market/detail-unavailable`
//	OFF:../dsh-plugin-market/src/types.ts:26-30
//	     失败档是 `details.reason` 上的六个值：too-large / network / denied /
//	     git-unavailable / invalid / other
//	OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:142
//	     官方客户端只把 `plugin-market/detail-unavailable` 读成六档，**其余错误码一律
//	     `other`**——这正是下面 PluginMarketDetailFailureOf 的判据，不是我们拍的。
//
// # 宿主证据（本仓真的实现了，而且**结果形状对得上**）
//
//	HOST:internal/transport/handlers_plugin_market_official.go:53-63
//	     handler 只解 `url` / `name` / `version` / `language` 四个键
//	HOST:internal/transport/handlers_plugin_market_official.go:65-70
//	     `OfficialMarketEntryDetail{status,origin,markdown,document}`——字段与官方同集合
//	HOST:cmd/dshd/serve_plugin_market.go:234-275
//	     真实现：索引内联 README → 这一版声明的 git 仓库 README（RepositoryCache 真跑
//	     git）→ 老索引兜底（本地化 description / title）
//	HOST:internal/transport/server.go:631-640
//	     分发顺序：pluginMarket 分支**在**旧 media 501 分支之前，所以上面那份实现赢
//
// # 两处已知差值（客户端补不了，界面按"没有"处理）
//
//  1. `origin`：本仓只产出 `readme` / `repository`。官方的 `locale` 档（插件清单里
//     声明的本地化长文）本仓没有——客户端仍把三种都解出来，宿主将来给出时不丢。
//  2. `options`（`refresh` / `npmFileSources`）：宿主 handler 根本不读这两个键，
//     所以本客户端**不提供**它们。给一个"传了也没用"的旋钮，比不给更糟：界面会
//     以为"重试能绕过缓存"，而缓存那一层在宿主里压根不存在。
package apiclient

import (
	"context"
	"errors"
)

// 官方 `PluginDetail` 的两个 status（`OFF:../dsh-plugin-market/src/types.ts:157-166`）。
const (
	// PluginDetailFound 是"读到了文档"。
	PluginDetailFound = "found"
	// PluginDetailNone 是"这一版没有文档"——**不是错误**，是官方合法回答。
	PluginDetailNone = "none"
)

// 官方 `PluginDetailOrigin` 的三个取值（`OFF:../dsh-plugin-market/src/types.ts:157-166`）。
const (
	// PluginDetailOriginLocale 是插件清单声明的本地化长文（本仓宿主不产出）。
	PluginDetailOriginLocale = "locale"
	// PluginDetailOriginReadme 是索引或仓库里的 README（本仓宿主的主要来源）。
	PluginDetailOriginReadme = "readme"
	// PluginDetailOriginRepository 是这一版声明的 git 仓库里的 README。
	PluginDetailOriginRepository = "repository"
)

// PluginMarketDetailFailure 是官方六个详情失败档
// （`OFF:../dsh-plugin-market/src/types.ts:26-30` 的 `DetailFailureReason`）。
//
// 界面按档取官方 zh 文案（`OFF:../dsh-client-ui-plugin-market/src/client/locales.ts:87-92`），
// 不自己编第七种。
type PluginMarketDetailFailure string

// 官方六个失败档，逐字来自 `OFF:../dsh-plugin-market/src/types.ts:26-30`。
const (
	// PluginMarketDetailTooLarge：文档或它所在的仓库超过大小限制。
	PluginMarketDetailTooLarge PluginMarketDetailFailure = "too-large"
	// PluginMarketDetailNetwork：连不上文档的来源。
	PluginMarketDetailNetwork PluginMarketDetailFailure = "network"
	// PluginMarketDetailDenied：仓库拒绝了用户的 Git 凭据。
	PluginMarketDetailDenied PluginMarketDetailFailure = "denied"
	// PluginMarketDetailGitUnavailable：这一版要从仓库读，但 Git 跑不起来。
	PluginMarketDetailGitUnavailable PluginMarketDetailFailure = "git-unavailable"
	// PluginMarketDetailInvalid：声明的详情文件无效（格式坏 / 文件不存在）。
	PluginMarketDetailInvalid PluginMarketDetailFailure = "invalid"
	// PluginMarketDetailOther：其余一切（含本仓的 `detail_unavailable` /
	// `registry_unavailable`——宿主没有结构化 reason，见文件头）。
	PluginMarketDetailOther PluginMarketDetailFailure = "other"
)

// PluginMarketEntryDetail 是官方 `PluginDetail` 的 Go 侧形状。
//
// Found() 是唯一的判据：**不要**看 Markdown 是否为空来猜状态——`none` 也可以
// 带着空 markdown，那是两个不同的回答（"这一版没有文档" vs "读到了空文档"）。
type PluginMarketEntryDetail struct {
	// Status 是 PluginDetailFound 或 PluginDetailNone。
	Status string `json:"status"`
	// Origin 只在 found 时有值：locale / readme / repository。
	Origin string `json:"origin,omitempty"`
	// Markdown 是 UTF-8 解码后的正文（官方 typert 原文："Markdown text, decoded as UTF-8"）。
	Markdown string `json:"markdown,omitempty"`
	// Document 是这份文档的 id：官方用它去 `GET /api/plugin-market/detail-file`
	// 取同一份文档旁边的相对图片。
	//
	// ⚠ 本仓宿主没有那条路由（`grep -rn 'detail-file' cmd/ internal/` = 0 命中），
	// 所以界面**不解析**相对图片（不给假图），只留这个 id 备将来。
	Document string `json:"document,omitempty"`
}

// Found 报告是否读到了真实文档（`status === "found"` 且有正文）。
func (d PluginMarketEntryDetail) Found() bool {
	return d.Status == PluginDetailFound && d.Markdown != ""
}

// PluginMarketDetailFailureOf 把一次详情读的错误投影成官方的六个失败档。
//
// 判据与官方客户端逐条一致（`OFF:../dsh-client-ui-plugin-market/src/client/market-store.ts:142`）：
// 只有 `plugin-market/detail-unavailable` 带结构化 reason，其余一律 other。
// 本仓宿主用自己的错误码（`detail_unavailable` / `index_unavailable` /
// `registry_unavailable`），它们**都没有** `details.reason`，因此同样落到 other。
func PluginMarketDetailFailureOf(err error) PluginMarketDetailFailure {
	if err == nil {
		return ""
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return PluginMarketDetailOther
	}
	if apiErr.Code != "plugin-market/detail-unavailable" {
		return PluginMarketDetailOther
	}
	reason, _ := apiErr.Details["reason"].(string)
	switch PluginMarketDetailFailure(reason) {
	case PluginMarketDetailTooLarge, PluginMarketDetailNetwork, PluginMarketDetailDenied,
		PluginMarketDetailGitUnavailable, PluginMarketDetailInvalid:
		return PluginMarketDetailFailure(reason)
	}
	return PluginMarketDetailOther
}

// PluginMarketReadEntryDetail 读一个市场条目某一版的详细介绍
// （官方 `pluginMarket.readEntryDetail(url, name, version, language, options?)`）。
//
// `language` 是官方必填的 BCP-47 语言标签，宿主用它挑本地化兜底文案
// （`HOST:cmd/dshd/serve_plugin_market.go:278-295` 的 localizedMarketText）。
//
// 不提供 options：宿主 handler 不读它们（见文件头第 2 条差值）。
func (c *Client) PluginMarketReadEntryDetail(ctx context.Context, url, name, version, language string) (PluginMarketEntryDetail, error) {
	params := map[string]any{"url": url, "name": name, "version": version, "language": language}
	var out PluginMarketEntryDetail
	if err := c.Call(ctx, "pluginMarket.readEntryDetail", params, &out); err != nil {
		return PluginMarketEntryDetail{}, err
	}
	return out, nil
}
