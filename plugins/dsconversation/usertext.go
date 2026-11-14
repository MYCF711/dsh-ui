package dsconversation

// usertext.go —— 用户消息里的**内联引用 chip** 与**凭据逐值显示/隐藏**。
//
// 官方对应物：
//   - `dsh-client-ui-reference` 的 `FileReference.tsx` / `FolderReference`
//   - `dsh-client-ui-conversation/src/client/user-text/UserText.tsx`（inline nodes）
//   - `dsh-client-ui-secret-mask/src/client/SecretText.tsx`（`interactive` 时给
//     每个 secret 一个眼睛按钮，点一下在本位显示原文，再点隐藏）
//
// # 为什么要在 Go 侧自己排版
//
// viva 的 `ui.Wrap` 只能画**一整段同一样式**的文字；官方这两类东西都是
// **行内节点**（chip 与 secret 都在句子中间）。所以这里做一个最小行内布局：
// 先量宽、按词/字符换行，再逐行用 `ui.Row` 画文字段、chip 和眼睛按钮。
//
// 几何取官方值：
//   - chip：icon 14、gap 3、label 14/20、水平 padding 4、radius sm、颜色
//     business-primary（`ReferenceChip.module.css` / `composer-editor.module.css`）
//   - secret：等宽字体 0.875em、眼睛按钮 1em（`SecretText.module.css`）
//
// 边界：**没有引用就不走这条路径**——普通消息仍然用 `ui.Wrap`，避免把整个
// 消息区的排版都换成近似实现。

