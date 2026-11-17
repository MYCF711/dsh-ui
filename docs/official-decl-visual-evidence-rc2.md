# 声明部件（`internal/uidecl` + `plugins/dsdecl`）的视觉依据

> 权威原文：官方 rc.2 随包 `src/**`（下面 `OFF:` 的锚点全部指向
> `tmp/GO8-RC2/official-rc2/dsh/node_modules/@deepseek-ai/`）。
> 本文只写**能给出 file:line 的东西**；给不出的，明写「无官方依据」。

## 1. 官方包名核实：**无**

在官方 rc.2 的 **337** 个 `@deepseek-ai/*` 包里，**没有任何**与「声明 / declaration /
disclosure」对等的包：

```
$ ls .../node_modules/@deepseek-ai/ | grep -iE 'declar|disclos|disclaim|consent|legal|notice|agreement'
（空）
```

`docs/official-parity.md` 记账的 66 个 `dsh-client-ui-*` 里也没有对等物
（`dsh-client-ui-plugin-manager` / `-plugin-market` / `-plugin-recommendations` 是
**插件市场**，不是运行时声明通道；`cordis-plugin-loader` 是 Node 侧插件加载器）。
官方也没有我们这种「一个 JSON = 一个只读部件」的槽位目录（`renderSlot` 的 100 个
槽位 id 里没有 `ui-plugins.d` 这一类）。

**结论：这条通道是我们的（`docs/plugin-api.md`），不存在「照官方包复刻」这条路。**
所以本文不谈"官方部件的数值"，只谈**官方对等控件**（一行只读 12px 文字）的数值——
也就是这个部件在官方设计语言里最接近的形状。

## 2. 对等控件的官方数值（逐条 file:line）

### 2.1 一行只读标签：22

`dsh-client-ui-agent-preset/src/client/AgentPresetLabel.module.css:3-18`

```css
.label { display:inline-flex; align-items:center; gap:4px; max-width:180px;
         padding:0 2px 0 0; height:22px; border-radius:var(--dsw-radius-xs);
         background:var(--dsw-alias-fill-tsp-secondary);
         font-size:12px; line-height:22px;
         color:var(--dsw-alias-label-tertiary); white-space:nowrap; overflow:hidden;
         text-overflow:ellipsis; }
```

注释原文：*"Session-header agent-preset label: static chrome, never a control."* ——
和声明部件「只画、不接交互」的定位一致，所以 `label` / `keyvalue` 的行高就取 **22**
（`internal/uidecl/decl.go` 的 `defaultHeight`）。

**但 22 不来自"侧栏底部那行"**：旧注释写「22 是侧栏底部那行（dssidebar.drawFooter）
的高度」——`dssidebar` 里根本没有 `drawFooter`（`grep -rn drawFooter plugins/dssidebar`
为空），那是同包 `dsactioncards` / `dsdirpicker` 的函数名。侧栏底部在官方是
`SidebarRoot.module.css:781-807` 的 `.footArea`：一列 `flex`、无高度、无内边距，
子块是 `sidebar.footer.action` 与 `sidebar.settings` 两个槽位——
**没有官方数值可抄**。旧注释已在本次改掉。

### 2.2 一行"点 + 文字"：20

`dsh-client-ui-chat/src/client/chat/StatsPills.module.css:6-28`

```css
.anchor { font-size: calc(var(--dsh-content-font-size-secondary, 13px) - 1px);
          line-height: calc(20px + var(--dsh-content-font-delta-secondary, 0px)); }   /* :9-11 → 12/20 */
.pill   { display:inline-flex; align-items:center; gap:6px; padding:1px 8px;
          border-radius:var(--dsw-radius-sm); background:transparent; ... }          /* :13-28 */
.pill svg { width:14px; height:14px; flex:none; }                                     /* :30-34 */
```

药丸的 1px 上下 padding 让 20 的行高落在 **22** 高的盒里——正好和 2.1 的标签行同高。
我们的 `dot` 行照这个来：行盒 22、行高 20、圆点 6、圆与字 gap 6。

### 2.3 状态圆点 6 / 圆与字 gap 6

- `dsh-experimental-client-ui-stats/src/client/stats.module.css:547-550`
  `.dot{display:inline-block;width:6px;height:6px;border-radius:50%;corner-shape:round;...}`
- `dsh-client-ui-settings-general/src/client/DesktopUpdateIndicator.module.css:47`
  `.errorDot{...width:6px;height:6px;border-radius:50%;corner-shape:round}`
- gap：`SidebarRoot.module.css:12` `--dsh-sidebar-label-gap:6px`；
  `StatsPills.module.css:16` `.pill{gap:6px}`

