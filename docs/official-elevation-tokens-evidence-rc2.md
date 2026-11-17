# 官方阴影 / elevation 令牌取证（rc.2 基线）

> 起因：`vc-dscore` 审计发现的一处**系统性**视觉差异，不是单点。
> 官方把阴影写成 7 个显式令牌，我们一个字段都没有，20+ 个插件各自拿
> viva 的 Tailwind 预设顶上——**viva 的档位和官方档位不是同一套数**，
> 所以每一张浮层的阴影都不等于官方那一张。
>
> 本文件是取值取证 + 当时改了什么 + 还剩什么。代码：
> `pluginapi/theme.go`（令牌类型）、`plugins/dscore/shadow.go`（取值与画法）。

## 1. 官方四档阴影（`--dsw-shadow-*`）

出处：`@deepseek-ai/dsh-client-ui-theme/src/styles/gradient-shadow-text.css`
（官方 rc.2 随包发 `src/**`）。

| 令牌 | 官方值 | 行 |
|---|---|---|
| `--dsw-shadow-lv1` | `0 2px 4px 0 rgba(0,0,0,0.05)` | `:5` |
| `--dsw-shadow-lv1-blur` | `0 4px 12px 0 rgba(0,0,0,0.02)` | `:6` |
| `--dsw-shadow-lv2` | `0 4px 12px 0 rgba(0,0,0,0.02), 0 2px 8px 0 rgba(0,0,0,0.04)` | `:7` |
| `--dsw-shadow-lv3` | `0 0 1px 0 rgba(0,0,0,0.2), 0 0 4px 0 rgba(0,0,0,0.02), 0 12px 32px 0 rgba(0,0,0,0.08)` | `:8-9` |

注意 `lv3` 是**三层**、`lv1-blur` 与 `lv1` 只差模糊和透明度——这不是一条可以
"取最接近的一档"的阶梯。

## 2. 官方 elevation 三件套（`--dsw-elevation-*`）

同一文件，`body, body *` 里逐元素声明（`:31-40`），描边是**画在 box-shadow
里的一条 0.5px 发丝环**（组件配 `border: 0`，不占布局，注释见 `:10-16`）：

```text
:33  --dsw-elevation-stroke:      0 0 0 0.5px var(--dsw-elevation-stroke-color)
:34-35 --dsw-elevation-panel:     stroke, 0 3px 8px 0 rgba(0,0,0,0.03), 0 0 16px 0 rgba(0,0,0,0.02)
:36-37 --dsw-elevation-prominent: stroke, 0 3px 8px 0 rgba(0,0,0,0.04), 0 0 20px 0 rgba(0,0,0,0.05)
:38-39 --dsw-elevation-soft:      stroke, 0 4px 16px 0 rgba(0,0,0,0.03), 0 0 24px 0 rgba(0,0,0,0.03)
```

描边**颜色**是间接的，官方让各组件重绑 `--dsw-elevation-stroke-color`：

