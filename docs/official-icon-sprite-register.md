# 官方图标 sprite 登记（rc.2 `dsh-client-ui-primitives`）

> 本文件是**图标符号**的出处台账：每个在 Go 侧被复用的图标都必须能在这里查到
> `name / weight / viewBox / 显示尺寸 / 来源 file:line / sha256`。
>
> 规则来自主库 `$DSH_HOME/dsh-ui-official/structure/official-assets-reuse.md`
> （`M(name,weight)` + `<use>`；登记 `file:line`/sha256/viewBox；**不自造**）。
> 本文件只登记 **已经接进产品代码**的符号；素材库目录
> （`assets/icons/**`）仍然只读，未收录的符号按 rc.2 dist 原文转写并在此登记。

## 0. 取法与机器指纹

官方 rc.2 把图标做成一张 sprite 表，运行时生成器是：

```js
// dist: /dsh/node_modules/@deepseek-ai/dsh-web-frontend/dist/assets/index-ghOpC9uV.js
const Vw = { "goal-outline": { size: 16, svg: w => `<svg … viewBox="0 0 16 16" fill="none" stroke-width="${w}">…` }, … }
const p7 = { regular: 1, medium: 1.5? }            // 权重 → stroke-width
function Hw(name, weight) { return `dsw-icon-${name}-${weight}` }
function M(name, weight) {                          // 组件
  return ({ size = Vw[name].size, className }) =>
    <svg width={size*aspect} height={size} data-icon={Hw(name,weight)} aria-hidden="true">
      <use href={`#${Hw(name,weight)}`} />
    </svg>
}
```

- dist 文件：`1093231` B，sha256 `061de8cefa15e1fbb87d301463ad4ebb79259b197ca08e83406f6997c1af8bb8`
- 权重表：`regular = 1`（`p7={regular:1,…}`），即 `<svg stroke-width="1">`
- SVG 默认 `stroke-linecap: butt` / `stroke-linejoin: miter`（sprite 没写这两项）→
  Go 侧 `ui.Icon` 的 `Cap/Join` 必须留零值，**不要**改成 Round
- 显示尺寸由调用方按官方 `size` 传（`<svg height={size}>`），**viewBox 一律不改**

## 1. 已接线的符号

| sprite name | weight | viewBox | 显示尺寸 | 官方用处（file:line） | dist 位置 | sprite `svg(1)` sha256 | 素材库同物 |
|---|---|---|---|---|---|---|---|
| `goal-outline` | regular (1) | `0 0 16 16` | **14** | `dsh-client-ui-goal/src/client/GoalBar.tsx:137` → `<IconGoalOutlineRegular size={14} />`；CSS `.goalGlyph{color:var(--dsw-alias-label-tertiary);flex:none;display:inline-flex}` | `index-ghOpC9uV.js` 第 129 行 第 53572 列 | `0fe6266990c7ce022cc1245145ffab032cba1db98736b743222f33d945f63485` | **有**：`assets/icons/tools/goal.jsx`，sha256 `aca46bf4068391abddb67c70b4af3c84c12a46144e587de4361ca22f14584ea0`（4 段 path 与 sprite **逐字节相同**） |
| `check-outline` | regular (1) | `0 0 16 16` | **14** | primitives `ConnectionIndicator` 的 success 态（`ui-settings-general/src/client/SettingsRoot.tsx` 的 `triggerRow` 渲染它）；`Cs = M("check-outline","regular")` | 第 129 行 第 10644 列 | `41e32b03bef4130a030a4f4827a13e2e474e9c0fe8f9fe42dc7ca9ba437fc291` | **有**：`assets/icons/composer/check.jsx`，sha256 `3b47a8c21d5bf626867f2d6b45a6ff8b2f7362123cfa76de5a395a2b2916bb8c`（`d` 逐字节相同） |
| `refresh-outline` | regular (1) | `0 0 16 16` | **14** | 同上，disconnected 态（可点重连的按钮图标）；`Jw = M("refresh-outline","regular")` | 第 129 行 第 14896 列 | `06a5745f7fe0ceddf5ecfd9dfbd80f78e3927e395eacd9d9a20f0b66fb71eadb` | 否（库内暂无；按 sprite 原文转写，登记在此） |
| `pin-fill` | regular (1) | `0 0 16 16` | **14** | `ui-sidebar/src/client/SidebarRoot.tsx:113-120` → `<IconPinFillRegular size={14} />`（面板行尾「取消固定」按钮 `.panelUnpin`） | `index-ghOpC9uV.js` 第 129 行 第 69448 列 | `51c6115cc935c69bfad1836e0a51ad5b44cf58b595944bd088d1df2e7a95cc07` | 否（库内暂无；按 sprite 原文转写，登记在此） |
| `pin-outline` | regular (1) | `0 0 16 16` | **14** | `dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:57` → `<IconPinOutlineRegular size={14} />`（#17 左栏 More 菜单行尾的「固定」按钮 `.pinButton`）；`Cs = M("pin-outline","regular")` | `index-ghOpC9uV.js` 第 129 行 第 729837 列 | `3c4451475b12ae2996ab4f460734a6a2aed49c5581ac82c67c5fd56b44293b3d` | 否（库内暂无；按 sprite 原文转写，登记在此） |
| `plugin-pinwheel-outline` | regular (1) | `0 0 16 16` | **16**（菜单行 CSS 把它夹到 14×14） | `dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:62` → `<IconPluginPinwheelOutlineRegular size={16} />`（#17「浏览更多插件」行首） | `index-ghOpC9uV.js` 第 129 行 第 719755 列 | `1d2abb76e6d23882f1bdab5fd08124d2f1fb9e0821544d3b505dbb79e062f485` | **有**：`assets/official/icons/sidebar/plugin-pinwheel.jsx`，sha256 `29e4835f09ea781114691721eaa8fa074223d73c72ca3e2fc3bf73a17bca645d`（4 段 `d` 与 sprite 逐字节相同） |
| `plus-outline` | regular (1) | `0 0 16 16` | **16**（菜单行 CSS 把它夹到 14×14） | `dsh-client-ui-entry-customization/src/client/EntryMenuContent.tsx:66` → `<IconPlusOutlineRegular size={16} />`（#17「创建你的插件」行首） | `index-ghOpC9uV.js` 第 129 行 第 10417 列 | `792449e19144422e5899b2180eba870e401e31495ad89c52514085d112e8b280` | 否（库内暂无；按 sprite 原文转写，登记在此） |
| `ellipsis-outline` | regular (1) | `0 0 16 16` | **13** | `ui-conversation/src/client/skeleton/ConversationSession.tsx:37-43` → `<IconEllipsisOutlineRegular size={13}/>`（view tab 行 More 按钮） | `index-ghOpC9uV.js` 第 129 行 第 9854 列 | `fe08b1b020c452a8887c22c846ce995fce5f0e54cb2110f25a356886fc1ad1ad` | 否（库内暂无；按 sprite 原文转写，登记在此） |
| `skill-outline` | regular (1) | `0 0 17 17` | **14** | `dsh-client-ui-skill/src/client/SkillRow.tsx:78,105` → `<IconSkillOutlineRegular size={14}/>`（SkillRow 的 leading；preparing 态同一枚） | `index-ghOpC9uV.js` 第 129 行，字节 717236（`s.find('"skill-outline":{')`） | `88dd2473512b155d4756633dc8c14f4951b5e6384874556d6742fa0e78b80e7e` | 否（库内暂无；按 sprite 原文转写，登记在此。**混用 fill 与 stroke**：第 3 段是 `fill="currentColor"`，所以 Go 侧叠画两枚，见 `plugins/dsskill/icons.go` 文件头） |
| `chevron-down-outline` | regular (1) | `0 0 16 16` | **14** | `dsh-client-ui-skill/src/client/SkillRow.tsx:77,83` → `IconChevronDownOutlineRegular`（sprite 默认 size 就是 14）；CSS `SkillRow.module.css:42-43` 的 `.leading svg{width:14px;height:14px}` 也把它钉在 14 | 同上，字节 672557 | `8fabc0c6c62873ed5a14e8117a84c5016a896723f8cd06d75a84344641d6a120` | **有**：`assets/official/icons/composer/chevron-down.jsx:1`（`d` 逐字节相同） |
| `inspect-outline` | regular (1) | `0 0 16 16` | **12** | `dsh-client-ui-skill/src/client/SkillRow.tsx:165` → `<IconInspectOutlineRegular />`（TSX 未传 size，CSS 也没覆盖 → sprite 默认 12） | 同上，字节 716758 | `3cd41f1c24f29278bb7aac0da984657388e96fd915d6e0e156fa1e112323b798` | 否（库内暂无；按 sprite 原文转写，登记在此） |

> 上表三枚 `skill` 符号的 `svg sha256` 沿用 `plugins/dsskill/icons.go` 文件头的同一口径
> （那一轮的 recipe 没留存，本机按「`svg` 模板把 `${e}` 换成 1」复算对不上）；**可复算**的证据
> 是「dist 文件 sha256 `061de8ce…f8bb8` + 上述字节偏移 + path 逐字节比对」，
> 三条 path 数据已与 sprite 原文逐字节核对一致（2026-11-17）。

### 1.1 bundle 内联组件转写（不在 sprite 表里）

`ReferenceIconRegular`（dist 符号 `_b`）不是 sprite 表生成的：它在
`index-ghOpC9uV.js` 里直接定义了三个内联组件，由 `kind` 分派：

```js
function yb({kind,size=16,className,strokeWidth}) {
  switch (kind) {
    case "session": return jsx(eZ, …)
    case "file":    return jsx(rZ, …)
    case "folder":  return jsx(vb, …)
  }
}
function _b(e){ return jsx(yb, {...e, strokeWidth: 1}) }
```

| 官方组件 | kind | viewBox | stroke | 显示尺寸 | 调用点 | path sha256（逐段） |
|---|---|---|---|---|---|---|
| `eZ` | `session` | `0 0 16 16` | 1 | **14** | `ui-conversation/src/client/input/editor/ReferenceChip.tsx:32`、`ui-input-trigger/src/client/MenuView.tsx:182` | 3 段：`d8d07b41…` / `da43b38e…` / `8017877f…` |
| `rZ` | `file` | `0 0 16 16` | 1 | **14** | 同上 | 2 描边 `af2dd47d…`/`689eee81…` + 1 填充 `9dd7f8a6…`（`fill=currentColor`） |
| `vb` | `folder` | `0 0 16 16` | 1 | **14** | 同上 | 2 段：`5ec3a9f2…` / `8cfa9012…` |

go 侧：`internal/reficon/reficon.go`（`File/Folder/Session` 三个 Kind + `Icon()`）；
调用点：`plugins/dsconversation/usertext.go`（消息里的引用 chip）、
`plugins/dscomposer/trigger.go`（`@` 候选菜单）。

**H/V 命令**：三枚组件的 path 用到 `H`（水平线）与 `V`（垂直线），
所以 `plugins/dsbrand.ParseSVGPath` 从 M/L/C/Z 扩到 M/L/C/H/V/Z（绝对命令；
相对命令/弧线/二次曲线仍整条作废）。

## 2. 落地位置

| 符号 | Go 侧定义 | 调用点 |
|---|---|---|
| `goal-outline` | `plugins/dsgoal/icons.go`（`goalOutlineIcon`、`goalGlyphSize=14`） | `plugins/dsgoal/official_seat.go`（官方座位 `conversation.composer`）、`plugins/dsgoal/goal.go`（legacy 座位） |
| `check-outline` | `plugins/dsstatusbar/icons.go`（`checkOutlineIcon`、`connectionIconSize=14`） | `plugins/dsstatusbar/connection.go`（`ConnectionIndicator` 的 success 态） |
| `refresh-outline` | `plugins/dsstatusbar/icons.go`（`refreshOutlineIcon`） | 同上，warning 态 |
| `pin-fill` | `plugins/dssidebar/icons.go`（`pinFillIcon`） | `plugins/dssidebar/entry_menu.go`（面板行尾取消固定按钮） |
| `ellipsis-outline` | `plugins/dstopbar/icons.go`（`ellipsisIcon`） | `plugins/dstopbar/entry_menu.go`（view tab 行 More 按钮，13px） |
| `pin-outline` | `plugins/dsentrycustom/icons.go`（`pinOutlineIcon`，14px） | `plugins/dsentrycustom/entry_menu.go`（#17 左栏 More 菜单行尾「固定」按钮） |
| `plugin-pinwheel-outline` | `plugins/dsentrycustom/icons.go`（`pluginPinwheelIcon`，14px） | `plugins/dsentrycustom/entry_menu.go`（#17「浏览更多插件」行首） |
| `check-outline`（#17 复用） | `plugins/dsentrycustom/icons.go`（`checkOutlineIcon`） | `plugins/dsentrycustom/entry_menu.go`（#17 会话头 More 里 active view 的行尾对勾） |
| `plus-outline` | `plugins/dsentrycustom/icons.go`（`plusOutlineIcon`） | `plugins/dsentrycustom/entry_menu.go`（#17「创建你的插件」行首） |
| `skill-outline` | `plugins/dsskill/icons.go`（`skillGlyphStroke` 描边 3 段 + `skillGlyphFill` 填充 1 段，viewBox 17；`drawSkillGlyph` 用 `ui.Stack` 叠画成一枚） | `plugins/dsskill/skillrow.go` 的 `drawLeading`（`!expandable` 图标态、`expandable` 静止态）与 `drawPreparing`，显示 14 |
| `chevron-down-outline` | `plugins/dsskill/icons.go`（`chevronDownIcon`，坐标与 `assets/official/icons/composer/chevron-down.jsx:1` 相同） | `plugins/dsskill/skillrow.go` 的 `drawLeading`（**展开态唯一 leading**；可展开静止态作为悬停换图叠在图标上），显示 14 |
| `inspect-outline` | `plugins/dsskill/icons.go`（`inspectIcon`） | `plugins/dsskill/skillrow.go` 的 `drawInspectButton`（`.inspectButton` 内），显示 12 |

## 3. 断言（防回退）

| 断言 | 位置 |
|---|---|
| viewBox=16 / stroke=1 / 4 段 path / 首点 `M14.5001 8` / 显示尺寸 14 | `plugins/dsgoal/icons_test.go` |
| viewBox=16 / stroke=1 / 段数 / 首点 | `plugins/dsstatusbar/connection_test.go`（`TestConnectionIconsAreOfficialSprites`） |
| 目标条不再有 4×4 圆点矩形、确有描边图元 | `plugins/dsgoal/official_slot_test.go`、`plugins/dsgoal/goal_visual_test.go`、`cmd/app/hf_real_chain_test.go` |
| `pin-fill`：viewBox=16 / stroke=1 / 2 条 path（1 描边+2 填充层）/ 取消固定按钮真的改固定态 | `plugins/dssidebar/entry_menu_test.go`（`TestEntryMenuUnpinButtonRestoresPinnedPanel`） |
| `ellipsis-outline`：viewBox=16 / 3 条填充 path / More 按钮 22×22 图标 13px | `plugins/dstopbar/entry_menu_test.go`（`TestTopbarEntryMenuOpensAndDispatches`） |
| `pin-outline` / `plugin-pinwheel-outline` / `check-outline` / `plus-outline`：viewBox=16 / stroke=1 / pin-outline 是「1 描边 + 1 填充层」/ pinwheel 4 段曲线 / plus 2 条直线 | `plugins/dsentrycustom/entry_menu_test.go`（`TestIconSymbolsMatchRc2Sprite`） |
| #17 会话头 active 行真的多画一枚对勾（矢量图元数） | 同上（`TestConversationRowsShowEveryFoldedViewWithActiveMark`） |
| `skill-outline` / `chevron-down-outline` / `inspect-outline`：viewBox 17/17/16/16、stroke 1（skill 填充段 0）、段数 10/18/4/10、首点、Cap/Join 留零、显示尺寸 14/14/12 | `plugins/dsskill/icons_test.go`（`TestSkillIconsMatchRc2Sprite`、`TestSkillIconSizes`） |

## 4. 边界（不要违反）

1. **素材库只读**：`$DSH_HOME/dsh-ui-official/` 是主库，产品会话不往里写。库内没有的符号
   按 rc.2 sprite 原文转写，并在本文件登记 sha256。
2. **不改 viewBox**：显示尺寸靠调用方传官方 `size`。把 16 的坐标整体缩成 14
   会让描边宽度（view 单位 1）跟着错一小截。
3. **不自造 glyph**：官方没有的图形（例如连接指示的「未知」态）要么不画图标，
   要么在注释里显式标 `derived`，不许画一个"看起来像"的手搓 SVG。
4. 新增符号必须同步本节表格（name/weight/viewBox/size/file:line/sha256）。
