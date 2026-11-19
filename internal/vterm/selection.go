package vterm

import (
	"runtime"
	"strings"
)

// 这一文件是**选区模型**：坐标怎么定、哪些格子被选中、选中的文本长什么样。
//
// 官方终端里的选区不是自己写的，是 xterm 的 `SelectionService` / `SelectionModel`。
// 官方 `terminal.tsx:102-105` 只 `new Terminal({minimumContrastRatio, cursorBlink,
// fontSize, fontFamily, scrollback})`，**没有**关掉或改写选区，所以以下行为全部是
// xterm 的默认行为，我们照它投影（证据是官方包里内联的那份 xterm，
// `dsh-client-ui-sidebar-terminal/lib/client.terminal.js`，509421 字节，用 `grep -b`
// 可复现；每条都带字节偏移）。
//
//  1. **坐标是缓冲区绝对行**：`selectionStart/End = [列, 行]`，行是虚拟缓冲行号
//     （0 = 滚回里最老的一行），不是视口行。xterm 的鼠标坐标转缓冲坐标在
//     `_getMouseBufferCoords`（byte 120966 起），视口行 + `ydisp` 才是缓冲行。
//     滚回里的行和屏幕行因此是**同一套坐标**，选到屏幕外不需要特殊处理。
//  2. **三种模式**：`_activeSelectionMode` 0 = 线性（流式）、1 = 词、2 = 行、
//     3 = 矩形（byte 122249 `_handleSingleClick` 里
//     `this._activeSelectionMode = this.shouldColumnSelect(e) ? 3 : 0`，
//     byte 122685 `shouldColumnSelect(e)` = `e.altKey && !(isMac && …)`，
//     所以 **Windows/Linux 上是按住 Alt 拖**）。
//  3. **词/行选中不改 Start，改的是 StartLength**：`_selectLineAt`
//     （byte 128691）设 `selectionStart=[0,first]`、`selectionEnd=undefined`、
//     `selectionStartLength=cols*(last-first)+cols`；`finalSelectionEnd`
//     （byte 95563）再按 Start+StartLength 算出终点。
//  4. **反向拖选**：`finalSelectionStart/End` 在 `areSelectionValuesReversed()`
//     为真时互换两端（byte 95563）。
//  5. **文本**（byte 117677 的 `selectionText`）：
//     - 线性：首行 `translateBufferLineToString(line,true,startCol,endCol)`；
//       中间行整行；末行 `(line,true,0,endCol)`；但**折行（isWrapped）的行拼到
//       前一行**（`e?.isWrapped ? s[s.length-1]+=t : s.push(t)`）。
//     - 矩形：每行都是 `(line,true,minCol,maxCol)`。
//     - 收尾 `s.map(e => e.replace(g,' '))`，`g = new RegExp(String.fromCharCode(160),'g')`
//       （byte 115700）：**NBSP 换成普通空格**。
//     - 连接符 `c.isWindows ? '\r\n' : '\n'`（byte 117677 同一句）。
//  6. **行尾空白**：`translateToString(trimRight=true)` 会把 endCol 夹到
//     `getTrimmedLength()`（byte 219185：最后一个有内容的格子 + 它的宽度），
//     所以行尾空格不会进剪贴板。
//  7. **宽字不切半字**：`translateToString`（byte 219948）逐格累加
//     `col += width || 1`，宽字占两列；起点落在宽字的**尾巴**格上会走成空格。
//     所以 xterm 在鼠标那层就把它挪开：`_handleSingleClick`（byte 122249）与
//     `_handleMouseMove`（byte 123245）都有
//     `0 === line.hasWidth(col) && col < cols → col++`。
//  8. **词分隔符**取 `wordSeparator` 默认值 `" ()[]{}',\"`"`
//     （DEFAULT_OPTIONS，byte 278356），判据 `_isCharWordSeparator`（byte 128691）：
//     `width !== 0 && wordSeparator.indexOf(cell.getChars()) >= 0`。
//
// 本包的边界：只做几何与文本，**不持有指针状态**（谁按住、拖到哪、点了几下是
// 界面的事）；也不做 xterm 的「滚回行被裁掉时把坐标上移」（`handleTrim`，
// byte 95563），因为滚回容量由宿主 environment 定、界面每帧重新夹坐标。

// SelPos 是一格在**虚拟缓冲**里的位置：X 是列，Line 是绝对行号
// （0 = 滚回里最老的一行，ScrollbackLen() 是第一行屏幕行）。
type SelPos struct {
	X, Line int
}

// SelMode 是 xterm 的 `_activeSelectionMode` 四档。
type SelMode uint8

