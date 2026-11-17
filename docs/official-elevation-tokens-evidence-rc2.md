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

## 5. 第一轮改了什么（分片：只动 `pluginapi`、`plugins/dscore` 与两个消费方）

| 位置 | 改动 |
|---|---|
| `pluginapi/theme.go` | 新增 `ShadowLayer` / `Elevation` / `Shadows` 三个类型与 `Theme.Shadows` 字段 |
| `internal/pluginapi/alias.go` | 三个类型的别名（漏一行就是"插件编译得动、壳编译不动"） |
| `plugins/dscore/shadow.go`（新） | 7 档取值（逐条 file:line）+ `PaintShadow` / `PaintElevation` 与两个 Overlay 变体 |
| `plugins/dscore/dscore.go` | `Light()` / `Dark()` 都装 `Shadows: shadows()` |
| `plugins/dscommands/panel.go` | 命令面板阴影 → 官方 `--dsw-elevation-prominent` + border-l1 描边 |
| `plugins/dsnotifications/card.go` | 通知卡片的本地 elevation 常量与 `drawPanelElevation` → 共用 `Shadows.Panel` |
| （第二轮，task-80） | 剩余 **18 处**消费方全部换掉：13 处 viva 预设 + 5 处手抄副本，逐条见 §7 |
| `plugins/dscore/shadow_consumers_guard_test.go`（新） | 守卫：这 15 个包不许再出现 `gfx.ShadowXS/SM/MD/LG` |
| `plugins/dscore/elevation_decision_table_test.go`（新） | 守卫：18 处的「档位 + 描边色」逐条钉住（§10） |

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

## 7. 18 处消费方（task-80，**已全部替换**）

第一轮只动了 2 个消费方（命令面板、通知卡）。这一轮把剩下的 18 处全部换完：
**13 处原本拿 viva 预设顶上**，**5 处手抄了同一套数值**。逐条如下（"行"是本轮
结束时的行号；`§` 指向 §10 的机器可核版本）。

### 7.1 预设 13 处

| # | 位置 | 旧值 → 新值 | 档位 / 描边 | 官方依据 |
|---|---|---|---|---|
| 1 | `plugins/dsagentteam/catalog.go:443` | `Border(1,BorderL2)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL1` | `TeamAction.module.css` `.VBK9GG_panel{--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-prominent);border:0}`（`tmp/asar-css/156__TeamAction`）；菜单原语 `._list_70kcw_7,._submenu_70kcw_8` 同档（`index-CSAdOBx6.css`） |
| 2 | `plugins/dsagentteam/confirm.go:81` | `Border(1,BorderL2)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL4` | 对话框原语 `._dialog_1ytzq_33{…box-shadow:var(--dsw-elevation-prominent);border:0}`（`index-CSAdOBx6.css`），**未重绑**描边色 → 默认 `border-l4`（`gradient-shadow-text.css:17`） |
| 3 | `plugins/dscomposer/composer.go:298` | `Shadow(gfx.ShadowXS)` → `PaintElevation` | `Soft` / `BorderL2` | `InputBar.module.css:69-72` `.GRgSLq_card{--dsw-elevation-stroke-color:var(--dsw-alias-border-l2);box-shadow:var(--dsw-elevation-soft);border:0}` |
| 4 | `plugins/dsconversation/feedback.go:330` | `Border(1,BorderL2)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL4` | 官方 `FeedbackDialog.module.css` 只有 `dialog/categories/chip/detail/submit`，**无 box-shadow** → 外框是对话框原语（上） |
| 5 | `plugins/dscordis/panel.go:224` | `Border(1,BorderL1)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL1` | `CordisPanel.module.css:96-98` `.Fu0XRq_panel{border:0;border-radius:12px;--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-prominent)}` |
| 6 | `plugins/dsjobs/official_jobs.go:266` | `Border(1,BorderL2)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL1` | `JobListAction.module.css:57-58` `.Ag-ohG_menu{--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-prominent)}` |
| 7 | `plugins/dssettings/official_panel.go:598`（AfterDraw） | `Border(1,BorderL2)+Shadow(gfx.ShadowXS)+Clip` → `PaintElevationOverlay` | `Prominent` / `BorderL4` | `SettingsRoot.module.css:110` `.AnXs9G_panel{box-shadow:var(--dsw-elevation-prominent)}`，未重绑描边 → 默认 `border-l4` |
| 8 | `plugins/dssettings/settings.go:432` | `Border(1,BorderL2)+Shadow(gfx.ShadowXS)` → `PaintElevation` | `Prominent` / `BorderL4` | 同上（rc.1 兼容桥的调试设置卡，官方无此卡，档位随设置面） |
| 9 | `plugins/dsshortcuts/shortcuts.go:251`（AfterDraw） | `Shadow(gfx.ShadowLG)+Clip` → `PaintShadowOverlay` | `Lv3` / **无描边** | `Reference.module.css:63` `.St8tea_dialog{box-shadow:0 0 1px #0003,0 0 4px #00000005,0 12px 32px #00000014}` —— 逐值等于 `--dsw-shadow-lv3`（`gradient-shadow-text.css:8-9`），**不是**三件套 |
| 10 | `plugins/dssidebar/workspace_actions.go:268` | `Border(1,BorderL2)+Shadow(gfx.ShadowMD)` → `PaintElevation` | `Prominent` / `BorderL4` | 官方 ui-workspace 的模态只有 `modalAction/modalError/menuStatus`（`WorkspacePicker.module.css` 239 字节，无 box-shadow）→ 对话框原语 |
| 11 | `plugins/dsstatusbar/official_stats_dialog.go:239` | `Shadow(gfx.ShadowLG)` → `PaintElevation` | `Prominent` / `BorderL1` | `stat-dialog.module.css:25-26` `.xXe6ja_panel{--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-prominent)}` |
| 12 | `plugins/dsterminal/guide.go:307` | `Shadow(gfx.ShadowLG)` → `PaintElevation` | `Prominent` / `BorderL1` | 官方 `TerminalGuide.module.css` 的 `.-4GESq_menu` 只有 `flex:none;display:flex`（无阴影）→ 内部的 primitives `Menu`（`._list_70kcw_7`，prominent+l1） |
| 13 | `plugins/dsagentteam/commandrow.go` | `var _ = gfx.ShadowMD`（无绘制占位引用） | **删** | 该文件没有任何浮层，占位引用是编译期探针，随令牌落地失去意义 |