| 场景 | 值 | 出处 |
|---|---|---|
| 默认 | `var(--dsw-alias-border-l4)` | `gradient-shadow-text.css:17` |
| 亮色 `border-l4` | `rgba(0,0,0,0.16)` | `design-platform.css:199` |
| 暗色 `border-l4` | `rgba(255,255,255,0.2)` | `design-platform.css:351` |
| 菜单面（暗） | `var(--dsw-alias-border-l3)` | `gradient-shadow-text.css:23-25`（仅 `[data-menu-material]`） |
| 命令面板 | `var(--dsw-alias-border-l1)` | `dsh-client-ui-commands/src/client/PopupSelectView.module.css:28` |
| dock 浮动面板 | `var(--dsw-alias-border-l2)` | `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 的 `._float_123xe_158` |

**所以描边色不在令牌里**：`pluginapi.Elevation` 只带宽度与柔光层，颜色由调用
方按官方在那一处的重绑传进去（`dscore.PaintElevation(..., stroke)`）。

## 3. 深色主题没有第二套阴影值（如实标注，不猜）

全官方包 `grep -- "--dsw-shadow"` 只命中 `gradient-shadow-text.css:5-8`；
三件套的派生值也只在 `:31-40` 的 `body, body *` 里声明一次。
`body[data-ds-dark-theme]` 的两段（`:23-25`、`:42-45`）只改
**菜单面的描边色**与**线性渐变文本**，没有任何阴影数值覆盖。

结论：**浅/暗两套阴影数值相同**，随主题变的只有描边色。
`TestShadowsHaveNoDarkOverride` 把这个事实钉住了——哪天有人给暗色"调深一点"，
它会红，那就得先拿出官方证据。

## 4. 不能用 viva 预设代替（差异逐档）

viva `@v0.6.4/gfx/canvas.go:117-129` 的四档全部是 Tailwind 阶梯，
`ShadowMD`/`ShadowXS`/`ShadowLG` 与官方档位**没有一一对应**：

| viva | 值 | 官方哪一档 | 差异 |
|---|---|---|---|
| `ShadowXS` `canvas.go:121` | `0 1px 2px 0 rgba(0,0,0,.05)` | `--dsw-shadow-lv1` `0 2px 4px .05` | 位移 1→2、模糊 2→4 |
| `ShadowMD` `canvas.go:123` | `0 4px 6px -1px .10, 0 2px 4px -2px .10` | `--dsw-shadow-lv2` `0 4px 12px .02, 0 2px 8px .04` | 模糊 6→12、**负扩展**官方没有、透明度 .10→.02/.04 |
| `ShadowLG` `canvas.go:124` | `0 10px 15px -3px .10, 0 4px 6px -4px .10` | `--dsw-shadow-lv3` `0 0 1px .2, 0 0 4px .02, 0 12px 32px .08` | 官方**三层**、第一层无偏移；viva 只有两层 |
| 无 | — | `--dsw-shadow-lv1-blur` | viva 完全没有这一档 |
| 无 | — | 三件套（描边 + 双柔光） | viva 预设**没有描边层**，也没有 3 层容量 |

viva 的单层原语 `Canvas.Shadow(bounds, radius, ShadowStyle, color)`
（`canvas.go:1157`）够用——`ShadowStyle` 就是 `box-shadow` 的四个参数，
官方每一层都能逐层照抄，只是**层数与档位得自己拿到**。这也是这次加令牌
而不是"选一个最像的预设"的理由。

## 5. 本轮改了什么（分片：只动 `pluginapi`、`plugins/dscore` 与两个消费方）

| 位置 | 改动 |
|---|---|
| `pluginapi/theme.go` | 新增 `ShadowLayer` / `Elevation` / `Shadows` 三个类型与 `Theme.Shadows` 字段 |
| `internal/pluginapi/alias.go` | 三个类型的别名（漏一行就是"插件编译得动、壳编译不动"） |
| `plugins/dscore/shadow.go`（新） | 7 档取值（逐条 file:line）+ `PaintShadow` / `PaintElevation` 与两个 Overlay 变体 |
| `plugins/dscore/dscore.go` | `Light()` / `Dark()` 都装 `Shadows: shadows()` |
| `plugins/dscommands/panel.go` | 命令面板阴影 → 官方 `--dsw-elevation-prominent` + border-l1 描边 |
| `plugins/dsnotifications/card.go` | 通知卡片的本地 elevation 常量与 `drawPanelElevation` → 共用 `Shadows.Panel` |

### 消费方 原值 → 新值

**`plugins/dscommands/panel.go`（命令面板）**

- 原值：`Shadow(gfx.ShadowMD)`（`panel.go:58` 旧行）= Tailwind
  `0 4px 6px -1px rgba(0,0,0,.10)` + `0 2px 4px -2px rgba(0,0,0,.10)`，
  没有描边层。
- 新值：官方 `PopupSelectView.module.css:28-29` 的
  `--dsw-elevation-stroke-color: var(--dsw-alias-border-l1)` +
  `box-shadow: var(--dsw-elevation-prominent)`
  = `0.5px border-l1 环` + `0 3px 8px 0 rgba(0,0,0,.04)` + `0 0 20px 0 rgba(0,0,0,.05)`。

**`plugins/dsnotifications/card.go`（通知卡片）**

- 原值：本包自己写的一份 elevation-panel——`PanelStroke 0.5`、
  `PanelShadowY 3` / `PanelShadowBlur 8` / `PanelShadowAlpha 0.03`、
  `PanelGlowBlur 16` / `PanelGlowAlpha 0.02`，加一个 `drawPanelElevation`。
  **数值本来就是官方值**（`AccountNotifications.module.css:4` 的
  `box-shadow:var(--dsw-elevation-panel)`），问题是它是**第 N 份副本**。
- 新值：`dscore.PaintElevation(d, CardRadius, t.Shadows.Panel, t.BorderL4)`
  = `0.5px border-l4 环` + `0 3px 8px 0 rgba(0,0,0,.03)` + `0 0 16px 0 rgba(0,0,0,.02)`。
  描边色不重绑，走官方默认档（`gradient-shadow-text.css:17`）。

## 6. 顺着改发现的两个 viva 事实（都会让"看起来改了其实没画")

1. **带 `.Clip()` 的元素，它的 `Draw` 钩子被一起裁掉**：
   `ui/paint.go:50-52` 先 `canvas.Clip(r)`，`:132` 才跑 `hooks.under`。
   阴影记进 canvas 时带着那道裁剪（`gfx/canvas.go:1169` 的 `clipSnapshot`），
   而阴影又被自己的投射矩形挖空（`canvas.go:78-80`），两者相减是**空集**。
   → 命令面板原来那两行 `gfx.ShadowMD` 其实一直等于没画（改前实测
   `hasClip=true, clip==bounds`）。现在它的阴影在 `AfterDraw` 里画：
   那个阶段是"布局完成后覆画"（`pluginapi/plugin.go:286-292`），裁剪栈已清空。
   `plugins/dscommands/panel_elevation_test.go` 断言 `HasClip == false`。
2. **`Blur == 0` 的层画不出来**：`canvas.go:1158` 直接 return。官方的
   `0 0 0 0.5px` 发丝描边正好就是这种层，所以令牌把它单列成
   `Elevation.Stroke`，画法用 `RoundedOutline` 描外扩 0.5px 的环。
   副作用：`pixelScale > 0` 时 `RoundedOutline` 会走 `borderWidth()`
   把宽度吸附到整设备像素（`gfx/rounded_outline.go:11-14`），
   100% 缩放下 0.5px 环可能画成 ~1px；这是 viva 的限制，不是令牌的偏差。

## 7. 剩余待替换（**本轮只列不改**，下一轮分批）

### 7.1 还在用 viva 预设的（13 处）

| 插件 | 位置 | 现值 | 官方目标（有证据的已标） |
|---|---|---|---|
| dsagentteam | `catalog.go:433` | `ShadowMD` | 卡片面，待对官方包取证 |
| dsagentteam | `commandrow.go:172` | `var _ = gfx.ShadowMD`（占位引用，无实际绘制） | 可直接删 |
| dsagentteam | `confirm.go:74` | `ShadowMD` | 确认弹窗 → `prominent`（见下方「弹窗」注） |
| dscomposer | `composer.go:289` | `ShadowXS` | 待对官方包取证（输入面多半是 `soft`） |
| dsconversation | `feedback.go:321` | `ShadowMD` | 反馈弹窗 → `prominent`（同上） |
| dscordis | `panel.go:215` | `ShadowMD` + 自画 1px `border-l1` | `dsh-client-ui-cordis` 用 `elevation-prominent`（描边 0.5px） |
| dsjobs | `official_jobs.go:258` | `ShadowMD` | `dsh-client-ui-jobs` 用 `elevation-prominent` |
| dssettings | `official_panel.go:538` | `ShadowXS` | `dsh-client-ui-settings-general` 用 `elevation-prominent` |
| dssettings | `settings.go:424` | `ShadowXS` | 同上 |
| dsshortcuts | `shortcuts.go:199` | `ShadowLG` | 快捷键面板，待取证 |
| dssidebar | `workspace_actions.go:260` | `ShadowMD` | 工作区弹窗 → `prominent`（同上） |
| dsstatusbar | `official_stats_dialog.go:232` | `ShadowLG` | 统计弹窗 → `prominent`（同上） |
| dsterminal | `guide.go:295` | `ShadowLG` | 引导面板，待取证 |

> **弹窗（注）**：官方的模态框住在打包产物里，没有独立 `src/`：
> `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 的
> `._dialog_1ytzq_33{…box-shadow:var(--dsw-elevation-prominent)}`。
> 所以"确认/工作区/反馈/统计这些模态框改用 prominent"是有出处的，
> 不是按"看着像"推的；但那几条仍要逐个核对自己是不是**同一个**组件。

