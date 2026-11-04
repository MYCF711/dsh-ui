# FG-1 Windows Chrome 验收

> 原 owner `564bdf64` 在最终文档收尾前失败；本文是 Lead 独立验收记录。范围仅 `cmd/app/**` 与 Chrome/右栏 glue，不包括 config owner 的缓存语义或完整应用行为。

## Lead 复验

```bash
CGO_ENABLED=0 go test ./cmd/app -count=1  # exit 0
CGO_ENABLED=0 go vet ./cmd/app            # exit 0
CGO_ENABLED=0 go build ./...              # exit 0
```

FG-4 集成后再次执行全仓 `go test ./...`、`go vet ./...`、`go build ./...`，全部 exit 0。

## 已核实的功能与证据

- `frameChromeLayout` / `configureFrameWindow` 使用 Viva `WindowCustomTitlebar`、`TitlebarHeight=40` 与原生 `WindowControls`；禁用 Chrome 时可恢复普通 titled window。
- 1182 实测：caption `(0,0,1182,40)`，内容 `(0,40,1182,792)`，应用/编辑按钮各 48×28、y=6、gap 2、14px 字、10px 横 padding、6px 圆角。1920 亦有同结构 JSON/PNG。
- `frame_chrome_test.go` 覆盖 caption 拖动、菜单点击、dismiss 后编辑器焦点/selection 恢复、应用菜单仅执行真实动作、编辑菜单真实选择/剪贴板及剪贴板失败不改编辑器。
- owner 的原生探针记录：最小化后 `IsIconic=true`、最大化后 `IsZoomed=true`、还原后 false、关闭钮自然退出；`headless_windows.go:336-369` 有对应系统调用与断言。Lead 未再次启动原生窗口，故这部分标为 owner 原生证据而非 Lead 重跑。
- 右栏 glue：`TestRightbarDefaultsClosedWithoutToolTrajectory`、`TestRightbarDefaultsClosedEvenWithToolTrajectory`、`TestRightbarSessionStateDoesNotLeak` 随 `cmd/app` 全绿。非 explicit 状态不再因 `hasToolTrajectory` 自动展开，按会话 explicit 状态保留。

## 实图

- `cmd/app/testdata/frame-chrome-1920x1032.png` + 同名 JSON
- `cmd/app/testdata/frame-chrome-1182x832.png` + 同名 JSON
- 1182 PNG SHA-256：`cb4bc0ed79b52b20c7d2a5c21efac4677b557c48841e118fdfbe992b7220780e`

## 边界

- 关于、更新、CLI、退出确认等无真实动作的入口未伪造。
- `m.uiConfig` 编译错误由 FG-1 按既有包级 `uiConfig` 声明做最小修正；ConfigGet 缓存加载/并发语义仍归 config owner，不在本 DONE 内。
- 截图中的连接错误/状态组合不属于 Chrome 验收，不据单图签完整应用或 Transport 完成。
- OS IME、多行编辑器、高 DPI、完整工作日未在本包验收。
