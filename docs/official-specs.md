# 官方视觉规格（侧栏 + 底部状态栏）

> 本文件是「照抄官方数值」的**举证表**。每条都给：**原始 CSS 规则原文** → **所属官方包/文件** → **我们采用的数值**。
> 找不到的一律写「未找到」，并说明保持现状，绝不自己编一个数。
>
> 取证来源（都是本机真值，不是回忆）：
>
> | 代号 | 路径 | 说明 |
> |---|---|---|
> | `asar` | `D:/DeepSeek Harness/Desktop/resources/app.asar` | 安装态的 DSH 应用包（182 MB）。CSS Module 以 `const css = "…"` 内联在 client.js 里，可直接 grep 原文 |
> | `sidebar.pkg` | `tmp/official/dsh-client-ui-sidebar/client.js` | 官方 `@deepseek-ai/dsh-client-ui-sidebar` |
> | `layout.pkg` | `tmp/official/dsh-client-ui-layout/client.js` | 官方 `@deepseek-ai/dsh-client-ui-layout`（AppFrame） |
> | `index.css` | `tmp/official-index.css` | 打包后的 CSS（类名形如 `._heading_121p8_19`，中间 `121p8` 是模块 hash） |
> | `theme.js` | `tmp/official-theme.js` | 官方 `ui-theme` 包，含 `:root` 令牌定义 |
> | `roads` | asar 内的 `ui-workspace/src/client/rows/Rows.module.css` | 官方侧栏**会话行 / 项目行**，模块 hash `wnFoaW` |
> | `browser` | asar 内的 `ui-workspace/src/client/WorkspaceBrowser.module.css` | 官方会话浏览器（搜索框、分组容器） |
> | `pills` | asar 内的 `ui-chat/src/client/chat/StatsPills.module.css` | 官方统计胶囊（状态栏那一行数字的字号/间隔） |

---

## 0. 先纠正两条线索（都**不是**状态栏）

### 0.1 `._footer_4ub78_62` 不是状态栏，是**下拉菜单的 footer**

任务里给的线索是 `._footer_4ub78_62{flex:none;display:flex;flex-direction:column;margin-top:3px;padding-top:3px;border-top:.5px solid var(--dsw-alias-border-l2)}`。
模块 hash `4ub78` 的同一批规则里还有：

```css
._root_4ub78_1{position:relative;display:inline-flex}
._list_4ub78_7,._submenu_4ub78_8{box-sizing:border-box;padding:4px;display:flex;flex-direction:column;gap:0;border:0;--dsw-elevation-stroke-color: var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-prominent);…}
._list_4ub78_7{position:absolute;top:calc(100% + 4px);left:0;z-index:100;min-width:144px;max-width:360px}
._item_4ub78_90{display:flex;align-items:center;gap:6px;width:100%;min-height:34px;padding:6px 8px;border:none;border-radius:var(--dsw-radius-md);background:transparent;cursor:pointer;font-size:13px;line-height:20px;color:var(--dsw-alias-label-primary);text-align:left}
._label_4ub78_129{padding:6px 8px;font-size:11px;line-height:15px;color:var(--dsw-alias-label-tertiary)}
._separator_4ub78_80{height:.5px;margin:3px 2px;background:var(--dsw-alias-border-l2)}
```

`_submenu` / `_item` / `_separator` / `z-index:100` → 这是一个**右键菜单/下拉菜单**组件，
`_footer_4ub78_62` 是菜单底部那一条动作区。**不是底部状态栏。**
（出处：`index.css`，模块 `4ub78`，共 39 条规则。）

**结论：这条线索不能用作状态栏分隔线，未采纳为「状态栏」规格。**
其中唯一被复用的数值是那一条粗细与色阶（`.5px solid var(--dsw-alias-border-l2)`），
作为「整条边」那一档的参考见 §3。

### 0.2 `tmp/official-tokens.css` 与 `tmp/official-app.css` 都不是 DSH 的令牌

