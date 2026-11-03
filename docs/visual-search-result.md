# U3A · 官方 48px `searchResultRow` 搜索结果卡（实现与证据）

- Base rev：`bc79784`；改动未提交、未 push。
- 允许路径内的改动：`plugins/dssidebar/sidebar.go`、新增 `plugins/dssidebar/search_result_test.go`、
  新增四张 PNG（`plugins/dssidebar/testdata/`）、本文 `docs/visual-search-result.md`。
- 未改 `cmd/app`、`docs/progress.md`、`UIPLAN.md`、`official-specs.md`、其他插件、契约与快照；
  未 commit/push；未杀任何非本人启动的进程。
- 上游缺口口径：`docs/visual-v2.md` §5 第 3 条（"官方搜索结果卡未完整移植"）由本文收口。该文件不在本轮
  允许路径内，故未修改，特此注明。

## 1. 官方真值与本轮口径

原始 CSS / JSX 都取自本机安装包解出的官方包 `tmp/official/dsh-client-ui-workspace/client.js`
（模块 hash `wnFoaW` = `ui-workspace/src/client/rows/Rows.module.css`）：

```css
.wnFoaW_searchResultRow{box-sizing:border-box;border-radius:var(--dsw-radius-lg);cursor:pointer;
  text-align:left;width:100%;min-height:48px;color:var(--dsw-alias-label-primary);background:0 0;
  border:none;flex-direction:column;align-items:stretch;padding:4px 8px;display:flex}
.wnFoaW_searchResultRow:hover,.wnFoaW_searchResultRow.wnFoaW_selected{background:var(--dsw-alias-interactive-bg-hover)}
.wnFoaW_searchResultHeading{align-items:center;min-width:0;display:flex}
.wnFoaW_searchResultTitle{…;margin-left:4px;font-size:14px;line-height:20px;…}
.wnFoaW_searchResultMeta{align-items:center;gap:6px;min-width:0;margin-left:20px;display:flex}
.wnFoaW_searchResultWorkspace,.wnFoaW_searchResultSnippet{…;font-size:12px;line-height:17px;…}
.wnFoaW_searchResultWorkspace{max-width:40%;color:var(--dsw-alias-label-tertiary);flex:none}
.wnFoaW_searchResultWorkspace:only-child{max-width:100%}
.wnFoaW_searchResultSnippet{min-width:0;color:var(--dsw-alias-label-secondary);flex:1}
.GAkraW_searchTree>[role=treeitem]+[role=treeitem]{margin-top:2px}
.GAkraW_empty{color:var(--dsw-alias-label-tertiary);padding:16px 12px;font-size:13px}
```

```jsx
function SearchResultItem({result, currentId, onOpen, onUnarchive, t}) {
  const selected = result.id === currentId;
  return <button className={clsx(searchResultRow, selected && selected, result.archived && archived)}
                 onClick={() => onOpen(result.id)}>
    <div className={searchResultHeading}>
      {!result.archived && primaryStatus.state !== "idle" && <SessionStatusDots …/>}
      <span className={searchResultTitle}>{result.title || t("session.untitled")}</span>
    </div>
    <div className={searchResultMeta}>
      <span className={searchResultWorkspace}>{result.workspace || t("group.ungrouped")}</span>
      {result.snippet !== void 0 && <span className={searchResultSnippet}>{result.snippet}</span>}
    </div>
  </button>;
}
```

官方 zh 字典原文（同一包内）：`session.untitled`「未命名」、`group.ungrouped`「未分组」、
`search.noMatches`「无匹配会话」。卡片上的空值兜底照这三条来，不用分组态那套自造词。

## 2. snippet 真值查证（先查，再决定画不画）

| 查证对象 | 结果 |
|---|---|
| 官方数据面 | `deriveSearchResults` 里**只有 Host 全文检索结果页 `content.items`** 的项带 `snippet`；同一批**本地元信息命中**（标题 / 工作区名）的项**没有**该字段，官方 JSX 也是 `result.snippet !== void 0` 才渲染 |
| 本仓 Transport 客户端 | `internal/apiclient/client.go` 的 `Session` 只有 `ID/Title/Cwd/UpdatedAt/Mode`，无 `snippet` |
| 本仓快照 | `pluginapi.Snapshot` 字段为 `Sessions/Cur/Search/Status/Msgs/Total/Stream/Busy/Approval/Usage…`，无逐会话片段 |
| 宿主契约 | `../DSH-GO/docs/transport-contract.md` 与两份 `testdata/api.snapshot.json` grep 不到任何 `snippet`；本仓 `session.list` 消费路径也没有该字段 |
| 结论 | **没有 snippet 真值**：结果卡不画片段行、不留占位、绝不拿标题或 cwd 拼摘要。`searchResult.HasSnippet` 永远是 `false`，将来宿主提供时在 `searchResults` 里接入（唯一接入点，已在源码注释里写死这句话） |