> **关于"对话框 → `prominent` + `border-l4`"**：官方模态框住在打包产物里、没有独立
> `src/**`，唯一证据是 `dsh-web-frontend/dist/assets/index-CSAdOBx6.css` 的
> `._dialog_1ytzq_33{…;box-shadow:var(--dsw-elevation-prominent)}`，而且它**没有**
> 重绑 `--dsw-elevation-stroke-color`（所以是默认档 `border-l4`，见 `:17`；
> 菜单面才重绑成 l1，见 `:23-25`）。第 2/4/10 三处都按这一条处理，而不是按
> "看着像弹窗"推的。

### 7.2 手抄同套数值 5 处（改来源，不改数值）

这 5 处的数值本来就和官方一致（各包自己在常量注释里写了 file:line），问题是
**第 N 份副本**：官方哪里调一次 `var(--dsw-elevation-*)`，我们就有 5 个地方各抄
一遍同样的 3 个数字。现在全部换成令牌，本地常量与那段"描边 + 两层柔光"的几何一起删掉。

| # | 位置 | 旧值 → 新值 | 档位 / 描边 | 官方依据 |
|---|---|---|---|---|
| 14 | `plugins/dsattachment/rail.go:886` | 本地 `arrowStroke/arrowShadowY/Blur/Alpha/arrowGlowBlur/Alpha` + `drawAttachmentArrowElevation` 的手画几何 → `PaintElevation`（函数保留一行） | `Panel` / `BorderL2DarkmodeThin` | `AttachmentRail.module.css` `.Onur9G_arrow{--dsw-elevation-stroke-color:var(--dsw-alias-border-l2-darkmode-thin);box-shadow:var(--dsw-elevation-panel);border:0;border-radius:999px}` |
| 15 | `plugins/dscontextmenu/context_menu.go:368` | 本地 `menuShadowOffsetY/Blur/Alpha/menuGlowBlur/Alpha/menuStroke` + 手画几何 → `PaintElevation` | `Prominent` / `BorderL1` | `MenuView.module.css` `.pc3rQa_menu{border:0;…box-shadow:var(--dsw-elevation-prominent)}`（`:28` 重绑 l1）；菜单原语 `._list_70kcw_7` 同档 |
| 16 | `plugins/dsdirpicker/browse.go:1270`（AfterDraw，两张卡） | 本地 `modalElevation*` 六个常量 + `drawModalElevation` 的手画几何 → `PaintElevationOverlay` | `Prominent` / `BorderL4` | `DirectoryBrowser.module.css` 的 `._3zXc5G_dialog{…padding:0}` **无 box-shadow** → 外框是对话框原语（同 §7.1 注） |
| 17 | `plugins/dsgoal/official_seat.go:178` | 本地 `goalPanelShadowY/Blur/A/goalPanelGlowBlur/A/goalPanelStroke` → `PaintElevation`（函数保留一行） | `Panel` / `BorderL1` | `GoalBar.module.css` `.Gc24Bq_bar{--dsw-elevation-stroke-color:var(--dsw-alias-border-l1);box-shadow:var(--dsw-elevation-panel);border:0}` |
| 18 | `plugins/dsquestions/plan_review.go:205`（两处调用点：`plan_review.go`/`question_flow.go`） | 本地 `planReviewPanelShadow*/Glow*` + 手画几何 → `PaintElevation` | `Panel` / `BorderL2` | `PlanReviewPanel.module.css` `.Nksazq_card{--dsw-elevation-stroke-color:var(--dsw-alias-border-l2);box-shadow:var(--dsw-elevation-panel);border:0}` |

