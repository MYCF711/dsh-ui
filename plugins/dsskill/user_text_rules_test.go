// user_text_rules_test.go —— 官方 `user-text-rules.ts` 的**逐条**断言。
//
// 一条规则有 15 处会改变结果的行为（`officialSkillRuleClauses`）。
// 本文件按**条款 id** 建子用例，最后一条断言核对「条款表里每一条都有用例」——
// 少一条就不会静默通过。
package dsskill

import (
	"testing"
)

// spyContext 是记读次数的 `UserTextContext`（用来钉「懒读」）。
type spyContext struct {
	names []string
	// provided 为 false 时 `Read` 返回 nil（官方 `undefined`）。
	provided bool
	reads    int
}

func (c *spyContext) Read(fact string) any {
	if fact != SkillFactSkillNames {
		// 官方只读 `skillNames` 一个键；读别的必须返回 nil，且**不计**入懒读核对。
		return nil
	}
	c.reads++
	if !c.provided {
		return nil
	}
	return c.names
}

// ruleCase 是一条条款的用例。
type ruleCase struct {
	// matches 是期望命中的引用；nil = 期望不命中。
	matches *SkillReference
}

// runClause 跑一条条款。
func runClause(t *testing.T, id string, text string, from int, ctx *spyContext, want *SkillReference) {
	t.Helper()
	got := skillReferenceRule.Match(text, from, ctx)
	switch {
	case want == nil && got != nil:
		t.Fatalf("[%s] 期望不命中，实际 %+v（text=%q from=%d）", id, *got, text, from)
	case want != nil && got == nil:
		t.Fatalf("[%s] 期望 %+v，实际不命中（text=%q from=%d）", id, *want, text, from)
	case want != nil && got != nil:
		if *got != *want {
			t.Fatalf("[%s] 实际 %+v，期望 %+v（text=%q from=%d）", id, *got, *want, text, from)
		}
	}
}

// TestUserTextRuleIdentityAndOrder 钉 `id` / `order` 两条（`user-text-rules.ts:18-19`）。
func TestUserTextRuleIdentityAndOrder(t *testing.T) {
	if skillReferenceRule.ID != "skill" {
		t.Fatalf("id = %q，want %q", skillReferenceRule.ID, "skill")
	}
	if skillReferenceRule.Order != 20 {
		t.Fatalf("order = %d，want 20", skillReferenceRule.Order)
	}
	// 常量与规则必须同源（两处写死会漂移）。
	if SkillUserTextRuleID != skillReferenceRule.ID {
		t.Fatalf("SkillUserTextRuleID = %q ≠ 规则的 %q", SkillUserTextRuleID, skillReferenceRule.ID)
	}
	if SkillUserTextRuleOrder != skillReferenceRule.Order {
		t.Fatalf("SkillUserTextRuleOrder = %d ≠ 规则的 %d", SkillUserTextRuleOrder, skillReferenceRule.Order)
	}
}

