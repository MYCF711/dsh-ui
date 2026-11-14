# 官方素材清单（official assets）

> 这份文档回答一个问题：**界面上的那枚图标，到底是从官方安装包里来的，还是我们自己画的。**
> 关于"应该长什么样"的规格另有 `docs/official-specs.md`；这里只谈矢量素材的出处与去向。

- 素材来源：本机安装态官方包 `D:/DeepSeek Harness/Desktop/resources/app.asar`（只读）。
- 代码侧唯一来源：[`plugins/dsassets/`](../plugins/dsassets/)（`paths.go` 存原文，`registry.go` 存出处，`parse.go` 存解析）。
- 逐字节证据：[`testdata/official/`](../plugins/dsassets/testdata/official/) 里是官方 JSX 原文片段 + `manifest.json`（每条路径的 sha256 与长度）。
- 本文与代码的绑定：`plugins/dsassets` 的测试要求**每条素材 ID 和每条 `d` 原文都出现在本文里**，改一边不改另一边会红。

---

## 1. 一句话现状

| 图标 | 官方有没有 | 我们画的是不是官方 | 结论 |
|---|---|---|---|
| 发送（输入框主按钮） | 有（内联 path，359 字节） | **是**（逐命令一致） | replace |
| 停止（运行态主按钮） | 有，但是 `<rect rx=3>` | **不是**（二次贝塞尔近似，角部偏 0.182） | skip |
| 附件（回形针） | 有（1 条 path） | 界面上**还没有这个控件** | exact（待接） |
| 权限四档 | 有（4 枚，同一面盾牌） | **是** | replace |
| 复制 | 有，但含 `<rect rx=2>` | 界面用的是文字按钮"复制" | skip |
| 分支 | 有，但含 2 枚 `<circle>` | 界面上**没有这个控件** | skip |
| 反馈拇指 👍/👎 | 有（4 条不同路径） | 以前是**凭空画的**（👎 还是 👍 的镜像），现在**是官方** | replace |
| 朗读 | **官方没有这个控件** | 我们也没有 | 缺失（不自造） |

---

## 2. 盘点：当前界面用到的图标

状态三档的定义：

- **exact**：我们的矢量与官方 `d` 逐命令一致（`d` 原文逐字节相同，解析只是把 H/V 展开成 L）。
- **手绘近似**：形状是我们自己画的，与官方不等价（写明偏差）。
- **缺失**：界面上没有这枚图标，或者官方根本没有。

| 图标 | 官方包 | 文件 | 符号 | viewBox | 当前 Go 实现 | 状态 |
|---|---|---|---|---|---|---|
| 发送 | `@deepseek-ai/dsh-client-ui-conversation` | `dsh-client-ui-conversation/lib/client.js` | InputBar 主按钮内联 `<path>` | `0 0 16 16` | [`plugins/dscomposer/icons.go`](../plugins/dscomposer/icons.go) `primarySendIcon` → `dsassets.ComposerSend` | **exact** |
| 停止 | 同上 | 同上 | InputBar 主按钮内联 `<rect x=3 y=3 width=10 height=10 rx=3>` | `0 0 16 16` | 同上 `primaryStopIcon`（自绘，未接素材） | **手绘近似**（偏差 0.182） |
| 附件 | `@deepseek-ai/dsh-client-ui-primitives` | `dsh-web-frontend/dist/assets/index-BXV-oxvX.js` | `IconPaperclipOutlineRegular` | `0 0 16 16` | 无（宿主动作没接，加号不画） | **缺失**（素材已备，exact） |
| 命令入口（加号） | primitives | 同上 | `IconPlusOutlineMedium` | `0 0 16 16` | [`plugins/dscomposer/composer.go`](../plugins/dscomposer/composer.go) `addButton` → `dsassets.ComposerAdd`（仅当 `ui.commands.open` 服务在、命令表非空） | **exact**（Medium 档 1.3） |
| 权限·只读 | primitives | 同上 | `PermissionIconReadOnlyRegular` | `0 0 16 16` | [`plugins/dscomposer/icons.go`](../plugins/dscomposer/icons.go) `permissionIconFor("read-only")` | **exact** |
| 权限·工作区写入 | primitives | 同上 | `PermissionIconWorkspaceWriteRegular` | `0 0 16 16` | 同上 `("workspace-write")` | **exact** |
| 权限·完全访问 | primitives | 同上 | `PermissionIconFullAccessRegular` | `0 0 16 16` | 同上 `("danger-full-access" / "allow")` | **exact** |
| 权限·自动审查 | primitives | 同上 | `PermissionIconAutoReviewRegular` | `0 0 16 16` | 同上 `("auto")` | **exact** |
| 权限·ask / deny | **官方没有** | — | — | `0 0 16 16` | 同上，官方盾牌 + **自造**问号/斜线 | 自造（盾牌 exact） |
| 复制 | primitives | 同上 | `IconCopyOutlineRegular` | `0 0 16 16` | [`plugins/dsconversation/conversation.go`](../plugins/dsconversation/conversation.go) 里的文字按钮「复制」 | **skip**（含 `<rect rx=2>`） |
| 朗读 | **官方没有** | — | — | — | 无 | **缺失**（不造） |
| 分支 | primitives | 同上 | `IconBranchOutlineRegular` | `0 0 16 16` | 无（动作行只摆 copy→feedback→retry） | **skip**（含 2 枚 `<circle>`） |
| 👍 未评分 | primitives | 同上 | `IconLikeOutlineRegular` | `0 0 16 16` | [`plugins/dsconversation/feedback.go`](../plugins/dsconversation/feedback.go) `thumbIcon(true,false)` | **exact** |
| 👍 已评分 | primitives | 同上 | `IconLikeFillRegular` | `0 0 16 16` | 同上 `thumbIcon(true,true)` | **exact** |
| 👎 未评分 | primitives | 同上 | `IconDislikeOutlineRegular` | `0 0 16 16` | 同上 `thumbIcon(false,false)` | **exact** |
| 👎 已评分 | primitives | 同上 | `IconDislikeFillRegular` | `0 0 16 16` | 同上 `thumbIcon(false,true)` | **exact** |