```
$ file tmp/official-tokens.css
tmp/official-tokens.css: ASCII text
$ head -c 120 tmp/official-tokens.css
/*
 * Copyright 2023 The Chromium Authors
 * …
```
两份都是 **Chrome DevTools** 的 `design_system_tokens.css` / `app.css`。
真正的 DSH 令牌在 `tmp/official-theme.js` 里（`ui-theme` 包的 `base.css`）。

**官方设计令牌（`theme.js` 原文）：**

```css
:root{--dsh-font-family-text-default:-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", …;
--ds-ease-in-out:cubic-bezier(.4, 0, .2, 1);--ds-transition-duration:.2s;
--ds-transition-duration-fast:.1s;--ds-transition-duration-slow:.3s;
--dsw-radius-xs:4px;--dsw-radius-sm:8px;--dsw-radius-md:12px;--dsw-radius-lg:16px;--dsw-radius-xl:20px;--dsw-radius-panel:28px}
```

**我们采用：** 与 `plugins/dscore/dscore.go` 的 `radii = {XS:4, SM:8, MD:12, LG:16, XL:20, Panel:28}` **逐项一致**，无需改动。

---

## 1. 官方侧栏外壳

**原文**（`sidebar.pkg`，`SidebarRoot.module.css`，类名前缀 `wtg9ga_`）：

```css
.wtg9ga_root{--dsh-sidebar-inline-padding:12px;height:100%;padding:6px var(--dsh-sidebar-inline-padding);box-sizing:border-box;background:var(--dsw-specific-sidebar-fill);color:var(--dsw-alias-label-primary);--dsh-scrollbar-thumb:var(--dsw-alias-scrollbar-bg-l2);--dsh-scrollbar-thumb-hover:var(--dsw-alias-scrollbar-hover-l2);flex-direction:column;font-size:14px;display:flex}
.wtg9ga_regionArea{min-height:0;margin-left:-4px;margin-right:calc(-1 * var(--dsh-sidebar-inline-padding));flex-direction:column;flex:1;padding-left:4px;display:flex;overflow:hidden}
.wtg9ga_footArea{flex-direction:column;flex:none;display:flex}
.wtg9ga_settingsArea,.wtg9ga_footerActions{flex:none;width:100%;min-width:0}
.wtg9ga_footerActions{display:flex}
.wtg9ga_iconButton{border-radius:var(--dsw-radius-sm);cursor:pointer;width:28px;height:28px;color:var(--dsw-alias-label-secondary);background:0 0;border:none;flex:none;justify-content:center;align-items:center;padding:0;display:inline-flex}
.wtg9ga_logoRow{…height:60px;margin-bottom:4px;padding:8px 0 8px 4px;…}
```

| 项 | 官方值 | 我们采用 | 落地处 |
|---|---|---|---|
| 侧栏内边距 | `padding:6px 12px` | `PadX(12).PadY(6)`（原来是四边 12） | `plugins/dsshell/shell.go` `drawSidebar` |
| 侧栏宽度 | `ui-layout/README.zh.md` 原文：「侧边栏宽度为 264～420px，默认为 280px，收起后保留 56px 控制栏」，且**定宽、不参与压缩** | `Width(280).Shrink(0)`——`Shrink(0)` 是这次补的，见 §1.1 | 同上 |
| 侧栏根字号 | `font-size:14px` | 行内各控件自定字号（未在侧栏根上设） | — |
| 侧栏底色 | `--dsw-specific-sidebar-fill` | `t.SidebarFill`（不变） | 同上 |

### 1.1 顺带修掉的一个真 bug：侧栏被压到 239，行底色横穿到会话正文上

修底部条时量像素量出来的（不是猜的）：

```
修前 tmp/base-1445.png   y=799: [(8,246,(27,27,28))  (247,759,(21,21,23)) …]
                                        ↑ 侧栏只有 239 宽，而不是 280
修前选中行             y=172: [(22,246,(45,45,46))  (247,273,(40,40,42)) …]
                                        ↑ 行背景一直探到 x=273，越过侧栏边界 26px
```

