package dssettings

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// 这一组测试钉住三件事：
//
//	① 缺 Action 就**整块不画**（不画死按钮）
//	② 每一行点下去，收到的值**是它自己那一个**（不是循环里最后一个）
//	③ 当前值只来自真值；真值不知道时**不标选中**（不伪造）
//
// 它们都不开窗口：判断全在 model.go 里，画法在视觉测试里验。

// fullActions 是"壳什么都接上了"的一份 Actions。
//
// 每个动作都记下收到的值，测试直接比对，不需要看界面。
type recorder struct {
	theme string
	model string
	perm  string
	// 记录调用次数，用来区分"没调用"和"调用成空串"
	themeN, modelN, permN int
}

func (r *recorder) actions(models []pluginapi.Model) pluginapi.Actions {
	return pluginapi.Actions{
		SetTheme:      func(mode string) { r.theme, r.themeN = mode, r.themeN+1 },
		ModelsList:    func() []pluginapi.Model { return models },
		SetModel:      func(id string) { r.model, r.modelN = id, r.modelN+1 },
		SetPermission: func(p string) { r.perm, r.permN = p, r.permN+1 },
	}
}

func sectionByTitle(secs []Section, title string) (Section, bool) {
	for _, s := range secs {
		if s.Title == title {
			return s, true
		}
	}
	return Section{}, false
}

func rowByKey(sec Section, key string) (Row, bool) {
	for _, r := range sec.Rows {
		if r.Key == key {
			return r, true
		}
	}
	return Row{}, false
}

func selectedKeys(secs []Section) []string {
	var out []string
	for _, s := range secs {
		for _, r := range s.Rows {
			if r.Selected {
				out = append(out, r.Key)
			}
		}
	}
	return out
}

// TestNoSettingsActionsMeansNoEntryAndNoPanel 是"缺 Action 不画"的第一层：
// 一个设置动作都没有时，Shift+F2 不响应、Open 打不开、也不存在外观/模型/权限分区。
func TestNoSettingsActionsMeansNoEntryAndNoPanel(t *testing.T) {
	p := &Plugin{}
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF2, Modifiers: gfx.ModifierShift}) {
		t.Fatal("没有任何设置动作时，Shift+F2 不该被吃掉")
	}
	if p.Open() {
		t.Fatal("没有任何设置动作时，Open 必须返回 false")
	}
	if p.Shown() {
		t.Fatal("没有任何设置动作时，浮层不该打开")
	}
	secs := BuildSections(pluginapi.Actions{}, pluginapi.Snapshot{}, nil, true)
	if len(secs) != 0 {
		t.Fatalf("没有动作、没有会话时该一块都不画，得到 %+v", secs)
	}
}

// TestPlainF2AlwaysFallsThrough 钉住"不和命令面板打架"：
// 普通 F2（以及带 Ctrl 的 F2）无论浮层开没开都必须放行给 dscommands。
func TestPlainF2AlwaysFallsThrough(t *testing.T) {
	r := &recorder{}
	p := &Plugin{}
	p.act = r.actions([]pluginapi.Model{{ID: "m1"}})
	for _, ev := range []gfx.KeyEvent{
		{Key: gfx.KeyF2},
		{Key: gfx.KeyF2, Modifiers: gfx.ModifierControl},
		{Key: gfx.KeyF2, Modifiers: gfx.ModifierShift | gfx.ModifierControl},
	} {
		if p.HandleKey(ev) {
			t.Fatalf("ev=%+v 不该被设置浮层吃掉（F2 是命令面板的）", ev)
		}
	}
}

// TestShiftF2OpensAndEscapeCloses 是入口本身：Shift+F2 开、Esc 关。
func TestShiftF2OpensAndEscapeCloses(t *testing.T) {
	r := &recorder{}
	p := &Plugin{}
	p.act = r.actions([]pluginapi.Model{{ID: "m1"}})

	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyF2, Modifiers: gfx.ModifierShift}) {
		t.Fatal("有设置动作时 Shift+F2 应打开浮层")
	}
	if !p.Shown() {
		t.Fatal("Shift+F2 之后浮层没打开")
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("浮层开着时 Esc 应被吃掉并关闭它")
	}
	if p.Shown() {
		t.Fatal("Esc 之后浮层还开着")
	}
	// 关掉之后再按 Esc 必须放行——那是审批拒绝 / 停止 turn 的键。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEscape}) {
		t.Fatal("浮层关着时 Esc 不该被吞（会顶掉审批拒绝/停止 turn）")
	}
}

