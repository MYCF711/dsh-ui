// changes.go —— 「写过的文件」列表上那半条**按文件看 diff**（GUI 账本 #14 的缺口服）。
//
// # 数据从哪来
//
// 宿主补齐了官方 deliverables 的三条变更路由（`GET /api/changes.summary`、
// `GET /api/changes.diff`、`POST /api/changes.open`，
// HOST:internal/transport/handlers_changes_routes.go），由
// `internal/apiclient/changes.go` 做客户端。插件拿不到 `*apiclient.Client`
// （红线 R2：模块只拿 `*Scope`），所以这里照 `dsattachment.SetSink` 的形状
// 开一个**包级注入点**，壳在装配时塞一个实现进来
// （壳侧接法见 cmd/app/official_panels_wiring.go 的同款注入）。
//
// # 它是"痕迹推断"的**补强**，不是替代
//
// 侧栏那个列表本身仍然只认 write/edit 的参数（不依赖任何路由，见 deliverables.go）；
// 这里补的是"点开之后能看什么"：有变更记录就画 diff，没有就照旧退回全文预览。
// 两条路各自独立——老宿主没有这三条路由时，插件退化回原来的行为，**不是**报错。
//
// # seq 不许猜
//
// 三条路由都要 `seq` = 那条 `workspace/changes` 事件的序号。插件从当前会话的
// 原始 Transport 帧（`pluginapi.Snapshot.Frames`）里筛 `type == "workspace/changes"`
// 取**最新**一条——自己编一个 seq 只会拿到 404，然后被误当成"这一轮没有变更"。
package dsdeliverables

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/pluginapi"
)

// ChangesFrameType 是"这一轮改了哪些文件"那条 Transport 帧的 type。
//
// 逐字取自宿主的事件名（与官方 `workspace/changes` 同名；宿主侧写入点见
// HOST:cmd/dshd/workspace_changes_recorder.go）。
const ChangesFrameType = "workspace/changes"

// ChangesSource 是三条变更路由的数据面（壳注入）。
//
// 三个方法都**同步返回**：本仓的 UI 是立即模式，插件在绘制里同步问、拿到就画
// （与 `env.Act.FSRead` 同一形状）。壳的实现要给 ctx 设超时——绘制线程会等它。
//
// # 为什么接口里有 Open 而界面还没画按钮
//
// 官方那三条动作（默认程序打开 / `reveal` 定位 / 指定 application）在官方侧是
// **slot 贡献**的控件（OFF:src/client/file-actions.ts:8-25：按钮文案由贡献方给，
// 本包只提供 `actionUrl`/`onAction`）。本包因此没有"打开"这个词的官方文案，
// 凭空画一个按钮就是自造文案——所以 Open 先作为契约面接好、由壳可调，
// 界面等有文案证据的落点（右栏 review 面板）再画。
type ChangesSource interface {
	// Summary 取一条 `workspace/changes` 事件的摘要。
	Summary(ctx context.Context, sessionID string, seq int) (*apiclient.ChangesSummary, error)
	// Diff 比较摘要里第 index 个文件（index 是 **files 数组的下标**，不是路径）。
	Diff(ctx context.Context, sessionID string, seq, index int) (*apiclient.ChangesDiff, error)
	// Open 让宿主桌面打开/定位摘要里第 index 个文件。
	Open(ctx context.Context, sessionID string, seq, index int, action, application string) error
}

var (
	changesMu     sync.RWMutex
	changesSource ChangesSource
)

// SetChangesSource 由壳注入变更数据面（**在 Load 之后**）。
//
// 传 nil = 收回能力：浮层从此退回全文预览并写明"这一轮没有变更记录"
// （老宿主 / 无头测试就是这样），不是报错。
func SetChangesSource(s ChangesSource) {
	changesMu.Lock()
	changesSource = s
	changesMu.Unlock()
}

// currentChangesSource 每帧重新问：壳中途注入/收回都立刻生效（同 dsattachment.currentSink）。
func currentChangesSource() ChangesSource {
	changesMu.RLock()
	defer changesMu.RUnlock()
	return changesSource
}

