package dscomposer

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// mention_test.go 测 `@` 补全。
//
// 分两层：**纯函数**（认 token、补全、筛候选）+ **一帧真布局**（菜单真的弹出来、
// 真的点得动、Enter 只在有菜单时才被拿走）。第二层比第一层重要——
// 纯函数全绿而菜单根本不画，是完全可能的（插槽、布局、id 三处都可能断）。

func TestParseAt(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		ok     bool
		start  int
		dir    string
		prefix string
	}{
		{name: "行首", in: "@", ok: true, start: 0, dir: "", prefix: ""},
		{name: "行首带名字", in: "@ds", ok: true, start: 0, dir: "", prefix: "ds"},
		// ⚠️ start 是**字节**下标，不是字符下标：`看下 ` 是 3+3+1 = 7 个字节。
		// 我第一版按字符数写了 3，测试立刻红了——这正是 Go 里最容易写错的一类东西，
		// 所以把这个样本留在表里。
		{name: "空格之后（中文按字节算下标）", in: "看下 @tmp/a", ok: true, start: 7, dir: "tmp/", prefix: "a"},
		{name: "已经进到一层", in: "@plugins/dsopen/", ok: true, start: 0, dir: "plugins/dsopen/", prefix: ""},

		// 下面四条是**不该认**的。它们决定了"什么时候 Enter 还能发送"。
		{name: "邮箱不算", in: "发到 a@b.com", ok: false},
		{name: "token 已被空格结束", in: "@tmp/a 然后呢", ok: false},
		{name: "没有 @", in: "普通一句话", ok: false},
		{name: "空文本", in: "", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := parseAt(tc.in)
			if ok != tc.ok {
				t.Fatalf("parseAt(%q) ok = %v，want %v", tc.in, ok, tc.ok)
			}
			if !ok {
				return
			}
			if ref.Start != tc.start || ref.Dir != tc.dir || ref.Prefix != tc.prefix {
				t.Errorf("parseAt(%q) = {start:%d dir:%q prefix:%q}，want {start:%d dir:%q prefix:%q}",
					tc.in, ref.Start, ref.Dir, ref.Prefix, tc.start, tc.dir, tc.prefix)
			}
		})
	}
}

func TestCompleteAt(t *testing.T) {
	// 补全要**只动那个 token**——前后的话一个字都不能少。
	// 这是"补全"和"替换整段"的分界：写错成后者，用户前面打的话就没了。
	cases := []struct {
		name string
		text string
		dir  string
		file string
		isD  bool
		want string
	}{
		{name: "文件", text: "@ds", dir: "", file: "dshd.go", want: "@dshd.go"},
		{name: "目录补斜杠", text: "@ds", dir: "", file: "dsopen", isD: true, want: "@dsopen/"},
		{name: "保留前面的正文", text: "看下 @tmp/a", dir: "tmp/", file: "abc.txt", want: "看下 @tmp/abc.txt"},
		{name: "进到下一层", text: "@plugins/ds", dir: "plugins/", file: "dsopen", isD: true, want: "@plugins/dsopen/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := parseAt(tc.text)
			if !ok {
				t.Fatalf("parseAt(%q) 没认出来，样本本身有问题", tc.text)
			}
			if got := completeAt(tc.text, ref, tc.file, tc.isD); got != tc.want {
				t.Errorf("completeAt = %q，want %q", got, tc.want)
			}
		})
	}
}

func TestFilterMentions(t *testing.T) {
	entries := []pluginapi.FSEntry{
		{Name: "README.md"},
		{Name: "Plugins", IsDir: true},
		{Name: "plugins-go", IsDir: true},
		{Name: "main.go"},
		{Name: ""}, // 空名字必须被丢掉：画出来是一行空白，点了补出 "@" 什么都没有
	}

	got := filterMentions(entries, "pl")
	want := []string{"Plugins", "plugins-go"}
	if len(got) != 2 || got[0].Name != want[0] || got[1].Name != want[1] {
		t.Fatalf("前缀 pl（大小写不敏感 + 目录在前）= %v，want %v", names(got), want)
	}

	// 目录排在文件前面，和文件系统浏览器的直觉一致。
	got = filterMentions(entries, "")
	if len(got) != 4 {
		t.Fatalf("空前缀应当返回全部（去掉空名字那条），得到 %v", names(got))
	}
	if !got[0].IsDir || !got[1].IsDir {
		t.Errorf("目录没排在前面：%v", names(got))
	}
	if got[2].IsDir {
		t.Errorf("第三个应当是文件：%v", names(got))
	}

	if got := filterMentions(entries, "zzz"); got != nil {
		t.Errorf("没有匹配时应当返回 nil（调用方据此不画菜单），得到 %v", names(got))
	}
	if got := filterMentions(nil, ""); got != nil {
		t.Errorf("空输入应当返回 nil，得到 %v", names(got))
	}
}