// TestMissingActionHidesWholeSection 钉住"整块隐藏"：
// 只有哪几类动作，就只有哪几块；模型那一块要两条动作同时存在才算。
func TestMissingActionHidesWholeSection(t *testing.T) {
	cases := []struct {
		name string
		act  pluginapi.Actions
		want []string
		gone []string
	}{
		{
			name: "只有主题",
			act:  pluginapi.Actions{SetTheme: func(string) {}},
			want: []string{"外观"},
			gone: []string{"模型", "权限"},
		},
		{
			name: "只有权限",
			act:  pluginapi.Actions{SetPermission: func(string) {}},
			want: []string{"权限"},
			gone: []string{"外观", "模型"},
		},
		{
			name: "模型只有列表没有 SetModel",
			act:  pluginapi.Actions{ModelsList: func() []pluginapi.Model { return []pluginapi.Model{{ID: "m"}} }},
			want: nil,
			gone: []string{"外观", "模型", "权限"},
		},
		{
			name: "模型只有 SetModel 没有列表",
			act:  pluginapi.Actions{SetModel: func(string) {}},
			want: nil,
			gone: []string{"外观", "模型", "权限"},
		},
		{
			name: "模型动作齐全但列表是空（壳没拉到）",
			act: pluginapi.Actions{
				ModelsList: func() []pluginapi.Model { return nil },
				SetModel:   func(string) {},
			},
			want: nil,
			gone: []string{"外观", "模型", "权限"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			secs := BuildSections(c.act, pluginapi.Snapshot{}, nil, true)
			for _, title := range c.want {
				if _, ok := sectionByTitle(secs, title); !ok {
					t.Fatalf("%s 分区该在，实际 %+v", title, secs)
				}
			}
			for _, title := range c.gone {
				if _, ok := sectionByTitle(secs, title); ok {
					t.Fatalf("%s 分区不该在（对应 Action 缺失）", title)
				}
			}
		})
	}
}

// TestThemeRowsCarryTheirOwnValue 是那个经典坑：`for _, o := range` 里不复制变量，
// 三个闭包会共享最后一个值——症状不是崩溃，是"点哪个都变成跟随系统"。
func TestThemeRowsCarryTheirOwnValue(t *testing.T) {
	r := &recorder{}
	secs := BuildSections(r.actions(nil), pluginapi.Snapshot{}, nil, true)
	sec, ok := sectionByTitle(secs, "外观")
	if !ok {
		t.Fatal("外观分区不在")
	}
	want := map[string]string{"theme-dark": "dark", "theme-light": "light", "theme-system": "system"}
	if len(sec.Rows) != len(want) {
		t.Fatalf("外观应有 %d 行，得到 %d：%+v", len(want), len(sec.Rows), sec.Rows)
	}
	for key, value := range want {
		row, ok := rowByKey(sec, key)
		if !ok {
			t.Fatalf("缺 %s 行", key)
		}
		r.theme, r.themeN = "", 0
		row.Run()
		if r.themeN != 1 || r.theme != value {
			t.Fatalf("%s 点下去应收到 %q，得到 %q（调用 %d 次）", key, value, r.theme, r.themeN)
		}
	}
}