> 官方 primitives 在本安装态里**不是一个独立发布的包**，而是打进 web 前端的 dist chunk
> （`@deepseek-ai/dsh-web-frontend/dist/assets/index-BXV-oxvX.js`，941 436 字节）。
> 所以"包名"写 primitives、"文件"写 dist chunk——两者不是同一层，表里照实写。

---

## 3. 素材登记表

`plugins/dsassets` 的 15 条素材（ID 即 `testdata/official/<ID>.jsx` 的文件名）：

| ID | 官方符号 | viewBox | 命令数 | 构建 |
|---|---|---|---|---|
| `composer-send` | InputBar 主按钮内联 path | 16 | 16 | ✅ |
| `composer-stop` | InputBar 主按钮内联 rect | 16 | — | ❌ 需要弧 |
| `send-outline` | `IconSendOutlineRegular` | 16 | 12 | ✅ |
| `paperclip` | `IconPaperclipOutlineRegular` | 16 | 17 | ✅ |
| `composer-add` | `IconPlusOutlineMedium` | 16 | 4 | ✅ |
| `copy` | `IconCopyOutlineRegular` | 16 | 14（只算 path；另有 1 枚 rect） | ❌ 需要弧 |
| `branch` | `IconBranchOutlineRegular` | 16 | 12（只算两条支线；另有 2 枚圆） | ❌ 需要弧 |
| `thumb-up-outline` | `IconLikeOutlineRegular` | 16 | 18 | ✅ |
| `thumb-up-fill` | `IconLikeFillRegular` | 16 | 18 | ✅ |
| `thumb-down-outline` | `IconDislikeOutlineRegular` | 16 | 18 | ✅ |
| `thumb-down-fill` | `IconDislikeFillRegular` | 16 | 18 | ✅ |
| `permission-read-only` | `PermissionIconReadOnlyRegular` | 16 | 13 | ✅ |
| `permission-workspace-write` | `PermissionIconWorkspaceWriteRegular` | 16 | 45 | ✅ |
| `permission-full-access` | `PermissionIconFullAccessRegular` | 16 | 13 | ✅ |
| `permission-auto-review` | `PermissionIconAutoReviewRegular` | 16 | 14 | ✅ |

"命令数"是解析后的 viva 命令总数，测试里写死（`TestParseCommandCounts`）：少一条＝路径被截断。

---

## 4. 官方 `d` 原文

下面是**逐字节原文**（未重排、未展开 H/V、未补零）。代码里的同名常量在
[`plugins/dsassets/paths.go`](../plugins/dsassets/paths.go)，测试要求本文与它
完全一致；`manifest.json` 里记着每条路径的 sha256 与长度。