`plugins/dsshortcuts/official_geometry.go:18-19` 的注释已经点明这一条：
"`gfx.ShadowXS..ShadowLG` 五档预设（`ui/box.go:299-304`），表达不出自定义层"。

### 7.2 手抄了同一套 elevation 数值的（5 处，改来源不改数值）

| 插件 | 位置 | 手抄的是哪一档 | 本地常量 |
|---|---|---|---|
| dsattachment | `rail.go:878-889` | `panel` | `arrowStroke`、`arrowShadowY/Blur/Alpha`、`arrowGlowBlur/Alpha` |
| dscontextmenu | `context_menu.go:360-362` | `prominent` | `menuShadowOffsetY/Blur/Alpha`、`menuGlowBlur/Alpha`、`menuStroke` |
| dsdirpicker | `browse.go:1231-1234` | `prominent` | `modalElevationStroke`、`modalElevationShadowY/B/A`、`modalElevationGlowB/A` |
| dsgoal | `official_seat.go:185-187` | `panel` | `goalPanelShadow*`、`goalPanelGlow*` |
| dsquestions | `plan_review.go:213-215` | `panel` | `planReviewPanelShadow*`、`planReviewPanelGlow*` |

这 5 处的数值与官方一致，属于"第 N 份副本"——正是这次系统化要消掉的东西，
但每个都在别人的写作用域里，本轮不动。