// TestThemeSelectionOnlyFromTruth 钉住"未知就不伪造选中"。
func TestThemeSelectionOnlyFromTruth(t *testing.T) {
	cases := []struct {
		name      string
		cfg       map[string]string
		themeDark bool
		want      []string
	}{
		{"没有配置", nil, true, nil},
		{"配置说深且这帧就是深", map[string]string{"ui.theme": "dark"}, true, []string{"theme-dark"}},
		{"配置说浅且这帧就是浅", map[string]string{"ui.theme": "light"}, false, []string{"theme-light"}},
		{"配置说跟随系统", map[string]string{"ui.theme": "system"}, true, []string{"theme-system"}},
		{"配置说系统且解析成浅", map[string]string{"ui.theme": "system"}, false, []string{"theme-system"}},
		{"配置说深但这帧是浅（-theme 覆盖了旧配置）", map[string]string{"ui.theme": "dark"}, false, nil},
		{"配置说浅但这帧是深", map[string]string{"ui.theme": "light"}, true, nil},
		{"配置里是不认识的词", map[string]string{"ui.theme": "sepia"}, true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			secs := BuildSections(r.actions(nil), pluginapi.Snapshot{}, c.cfg, c.themeDark)
			sec, _ := sectionByTitle(secs, "外观")
			got := selectedKeys([]Section{sec})
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("选中态 = %v，期望 %v", got, c.want)
			}
			if len(c.want) == 0 && sec.Note == "" {
				t.Fatal("未知时必须说明「当前：未知」，不能只靠不选中表达")
			}
		})
	}
}

// TestModelRowsAreTheRealIDs 钉住"点哪个模型就传哪个 id"（含渠道前缀）：
// 去掉前缀只是**显示**，回传的必须是 models.list 里的原样 id。
func TestModelRowsAreTheRealIDs(t *testing.T) {
	models := []pluginapi.Model{
		{ID: "group/deepseek", OwnedBy: "group"},
		{ID: "streamlake/deepseek", OwnedBy: "streamlake"},
		{ID: "group/model-free", OwnedBy: "group"},
	}
	r := &recorder{}
	secs := BuildSections(r.actions(models), pluginapi.Snapshot{}, nil, true)
	sec, ok := sectionByTitle(secs, "模型")
	if !ok {
		t.Fatal("模型分区不在")
	}
	if len(sec.Rows) != len(models)+1 {
		t.Fatalf("应有 %d 个模型行 + 默认，得到 %d：%+v", len(models), len(sec.Rows), sec.Rows)
	}
	// 同名不同渠道：两条都得保留完整 id，否则用户没法知道自己在点哪个。
	for _, want := range []string{"group/deepseek", "streamlake/deepseek"} {
		row, ok := rowByKey(sec, "model-"+want)
		if !ok {
			t.Fatalf("缺 %s 行", want)
		}
		if row.Label != want {
			t.Fatalf("重名模型 %s 的标签应保留渠道，得到 %q", want, row.Label)
		}
		r.model, r.modelN = "", 0
		row.Run()
		if r.modelN != 1 || r.model != want {
			t.Fatalf("点 %s 应回传 %q，得到 %q", want, want, r.model)
		}
	}
	// 唯一的名字去掉前缀。
	row, _ := rowByKey(sec, "model-group/model-free")
	if row.Label != "model-free" {
		t.Fatalf("唯一模型应显示短名，得到 %q", row.Label)
	}
	// 默认模型：空串 = 清除覆盖。
	def, ok := rowByKey(sec, "model-default")
	if !ok {
		t.Fatal("缺「默认模型」行")
	}
	r.model, r.modelN = "sentinel", 0
	def.Run()
	if r.modelN != 1 || r.model != "" {
		t.Fatalf("默认模型应回传空串，得到 %q", r.model)
	}
}

// TestModelSelectionOnlyFromSnapshot 钉住"当前模型来自 Snapshot"。
func TestModelSelectionOnlyFromSnapshot(t *testing.T) {
	models := []pluginapi.Model{{ID: "group/a"}, {ID: "b"}}
	cases := []struct {
		name   string
		model  string
		want   []string
		noteIn string
	}{
		{"知道就选中", "b", []string{"model-b"}, "当前：b"},
		{"不知道就一行都不选", "", nil, "未知"},
		{"不在列表里就不选，但要说明", "other", nil, "不在可选列表里"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			secs := BuildSections(r.actions(models), pluginapi.Snapshot{Model: c.model}, nil, true)
			sec, _ := sectionByTitle(secs, "模型")
			got := selectedKeys([]Section{sec})
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("选中态 = %v，期望 %v", got, c.want)
			}
			if !strings.Contains(sec.Note, c.noteIn) {
				t.Fatalf("Note = %q，应包含 %q", sec.Note, c.noteIn)
			}
		})
	}
}