### 4.1 发送键箭头（`composer-send`，359 字节）

```
M8.3125 0.980183C8.66767 1.0531 8.97902 1.20418 9.2627 1.43233C9.48724 1.61297 9.73029 1.85793 9.97949 2.10714L14.707 6.83468L13.293 8.24874L9 3.95577V15.0417H7V3.95577L2.70703 8.24874L1.29297 6.83468L6.02051 2.10714C6.26971 1.85793 6.51277 1.61297 6.7373 1.43233C6.97662 1.23986 7.28445 1.04402 7.6875 0.980183C7.8973 0.947006 8.1031 0.95516 8.3125 0.980183Z
```

### 4.2 发送图标（`send-outline`，205 + 32 字节）

第一条是填充的机尾，第二条是描边的机身前段：

```
M6.97211 1.94476C7.55785 1.35914 8.50767 1.35919 9.09343 1.94476L13.921 6.77228L13.2138 7.47939L8.38632 2.65187C8.19108 2.45682 7.87443 2.45677 7.67922 2.65187L2.74397 7.58711L2.03687 6.88L6.97211 1.94476Z
```

```
M7.97571 14.5732L8.02421 2.34139
```

### 4.3 回形针（`paperclip`，512 字节）

```
M12.75 4.5V9.5C12.75 10.7598 12.2496 11.968 11.3588 12.8588C10.468 13.7496 9.25978 14.25 8 14.25C6.74022 14.25 5.53204 13.7496 4.64124 12.8588C3.75045 11.968 3.25 10.7598 3.25 9.5V5C3.25 4.13805 3.59241 3.3114 4.2019 2.7019C4.8114 2.09241 5.63805 1.75 6.5 1.75C7.36195 1.75 8.1886 2.09241 8.7981 2.7019C9.40759 3.3114 9.75 4.13805 9.75 5V9.5C9.75 9.96413 9.56563 10.4092 9.23744 10.7374C8.90925 11.0656 8.46413 11.25 8 11.25C7.53587 11.25 7.09075 11.0656 6.76256 10.7374C6.43437 10.4092 6.25 9.96413 6.25 9.5V5.5
```

### 4.4 加号（`composer-add`，7 字节 × 2）

官方 `IconPlusOutlineMedium` 的两笔，纯描边（`stroke="currentColor"`）。
内层组件与 Regular 档共用同一个 svg，差别只有 `strokeWidth`：Medium 是 `K=1.3`、Regular 是 `1`。
它就是官方 InputBar 左侧命令入口 `.GRgSLq_add` 里的 `<IconPlusOutlineMedium size={14}/>`。

```
M8 2V14
```

```
M2 8H14
```

### 4.5 复制（`copy`，348 字节的 path + 一枚 `<rect>`）

前面那张纸是 `<rect x=1.52075 y=4.07373 width=10.3932 height=10.3932 rx=2>`，
**画不出来**（需要弧形圆角）；下面这条是后面那张纸（填充）：

```
M11.9792 1.53296C13.36 1.53296 14.4792 2.65225 14.4792 4.03296V9.42847C14.4792 10.3756 13.9521 11.1987 13.1755 11.6228V10.3298C13.3652 10.0787 13.4792 9.7674 13.4792 9.42847V4.03296C13.4792 3.20453 12.8077 2.53296 11.9792 2.53296H6.58374C6.27966 2.53301 5.99684 2.6235 5.7605 2.77905H4.42358C4.85652 2.03463 5.66056 1.53304 6.58374 1.53296H11.9792Z
```

### 4.6 分支（`branch`，182 + 182 字节的两条支线 + 2 枚 `<circle>`）

节点是 `<circle cx=12.4502 cy=3.3079 r=1.56962>` 与 `<circle cx=12.4502 cy=12.6921 r=1.56962>`，
**画不出来**。两条支线原文：

```
M1.01503 8.0001L5.6964 8.0001C6.41913 8.0001 6.78049 8.0001 7.12115 7.91951C7.4232 7.84804 7.71233 7.73014 7.97821 7.57C8.27809 7.38939 8.5364 7.13669 9.05303 6.63129L11.3281 4.40564
```

```
M1.01221 7.9999L5.6964 7.9999C6.41913 7.9999 6.78049 7.9999 7.12115 8.08049C7.4232 8.15196 7.71233 8.26986 7.97821 8.43C8.27809 8.61061 8.5364 8.86331 9.05303 9.36871L11.3281 11.5944
```

