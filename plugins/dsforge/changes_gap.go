// changes_gap.go —— 官方「未变更行 gap 可点开」的**逐行移植**（GUI 账本 #64 ① 的最后一项）。
//
// # 官方规格（逐条 file:line，rc.2 随包 src）
//
//	OFF:…/src/client/diff-rows.ts:28-35   `DiffSegment = {kind:'rows',rows} | {kind:'gap',index,count|null}`
//	OFF:…/src/client/diff-rows.ts:151-184 `segmentsOf(file, lines, open)`：hunk 前 / hunk 间 / 最后一个
//	                                     hunk 之后各折成**一个** gap；`open.has(index)` 且行已知时把那
//	                                     一段未变更行**整段**摊成 context 行，否则 flush 出 gap 段
//	OFF:…/src/client/diff-rows.ts:172       hunk 前/间的判据 `hunk.newStart - 1 > newEnd`
//	OFF:…/src/client/diff-rows.ts:174-175   `newEnd = newStart + newLines - 1`、`oldEnd = oldStart + oldLines - 1`
//	OFF:…/src/client/diff-rows.ts:177-181   末尾 gap：非新增/非删除/最后一个 hunk 没到头，且
//	                                     `lines.length > newEnd`；行还没读到（lines undefined）时
//	                                     **照样画一条 count=null 的 gap**（文案是另一句）
//	OFF:…/src/client/diff-rows.ts:131-138   `endsNewSide()`：`\ No newline at end of file`
//	                                     跟在非删除行后面 = 这个 hunk 就结束新侧，没有末尾 gap
//	OFF:…/src/client/diff-rows.ts:104-108   `textLines()`：按 `\n` 切，末尾换行不算最后一行
//	OFF:…/src/client/diff-rows.ts:117-128   `matchesHunks()`：文件文本必须**逐行等于**新侧
//	                                     （context 与 add 行按 newStart 起算），读后文件又变过就拒
//	OFF:…/src/client/diff-rows.ts:37-40     `drawn()`：显示前去掉行尾 `\r`（CRLF patch）
//	OFF:…/src/client/DiffFile.tsx:411-415   gap 是一个 `<button class=.gap>`，图标
//	                                     `IconChevronsUpDownOutlineRegular size={12}`，文案两句
//	OFF:…/src/client/DiffFile.tsx:411      `disabled={text === 'loading'}`
//	OFF:…/src/client/locales.ts:276-277     `'diff.unchanged': '{count} 行未变更'` / `'diff.unchangedRest': '其余未变更的行'`
//	OFF:…/src/client/ChangesTab.module.css:734-766 `.gap` 条纹（forge-fill 底、26px 高、0 12px、
//	                                     gap 6、12/18、label-tertiary；hover 换底与字色；disabled 光标 progress）
//	OFF:…/src/client/ChangesTab.tsx:776-796  `expand()`：**只并集加入** index，然后把该文件新侧全文
//	                                     读出来（local → `localText(scope, path)`），`matchesHunks`
//	                                     不过就报 stale 并**把 open/text 一起清掉**
//
// # 能不能再收起：**不能**（官方就是不能）
//
// `OFF:…/src/client/ChangesTab.tsx:778` 是 `new Set([...open[path] ?? [], index])`，
// 全文件只有这一个写 `open` 的地方（`grep -n "open\[" ChangesTab.tsx` 只有 :778 与 :1118 的读），
// **没有删除路径**；展开后那一段 gap 段消失、变成 context 行，也就没有可点的按钮了。
// 唯一的"收起来"是**文件级**折叠（`:1019-1023` 的 `toggle`），而它只改 `chosen[path]`，
// `Expansion.open` 原样保留。所以本仓照抄：单 gap 不可收起，文件卡折叠只是把行藏起来。
//
// # 行从哪来（数据来源，不是编的）
//
// 官方 `localText(scope, path)` 读**工作树/暂存区/某个提交里的真实文件**：
// `HOST:internal/forge/git.go:179-207`（`git show :path` / `git show <commit>:path` /
// `os.ReadFile`）→ `HOST:cmd/dshd/serve_forge_git.go:76-89` → GUI `internal/apiclient/forge.go:264-277`
// → `cmd/app/forge_wiring.go` 的 `forgeLoadText` → `pluginapi.ForgeDiff.Texts[path]`。
// 本文件只消费 `Texts`，**一个字符都不造**；验证不过（文件在读 diff 之后又变过）就按官方拒绝。
package dsforge