// TestModelListCappedWithoutDeadRows：超过上限的部分**不画成行**，只写条数。
func TestModelListCappedWithoutDeadRows(t *testing.T) {
	var models []pluginapi.Model
	for i := 0; i < 9; i++ {
		models = append(models, pluginapi.Model{ID: "m" + itoa(i)})
	}
	r := &recorder{}
	secs := BuildSections(r.actions(models), pluginapi.Snapshot{}, nil, true)
	sec, _ := sectionByTitle(secs, "模型")
	if len(sec.Rows) != maxModelRows+1 {
		t.Fatalf("行数 = %d，期望 %d（上限 + 默认）", len(sec.Rows), maxModelRows+1)
	}
	if !strings.Contains(sec.Note, "另有 3 个未列出") {
		t.Fatalf("Note = %q，应写明还有 3 个未列出", sec.Note)
	}
}

// TestPermissionRowsCarryHostWhitelistValues：五条（四档 + 恢复默认），值必须是
// 宿主白名单里的原词，恢复默认是空串。
func TestPermissionRowsCarryHostWhitelistValues(t *testing.T) {
	r := &recorder{}
	secs := BuildSections(r.actions(nil), pluginapi.Snapshot{}, nil, true)
	sec, ok := sectionByTitle(secs, "权限")
	if !ok {
		t.Fatal("权限分区不在")
	}
	want := map[string]string{
		"perm-deny": "deny", "perm-ask": "ask", "perm-allow": "allow",
		"perm-auto": "auto", "perm-default": "",
	}
	if len(sec.Rows) != len(want) {
		t.Fatalf("权限应有 %d 行，得到 %d：%+v", len(want), len(sec.Rows), sec.Rows)
	}
	for key, value := range want {
		row, ok := rowByKey(sec, key)
		if !ok {
			t.Fatalf("缺 %s 行", key)
		}
		r.perm, r.permN = "sentinel", 0
		row.Run()
		if r.permN != 1 || r.perm != value {
			t.Fatalf("%s 点下去应收到 %q，得到 %q", key, value, r.perm)
		}
	}
}

// TestPermissionSelectionOnlyFromSnapshot：当前权限来自 Snapshot.Permission，
// 未知 / 不在白名单里时都不许选中任何一档。
func TestPermissionSelectionOnlyFromSnapshot(t *testing.T) {
	cases := []struct {
		name   string
		perm   string
		want   []string
		noteIn string
	}{
		{"知道就选中", "ask", []string{"perm-ask"}, "当前：ask"},
		{"不知道就一行都不选", "", nil, "未知"},
		{"宿主给了界面不认识的档", "never", nil, "不在界面认识的四档里"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &recorder{}
			secs := BuildSections(r.actions(nil), pluginapi.Snapshot{Permission: c.perm}, nil, true)
			sec, _ := sectionByTitle(secs, "权限")
			got := selectedKeys([]Section{sec})
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("选中态 = %v，期望 %v", got, c.want)
			}
			if !strings.Contains(sec.Note, c.noteIn) {
				t.Fatalf("Note = %q，应包含 %q", sec.Note, c.noteIn)
			}
		})
	}
}

// TestSessionSectionIsReadOnly：会话 id / 标题 / cwd 只读——没有 Run、不参与光标。
func TestSessionSectionIsReadOnly(t *testing.T) {
	data := pluginapi.Snapshot{
		Cur: "sess-1",
		CWD: `C:\work\demo`,
		Sessions: []apiclient.Session{
			{ID: "other", Title: "别的会话"},
			{ID: "sess-1", Title: "设置浮层"},
		},
	}
	secs := BuildSections(pluginapi.Actions{}, data, nil, true)
	sec, ok := sectionByTitle(secs, "会话")
	if !ok {
		t.Fatal("有当前会话时应有会话分区")
	}
	var text []string
	for _, r := range sec.Rows {
		if r.Run != nil {
			t.Fatalf("%s 不该可点", r.Key)
		}
		text = append(text, r.Hint)
	}
	want := []string{"sess-1", "设置浮层", `C:\work\demo`}
	if strings.Join(text, "|") != strings.Join(want, "|") {
		t.Fatalf("会话行 = %v，期望 %v", text, want)
	}
	// 未知的 CWD / 标题不画那一行，而不是画"未知"。
	secs = BuildSections(pluginapi.Actions{}, pluginapi.Snapshot{Cur: "sess-1"}, nil, true)
	sec, _ = sectionByTitle(secs, "会话")
	if len(sec.Rows) != 1 {
		t.Fatalf("只有 id 可画时不该多出行，得到 %+v", sec.Rows)
	}
}