> `dsdirpicker` 的 18 行清单里只算 1 处（原始清单 `:1231` 指的就是 `drawModalElevation`），
> 而它其实有**两张卡**（浏览卡 `browse.go:870`、新建文件夹卡 `browse.go:1143`）都在用它，
> 所以两处调用点一起改、AfterDraw 里按"外层卡 → 新建卡"的次序画。

### 7.3 为什么有的地方删了 `Border(1, …)`

官方这些浮层一律 `border:0`：那圈分离线是 `--dsw-elevation-stroke`
（`0 0 0 0.5px`）**画在 box-shadow 里**的，**不占布局**（`gradient-shadow-text.css:10-16`
的注释专门解释了为什么这么做）。我们原来的 `Border(1, t.BorderL2)` 是自造的 1px
**占布局**描边，且和描边令牌叠在一起会是两圈。所以凡是官方 `border:0` 的那几处，
`Border(1, …)` 与 `Shadow(…)` 一起换成 `PaintElevation`——这不是顺手改样式，是同一处
分离线的正确画法。

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

### 8.1 第二轮（task-80）新增的证据

| 测试 | 钉住什么 |
|---|---|
| `plugins/dscore/shadow_consumers_guard_test.go` | 15 个已换完的包**不许**再出现 `gfx.ShadowXS/SM/MD/LG`，且每个包必须真的调到 `dscore.PaintShadow*`/`PaintElevation*`（删掉调用也算回退） |
| `plugins/dscore/elevation_decision_table_test.go` | 18 处的「档位 + 描边色」逐条钉住（机器版本见 §10）；改任何一处的档位或描边色都点名报出是哪个文件 |
| `plugins/dsshortcuts/sheet_elevation_test.go` | 快捷键面板是 `Lv3` 的**三层**、`HasClip == false`；另有一条只跑 `Draw` 的用例，断言阴影**不在** Draw 钩子里 |
| `plugins/dsdirpicker/browse_visual_test.go`（改） | 两张卡的两层柔光 = `Shadows.Prominent`、`HasClip == false`；harness 现在按壳的次序跑 AfterDraw（`cmd/app/main.go:2178-2182`） |
| `plugins/dssettings/official_panel_elevation_test.go`（新） | 设置面板两层柔光 = `Prominent`、0.5px `border-l4` 发丝环绕在卡片外、`HasClip == false` |
| `plugins/dsdirpicker/browse_test.go`（改） | 删掉 6 行"常量等于自身"的断言（常量已并入令牌）；取值改由 `plugins/dscore/shadow_test.go` 的官方值表 + 上面两条可视化测试钉住 |

### 8.2 变异验证（本轮，改坏必红、按字节复原）