// ── 官方中文文案（逐字，OFF:src/client/locales.ts）─────────────────────────────
//
// 只抄这里**真的用到**的那几句；抄错一个字就等于自造文案。
const (
	// OFF:locales.ts:35-36  'changes.added' / 'changes.deleted'
	msgChangesAdded   = "+%d"
	msgChangesDeleted = "-%d"
	// OFF:locales.ts:37,43  'changes.binary' / 'changes.oversized'
	msgChangesBinary    = "二进制"
	msgChangesOversized = "过大"
	// OFF:locales.ts:55-64  diff.*
	msgDiffMissing   = "这轮改动的内容已不可用"
	msgDiffError     = "无法读取改动"
	msgDiffBinary    = "二进制文件，无法显示改动"
	msgDiffOversized = "文件过大，无法显示改动"
	msgDiffCreated   = "本轮新建的文件"
	msgDiffDeleted   = "本轮删除的文件"
	msgDiffUnchanged = "两侧内容相同"
	msgDiffCoarse    = "逐行对比超时，按整个文件替换显示"
	// OFF:locales.ts:64  'diff.truncated'：'只显示前 {count} 行'
	msgDiffTruncated = "只显示前 %d 行"
	// OFF:locales.ts:14  'presented.retry'
	msgRetry = "重试"
)

// msgNoChangesRecord 是"宿主明确说这个 seq 没有记录"时浮层里写的那句话。
//
// **这句不是官方文案**（官方只有 `diff.missing`='这轮改动的内容已不可用'）：
// 它是本仓已登记的裁剪的配套口径——非 git 工作目录的会话**没有** changes 记录，
// 宿主回 404 且不伪造空摘要，界面必须把 404 显示成"没有记录这个事实"，
// 不能显示成"没有变更"（那是替宿主断言），也不能画一个空列表。
const msgNoChangesRecord = "这一轮没有变更记录"

// LatestChangesSeq 在原始 Transport 帧里取最新一条 `workspace/changes` 的 seq。
//
// 取**最大** seq 而不是最后一条：`Frames` 已按 seq 升序去重，但这里不依赖
// 调用方的排序——排序一变就静默拿错 seq，那是这个函数最难发现的错法。
func LatestChangesSeq(frames []apiclient.Frame) (int, bool) {
	seq, found := 0, false
	for _, fr := range frames {
		if fr.Type != ChangesFrameType {
			continue
		}
		if !found || fr.Seq > seq {
			seq, found = fr.Seq, true
		}
	}
	return seq, found
}

// NormalizePath 折反斜杠、去首尾空白。
//
// 只做这两件事：**不做大小写折叠、不做 clean**——`..` 与盘符大小写都是"另一个
// 路径"的可能性来源，折叠它们会让我们自信地匹配到错误的文件。
func NormalizePath(p string) string {
	return strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
}

// ChangedIndex 在摘要的 `files` 里找 `clicked` 这个路径，回它在数组里的下标。
//
// # 为什么需要匹配而不是逐字比对
//
// 侧栏列出的路径来自 write/edit 的**工具参数**（可能是相对的 `src/a.go`，
// 也可能是绝对路径），而摘要里的 `path` 是**相对会话工作目录**的（`display`
// 另有一份展示形）——两者不是同一个字符串域。这里按"越来越松"的顺序试，
// 每一步都是**确定性**判据；最后一步（只给文件名）只在**唯一**同名时才认，
// 多义就返回 false —— 宁可退回全文预览，也不能把 A 文件的 diff 画在 B 文件上。
func ChangedIndex(files []apiclient.ChangesFile, clicked, cwd string) (int, bool) {
	want := NormalizePath(clicked)
	if want == "" {
		return 0, false
	}
	for i, f := range files {
		if NormalizePath(f.Path) == want {
			return i, true
		}
	}
	for i, f := range files {
		if d := NormalizePath(f.Display); d != "" && d == want {
			return i, true
		}
	}
	if base := strings.TrimSuffix(NormalizePath(cwd), "/"); base != "" {
		for i, f := range files {
			if p := NormalizePath(f.Path); p != "" && want == base+"/"+p {
				return i, true
			}
		}
	}
	for i, f := range files {
		if p := NormalizePath(f.Path); p != "" && strings.HasSuffix(want, "/"+p) {
			return i, true
		}
	}
	name := path.Base(want)
	match := -1
	for i, f := range files {
		if path.Base(NormalizePath(f.Path)) != name {
			continue
		}
		if match >= 0 {
			return 0, false
		}
		match = i
	}
	if match >= 0 {
		return match, true
	}
	return 0, false
}