const (
	// SelLinear 是拖动（以及 Shift+点击）的默认模式：从起点一路流到终点。
	SelLinear SelMode = iota
	// SelWord 是双击选中的词（终点由 StartLength 给出）。
	SelWord
	// SelLine 是三击选中的整行（`_selectLineAt`）。
	SelLine
	// SelColumn 是按住 Alt 拖出来的矩形选区（`shouldColumnSelect`）。
	SelColumn
)

// Selection 是 xterm `SelectionModel` 的投影。
//
// HasEnd 对应 xterm 的「`selectionEnd !== undefined`」：词/行选中只有起点和长度，
// 没有终点（见 `finalSelectionEnd` 的两条分支）。
type Selection struct {
	Start, End SelPos
	HasEnd     bool
	// StartLength 是 xterm 的 `selectionStartLength`：从 Start 起**跨几列**。
	StartLength int
	Mode        SelMode
}

// Reversed 报告这是一次反向（从右下往左上）的拖选，
// 即 xterm 的 `areSelectionValuesReversed`（byte 95563）。
func (s Selection) Reversed() bool {
	if !s.HasEnd {
		return false
	}
	return s.Start.Line > s.End.Line || (s.Start.Line == s.End.Line && s.Start.X > s.End.X)
}

// Normalized 返回 xterm 的 `finalSelectionStart` / `finalSelectionEnd`
// （byte 95563，逐条同序）：
//
//	start = reversed ? End : Start
//	end   = 没有 End 或反向时 → Start + StartLength（跨行时按列回卷）
//	        有 End 且与 Start 同一行且有 StartLength → max(Start+StartLength, End)
//	        否则 End
func (s Selection) Normalized(cols int) (SelPos, SelPos) {
	if cols <= 0 {
		cols = 1
	}
	start, end := s.Start, s.End
	if !s.HasEnd || s.Reversed() {
		length := s.StartLength
		x := s.Start.X + length
		if x > cols {
			if x%cols == 0 {
				// 正好整行：终点落在上一行的最后一列（官方 `[cols, y+floor(x/cols)-1]`）。
				end = SelPos{X: cols, Line: s.Start.Line + x/cols - 1}
			} else {
				end = SelPos{X: x % cols, Line: s.Start.Line + x/cols}
			}
		} else {
			end = SelPos{X: x, Line: s.Start.Line}
		}
		if !s.HasEnd {
			return s.Start, end
		}
		return s.End, end
	}
	if s.StartLength > 0 && s.End.Line == s.Start.Line {
		x := s.Start.X + s.StartLength
		if x > cols {
			return start, SelPos{X: x % cols, Line: s.End.Line + x/cols}
		}
		if x < s.End.X {
			x = s.End.X
		}
		return start, SelPos{X: x, Line: s.End.Line}
	}
	return start, end
}

// Empty 报告这一份选区有没有真的选中东西（xterm 的 `hasSelection`，
// byte 117544：起终点都存在且不是同一格；没有终点时看长度）。
func (s Selection) Empty() bool {
	if !s.HasEnd {
		return s.StartLength <= 0
	}
	return s.Start == s.End
}

// Lines 返回选区覆盖的**绝对行**范围（含两端，顺序已归一）。
func (s Selection) Lines() (first, last int) {
	start, end := s.Normalized(1 << 20)
	if start.Line > end.Line {
		return end.Line, start.Line
	}
	return start.Line, end.Line
}

// Contains 报告 (x, line) 这一格是否在选区里，逐条照 `isCellInSelection`
// （byte 94437 与 byte 118065 的 `_areCoordsInSelection`）：
//
//	矩形：start.x <= end.x ? x∈[start.x,end.x) && line∈[start.line,end.line]
//	                         : x∈[end.x,start.x) && line∈[start.line,end.line]
//	线性：line 夹在两端之间；首行从 start.x 起，末行到 end.x 止（末行不含 end.x）
func (s Selection) Contains(cols, x, line int) bool {
	if cols <= 0 {
		cols = 1
	}
	if s.Empty() {
		return false
	}
	start, end := s.Normalized(cols)
	if start.Line > end.Line || (start.Line == end.Line && start.X > end.X) {
		start, end = end, start
	}
	if line < start.Line || line > end.Line {
		return false
	}
	if s.Mode == SelColumn {
		lo, hi := start.X, end.X
		if lo > hi {
			lo, hi = hi, lo
		}
		return x >= lo && x < hi
	}
	if line > start.Line && line < end.Line {
		return true
	}
	if start.Line == end.Line {
		return x >= start.X && x < end.X
	}
	if line == start.Line {
		return x >= start.X
	}
	return x < end.X
}