### 2.4 多行只读详情：行高 18、行距 2

`dsh-client-ui-deliverables/src/client/Deliverables.module.css:6,17`

```css
.hostStatus { display:flex; align-items:center; gap:8px; font-size:12px; line-height:18px;
              color:var(--dsw-alias-label-secondary); }     /* :6  一行只读状态 12/18 */
.details    { display:flex; flex:1; flex-direction:column; justify-content:center;
              gap:2px; min-width:0; }                        /* :17 两行堆叠 gap:2 */
```

同值的 caption 档在官方到处都是：`dsh-client-ui-shortcuts/src/client/Reference.module.css:192`
`.hint{font-size:12px;line-height:18px}`、`:215` `.inlineAction`…
所以我们 `list` 的行高 = 主题 `CaptionLine`（12/18，`plugins/dscore/dscore.go:130-131`），
行距 = **2**。旧实现的 18 是写死的常量、且**没给 LineHeight**，见第 3 节。

### 2.5 keyvalue 的 gap：8

`.hostStatus{gap:8px}`（同上 `:6`）、`.details` 与标题之间也用 8/12 一档。
我们的 `keyvalue`（左标签 + 右值）取 **8**，旧实现已经是 8，未改。

## 3. 改掉的数值（官方值 vs 旧值）

| # | 位置 | 旧值 | 官方值 | 依据 |
|---|---|---|---|---|
| 1 | `label` / `keyvalue` 的**文字行高** | 没给 → Viva 按 `fontSize+7` 排出 **19** | **22** | `AgentPresetLabel.module.css:9,13`（height:22px;line-height:22px；12px 字） |
| 2 | `dot` 的**文字行高** | 没给 → **19** | **20** | `StatsPills.module.css:9-11`（12px/20）+ `:19`（padding:1px 撑到 22） |
| 3 | `list` 的**文字行高 / 行盒** | 行盒写死 **18**、文字行高没给 → 文字实际 **19**（18 高的行盒里溢出 1px） | 文字行高 = **CaptionLine 18**，行盒同 18 | `Deliverables.module.css:6`（12/18） |
| 4 | `defaultHeight` 的来源注释 | 写「侧栏底部 drawFooter 的 22」（`dssidebar` 里没有这个函数） | 写官方 `.label` 的 22 | `AgentPresetLabel.module.css:3-18` |

没改：`defaultOrder` 100（我们的约定，官方无此概念）、`listLineSpacing` 2（本来就对）、
`keyvalue` gap 8（本来就对）、`dot` 圆点 6 与 gap 6（本来就对）、取色表与槽位表
（官方无对应概念，属于我们的契约）。

## 4. 图标

**没有可复用的官方素材，也没有手画。**

四种 kind 里只有 `dot` 画"图"，用的是 `pluginapi.Primitives.Dot`——
一个 `Width(size).Height(size).Radius(gfx.RadiusFull).Background(color)` 的圆
（`plugins/dscore/primitives.go:103-105`），**不是矢量图标**：
- 官方那两处 `.dot`（`stats.module.css:547`、`DesktopUpdateIndicator.module.css:47`）
  同样是 `width/height + border-radius:50%` 的纯 CSS 圆，不是 svg，
  所以官方的 **6px 圆**是数值对齐，不是素材复用（没有 path 可抄）；
- 其余三种 kind 是纯文字，没有图标。

本次**没有**新增任何 `dsassets` 资产、没有 `plugins/dsassets` 改动。

## 5. 证据（反证：去掉改动必须红）

新增 `internal/uidecl/visual_test.go`（真 `gfx.Canvas` + 真 `ui.Page`，命中区挂在
**文字本身**上量行盒）：

- `TestCaptionLineHeightsAreOfficial`：三种行高画出来必须 22/20/18。
  把 `captionText` 的 `LineHeight` 去掉后定向跑：

  ```
  --- FAIL: TestCaptionLineHeightsAreOfficial
      visual_test.go:107: label+keyvalue 行高（AgentPresetLabel.module.css:9,13）:
      实际 19，官方 22（差值 -3）—— 少了 LineHeight 时 Viva 按 fontSize+7 排出 19
  ```

  改回官方值后 `--- PASS`。
- `TestDrawnBlockHeightsAreOfficial`：三种撑开的 kind 整块高 22/22/58。
  **它不算行高证据**（外层行盒高度就是调用方给的 `d.height()`，量它只能证明常量没写错；
  第一版把命中区挂在外层固定高行盒上，删掉 LineHeight 照样绿——假绿已修掉）。
  它留着的价值是"非空内容必须至少落下一块"。

`plugins/dsdecl` 的 8 条测试没动（这条通道的装配/报错路径本来就不涉及几何）。