渲染路径本身是通的并有测试（用合成真值验"有真值才画、没真值一个字都不补"），
生产路径上这个字段**只会是 false**——两者在 §4 的测试与 §6 的截图像素上分开举证。

## 3. 实现

| 项 | 行为 | 落地处 |
|---|---|---|
| 分流 | 过滤词非空 → 整条绘制路径换成平铺结果树（不画分组头、不折叠）；空 → 原分组树完全不变 | `sidebar.go drawList` |
| 卡片 | 48px 结果卡，整张卡是命中区域；悬停/选中底色同为 `t.Hover`（会话行同令牌） | `drawSearchCard`（容器复用 `prims.ListRow`，见 §5 偏差） |
| 两行 | 标题行 20（`14/20`，缩进 8+16+4）+ 元信息行 17（`margin-left:20`、`gap:6`、`12/17`） | 同上 |
| workspace | 由**真实 cwd** 推导（末段）；空 cwd → 官方 `group.ungrouped`「未分组」 | `searchResults` / `cardWorkspace` |
| snippet | 只有 `HasSnippet` 为真才画；本轮生产路径恒为 false（§2） | `drawSearchCard` |
| 结果顺序 | 与分组态同一比较器：`updatedAt` 新→旧，ID 稳定 tie-break；键盘顺序 = 绘制顺序 | `searchResults` / `newerSession` |
| 过滤 | 与分组态**共用** `matchesQuery`（标题或 cwd 包含，大小写不敏感），避免两条路漂移 | `matchesQuery` |
| 点击 / 选中 | 图元 ID 仍是 `s-<会话ID>`（与分组行同一套），`Clicked()` 原样把会话 ID 交给 `OpenSession`；`active = ID == env.Data.Cur` | `drawSearchCard` |
| 活动点 | 只有当前会话、且 `Busy` 或 `Stream` 有真值时点一个 `Infult` 6px 点；其余卡片一律留空（规则与分组行完全相同） | 同上 |
| 空态 | 官方 `search.noMatches`「无匹配会话」，`padding:16px 12px`、13px、`label-tertiary` | `drawSearch` |
| 时间 | 卡片上**不画时间**：官方 `searchResultRow` 没有时间元素（分组行的时间不受影响） | — |
| 取证日志 | `-sidebar-debug` 在搜索态打印每条卡片的 ID/标题/工作区/时间/片段真值/选中 | `logSearchSnapshot` |

## 4. 先红后绿

1) **行为红（旧实现可编译，必须失败）**：新增 `search_result_test.go` 的四个行为测试后，

```bash
CGO_ENABLED=0 go test ./plugins/dssidebar \
  -run 'TestSearchModeDraws48pxResultCards|TestSearchCardsShowRealWorkspaceWithoutInventedSnippet|TestSearchNoMatchesUsesOfficialEmptyText|TestSearchCardClickUsesRealSessionID' \
  -count=1 -v
```

**exit 1**，三条真实断言抓到旧实现：

```
第 0 张结果卡高度 = 32/32，官方 .wnFoaW_searchResultRow{min-height:48px}
"default-workspace" 的字号 = 14，官方是 12
没有命中时应显示官方 search.noMatches「无匹配会话」/13px，实际画了 [{没有匹配的会话 12}]
```

（第四个 `TestSearchCardClickUsesRealSessionID` 刚开始就是 PASS：它是"分组态那套 ID/点击不许回退"的
回归钉，不是本次要修的红。）

2) **规格红（新符号尚不存在）**：同一文件里补 `searchResults` / `drawSearchCard` / `searchCardBoxes`
的几何与数据面断言后，`CGO_ENABLED=0 go test ./plugins/dssidebar -run 'TestSearchResult' -count=1`
**exit 1**，报 `undefined: searchResults / searchCardBoxes`、`p.drawSearchCard undefined`。

3) **绿**：实现后