// SelectionText 返回选区的文本，与 xterm 的 `selectionText`（byte 117677）一致。
//
// g 是**已经取到的一段缓冲**（虚拟缓冲的第 first 行起、共 g.Height 行）；
// 选区里落在 g 之外的行按空串处理（界面只会取选中范围，取不到就是壳没给）。
// 行间连接符按官方平台判据 `isWindows ? "\r\n" : "\n"`。
func SelectionText(g *Grid, first, cols int, s Selection) string {
	if g == nil || cols <= 0 || s.Empty() {
		return ""
	}
	start, end := s.Normalized(cols)
	reversed := start.Line > end.Line || (start.Line == end.Line && start.X > end.X)
	if reversed {
		start, end = end, start
	}
	lines := make([]string, 0, end.Line-start.Line+1)
	if s.Mode == SelColumn {
		if start.X == end.X {
			return ""
		}
		lo, hi := start.X, end.X
		if lo > hi {
			lo, hi = hi, lo
		}
		for line := start.Line; line <= end.Line; line++ {
			lines = append(lines, g.LineText(line-first, lo, hi))
		}
	} else if start.Line == end.Line {
		lines = append(lines, g.LineText(start.Line-first, start.X, end.X))
	} else {
		lines = append(lines, g.LineText(start.Line-first, start.X, -1))
		for line := start.Line + 1; line <= end.Line-1; line++ {
			lines = append(lines, g.LineText(line-first, 0, -1))
		}
		lines = append(lines, g.LineText(end.Line-first, 0, end.X))
	}
	for i, line := range lines {
		// 官方 `replace(g,' ')`，g 是 NBSP 的全局正则（byte 115700）。
		lines[i] = strings.ReplaceAll(line, "\u00a0", " ")
	}
	return strings.Join(lines, selectionLineSeparator())
}

// selectionLineSeparator 是官方 `c.isWindows ? '\r\n' : '\n'`（byte 117677）。
func selectionLineSeparator() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}
	return "\n"
}

// LineText 把第 row 行（相对 g 的第一行）在 [startCol,endCol) 列之间的文字读出来，
// 逐条照官方 `BufferLine.translateToString(trimRight, startCol, endCol)`
// （byte 219948）：
//
//   - `endCol < 0` 表示到行尾；
//   - 先按 trimRight 夹到 `getTrimmedLength()`（byte 219185）；
//   - 逐格取字，**列按格子宽度前进**（`col += width || 1`），所以宽字不会被切一半；
//   - 空格格（没有内容）出一格空格（`WHITESPACE_CELL_CHAR=' '`，byte 225334）。
func (g *Grid) LineText(row, startCol, endCol int) string {
	if g == nil || row < 0 || row >= g.Height {
		return ""
	}
	if startCol < 0 {
		startCol = 0
	}
	if endCol < 0 || endCol > g.Width {
		endCol = g.Width
	}
	if trimmed := g.TrimmedLength(row); endCol > trimmed {
		endCol = trimmed
	}
	var out strings.Builder
	for col := startCol; col >= 0 && col < endCol; {
		cell, ok := g.Cell(col, row)
		if !ok {
			break
		}
		if cell.Content == "" {
			out.WriteByte(' ')
		} else {
			out.WriteString(cell.Content)
		}
		width := cell.Width
		if width <= 0 {
			width = 1
		}
		col += width
	}
	return out.String()
}

// TrimmedLength 是这一行最后一个**有内容**的格子之后的那一列
// （官方 `getTrimmedLength`，byte 219185：`index + width`）。
//
// 登记一处差异：xterm 判「有内容」是 `code !== 0`，即**未写过的格子**没有字符；
// 上游模拟器把没写过的格子填成空格字形（实测 `Content == " "`，与真正打出来的
// 空格不可区分），所以这里只能退到「最后一个非空格格」。
// 后果只有一条：**行尾真被打出来的空格**在复制时也会被去掉（官方会留着），
// 行内空格与行首空格不受影响。要彻底对齐得让上游保留「未写过」这一位，不在本仓兜底。
func (g *Grid) TrimmedLength(row int) int {
	if g == nil || row < 0 || row >= g.Height {
		return 0
	}
	for x := g.Width - 1; x >= 0; x-- {
		cell, ok := g.Cell(x, row)
		if !ok || cell.Content == "" || cell.Content == " " {
			continue
		}
		width := cell.Width
		if width <= 0 {
			width = 1
		}
		return x + width
	}
	return 0
}

// WideTail 报告这一格是不是宽字的**尾巴**（`Cell.Width == 0`，见 cell.go 的 Cell）。
//
// 鼠标落到尾巴格上要挪到下一格（官方 `_handleSingleClick` / `_handleMouseMove`：
// `0 === line.hasWidth(col) && col < cols → col++`），否则选区会从一个字的中间开始。
func (g *Grid) WideTail(row, col int) bool {
	cell, ok := g.Cell(col, row)
	return ok && cell.Width == 0 && cell.Content == ""
}