import (
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/secretmask"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// refKind 是官方 `ReferenceIconKind` 的三个值。
type refKind int

const (
	refFile refKind = iota
	refFolder
	refSession
)

// referenceSpan 是一条用户文本里的引用（字节偏移）。
type referenceSpan struct {
	Start int
	End   int
	Kind  refKind
	Label string
	Ref   string // 官方 mention 原文（file 是 `@path`，session 是 `@[label](dsh-session:id)`）
}

// parseReferenceSpans 从原文里找出官方 `user-text-rules.ts` 认的两类引用：
//
//	session: `@[label](dsh-session:id)`
//	file:    `@path`（空白边界；`@"带 空格 的路径"` 也认）
//
// **只做识别，不猜文件是否存在**：官方 chip 的 invalid 态需要 owner 解析，
// 我们这一版没有真实判据，所以不画 invalid（不把"没解析"说成"坏了"）。
func parseReferenceSpans(s string) []referenceSpan {
	var out []referenceSpan
	// session wire 优先：`@[label](dsh-session:id)`
	for i := 0; i < len(s); i++ {
		if s[i] != '@' || i+1 >= len(s) || s[i+1] != '[' {
			continue
		}
		labelEnd := strings.IndexByte(s[i+2:], ']')
		if labelEnd < 0 {
			break
		}
		labelEnd += i + 2
		if labelEnd+1 >= len(s) || s[labelEnd+1] != '(' || !strings.HasPrefix(s[labelEnd+2:], "dsh-session:") {
			continue
		}
		close := strings.IndexByte(s[labelEnd+2:], ')')
		if close < 0 {
			continue
		}
		close += labelEnd + 2
		id := s[labelEnd+2+len("dsh-session:") : close]
		if id == "" || strings.ContainsAny(id, " \t\r\n") {
			continue
		}
		// label 里不能有换行（官方正则是 `[^\]\n]+`）。
		if strings.ContainsAny(s[i+2:labelEnd], "\n") {
			continue
		}
		out = append(out, referenceSpan{Start: i, End: close + 1, Kind: refSession, Label: s[i+2 : labelEnd], Ref: s[i : close+1]})
	}
	// file/folder：手工扫，避免 Go RE2 不支持 lookbehind。
	for i := 0; i < len(s); {
		if s[i] != '@' {
			i++
			continue
		}
		if i > 0 && !isRefBoundary(s[i-1]) {
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '[' { // session wire 已处理
			i++
			continue
		}
		start := i
		rawEnd := start + 1
		quoted := false
		if rawEnd < len(s) && s[rawEnd] == '"' {
			quoted = true
			close := strings.IndexByte(s[rawEnd+1:], '"')
			if close < 0 {
				i++
				continue
			}
			rawEnd = rawEnd + 1 + close + 1
		} else {
			for rawEnd < len(s) && !isRefSpace(s[rawEnd]) {
				rawEnd++
			}
			// 官方会剥掉句末标点。
			for rawEnd > start+1 && strings.ContainsRune(".,;:!?，。；：！？", rune(s[rawEnd-1])) {
				rawEnd--
			}
		}
		if rawEnd <= start+2 {
			i++
			continue
		}
		path := s[start+1 : rawEnd]
		if quoted {
			path = strings.Trim(path, `"`)
		}
		if path == "" {
			i++
			continue
		}
		kind := refFile
		if strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`) {
			kind = refFolder
		}
		out = append(out, referenceSpan{Start: start, End: rawEnd, Kind: kind, Label: refLabel(path), Ref: s[start:rawEnd]})
		i = rawEnd
	}
	return out
}

func isRefBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == '[' || c >= 0x80
}

func isRefSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// refLabel 取路径最后一段（官方 `path.split(/[\\/]/).filter(Boolean).at(-1)`）。
func refLabel(path string) string {
	trimmed := strings.TrimRight(path, `/\`)
	if trimmed == "" {
		return path
	}
	if idx := strings.LastIndexAny(trimmed, `/\`); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

// userInlineSegment 是用户文本排版的最小单位。
type inlineSegKind int

const (
	segText inlineSegKind = iota
	segReference
	segSecret
)

type inlineSegment struct {
	kind inlineSegKind
	text string
	ref  referenceSpan
	// secret 只在 kind==segSecret 时有效。
	secret secretmask.Span
	// secretIndex 是这条消息里第几个 secret（眼睛按钮的状态键）。
	secretIndex int
}

// userInlineSegments 把一条用户消息拆成"普通文字 / 引用 chip / 凭据"三种段。
//
// 重叠时**凭据优先**（官方 secret rule 的 order=0，同起点先遮）；
// 引用范围里如果落进凭据，就不再画 chip——遮住的部分不能当文件名展示。
func userInlineSegments(text string) []inlineSegment {
	secrets := secretmask.Spans(text)
	refs := parseReferenceSpans(text)
	// 去掉与凭据重叠的引用。
	keptRefs := refs[:0]
	for _, r := range refs {
		overlap := false
		for _, s := range secrets {
			if r.Start < s.End && r.End > s.Start {
				overlap = true
				break
			}
		}
		if !overlap {
			keptRefs = append(keptRefs, r)
		}
	}
	refs = keptRefs

	type mark struct {
		start, end int
		kind       inlineSegKind
		ref        referenceSpan
		secret     secretmask.Span
		secretIdx  int
	}
	var marks []mark
	for i, s := range secrets {
		marks = append(marks, mark{start: s.Start, end: s.End, kind: segSecret, secret: s, secretIdx: i})
	}
	for _, r := range refs {
		marks = append(marks, mark{start: r.Start, end: r.End, kind: segReference, ref: r})
	}
	if len(marks) == 0 {
		return nil
	}
	// 按起点排序；同起点凭据优先。
	for i := 1; i < len(marks); i++ {
		for j := i; j > 0 && marks[j].start < marks[j-1].start; j-- {
			marks[j], marks[j-1] = marks[j-1], marks[j]
		}
	}
	var out []inlineSegment
	last := 0
	for _, m := range marks {
		if m.start < last {
			continue
		}
		if m.start > last {
			out = append(out, inlineSegment{kind: segText, text: text[last:m.start]})
		}
		if m.kind == segSecret {
			out = append(out, inlineSegment{kind: segSecret, secret: m.secret, secretIndex: m.secretIdx, text: m.secret.Raw})
		} else {
			out = append(out, inlineSegment{kind: segReference, ref: m.ref})
		}
		last = m.end
	}
	if last < len(text) {
		out = append(out, inlineSegment{kind: segText, text: text[last:]})
	}
	return out
}

// inlineToken 是排版时不可再分的一小段。
type inlineToken struct {
	seg    inlineSegment
	text   string
	width  float32
	space  bool
	secret bool
	// secretTail 表示这是被强制拆开的 secret 的最后一段（眼睛按钮挂在这里）。
	secretTail bool
}

// inlineLine 是一行 token 及总宽。
type inlineLine struct {
	tokens []inlineToken
	width  float32
}

// layoutUserInline 按宽度把段排成行。
//
// 文本按"词 + 空白"切；CJK 没有空格，按 rune 切（每个汉字都可断行）。
// 引用 chip 与 secret 是原子 token；放不下时单独换行（secret 太长时按 rune 硬切）。
func layoutUserInline(canvas *gfx.Canvas, segs []inlineSegment, fontSize, maxWidth float32) []inlineLine {
	if maxWidth < 40 {
		maxWidth = 40
	}
	var tokens []inlineToken
	for _, seg := range segs {
		switch seg.kind {
		case segText:
			tokens = append(tokens, textTokens(canvas, seg, fontSize)...)
		case segReference:
			w := referenceChipWidth(canvas, seg.ref.Label, fontSize)
			tokens = append(tokens, inlineToken{seg: seg, width: w})
		case segSecret:
			// secret 用等宽体（官方 0.875em）；宽度按原文算（遮罩同长）。
			w := canvas.TextWidthStyle(seg.text, gfx.TextStyle{Size: fontSize * 0.875, Mono: true})
			token := inlineToken{seg: seg, width: w, secret: true}
			if w > maxWidth {
				// 超宽：按 rune 硬切成若干段，最后一段挂眼睛。
				parts := splitRunesByWidth(canvas, seg.text, fontSize*0.875, maxWidth)
				for i, part := range parts {
					tokens = append(tokens, inlineToken{seg: seg, text: part, width: canvas.TextWidthStyle(part, gfx.TextStyle{Size: fontSize * 0.875, Mono: true}), secret: true, secretTail: i == len(parts)-1})
				}
				continue
			}
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	var lines []inlineLine
	var cur inlineLine
	flush := func() {
		// 行尾空白不占宽度。
		for len(cur.tokens) > 0 && cur.tokens[len(cur.tokens)-1].space {
			cur.width -= cur.tokens[len(cur.tokens)-1].width
			cur.tokens = cur.tokens[:len(cur.tokens)-1]
		}
		if len(cur.tokens) > 0 {
			lines = append(lines, cur)
		}
		cur = inlineLine{}
	}
	for _, tok := range tokens {
		if tok.space && len(cur.tokens) == 0 {
			continue
		}
		if cur.width+tok.width > maxWidth && len(cur.tokens) > 0 {
			flush()
			if tok.space {
				continue
			}
		}
		cur.tokens = append(cur.tokens, tok)
		cur.width += tok.width
	}
	flush()
	return lines
}

// textTokens 把一段普通文字切成词/空白 token；CJK 字符单独成 token。
func textTokens(canvas *gfx.Canvas, seg inlineSegment, fontSize float32) []inlineToken {
	var out []inlineToken
	var word strings.Builder
	flushWord := func() {
		if word.Len() == 0 {
			return
		}
		text := word.String()
		out = append(out, inlineToken{seg: seg, text: text, width: canvas.TextWidth(text, fontSize)})
		word.Reset()
	}
	for _, r := range seg.text {
		switch {
		case r == ' ' || r == '\t':
			flushWord()
			if r == ' ' {
				out = append(out, inlineToken{seg: seg, text: " ", space: true, width: canvas.TextWidth(" ", fontSize)})
			}
		case r == '\n' || r == '\r':
			flushWord()
			out = append(out, inlineToken{seg: seg, text: "", width: 0})
		case isWideRune(r):
			flushWord()
			text := string(r)
			out = append(out, inlineToken{seg: seg, text: text, width: canvas.TextWidth(text, fontSize)})
		default:
			word.WriteRune(r)
		}
	}
	flushWord()
	return out
}

func isWideRune(r rune) bool {
	return r >= 0x2e80 && r <= 0x9fff || r >= 0xf900 && r <= 0xfaff || r >= 0xff00 && r <= 0xffef
}

// splitRunesByWidth 按宽度把一段文字硬切成多段（每段不超过 maxWidth）。
func splitRunesByWidth(canvas *gfx.Canvas, text string, fontSize, maxWidth float32) []string {
	var out []string
	var cur strings.Builder
	var curW float32
	for _, r := range text {
		part := string(r)
		w := canvas.TextWidthStyle(part, gfx.TextStyle{Size: fontSize, Mono: true})
		if curW+w > maxWidth && cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
			curW = 0
		}
		cur.WriteRune(r)
		curW += w
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// referenceChipWidth 是一条 chip 的占位宽度：icon 14 + gap 3 + label + 左右 padding 4×2。
func referenceChipWidth(canvas *gfx.Canvas, label string, fontSize float32) float32 {
	const icon, gap, pad = float32(14), float32(3), float32(4)
	w := canvas.TextWidth(label, fontSize)
	if w > 240-icon-gap-pad*2 {
		w = 240 - icon - gap - pad*2
	}
	return pad*2 + icon + gap + w
}

// hasUserInline 报告这条用户消息要不要走行内渲染。
func hasUserInline(text string) bool {
	return len(secretmask.Spans(text)) > 0 || len(parseReferenceSpans(text)) > 0
}

// drawUserInline 在气泡内画一条用户消息。
func (p *Plugin) drawUserInline(f ui.C, env *pluginapi.Env, msg apiclient.Message, width float32) {
	segs := userInlineSegments(msg.Text)
	if len(segs) == 0 {
		ui.Wrap(f, userDisplayText(msg)).FontSize(p.theme.Fonts.Body)
		return
	}
	fontSize := p.theme.Fonts.Body
	maxWidth := width - 60 // 主区列宽 - 行边距 40 - 气泡左右 padding 20
	if maxWidth < 80 {
		maxWidth = 80
	}
	lines := layoutUserInline(env.Canvas, segs, fontSize, maxWidth)
	for _, line := range lines {
		ui.Row(f).Width(line.width).Items(ui.Center).Children(func() {
			for _, tok := range line.tokens {
				switch tok.seg.kind {
				case segReference:
					p.drawReferenceChip(f, env, tok.seg.ref)
				case segSecret:
					p.drawSecretToken(f, env, msg, tok)
				default:
					if tok.text == "" {
						continue
					}
					ui.Text(f, tok.text).FontSize(fontSize).LineHeight(p.theme.Fonts.BodyLine).
						Color(p.theme.LabelPrimary)
				}
			}
		})
	}
}

// drawSecretToken 画一枚凭据：遮罩态（或本帧已点开的原文）+ 眼睛按钮。
func (p *Plugin) drawSecretToken(f ui.C, env *pluginapi.Env, msg apiclient.Message, tok inlineToken) {
	t := p.theme
	key := p.secretKey(env, msg, tok.seg.secretIndex)
	revealed := p.revealed[key]
	// 整段命中时 tok.text 为空（用原文/遮罩整串）；被硬拆的超长 secret 只在
	// 最后一段挂眼睛，前面几段按"当前可见性"显示对应片段。
	text := tok.text
	if revealed {
		if text == "" {
			text = tok.seg.secret.Raw
		}
	} else if text == "" {
		text = tok.seg.secret.Masked
	} else {
		text = strings.Repeat("*", len([]rune(text)))
	}
	showToggle := tok.text == "" || tok.secretTail
	ui.Row(f).Items(ui.Center).Children(func() {
		ui.Text(f, text).Mono().FontSize(t.Fonts.Body * 0.875).LineHeight(t.Fonts.BodyLine).
			Color(t.LabelPrimary)
		if !showToggle {
			return
		}
		// 官方 EyeGlyph：16 viewBox、1em；点击在原文/遮罩之间切换。
		label := "显示密钥" // 官方 secret.reveal
		if revealed {
			label = "隐藏密钥" // 官方 secret.hide
		}
		id := env.Tree.ID(env.Feat, "secret-toggle-"+p.messageSlotKey(env, msg)+"-"+itoa(tok.seg.secretIndex))
		btn := ui.Row(f).Width(t.Fonts.Body).Height(t.Fonts.Body).Items(ui.Center).
			Justify(ui.Center).Hover(ui.Look{}.Background(t.Hover)).Hit(id, label).
			Children(func() {
				ui.Icon(f, eyeIcon(revealed)).Width(t.Fonts.Body * 0.875).Height(t.Fonts.Body * 0.875).
					Color(t.LabelSecondary)
			})
		if btn.Clicked() {
			if p.revealed == nil {
				p.revealed = map[string]bool{}
			}
			p.revealed[key] = !revealed
		}
	})
}

// messageSlotKey 给"这条消息"一个稳定键：优先宿主 messageId，其次 v4 seq。
func (p *Plugin) messageSlotKey(env *pluginapi.Env, msg apiclient.Message) string {
	// 只认宿主给的真实身份：messageId 优先，其次持久 v4 seq，再其次 load 的 v4 seq。
	// **不用消息下标**：下标会随翻页/窗口漂移，切一次会话键就串了。
	id := msg.MessageID
	if id == "" && msg.V4Seq != nil {
		id = "v4:" + itoa(int(*msg.V4Seq))
	}
	if id == "" && msg.Seq > 0 {
		id = "seq:" + itoa(msg.Seq)
	}
	return env.Data.Cur + "\x00" + id
}

func (p *Plugin) secretKey(env *pluginapi.Env, msg apiclient.Message, index int) string {
	return p.messageSlotKey(env, msg) + "\x00" + itoa(index)
}

// drawReferenceChip 画官方 `ReferenceChip` 的 file/folder/session 三态。
func (p *Plugin) drawReferenceChip(f ui.C, env *pluginapi.Env, ref referenceSpan) {
	t := p.theme
	label := ref.Label
	if ref.Kind == refFolder {
		label += "/"
	}
	box := ui.Row(f).Height(20).Radius(t.Radii.XS).PadX(4).Items(ui.Center).Gap(3).
		Background(gfx.RGBA(0, 0, 0, 0)).Hover(ui.Look{}.Background(t.Hover))
	if ref.Kind == refFile && env.Act.PreviewFile != nil {
		// 官方 file chip 是可点的（openResource）；这里接 Actions.PreviewFile。
		// 壳没接就不画成可点（死按钮比没有更坏）。
		id := env.Tree.ID(env.Feat, "ref-file-"+ref.Ref)
		box = box.Hit(id, label)
	}
	box.Children(func() {
		ui.Icon(f, referenceIcon(ref.Kind, t.Info)).Width(14).Height(14)
		ui.Text(f, label).FontSize(t.Fonts.Body).LineHeight(t.Fonts.BodyLine).Color(t.Info).
			MaxWidth(240 - 14 - 3 - 8).Ellipsis()
	})
	if ref.Kind == refFile && box.Clicked() && env.Act.PreviewFile != nil {
		env.Act.PreviewFile(referencePath(ref.Ref))
	}
}

// referencePath 把 mention 原文还原成裸路径（去掉 @ 与可选引号）。
func referencePath(mention string) string {
	path := strings.TrimPrefix(mention, "@")
	if strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) && len(path) >= 2 {
		path = path[1 : len(path)-1]
	}
	return path
}

// eyeIcon 是官方 `SecretText.tsx` 的 EYE_PATH / EYE_OFF_PATH（16 viewBox、stroke 1）。
func eyeIcon(revealed bool) gfx.Icon {
	commands := []gfx.PathCommand{
		gfx.MoveTo(1.5, 8),
		gfx.CubicTo(2.9, 5.25, 5.25, 3.5, 8, 3.5),
		gfx.CubicTo(10.75, 3.5, 13.1, 5.25, 14.5, 8),
		gfx.CubicTo(13.1, 10.75, 10.75, 12.5, 8, 12.5),
		gfx.CubicTo(5.25, 12.5, 2.9, 10.75, 1.5, 8),
		gfx.ClosePath(),
	}
	if revealed {
		commands = append(commands, gfx.MoveTo(2.5, 2.5), gfx.LineTo(13.5, 13.5))
	} else {
		commands = append(commands,
			gfx.MoveTo(10, 8), gfx.CubicTo(10, 9.1, 9.1, 10, 8, 10),
			gfx.CubicTo(6.9, 10, 6, 9.1, 6, 8),
			gfx.CubicTo(6, 6.9, 6.9, 6, 8, 6),
			gfx.CubicTo(9.1, 6, 10, 6.9, 10, 8), gfx.ClosePath())
	}
	return gfx.Icon{ViewBox: 16, Stroke: 1, Commands: commands}
}