import (
	"sort"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// changesSegmentKind 是官方 `DiffSegment['kind']`（`OFF:src/client/diff-rows.ts:29-35`）。
type changesSegmentKind uint8

const (
	// changesSegmentRows 是一段按 patch 顺序画的行（hunk 行或已摊开的未变更行）。
	changesSegmentRows changesSegmentKind = iota
	// changesSegmentGap 是一段折起来的未变更行。
	changesSegmentGap
)

// changesSegment 是官方 `DiffSegment`（`OFF:src/client/diff-rows.ts:28-35`）。
//
// `index` 是这段未变更行**紧接其前**的那个 hunk 的下标；最后一个 hunk 之后的
// 那段用 `len(file.Hunks)`（官方注释 `:32-33` 原话）。`count == nil` 表示
// "新侧全文还没读到，行数不知道"（官方 `count: null`）。
type changesSegment struct {
	kind  changesSegmentKind
	rows  []changesRow
	index int
	count *int
}

// changesTextLines 是官方 `textLines()`（`OFF:…/src/client/diff-rows.ts:104-108`）：
// 按 `\n` 切；**末尾的那个换行不算多一行**。`\r` 保留在这里（官方注释 :101-103 明说），
// 显示时由 `changesDrawnText` 去掉，与 `drawn()` 一致。
func changesTextLines(text string) []string {
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// changesDrawnText 是官方 `drawn()`（`OFF:…/src/client/diff-rows.ts:38-40`）：
// 显示前去掉行尾的 `\r`（CRLF 的 patch 与文件都带它）。
func changesDrawnText(text string) string {
	return strings.TrimSuffix(text, "\r")
}

// changesMatchesHunks 是官方 `matchesHunks()`（`OFF:…/src/client/diff-rows.ts:117-128`）：
// 文件文本必须**就是**这批 hunk 描述的新侧——每个 context/add 行都落在它自己的
// 行号上。读 diff 之后文件又被改过时返回 false（官方据此报 `toast.expandStale`
// 并拒绝展开，见 `ChangesTab.tsx:782-794`）。
func changesMatchesHunks(lines []string, hunks []apiclient.ForgeHunk) bool {
	for _, hunk := range hunks {
		at := hunk.NewStart - 1
		for _, line := range hunk.Lines {
			if line == "" {
				// 官方 `line.charAt(0)` 是 ""，既不等于 '-' 也不等于 '\\'，
				// 于是拿 `line.slice(1)`（""）去比——对不上就 false。
				if at < 0 || at >= len(lines) || lines[at] != "" {
					return false
				}
				at++
				continue
			}
			sign := line[0]
			if sign == '-' || sign == '\\' {
				continue
			}
			if at < 0 || at >= len(lines) || lines[at] != line[1:] {
				return false
			}
			at++
		}
	}
	return true
}

// changesEndsNewSide 是官方 `endsNewSide()`（`OFF:…/src/client/diff-rows.ts:131-138`）：
// `\ No newline at end of file` 跟在一个**非删除**行后面时，这个 hunk 就结束新侧
// （文件末尾没有多出来的行，不该再画末尾 gap）。
func changesEndsNewSide(hunk apiclient.ForgeHunk) bool {
	previous := "-"
	for _, line := range hunk.Lines {
		if strings.HasPrefix(line, `\`) && !strings.HasPrefix(previous, "-") {
			return true
		}
		previous = line
	}
	return false
}

// changesHunkRows 是官方 `hunkRows()`（`OFF:…/src/client/diff-rows.ts:49-61`）：
// context 行两边都数、删行只数旧侧、加行只数新侧，`\ No newline at end of file`
// **不是一行**（官方 `sign !== '\\'` 才 push）。行尾 `\r` 按 `drawn()` 去掉。
func changesHunkRows(hunk apiclient.ForgeHunk, index int) []changesRow {
	rows := make([]changesRow, 0, len(hunk.Lines))
	oldNo, newNo := hunk.OldStart, hunk.NewStart
	for _, line := range hunk.Lines {
		row := changesDiffRowOf(line, oldNo, newNo)
		if row.old != nil {
			oldNo++
		}
		if row.new != nil {
			newNo++
		}
		if row.old == nil && row.new == nil {
			continue // `\ No newline at end of file`
		}
		row.text = changesDrawnText(row.text)
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		rows[0].hunk = index
	}
	return rows
}

// changesSegmentsOf 是官方 `segmentsOf()`（`OFF:…/src/client/diff-rows.ts:151-184`）的逐行移植。
//
// `lines` 是该文件的**新侧全文**（`changesMatchesHunks` 验过的那一份），还没读到时传 nil；
// `open` 是用户点开过的 gap 下标集合。返回的段按文件顺序排列。
func changesSegmentsOf(file apiclient.ForgeDiffFile, lines []string, open map[int]bool) []changesSegment {
	segments := make([]changesSegment, 0, len(file.Hunks)*2+1)
	rows := make([]changesRow, 0, 64)
	flush := func() {
		if len(rows) == 0 {
			return
		}
		segments = append(segments, changesSegment{kind: changesSegmentRows, rows: rows})
		rows = nil
	}
	// gap 是官方 :159-168 的闭包：`from`/`to` 是**新侧**行号（含两端），
	// `offset` 是同一段未变更行在旧侧的偏移（旧号 = 新号 + offset）。
	gap := func(index, from int, to *int, offset int) {
		if lines != nil && to != nil && open[index] {
			added := 0
			for next := from; next <= *to; next++ {
				if next-1 < 0 || next-1 >= len(lines) {
					// 行数对不上（文件比 hunk 描述的短）：不给半段假行，退回折叠态。
					break
				}
				oldNo, newNo := next+offset, next
				rows = append(rows, changesRow{
					old: &oldNo, new: &newNo, sign: " ", hunk: -1,
					text: changesDrawnText(lines[next-1]),
				})
				added++
			}
			if added > 0 {
				return
			}
		}
		flush()
		var count *int
		if to != nil {
			value := *to - from + 1
			if value < 0 {
				value = 0
			}
			count = &value
		}
		segments = append(segments, changesSegment{kind: changesSegmentGap, index: index, count: count})
	}

	newEnd, oldEnd := 0, 0
	for index, hunk := range file.Hunks {
		if hunk.NewStart-1 > newEnd {
			to := hunk.NewStart - 1
			gap(index, newEnd+1, &to, hunk.OldStart-hunk.NewStart)
		}
		rows = append(rows, changesHunkRows(hunk, index)...)
		newEnd = hunk.NewStart + hunk.NewLines - 1
		oldEnd = hunk.OldStart + hunk.OldLines - 1
	}
	last := len(file.Hunks) - 1
	if last >= 0 && file.Status != apiclient.ForgeFileAdded && file.Status != apiclient.ForgeFileDeleted &&
		!changesEndsNewSide(file.Hunks[last]) {
		index := len(file.Hunks)
		if lines == nil {
			gap(index, newEnd+1, nil, oldEnd-newEnd)
		} else if len(lines) > newEnd {
			to := len(lines)
			gap(index, newEnd+1, &to, oldEnd-newEnd)
		}
	}
	flush()
	return segments
}

// ── 界面态：哪些 gap 被点开、哪些正文正在读 ─────────────────────────────

// gapSet 返回一个文件已点开的 gap 下标集合（nil = 没有）。
//
// 官方把这份状态放在 `Expansion.open`（`OFF:…/src/client/ChangesTab.tsx:177-181`
// 的 `Record<path, ReadonlySet<number>>`）。本仓同一形状，按会话分桶。
func (s *changesOpen) gapSet(session, path string) map[int]bool {
	return s.gaps[session][path]
}

// openGap 把一个 gap 加入展开集合（官方 `ChangesTab.tsx:778` 的**并集**语义：
// 只加不删——单个 gap 不能被收起，见本文件头）。
func (s *changesOpen) openGap(session, path string, index int) {
	if s.gaps[session] == nil {
		s.gaps[session] = map[string]map[int]bool{}
	}
	if s.gaps[session][path] == nil {
		s.gaps[session][path] = map[int]bool{}
	}
	s.gaps[session][path][index] = true
}

// clearGaps 丢掉一个文件的全部展开态（官方读正文失败/读到过期文件时
// `ChangesTab.tsx:790-794` 把 `open[path]` 与 `texts[path]` 一起删掉）。
func (s *changesOpen) clearGaps(session, path string) {
	if byPath := s.gaps[session]; byPath != nil {
		delete(byPath, path)
	}
}

// gapWanted 报告这个文件的正文是不是"已经在读"（官方 `texts[path] === 'loading'`）。
func (s *changesOpen) gapWanted(session, path string) bool {
	return s.gapWant[session][path]
}

// setGapWanted 记下"正在读这个文件的正文"。
func (s *changesOpen) setGapWanted(session, path string, wanted bool) {
	if s.gapWant[session] == nil {
		s.gapWant[session] = map[string]bool{}
	}
	if wanted {
		s.gapWant[session][path] = true
		return
	}
	delete(s.gapWant[session], path)
}

// resetGaps 在换范围/换 diff 时清空一个会话的全部 gap 状态（官方换一份
// `ChangesData` 就把 `Expansion` 整个换掉，`ChangesTab.tsx:773`）。
func (s *changesOpen) resetGaps(session string) {
	delete(s.gaps, session)
	delete(s.gapWant, session)
	delete(s.gapScope, session)
}

// syncGapScope 在 diff 的范围变了时清空 gap 状态（返回本帧适用的范围）。
func (s *changesOpen) syncGapScope(session, scope string) {
	if s.gapScope[session] != scope {
		s.resetGaps(session)
		if s.gapScope == nil {
			s.gapScope = map[string]string{}
		}
		s.gapScope[session] = scope
	}
}

// ── 正文读取：只消费壳给的 `Texts`，一个字符都不造 ───────────────────────

// gapLinesOf 取一个文件可用于展开的**新侧行**；nil = 还没有可用正文。
//
// 壳的 `ForgeDiff.Texts[path]` 是 `forgePanel.localText` 的原文
// （`HOST:internal/forge/git.go:179-207` 用 `git show :path` / `git show <commit>:path`
// / `os.ReadFile` 读真实文件）。这里按官方 `ChangesTab.tsx:782-786` 同样处理：
// `textLines()` 切行后必须过 `matchesHunks()`，否则视为过期、不给展开。
func gapLinesOf(diff *pluginapi.ForgeDiff, file apiclient.ForgeDiffFile) []string {
	if diff == nil {
		return nil
	}
	text, ok := diff.Texts[file.Path]
	if !ok {
		return nil
	}
	lines := changesTextLines(text)
	if !changesMatchesHunks(lines, file.Hunks) {
		return nil
	}
	return lines
}

// gapText 是 gap 行上那句官方文案（`OFF:…/src/client/locales.ts:276-277`）：
// 行数已知时 `%s 行未变更`，未知时 `其余未变更的行`。
//
// 数字用千分位：官方 `DiffFile.tsx:414` 是 `segment.count.toLocaleString('en-US')`
// （1200 → "1,200"），不是裸整数。
func gapText(segment changesSegment) string {
	if segment.count == nil {
		return copyDiffUnchangedRest
	}
	return sprintf(copyDiffUnchanged, changesThousands(*segment.count))
}

// changesThousands 是官方 `toLocaleString('en-US')` 对非负整数的效果：每三位一个逗号。
func changesThousands(n int) string {
	if n < 0 {
		return itoa(n)
	}
	digits := itoa(n)
	if len(digits) <= 3 {
		return digits
	}
	var out strings.Builder
	for i, ch := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(ch)
	}
	return out.String()
}

// gapTextSignature 是左右对比排版缓存的键的一部分：展开了哪些 gap、正文到没到。
//
// 排版随这两件事变化（摊开 N 行 vs 一条 26px 的条纹），缓存必须把它们算进去，
// 否则点开 gap 后看到的还是旧排版。
func gapTextSignature(open map[int]bool, lines []string) string {
	indexes := make([]int, 0, len(open))
	for index := range open {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := ""
	for i, index := range indexes {
		if i > 0 {
			out += ","
		}
		out += itoa(index)
	}
	out += "|"
	if lines != nil {
		out += itoa(len(lines))
	}
	return out
}
