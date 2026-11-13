// user_text_rules.go —— 官方 `user-text-rules.ts` 的**逐条**移植。
//
// # 官方原文（`src/client/user-text-rules.ts`，36 行）
//
//	export const skillReferenceRule: UserTextRule<'skill'> = {
//	  id: 'skill',
//	  order: 20,
//	  match(text, from, context) {
//	    const tokens = /(?<!\S)\/([\w-]+)(?!\S)/gu
//	    tokens.lastIndex = from
//	    let names: readonly string[] | undefined
//	    for (let token = tokens.exec(text); token !== null; token = tokens.exec(text)) {
//	      if (names === undefined) {
//	        names = context.read('skillNames')
//	        if (names === undefined || names.length === 0) return undefined
//	      }
//	      const name = token[1] as string
//	      if (names.includes(name)) return {
//	        id: 'skill', start: token.index, end: token.index + token[0].length, data: { name },
//	      }
//	    }
//	    return undefined
//	  },
//	}
//
// # 为什么不用 Go 的 `regexp` 直接照抄
//
// 官方用的是**带 lookbehind 的 JS 正则**（`(?<!\S)` / `(?!\S)`），
// 而 Go 的 `regexp` 是 RE2，**不支持 lookbehind**。所以本文件把这条正则
// 拆成「主体 `/[\w-]+` 用 RE2 扫 + 两侧边界用显式谓词判」——
// 语义完全相同，但每一条边界都可以单独断言（见 `officialSkillRuleClauses`）。
//
// 还有两处 JS 与 Go 的**默认不同**，不处理就会在边界上悄悄漂移：
//
//	① `\S` 在 JS 是 `u` 模式下的**Unicode** 空白补集（含 U+00A0 / U+3000 /
//	   U+FEFF …），Go 的 `\S` 只认 ASCII。→ `jsSpace` 按 ECMAScript
//	   `WhiteSpace` + `LineTerminator` 枚举，不用 Go 的 `\s`。
//	② JS 的 `exec` 用 `lastIndex` 从 `from` **开始找**（不是「必须从 from 开始」），
//	   且 `from` 为负时等价 0。→ `matchSkillReference` 用 `text[from:]` 扫、
//	   再把偏移换算回全串，并在 `from < 0` 时钳到 0。
package dsskill

import (
	"regexp"
	"unicode/utf8"
)

// SkillFactSkillNames 是官方唯一读的 fact 名（`user-text-rules.ts:25` 的 `'skillNames'`）。
const SkillFactSkillNames = "skillNames"

// UserTextContext 是官方 `UserTextRule.match` 的第三个参数：只读事实面。
//
// 官方签名是 `context.read('skillNames')`，返回 `readonly string[] | undefined`。
// Go 没有 `undefined`，**nil 切片**就是它（`len(nil) == 0`，
// 但 `nil` 与 `[]string{}` 在官方的语义里是**同一件事**：都返回 undefined，
// 见 `officialSkillRuleClauses` 的 `read-empty` 两条）。
type UserTextContext interface {
	// Read 按 fact 名读一条事实；没提供时返回 nil。
	Read(fact string) any
}

// SkillNamesContext 是 `UserTextContext` 的最简实现（只提供 skillNames）。
type SkillNamesContext struct {
	// Names 是已确认的 skill 名（**不带** `/`）。
	Names []string
}

// Read 实现 UserTextContext。
func (c SkillNamesContext) Read(fact string) any {
	if fact != SkillFactSkillNames {
		return nil
	}
	if c.Names == nil {
		return nil
	}
	return c.Names
}

// SkillReference 是官方 `match` 返回的 `{id, start, end, data:{name}}`。
type SkillReference struct {
	// ID 恒为 `skill`（官方字面量 `id: 'skill'`）。
	ID string
	// Start 是 `/` 的**字节**偏移（官方 `token.index`，JS 里是 UTF-16 索引；
	// 官方 token 主体是 ASCII，所以两者在合法输入上一致——
	// `[\w-]` 只匹配 ASCII，`/` 也是 ASCII，见 `officialSkillRuleClauses` 的 `ascii-only`）。
	Start int
	// End 是名字末尾之后的字节偏移（官方 `token.index + token[0].length`）。
	// **包含**前导 `/`：`/foo` 的 End - Start == 4。
	End int
	// Name 是不带 `/` 的 skill 名（官方 `token[1]`）。
	Name string
}