// ── 渲染决策（纯函数，能脱离画布单测）──────────────────────────────────────

// diffRowKind 是一行的种类（官方 `DiffRow.kind`，OFF:FileDiff.tsx:18）。
type diffRowKind int

const (
	rowContext diffRowKind = iota
	rowAdd
	rowDel
	// rowHunk 是 `@@ -a,b +c,d @@` 那一行（官方另有 `hunkHeader()`，
	// OFF:FileDiff.tsx:165-167；这里把它也当一行，画的时候少一层分支）。
	rowHunk
)

// diffRow 是浮层里要画的一行。
//
// `OldNo`/`NewNo` 为 0 表示"这一侧没有行号"——官方用 `undefined`
// （OFF:FileDiff.tsx:72-74：新增行只有新侧、删除行只有旧侧）。
type diffRow struct {
	Kind         diffRowKind
	OldNo, NewNo int
	Text         string
}

// hunkHeaderText 是那一行的逐字形状（OFF:FileDiff.tsx:165-167）。
func hunkHeaderText(h apiclient.ChangesHunk) string {
	return "@@ -" + itoa(h.OldStart) + "," + itoa(h.OldLines) +
		" +" + itoa(h.NewStart) + "," + itoa(h.NewLines) + " @@"
}

// hunkRows 给一段 hunk 编号（OFF:FileDiff.tsx:66-77 的 `hunkRows` 逐字等价）：
// 上下文两侧都数，`+` 只数新侧，`-` 只数旧侧，正文是去掉首字符之后的那些字符。
func hunkRows(h apiclient.ChangesHunk) []diffRow {
	rows := []diffRow{{Kind: rowHunk, Text: hunkHeaderText(h)}}
	oldNo, newNo := h.OldStart, h.NewStart
	for _, line := range h.Lines {
		if line == "" {
			// 客户端已经在解码时挡掉空行；这里再兜一次，免得 `line[0]` 越界。
			rows = append(rows, diffRow{Kind: rowContext, OldNo: oldNo, NewNo: newNo})
			oldNo++
			newNo++
			continue
		}
		text := line[1:]
		switch line[0] {
		case '+':
			rows = append(rows, diffRow{Kind: rowAdd, NewNo: newNo, Text: text})
			newNo++
		case '-':
			rows = append(rows, diffRow{Kind: rowDel, OldNo: oldNo, Text: text})
			oldNo++
		default:
			rows = append(rows, diffRow{Kind: rowContext, OldNo: oldNo, NewNo: newNo, Text: text})
			oldNo++
			newNo++
		}
	}
	return rows
}

// maxDiffRows 是浮层里**最多画几行**。
//
// 官方的预算是 5000 行（OFF:FileDiff.tsx:14 的 `MAX_RENDERED_LINES`）——那是
// 浏览器里一个可滚动的 DOM 列表。本仓是立即模式：每一行都是这一帧现画的盒子，
// 5000 行会把帧时间拖垮。所以取 300 行当渲染预算，超出的部分**照实说**
// （`diff.truncated`='只显示前 300 行'），不假装画全了。
const maxDiffRows = 300

// diffRows 把几段 hunk 摊平成一串要画的行，并在预算处截断。
//
// 截断是**按行**的：某段 hunk 被截了一半就停在那里，不跳过它去画后面的
// （跳着画会让行号看起来是连续的，而中间其实缺了一大段）。
func diffRows(hunks []apiclient.ChangesHunk) (rows []diffRow, truncated bool) {
	for _, h := range hunks {
		for _, row := range hunkRows(h) {
			if len(rows) >= maxDiffRows {
				return rows, true
			}
			rows = append(rows, row)
		}
	}
	return rows, false
}

// changesOutcome 是"这个文件的变更记录取到了没有"。
type changesOutcome int

