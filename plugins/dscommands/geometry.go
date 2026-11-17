package dscommands

// 命令面板几何的唯一来源：官方 0.2.2-rc.2。
// PopupSelectView.module.css 是 ui-commands 自己唯一的 *.module.css；
// 图标尺寸由同一命令目录实际渲染它的 ui-input-trigger/MenuView.module.css 钉住。
const (
	// commandPanelMaxHeight 是 `.card{max-height:320px}`。
	// OFF:dsh-client-ui-commands/src/client/PopupSelectView.module.css:19-21
	commandPanelMaxHeight float32 = 320
	// commandPanelPad 是 `.card{padding:4px}`；同文件 :12。
	commandPanelPad float32 = 4
	// commandPanelAnchorGap 是 `.card{bottom:calc(100% + 4px)}`；同文件 :6-9。
	commandPanelAnchorGap float32 = 4

	// commandRowGap 是 `.row{gap:6px}`；同文件 :40-44。
	commandRowGap float32 = 6
	// commandRowPadY / commandRowPadX 是 `.row{padding:5px 7px}`；同文件 :44。
	commandRowPadY float32 = 5
	commandRowPadX float32 = 7
	// commandRowFont 是 `.row{font-size:12px}`；同文件 :47。
	commandRowFont float32 = 12
	// commandDetailFont 是 `.detail{font-size:11px}`；同文件 :81-85。
	commandDetailFont float32 = 11

	// commandStatusPadY / commandStatusPadX 是 `.status{padding:7px 8px}`；同文件 :103-106。
	commandStatusPadY float32 = 7
	commandStatusPadX float32 = 8
	// commandStatusFont 是 `.status{font-size:12px}`；同文件 :105。
	commandStatusFont float32 = 12

	// 搜索框四边 margin 是 `.search{margin:2px 2px 3px}`；同文件 :109-110。
	commandSearchMarginTop    float32 = 2
	commandSearchMarginRight  float32 = 2
	commandSearchMarginBottom float32 = 3
	commandSearchMarginLeft   float32 = 2
	// commandSearchPadY / commandSearchPadX 是 `.search{padding:5px 7px}`；同文件 :111。
	commandSearchPadY float32 = 5
	commandSearchPadX float32 = 7
	// commandSearchBorder 是 `.search{border:0.5px solid transparent}`；同文件 :112。
	commandSearchBorder float32 = 0.5
	// commandSearchFont 是 `.search{font-size:12px}`；同文件 :115。
	commandSearchFont float32 = 12

	// commandIconSize 是 `.itemIcon{width/height:14px}` 与 `.itemIcon svg{14×14}`。
	// OFF:dsh-client-ui-input-trigger/src/client/MenuView.module.css:88-101
	// 调用处同样传 size={14}：OFF:.../MenuView.tsx:179-184。
	commandIconSize float32 = 14
)

// 官方中文文案：ui-commands/src/client/locales.ts:32-33。
const (
	commandSearchPlaceholder = "搜索…"
	commandSearchAria        = "筛选选项"
)