// UserTextRule 是官方 `UserTextRule<'skill'>` 的 Go 形态。
type UserTextRule struct {
	// ID 是规则 id（官方 `id: 'skill'`）。
	ID string
	// Order 是规则优先级（官方 `order: 20`）。
	Order int
	// Match 从 `from` 起找第一个**已确认**的 `/name`；没有就返回 nil。
	Match func(text string, from int, ctx UserTextContext) *SkillReference
}

// skillReferenceRule 是官方 `skillReferenceRule` 本体。
var skillReferenceRule = UserTextRule{
	ID:    SkillUserTextRuleID,
	Order: SkillUserTextRuleOrder,
	Match: matchSkillReference,
}

// skillTokenPattern 是官方正则的**主体**：`/([\w-]+)`。
//
// 两侧的 `(?<!\S)` / `(?!\S)` 在 RE2 里写不出来，改由 `jsSpace` 判，
// 见 `boundedBySpace`。`\w` 在 Go 与 JS（`u` 模式）里都是 `[0-9A-Za-z_]`，
// 所以这一段可以直接照抄。
var skillTokenPattern = regexp.MustCompile(`/([\w-]+)`)

// matchSkillReference 实现官方 `match`。
//
// 步骤逐条对应官方源码：
//
//	① `from` 钳到 `[0, len(text)]`（JS 负 lastIndex 等价 0）。
//	② 从 `from` 起扫**全部** token（官方 `for (…exec…)`）。
//	③ 第一次进循环才 `context.read('skillNames')`（官方把 read 放在循环里，
//	   所以「一个 token 都没有」时**一次都不读**）。
//	④ `names` 为 nil 或空 → 立刻返回 nil（**不继续扫**）。
//	⑤ token 名在 `names` 里 → 返回；否则继续下一个 token。
//	⑥ 扫完没有 → nil。
func matchSkillReference(text string, from int, ctx UserTextContext) *SkillReference {
	if from < 0 {
		from = 0
	}
	if from > len(text) {
		return nil
	}
	var names []string
	read := false
	for _, token := range skillTokenPattern.FindAllStringSubmatchIndex(text[from:], -1) {
		start := from + token[0]
		end := from + token[1]
		// 官方 `(?<!\S)` / `(?!\S)`：两侧必须是「串首/串尾 或 空白」。
		if !boundedBySpace(text, start, end) {
			continue
		}
		if !read {
			names = readSkillNames(ctx)
			read = true
			if len(names) == 0 {
				return nil
			}
		}
		name := text[from+token[2] : from+token[3]]
		if containsName(names, name) {
			return &SkillReference{ID: SkillUserTextRuleID, Start: start, End: end, Name: name}
		}
	}
	return nil
}

// readSkillNames 取官方 `context.read('skillNames')` 的字符串切片。
//
// 只认 `[]string` 与 `nil`；别的类型一律当「没提供」（官方 `undefined`）——
// 静默把 `[]any` 或 `string` 当名字列表会造出**官方没有的引用**。
func readSkillNames(ctx UserTextContext) []string {
	if ctx == nil {
		return nil
	}
	switch names := ctx.Read(SkillFactSkillNames).(type) {
	case []string:
		return names
	case nil:
		return nil
	}
	return nil
}