```bash
CGO_ENABLED=0 go test ./plugins/dssidebar -count=1      # exit 0，22 个顶层测试全过
go vet ./plugins/dssidebar                              # exit 0
CGO_ENABLED=0 go build ./...                            # exit 0
git diff --check -- plugins/dssidebar                   # exit 0
```

22 个测试里包含**未改一行的** `visual_v2_test.go`（`TestV2SearchUsesFieldAndNarrowsRealRows` 等 6 个）
和 `sidebar_test.go`（9 个）：搜索卡容器仍走 `prims.ListRow`，所以 V2 的"搜索后的实际绘制行"观测点、
几何常量（34/32/2/4）、选中/点击、活动点、时间隐藏全部原样通过。

新增的 7 个规格测试（`search_result_test.go`）：

| 测试 | 钉住的规格 |
|---|---|
| `TestSearchModeDraws48pxResultCards` | 搜索命中 3 条要画 3 张 **48px** 卡、宽 268、无分组头、卡间距 **2**、可见/键盘顺序 = 最近更新降序、选中只认 `Cur`、活动点只在当前卡 |
| `TestSearchCardsShowRealWorkspaceWithoutInventedSnippet` | 一帧里**画出来的字**去重后只能来自真值（3 条真标题 14px + 真实 cwd 的 workspace 12px），其余只允许省略号截断的前缀候选——多一个字就报错 |
| `TestSearchNoMatchesUsesOfficialEmptyText` | 无命中时是官方 `search.noMatches`「无匹配会话」/13px |
| `TestSearchCardClickUsesRealSessionID` | 真实命中区域点击 → `OpenSession("s-003")` |
| `TestSearchResultsKeepTruthOnly` | 顺序/过滤/空值兜底（"未命名"/"未分组"）；**每条结果的 `HasSnippet` 必须为 false**（契约缺口） |
| `TestSearchResultCardInnerGeometry` | 标题行 20、元信息行 17、标题左 28、元信息左 28、底部留白 ≥4、workspace 独子时上限 100%（`:only-child`）、与片段并排时上限 40% |
| `TestSearchCardSnippetRendersOnlyWithTruth` | 合成真值 → 片段按 12px/17px 画在工作区名右侧；`HasSnippet=false` → 一个字都不画 |

## 5. 无头抓图与命令

临时 mockd 只监听 **OS 分配**端口（`-addr 127.0.0.1:0`，本轮为 `127.0.0.1:56463`），
四张图完成后只回收本次启动的 job（已确认 `mockd-u3a.exe` 不再存在于进程表、端口无监听）。

```bash
# 构建：exit 0
CGO_ENABLED=0 go build -o /tmp/dssidebar-u3a/app-u3a.exe   ./cmd/app
CGO_ENABLED=0 go build -o /tmp/dssidebar-u3a/mockd-u3a.exe ./cmd/mockd

# 1920×1032 外框（客户区 1904×993）：exit 0
/tmp/dssidebar-u3a/app-u3a.exe -addr http://127.0.0.1:56463 -size 1904x993 \
  -session s-002 -sidebar-filter e \
  -shot plugins/dssidebar/testdata/visual-search-1920x1032.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug -title dsh-ui-u3a

# 1182×832 外框（客户区 1166×793）：exit 0
/tmp/dssidebar-u3a/app-u3a.exe -addr http://127.0.0.1:56463 -size 1166x793 \
  -session s-002 -sidebar-filter e \
  -shot plugins/dssidebar/testdata/visual-search-1182x832.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug -title dsh-ui-u3a

# 真标题命中（单卡）：exit 0
/tmp/dssidebar-u3a/app-u3a.exe -addr http://127.0.0.1:56463 -size 1166x793 \
  -session s-002 -sidebar-filter 插件 \
  -shot plugins/dssidebar/testdata/visual-search-title-1182x832.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug -title dsh-ui-u3a

# 分组态基线（不过滤，证"没回退"）：exit 0
/tmp/dssidebar-u3a/app-u3a.exe -addr http://127.0.0.1:56463 -size 1166x793 \
  -session s-001 -shot plugins/dssidebar/testdata/visual-search-grouped-1182x832.png \
  -shot-wait 2 -tail 5 -rightbar=false -sidebar-debug -title dsh-ui-u3a
```

`-sidebar-filter` 走的是与用户打字**完全相同**的路径（`Field.SetText → drawList`），不是另开分支。

抓图日志断言（`-sidebar-debug` 原文，证明卡片上的字都来自宿主快照）：