// TestKeyboardMovesOnlyBetweenClickableRows：↑↓ 只在可点行之间走，只读行不占号；
// 浮层关着时 ↑↓ 必须放行（侧栏要用）。
func TestKeyboardMovesOnlyBetweenClickableRows(t *testing.T) {
	r := &recorder{}
	p := &Plugin{}
	p.act = r.actions([]pluginapi.Model{{ID: "m1"}})
	p.rows = []Row{{Key: "a", Run: func() { r.theme = "a" }}, {Key: "b", Run: func() { r.theme = "b" }}}

	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) {
		t.Fatal("浮层关着时 ↓ 不该被吞")
	}
	p.show("设置已打开")
	// 打开时**没有光标**：第一行不该因为"光标默认在 0"看起来像当前值。
	if p.cursor != -1 {
		t.Fatalf("打开时不该有光标，得到 %d", p.cursor)
	}
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown})
	if p.cursor != 0 {
		t.Fatalf("第一次 ↓ 应落到 0，得到 %d", p.cursor)
	}
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) // 1
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown}) // 回绕到 0
	if p.cursor != 0 {
		t.Fatalf("↓ 应回绕到 0，得到 %d", p.cursor)
	}
	p.HandleKey(gfx.KeyEvent{Key: gfx.KeyUp})
	if p.cursor != 1 {
		t.Fatalf("↑ 应回绕到 1，得到 %d", p.cursor)
	}
	if !p.HandleKey(gfx.KeyEvent{Key: gfx.KeyEnter}) {
		t.Fatal("浮层开着时 Enter 应被吃掉")
	}
	if r.theme != "b" {
		t.Fatalf("Enter 应执行光标那一行（b），得到 %q", r.theme)
	}
	// 带修饰键的组合一律放行（Ctrl+Shift+C 复制之类）。
	if p.HandleKey(gfx.KeyEvent{Key: gfx.KeyDown, Modifiers: gfx.ModifierControl}) {
		t.Fatal("带修饰键的方向键不该被吞")
	}
}

// TestOpenRequiresRealActions：`-open-settings` 那条路也不能开出空面板。
func TestOpenRequiresRealActions(t *testing.T) {
	p := &Plugin{}
	if p.Open() {
		t.Fatal("没有动作时 Open 不该成功")
	}
	r := &recorder{}
	p.act = r.actions(nil) // 有 SetTheme 等，列表为空也算有设置动作
	if p.act.ModelsList() == nil {
		_ = p.act
	}
	p.act = pluginapi.Actions{SetPermission: func(string) {}}
	if !p.Open() {
		t.Fatal("有权限动作时 Open 应成功")
	}
	if !p.Shown() {
		t.Fatal("Open 之后浮层应开着")
	}
	p.Close()
	if p.Shown() {
		t.Fatal("Close 之后浮层应关着")
	}
}

// TestShortModelKeepsChannelForDuplicates：名字唯一才去前缀，重名保留完整 id。
func TestShortModelKeepsChannelForDuplicates(t *testing.T) {
	if got := shortModel("group/deepseek"); got != "deepseek" {
		t.Fatalf("shortModel = %q", got)
	}
	if got := shortModel("plain"); got != "plain" {
		t.Fatalf("没有渠道前缀时应原样，得到 %q", got)
	}
	if got := shortModel("group/"); got != "group/" {
		t.Fatalf("前缀后面什么都没有时应原样，得到 %q", got)
	}
}
