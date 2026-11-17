// Package dsjobs 在侧栏底部列出**后台任务**（官方 `dsh-client-ui-jobs` 的一小块），
// 并把它从"只能看"补成**能读输出、能停**。
//
// # 官方对应物与这一版的边界（逐条说清，不当成"与官方同构"）
//
// 官方包注册的是 `conversation.session.header.actions` 的 `job-list`（order 20）
// 触发器 + 一个 500px 弹层（`OFF:dsh-client-ui-jobs/src/client/JobListAction.tsx:1-8`
// 与 `index.ts:29-39`）。本仓这一版仍然画在**侧栏底部**（`sidebar.footer`，
// 本仓自有槽，`plugin` 归属登记为 derived）：
//
//   - 触发器/弹层几何（28 高、14 图标、500 宽、480 高上限、20×20 停止钮）
//     逐值取官方 CSS（`JobListAction.module.css`），但**容器位置不同**；
//   - 官方行有 `kind`/`progress`/`startedAt`（→ 持续时长），本仓宿主 `jobs.list`
//     只给 id/sessionId/command/done/exit_code/bytes——**没有时长就不画时长**，
//     不拿"收到清单的时刻"冒充开始时刻；
//   - 官方两处状态 `stopping`/`killed` 依赖 jobs 控制流（`JobView.status`），
//     本仓只有 done/exit_code，所以只画 运行中 / 已完成 / 已失败 三态，
//     点击停止后不乐观改成"正在停止"（真正的结果以清单刷新为准）。
//
// # 输出与停止是**真实动作**（这是本文件的核心）
//
// 宿主两条方法早就有了（`HOST:internal/transport/api.go:42-45` canonical 25 的
// `jobs.output` + additive `jobs.stop`；生产接线 `HOST:cmd/dshd/serve.go:658-668`），
// 而界面一直没有调用者——审计 `ui-face.md` #22 的"无输出分页、无停止按钮"就是这个。
// 现在两处都接上：
//
//   - 输出按宿主 rune 游标分页（`cursor/nextCursor/hasMore`），"加载更多"是真的再要一页；
//   - `truncated`（1 MiB 保留窗口之前被淘汰）单独一行说明，不折叠成"没有输出"；
//   - 读失败（轮次结束、owner 不在、跨会话被拒）画成错误行 + 重试，不吞；
//   - 停止是**两段式**（首次点击进入确认，3 秒内再点才发请求），
//     与官方 `kill.confirm`/`kill.confirmAction` 语义一致。
//
// 停止失败的行内红色态**没做**：Actions.JobsStop 是 fire-and-forget（壳把失败写
// 状态栏），没有结果回来就不画"停止失败"——那会是编一条没发生的事实。
package dsjobs

