# V2 侧栏视觉与行为证据

- Base rev：`bf7e8885a73a21d839d46c0684eab41d6e20f5e1`。
- 范围：`plugins/dssidebar/**` 与本文；未提交，未修改 `cmd/app`、共享契约或其他插件。
- 数据边界：侧栏只消费 `pluginapi.Env.Data.Sessions/Cur/Busy/Stream` 与 `Actions.OpenSession`；没有 v4/jsonl/zstd 读取，也不在绘制期发请求。
- 截图口径：当前工作树的 `cmd/app -shot` + 独立临时 mockd（Transport 夹具，不是 v4）；窗口移出屏幕后 `PrintWindow` 抓真实像素。它证明布局、绘制和夹具行为，不冒充真宿主验收。

## 1. 实现与真值门控

| 项 | 当前行为 | 证据 |
|---|---|---|
| 标题 | 使用 `Session.Title`，不按 cwd/ID 编造；空白才显示明确的“（无标题）” | `sidebar.go` `drawSession`；搜索/选中绘制测试 |
| 时间 | 只解析宿主 `UpdatedAt` 的 RFC3339/RFC3339Nano；空、非法、未来值整段隐藏，不回显坏值、不截片段、不把未来值写成“刚刚” | `relativeTimeAt`；`TestV2UnknownTimeIsHidden` |
| 活动 | 仅当前会话行的 `Busy=true` 或 `Stream!=""` 是正向活动真值，显示现有 `Info` 6px 点；其他会话没有逐会话活动字段，全部留空。`Busy=false` 只视为未知，不当成“空闲” | `TestV2ActivityDotOnlyForCurrentSessionWithTruth` 正/反断言 |
| 选中 | `s.ID == env.Data.Cur` 才传 `active=true`；底色走 `t.Hover`，与官方 selected/hover 同令牌 | `TestV2SelectionAndClickUseRealSessionID`；选中矩形像素 |
| 切换 | 点击经真实 `ListRow.Hit/Clicked`，原始会话 ID 原样调用 `OpenSession`；↑↓/Home/End 使用过滤、分组、折叠后的可见顺序 | 同上；`TestHandleKeyNavigation` |
| 搜索 | 真实 `flex.Field` → trim/lowercase → 标题+cwd 包含过滤；实际绘制行与键盘顺序同步缩小 | `TestV2SearchUsesFieldAndNarrowsRealRows`；搜索截图 |
| 分组/排序 | cwd 去尾分隔符；组内与组间按有效 `UpdatedAt` 新到旧，ID/key 稳定 tie-break；POSIX `/` 不再误归“未指定工作区” | `TestGroupByWorkspaceOrder`、`TestV2WorkspaceRootIsNotUnknown` |
| 几何 | 会话 32、组头 34、行距 2、组间 4、列表底部 16、搜索 30；标题 14/20、时间 10/16、图标槽 16×20 | 常量测试 + 真实 flex 矩形 + PNG 像素 |
| 滚动 | 固定 `sidebar-scroll` ID、Clip、视口/内容测量；长列表 wheel 真实移动 offset，底部留 16px | `TestV2GroupGeometryAndScroll` |

## 2. 先红后绿记录

1. 新增 `visual_v2_test.go` 后，首轮 `CGO_ENABLED=0 go test ./plugins/dssidebar -run 'TestV2' -count=1 -v`：**exit 1**。非法值 `not-a-time`、非 RFC3339 日期、未来时间仍被显示；时间规格测试确实抓到旧实现。活动断言最初按“全部隐藏”写成，随后依据现有 Snapshot 契约改为一组更精确的正向真值/非当前行断言，不把设计口径调整冒充产品修复。
2. 新增根目录与滚动底部断言后，`CGO_ENABLED=0 go test ./plugins/dssidebar -run 'TestV2WorkspaceRootIsNotUnknown|TestV2GroupGeometryAndScroll' -count=1 -v`：**exit 1**。实测 `workspaceKey("/") == ""`、底部留白 `0 != 16`；随后修实现。
3. 最终 `CGO_ENABLED=0 go test ./plugins/dssidebar -count=1`：**exit 0**，15 个顶层测试（含时间 4 个子用例）全过。
4. `CGO_ENABLED=0 go vet ./plugins/dssidebar`：**exit 0**；`git diff --check -- plugins/dssidebar`：**exit 0**。
5. 当前 app/mockd 构建：`CGO_ENABLED=0 go build -o /tmp/dssidebar-v2-visual/app-v2.exe ./cmd/app` 与对应 mockd 命令：均 **exit 0**。此前一次 app 构建曾被并行 `dsconversation` 的 `fishPath` 中间态挡住，待其 owner 收口后重跑成功；未越权修改该插件。

## 3. 无头抓图命令与退出码

临时 mockd 仅监听 `127.0.0.1:58898`，三张图完成后只回收本次启动的 job。