### 4.7 👍（`thumb-up-outline` / `thumb-up-fill`，517 字节，两档同一条 `d`）

Outline 只有 `stroke`；Fill 是**同一条 `d`** 加 `fill`：

```
M13.537 8.12098L12.3983 12.8455C12.1818 13.7438 11.378 14.3769 10.454 14.3769L9.35595 14.3769H7.43799H5.16577C3.50892 14.3769 2.16577 13.0337 2.16577 11.3769V7.88668C2.16577 7.33439 2.61349 6.88668 3.16577 6.88668H4.02665C5.84943 6.88668 7.38083 3.28711 7.67689 2.54578C7.71259 2.45639 7.73501 2.36373 7.77922 2.27824C7.86506 2.11221 8.08228 1.87578 8.59039 2.07775C10.3291 2.76886 9.23144 6.04071 8.96955 6.75058C8.94502 6.81707 8.99495 6.88668 9.06581 6.88668H12.5648C13.2119 6.88668 13.6886 7.49192 13.537 8.12098Z
```

### 4.8 👎（`thumb-down-outline` / `thumb-down-fill`，549 字节）

**不是 👍 的镜像**：这是另一条手画路径（掌根在右、拇指朝下）：

```
M2.46302 8.06749L3.60171 3.34299C3.81822 2.44467 4.62196 1.81162 5.546 1.8116L6.64406 1.81158L8.56202 1.81158L10.8342 1.81158C12.4911 1.81158 13.8342 3.15473 13.8342 4.81158L13.8342 8.3018C13.8342 8.85408 13.3865 9.3018 12.8342 9.3018L11.9734 9.3018C10.1506 9.3018 8.61918 12.9014 8.32311 13.6427C8.28741 13.7321 8.26499 13.8247 8.22078 13.9102C8.13494 14.0763 7.91772 14.3127 7.40961 14.1107C5.67089 13.4196 6.76856 10.1478 7.03045 9.43789C7.05498 9.37141 7.00505 9.3018 6.93419 9.3018L3.43519 9.3018C2.78811 9.3018 2.31141 8.69656 2.46302 8.06749Z
```

### 4.9 权限盾牌（`permission-read-only` / `-full-access` / `-auto-review` 共用，209 字节）

```
M6.59624 2.14853C7.50155 1.80917 8.49914 1.80919 9.40444 2.14859L13.9245 3.84317V7.11961C13.9245 11.6089 10.5565 13.5975 8.00035 14.5779C5.44423 13.5975 2.07544 11.6089 2.07544 7.11961V3.84317L6.59624 2.14853Z
```

三档各自叠的中间字形：

只读对勾（95 字节）：

```
M5.08545 8.13775L7.18455 10.2368C7.26636 10.3187 7.4003 10.3142 7.47649 10.2271L11.5148 5.61194
```

完全访问的感叹号（18 + 18 字节）：

```
M8 4.39209V9.89209
```

```
M8 10.8081V11.8081
```

自动审查的 A 字（45 字节，官方这条写了 `stroke-linecap:round`）：

```
M5.75 10.15L8 4.95L10.25 10.15M6.43 8.59H9.57
```

### 4.10 权限·工作区写入（`permission-workspace-write`，5 条）

这面盾**是填充**（官方 `fill="currentColor"`，靠路径画出环带），不是另外三档那种描边盾：

```
M6.4209 1.68067C7.43922 1.299 8.56177 1.29898 9.58008 1.68067L14.0996 3.375C14.2946 3.44811 14.4236 3.63455 14.4238 3.84278V6.89063C14.1115 6.71853 13.7761 6.58312 13.4238 6.48926V4.18946L9.22852 2.61621C8.43657 2.31947 7.56341 2.31939 6.77148 2.61621L2.5752 4.18946V7.11914C2.5752 11.1796 5.52369 13.056 8 14.0391C8.27653 13.9293 8.55827 13.8067 8.8418 13.6729C9.07101 13.9468 9.33228 14.1929 9.62012 14.4053C9.12409 14.6579 8.63578 14.8696 8.17871 15.0449C8.0637 15.0889 7.93628 15.0889 7.82129 15.0449C5.22011 14.0472 1.5752 11.9381 1.5752 7.11914V3.84278C1.57541 3.63469 1.70463 3.44821 1.89941 3.375L6.4209 1.68067Z
```