const (
	// outcomeNoRecord = **没有这条记录**：没接数据源、会话里没有
	// `workspace/changes` 帧、宿主回 404、宿主没接线（501）、或摘要里没有这个文件。
	// 五种情况对用户是同一件事（没有记录可看），所以合成一档；但**都必须**
	// 退回全文预览并写明"没有记录"，不许画空 diff。
	outcomeNoRecord changesOutcome = iota
	// outcomeReady 拿到了摘要和比较。
	outcomeReady
	// outcomeDiffMissing 有摘要，但该文件的比较已被宿主丢弃（diff 404）。
	outcomeDiffMissing
	// outcomeFailed 读失败（传输错、500、400…）：与"没有记录"不同，可以重试。
	outcomeFailed
)

// changesView 是一次解析的结果（纯数据，测试直接构造）。
type changesView struct {
	Outcome changesOutcome
	File    *apiclient.ChangesFile
	Diff    *apiclient.ChangesDiff
	Err     error
}

// overlayPlan 是浮层这一帧要画什么（纯决策，不碰画布）。
type overlayPlan struct {
	// Path 是头部显示的路径。
	Path string
	// ShowCounts 为真时画 `+Added -Deleted` 两个计数。
	ShowCounts bool
	Added      int
	Deleted    int
	// CountsLabel 是替换计数的状态词（"二进制"/"过大"）：非空时**只**画它。
	// 官方 Counts 就是这个判据（OFF:ChangedFiles.tsx:23-28、ReviewTab.tsx:54-61）。
	CountsLabel string
	// Notes 是正文上方的说明行（官方那几句，逐字）。
	Notes []string
	// Rows 是要画的 diff 行（含 hunk 头）。
	Rows []diffRow
	// FullText 为真时在说明行下面画**全文预览**（兜底路径）。
	FullText bool
	// Retry 为真时画一颗「重试」。
	Retry bool
}

// planOverlay 把一次解析结果翻成"画什么"。
//
// 所有分支都只有两种结局：**能画出真的 diff**，或者**退回全文预览并写明原因**。
// 没有第三种（画一个空 diff 框）——那会让人以为"这个文件没改"。
func planOverlay(v changesView, fallbackPath string) overlayPlan {
	plan := overlayPlan{Path: fallbackPath}
	switch v.Outcome {
	case outcomeNoRecord:
		plan.Notes = []string{msgNoChangesRecord}
		plan.FullText = true
		return plan
	case outcomeFailed:
		plan.Notes = []string{msgDiffError}
		plan.Retry = true
		plan.FullText = true
		return plan
	case outcomeDiffMissing:
		plan.Notes = []string{msgDiffMissing}
		plan.FullText = true
		return plan
	}
	if v.File != nil {
		plan.Path = v.File.Display
		if plan.Path == "" {
			plan.Path = v.File.Path
		}
		switch {
		case v.File.Binary:
			plan.CountsLabel = msgChangesBinary
		case v.File.Oversized:
			plan.CountsLabel = msgChangesOversized
		default:
			plan.ShowCounts = true
			plan.Added, plan.Deleted = v.File.Added, v.File.Deleted
		}
	}
	d := v.Diff
	if d == nil {
		// 不该发生（outcomeReady 一定有 diff）：真发生了就照"读不到"处理，
		// 不赌一个空正文。
		plan.Notes = []string{msgDiffError}
		plan.Retry = true
		plan.FullText = true
		return plan
	}
	switch d.Kind {
	case "binary":
		plan.Notes = []string{msgDiffBinary}
		return plan
	case "oversized":
		plan.Notes = []string{msgDiffOversized}
		return plan
	}
	// 官方 `noteOf`（OFF:FileDiff.tsx:128-133）：两侧存在性先说，
	// 都没有差异时才说"内容相同"。
	switch {
	case !d.Before:
		plan.Notes = append(plan.Notes, msgDiffCreated)
	case !d.After:
		plan.Notes = append(plan.Notes, msgDiffDeleted)
	case len(d.Hunks) == 0:
		plan.Notes = append(plan.Notes, msgDiffUnchanged)
	}
	if d.Coarse {
		// 本仓恒 false（宿主用 git 的统一 diff，没有"JS 行比较超时"这个事件），
		// 但契约里有这一位，就照官方画出来——不假装它不存在。
		plan.Notes = append(plan.Notes, msgDiffCoarse)
	}
	rows, truncated := diffRows(d.Hunks)
	plan.Rows = rows
	if truncated {
		plan.Notes = append(plan.Notes, fmt.Sprintf(msgDiffTruncated, maxDiffRows))
	}
	return plan
}