```bash
# 1920×1032 外框：客户区 1904×993；exit 0
/tmp/dssidebar-v2-visual/app-v2.exe -addr http://127.0.0.1:58898 \
  -size 1904x993 -shot plugins/dssidebar/testdata/visual-v2-1920x1032.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug

# 1182×832 外框：客户区 1166×793；exit 0
/tmp/dssidebar-v2-visual/app-v2.exe -addr http://127.0.0.1:58898 \
  -size 1166x793 -shot plugins/dssidebar/testdata/visual-v2-1182x832.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug

# 搜索 + 可见选中；exit 0
/tmp/dssidebar-v2-visual/app-v2.exe -addr http://127.0.0.1:58898 \
  -size 1166x793 -session s-002 -sidebar-filter 插件 \
  -shot plugins/dssidebar/testdata/visual-v2-search-1182x832.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug
```

搜索图日志断言：`可见行=1 会话=3 过滤="插件" ... 行 "s-002" 标题="UI 插件化架构方案讨论" ... 选中=true`。普通图日志为 3 行，顺序 `s-003 → s-001 → s-002`，仅 `s-001` 选中。

### PNG

- [1920×1032](../plugins/dssidebar/testdata/visual-v2-1920x1032.png)，SHA-256 `56196e38e4d52ad9d897f9590d866ad5fe7a2e2e818b52203abbcd4a9e25800d`。
- [1182×832](../plugins/dssidebar/testdata/visual-v2-1182x832.png)，SHA-256 `7e7f4a9cd6ca2d839fbd86585e851cdd51275a413fea0be9c3cb533fca9f7e9e`。
- [1182×832 搜索/选中](../plugins/dssidebar/testdata/visual-v2-search-1182x832.png)，SHA-256 `ed8f54f65aa6cb9d9d59b3deb8263726d2689dca5e902c254c4e776fbe6a53e2`。

## 4. 逐容器几何记录

坐标均为 PNG 外框像素，区间含首尾；两张普通图的左侧几何完全相同。`w=x1-x0+1`、`h=y1-y0+1`。搜索框与选中行由像素直接量得；无底色组头/普通行同时由真实 flex 矩形断言，避免靠文字位置猜。

| 容器 | 实测 `(x0…x1, y0…y1)` / 值 | 官方/推导目标 | 偏差 |
|---|---|---|---|
| 侧栏列 | `x=8…287`，**w=280** | 280 | **0** |
| 搜索框 | `x=18…277, y=83…112`，**260×30** | 内容宽 256+4；高 30 | **0 / 0** |
| 列表/行内容宽 | `x=20…287`，**w=268** | regionArea 吃掉右侧 12px padding 后的 268 | **0** |
| 组头 | `x=20…287, y=131…164`，**268×34** | 高 34 | **0** |
| 会话 1 | `x=20…287, y=166…197`，**268×32** | 高 32 | **0** |
| 选中会话 2 | `x=20…287, y=200…231`，**268×32**，RGB `(45,45,46)` | 高 32；selected/hover 同令牌 | **0** |
| 会话 3 | `x=20…287, y=234…265`，**268×32** | 高 32 | **0** |
| 行距 | 行起始 y `166→200→234`，pitch 34，**gap=2** | 2 | **0** |
| 组间距 | flex：上一组末行底到下一组头 **4** | 4 | **0** |
| 列表底部 | flex：`Content - 末行底 = 16` | 16 | **0** |
| 缩进/活动槽 | 12 / 16×20；长标题时 `Shrink(0)` | depth×12；slot 16×20 | **0** |
| 滚动 | 长列表 `Content>Viewport`；wheel 后 `offset=24` | 可滚动且不越过负责容器 | 行为通过 |

结论：本轮负责容器的已测几何偏差均 **≤8px（实际为 0）**。

## 5. 缺口与未验项

1. **真宿主未验**：三张图来自 mockd Transport 夹具；本轮没有用新 dshd 验证真实标题/时间、流式活动点或进程内宿主切换。单元测试证明的是侧栏消费与回调语义，不替代 H 系列整链。
2. **逐会话活动仍是宿主缺口**：`Session` 没有其他会话的 running/unread/error 真值，所以这些行不画状态；当前前台只使用 `Busy/Stream` 正向真值。官方活动图标的具体形态未取到完整真值，本轮仅复用现有 16×20 槽与 `Info` 点，不宣称完整官方活动视觉移植。
3. **官方搜索结果卡未完整移植**：当前是标题+cwd 过滤后的分组 32px 行；官方 `searchResultRow` 的 48px 结果卡、workspace/snippet 元信息未实现。没有 snippet 真值，不编摘要。
4. **搜索描边仍约 1px**：官方 CSS 为 0.5px；Viva `PaintInput` 自画 1px，修复需 dscore/Viva，超出本轮允许路径。30px 高、12px 圆角与 13px 字号已对齐。
5. **原生输入未验**：搜索截图的 `-sidebar-filter` 走同一 `Field.SetText→drawList` 路径，点击测试走 canvas 事件，wheel 测试走 flex 路由；它们不证明 OS IME/键盘、真实鼠标命中或滚动条拖动手感。
6. **部分几何只有布局证据**：本轮 mockd 仅一个工作区且三行不溢出，组间 4、底部 16、长列表 wheel 由真实 flex 断言覆盖，未留对应多组/滚动后 PNG。
7. **高 DPI/主题未验**：只测当前深色主题、96 DPI 的两个窗口尺寸；浅色、高 DPI、OS 键盘焦点未测。
