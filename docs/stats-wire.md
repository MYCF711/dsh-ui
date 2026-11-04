# STATS-WIRE：上下文占用与容量真值接线

## 最终合并口径

- 最终 base：最新共享导出 `fd457ff`。
- 只取 `dsh-ui-stats-wire-final` / `dsh-ui-stats-wire-isolated` 中的 **contextWindow 增量语义**；没有直接 apply 成品 patch，也没有用旧树文件覆盖当前树。
- 保留当前树的最新模型选择、WP-C1/C2、jobs、permission、terminal、GoalBar、A/B/FG 与 API-TIME。
- `internal/apiclient/history.go`、`internal/apiclient/mux.go`、`cmd/app/wpc2_runtime.go`、其他 `cmd/app` 基线文件以及其他包均未改。

## 数据链路与边界

`session.load` 的 `contextTokens`（当前占用）与 `contextWindow`（容量上限）经
`internal/apiclient.LoadResult` 独立解码，进入每个会话自己的 `cachedSession`，再发布到
`pluginapi.Snapshot`，最后由 `plugins/dsstatusbar` 生成 dock 文本。容量不是全局最后一次
load 的值；A 后台帧、history 与 journal reset 不得改写 B 的前台占用或容量。

当前共享树已把 WP-C2 的 `integrateLoadLocked` 放在冻结的 `cmd/app/wpc2_runtime.go`。
因此增量在 `cmd/app/main.go` 的 `integrateLoadWithContextWindowLocked` 中接入：同一临界区先把
`result.ContextWindow` 写入该会话缓存，再调用唯一的 WP-C2 `integrateLoadLocked` 正文归并路径，
最后只向前台发布同会话容量。温切恢复容量，冷切清零，后台 load 不污染前台；没有复制或回退
WP-C2 归并逻辑。

`UsageTotal` 是 whole-log 累计，`ContextTokens` 会随压缩回落，二者不得相加或互相替代。
旧 `Usage.TotalTokens` 只在 whole-log 数据缺席、`ContextTokens == 0` 且没有正容量时作为兼容
上下文值；已知正容量必须搭配显式 `ContextTokens`，不能拿旧总量制造假百分比。

## 显示语义

| 输入 | 显示 |
| --- | --- |
| `used == 0 && window <= 0` | 上下文段隐藏 |
| `used > 0 && window <= 0` | 只显示原始占用，不显示百分比 |
| `used < 0` | 显示“异常”，不显示百分比 |
| `used == 0 && window > 0` | 显示真实 `0%` |
| `0 <= used <= window` | 显示占用及四舍五入百分比；真实 `100%` 合法 |
| `used > window` | 显示 `上下文 used / window（超出容量）`，不显示百分比 |

`contextSegment` 在调用既有 `contextPercent` 之前拦截超容量，所以最终 dock 绝不会把
`used > capacity` 钳成 `100%`。既有 `metrics.go` / `metrics_test.go` 属于当前 FG 基线，保持原样。

## 2026-11-04 验收证据

以下命令均 exit 0：

```bash
go test ./internal/apiclient -run '^TestLoadResultDecodesContextWindowIndependently$' -count=3
go test ./cmd/app -run '^TestContextWindow' -count=3
go test ./plugins/dsstatusbar -run '^TestContextWindow' -count=3
go test ./plugins/dsstatusbar -count=1
go test ./internal/apiclient ./pluginapi ./cmd/app ./plugins/dsstatusbar -count=1
go test ./plugins/dscomposer -run 'Model|Controls|Host' -count=1
go test ./plugins/dscommands -run Model -count=1
go test ./... -count=1
go vet ./...
CGO_ENABLED=0 go build ./...
```

真实实画命令：

```bash
FG5_CAPTURE_DIR="C:/Users/Administrator/AppData/Local/Temp/stats-wire-final2-captures" \
  go test ./plugins/dsstatusbar -run '^TestFG5HeadlessDockAndConnection$' -count=1 -v
```

该命令通过并生成 13 张非空 PNG，覆盖 1920×1032 与 1182×832 的已知容量、未知容量、
超容量、压缩回落、A→B→A 及断线画面。超容量证据：
`C:/Users/Administrator/AppData/Local/Temp/stats-wire-final2-captures/stats-wire-over-capacity-1182x832.png`。
完整实画日志：`plugins/dsstatusbar/testdata/stats-wire-final2.log`。

实画还直接断言完整异常文案且没有 `%`，并验证 dock 高 26、位于 composer 卡片下方且同轴、
连接状态点颜色正确，以及 A→B→empty 不残留旧数字。最新模型选择仍由当前 `dscommands`
与壳的 `ModelsList` / `SetModel` 路径提供，其定向测试和全仓测试均通过。