// TestDropExactFileIsWhatKeepsEnterUsable 是这个文件里最要紧的一条。
//
// 用户把路径敲全（`@plugins/dsopen/open.go`）之后，候选里正好有它自己。
// 不去掉它：菜单一直挂着，Enter 一直被拿走"补全成同一个字符串"，
// **这条消息永远发不出去**，而且界面上没有任何东西解释为什么。
func TestDropExactFileIsWhatKeepsEnterUsable(t *testing.T) {
	exact := []pluginapi.FSEntry{{Name: "open.go"}}
	if got := dropExactFile(exact, "open.go"); got != nil {
		t.Errorf("已经敲全的文件应当被去掉，得到 %v", names(got))
	}

	// 反向三条：不能顺手把该补的也去掉。
	partial := []pluginapi.FSEntry{{Name: "open.go"}}
	if got := dropExactFile(partial, "open"); len(got) != 1 {
		t.Errorf("没敲全的文件不该被去掉，得到 %v", names(got))
	}
	// 目录敲全（`@plugins/`）是"进到这一层"，菜单该继续显示里面的东西。
	dir := []pluginapi.FSEntry{{Name: "plugins", IsDir: true}}
	if got := dropExactFile(dir, "plugins"); len(got) != 1 {
		t.Errorf("目录不该被去掉，得到 %v", names(got))
	}
	// 多个候选时不去掉：里面有同名的那也是巧合，用户可能还想选别的。
	multi := []pluginapi.FSEntry{{Name: "open.go"}, {Name: "open.go.bak"}}
	if got := dropExactFile(multi, "open.go"); len(got) != 2 {
		t.Errorf("多候选时不该去掉，得到 %v", names(got))
	}
}

// TestEnterNotTakenWithoutMenu 钉住"没有菜单就不许吃 Enter"。
//
// 这是整个功能最容易造成事故的一条：吃错一次，用户发不出消息，
// 而他看到的是"我按了 Enter 什么都没发生"。
func TestEnterNotTakenWithoutMenu(t *testing.T) {
	p := &Plugin{}
	if p.acceptMention() {
		t.Fatal("没有候选时 acceptMention 必须返回 false（按键要原样让回去）")
	}

	p.mentionRows = []pluginapi.FSEntry{{Name: "dsopen", IsDir: true}}
	p.mentionRef = atRef{Start: 0, Raw: "ds", Prefix: "ds"}
	p.field.SetText("@ds")
	if !p.acceptMention() {
		t.Fatal("有候选时应当补全")
	}
	if got := p.field.Text(); got != "@dsopen/" {
		t.Errorf("补全后草稿 = %q，want %q", got, "@dsopen/")
	}
}

// TestMentionMenuDrawsAndPicksByClick 是那一帧真布局的验收。
//
// 断言两件事：菜单真的算出来了（mentionRows 非空），以及点某一行真的改草稿。
// 只断言前者的话，"算了但没画"照样绿；只断言后者的话，直接调 completeAt 也行——
// 那样测的就不是这条装配路径了。
func TestMentionMenuDrawsAndPicksByClick(t *testing.T) {
	entries := map[string][]pluginapi.FSEntry{
		`D:\ws`: {
			{Name: "dsopen", IsDir: true},
			{Name: "dsquestions", IsDir: true},
			{Name: "main.go"},
		},
	}
	act := pluginapi.Actions{
		FSList: func(dir string) []pluginapi.FSEntry { return entries[dir] },
	}
	data := pluginapi.Snapshot{Cur: "s-001", CWD: `D:\ws`}

	// 一次：只看菜单有没有算出来（不点击）。
	p := &Plugin{}
	p.field.SetText("@ds")
	renderComposer(t, p, data, act, 1640, 1920, "")
	if len(p.mentionRows) == 0 {
		t.Fatal("草稿是 @ds、目录里有 dsopen/dsquestions，菜单却一个候选都没有")
	}
	if p.mentionRows[0].Name != "dsopen" {
		t.Errorf("第一个候选 = %q，want dsopen（目录在前 + 字母序）", p.mentionRows[0].Name)
	}

	// 二次：点第二个候选，草稿应当变成 "@dsquestions/"。
	p2 := &Plugin{}
	p2.field.SetText("@ds")
	renderComposer(t, p2, data, act, 1640, 1920, "mention-dsquestions")
	if got := p2.field.Text(); got != "@dsquestions/" {
		t.Errorf("点了 dsquestions 之后草稿 = %q，want %q", got, "@dsquestions/")
	}

	// 三次：没有 CWD 就整块不画（列出来也读不了），更不许吃 Enter。
	p3 := &Plugin{}
	p3.field.SetText("@ds")
	renderComposer(t, p3, pluginapi.Snapshot{Cur: "s-001"}, act, 1640, 1920, "")
	if len(p3.mentionRows) != 0 {
		t.Errorf("没有 CWD 时不该有候选，得到 %v", names(p3.mentionRows))
	}

	// 四次：壳没接 FSList（老宿主）同样不画。
	p4 := &Plugin{}
	p4.field.SetText("@ds")
	renderComposer(t, p4, data, pluginapi.Actions{}, 1640, 1920, "")
	if len(p4.mentionRows) != 0 {
		t.Errorf("FSList 为 nil 时不该有候选，得到 %v", names(p4.mentionRows))
	}
}

func names(es []pluginapi.FSEntry) string {
	var b []string
	for _, e := range es {
		b = append(b, e.Name)
	}
	return "[" + strings.Join(b, " ") + "]"
}