原因：三列是同一个 `Row` 的孩子，中栏 + 右栏一起超宽时 flex 按比例压缩，
把侧栏从 280 压到 239；而侧栏内部滚动区是按 280 算的（`280 - 24 = 256`），
于是**每一行都比侧栏宽 41px**——悬停/选中底色会压到会话正文上，圆角也看不出来。

修法：`f.Column().Width(sw).Shrink(0)`。侧栏是定宽的，不该参与压缩。实测修后：

```
修后 tmp/official-final.png  y=700: [(8,287,(27,27,28))  (288,778,(21,21,23)) …]   ← 8..287 = 280 ✓
修后选中行                   y=170: [(23,272,(45,45,46))  (274,287,(27,27,28)) …]  ← 行 20..276，留在侧栏内 ✓
```

---

## 2. 会话行 / 项目行（分组长）

**原文**（`roads` = `ui-workspace/src/client/rows/Rows.module.css`，hash `wnFoaW`；取自 asar）：

```css
.wnFoaW_projectRow,.wnFoaW_sessionRow{border-radius:var(--dsw-radius-md);padding:0 8px;cursor:pointer;user-select:none;color:var(--dsw-alias-label-primary);align-items:center;gap:6px;padding-inline-start:calc(8px + var(--dsh-workspace-indent,0px));display:flex}
.wnFoaW_projectRow:hover,.wnFoaW_sessionRow:hover,.wnFoaW_sessionRow.wnFoaW_selected{background:var(--dsw-alias-interactive-bg-hover)}
.wnFoaW_projectRow{box-sizing:border-box;align-items:center;height:34px}
.wnFoaW_projectRow .wnFoaW_rowActions{height:20px}
.wnFoaW_sessionRow{gap:0;height:32px}
.wnFoaW_sessionRow .wnFoaW_title{margin:0 6px 0 4px}
.wnFoaW_sessionRow .wnFoaW_title{flex:1}
.wnFoaW_title{text-overflow:ellipsis;white-space:nowrap;min-width:0;font-size:14px;line-height:20px;overflow:hidden}
.wnFoaW_meta{text-overflow:ellipsis;white-space:nowrap;color:var(--dsw-alias-label-tertiary);font-size:12px;line-height:20px;overflow:hidden}
.wnFoaW_time{color:var(--dsw-alias-label-tertiary);flex:none;font-size:10px;line-height:16px}
.wnFoaW_chevron{color:var(--dsw-alias-label-caption)}
.wnFoaW_arrow{transition:transform .15s var(--ds-ease-in-out)}
.wnFoaW_arrowOpen{transform:rotate(90deg)}
```

**缩进的来源**（asar 内 JSX 原文）：

```js
style: { "--dsh-workspace-indent": `${depth * 12}px` }
```

→ 每层缩进 **12px**（`--dsh-workspace-indent` 本身在 CSS 里没有定义，是 JS 内联设的）。

**分组之间的间距**（`browser` = `WorkspaceBrowser.module.css`）：

```css
.GAkraW_groupSection{position:relative}
.GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px}
.GAkraW_flatList>*+*{margin-top:2px}
.GAkraW_list{…padding-bottom:16px;scrollbar-gutter:stable;…}
```

