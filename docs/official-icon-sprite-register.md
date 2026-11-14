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

## 3. 断言（防回退）

| 断言 | 位置 |
|---|---|
| viewBox=16 / stroke=1 / 4 段 path / 首点 `M14.5001 8` / 显示尺寸 14 | `plugins/dsgoal/icons_test.go` |
| viewBox=16 / stroke=1 / 段数 / 首点 | `plugins/dsstatusbar/connection_test.go`（`TestConnectionIconsAreOfficialSprites`） |
| 目标条不再有 4×4 圆点矩形、确有描边图元 | `plugins/dsgoal/official_slot_test.go`、`plugins/dsgoal/goal_visual_test.go`、`cmd/app/hf_real_chain_test.go` |

## 4. 边界（不要违反）

1. **素材库只读**：`$DSH_HOME/dsh-ui-official/` 是主库，产品会话不往里写。库内没有的符号
   按 rc.2 sprite 原文转写，并在本文件登记 sha256。
2. **不改 viewBox**：显示尺寸靠调用方传官方 `size`。把 16 的坐标整体缩成 14
   会让描边宽度（view 单位 1）跟着错一小截。
3. **不自造 glyph**：官方没有的图形（例如连接指示的「未知」态）要么不画图标，
   要么在注释里显式标 `derived`，不许画一个"看起来像"的手搓 SVG。
4. 新增符号必须同步本节表格（name/weight/viewBox/size/file:line/sha256）。