// TestUserTextRuleClauses 按条款 id 逐条断言。
func TestUserTextRuleClauses(t *testing.T) {
	names := func(n ...string) *spyContext {
		return &spyContext{names: n, provided: true}
	}

	cases := []struct {
		id   string
		text string
		from int
		ctx  *spyContext
		want *SkillReference
	}{
		// ── token-body：`([\w-]+)` ──────────────────────────────
		{
			id: "token-body", text: "/foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 0, End: 4, Name: "foo"},
		},
		{
			// `\w` 含数字与下划线；`-` 在字符组里是字面量。
			id: "token-body", text: "/a_b-1", from: 0, ctx: names("a_b-1"),
			want: &SkillReference{ID: "skill", Start: 0, End: 6, Name: "a_b-1"},
		},
		{
			// 至少要一个字符：`/` 后面什么都没有 → 不命中。
			id: "token-body", text: "/", from: 0, ctx: names(""), want: nil,
		},
		{
			// 名字里不能有 `.`：`/foo.bar` 的 token 是 `/foo`，
			// 但它后面紧跟 `.`（非空白）→ `(?!\S)` 失败 → 不命中。
			id: "token-body", text: "/foo.bar", from: 0, ctx: names("foo", "foo.bar"), want: nil,
		},
		{
			// 非 ASCII 字母不属于 `\w`（JS 的 `u` 模式也一样）。
			id: "token-body", text: "/技能", from: 0, ctx: names("技能"), want: nil,
		},

		// ── leading-slash：start 指向 `/` ───────────────────────
		{
			id: "leading-slash", text: "x /foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 2, End: 6, Name: "foo"},
		},

		// ── start-boundary：`(?<!\S)` ───────────────────────────
		{
			// 前一个字符是非空白（`a`）→ 不是 token 起点。
			id: "start-boundary", text: "a/foo", from: 0, ctx: names("foo"), want: nil,
		},
		{
			// 前一个是 Tab（空白）→ 是 token 起点。
			id: "start-boundary", text: "\t/foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 1, End: 5, Name: "foo"},
		},
		{
			// 前一个是 NBSP（U+00A0）：JS `\s` **包含**它 → 是起点。
			// 这一条挡的是"用 Go 的 `\s` 或者 `unicode.IsSpace` 抄"——
			// 前者不认 NBSP，后者多认 NEL。
			id: "start-boundary", text: "\u00a0/foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 2, End: 6, Name: "foo"},
		},
		{
			// 前一个是 NEL（U+0085）：Unicode 里是空白，**JS `\s` 里不是** → 不是起点。
			id: "start-boundary", text: "\u0085/foo", from: 0, ctx: names("foo"), want: nil,
		},

		// ── end-boundary：`(?!\S)` ────────────────────────────
		{
			id: "end-boundary", text: "/foo!", from: 0, ctx: names("foo"), want: nil,
		},
		{
			id: "end-boundary", text: "/foo bar", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 0, End: 4, Name: "foo"},
		},
		{
			// 后一个是 NBSP：JS 里算空白 → 命中。
			id: "end-boundary", text: "/foo\u00a0", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 0, End: 4, Name: "foo"},
		},

		// ── from-offset：`tokens.lastIndex = from` ─────────────
		{
			id: "from-offset", text: "/alpha /beta", from: 7, ctx: names("alpha", "beta"),
			want: &SkillReference{ID: "skill", Start: 7, End: 12, Name: "beta"},
		},
		{
			// from 落在 token 中间：那个 token 起点 < from，**不算**。
			id: "from-offset", text: "/alpha", from: 3, ctx: names("alpha", "lpha"), want: nil,
		},
		{
			// from 超过长度 → 不命中（不是 panic）。
			id: "from-offset", text: "/alpha", from: 99, ctx: names("alpha"), want: nil,
		},
		{
			// 负数 from 等价 0（JS 的 lastIndex 负值当 0）。
			id: "from-offset", text: "/alpha", from: -5, ctx: names("alpha"),
			want: &SkillReference{ID: "skill", Start: 0, End: 6, Name: "alpha"},
		},

		// ── lazy-read：没有 token 就一次都不读 ─────────────────
		{
			id: "lazy-read", text: "no tokens here", from: 0, ctx: names("foo"), want: nil,
		},

		// ── read-undefined / read-empty ───────────────────────
		{
			id: "read-undefined", text: "/foo", from: 0, ctx: &spyContext{provided: false}, want: nil,
		},
		{
			id: "read-empty", text: "/foo", from: 0, ctx: &spyContext{names: []string{}, provided: true}, want: nil,
		},

		// ── confirmed-only ────────────────────────────────────
		{
			id: "confirmed-only", text: "/bar /foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 5, End: 9, Name: "foo"},
		},

		// ── first-match ───────────────────────────────────────
		{
			id: "first-match", text: "/beta /alpha", from: 0, ctx: names("alpha", "beta"),
			want: &SkillReference{ID: "skill", Start: 0, End: 5, Name: "beta"},
		},

		// ── span ──────────────────────────────────────────────
		{
			// End - Start 必须**包含**前导 `/`：`/foo` 是 4 个字符。
			id: "span", text: "/foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 0, End: 4, Name: "foo"},
		},

		// ── data-name ─────────────────────────────────────────
		{
			// Name 不带 `/`（`text[Start:End]` 才是带 `/` 的那一段）。
			id: "data-name", text: "see /foo", from: 0, ctx: names("foo"),
			want: &SkillReference{ID: "skill", Start: 4, End: 8, Name: "foo"},
		},

		// ── no-match ──────────────────────────────────────────
		{
			id: "no-match", text: "/bar", from: 0, ctx: names("foo"), want: nil,
		},
	}

	// `id` / `order` 两条由 TestUserTextRuleIdentityAndOrder 断言（它们是
	// 规则的**元数据**，没有 text/from 可喂），这里登记为已覆盖。
	covered := map[string]bool{"id": true, "order": true}
	for _, c := range cases {
		covered[c.id] = true
		c := c
		t.Run(c.id, func(t *testing.T) {
			runClause(t, c.id, c.text, c.from, c.ctx, c.want)
		})
	}

	// 条款表里每一条都必须有用例——加条款不加用例，这里就红。
	for _, clause := range officialSkillRuleClauses {
		if !covered[clause.ID] {
			t.Errorf("条款 %q（%s，%s）没有对应用例", clause.ID, clause.Text, clause.Anchor)
		}
	}
}