| 项 | 官方值 | 我们采用 | 落地处 |
|---|---|---|---|
| 会话行高度 | `height:32px`（**固定**，不是 min-height） | `Height(32)` | `prims.ListRow` 调用处 `sidebar.go drawSession` |
| 会话行圆角 | `--dsw-radius-md` = 12 | `t.Radii.MD` | `prims.ListRow` |
| 会话行内边距 | `padding:0 8px` + `padding-inline-start:calc(8px + indent)` | `PadX(8).PadY(0)`，行内再补 12px 缩进块 | `prims.ListRow` + `drawSession` |
| 会话行 gap | `gap:0` | `Gap(0)` | `prims.ListRow` |
| 会话标题字号 | `font-size:14px;line-height:20px` | `FontSize(t.Fonts.Body)` = 14 | `drawSession` |
| 会话标题颜色 | `color:var(--dsw-alias-label-primary)`（**未选中不降级**） | `t.LabelPrimary`（原来未选中是 `LabelSecondary`） | `drawSession` |
| 相对时间 | `font-size:10px;line-height:16px;color:var(--dsw-alias-label-tertiary)` | `FontSize(10)` + `t.LabelTertiary` | `drawSession` |
| 悬停/选中底色 | **同为** `--dsw-alias-interactive-bg-hover` | `t.Hover`（原来选中用自造的 `NavActive`，已去掉） | `prims.ListRow` |
| 分组长（项目行）高度 | `height:34px` | `Height(34)`（原来 26） | `prims.SectionHeader` |
| 分组长圆角/内边距/gap | `radius-md` / `padding:0 8px` / `gap:6px` | `Radius(MD)` / `PadX(8)` / `Gap(6)` | `prims.SectionHeader` |
| 分组长标题 | `font-size:14px;color:var(--dsw-alias-label-primary)` | `FontSize(t.Fonts.Body)` + `t.LabelPrimary`（原来 12px 灰字） | `prims.SectionHeader` |
| 折叠箭头色 | `--dsw-alias-label-caption` | `t.LabelCaption` | `prims.SectionHeader` |
| 行间距 | `.GAkraW_flatList>*+*{margin-top:2px}` | `Gap(2)`（现状，保持不变） | `sidebar.go drawList` |
| 分组间距 | `.GAkraW_groupSection+.GAkraW_groupSection{margin-top:4px}` | **未实现**：列表整体用统一 `Gap(2)`，官方是「行间 2 / 组间 4」 | 见 §6 遗留 |
| 分组条数 | 未找到。官方项目行没有计数 | 沿用现有 12px/`label-tertiary`（取自 `.wnFoaW_meta`） | `prims.SectionHeader` |
| 会话行前导图标槽 | `.wnFoaW_slot{width:16px;height:20px;…}` | 未实现（我们没有图标） | — |

**实测核验**：截图里会话行节距（pitch）**34px** = 行高 32 + 行间距 2，与官方一致
（`tmp/official-final.png`，x=200 纵向色带：250→284→318→352→386→420…）。

---

## 3. 搜索框

**原文**（`browser` = `WorkspaceBrowser.module.css`）：

```css
.GAkraW_search{box-sizing:border-box;border-radius:var(--dsw-radius-sm);cursor:text;width:100%;height:28px;color:var(--dsw-alias-label-secondary);…background:0 0;border:none;…}
.GAkraW_searchExpanded{border:.5px solid var(--dsw-alias-border-l4);border-radius:var(--dsw-radius-md);width:calc(100% + 4px);height:30px;color:var(--dsw-alias-label-caption);background:0 0;margin-inline:-2px;padding:0 4px 0 0}
.GAkraW_searchButton{border-radius:var(--dsw-radius-sm);cursor:pointer;width:28px;height:28px;color:inherit;background:0 0;border:none;flex:none;justify-content:center;align-items:center;padding:0;display:inline-flex}
```

以及输入框本身：

```css
.GAkraW_searchInput{…font-size:13px;line-height:18px;color:var(--dsw-alias-label-primary)}
.GAkraW_searchInput::placeholder{color:var(--dsw-alias-label-tertiary)}
```

| 项 | 官方值 | 我们采用 | 落地处 |
|---|---|---|---|
| 搜索框高度 | `height:30px`（展开态） | `Height(30)`（原来是 viva `.Small()` 的 28） | `sidebar.go drawHeader` |
| 搜索框圆角 | `--dsw-radius-md` = 12 | `Radius(t.Radii.MD)` | 同上 |
| 描边 | `.5px solid var(--dsw-alias-border-l4)` | `Border(0.5, t.BorderL4)` —— **实际被 viva 覆盖成 1px**，见下 | 同上 |
| 字号 | `font-size:13px;line-height:18px` | `FontSize(t.Fonts.Secondary)` = 13 | 同上 |
| 文字色 / placeholder | `label-primary` / `label-tertiary` | viva 组件自带（`theme.Foreground` / `theme.MutedForeground`） | — |

**实测核验**：截图里搜索框矩形 y = 83…112 → **高 30px**，上下各 1px 描边（像素值 `(58,58,60)`），
内部填充 `(32,32,34)`。