两条「文字行」（24 + 24 字节，描边）：

```
M5.26392 6.60339H10.7361
```

```
M5.26392 9.86902H8.32833
```

铅笔（395 字节）与它下面的横线（398 字节），都是填充：

```
M10.0317 13.2229C10.263 13.3929 10.4943 13.563 10.7256 13.733C10.7932 13.6482 10.8608 13.5634 10.9284 13.4786C12.1455 11.9522 13.3626 10.4258 14.5798 8.89935C14.6474 8.81455 14.715 8.72975 14.7826 8.64495C14.4143 8.37419 14.046 8.10344 13.6777 7.83268C13.6169 7.92252 13.5562 8.01236 13.4954 8.10219C12.4016 9.71926 11.3078 11.3363 10.214 12.9534C10.1532 13.0432 10.0924 13.1331 10.0317 13.2229Z
```

```
M12.6516 12.6696C12.6516 12.925 12.6516 13.1804 12.6516 13.4359C12.6952 13.4378 12.7387 13.4398 12.7823 13.4417C13.5663 13.4768 14.3504 13.5118 15.1345 13.5469C15.178 13.5488 15.2216 13.5508 15.2651 13.5527C15.2651 13.2194 15.2651 12.8861 15.2651 12.5527C15.2216 12.5547 15.178 12.5566 15.1345 12.5586C14.3504 12.5936 13.5663 12.6287 12.7823 12.6637C12.7387 12.6657 12.6952 12.6676 12.6516 12.6696Z
```

---

## 5. exact / replace / skip 清单

### exact（矢量与官方逐命令一致）

| 素材 | 落在哪 | 证据 |
|---|---|---|
| `composer-send` | `dscomposer.primarySendIcon` | `dscomposer.TestPrimarySendIconIsOfficial`、`dsassets.TestParseCommandCounts` |
| `permission-read-only` | `dscomposer.permissionIconFor("read-only")` | `dscomposer.TestPermissionIconsAreOfficialAssets` |
| `permission-workspace-write` | `("workspace-write")` | 同上 |
| `permission-full-access` | `("danger-full-access"/"allow")` | 同上 |
| `permission-auto-review` | `("auto")` | 同上 |
| `thumb-up-outline` / `thumb-up-fill` | `dsconversation.thumbIcon(true, …)` | `dsconversation.TestThumbIconUsesOfficialPaths` |
| `thumb-down-outline` / `thumb-down-fill` | `dsconversation.thumbIcon(false, …)` | 同上 |
| `send-outline` / `paperclip` | 尚未接到界面（素材已备） | `dsassets.TestBuildShapes` |

### replace（这次真的换了）

| 文件 | 换掉了什么 | 依据 |
|---|---|---|
| [`plugins/dscomposer/icons.go`](../plugins/dscomposer/icons.go) | 发送箭头改成 `dsassets.ComposerSend`；权限四档改成 `dsassets.Permission*`；`ask`/`deny` 的盾牌改成 `dsassets.ShieldOutline()` | 逐命令一致（命令数 16 / 13 / 45 / 13 / 14 对得上） |
| [`plugins/dsconversation/feedback.go`](../plugins/dsconversation/feedback.go) | 自绘拇指（`thumbCommands` + `mirrorY`）整段删除，改成官方四条路径 | 官方 👍/👎 是两条不同的路径，镜像关系不成立 |

### skip（明确不换，写清原因）

| 素材 | 为什么不换 | 代价 |
|---|---|---|
| `composer-stop` | 官方是 `<rect … rx=3>`，圆角是**四分之一圆弧**；viva 的 `PathCommand` 只有 M/L/Q/C/Z，没有弧 | 现用二次贝塞尔近似，`dscomposer.TestStopIconDeviationIsKnownAndBounded` 实测最大偏差 **0.182** viewBox 单位（16px 下约 0.18px） |
| `copy` | 官方含 `<rect rx=2>` | 界面上是文字按钮「复制」，本来就没画图标 |
| `branch` | 官方含 2 枚 `<circle r=1.56962>` | 界面上没有分支按钮 |

### 缺失

| 图标 | 情况 |
|---|---|
| 朗读 | **官方客户端根本没有这个控件**：55 个 `@deepseek-ai/dsh-client-ui-*` 包与 web 前端 chunk 里没有任何 speaker / read-aloud / TTS 符号或文案（中文 `朗读` 零命中）。**不自造**。 |
| 附件 | 官方有回形针图标；本仓 `Composer` 因为没有宿主动作而**不画加号**（`composer.go` 顶部注释），素材已备但未接。 |
| 分支按钮 | 官方有分支图标；本仓动作行只摆 copy → feedback → retry。 |