// TestUserTextRuleSpanIncludesSlash 钉住 `End - Start` 的语义。
//
// 官方 `end = token.index + token[0].length`，而 `token[0]` 是**含 `/`** 的
// 整段（`/foo`）。写成 `start + len(name)` 会少一个字符，高亮区就会
// 把 `/` 漏在外面——那是唯一一处"看起来对、差一个字节"的地方。
func TestUserTextRuleSpanIncludesSlash(t *testing.T) {
	ctx := &spyContext{names: []string{"foo"}, provided: true}
	ref := skillReferenceRule.Match("see /foo now", 0, ctx)
	if ref == nil {
		t.Fatal("没有命中")
	}
	if ref.End-ref.Start != len("/"+ref.Name) {
		t.Fatalf("span = %d，want %d（含前导 '/'）", ref.End-ref.Start, len("/"+ref.Name))
	}
	if got := "see /foo now"[ref.Start:ref.End]; got != "/foo" {
		t.Fatalf("命中区 = %q，want %q", got, "/foo")
	}
}

// TestUserTextRuleReadCount 钉住「懒读」与「只读一次」。
//
// 官方把 `context.read('skillNames')` 放在循环里、且用 `names === undefined`
// 判"读过没有"。三个可观察后果：
//
//	没有 token        → 0 次
//	有 token 但没确认  → 1 次
//	有 token 且确认    → 1 次（不是每个 token 一次）
func TestUserTextRuleReadCount(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"没有 token", "nothing", 0},
		{"有 token 但没确认", "/bar /baz", 1},
		{"有 token 且确认", "/foo /bar", 1},
	}
	for _, c := range cases {
		ctx := &spyContext{names: []string{"foo"}, provided: true}
		_ = skillReferenceRule.Match(c.text, 0, ctx)
		if ctx.reads != c.want {
			t.Errorf("%s：read 被调 %d 次，want %d", c.name, ctx.reads, c.want)
		}
	}
}

// TestUserTextRuleReadUndefinedStopsScanning 钉住「read 返回 undefined 立刻退出」。
//
// 官方 `if (names === undefined || names.length === 0) return undefined` 在
// 循环**里面**：它不会继续扫后面的 token。这条与"扫完再判空"的可观察差别是
// read 次数——后者对每个 token 都会再读一次。
func TestUserTextRuleReadUndefinedStopsScanning(t *testing.T) {
	ctx := &spyContext{provided: false}
	if ref := skillReferenceRule.Match("/a /b /c", 0, ctx); ref != nil {
		t.Fatalf("read 返回 undefined 时必须不命中，实际 %+v", *ref)
	}
	if ctx.reads != 1 {
		t.Fatalf("read 被调 %d 次，want 1（第一次就返回）", ctx.reads)
	}
}

// TestUserTextRuleNonStringNamesRejected 钉住「只认 []string」。
//
// `UserTextContext.Read` 返回 `any`；把 `[]any{"foo"}` 或 `"foo"` 当名字列表
// 会造出**官方没有的引用**（官方 `read` 的类型是 `readonly string[] | undefined`）。
func TestUserTextRuleNonStringNamesRejected(t *testing.T) {
	for _, value := range []any{[]any{"foo"}, "foo", 42, map[string]bool{"foo": true}} {
		ctx := anyContext{value: value}
		if ref := skillReferenceRule.Match("/foo", 0, ctx); ref != nil {
			t.Errorf("Read 返回 %T 时不该命中，实际 %+v", value, *ref)
		}
	}
}

// anyContext 是返回任意值的 `UserTextContext`。
type anyContext struct{ value any }

func (c anyContext) Read(string) any { return c.value }

// TestUserTextRuleNilContext 钉住「nil context 不 panic」。
func TestUserTextRuleNilContext(t *testing.T) {
	if ref := skillReferenceRule.Match("/foo", 0, nil); ref != nil {
		t.Fatalf("nil context 必须不命中，实际 %+v", *ref)
	}
}

// TestSkillFactName 钉住官方唯一读的 fact 名。
func TestSkillFactName(t *testing.T) {
	if SkillFactSkillNames != "skillNames" {
		t.Fatalf("fact 名 = %q，want %q（官方 `context.read('skillNames')`）",
			SkillFactSkillNames, "skillNames")
	}
}

// TestSkillNamesContext 钉住最简实现的两个分支。
func TestSkillNamesContext(t *testing.T) {
	if got := (SkillNamesContext{Names: []string{"a"}}).Read("skillNames"); got == nil {
		t.Fatal("有名字时不该返回 nil")
	}
	if got := (SkillNamesContext{}).Read("skillNames"); got != nil {
		t.Fatalf("Names 为 nil 时返回 %v，want nil", got)
	}
	if got := (SkillNamesContext{Names: []string{"a"}}).Read("other"); got != nil {
		t.Fatalf("读别的 fact 应返回 nil，实际 %v", got)
	}
}