import (
	"strings"
	"sync"
	"time"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/internal/uikernel"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// maxShown 最多列几条。侧栏底部那块地方有限，超出的写「还有 N 条」。
const maxShown = 4

// 官方几何（`ui-jobs/src/client/JobListAction.module.css`，逐值）。
const (
	jobRowGap           = 6    // .row{gap:6px}（:311-327）
	jobDotSize          = 10   // StateDot 非 ongoing 外框 10×10（primitives dist index-ghOpC9uV.js:129）
	jobDotCoreSize      = 6    // StateDot::after inset:20% → 6×6（index-CSAdOBx6.css:1）
	jobDotOngoingSize   = 14   // StateDot ongoing 默认 14×14（index-ghOpC9uV.js:129）
	jobStopBox          = 20   // .stop{width:20px;height:20px}（:250-264）
	jobStopBorder       = 0.5  // .stop{border:0.5px solid border-l2}（:259）
	jobStopArmPadH      = 7    // .stopArmed{padding:0 7px}（:280-288）
	jobStopArmGap       = 4    // .stopArmed{gap:4px}（:282）
	jobStopArmBorderMix = 0.45 // border-color:color-mix(error 45%,transparent)（:285）
	jobStopArmBgMix     = 0.12 // background:color-mix(error 12%,transparent)（:286）
	jobStopFont         = 11   // .stopLabel{font-size:11px;line-height:18px}（:301-305）
	jobStopLine         = 18
	jobOutputMaxH       = 288 // .panel --dsl-terminal-output-max-height:288px（:430-438）
	jobOutputIndent     = 12  // .panel [data-terminal]{--dsl-terminal-gutter:12px}（:444-446）
	jobOutputPadH       = 8   // .panel{margin:0 8px 8px}（:430-431）
	jobOutputPadBot     = 8
	jobNoticeMarginB    = 6 // .notice{margin:0 8px 6px}（:448-453）
	jobNoticeFont       = 11
	jobNoticeLine       = 16
	jobKillArmWindow    = 3000 // KILL_ARM_MS = 3_000（JobListAction.tsx:54-55）
	jobCopiedWindow     = 1200 // "已复制"停留时长（本地反馈，非官方几何）
)

// 官方文案（zh 字典逐字；`ui-jobs/src/client/locales.ts`）。
const (
	zhJobsTitle        = "后台任务" // 官方没有这个标题（触发器是图标+计数），侧栏形态需要一段标题
	zhStatusRunning    = "运行中"
	zhStatusCompleted  = "已完成"
	zhStatusFailed     = "已失败"
	zhOutputGap        = "……较早的输出已丢弃……"
	zhOutputError      = "实时输出流中断："
	zhOutputEmpty      = "（无输出）"
	zhOutputLoading    = "正在读取输出…"
	zhOutputMore       = "加载更多"
	zhOutputCopy       = "复制"
	zhOutputCopied     = "已复制"
	zhOutputRetry      = "重试"
	zhStopLabel        = "停止"
	zhStopConfirm      = "确认停止"
	zhStopTitle        = "停止任务"
	zhOutputNoteFailed = "读取失败" // 错误行的前缀（官方只有 output.error 模板：`实时输出流中断：{error}`）
)

type Plugin struct {
	theme *pluginapi.Theme
	prims pluginapi.Primitives

	// mu 保护下面这些**既被绘制读、又被按键/定时器写**的状态。
	// 绘制与 HandleKey 可能来自不同线程（见 dssettings 的同款注释）。
	mu sync.Mutex
	// expanded 是当前展开输出的任务键（jobKey）；空 = 没有展开。
	expanded string
	// cursor 是每个任务"下一次要请求的游标"（0 = 从头读）。
	cursor map[string]int
	// out 是每个任务最近一次**成功读到**的累计视图：拉下一页时这一帧可能
	// 拿不到新数据（壳在后台拉），但界面不能因此把已显示的内容抹掉。
	out map[string]pluginapi.JobOutput
	// armed 是已进入"确认停止"态的任务键；armedUntil 是自动解除时刻。
	armed      string
	armedUntil time.Time
	// copied/copiedUntil 控制"复制 → 已复制"的短暂反馈。
	copied      string
	copiedUntil time.Time

	// tier 由壳的 AttachShellTier 注入：官方子槽只在官方骨架且没走兼容桥时
	// 才被 dispatch（见 official_jobs.go 顶部）。
	tier *uikernel.ShellTier
	// open 是官方弹层是否打开；openableCount 是上一帧的可见任务数
	// （HandleKey 没有 env，只能记住上一帧——与 dscomposer 同一做法）。
	open          bool
	openableCount int
	// settledOpen 是"已结束段"的显式折叠状态；nil = 按官方默认（有活任务时折叠）。
	settledOpen *bool
	// cleared 是被用户「清空」的已结束行（客户端隐藏；注册表记录仍在）。
	cleared map[string]bool
}

func (*Plugin) ID() string { return "ds-jobs" }

func (*Plugin) Provides() []string { return nil }

func (*Plugin) Requires() []string {
	return []string{pluginapi.KeyTheme, pluginapi.KeyPrimitives}
}

func (p *Plugin) Register(ctx pluginapi.Context) error {
	p.theme = ctx.Inject(pluginapi.KeyTheme).(*pluginapi.Theme)
	p.prims = ctx.Inject(pluginapi.KeyPrimitives).(pluginapi.Primitives)
	return nil
}

func (p *Plugin) Panels() []pluginapi.Painter {
	return []pluginapi.Painter{
		// 官方档：`conversation.session.header.actions` 的 job-list（order 20，
		// 与 ui-agent-preset 的 order -10、ui-subagent 的 10 同一条带）。
		{ID: "jobs-trigger", Slot: SlotSessionHeaderActions, Order: 20, Draw: p.drawTrigger},
		// 官方档：全窗遮罩 + 500px 弹层（官方 createPortal 的等价物）。
		{ID: "jobs-panel", Slot: pluginapi.SlotShellOverlay, Order: 45,
			Draw: p.drawPanelBackdrop, Children: p.drawPanel},
		// 旧三栏档：侧栏底部清单（derived；官方子槽在这档不会被 dispatch）。
		{ID: "jobs-list", Slot: pluginapi.SlotSidebarFooter, Order: 70,
			Draw: p.drawLegacyList},
	}
}

// drawLegacyList 只在官方子槽不生效的档位画（否则会与 header 触发器重复）。
func (p *Plugin) drawLegacyList(f ui.C, env *pluginapi.Env) {
	if p.officialSlotsLive() {
		return
	}
	p.draw(f, env)
}

// jobKey 是任务在插件状态里的键：同一 id 在不同会话里是两条任务。
func jobKey(j pluginapi.Job) string { return j.SessionID + "\x00" + j.ID }

// live 报告宿主清单里这条任务是否还在跑。
//
// **它只是"上一次推送时还在跑"**：宿主那份清单来自子进程的 `@@JOBS@@` 快照
// （`plugins/dsjobs` 原有注释与 `HOST:cmd/dshd/serve_approval.go:184-193`），
// 所以它不是实时状态——本文件不据此画"已停止"之类的断言。
func live(j pluginapi.Job) bool { return !j.Done }

// statusLabel 是这一行的状态文字。官方五态（running/stopping/completed/killed/failed）
// 里有三态需要 jobs 控制流；本仓只有 done + exit_code，所以只画三态。
func statusLabel(j pluginapi.Job) string {
	switch {
	case !j.Done:
		return zhStatusRunning
	case j.ExitCode != 0:
		return zhStatusFailed
	default:
		return zhStatusCompleted
	}
}

// statusColor 是官方 `.status` 的固定颜色；失败由 StateDot(error) 表达，不把文字改红。
func statusColor(t *pluginapi.Theme, _ pluginapi.Job) ui.Color {
	return t.LabelTertiary
}

// dotColor 是官方 StateDot 的三态颜色（JobListAction.tsx:88-98 → primitives 令牌）。
// ongoing=label-tertiary、done=state-success-primary、error=state-error-primary。
func dotColor(t *pluginapi.Theme, j pluginapi.Job) ui.Color {
	switch {
	case !j.Done:
		return t.LabelTertiary
	case j.ExitCode != 0:
		return t.Error
	default:
		return t.SuccessPrimary
	}
}

// drawJobStateDot 画官方 StateDot：ongoing 是 14px spinner，其余是 10px
// 外框里 inset:20% 的 6px 实心圆（primitives dist，见 icons.go 的素材注释）。
func drawJobStateDot(f ui.C, t *pluginapi.Theme, j pluginapi.Job) {
	if live(j) {
		ui.Icon(f, jobStateDotOngoingIcon(t.LabelTertiary)).
			Size(jobDotOngoingSize, jobDotOngoingSize).Color(t.LabelTertiary)
		return
	}
	ui.Row(f).Size(jobDotSize, jobDotSize).Center().Children(func() {
		ui.Slot(f).Size(jobDotCoreSize, jobDotCoreSize).
			Radius(jobDotCoreSize / 2).Background(dotColor(t, j))
	})
}

// computeJobs 选出这一帧要画的行，并把"已经消失的任务"的状态清掉。
func (p *Plugin) computeJobs(jobs []pluginapi.Job) (shown []pluginapi.Job, more int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	shown = jobs
	if len(shown) > maxShown {
		more = len(shown) - maxShown
		shown = shown[:maxShown]
	}
	// 任务离开清单：展开态、游标、缓存、确认态全部作废。
	live := make(map[string]bool, len(jobs))
	for _, j := range jobs {
		live[jobKey(j)] = true
	}
	if p.expanded != "" && !live[p.expanded] {
		p.expanded = ""
	}
	if p.armed != "" && !live[p.armed] {
		p.armed = ""
	}
	if p.copied != "" && !live[p.copied] {
		p.copied = ""
	}
	for key := range p.cursor {
		if !live[key] {
			delete(p.cursor, key)
		}
	}
	for key := range p.out {
		if !live[key] {
			delete(p.out, key)
		}
	}
	return shown, more
}

// draw 画「后台任务」区。**没有任务就整块不画**——
// 空标题会让侧栏看着像坏了（"后台任务"下面什么都没有）。
func (p *Plugin) draw(f ui.C, env *pluginapi.Env) {
	if env.Act.JobsList == nil {
		return
	}
	jobs := env.Act.JobsList()
	if len(jobs) == 0 {
		return
	}
	shown, more := p.computeJobs(jobs)

	ui.Column(f).FillWidth().Gap(2).PadY(4).Children(func() {
		ui.Text(f, zhJobsTitle).FontSize(p.theme.Fonts.Caption).Color(p.theme.LabelTertiary)
		for _, j := range shown {
			p.drawRow(f, env, j)
			if p.isExpanded(j) {
				p.drawOutput(f, env, j)
			}
		}
		if more > 0 {
			ui.Text(f, "还有 "+itoa(more)+" 条").FontSize(p.theme.Fonts.Caption).Color(p.theme.LabelTertiary)
		}
	})
}

// drawRow 画一行：状态点 + 命令 + 状态 + （展开箭头 / 停止按钮）。
//
// 行与停止钮是**兄弟**而不是父子：命中区互不遮盖，点"停止"不会同时展开输出。
func (p *Plugin) drawRow(f ui.C, env *pluginapi.Env, j pluginapi.Job) {
	t := p.theme
	key := jobKey(j)
	clickable := env.Act.JobsOutput != nil && j.SessionID != ""
	id := env.Tree.ID(env.Feat, "job-row-"+key)
	ui.Row(f).FillWidth().Items(ui.Center).Gap(jobRowGap).Children(func() {
		row := ui.Row(f).Items(ui.Center).Gap(jobRowGap).Grow(1).MinWidth(0).
			MinHeight(jobsRowMinHeight).
			PadSides(jobsRowPadV, jobsRowPadH, jobsRowPadV, jobsRowPadH).
			Radius(t.Radii.MD).Hit(id, shortCmd(j.Command)).Children(func() {
			drawJobStateDot(f, t, j)
			ui.Text(f, shortCmd(j.Command)).FontSize(jobsRowFont).LineHeight(jobsRowLine).
				Color(t.LabelSecondary).Grow(1).Ellipsis()
			ui.Text(f, statusLabel(j)).FontSize(10).LineHeight(16).Color(statusColor(t, j))
			// 已结束的行给字节数（旧侧栏的 derived 信息；官方弹层不拿它冒充时长）。
			if j.Done {
				ui.Text(f, sizeLabel(j.Bytes)).FontSize(jobNoticeFont).LineHeight(jobNoticeLine).
					Color(t.LabelTertiary)
			}
			// 展开箭头复用官方 chevron sprite，不再用文本三角形。
			if clickable {
				ui.Icon(f, jobRowChevronIcon(p.isExpanded(j))).
					Size(jobsChevronIcon, jobsChevronIcon).Color(t.LabelTertiary)
			}
		})
		if clickable && row.Clicked() {
			p.toggleExpanded(key)
		}
		if live(j) && env.Act.JobsStop != nil && j.SessionID != "" {
			p.drawStop(f, env, j)
		}
	})
}

// drawStop 画两段式停止钮：第一次点击进入"确认停止"，3 秒内再点才真的发请求。
//
// 官方 `pressKill`：`armed → pending → 等行状态翻掉`；本仓没有 pending 态
// （Actions.JobsStop 不回结果），所以第二击之后直接回到普通态，真正的"停了"看清单。
func (p *Plugin) drawStop(f ui.C, env *pluginapi.Env, j pluginapi.Job) {
	t := p.theme
	key := jobKey(j)
	armed := p.armedFor(key)
	id := env.Tree.ID(env.Feat, "job-stop-"+key)
	label := zhStopLabel
	border := t.BorderL2
	if armed {
		label = zhStopConfirm
		border = jobErrorMix(t.Error, jobStopArmBorderMix)
	}
	box := ui.Row(f).MinHeight(jobStopBox).Items(ui.Center).Gap(jobStopArmGap).
		Radius(t.Radii.SM).Border(jobStopBorder, border).
		PadSides(0, jobStopArmPadH, 0, jobStopArmPadH).
		Hit(id, zhStopTitle+" "+shortCmd(j.Command)).Children(func() {
		ui.Text(f, label).FontSize(jobStopFont).LineHeight(jobStopLine).Color(stopColor(t, armed))
	})
	if armed {
		box.Background(jobErrorMix(t.Error, jobStopArmBgMix))
	}
	if box.Clicked() {
		if p.takeArmed(key) {
			env.Act.JobsStop(j.SessionID, j.ID)
			return
		}
		p.arm(key, env.Act.Invalidate)
	}
}

// stopColor 是停止钮文字色：未确认时次要色，确认态用错误色（官方 .stopArmed）。
func stopColor(t *pluginapi.Theme, armed bool) ui.Color {
	if armed {
		return t.Error
	}
	return t.LabelSecondary
}

// jobErrorMix 等价于官方 color-mix(in srgb, error N%, transparent)。
func jobErrorMix(color ui.Color, alpha float32) ui.Color {
	color.Alpha *= alpha
	return color
}

// drawOutput 画展开后的输出块。
//
// 每帧都会尝试从壳缓存取一次（同步、不会卡绘制）：壳没就绪时返回 ok=false，
// 这时如果之前已经读到过内容就继续画旧内容（只在尾部加一行"正在读取"），
// **不把已经显示的输出抹掉**。
func (p *Plugin) drawOutput(f ui.C, env *pluginapi.Env, j pluginapi.Job) {
	p.drawOutputBlock(f, env, j, jobOutputMaxH, jobOutputIndent)
}

// drawOutputBlock 是输出块的实体（两种容器都使用官方 288px 上限与 12px gutter）。
func (p *Plugin) drawOutputBlock(f ui.C, env *pluginapi.Env, j pluginapi.Job, maxH, gutter float32) {
	t := p.theme
	key := jobKey(j)
	if env.Act.JobsOutput == nil || j.SessionID == "" {
		return
	}
	cursor := p.cursorFor(key)
	if view, ok := env.Act.JobsOutput(j.SessionID, j.ID, cursor); ok {
		p.storeOutput(key, view)
	}
	view, has := p.storedOutput(key)

	ui.Column(f).FillWidth().Gap(0).
		PadSides(0, jobOutputPadH, jobOutputPadBot, gutter).Children(func() {
		switch {
		case has && view.Err != "":
			ui.Wrap(f, zhOutputError+view.Err).FillWidth().FontSize(jobNoticeFont).
				LineHeight(jobNoticeLine).Color(t.Error).MarginSides(0, 0, jobNoticeMarginB, 0)
			p.drawInlineButton(f, env, key+"-retry", zhOutputRetry, func() {
				p.resetOutput(key)
			})
		case !has:
			ui.Text(f, zhOutputLoading).FontSize(jobNoticeFont).LineHeight(jobNoticeLine).
				Color(t.LabelTertiary)
		default:
			if view.Truncated {
				ui.Wrap(f, zhOutputGap).FillWidth().FontSize(jobNoticeFont).
					LineHeight(jobNoticeLine).Color(t.LabelTertiary).
					MarginSides(0, 0, jobNoticeMarginB, 0)
			}
			body := view.Output
			if strings.TrimSpace(body) == "" {
				ui.Text(f, zhOutputEmpty).FontSize(jobNoticeFont).LineHeight(jobNoticeLine).
					Color(t.LabelTertiary)
			} else {
				// 输出区自己滚：官方 `--dsl-terminal-output-max-height:288px`。
				ui.Scroll(f).FillWidth().MaxHeight(maxH).Clip().Children(func() {
					ui.Wrap(f, body).FillWidth().Mono().
						FontSize(t.Fonts.Code).LineHeight(t.Fonts.CodeLine).
						Color(t.LabelSecondary)
				})
			}
			ui.Row(f).FillWidth().Items(ui.Center).Gap(8).Children(func() {
				if strings.TrimSpace(body) != "" {
					label := zhOutputCopy
					if p.copiedFor(key) {
						label = zhOutputCopied
					}
					p.drawInlineButton(f, env, key+"-copy", label, func() {
						env.Act.Copy(body)
						p.markCopied(key, env.Act.Invalidate)
					})
				}
				if view.HasMore {
					p.drawInlineButton(f, env, key+"-more", zhOutputMore, func() {
						p.setCursor(key, view.NextCursor)
					})
				}
			})
		}
	})
}

// drawInlineButton 复用官方 sectionToggle/sectionClear 的 11/16、2×4 小文字钮几何。
func (p *Plugin) drawInlineButton(f ui.C, env *pluginapi.Env, suffix, label string, onClick func()) {
	t := p.theme
	id := env.Tree.ID(env.Feat, "job-btn-"+suffix)
	box := ui.Row(f).Items(ui.Center).PadSides(jobsTogglePadV, jobsTogglePadH, jobsTogglePadV, jobsTogglePadH).
		Radius(t.Radii.SM).Hit(id, label).Children(func() {
		ui.Text(f, label).FontSize(jobNoticeFont).LineHeight(jobNoticeLine).Color(t.LabelTertiary)
	})
	if box.Clicked() && onClick != nil {
		onClick()
	}
}

// HandleKey 实现 pluginapi.KeyHandler：展开着输出时按 Escape 收起。
//
// 返回 true 表示按键被吃掉——**只在真的收了东西时**返回 true，
// 别把 Escape 从别的控件那里抢走。
func (p *Plugin) HandleKey(ev gfx.KeyEvent) bool {
	// 官方弹层的手势优先：ArrowDown 打开、Escape 关闭（JobListAction.tsx:521-546）。
	if p.handleOfficialKey(ev) {
		return true
	}
	if ev.Key != gfx.KeyEscape {
		return false
	}
	p.mu.Lock()
	expanded := p.expanded
	p.expanded = ""
	p.armed = ""
	p.mu.Unlock()
	return expanded != ""
}

// ── 状态访问（都走锁）────────────────────────────────────────────────

func (p *Plugin) isExpanded(j pluginapi.Job) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.expanded == jobKey(j)
}

