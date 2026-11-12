package uidecl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// 这个文件只测**纯逻辑**：目录扫描、校验、模板变量、包装。
// draw 里那些 ui.Text(f, ...) 靠无头截图验收（tmp/decl-on.png / tmp/decl-off.png）。

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写 %s 失败: %v", p, err)
	}
	return p
}

// ── 目录扫描 ────────────────────────────────────────────────────────────

func TestLoadDirScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.json", `{"id":"a.one","slot":"sidebar.footer","order":90,"kind":"label","text":"甲"}`)
	writeFile(t, dir, "b.json", `{"id":"b.two","slot":"shell.bottom","kind":"dot","text":"乙"}`)
	// 不是 .json 的一律不看：目录里放 README 是常态，不该因此报错。
	writeFile(t, dir, "README.md", "# 说明")
	writeFile(t, dir, ".hidden.json", `{"id":"x","slot":"sidebar.footer","text":"隐藏"}`)

	decls, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir 出错: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("扫到 %d 份声明，want 2（%+v）", len(decls), decls)
	}
	// 顺序按文件名，跨机器稳定（截图对比要用）。
	if decls[0].ID != "a.one" || decls[1].ID != "b.two" {
		t.Fatalf("顺序不对: %s, %s", decls[0].ID, decls[1].ID)
	}
	if decls[0].Slot != pluginapi.SlotSidebarFooter || decls[0].Order != 90 || decls[0].Kind != KindLabel {
		t.Fatalf("a.json 解析错: %+v", decls[0])
	}
	// 没写 order 的落到 defaultOrder，不是 0（0 会跑到槽位最前面去）。
	if decls[1].Order != defaultOrder {
		t.Fatalf("缺省 order = %d，want %d", decls[1].Order, defaultOrder)
	}
	if !strings.HasSuffix(decls[0].File, "a.json") {
		t.Fatalf("File 没记下来源: %q", decls[0].File)
	}
}

func TestLoadDirMissingAndEmpty(t *testing.T) {
	dir := t.TempDir()

	// 空目录：0 份声明、0 个错误。
	decls, err := LoadDir(dir)
	if err != nil || len(decls) != 0 {
		t.Fatalf("空目录应该 (0, nil)，实际 (%d, %v)", len(decls), err)
	}

	// 目录不存在：这是**正常状态**（大多数机器根本没建这个目录）。
	decls, err = LoadDir(filepath.Join(dir, "nope"))
	if err != nil || len(decls) != 0 {
		t.Fatalf("目录不存在应该 (0, nil)，实际 (%d, %v)", len(decls), err)
	}

	// 空字符串：没有配置。
	decls, err = LoadDir("")
	if err != nil || len(decls) != 0 {
		t.Fatalf("空目录名应该 (0, nil)，实际 (%d, %v)", len(decls), err)
	}
}

func TestLoadDirBadJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.json", `{"id":"a.one","slot":"sidebar.footer","text":`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("坏 JSON 必须报错，不能静默跳过")
	}
	// 错误必须能定位文件——否则用户面对「装配失败」四个字无从下手。
	if !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("错误里没有文件名: %v", err)
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("错误里没说清是 JSON 坏了: %v", err)
	}
}

func TestLoadDirUnknownField(t *testing.T) {
	// 把 text 写成 tetx：不报错的话就是「以为自己挂上了」。
	dir := t.TempDir()
	writeFile(t, dir, "typo.json", `{"id":"a.one","slot":"sidebar.footer","tetx":"甲"}`)

	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "typo.json") {
		t.Fatalf("字段写错必须报错且带文件名，实际: %v", err)
	}
}

func TestLoadDirMissingID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "noid.json", `{"slot":"sidebar.footer","text":"甲"}`)

	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "noid.json") || !strings.Contains(err.Error(), "id") {
		t.Fatalf("缺 id 要报错并指出文件，实际: %v", err)
	}
}

func TestLoadDirDuplicateID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.json", `{"id":"same.id","slot":"sidebar.footer","text":"甲"}`)
	writeFile(t, dir, "b.json", `{"id":"same.id","slot":"shell.bottom","text":"乙"}`)

	_, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "same.id") {
		t.Fatalf("id 重复要报错，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "a.json") || !strings.Contains(err.Error(), "b.json") {
		t.Fatalf("id 重复的错误要指出两个文件，实际: %v", err)
	}
}

// ── 校验 ────────────────────────────────────────────────────────────────

