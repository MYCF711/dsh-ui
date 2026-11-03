// kernel_test.go —— 插件内核的装配规则。
//
// 这个包之前覆盖率 0%，但它是"第三方插件通道"的地基：装错顺序、缺依赖、
// 能力冲突这些错必须在**装配期**报出来，而不是等到绘制时某个插件拿到 nil。
package uikernel

import (
	"errors"
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/pluginapi"
)

// fake 是最小插件：id / 提供 / 依赖 / 注册时做什么，都能配。
type fake struct {
	id       string
	provides []string
	requires []string
	regErr   error
	onReg    func(ctx pluginapi.Context)
	panels   []pluginapi.Painter
	hasPanel bool
}

func (f *fake) ID() string         { return f.id }
func (f *fake) Provides() []string { return f.provides }
func (f *fake) Requires() []string { return f.requires }
func (f *fake) Panels() []pluginapi.Painter {
	if !f.hasPanel {
		return nil
	}
	return f.panels
}
func (f *fake) Register(ctx pluginapi.Context) error {
	if f.regErr != nil {
		return f.regErr
	}
	for _, k := range f.provides {
		ctx.Provide(k, f.id) // 提供的能力值就用 id，方便断言
	}
	if f.onReg != nil {
		f.onReg(ctx)
	}
	return nil
}

func TestLoadTopologicalOrder(t *testing.T) {
	k := New("t")
	var installed []string
	mk := func(id string, req, prov []string) *fake {
		return &fake{id: id, requires: req, provides: prov,
			onReg: func(pluginapi.Context) { installed = append(installed, id) }}
	}
	// 故意打乱顺序：consumer 依赖 theme，theme 依赖 primitives
	err := k.Load(
		mk("consumer", []string{"ui.theme"}, nil),
		mk("theme", []string{"ui.primitives"}, []string{"ui.theme"}),
		mk("primitives", nil, []string{"ui.primitives"}),
	)
	if err != nil {
		t.Fatalf("应当装配成功：%v", err)
	}
	want := []string{"primitives", "theme", "consumer"}
	if strings.Join(installed, ",") != strings.Join(want, ",") {
		t.Fatalf("装配顺序 = %v，期望 %v（依赖必须先装）", installed, want)
	}
	if got := k.Order(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Order() = %v", got)
	}
}

func TestLoadRejectsBadPlugins(t *testing.T) {
	cases := map[string]pluginapi.Plugin{
		"nil":  nil,
		"空 ID": &fake{id: ""},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if err := New("t").Load(p); err == nil {
				t.Fatal("应当报错")
			}
		})
	}
	// 重复 ID
	k := New("t")
	if err := k.Load(&fake{id: "dup"}); err != nil {
		t.Fatal(err)
	}
	if err := k.Load(&fake{id: "dup"}); err == nil {
		t.Fatal("重复 ID 应当报错")
	}
}

func TestLoadReportsMissingDependency(t *testing.T) {
	err := New("t").Load(&fake{id: "needy", requires: []string{"ui.theme"}})
	if err == nil {
		t.Fatal("缺依赖应当报错")
	}
	// 错误里必须点名是哪个插件卡住了，否则排查像大海捞针
	if !strings.Contains(err.Error(), "needy") {
		t.Fatalf("错误应点名插件，得到：%v", err)
	}
}

func TestLoadReportsCycle(t *testing.T) {
	err := New("t").Load(
		&fake{id: "a", requires: []string{"b"}},
		&fake{id: "b", requires: []string{"a"}},
	)
	if err == nil {
		t.Fatal("环应当报错")
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("错误应点名环上两个插件，得到：%v", err)
	}
}

func TestLoadRejectsDuplicateCapability(t *testing.T) {
	err := New("t").Load(
		&fake{id: "p1", provides: []string{"ui.theme"}},
		&fake{id: "p2", provides: []string{"ui.theme"}},
	)
	if err == nil {
		t.Fatal("同一能力被两个插件提供应当报错")
	}
	if !strings.Contains(err.Error(), "ui.theme") {
		t.Fatalf("错误应点名冲突的能力，得到：%v", err)
	}
}

func TestLoadPropagatesRegisterError(t *testing.T) {
	sentinel := errors.New("注册期炸了")
	err := New("t").Load(&fake{id: "bad", regErr: sentinel})
	if err == nil || !strings.Contains(err.Error(), "注册期炸了") {
		t.Fatalf("Register 的错误要原样冒出，得到：%v", err)
	}
}

func TestLoadRejectsInvalidPanel(t *testing.T) {
	// 挂了槽位但没有任何绘制函数：这种画在绘制期什么都不做，属于配置错，装配期就该拦。
	p := &fake{id: "painter", hasPanel: true, panels: []pluginapi.Painter{{Slot: "main"}}}
	if err := New("t").Load(p); err == nil {
		t.Fatal("没有 Draw 的槽位应当报错")
	}

}

func TestScopeProvideInjectAndEffect(t *testing.T) {
	k := New("t")
	var cleaned bool
	p := &fake{id: "p", provides: []string{"k1"}, onReg: func(ctx pluginapi.Context) {
		ctx.Effect(func() { cleaned = true })
	}}
	if err := k.Load(p); err != nil {
		t.Fatal(err)
	}
	if got := k.Get("k1"); got != "p" {
		t.Fatalf("Get(k1) = %v，期望 p", got)
	}
	if k.MustGet("k1") != "p" {
		t.Fatal("MustGet 应返回同一个值")
	}
	_ = cleaned
}