// containsName 是官方 `names.includes(name)`（**精确**、区分大小写）。
func containsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// boundedBySpace 实现官方 `(?<!\S)/([\w-]+)(?!\S)` 的两侧边界。
//
// `(?<!\S)` = 前一个字符不是「非空白」= 串首 或 空白；
// `(?!\S)`  = 后一个字符不是「非空白」= 串尾 或 空白。
// start/end 是**字节**偏移，指向 `/` 与名字末尾之后。
func boundedBySpace(text string, start, end int) bool {
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if !jsSpace(before) {
			return false
		}
	}
	if end < len(text) {
		after, _ := utf8.DecodeRuneInString(text[end:])
		if !jsSpace(after) {
			return false
		}
	}
	return true
}

// jsSpace 报告 r 是否属于 ECMAScript 的 `WhiteSpace` ∪ `LineTerminator`。
//
// 枚举来源：ECMA-262 §11.2（`\s` 在 `u` 模式下等价于这个集合）。
// **不要**换成 `unicode.IsSpace`：那个集合更宽（例如 U+0085 NEL 在 Unicode
// 里是空白，在 JS 里不是），会让 `\S` 的边界和官方差一个字符。
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00a0', '\u1680', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	// U+2000–U+200A 是连续的一段（EN QUAD … HAIR SPACE）。
	return r >= '\u2000' && r <= '\u200a'
}

// ── 可断言的条款清单 ────────────────────────────────────────────────
//
// 「逐条实现并断言」不是一句形容词：下面这 15 条是官方 `user-text-rules.ts`
// 里**每一处**会改变结果的行为，测试 `TestUserTextRuleClauses` 按 ID 逐条跑。

// SkillRuleClause 是官方规则的一条可断言条款。
type SkillRuleClause struct {
	// ID 是条款短 id（测试子用例名）。
	ID string
	// Text 是官方源码里对应那一句（或那一句的直接推论）。
	Text string
	// Anchor 是官方 rc.2 行号。
	Anchor string
}

// officialSkillRuleClauses 是官方 `user-text-rules.ts` 的**全量行为条款**。
var officialSkillRuleClauses = []SkillRuleClause{
	{ID: "id", Text: "id: 'skill'", Anchor: "user-text-rules.ts:18"},
	{ID: "order", Text: "order: 20", Anchor: "user-text-rules.ts:19"},
	{ID: "token-body", Text: `/([\w-]+)/u —— 名字 = 至少一个 [A-Za-z0-9_] 或 '-'`, Anchor: "user-text-rules.ts:21"},
	{ID: "leading-slash", Text: "token 以 '/' 开头（start 指向 '/'）", Anchor: "user-text-rules.ts:21"},
	{ID: "start-boundary", Text: "(?<!\\S) —— '/' 前必须是串首或空白", Anchor: "user-text-rules.ts:21"},
	{ID: "end-boundary", Text: "(?!\\S) —— 名字后必须是串尾或空白", Anchor: "user-text-rules.ts:21"},
	{ID: "from-offset", Text: "tokens.lastIndex = from —— 从 from 起扫", Anchor: "user-text-rules.ts:22"},
	{ID: "lazy-read", Text: "context.read 在**第一个 token** 时才调（无 token 不读）", Anchor: "user-text-rules.ts:25-26"},
	{ID: "read-undefined", Text: "read 返回 undefined → 立刻 undefined（不继续扫）", Anchor: "user-text-rules.ts:27"},
	{ID: "read-empty", Text: "read 返回空数组 → 同样立刻 undefined", Anchor: "user-text-rules.ts:27"},
	{ID: "confirmed-only", Text: "只有 names.includes(name) 才命中（未确认的名字不命中）", Anchor: "user-text-rules.ts:30"},
	{ID: "first-match", Text: "返回**第一个**已确认的 token（后面的不再看）", Anchor: "user-text-rules.ts:30-32"},
	{ID: "span", Text: "start = token.index，end = token.index + token[0].length（含前导 '/'）", Anchor: "user-text-rules.ts:31"},
	{ID: "data-name", Text: "data.name 是**不带** '/' 的名字（token[1]）", Anchor: "user-text-rules.ts:31"},
	{ID: "no-match", Text: "循环结束仍没命中 → undefined（不是空引用）", Anchor: "user-text-rules.ts:34"},
}