func TestUnknownSlotListsAvailable(t *testing.T) {
	dir := t.TempDir()
	// 少一个 t：最容易犯的拼写错误。
	writeFile(t, dir, "x.json", `{"id":"a.one","slot":"sidebar.foter","text":"甲"}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("未知槽位必须报错——静默忽略会让第三方以为挂上了")
	}
	msg := err.Error()
	if !strings.Contains(msg, "sidebar.foter") {
		t.Fatalf("错误里要回显写错的槽位: %v", err)
	}
	for _, want := range []string{"sidebar.footer", "shell.bottom", "main.header"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("错误里要列出可用槽位（缺 %s）: %v", want, err)
		}
	}
	if !strings.Contains(msg, "x.json") {
		t.Fatalf("错误里要有文件名: %v", err)
	}
}

func TestSlotAcceptsConstName(t *testing.T) {
	// 照着文档抄常量名也该能用。
	d, err := parseJSON("x.json", []byte(`{"id":"a","slot":"SlotSidebarFooter","text":"甲"}`))
	if err != nil {
		t.Fatalf("常量名形式的槽位应该被接受: %v", err)
	}
	if d.Slot != pluginapi.SlotSidebarFooter {
		t.Fatalf("槽位解析错: %q", d.Slot)
	}
}

func TestUnknownKindAndColor(t *testing.T) {
	if _, err := parseJSON("x.json", []byte(`{"id":"a","slot":"sidebar.footer","kind":"lable","text":"甲"}`)); err == nil ||
		!strings.Contains(err.Error(), "keyvalue") {
		t.Fatalf("未知 kind 要报错并列出可用 kind，实际: %v", err)
	}
	if _, err := parseJSON("x.json", []byte(`{"id":"a","slot":"sidebar.footer","text":"甲","color":"green"}`)); err == nil ||
		!strings.Contains(err.Error(), "label-secondary") {
		t.Fatalf("未知颜色要报错并列出可用颜色，实际: %v", err)
	}
	for name := range colorTable {
		body := `{"id":"a","slot":"sidebar.footer","text":"甲","color":"` + name + `"}`
		if _, err := parseJSON("x.json", []byte(body)); err != nil {
			t.Fatalf("颜色 %s 应该被接受: %v", name, err)
		}
	}
}

func TestFieldsThatWouldBeIgnoredAreErrors(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"label 给 label 用", `{"id":"a","slot":"sidebar.footer","kind":"label","label":"左","text":"右"}`, "label"},
		{"items 给 dot 用", `{"id":"a","slot":"sidebar.footer","kind":"dot","items":["甲"],"text":"乙"}`, "items"},
		{"缺 text", `{"id":"a","slot":"sidebar.footer","kind":"label"}`, "text"},
		{"list 没内容", `{"id":"a","slot":"sidebar.footer","kind":"list"}`, "items"},
		{"height 是负数", `{"id":"a","slot":"sidebar.footer","text":"甲","height":-1}`, "height"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseJSON("x.json", []byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("应该报含 %q 的错，实际: %v", tc.want, err)
			}
		})
	}
}

func TestKeyValueAndListAccepted(t *testing.T) {
	d, err := parseJSON("x.json", []byte(`{"id":"a","slot":"rightbar","kind":"keyvalue","label":"会话","text":"{sessions}"}`))
	if err != nil {
		t.Fatalf("keyvalue 应该被接受: %v", err)
	}
	if d.Label != "会话" {
		t.Fatalf("label 没解出来: %+v", d)
	}

	d, err = parseJSON("x.json", []byte(`{"id":"b","slot":"rightbar","kind":"list","items":["甲","乙"]}`))
	if err != nil {
		t.Fatalf("list + items 应该被接受: %v", err)
	}
	if len(d.lines()) != 2 {
		t.Fatalf("items 没进 lines: %+v", d.lines())
	}

	d, err = parseJSON("x.json", []byte(`{"id":"c","slot":"rightbar","kind":"list","text":"甲\n乙"}`))
	if err != nil {
		t.Fatalf("list + 换行 text 应该被接受: %v", err)
	}
	if len(d.lines()) != 2 || d.lines()[1] != "乙" {
		t.Fatalf("按换行分行不对: %+v", d.lines())
	}
}

// ── 模板变量 ────────────────────────────────────────────────────────────

func testSnapshot() pluginapi.Snapshot {
	return pluginapi.Snapshot{
		Sessions: []apiclient.Session{
			{ID: "s1", Title: "第一个会话"},
			{ID: "s2", Title: "第二个会话"},
			{ID: "s3", Title: "第三个会话"},
		},
		Cur:    "s2",
		Status: "已连接",
		Msgs:   []apiclient.Message{{Seq: 1}, {Seq: 2}},
		Total:  7,
		Usage:  pluginapi.Usage{TotalTokens: 1500},
	}
}

func TestExpand(t *testing.T) {
	snap := testSnapshot()
	cases := []struct{ name, in, want string }{
		{"会话数", "共 {sessions} 个会话", "共 3 个会话"},
		{"当前标题", "{cur_title}", "第二个会话"},
		{"消息数", "消息 {messages}", "消息 7"},
		{"状态栏文字", "状态：{status}", "状态：已连接"},
		{"token 压缩", "上下文 {tokens}", "上下文 1.5k"},
		{"多个变量", "{cur_title} · {messages}/{sessions}", "第二个会话 · 7/3"},
		{"认不出来的变量换成空串", "a{nope}b", "ab"},
		{"大括号转义", "{{literal}}", "{literal}"},
		{"没有变量就原样", "纯文本", "纯文本"},
		{"空模板", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Expand(tc.in, snap); got != tc.want {
				t.Fatalf("Expand(%q) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandNeverLeaksPlaceholder(t *testing.T) {
	// 界面上出现 {xxx} 只会让用户以为界面坏了，而且不知道坏在哪。
	// 所以任何花括号变量都不许原样漏出去。
	snap := testSnapshot()
	for _, in := range []string{"{nope}", "{cur_title}", "{中文变量}", "{a-b.c}", "{tokens}"} {
		out := Expand(in, snap)
		if strings.ContainsAny(out, "{}") {
			t.Fatalf("Expand(%q) = %q，花括号漏出来了", in, out)
		}
	}
}

func TestExpandEmptyValues(t *testing.T) {
	// 没打开会话 / 宿主没给 usage：变量查不到就是空串，不是 "0"。
	empty := pluginapi.Snapshot{}
	cases := map[string]string{
		"{cur_title}": "",
		"{status}":    "",
		"{tokens}":    "",
		"{sessions}":  "0",
		"{messages}":  "0",
	}
	for in, want := range cases {
		if got := Expand(in, empty); got != want {
			t.Fatalf("空快照 Expand(%q) = %q，want %q", in, got, want)
		}
	}
}

func TestMessagesFallsBackToLen(t *testing.T) {
	// 壳一般会给 Total，但它为 0 而 Msgs 非空时（老数据 / 别的壳）要能兜住。
	snap := pluginapi.Snapshot{Msgs: []apiclient.Message{{Seq: 1}, {Seq: 2}, {Seq: 3}}}
	if got := Expand("{messages}", snap); got != "3" {
		t.Fatalf("Expand = %q，want 3", got)
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"}, {999, "999"}, {1000, "1.0k"}, {6500, "6.5k"},
		{190800, "190.8k"}, {1000000, "1.0M"}, {1500000, "1.5M"},
	}
	for _, tc := range cases {
		if got := formatTokens(tc.in); got != tc.want {
			t.Fatalf("formatTokens(%d) = %q，want %q", tc.in, got, tc.want)
		}
	}
}

// ── 包装成插件 ──────────────────────────────────────────────────────────

func TestPluginPainter(t *testing.T) {
	d, err := parseJSON("x.json", []byte(`{"id":"hello.badge","slot":"sidebar.footer","order":90,"kind":"dot","text":"外部 JSON 插件 · {sessions} 个会话"}`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	p := d.Plugin(&pluginapi.Theme{}, nil)
	if p.ID() != "hello.badge" {
		t.Fatalf("ID = %q", p.ID())
	}
	if got := p.Requires(); len(got) != 2 || got[0] != pluginapi.KeyTheme || got[1] != pluginapi.KeyPrimitives {
		t.Fatalf("Requires = %v", got)
	}
	if err := p.Register(nil); err != nil {
		t.Fatalf("Register 应该是空操作: %v", err)
	}
	with, ok := p.(pluginapi.Panels)
	if !ok {
		t.Fatal("声明部件必须实现 Panels")
	}
	panels := with.Panels()
	if len(panels) != 1 {
		t.Fatalf("一个声明 = 一块画，实际 %d", len(panels))
	}
	if panels[0].Slot != pluginapi.SlotSidebarFooter || panels[0].Order != 90 || panels[0].Draw == nil {
		t.Fatalf("画错了: %+v", panels[0])
	}
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir 出错: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(dir), "dsh-ui/"+SubDir) {
		t.Fatalf("默认目录形状不对: %q", dir)
	}
}

func TestSlotsAndColorsSorted(t *testing.T) {
	// 报错信息要稳定（测试、截图对比都用得上）。
	if got := Slots(); len(got) != len(slotTable) {
		t.Fatalf("Slots 数量 %d，want %d", len(got), len(slotTable))
	}
	for i := 1; i < len(Slots()); i++ {
		if Slots()[i-1] > Slots()[i] {
			t.Fatalf("Slots 没排序: %v", Slots())
		}
	}
	for i := 1; i < len(Colors()); i++ {
		if Colors()[i-1] > Colors()[i] {
			t.Fatalf("Colors 没排序: %v", Colors())
		}
	}
	e := &Decl{Kind: KindDot}
	if _, ok := colorTable[e.Color]; ok {
		t.Fatal("空颜色名不该命中颜色表")
	}
}