### 7.3 官方哪一种浮层用哪一档（下一轮分批的依据）

按官方各包 `src/**/*.css` 的 `box-shadow:` 统计：

| 官方令牌 | 用它的官方包 |
|---|---|
| `elevation-panel` | account-notifications、attachment、trajectory、user-questions、approval（另带一条 `shadow-lv2`）、chat（2 处）、conversation、goal |
| `elevation-prominent` | commands、cordis、jobs、schedule（3 处）、sidebar-files（2 处）、sidebar-documentpreview（2 处）、settings-general、model-selection、plugin-manager、subagent、input-trigger、chat、conversation |
| `elevation-soft` | conversation（1 处） |
| `shadow-lv3` | 悬停卡 `._card_l6gf9_9`、toast `._toast_af3m1_6`（都在 `dsh-web-frontend` 的 bundle 里） |
| `shadow-lv2` | approval（1 处） |

## 8. 验证

- `plugins/dscore/shadow_test.go`：7 档取值逐层核对、浅/暗同值、每层可画、
  画法几何（描边外扩 0.5、柔光不外扩）。
- `plugins/dscommands/panel_elevation_test.go`：面板两层柔光 = `prominent`、
  `HasClip == false`、`border-l1` 描边外扩 0.5。
- `plugins/dsnotifications/card_elevation_test.go`：卡片两层柔光 = `Panel`、
  `border-l4` 描边外扩 0.5（原来的 `TestCardPanelElevation` 继续钉层数/顺序）。
- 变异验证（改坏必红，改完复原再绿）：

| 变异 | 结果 |
|---|---|
| `elevation-panel` 第一层 alpha `0.03 → 0.04` | `TestOfficialElevationTokens` 红（点名 Light/Dark `--dsw-elevation-panel` 第 0 层） |
| 命令面板改传 `Shadows.Panel` 而不是 `Prominent` | `TestCommandPanelPaintsOfficialProminentElevation` 红（逐层点名应为 `{3,8,.04}`/`{0,20,.05}`） |
| 通知卡片改传 `Shadows.Prominent`（`drawStack` 的后层卡路径 / `drawCard` 的前景卡路径**各改一处分别验**） | 两处都让 `TestCardElevationIsTheSharedPanelToken` 红——只测一张卡会漏掉另一条路径，所以这条测试按"两组柔光"验 |
| 命令面板阴影改回 `Draw` 钩子里画 | 同上测试红：`HasClip == true`，报出"面板自己 `.Clip()`，阴影会被整层裁掉" |
| 三件套描边宽度 `0.5 → 0` | `TestOfficialElevationTokens` + `TestCardElevationIsTheSharedPanelToken` 双红（"没有 0.5px 发丝描边"） |

  复原后三个包 `go test -count=1` 全绿。

## 9. 已知限制（如实登记）

- 0.5px 发丝描边在 100% 缩放下可能吸附成 1 设备像素（viva `borderWidth`，
  `gfx/rounded_outline.go:11-14`）；官方是 CSS 的 0.5px，物理像素层面同样
  不一定落得下去，这里的差异属于渲染器粒度，不是令牌取值。
- 官方 `shadow-lv3` 的第三层 `0 12px 32px` 目前只进了令牌，还没有消费方用它
  （toast/悬停卡在我们的实现里还没接），属于下一轮。