// changesKey 是缓存的键：**(会话, seq, 路径)** 三者任一变了就得重拉。
//
// 为什么不只按路径：同一个路径在不同会话/不同轮次是完全不同的比较结果，
// 只按路径缓存会把上一轮的 diff 画在这一轮上。
type changesKey struct {
	session string
	seq     int
	path    string
}

// resolveChanges 取"浮层当前这个文件"的变更视图（每帧调用，命中缓存就不发请求）。
func (p *Plugin) resolveChanges(env *pluginapi.Env) changesView {
	noRecord := changesView{Outcome: outcomeNoRecord}
	if env == nil || env.Data.Cur == "" {
		// 还不知道是哪个会话：**不猜**（猜错就是把别人的 diff 画上来）。
		return noRecord
	}
	seq, ok := LatestChangesSeq(env.Data.Frames)
	if !ok {
		// 这一轮没有 `workspace/changes` 帧（非 git 会话就是这样）：没有记录。
		return noRecord
	}
	source := currentChangesSource()
	if source == nil {
		// 不缓存这一档：壳可能在 Load 之后才注入，下一帧就该恢复。
		return noRecord
	}
	key := changesKey{session: env.Data.Cur, seq: seq, path: p.preview}
	if p.changesValid && p.changesKey == key {
		return p.changes
	}
	v := fetchChanges(source, key)
	p.changesKey, p.changes, p.changesValid = key, v, true
	return v
}

// invalidateChanges 丢掉缓存：下一帧的 resolveChanges 会重拉（「重试」用）。
func (p *Plugin) invalidateChanges() { p.changesValid = false }

// changesTimeout 是这两次同步请求的上限。
//
// 浮层是在**绘制线程**上同步问的（与 `FSRead` 同一形状），所以必须给它一个上界：
// 宿主没响应时最多卡这么久，而不是等底层 HTTP 客户端自己的默认超时。
// 取 15s 与 `dsdirpicker.reqTimeout` 一致（同一个形状的同步数据面）。
const changesTimeout = 15 * time.Second

// fetchChanges 同步拉一次摘要 + 比较。
func fetchChanges(source ChangesSource, key changesKey) changesView {
	ctx, cancel := context.WithTimeout(context.Background(), changesTimeout)
	defer cancel()
	summary, err := source.Summary(ctx, key.session, key.seq)
	if err != nil {
		if apiclient.IsChangesMissing(err) || changesNotWired(err) {
			return changesView{Outcome: outcomeNoRecord}
		}
		return changesView{Outcome: outcomeFailed, Err: err}
	}
	index, ok := ChangedIndex(summary.Files, key.path, summary.Cwd)
	if !ok {
		// 摘要里没有这个文件：对这个文件而言就是"没有记录"（它可能是本轮之前
		// 写的，也可能只是被别的工具改的）。退回全文预览，不画空 diff。
		return changesView{Outcome: outcomeNoRecord}
	}
	diff, err := source.Diff(ctx, key.session, key.seq, index)
	if err != nil {
		if apiclient.IsChangesMissing(err) {
			return changesView{Outcome: outcomeDiffMissing}
		}
		return changesView{Outcome: outcomeFailed, Err: err}
	}
	file := summary.Files[index]
	return changesView{Outcome: outcomeReady, File: &file, Diff: diff}
}

// changesNotWired 报告 501（宿主没接线这三条路由）。
//
// 对用户它和"没有记录源"是同一件事：这台宿主给不出变更记录。
func changesNotWired(err error) bool {
	var ce *apiclient.ChangesError
	return errors.As(err, &ce) && ce.Kind == apiclient.ChangesStatusNotWired
}
