// model.go 是设置浮层的**数据模型**：从一帧的 Actions + Snapshot + 配置缓存
// 算出「这一帧该画哪几块、每块哪几行、哪一行是当前值」。
//
// 单独一个文件、不碰 flex 的目的：这里的每一条判断都能用普通单测钉住，
// 不需要开窗口。画法是 settings.go 的事。
//
// # 这个文件唯一的规矩：只画真值
//
//   - **Action 为 nil = 壳没接这个能力**：整块不画（不是画成灰的）。
//     画一个点了没反应的选项，比不画更糟——用户会以为"设了"。
//   - **当前值未知就不标选中**：不猜默认值、不拿别的字段顶替。
//     未知时行照常可点（点了是真动作），只是没有哪一行看起来"已经选上"。
//   - 读到的值不在宿主的白名单里时，照原样显示并说明，不硬塞进某一档。
package dssettings

import (
	"strings"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// Row 是浮层里的一行。
//
// Run == nil 表示**只读行**（会话 id / 标题 / 工作目录）：不挂命中区、
// 不参与键盘光标——它看起来就不该是个按钮。
type Row struct {
	// Key 是稳定的行标识（拼交互 id 用）。只读行也要给：日志/测试里好认。
	Key   string
	Label string
	Hint  string
	// Selected 是"这一行就是当前值"。**只有确知当前值时才可能为 true。**
	Selected bool
	Run      func()
}

// Section 是一块分区。Rows 为空时整块不画——空标题只会让人以为加载失败。
type Section struct {
	Title string
	// Note 是分区级的只读说明（当前值 / 未知 / 列表被截断）。
	Note string
	Rows []Row
}

// maxModelRows 是模型分区最多列几行。
//
// 真宿主的 models.list 是几十条（网关上的全部渠道）。浮层不是列表页：
// 塞满会把权限分区挤出屏幕。超出部分**不画成死行**，只在 Note 里写清还有几条
// ——用户能在命令面板里选到其余的（那里也是 8 条上限，但至少是个入口）。
const maxModelRows = 6

// themeOptions 是宿主认识的三种模式（cmd/app/theme.go 的 validThemeMode 是判据）。
//
// 界面只递这三个值：传别的壳不改主题、只写状态栏——那种"看起来能点但没用"的项
// 不该出现在设置里。
var themeOptions = []struct{ value, label, hint string }{
	{"dark", "深色", "官方深色令牌（ds-dark）"},
	{"light", "浅色", "官方浅色令牌（ds-light）"},
	{"system", "跟随系统", "系统明暗变了界面跟着变"},
}

// permissionOptions 是宿主的审批策略白名单（`session.setPermission`）。
//
// **不要在这里加"看起来合理"的词**：宿主会 400，而界面上看起来像设成功了。
// 顺序按合同的 deny / ask / allow / auto 排，和命令面板的文案保持一致。
var permissionOptions = []struct{ value, label, hint string }{
	{"deny", "只读", "写类工具一律拒绝（deny）"},
	{"ask", "每次都问我", "写类工具每次都要点允许（ask）"},
	{"allow", "自动允许", "写类工具不问直接跑（allow）"},
	{"auto", "按模式自动", "由当前模式决定（auto）"},
}

// HasSettingsActions 判断"这个壳到底接上了几个真实设置动作"。
//
// 这是设置浮层**存在的前提**：一个都接不上时，浮层没有可操作的内容，
// 入口也不该出现（F2 里不画「设置」、热键不响应）——空面板比没有面板更让人困惑。
//
// 模型要两条一起才算：只有列表没有 SetModel 是"能看不能选"，
// 只有 SetModel 没有列表是"能选但不知道选什么"（dscommands 用同一条判据）。
func HasSettingsActions(a pluginapi.Actions) bool {
	return a.SetTheme != nil ||
		a.SetPermission != nil ||
		(a.ModelsList != nil && a.SetModel != nil)
}

// validThemeMode 是 cmd/app 认识的三种模式。重复一份而不是 import cmd/app：
// 插件不许 import 壳（R2）。两边的判据由各自的测试守着。
func validThemeMode(mode string) bool {
	return mode == "dark" || mode == "light" || mode == "system"
}

// currentThemeMode 返回"用户选的模式"，未知时 ok=false。
//
// 两个来源，缺一不可：
//
//	① ConfigGet 的 `ui.theme`：壳把用户的选择落在这里（SetTheme 自己会写）。
//	   **没有这个键就是不知道**——新装的机器、只用 `-theme` 启动过的机器都没有它。
//	② 壳这一帧的真实解析结果 `themeDark`：用来**核对**①。
//	   `-theme light` 是显式启动参数，会压过 ui.theme（见 cmd/app/theme.go 的
//	   themeExplicit）；那时盘上留着旧值就是过期数据。拿它当"当前选中"就是伪造。
//
// 所以只有"配置说是深/浅，且和这一帧解析出来的一致"才算知道。
// `system` 不用核对：它解析成深或浅都对。
func currentThemeMode(cfg map[string]string, themeDark bool) (string, bool) {
	mode := strings.TrimSpace(cfg["ui.theme"])
	if !validThemeMode(mode) {
		return "", false
	}
	switch mode {
	case "dark":
		if !themeDark {
			return "", false
		}
	case "light":
		if themeDark {
			return "", false
		}
	}
	return mode, true
}

// permissionKnown 判断一个值是不是宿主白名单里的。
func permissionKnown(perm string) bool {
	for _, o := range permissionOptions {
		if o.value == perm {
			return true
		}
	}
	return false
}

// shortModel 去掉渠道前缀（`group/deepseek-v4.1-flash1017` → `deepseek-v4.1-flash1017`）。
//
// 和 dscommands 里的同名函数是**同一个理由**：渠道对"选哪个模型"没帮助。
// 不复用它是因为插件之间不该互相 import（那是把两个部件的内部实现焊在一起）；
// 5 行逻辑各留一份，谁改谁负责。
//
// 重名处理也照抄：不同渠道会有同名模型，去掉前缀就分不出来。这里对重名的
// 保留完整 id（带渠道）。做法见 modelRows。
func shortModel(id string) string {
	if i := strings.Index(id, "/"); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

// modelRows 把 models.list 的投影变成可点的行。
//
// 三件事：只列前 maxModelRows 条、重名的保留完整 id、当前模型高亮。
// **当前模型来自 Snapshot.Model**（宿主最近一次 request 的模型名）；空串 = 不知道，
// 那时一行都不高亮——不拿"列表第一条"冒充当前模型。
func modelRows(models []pluginapi.Model, current string, set func(string)) []Row {
	shown := models
	if len(shown) > maxModelRows {
		shown = shown[:maxModelRows]
	}
	// 先算"短名"，只对重名的退回完整 id。
	short := make([]string, len(shown))
	count := map[string]int{}
	for i, m := range shown {
		short[i] = shortModel(m.ID)
		count[short[i]]++
	}
	rows := make([]Row, 0, len(shown)+1)
	for i, m := range shown {
		label := short[i]
		if count[label] > 1 {
			label = m.ID
		}
		m := m
		rows = append(rows, Row{
			Key:      "model-" + m.ID,
			Label:    label,
			Hint:     m.OwnedBy,
			Selected: current != "" && m.ID == current,
			Run:      func() { set(m.ID) },
		})
	}
	// "恢复默认"是**真实动作**（空串 = 清除覆盖，回到宿主的 --model），
	// 所以它总在列表末尾；它不参与选中态——"当前是默认"这件事从快照里看不出来
	// （Snapshot.Model 只有名字，没有"是不是覆盖"这个信息）。
	rows = append(rows, Row{
		Key:   "model-default",
		Label: "默认模型",
		Hint:  "清除覆盖，回到宿主默认",
		Run:   func() { set("") },
	})
	return rows
}

// BuildSections 是浮层的全部内容。
//
// 入参就是绘制期能拿到的全部真值：
//
//	a    壳这一帧接上的动作（nil = 没接）
//	data 这一帧的快照（Model / Permission / Cur / Sessions / CWD）
//	cfg  ConfigGet 的缓存（可能为空 map —— 必须按"没有"处理）
//	themeDark 这一帧真正解析出来的主题明暗（核对 ui.theme 用）
func BuildSections(a pluginapi.Actions, data pluginapi.Snapshot, cfg map[string]string, themeDark bool) []Section {
	var out []Section

	// ── 外观 ───────────────────────────────────────────────────────────────
	if a.SetTheme != nil {
		set := a.SetTheme
		cur, known := currentThemeMode(cfg, themeDark)
		sec := Section{Title: "外观"}
		if !known {
			// 不知道就说不知道。写"当前：深色"会让只用过 `-theme` 的人
			// 看到一行假的选中态。
			sec.Note = "当前：未知（只读得到壳落盘的模式；这轮是启动参数定的）"
		}
		for _, o := range themeOptions {
			o := o
			sec.Rows = append(sec.Rows, Row{
				Key:      "theme-" + o.value,
				Label:    o.label,
				Hint:     o.hint,
				Selected: known && cur == o.value,
				Run:      func() { set(o.value) },
			})
		}
		out = append(out, sec)
	}

	// ── 模型 ───────────────────────────────────────────────────────────────
	if a.ModelsList != nil && a.SetModel != nil {
		set := a.SetModel
		if models := a.ModelsList(); len(models) > 0 {
			sec := Section{Title: "模型", Rows: modelRows(models, data.Model, set)}
			if data.Model == "" {
				sec.Note = "当前：未知（宿主还没给过这个会话的模型名）"
			} else {
				sec.Note = "当前：" + data.Model
				if !hasModel(models, data.Model) {
					// 真实会出现：宿主默认模型（--model）不一定在网关清单里。
					sec.Note += "（不在可选列表里）"
				}
			}
			if len(models) > maxModelRows {
				sec.Note += " · 另有 " + itoa(len(models)-maxModelRows) + " 个未列出"
			}
			out = append(out, sec)
		}
		// models.list 为空 = 壳没拉到（网关/宿主不通），这一块整块不画：
		// 画一个只有"默认模型"的分区，用户会以为可选模型只有一个。
	}

	// ── 权限 ───────────────────────────────────────────────────────────────
	if a.SetPermission != nil {
		set := a.SetPermission
		perm := strings.TrimSpace(data.Permission)
		known := permissionKnown(perm)
		sec := Section{Title: "权限"}
		switch {
		case perm == "":
			sec.Note = "当前：未知（宿主没给这一进程的审批策略）"
		case !known:
			// 宿主以后加档位时，界面不认识的词照原样显示——不硬塞进"自动允许"。
			sec.Note = "当前：" + perm + "（不在界面认识的四档里）"
		default:
			sec.Note = "当前：" + perm
		}
		for _, o := range permissionOptions {
			o := o
			sec.Rows = append(sec.Rows, Row{
				Key:      "perm-" + o.value,
				Label:    o.label,
				Hint:     o.hint,
				Selected: known && perm == o.value,
				Run:      func() { set(o.value) },
			})
		}
		// 恢复默认是**独立的一条**：没有它，用户设过一次就再也回不到宿主的
		// --policy（空串 = 清除覆盖）。它不参与选中态：快照只说"当前是什么"，
		// 说不出"是不是覆盖出来的"。
		sec.Rows = append(sec.Rows, Row{
			Key:   "perm-default",
			Label: "恢复默认",
			Hint:  "清除覆盖，回到宿主的 --policy",
			Run:   func() { set("") },
		})
		out = append(out, sec)
	}

	// ── 会话（只读）────────────────────────────────────────────────────────
	//
	// 它没有动作，也不依赖任何 Action：只要有当前会话就有内容。
	// **未知就不画那一行**（不写"未知"占位）：CWD 老宿主没有、标题可能为空。
	if data.Cur != "" {
		sec := Section{Title: "会话"}
		sec.Rows = append(sec.Rows, Row{Key: "session-id", Label: "id", Hint: data.Cur})
		if title := sessionTitle(data); title != "" {
			sec.Rows = append(sec.Rows, Row{Key: "session-title", Label: "标题", Hint: title})
		}
		if data.CWD != "" {
			sec.Rows = append(sec.Rows, Row{Key: "session-cwd", Label: "工作目录", Hint: data.CWD})
		}
		out = append(out, sec)
	}

	return out
}

// hasModel 判断 id 是否在清单里（决定 Note 里要不要写"不在可选列表"）。
func hasModel(models []pluginapi.Model, id string) bool {
	for _, m := range models {
		if m.ID == id {
			return true
		}
	}
	return false
}

// sessionTitle 从会话列表里找当前会话的标题；找不到就是空串（= 不画那一行）。
func sessionTitle(data pluginapi.Snapshot) string {
	for _, s := range data.Sessions {
		if s.ID == data.Cur {
			return strings.TrimSpace(s.Title)
		}
	}
	return ""
}

// itoa 只给 Note 里的条数用（标准库 strconv 也行，这里不引依赖）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
