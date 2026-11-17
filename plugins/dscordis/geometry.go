package dscordis

// geometry.go —— 官方面板的几何常量，**逐条注明出处行号**。
//
// 官方唯一来源：
//
//	CSS:12  = `@deepseek-ai/dsh-client-ui-cordis/src/client/CordisPanel.module.css:12`
//	          （该文件 366 行，全部几何都在里面）
//	TSX:496 = `…/src/client/CordisPanel.tsx:496`（组件里写死的显示尺寸与偏移）
//
// 规矩（照本仓既有做法，见 `plugins/dsagentteam/catalog.go` 的几何段）：
// **一个值一行、行号写在旁边**。没有出处就写 derived 并说清推导，不许"看着差不多"。
const (
	// ── 触发器：`.layer` / `.footerButtons` / `.badge`（CSS:3-61）──────────────

	// cordisLayerMarginTop 是 `.layer{margin:8px 0 0}`（CSS:10）。
	cordisLayerMarginTop float32 = 8
	// cordisTriggerHeight 是 `.layer{height:42px}`（CSS:9）与
	// `.badge{height:42px}`（CSS:24）——两处同值，触发器本体就是 42 高。
	cordisTriggerHeight float32 = 42
	// cordisTriggerRadius 是 `.badge{border-radius:12px}`（CSS:28）。
	cordisTriggerRadius float32 = 12
	// cordisTriggerGap 是 `.badge{gap:8px}`（CSS:22）。
	cordisTriggerGap float32 = 8
	// cordisTriggerPadLeft / Right 是 `.badge{padding:0 10px 0 8px}`（CSS:26）。
	cordisTriggerPadLeft  float32 = 8
	cordisTriggerPadRight float32 = 10
	// cordisTriggerFont 是 `.badge{font-size:14px}`（CSS:32）。
	cordisTriggerFont float32 = 14
	// cordisTriggerIcon 是宽态图标的显示尺寸：
	// `<IconCordisPluginOutlineRegular size={wide ? 16 : 18}/>`（TSX:496）。侧栏底部是宽态 → 16。
	//
	// **本仓没有这枚 sprite**（`plugins/dsassets` 只登记 7 枚：send / composer-stop /
	// paperclip / composer-add / copy / branch / thumbs），按本任务书与
	// `plugins/dsagentteam/catalog.go:512-514` 的已知做法：**只保留 16×16 的槽宽，
	// 不手画 SVG**（差异登记见 `plugin.go` 包注释的「照实登记的差异」第 2 条）。
	cordisTriggerIcon float32 = 16
	// cordisTriggerCountFont / Line 是 `.badgeCount{font-size:12px;line-height:16px}`（CSS:58-59）。
	cordisTriggerCountFont float32 = 12
	cordisTriggerCountLine float32 = 16

	// ── 面板壳：`.panel`（CSS:86-103）────────────────────────────────────────

	// cordisPanelWidth 是 `.panel{width:420px}`（CSS:92）。
	cordisPanelWidth float32 = 420
	// cordisPanelWindowInset 是 `.panel{max-width:calc(100vw - 24px)}`（CSS:93）。
	cordisPanelWindowInset float32 = 24
	// cordisPanelMaxHeightRatio 是 `.panel{max-height:60vh}`（CSS:94）。
	cordisPanelMaxHeightRatio float32 = 0.6
	// cordisPanelRadius 是 `.panel{border-radius:12px}`（CSS:97）。
	cordisPanelRadius float32 = 12
	// cordisPanelAnchorGap 是 `bottom: window.innerHeight - rect.top + 8`（TSX:145）里的 8：
	// 面板底边落在**触发器上沿上方 8px**（CSS:84-85 的注释说明它为什么是 fixed）。
	cordisPanelAnchorGap float32 = 8
	// 官方"描边"的来历：`.panel{border:0}`（CSS:96）+ 抬升阴影里的
	// `--dsw-elevation-stroke-color: var(--dsw-alias-border-l1)`（CSS:98）。
	// 那圈 0.5px 描边现在由 `dscore.PaintElevation(..., Shadows.Prominent, BorderL1)`
	// 画在 elevation 的描边层里（不占布局），原来这里的手工代偿常量
	// `cordisPanelStroke = 1` 已随 `Shadow(gfx.ShadowMD)` 一起删掉。

	// ── 面板头部 / 正文：`.header` / `.title` / `.body`（CSS:116-131）─────────

	// cordisHeaderMinHeight 是 `.header{min-height:44px}`（CSS:121）。
	cordisHeaderMinHeight float32 = 44
	// cordisHeaderPadV / H 是 `.header{padding:10px 12px}`（CSS:122）。
	cordisHeaderPadV float32 = 10
	cordisHeaderPadH float32 = 12
	// cordisTitleFont / Line 是 `.title{font-size:14px;line-height:20px;font-weight:500}`（CSS:134-136）。
	cordisTitleFont float32 = 14
	cordisTitleLine float32 = 20
	// cordisBodyPadH / Bottom 是 `.body{padding:0 12px 12px}`（CSS:130）。
	cordisBodyPadH      float32 = 12
	cordisBodyPadBottom float32 = 12

	// ── 说明行：`.note` / `.readError`（CSS:140-150）────────────────────────

	// cordisNoteMargin 是 `.note,.readError{margin:4px 0}`（CSS:142）。
	cordisNoteMargin float32 = 4
	// cordisNoteFont / Line 是 `.note,.readError{font-size:12px;line-height:18px}`（CSS:143-144）。
	cordisNoteFont float32 = 12
	cordisNoteLine float32 = 18

	// ── 分组标题：`.group`（CSS:153-161）────────────────────────────────────

	// cordisGroupMargin 是 `.group{margin:8px 0 8px}`（CSS:154）。
	cordisGroupMargin float32 = 8
	// cordisGroupFont / Line 是 `.group{font-size:11px;line-height:16px;font-weight:500}`
	// （CSS:155-157）。
	cordisGroupFont float32 = 11
	cordisGroupLine float32 = 16
	// CSS:160 还有 `text-transform:uppercase;letter-spacing:0.04em`：
	//   - uppercase 对中文（本仓唯一语言）无效果，viva 也没有大小写变换；
	//   - letter-spacing viva 没有对应能力。
	// 两条**都不实现**，登记在 `plugin.go` 的差异表第 5 条（不是漏做）。

	// ── 行列表：`.rows` / `.row`（CSS:163-179）──────────────────────────────

	// cordisRowsGap 是 `.rows{gap:8px}`（CSS:167）。
	cordisRowsGap float32 = 8
	// cordisRowGap 是 `.row{gap:8px}`（CSS:175）。
	cordisRowGap float32 = 8
	// cordisRowPadTop / Bottom 是 `.row{padding:14px 12px 10px}`（CSS:176）。
	cordisRowPadTop    float32 = 14
	cordisRowPadH      float32 = 12
	cordisRowPadBottom float32 = 10
	// cordisRowBorder 是 `.row{border:0.5px solid var(--dsw-alias-border-l4)}`（CSS:177）。
	cordisRowBorder float32 = 0.5
	// cordisRowRadius 是 `.row{border-radius:12px}`（CSS:178）。
	cordisRowRadius float32 = 12

	// ── 行头：`.rowHead` / `.rowId` / `.rowName` / `.rowStatus`（CSS:185-220）─

	// cordisRowHeadGap 是 `.rowHead{gap:8px}`（CSS:187）。
	cordisRowHeadGap float32 = 8
	// cordisRowIDFont / Line 是 `.rowId{font-family:var(--dsh-font-mono,monospace);
	// font-size:11px;line-height:20px}`（CSS:193-196）。
	cordisRowIDFont float32 = 11
	cordisRowIDLine float32 = 20
	// cordisRowNameFont / Line 是 `.rowName{font-size:13px;font-weight:500;line-height:20px}`
	// （CSS:203-205）。
	cordisRowNameFont float32 = 13
	cordisRowNameLine float32 = 20
	// cordisRowStatusGap 是 `.rowStatus{gap:4px}`（CSS:215）。
	cordisRowStatusGap float32 = 4
	// cordisRowStatusHeight 是 `.rowStatus{height:20px}`（CSS:216）。
	cordisRowStatusHeight float32 = 20
	// cordisRowStatusFont / Line 是 `.rowStatus{font-size:11px;line-height:20px}`（CSS:217-218）。
	cordisRowStatusFont float32 = 11
	cordisRowStatusLine float32 = 20

	// ── 行详情：`.rowDetail` / `.rowPurpose` / `.rowActions`（CSS:222-272）───

	// cordisRowDetailGap 是 `.rowDetail{gap:8px}`（CSS:225）。
	cordisRowDetailGap float32 = 8
	// cordisRowDetailMinHeight 是 `.rowDetail{min-height:28px}`（CSS:226）——
	// 它就是动作按钮那一行的高度（28 的按钮）。
	cordisRowDetailMinHeight float32 = 28
	// cordisRowPurposeFont / Line 是 `.rowPurpose{font-size:12px;line-height:18px}`
	// （CSS:254-255）。
	cordisRowPurposeFont float32 = 12
	cordisRowPurposeLine float32 = 18
	// cordisRowActionsGap 是 `.rowActions{gap:10px}`（CSS:271）。
	cordisRowActionsGap float32 = 10

	// ── 动作按钮：`.actionButton`（CSS:320-333）──────────────────────────────

	// cordisActionButton 是 `.actionButton{width:28px;height:28px;border-radius:999px}`
	// （CSS:324-328）。
	cordisActionButton float32 = 28
	// cordisActionIcon 是按钮里图标的显示尺寸：官方四处 `RowAction` 都是
	// `<Icon… size={14}/>`（TSX:300 / :340 / :384 / :394）。
	//
	// **本仓没有 play-outline / stop-fill / trash-outline 这三枚 sprite**
	// （`plugins/dsassets` 的登记表里没有它们；`plugins/dsjobs/icons.go` 的 stop-fill
	// 是那个包的私有副本）。照任务书与 `plugins/dsagentteam` 的已知做法：
	// **只留 14×14 的槽宽，不手画 SVG**（差异登记见 `plugin.go` 第 3 条）。
	cordisActionIcon float32 = 14
	// cordisActionButtonRadius 是 999px 圆角在 28×28 上的结果（半径 = 边长的一半）。
	cordisActionButtonRadius float32 = cordisActionButton / 2

	// ── 行错误 / 运行中版本：`.rowError` / `.activeVersion`（CSS:261-316）───

	// cordisRowErrorFont / Line 是 `.rowError{font-size:12px;line-height:18px}`
	// （CSS:263-264）。
	cordisRowErrorFont float32 = 12
	cordisRowErrorLine float32 = 18
	// cordisActiveVersionFont / Line 是 `.activeVersion{font-size:11px;line-height:16px}`
	// （CSS:313-315）。
	cordisActiveVersionFont float32 = 11
	cordisActiveVersionLine float32 = 16

	// ── 状态点（StateDot，primitives 预构建产物的本仓等价物）─────────────────

	// cordisDotSize 是 `StateDot.dot` 的默认边长 10（官方 ongoing 分支换成 14 的
	// 旋转 spinner，见 `plugins/dsconversation/official_transcript_geometry.go:925-931`
	// 的 StateDot 两档：`t ?? (e === 'ongoing' ? 14 : 10)`）。
	//
	// 本仓静态画布没有时间轴，画不出那圈 spinner，所以**统一 10px 实心圆**
	// （`plugins/dsagentteam/board.go` 的 `drawDot` 是同一做法）；ongoing 态用品牌色
	// 而不是"转起来的圈"来表达，登记在 `plugin.go` 差异表第 4 条。
	cordisDotSize float32 = 10
)

// 面板锚点的兜底几何（derived）。
//
// 正常情况下锚点来自**上一帧触发器的真实矩形**（`panel.go` 的 `panelAnchor`，
// 与 `plugins/dsagentteam/model_slot.go:196-215` 同一做法：viva 的 `Box.Rect()` 要等
// 布局解算之后才有效，同一帧的 Draw 拿不到）。只有"面板已经打开、但触发器这一帧
// 还没画过"（壳直接打开 / 单测首帧）才落到这两个值：
//
//	左缘   8  = 侧栏根 padding（`uikernel.SidebarRootPadX`，触发器就在那一列里）
//	下缘  50  = 面板与窗口底的距离：官方是"触发器上沿 - 8"，而触发器高 42（CSS:24），
//	            8 + 42 = 50（设置行还在它下面，这里不猜那 40px）
const (
	cordisPanelFallbackLeft   float32 = 8
	cordisPanelFallbackBottom float32 = 50
	// cordisPanelEdgeMin 是面板左缘的最小留白（官方由 `max-width:calc(100vw-24px)` +
	// 固定的 420 宽保证不出横向滚动；viva 没有 clamp()，这里显式夹一次）。
	cordisPanelEdgeMin float32 = 12
)