func (p *Plugin) toggleExpanded(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.expanded == key {
		p.expanded = ""
		return
	}
	p.expanded = key
	// 重新展开 = 从头读（宿主那边的输出可能又长了）。
	if p.cursor == nil {
		p.cursor = map[string]int{}
	}
	p.cursor[key] = 0
	if p.out != nil {
		delete(p.out, key)
	}
}

func (p *Plugin) cursorFor(key string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cursor[key]
}

func (p *Plugin) setCursor(key string, cursor int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cursor == nil {
		p.cursor = map[string]int{}
	}
	p.cursor[key] = cursor
}

func (p *Plugin) storeOutput(key string, view pluginapi.JobOutput) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.out == nil {
		p.out = map[string]pluginapi.JobOutput{}
	}
	p.out[key] = view
}

func (p *Plugin) storedOutput(key string) (pluginapi.JobOutput, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.out[key]
	return v, ok
}

func (p *Plugin) resetOutput(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.out, key)
	if p.cursor == nil {
		p.cursor = map[string]int{}
	}
	p.cursor[key] = 0
}

func (p *Plugin) armedFor(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.armed != key {
		return false
	}
	if time.Now().After(p.armedUntil) {
		p.armed = ""
		return false
	}
	return true
}

// takeArmed 消费一次确认：只有仍在窗口内才算数（超时视为没按过）。
func (p *Plugin) takeArmed(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.armed != key || time.Now().After(p.armedUntil) {
		p.armed = ""
		return false
	}
	p.armed = ""
	return true
}

