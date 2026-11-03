# V4A · shell / rightbar 视觉与几何证据

- base rev：`bf7e888`；最终核验时工作树已由外部推进到 `f4440d0`（本 owner 未执行 commit）。
- owned paths：`plugins/dsshell/**`、`plugins/dsrightbar/**`；本文件是唯一新增共享证据。
- 未改 `cmd/app/main.go`、其他插件或既有共享文档；未提交；未改右栏默认开关（V4B）。
- 结论：owned 容器的官方三栏公式对拍偏差为 **0px**（验收线 ≤8px）；两档逻辑尺寸均有真实无头 PNG，完整应用也有两档抓图。完整官方首屏、真宿主数据和 OS 窗口管理器缩放不在本次已验范围内。
- `f4440d0` 最终定向复测退出码 0：[`final-head-tests.log`](../tmp/v4a/final-head-tests.log)。

## 1. 规格来源与实现

官方 `computeColumns` 位于 [`tmp/official/dsh-client-ui-layout/client.js`](../tmp/official/dsh-client-ui-layout/client.js#L38-L46)：侧栏 clamp 到 264–420，给中栏保留 400，右栏首选 45%、最小 300、最大 70%；`available < 300` 时右栏轨道为 0。底部整行规则同时见 [`docs/official-specs.md`](official-specs.md#L242-L251)。

最终实现文件：

- [`plugins/dsshell/shell.go`](../plugins/dsshell/shell.go)：逐帧求解三栏；同步真实 `MainWidth`；保留现有 `columnPad=12` 跨插件 inset；`main.body` 使用可伸展轨道，使 composer 的位置不依赖空态自己是否 `Grow`；右栏放不下时整列不画。
- [`plugins/dsshell/shell_test.go`](../plugins/dsshell/shell_test.go)：官方算例、320–3840 宽扫描、壳几何回退和 `MainWidth` 同步。
- [`plugins/dsshell/geometry_test.go`](../plugins/dsshell/geometry_test.go)：真实 flex 树逐容器矩形、composer 底部位置、960 宽右栏消失和右栏底色实画。
- [`plugins/dsrightbar/rightbar.go`](../plugins/dsrightbar/rightbar.go)：真实标题/条数/工具摘要/状态圆点/连接线；空态按官方 guide 在可用 body 内居中，使用 `Fonts.Secondary` 与 `LabelTertiary`；所有颜色继续取 theme，圆点/分隔线取 primitives。
- [`plugins/dsrightbar/rightbar_test.go`](../plugins/dsrightbar/rightbar_test.go)：空态字体/颜色/居中、真实工具文本与状态图元、同窗口缩放后的边界。
- [`plugins/dsrightbar/visual_test.go`](../plugins/dsrightbar/visual_test.go)：同一 Viva 无头会话驱动真实 shell+rightbar，依次执行 1920×1032 → 1182×832 → 960×832，并实际渲染 PNG。

测试先于本 owner 的实现建立。红色驱动点包括：右栏空态实测 12px 且贴顶（目标 secondary 13px、body 居中）；shell 真实布局断言用于钉住主区轨道和 composer 底部位置。初版断言曾把现有 12px 跨插件 inset 当成应清零的偏差；核对 composer 的「shell 12 + 自身 4 = 官方 16」约束后修订了该断言，没有单方面改其他插件。

## 2. 逐容器几何（真实 flex 断言）

下表期望值直接按官方 `computeColumns` 计算；实际值来自 `TestShellGeometryAndRealSlots`，不是只看截图估读。原始输出见 [`targeted-tests.log`](../tmp/v4a/targeted-tests.log)。

| 逻辑视口 | 容器 | 官方期望 | 实测 | 偏差 |
|---|---|---|---|---|
| 1920×1032 | sidebar | `x=0, w=280` | `x=0, w=280` | **0px** |
| 1920×1032 | main | `x=280, w=776` | `x=280, w=776` | **0px** |
| 1920×1032 | rightbar | `x=1056, w=864` | `x=1056, w=864` | **0px** |
| 1920×1032 | 三栏 body 高度 | `1008`（扣 bottom 24） | `1008` | **0px** |
| 1920×1032 | shell.bottom | `x=0, y=1008, w=1920, h=24` | 同左 | **0px** |
| 1182×832 | sidebar | `x=0, w=280` | `x=0, w=280` | **0px** |
| 1182×832 | main | `x=280, w=400` | `x=280, w=400` | **0px** |
| 1182×832 | rightbar | `x=680, w=502` | `x=680, w=502` | **0px** |
| 1182×832 | 三栏 body 高度 | `808` | `808` | **0px** |
| 1182×832 | shell.bottom | `x=0, y=808, w=1182, h=24` | 同左 | **0px** |
| 960×832 | rightbar | `w=0`（available=280<300） | 整列未画，`轨迹` 文本不存在 | **0px** |
| 960×832 | main | `x=280, w=680` | `x=280, w=680` | **0px** |

composer 轨道用 80px 高测试占用方验证：1920 档 `y=916`、1182 档 `y=716`，均紧贴三栏 body 底部并保留现有 12px 下 inset。这个 80px 是夹具高度，不是官方 composer 高度声明。

## 3. 真实无头 PNG 与同进程缩放

命令：

```bash
V4A_CAPTURE_DIR='D:/DeepSeek Harness/default-workspace/dsh-ui/tmp/v4a' \
CGO_ENABLED=0 go test -count=1 -run TestV4AHeadlessResizeAndScreenshots -v ./plugins/dsrightbar
```

退出码：**0**。三张图都由 Viva offscreen renderer 当场生成；测试同时断言逻辑场景尺寸、PNG 像素尺寸、非空文件及右栏文本存在/消失。

| 证据 | 尺寸 | 真实断言 | SHA-256 |
|---|---:|---|---|
| [`headless-1920x1032.png`](../tmp/v4a/headless-1920x1032.png) | 1920×1032 | `轨迹`、bash/read、成功/失败状态实画 | `cfdbf8ca296860c97ff69aa2f1731fbee9717ca2fd260eae26d18e0014e6a05e` |
| [`headless-1182x832.png`](../tmp/v4a/headless-1182x832.png) | 1182×832 | 同一窗口缩放后右栏仍实画 | `d2e77d350a3d8a7824730a8b09d5c2cd2219176827ec63171bce3e014715fad8` |
| [`headless-resize-960x832.png`](../tmp/v4a/headless-resize-960x832.png) | 960×832 | 同一窗口继续缩小，右栏轨道消失 | `5d2cf2972af296cde30bfbdaa21399ee93aa021287ac7550fae25aab2ec06a43` |

另有 `TestToolCallsReallyPaint` 直接检查 renderer primitives：成功/失败 6×6 圆点和非零高度 2px 连接线均存在；`TestEmptyStateUsesOfficialGuideGeometry` 检查空态居中、13px 和 tertiary 颜色；`TestRightbarResizeKeepsRealContentInside` 在 500×600 → 300×480 后检查文本不越界。

## 4. 完整应用抓图（当前集成工作树）

构建：

```bash
CGO_ENABLED=0 go build -o tmp/v4a/app-v4a.exe ./cmd/app
CGO_ENABLED=0 go build -o tmp/v4a/mockd-v4a.exe ./cmd/mockd
```

均退出 **0**。使用本任务自己启动的 `mockd`（OS 分配端口 52585）和 `-demo` 工具夹具，分别运行：

```bash
./tmp/v4a/app-v4a.exe -addr http://127.0.0.1:52585 -session s-001 -tail 40 -demo \
  -rightbar=true -size 1920x1032 -shot tmp/v4a/app-1920x1032.png -shot-wait 2
./tmp/v4a/app-v4a.exe -addr http://127.0.0.1:52585 -session s-001 -tail 40 -demo \
  -rightbar=true -size 1182x832 -shot tmp/v4a/app-1182x832.png -shot-wait 2
```

两次退出码均 **0**；日志见 [`app-1920.log`](../tmp/v4a/app-1920.log)、[`app-1182.log`](../tmp/v4a/app-1182.log)。抓图完成后只回收了本任务启动的 mockd 后台任务。

- [`app-1920x1032.png`](../tmp/v4a/app-1920x1032.png)：1920×1032，SHA-256 `1504443e5ebf86cf4c2e62b671b175867c2b0bed8f81ee847bab4ff988fa3351`。
- [`app-1182x832.png`](../tmp/v4a/app-1182x832.png)：1198×871 外框，对应 1182×832 客户区，SHA-256 `b6b818a090810a4185b18363fd7d30bf01398f2d863f9161b3999e7ee568f5cd`。

Windows 外框/客户区必须分开：1920 请求被窗口管理器最大化，实际客户区宽 1904；1182 请求的 PNG 含 16px 横向和 39px 纵向外框。逐色扫描原始记录见 [`app-pixel-runs.log`](../tmp/v4a/app-pixel-runs.log)：

| 集成图 | 客户区口径 | 实测三栏 | 对官方公式偏差 |
|---|---|---|---|
| app-1920 | 客户区 1904 宽 | 280 / 767 / 857（绝对 x 色带 8–287 / 288–1054 / 1055–1911） | **0px** |
| app-1182 | 客户区 1182 宽 | 280 / 400 / 502（右栏从绝对 x=688 起，即客户区 x=680） | **0px** |

## 5. 命令与退出码汇总

| 命令 | 退出码 | 日志 |
|---|---:|---|
| `CGO_ENABLED=0 go test -count=1 -v ./plugins/dsshell ./plugins/dsrightbar` | **0** | [`targeted-tests.log`](../tmp/v4a/targeted-tests.log) |
| `CGO_ENABLED=0 go vet ./plugins/dsshell ./plugins/dsrightbar` | **0** | [`vet.log`](../tmp/v4a/vet.log) |
| `CGO_ENABLED=0 go test -count=1 ./plugins/...` | **0** | [`all-plugin-tests.log`](../tmp/v4a/all-plugin-tests.log) |
| `CGO_ENABLED=0 go build ./...` | **0** | [`full-build.log`](../tmp/v4a/full-build.log) |
| `CGO_ENABLED=0 go build -o tmp/v4a/app-v4a.exe ./cmd/app` | **0** | [`app-build.log`](../tmp/v4a/app-build.log) |
| `git diff --check -- plugins/dsshell plugins/dsrightbar` | **0** | 无输出 |

## 6. 未验证 / 不宣称完成

1. **没有做官方应用同尺寸截图像素 diff。** 本仓未找到官方 1920×1032 / 1182×832 首屏参考图；≤8px 结论仅覆盖上表 owned 容器与官方 `computeColumns` 的对拍，不代表 V1/V2/V3 或完整官方首屏全部达标。
2. **OS 窗口管理器的同窗口缩放未验。** 同进程 1920→1182→960 是 Viva headless driver + offscreen renderer；完整 app 两档图是两次独立启动。高 DPI（1.25/1.5/2）未测，无头截图为 scale=1。
3. **真宿主工具轨迹未验。** 无头测试使用合成 tool messages，完整 app 使用 mockd + `-demo`；这些只证明真实布局/绘制，不冒充真 Transport 生成或真实审批。
4. **右栏默认开关、自动开合和真实交互归 V4B。** 本轮只在测试快照中显式 `ShowRightbar=true`，未改生产默认值；dockkit 页签、拖 dock/fullscreen 也未实现或未验。
5. **右栏底色保留现有 `SidebarFill` 决策。** 它来自官方 theme token，但官方 Windows rightbarCol 本身透明、dockkit pane 自画底色；本轮不宣称右栏 pane 颜色已与官方完全一致。
6. **完整产品体验未验。** 真宿主、长会话、IME、滚动锚点、OS 键盘以及 V1/V2/V3 的控件清单不属于 V4A 本包完成声明。