**未能对齐的一处（如实记录）**：viva 的 `components.PaintInput` 在 `paintTextEntrySurface` 里
**自己重画**了一圈 1px 描边：

```go
// viva@v0.2.3/components/input.go:305
func paintTextEntrySurface(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, state textEntryState) {
	border, fill := textEntryTokens(theme, state)
	radius := theme.Metrics.RadiusControl
	…
	canvas.RoundedBorder(bounds, radius, 1, border.Over(theme.Background), fill.Over(theme.Background))
}
```
`Box.Border(.5, …)` 在 `flex/paint.go:32` 先画了一遍，随后被这一层 1px 盖掉。
所以**描边粗细是 1px，不是官方的 .5px**。修它要改 `plugins/dscore/dscore.go` 的
`VivaTheme()`（不在本次允许改动的文件里），或改 viva 上游。

---

## 4. 侧栏 footer 那一行

**原文**（`sidebar.pkg`）：

```css
.wtg9ga_footArea{flex-direction:column;flex:none;display:flex}
.wtg9ga_settingsArea,.wtg9ga_footerActions{flex:none;width:100%;min-width:0}
.wtg9ga_footerActions{display:flex}
.wtg9ga_iconButton{border-radius:var(--dsw-radius-sm);width:28px;height:28px;color:var(--dsw-alias-label-secondary);background:0 0;border:none;flex:none;justify-content:center;align-items:center;padding:0;display:inline-flex}
```

| 项 | 官方值 | 我们采用 | 落地处 |
|---|---|---|---|
| 整行宽度 | `width:100%;min-width:0` | `FillWidth()`（原来是 `WidthPercent(100)`） | `sidebar.go drawFooter` |
| 行高 | **未找到**（官方 footArea 不设高度，高度由里面的 28px 图标按钮撑） | `Height(28)`，取图标按钮那一档 | 同上 |
| 字号 | **未找到**（官方 footer 里没有文本，只有按钮） | 沿用原 `Fonts.Tiny`（不另编） | 同上 |

---

## 5. 底部状态栏

### 5.1 它是「整行」——这一条是真值，而且是本次修 bug 的依据

**原文**（`layout.pkg`，`AppFrame.module.css`，hash `mP3x-q`）：

```css
.mP3x-q_frame{background:var(--dsw-alias-bg-base);grid-template-rows:minmax(0,1fr) auto;height:100%;display:grid;position:relative;overflow:hidden}
.mP3x-q_bottomRow{background:var(--dsw-alias-bg-base);grid-column:1/-1;min-width:0;min-height:0;overflow:auto}
```

**官方 JSX 原文**（asar 内 `AppFrame.js`）：

```jsx
(0, react_jsx_runtime.jsx)("div", {
    className: AppFrame_module_css_default.bottomRow,
    "data-shell-bottom": true,
    children: renderSlot("shell.bottom", {})
}),
```

**官方 slot 文档原文**（asar 内 `ui-layout/README.md`）：

> The single root-scoped `shell.bottom` slot spans the full width below the three columns.
> Its occupant controls its own height and visibility; the columns shrink by that height,
> and empty content reserves no space.

**官方默认不占用这个 slot**（同一份 README）：

> 默认不占用此 slot。

只有实验性的 inspector 包会占用它（asar 内 `ctx.slots.register({name: "shell.bottom", …}, InspectorPage)`）。