---

## 6. 为什么不用字体 glyph / PNG 顶替

- **字体 glyph**：`↑` / `■` / `▸` 这类字符的形状取决于机器上装了什么字体，同一份代码在别人的 Windows 上会变样；而且字重、光学尺寸都对不上官方图标集。2026-11-04 之前 `dscomposer` 用 `↑`/`■` 冒充发送/停止，现在发送已经是官方矢量。
- **PNG**：官方 icons 全是矢量，PNG 要按每个 DPI 出图，而且要处理深浅主题两种颜色。没有引入。

本仓当前仍有字体 glyph 的地方（与本次素材无关，记录在此以免被误认为官方）：

| 位置 | 字符 | 用途 |
|---|---|---|
| [`dsconversation/conversation.go`](../plugins/dsconversation/conversation.go) | `▸` / `▾` | 思考块折叠箭头 |
| [`dscore/primitives.go`](../plugins/dscore/primitives.go) | `▸` / `▾` | DisclosureRow 折叠箭头 |
| [`dsfiles/files.go`](../plugins/dsfiles/files.go) | `▸` | 目录前缀 |
| [`dsrightbar/rightbar.go`](../plugins/dsrightbar/rightbar.go) | `▸` / `▾` | 分组折叠 |
| [`dscomposer/mention.go`](../plugins/dscomposer/mention.go) | `▸` | 提及列表里的目录标记 |

官方的折叠箭头是 `IconChevronRight/ChevronDown`（primitives，16 viewBox，纯 path，**可以 exact**），
但那是另一批素材、另一处改动，不在本次范围内；记在这里是为了不让 `▸` 被当成官方形状。

---

## 7. 刷新流程（官方包升级后）

两个脚本随包放在 [`plugins/dsassets/tools/`](../plugins/dsassets/tools/)，重跑即可复现整条证据链：

```bash
# 1. 解包官方 asar 里的 client-ui 包与 web 前端（只读 asar，只写 tmp/official/）
node plugins/dsassets/tools/extract-official.js            # 594 个文件
# 2. 抽出图标片段 + sha256 清单（写 plugins/dsassets/testdata/official/）
node plugins/dsassets/tools/gen-testdata.js
# 3. 跑测试：哪条素材对不上，测试会点名
CGO_ENABLED=0 go test ./plugins/dsassets/ ./plugins/dscomposer/ ./plugins/dsconversation/
```

两个脚本都是幂等的：同一份 asar 上跑两次，`testdata/official/` 逐字节不变
（2026-11-04 实测）。

素材变了要按这个顺序处理：

1. `gen-testdata.js` 跑完先看 `go test`——**文件哈希变了就必须先确认官方是真的换了图形**，
   而不是直接覆盖常量把测试哄绿；
2. 确认后同步改 `paths.go` 与本文（第 4 节原文、第 2/3 节表格），测试会强制两边一致；
3. 若官方新增了素材，在 `registry.go` 里加一条、在 `All()` 里登记，并在
   `parse_test.go` 的期望命令数表里写上新数字（不写会被测试点名）。

---

## 8. 未验项

1. **没有真机截图对照**：本次验收只做了无头渲染（`dscomposer.TestSendIconStatesPixels`、
   `dsconversation.TestThumbIconStatesPixels` 会把 PNG 落到临时目录），没有和官方客户端
   同尺寸截图做像素 diff。任务的约束是"不启动任何 GUI/真机窗口"，所以这一项留空。
2. **`composer-stop` 的偏差只有数值，没有视觉判据**：0.182 是解析出的几何距离，不是
   屏幕上的像素差（2x DPI 下的实际像素差异未测）。
3. **`ask` / `deny` 两枚自造图标**没有官方对照，形状是否合适需要设计确认。
4. **官方深浅两套主题下的图标是否都只用 `currentColor`**：这份清单核对的是形状；
   颜色一律由调用方给（本仓用主题令牌），没有逐一比对官方各主题的最终色值。
5. **官方包升级路径未演练**：`manifest.json` 里的 `file_sha256` 只在当前安装态上验证过。
6. **`send-outline` / `paperclip`** 已备好但界面上没有使用点，接上之后要补按钮状态截图。