```
ds-sidebar: 搜索="e" 结果=3 会话=3 当前="s-002" busy=false 流=0
  |卡 "s-003" 标题="1000 条压力会话" 工作区="default-workspace" 时间="2026-11-03T06:00:00+08:00" 片段=none 选中=false
  |卡 "s-001" 标题="界面线 spike 讨论" 工作区="default-workspace" 时间="2026-11-03T05:40:00+08:00" 片段=none 选中=false
  |卡 "s-002" 标题="UI 插件化架构方案讨论" 工作区="default-workspace" 时间="2026-11-03T04:10:00+08:00" 片段=none 选中=true

ds-sidebar: 搜索="插件" 结果=1 会话=3 当前="s-002" busy=false 流=0
  |卡 "s-002" 标题="UI 插件化架构方案讨论" 工作区="default-workspace" 时间="2026-11-03T04:10:00+08:00" 片段=none 选中=true

ds-sidebar: 可见行=3 会话=3 过滤="" 组=1 当前="s-001" busy=false 流=0 全部收起=false
  |组 "D:/DeepSeek Harness/default-workspace"
  |行 "s-003" … |行 "s-001" … 选中=true |行 "s-002" …
```

### PNG 与 SHA-256

- [1920×1032 搜索三卡](../plugins/dssidebar/testdata/visual-search-1920x1032.png)，SHA-256 `ab2125c2e82fef4f92ef312175f11b941cc9ef2fb7c4c69e8b329a46cb37124b`
- [1182×832 搜索三卡](../plugins/dssidebar/testdata/visual-search-1182x832.png)，SHA-256 `82d2c6402a25bdf5b41990ee530f2825b28c3439a47337f4aa7e2ee8006144a9`
- [1182×832 真标题命中单卡](../plugins/dssidebar/testdata/visual-search-title-1182x832.png)，SHA-256 `f26e71c5a62307ce00e6464ecdb763bab7a49ecf36634586790b042617f191d5`
- [1182×832 分组态基线（未过滤）](../plugins/dssidebar/testdata/visual-search-grouped-1182x832.png)，SHA-256 `c72caa2e3c0797cef1fd0b50eb90bf66ee286adbb9a791d50718754dade9cd96`

## 6. 几何：设计值 vs 实测

**设计值** = 单元测试里真实 flex 矩形（`-run TestSearchResultCardInnerGeometry -v`，一条卡片的实测日志）：

```
design card={X:0 Y:0 Width:268 Height:48} heading={X:8 Y:5.5 Width:252 Height:20}
       meta.y=25.5 meta.h=17 title.x=28 meta.x=28 ws.w=232 底留白=5.5
```

**实测值** = 四张 PNG 的像素量取（`x0…x1`/`y0…y1` 含首尾，`w=x1-x0+1`）。

| 容器 / 几何 | 设计值（flex） | 实测值（PNG） | 官方 / 推导 | 偏差 |
|---|---|---|---|---|
| 侧栏列 | — | `x=8…287`，**w=280** | 280 | **0** |
| 搜索框 | 260×30 | 填充 `x=19…276, y=84…111` (258×28) ＋左右各 1px 描边 = **260×30** | 260×30（同 V2） | **0** |
| 结果卡高 | 48 | 选中卡 `y=230…277`，**48** | `min-height:48px` | **0** |
| 结果卡宽 | 268 | 选中卡 `x=20…287`，**268** | 268（与分组行同口径：侧栏 280 − 被 regionArea 吃掉的右侧 12） | **0** |
| 结果卡节距 / 间距 | 48 + 2 | 三张卡标题带 `y=140/190/240` → 节距 **50**，间距 **2** | 48 + `margin-top:2px` | **0 / 0** |
| 标题文字左缘 | 28（8+16+4） | ink `x=48…49`（卡左 20 + 28） | 28 | **≤1**（字形左侧留白） |
| 元信息左缘 | 28（8+20） | ink `x=48`（卡左 20 + 28） | 28 | **0** |
| 元信息右缘 | — | ink 到 `x=145`：只有 workspace 文本（`default-workspace` ≈ 97px），**右侧没有任何片段文本** | 无 snippet 时不画 | **0（无伪造）** |
| 标题行 / 元信息行高 | 20 / 17 | 文字 ink 高 12/12（字面），行盒由 flex 断言 | `line-height:20px` / `17px` | **0（设计值）** |
| 卡片内容顶距 | **5.5** | 标题 ink 顶距卡顶 10（= 5.5 + 14px 字面在 20px 行盒里的偏移） | 官方 `padding:4px` → 4 | **1.5**（ListRow 垂直居中） |
| 卡片底部留白 | **5.5** | — | 官方 `padding-bottom:4px` + `min-height` 余量 3 → 7 | **1.5** |
| 卡片圆角 | **12**（`prims.ListRow` 的 `Radii.MD`） | 角部像素剖面（顶部行内缩约 7px 含 AA、dy=3 行内缩 3px）与 r≈12 一致 | 官方 `--dsw-radius-lg` = **16** | **4** |
| 分组行（基线图，防回退） | 32 | 选中行 `y=200…231`，**32**；节距 34（32+2）；宽 268 | 32 / 34 / 268 | **0** |