| 项 | 官方值 | 我们采用 | 落地处 |
|---|---|---|---|
| 底部条跨度 | `grid-column:1/-1`（横跨整行，从窗口左边到右边） | `f.Column().FillWidth().Height(h)` 放在外层 `Column` 的第二行 | `dsshell/shell.go drawBottom` |
| 底部条底色 | `background:var(--dsw-alias-bg-base)` | `Background(t.BgBase)`（原来是 `SidebarFill`，比 bg-base 深一档，所以右侧看着比左侧深） | `shell.go drawBottom` + `prims.StatusBarRow` |
| 高度 | **未找到**（slot 文档：「Its occupant controls its own height and visibility」；官方 CSS 不设高度） | 用壳给的 `env.Geometry.BottomHeight`（=24，来自 `cmd/app/main.go`，本次不许改） | `statusbar.go draw` |
| padding | **未找到**（bottomRow 不设 padding） | 左内边距 12，取 `--dsh-sidebar-inline-padding:12px`，让状态栏文字与侧栏文字左对齐 | `prims.StatusBarRow` |
| 字号 | **未找到专门的「状态栏」模块**；取最接近的官方统计行 `pills`：<br>`font-size:calc(var(--dsh-content-font-size-secondary,13px) - 1px)` = **12px** | `t.Fonts.Caption` = 12 | `statusbar.go` |
| 文字色 | `pills`：`color:var(--dsw-alias-label-tertiary)` | `t.LabelTertiary`（原来是 `LabelCaption`） | `statusbar.go` |
| 段间间隔 | `pills`：`.hNGZaa_pill{…align-items:center;gap:6px;padding:1px 8px}` / `.hNGZaa_sep{color:var(--dsw-alias-separator-primary);margin:0 6px}` | 行 `Gap(6)`；`·` 分隔符 `PadX(6)` + `t.LabelDimmed` | `prims.StatusBarRow` / `statusbar.go sep` |
| 分隔符颜色 | `--dsw-alias-separator-primary` —— **在打包产物里只有引用，没有定义**（asar 内 grep 不到赋值） | 退到现有的最弱一档文字色 `t.LabelDimmed` | `statusbar.go sep` |
| 上边分隔线 | **未找到**（`bottomRow` 没有 border-top）。`._footer_4ub78_62` 的那条 `.5px solid var(--dsw-alias-border-l2)` 属于**菜单**，见 §0.1 | `Height(0.5)` + `t.BorderL2`，只借用了粗细与色阶 | `prims.Hairline` |

**实测核验**（`tmp/official-final.png`，y 扫描）：

```
y=799  [(8,246,(27,27,28))  (247,759,(21,21,23))  (760,1187,(27,27,28))]   ← 三列（侧栏/中栏/右栏）
y=809  [(8,1187,(21,21,23))]                                             ← 底部条：整行 bg-base，无断口
x=20   … (791,806,sidebar) (808,830,(21,21,23)) (831,838,black)          ← 侧栏在 806 结束，底部条 808 起，不重叠
```

修前（`tmp/base-1445.png`，同一段扫描）：

```
y=809  [(8,597,(21,21,23)) (598,1187,(27,27,28))]   ← 状态栏背景只从 x=598 开始，左侧 590px 是空的
```

---

## 6. 遗留 / 明确没做到的

1. **搜索框描边粗细** 是 1px，官方 `.5px`。原因是 viva `components.PaintInput` 硬编码 1px（§3）。
   修它要动 `plugins/dscore/dscore.go` 的 `VivaTheme()`，不在本次允许改动的文件里。
   圆角、高度、字号**已经对齐**（`radius-md`、30px、13px，实测通过）。
2. **分组间距**：官方是「行间 2 / 组间 4」两档，我们列表整体用的是统一的 `Gap(2)`。
3. **`--dsw-alias-separator-primary`** 在打包产物里只有引用没有定义，取值只能猜；
   本次退回已有的 `LabelDimmed`。
4. **状态栏高度 24** 来自 `cmd/app/main.go` 的 `Geometry.BottomHeight`，官方没有对应真值，
   本次按「不许改 main.go」保持不动。
5. **分组长右侧计数** 官方项目行没有这个元素，本次沿用取自 `.wnFoaW_meta` 的 12px/`label-tertiary`。
6. 会话行**前导图标槽**（`.wnFoaW_slot`，16×20）我们本来就没有图标，未实现。
7. **状态栏「官方对应物」本身是空的**：官方 `shell.bottom` 默认不占用（只有实验性 inspector 插件会用），
   所以「状态栏的内容长什么样」在官方库里**没有直接真值**，
   §5 里的字号/间隔/颜色是从 `StatsPills` 借的最近似的那一档，已在表里逐条标注来源。
