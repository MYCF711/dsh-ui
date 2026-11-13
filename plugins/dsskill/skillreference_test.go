//go:build darwin || windows || linux

// skillreference_test.go —— 官方 `SkillReference.tsx`（28 行）的**行为**断言。
//
// 官方三条语义（注释逐字）：
//
//	*"Render a skill chip; previews omit activation even when the entry has an
//	 injected action."*
//
//	label={`/${node.data.name}`}   —— 前缀 `/` 是**拼**上去的
//	title={raw}                    —— title 是命中区原文（含 `/`）
//	interactive ? {onActivate} : {} —— 预览态**不挂**激活
package dsskill

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// TestSkillReferenceLabel 钉住 `label={`/${name}`}`。
func TestSkillReferenceLabel(t *testing.T) {
	cases := []struct{ name, want string }{
		{"demo", "/demo"},
		{"a-b_c", "/a-b_c"},
		{"", "/"}, // 空名字也只有一个 `/`（官方就是模板字符串）
	}
	for _, c := range cases {
		if got := SkillReferenceLabel(c.name); got != c.want {
			t.Errorf("SkillReferenceLabel(%q) = %q，want %q", c.name, got, c.want)
		}
	}
}

// TestSkillReferenceActivate 钉住「什么时候挂 onActivate」。
//
// 官方 `...(interactive ? { onActivate } : {})`：**两个条件都要**——
// `interactive` 为真**且** `openSkill` 注入。缺一个就根本没有那个属性。
func TestSkillReferenceActivate(t *testing.T) {
	var got []string
	injected := func(name string) { got = append(got, name) }

	cases := []struct {
		name        string
		props       SkillReferenceProps
		wantHandled bool
		wantCalls   int
	}{
		{
			name:        "interactive + 已注入 → 激活",
			props:       SkillReferenceProps{Name: "demo", Raw: "/demo", Interactive: true, OpenSkill: injected},
			wantHandled: true, wantCalls: 1,
		},
		{
			name:        "预览态（interactive=false）即使已注入也不激活",
			props:       SkillReferenceProps{Name: "demo", Raw: "/demo", Interactive: false, OpenSkill: injected},
			wantHandled: false, wantCalls: 0,
		},
		{
			name:        "interactive 但没注入 → 不激活",
			props:       SkillReferenceProps{Name: "demo", Raw: "/demo", Interactive: true, OpenSkill: nil},
			wantHandled: false, wantCalls: 0,
		},
		{
			name:        "都没 → 不激活",
			props:       SkillReferenceProps{Name: "demo", Raw: "/demo"},
			wantHandled: false, wantCalls: 0,
		},
	}
	for _, c := range cases {
		got = nil
		handled := c.props.Activate()
		if handled != c.wantHandled {
			t.Errorf("%s：Activate() = %v，want %v", c.name, handled, c.wantHandled)
		}
		if len(got) != c.wantCalls {
			t.Fatalf("%s：openSkill 被调 %d 次，want %d", c.name, len(got), c.wantCalls)
		}
		if c.wantCalls == 1 && got[0] != "demo" {
			t.Errorf("%s：openSkill 收到 %q，want demo（**不带** `/`）", c.name, got[0])
		}
	}
}

// TestSkillReferenceAppearance 钉住 `appearance="skill"` 的取值。
func TestSkillReferenceAppearance(t *testing.T) {
	if AppearanceSkill != "skill" {
		t.Fatalf("appearance = %q，want %q", AppearanceSkill, "skill")
	}
}

// TestDrawSkillReference 画两态，钉住「预览态没有命中区」。
//
// 官方预览态连 `onActivate` 都不传：点它**不会**有任何事发生，
// 而且不该出现一个"可点"的命中区（那会吞掉选择/复制的指针事件）。
func TestDrawSkillReference(t *testing.T) {
	theme := dscore.Dark()

	t.Run("预览态无命中区", func(t *testing.T) {
		harness := newReferenceHarness(t, theme,
			SkillReferenceProps{Name: "demo", Raw: "/demo", Interactive: false, OpenSkill: func(string) {}})
		session := harness.open(t)
		if _, ok := findHit(session, "/demo"); ok {
			t.Fatalf("预览态不该有命中区；hits=%+v", session.Snapshot().Hits)
		}
	})

	t.Run("可交互态有命中区", func(t *testing.T) {
		var got []string
		harness := newReferenceHarness(t, theme,
			SkillReferenceProps{Name: "demo", Raw: "/demo", Interactive: true,
				OpenSkill: func(name string) { got = append(got, name) }})
		session := harness.open(t)
		hit, ok := findHit(session, "/demo")
		if !ok {
			t.Fatalf("可交互态应有命中区；hits=%+v", session.Snapshot().Hits)
		}
		clickAt(t, session, hit)
		if len(got) != 1 || got[0] != "demo" {
			t.Fatalf("点击后 openSkill 被调 %v，want [demo]", got)
		}
	})
}

// ── 画布装配 ──────────────────────────────────────────────────────

// referenceHarness 是一套「真画布」的 chip 装配。
type referenceHarness struct {
	props SkillReferenceProps
	tree  *gfx.Tree
	feat  gfx.Scope
	theme *pluginapi.Theme
	page  *ui.Page
}

// newReferenceHarness 造一个 chip harness。
func newReferenceHarness(t *testing.T, theme *pluginapi.Theme, props SkillReferenceProps) *referenceHarness {
	t.Helper()
	tree := &gfx.Tree{}
	pageTheme := dscore.VivaTheme(theme)
	return &referenceHarness{
		props: props, tree: tree, feat: tree.Feature("skill-reference"),
		theme: theme, page: ui.NewPage(ui.PageConfig{Theme: &pageTheme}),
	}
}

// open 起一个 headless 会话并泵一帧。
func (h *referenceHarness) open(t *testing.T) *headless.Session {
	t.Helper()
	var invalidator app.Invalidator
	config := app.Config{
		Title: "skill-reference", Width: 400, Height: 120, Tree: h.tree,
		WindowMode: app.WindowFrameless, Invalidator: &invalidator,
		Paint: func(canvas *gfx.Canvas, current ui.Size) {
			bounds := gfx.Rect{Width: current.Width, Height: current.Height}
			canvas.Fill(bounds, h.theme.BgBase)
			h.page.Draw(canvas, bounds, func(f ui.C) {
				DrawSkillReference(f, h.theme, h.tree.ID(h.feat, "chip"), h.props)
			})
		},
	}
	session, err := headless.Open(config, nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatalf("headless 打不开：%v", err)
	}
	t.Cleanup(session.Close)
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	return session
}