| 变异 | 结果 |
|---|---|
| 在 `dsjobs` 里加回 `var _ = gfx.ShadowMD` | `TestConvertedPluginsUseOfficialShadowTokens` 红，点名 `dsjobs: gfx.ShadowMD` |
| `dsdirpicker` 的 `afterDraw` 直接 return（阴影等于没画） | `TestCardCastsOfficialProminentElevation` 红："卡片柔光=0 层，want 2" |
| 把 `dsdirpicker` 的阴影**搬回带 `.Clip()` 的 Draw 钩子**并关掉 afterDraw | 同上测试红：`HasClip == true`，`ClipOf` 正是卡片矩形 `{260,150,680,500}` —— **这就是 #150 那个坑的现场复现**（改前这包里两张卡一直是这个写法，所以它们的 elevation 从来没画出来过） |
| 快捷键面板改传 `Shadows.Prominent`（档位错） | `TestSheetCastsOfficialLv3Shadow` 红："官方 --dsw-shadow-lv3 是 3 层，实际画了 2 层" |
| 设置面板改传 `Shadows.Panel`（档位错） | `TestOfficialPanelCastsProminentElevation` 红：第 0 层 alpha `.03` vs 官方 `.04`、第 1 层 blur 16 vs 20 |
| 令牌值 `lv3` 第三层 alpha `.08 → .09` | `TestOfficialShadowTokens` 红（Light/Dark 各报一次） |
| `dsgoal` 描边色 `BorderL1 → BorderL2` | `TestElevationDecisionTablePerSite` 红，点名 `dsgoal/official_seat.go: 期望 Shadows.Panel（描边 "BorderL1"）` |

  复原后 `go test ./... -count=1` 全绿（53 包）。

## 9. 已知限制（如实登记）

- 0.5px 发丝描边在 100% 缩放下可能吸附成 1 设备像素（viva `borderWidth`，
  `gfx/rounded_outline.go:11-14`）；官方是 CSS 的 0.5px，物理像素层面同样
  不一定落得下去，这里的差异属于渲染器粒度，不是令牌取值。
- 官方 `shadow-lv3` 原来没有消费方；本轮快捷键面板接上了（`Reference.module.css:63`
  把 lv3 手抄成了三层原生 box-shadow），所以那一档现在有真实调用点。官方的
  `._toast_af3m1_6` / 悬停卡 `._card_l6gf9_9` 也用它，但本仓的 toast/悬停卡还没接。
- 三处对话框（`dsagentteam/confirm.go`、`dsconversation/feedback.go`、
  `dssidebar/workspace_actions.go`）与 `dsdirpicker` 的 modal，官方对应组件
  **都没有自己的 box-shadow**，阴影来自打包产物里的对话框原语 `._dialog_1ytzq_33`。
  这条证据是我们能拿到的最直接的一条（组件 → 原语），但不是组件级 CSS 声明；
  哪天官方把对话框原语换成别的档，这几处要一起跟。

## 10. 决策表（机器可核）

`plugins/dscore/elevation_decision_table_test.go` 里有一张 `elevationSites` 表，
用 AST 扫源码核对 18 处的「档位 + 描边色」：

```text
plugins/dsagentteam/catalog.go          Shadows.Prominent  BorderL1
plugins/dsagentteam/confirm.go          Shadows.Prominent  BorderL4
plugins/dscomposer/composer.go          Shadows.Soft       BorderL2
plugins/dsconversation/feedback.go      Shadows.Prominent  BorderL4
plugins/dscordis/panel.go               Shadows.Prominent  BorderL1
plugins/dsjobs/official_jobs.go         Shadows.Prominent  BorderL1
plugins/dssettings/official_panel.go    Shadows.Prominent  BorderL4
plugins/dssettings/settings.go          Shadows.Prominent  BorderL4
plugins/dsshortcuts/shortcuts.go        Shadows.Lv3        —
plugins/dssidebar/workspace_actions.go  Shadows.Prominent  BorderL4
plugins/dsstatusbar/official_stats_dialog.go Shadows.Prominent BorderL1
plugins/dsterminal/guide.go             Shadows.Prominent  BorderL1
plugins/dsattachment/rail.go            Shadows.Panel      l2-darkmode-thin（按亮/暗算，测试只核档位）
plugins/dscontextmenu/context_menu.go   Shadows.Prominent  BorderL1
plugins/dsdirpicker/browse.go           Shadows.Prominent  BorderL4
plugins/dsgoal/official_seat.go         Shadows.Panel      BorderL1
plugins/dsquestions/plan_review.go      Shadows.Panel      BorderL2
```

一处一档一条，不是"浮层都用同一个"。改表就是改契约：先改这张表的依据
（官方 CSS 的那一行），再改测试里的期望值。
