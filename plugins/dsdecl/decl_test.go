package dsdecl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/uidecl"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
)

// 这个文件测的是「扫哪个目录、扫到的怎么挂上去」。
// JSON 本身的校验在 internal/uidecl 里测（那边不用碰插件契约）。

// fakeCtx 是最小的 pluginapi.Context：只实现注入和日志。
type fakeCtx struct {
	values map[string]any
	logs   []string
}

func (c *fakeCtx) Name() string            { return "ds-decl" }
func (c *fakeCtx) Provide(string, any)     {}
func (c *fakeCtx) Inject(k string) any     { return c.values[k] }
func (c *fakeCtx) Effect(func())           {}
func (c *fakeCtx) Logf(f string, a ...any) { c.logs = append(c.logs, fmt.Sprintf(f, a...)) }

// newCtx 返回一个「根插件已经就位」的上下文（主题 + 图元都在）。
func newCtx() *fakeCtx {
	theme := &pluginapi.Theme{Name: "test"}
	return &fakeCtx{values: map[string]any{
		pluginapi.KeyTheme:      theme,
		pluginapi.KeyPrimitives: &dscore.Primitives{Theme: theme},
	}}
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("写 %s 失败: %v", name, err)
	}
}

func TestMeta(t *testing.T) {
	p := &Plugin{}
	if p.ID() != "ds-decl" {
		t.Fatalf("ID = %q", p.ID())
	}
	if got := p.Provides(); len(got) != 0 {
		t.Fatalf("Provides = %v，want 空", got)
	}
	// 只要求这两个：多要一个键就等于把声明通道绑死在别的插件的装配顺序上。
	got := p.Requires()
	if len(got) != 2 || got[0] != pluginapi.KeyTheme || got[1] != pluginapi.KeyPrimitives {
		t.Fatalf("Requires = %v", got)
	}
}

func TestRegisterLoadsDeclarations(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hello.json", `{"id":"hello.badge","slot":"sidebar.footer","order":90,"kind":"dot","text":"外部 JSON 插件 · {sessions} 个会话"}`)
	writeFile(t, dir, "more.json", `{"id":"more.kv","slot":"rightbar","kind":"keyvalue","label":"会话","text":"{sessions}"}`)
	t.Setenv(uidecl.EnvDir, dir)

	p := &Plugin{}
	ctx := newCtx()
	if err := p.Register(ctx); err != nil {
		t.Fatalf("Register 出错: %v", err)
	}
	if p.Dir() != dir {
		t.Fatalf("Dir = %q，want %q", p.Dir(), dir)
	}
	if len(p.Decls()) != 2 {
		t.Fatalf("扫到 %d 份，want 2", len(p.Decls()))
	}

	panels := p.Panels()
	if len(panels) != 2 {
		t.Fatalf("Panels = %d 块，want 2（每份声明一块）", len(panels))
	}
	if panels[0].Slot != pluginapi.SlotSidebarFooter || panels[0].Order != 90 || panels[0].Draw == nil {
		t.Fatalf("第一块画错了: %+v", panels[0])
	}
	if panels[0].ID != "hello.badge" {
		t.Fatalf("画的 ID 应该是声明 id: %q", panels[0].ID)
	}
	// 装配日志是「我到底挂上了没有」的唯一权威答案，必须逐条打出来。
	var joined strings.Builder
	for _, l := range ctx.logs {
		joined.WriteString(l + "\n")
	}
	if !strings.Contains(joined.String(), "hello.badge") || !strings.Contains(joined.String(), "sidebar.footer") {
		t.Fatalf("日志里要能看出挂了什么、挂在哪:\n%s", joined.String())
	}
}

func TestRegisterEmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(uidecl.EnvDir, dir)

	p := &Plugin{}
	if err := p.Register(newCtx()); err != nil {
		t.Fatalf("空目录不该报错: %v", err)
	}
	if len(p.Panels()) != 0 {
		t.Fatalf("空目录应该 0 块画，实际 %d", len(p.Panels()))
	}
}

func TestRegisterMissingDirIsQuiet(t *testing.T) {
	// 目录不存在是正常状态：大多数机器上根本不会建这个目录。
	t.Setenv(uidecl.EnvDir, filepath.Join(t.TempDir(), "not-here"))

	p := &Plugin{}
	if err := p.Register(newCtx()); err != nil {
		t.Fatalf("目录不存在不该报错: %v", err)
	}
	if len(p.Panels()) != 0 {
		t.Fatalf("应该 0 块画，实际 %d", len(p.Panels()))
	}
}

func TestRegisterBadJSONFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "broken.json", `{"id":"a","slot":"sidebar.footer","text":`)
	t.Setenv(uidecl.EnvDir, dir)

	err := (&Plugin{}).Register(newCtx())
	if err == nil {
		t.Fatal("坏 JSON 必须让装配失败——静默跳过等于界面上少一块且没人知道")
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("错误里要有文件名: %v", err)
	}
}

func TestRegisterUnknownSlotFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "typo.json", `{"id":"a","slot":"sidebar.foter","text":"甲"}`)
	t.Setenv(uidecl.EnvDir, dir)

	err := (&Plugin{}).Register(newCtx())
	if err == nil {
		t.Fatal("未知槽位必须报错，不能静默忽略")
	}
	if !strings.Contains(err.Error(), "sidebar.footer") || !strings.Contains(err.Error(), "typo.json") {
		t.Fatalf("错误里要回显槽位、列出可用槽位并指出文件: %v", err)
	}
}

func TestRegisterNeedsTheme(t *testing.T) {
	// 装配顺序错了（ds-core 没排前面）时要炸，不能拿 nil 主题去画。
	t.Setenv(uidecl.EnvDir, t.TempDir())
	err := (&Plugin{}).Register(&fakeCtx{values: map[string]any{}})
	if err == nil || !strings.Contains(err.Error(), pluginapi.KeyTheme) {
		t.Fatalf("缺主题要报错并说出缺哪个键，实际: %v", err)
	}
}

func TestPanelsStableOrder(t *testing.T) {
	// 同一份目录每帧画出来必须一样，否则截图对比会飘。
	dir := t.TempDir()
	writeFile(t, dir, "b.json", `{"id":"b.one","slot":"shell.bottom","order":50,"text":"乙"}`)
	writeFile(t, dir, "a.json", `{"id":"a.one","slot":"shell.bottom","order":10,"text":"甲"}`)
	t.Setenv(uidecl.EnvDir, dir)

	p := &Plugin{}
	if err := p.Register(newCtx()); err != nil {
		t.Fatalf("Register 出错: %v", err)
	}
	first := p.Panels()
	second := p.Panels()
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("Panels 数量不对: %d, %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Order != second[i].Order {
			t.Fatalf("两次 Panels 不一致: %+v vs %+v", first[i], second[i])
		}
	}
}