// SnapColumn 把一列夹进 [0,cols) 并躲开宽字的尾巴（官方同一条判据）。
func (g *Grid) SnapColumn(row, col, cols int) int {
	if col < 0 {
		return 0
	}
	if col >= cols {
		return max(cols-1, 0)
	}
	for col < cols-1 && g.WideTail(row, col) {
		col++
	}
	return col
}

// TerminalWordSeparator 是 xterm `wordSeparator` 的默认值
// （DEFAULT_OPTIONS，byte 278356），官方没有覆盖它。
const TerminalWordSeparator = " ()[]{}',\"`"

// WordAt 返回 (row 行, col 列) 这个格子所在的词，坐标是**列**：起点 col 与跨几列。
//
// 照官方 `_getWordAt`（byte 126328 起）的两条分支：
//
//   - 落在空格上：向两侧吃掉**一整串空格**（`while ' ' === charAt(a-1)` /
//     `while ' ' === charAt(l+1)`）；
//   - 否则向两侧扩到词分隔符为止（`_isCharWordSeparator`：宽字以外的、命中
//     `wordSeparator` 的字）。
//
// 与官方的差异（登记，不假装相同）：官方还会把**折行**的上一行/下一行接进来
// （`_getWordAt` 里的 `lines.get(y-1)` / `lines.get(y+1)` + `isWrapped` 判断）。
// 本包的 Grid 不带折行标志，所以词选在行边界收住。
func (g *Grid) WordAt(row, col, cols int) (start, length int, ok bool) {
	if g == nil || row < 0 || row >= g.Height || cols <= 0 {
		return 0, 0, false
	}
	if col < 0 || col >= cols {
		return 0, 0, false
	}
	col = g.SnapColumn(row, col, cols)
	cell, ok := g.Cell(col, row)
	if !ok {
		return 0, 0, false
	}
	if cell.Content == "" || cell.Content == " " {
		start, end := col, col+1
		for start > 0 {
			prev, ok := g.Cell(start-1, row)
			if !ok || prev.Content != " " {
				break
			}
			start--
		}
		for end < cols {
			next, ok := g.Cell(end, row)
			if !ok || next.Content != " " {
				break
			}
			end++
		}
		if end <= start {
			end = start + 1
		}
		return start, end - start, true
	}
	if isWordSeparator(cell) {
		return col, 1, true
	}
	start, end := col, col
	width := cell.Width
	if width <= 0 {
		width = 1
	}
	end += width
	for start > 0 {
		prev, ok := g.Cell(start-1, row)
		if !ok || isWordSeparator(prev) {
			break
		}
		// 左边可能落在宽字的尾巴上：那是同一个字，整格吃掉。
		start--
		if prev.Width == 0 && prev.Content == "" && start > 0 {
			start--
		}
	}
	for end < cols {
		next, ok := g.Cell(end, row)
		if !ok || isWordSeparator(next) {
			break
		}
		if next.Width == 0 && next.Content == "" {
			end++
			continue
		}
		w := next.Width
		if w <= 0 {
			w = 1
		}
		end += w
	}
	return start, end - start, true
}

// WordSelection 返回双击 (row,col) 应当选中的那份选区：模式 SelWord，
// 终点由 StartLength 给出、HasEnd 为假（官方 `_selectWordAtCursor` →
// `selectionEnd = undefined`，byte 119508）。
func (g *Grid) WordSelection(first, row, col, cols int) (Selection, bool) {
	startCol, length, ok := g.WordAt(row-first, col, cols)
	if !ok || length <= 0 {
		return Selection{}, false
	}
	return Selection{
		Start:       SelPos{X: startCol, Line: row},
		StartLength: length,
		Mode:        SelWord,
	}, true
}

// LineSelection 返回三击 (row) 应当选中的整行：官方 `_selectLineAt`
// （byte 128691）设 `[0,line]` + `StartLength = cols`，没有终点。
func LineSelection(line, cols int) Selection {
	return Selection{Start: SelPos{X: 0, Line: line}, StartLength: cols, Mode: SelLine}
}

// isWordSeparator 照 `_isCharWordSeparator`（byte 128691）：
// `width !== 0 && wordSeparator.indexOf(cell.getChars()) >= 0`。
func isWordSeparator(cell Cell) bool {
	if cell.Width == 0 || cell.Content == "" {
		return false
	}
	return strings.ContainsAny(cell.Content, TerminalWordSeparator)
}

// WordEndAt 返回拖动到 (row,col) 时词选模式的终点，照官方 `_selectToWordAt`
// （byte 128691 前的那段）：反向拖到词首、正向拖到词尾。
func (g *Grid) WordEndAt(first, row, col, cols int, reversed bool) SelPos {
	startCol, length, ok := g.WordAt(row-first, col, cols)
	if !ok {
		return SelPos{X: col, Line: row}
	}
	if reversed {
		return SelPos{X: startCol, Line: row}
	}
	return SelPos{X: startCol + length, Line: row}
}