结论：本轮**负责容器**（结果卡本体 48/268、卡间距 2、节距 50、内部两行 20/17 与左缘 28、
分组基线 32/34）实测偏差 **≤8px（实际为 0）**；两处设计值偏差（内容顶距 1.5、圆角 4）
同样 **≤8px**，成因与"未闭合"理由见 §7。

### 两尺寸一致

`1920×1032`（客户区 1904×993）与 `1182×832`（客户区 1166×793）两张三卡图的左侧几何读数**逐项相同**
（侧栏 280、搜索框 260×30、卡 48×268、节距 50、文字带 y=140/160/190/210/240/260）。侧栏定宽，不随窗口变。

## 7. 未验项与缺口（不写成"已对齐"）

1. **真宿主未验**：四张图都来自 mockd Transport 夹具（同一 workspace、三条会话）。没有用新 dshd 走真实
   `session.list` 验证真实标题/cwd/时间下的卡片像素；单元测试只证明消费与渲染语义，不替代 H 系列整链。
2. **snippet 是契约缺口（本轮如实隐藏）**：宿主没有该字段（§2），所以**没有任何现网截图能证明"片段行画对了"**；
   片段渲染只有单元测试（合成真值）。要真验，需要宿主先在 `/ws`/`session.list` 上给出全文检索结果页与 `snippet`。
3. **卡片圆角 12 vs 官方 16（4px）**：容器复用 `plugins/dscore` 的 `ListRow` 图元（`Radii.MD`，且
   `padding:0 8` + 垂直居中）。要对齐 `radius-lg(16)` / `padding:4px 8px` 必须改该图元或 `pluginapi.Primitives`
   契约签名——两者都不在本轮允许路径内，故保留并如实记录。
4. **卡片内容顶距 1.5px**：同上，来自 `ListRow` 的垂直居中；渲染观感一致，未越权改图元。
5. **未实现官方行内动作**：`rowActions`（悬停出现的行按钮）、`archived` 灰化 + 取消归档按钮、置顶指示
   ——宿主 `Session` 没有 `archived/pinned` 真值，官方这些元素也依赖 `result.archived` 等字段，故本轮不画。
6. **`search.hasMore` 未实现**：官方是"宿主全文检索分页上限"的提示（`仅显示前 {n} 条结果…`），
   我们没有结果上限，不适用也不假装有。
7. **卡片无时间**：官方 `searchResultRow` 里没有时间元素；分组行的时间列不受影响（`TestV2UnknownTimeIsHidden` 仍过）。
8. **逐会话活动仍是宿主缺口**：卡片上的点仍只用当前会话的 `Busy/Stream` 正向真值，别的卡片没有逐会话状态可画
   （与 `docs/visual-v2.md` §5 第 2 条同一口径，未新增能力）。
9. **原生输入/DPI/主题未验**：键盘顺序由 `lastOrder` 单测覆盖、点击由 canvas 事件覆盖，
   均**不证明** OS IME/真实鼠标命中/滚动条拖动；只测了当前深色主题、96 DPI 的两个窗口尺寸。
10. **数据面覆盖面**：mockd 只有"一个 workspace + 三条会话"，所以**没有留**多工作区、空 cwd 会话、
    超长标题/工作区名的截图；这些只有单元测试（空值兜底、`:only-child` 100% / 并排 40% 上限）。
11. **未跑**：`scripts/regress.sh`、`scripts/check.sh` 按纪律未在共享树执行（全仓 `go test ./... -count=1`
    本轮跑过一次，exit 0：含 `cmd/app`、`cmd/mockd`、`internal/*` 与全部插件包）。