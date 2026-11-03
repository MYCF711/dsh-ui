// config_test.go —— 插件配置的路径解析规则。
//
// 这个包之前 0% 覆盖，但它的错法特别难受：路径解析错了不会报错，只会
// "插件没装上"，用户看到的是界面少了几个部件——没有一次能直接指到配置上。
package uiplugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

func TestConfigDirDefaultsToCwd(t *testing.T) {
	got, err := ConfigDir("")
	if err != nil {
		t.Fatalf("空路径不该报错：%v", err)
	}
	wd, _ := os.Getwd()
	if got != wd {
		t.Fatalf("空路径应回当前目录 %q，得到 %q", wd, got)
	}
}

func TestConfigDirUsesParentOfFile(t *testing.T) {
	got, err := ConfigDir("a/b/plugins.json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "b" {
		t.Fatalf("应取配置文件的父目录，得到 %q", got)
	}
}

func TestResolveDirs(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "plugins.json")

	// 空 dirs → nil（不是空切片，调用方据此判断"没配"）
	if got, err := (Config{}).ResolveDirs(cfgPath); err != nil || got != nil {
		t.Fatalf("空 dirs 应返回 nil,nil，得到 %v,%v", got, err)
	}

	// 相对路径按**配置文件所在目录**解析，不是按 cwd——这条最容易搞错
	abs := filepath.Join(dir, "absone")
	cfg := Config{Dirs: []string{"rel/one", abs, "  padded  "}}
	got, err := cfg.ResolveDirs(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("应解析出 3 项，得到 %v", got)
	}
	if got[0] != filepath.Clean(filepath.Join(dir, "rel/one")) {
		t.Fatalf("相对路径应 join 到配置目录：%q", got[0])
	}
	if got[1] != filepath.Clean(abs) {
		t.Fatalf("绝对路径应原样保留（Clean）：%q", got[1])
	}
	if got[2] != filepath.Clean("  padded  ") && !strings.HasSuffix(got[2], "padded") {
		t.Fatalf("首尾空白应被去掉：%q", got[2])
	}
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Fatalf("解析结果必须都是绝对路径，得到 %q", p)
		}
	}

	// 空项必须报错：静默跳过会让「我明明配了」变成最难查的一类问题
	if _, err := (Config{Dirs: []string{"ok", "  "}}).ResolveDirs(cfgPath); err == nil {
		t.Fatal("dirs 里有空项应当报错")
	}
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()

	// 文件不存在 → 零值 + 不报错（二进制要能在没配置时单独跑）
	cfg, err := ReadConfig(filepath.Join(dir, "nope.json"))
	if err != nil {
		t.Fatalf("文件不存在不该报错：%v", err)
	}
	if len(cfg.Dirs) != 0 {
		t.Fatalf("应为零值配置，得到 %+v", cfg)
	}

	// 合法 JSON
	p := filepath.Join(dir, "ok.json")
	if err := os.WriteFile(p, []byte(`{"dirs":["a","b"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = ReadConfig(p)
	if err != nil || len(cfg.Dirs) != 2 {
		t.Fatalf("解析合法配置失败：%+v %v", cfg, err)
	}

	// 坏 JSON：必须报错，且错误里点名文件
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"dirs":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConfig(bad); err == nil {
		t.Fatal("坏 JSON 应报错")
	} else if !strings.Contains(err.Error(), "bad.json") {
		t.Fatalf("错误应点名配置文件，得到：%v", err)
	}
}

// ── Load 的 skip 语义 ───────────────────────────────────────────────────────
//
// 壳会把 ds-core/ds-shell/ds-sidebar/ds-conversation 这 4 个从 Load 的结果里
// **排除**（skip），然后自己 append 实例（因为它要读它们的 Fields/Width）。
// 所以 skip 在**任何**分支都必须生效——漏一个分支就是"同一插件装两次"，
// uikernel 报重复 id，而错误信息完全不会提示"配置文件没找到"。

type stubSource struct{ ps []pluginapi.Plugin }

func (s stubSource) Plugins() ([]pluginapi.Plugin, error) { return s.ps, nil }

type stubPlugin struct{ id string }

func (p stubPlugin) ID() string                       { return p.id }
func (p stubPlugin) Provides() []string               { return nil }
func (p stubPlugin) Requires() []string               { return nil }
func (p stubPlugin) Register(pluginapi.Context) error { return nil }

func TestLoadAppliesSkipInEveryBranch(t *testing.T) {
	src := stubSource{ps: []pluginapi.Plugin{
		stubPlugin{"ds-core"}, stubPlugin{"ds-shell"},
		stubPlugin{"ds-statusbar"}, stubPlugin{"ds-topbar"},
	}}
	skip := func(id string) bool { return id == "ds-core" || id == "ds-shell" }

	check := func(name string, got []pluginapi.Plugin, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, p := range got {
			if p.ID() == "ds-core" || p.ID() == "ds-shell" {
				t.Fatalf("%s: %s 应该被 skip 掉（壳会自己 append），实际还在", name, p.ID())
			}
		}
		if len(got) != 2 {
			t.Fatalf("%s: 应剩 2 个，实际 %d 个", name, len(got))
		}
	}

	// 分支一：没给配置路径
	got, err := Load("", src, skip)
	check("无配置路径", got, err)

	// 分支二：给了路径但文件不存在（**这条以前会漏 skip**）
	missing := filepath.Join(t.TempDir(), "nope.json")
	got, err = Load(missing, src, skip)
	check("配置不存在", got, err)

	// 分支三：配置存在但 plugins 为空数组
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte(`{"plugins":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = Load(empty, src, skip)
	check("plugins 为空", got, err)
}