// arm 进入确认态，并安排一次重绘让"确认停止"到点自动退回。
func (p *Plugin) arm(key string, invalidate func()) {
	p.mu.Lock()
	p.armed = key
	p.armedUntil = time.Now().Add(jobKillArmWindow * time.Millisecond)
	p.mu.Unlock()
	if invalidate != nil {
		time.AfterFunc(jobKillArmWindow*time.Millisecond, invalidate)
	}
}

func (p *Plugin) copiedFor(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.copied != key {
		return false
	}
	if time.Now().After(p.copiedUntil) {
		p.copied = ""
		return false
	}
	return true
}

func (p *Plugin) markCopied(key string, invalidate func()) {
	p.mu.Lock()
	p.copied = key
	p.copiedUntil = time.Now().Add(jobCopiedWindow * time.Millisecond)
	p.mu.Unlock()
	if invalidate != nil {
		time.AfterFunc(jobCopiedWindow*time.Millisecond, invalidate)
	}
}

// ── 纯函数 ──────────────────────────────────────────────────────────

// shortCmd 把命令压成一行：取第一个 token，太长就截断。
//
// **不做 shell 解析**：`echo a; rm -rf b` 这种取第一个 token 就够表示"这条在干嘛"，
// 真解析 shell 语法会出错，而且这里只是给人看的标签。
func shortCmd(cmd string) string {
	c := strings.TrimSpace(cmd)
	if i := strings.IndexAny(c, " \t\n"); i > 0 {
		c = c[:i]
	}
	if len(c) > 24 {
		c = c[:24] + "…"
	}
	if c == "" {
		return "(空命令)"
	}
	return c
}

// sizeLabel 把字节数变成 `12 B` / `3.4 KB` / `1.2 MB`。
func sizeLabel(n int) string {
	switch {
	case n < 1024:
		return itoa(n) + " B"
	case n < 1024*1024:
		return itoa(n/1024) + " KB"
	default:
		return itoa(n/1024/1024) + " MB"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
