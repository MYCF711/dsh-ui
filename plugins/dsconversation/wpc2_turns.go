package dsconversation

import (
	"strconv"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/ui"
)

// loadedTurn 只描述当前快照里已经加载、且带真实 TurnID/turn 的连续消息段。
// 没有身份的相邻消息保持平铺；不会按 assistant 数或数组下编号造 Turn。
type loadedTurn struct {
	key      string
	identity string
	label    string
	id       string
	number   *int64
	start    int
	end      int
}

func turnIdentity(msg apiclient.Message) (string, string, string, *int64) {
	if msg.TurnID != "" {
		return "id\x00" + msg.TurnID, "Turn · " + msg.TurnID, msg.TurnID, nil
	}
	if msg.Turn != nil {
		n := *msg.Turn
		return "number\x00" + strconv.FormatInt(n, 10), "Turn #" + strconv.FormatInt(n, 10), "", &n
	}
	return "", "", "", nil
}

func buildLoadedTurns(data pluginapi.Snapshot, visible []int) []loadedTurn {
	var turns []loadedTurn
	for _, index := range visible {
		if index < 0 || index >= len(data.Msgs) {
			continue
		}
		identity, label, id, number := turnIdentity(data.Msgs[index])
		if identity == "" {
			continue
		}
		if len(turns) > 0 {
			last := &turns[len(turns)-1]
			if last.identity == identity && last.end == index-1 {
				last.end = index
				continue
			}
		}
		turns = append(turns, loadedTurn{
			key: identity + "@" + strconv.Itoa(index), identity: identity,
			label: label, id: id, number: number, start: index, end: index,
		})
	}
	return turns
}

func sameLoadedTurn(group loadedTurn, id string, number *int64) bool {
	if id != "" && group.id != "" {
		return id == group.id
	}
	if number != nil && group.number != nil {
		return *number == *group.number
	}
	return false
}

// jumpTo 记下这次跳转，等本帧稍后滚动区声明出来再落位。
//
// 为什么排队：Turn 导航声明在滚动区**之前**（它要排在消息上方），而 v0.6.0 的
// ScrollState 只能从声明期的 Box 上取——没有按 ID 查表的入口了。
func (p *Plugin) jumpTo(offset float32) {
	p.scrollJump, p.jumpReady = offset, true
}

// turnOffset 用**上一帧布局后**的滚动偏移把 Turn 锚点换算成内容偏移。
//
// v0.6.0 没有按 ID 查滚动状态的入口，偏移由 afterDraw 记在 p.scrollOffset 上；
// 与旧 env.Scroll(id) 拿到的是同一份值（上一帧 settled 的 Offset），时序不变。
func (p *Plugin) turnOffset(group loadedTurn) (float32, bool) {
	box, ok := p.turnAnchors[group.key]
	if !ok || p.bodyRect.Height <= 0 {
		return 0, false
	}
	// Box 的屏幕坐标来自上一帧完整布局；换算回内容偏移后，虚拟化之外
	// 的已加载 Turn 也能定位，不需要宿主提供未加载大纲。
	target := p.scrollOffset + box.Y - p.bodyRect.Y
	if target < 0 {
		target = 0
	}
	return target, true
}

func (p *Plugin) drawTurnNavigation(f ui.C, turns []loadedTurn) {
	if len(turns) == 0 {
		return
	}
	t := p.theme
	current := 0
	if p.scrollAtEnd {
		// 回底是明确的用户意图：最新已加载 Turn 就是当前项，不能
		// 因本帧尚未完成锚点布局而留在上一轮。
		current = len(turns) - 1
	} else {
		for i, group := range turns {
			if target, ok := p.turnOffset(group); ok && target <= p.scrollOffset+8 {
				current = i
			}
		}
	}
	ui.Row(f).WidthPercent(100).Gap(8).Items(ui.Center).PadY(2).Children(func() {
		ui.Text(f, "已加载 "+strconv.Itoa(len(turns))+" 个 Turn").FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		if current > 0 {
			target := turns[current-1]
			if ui.Button(f, "← "+target.label).Ghost().Small().Clicked() {
				if offset, ok := p.turnOffset(target); ok {
					p.jumpTo(offset)
				}
			}
		}
		ui.Text(f, turns[current].label).FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
		if current+1 < len(turns) {
			target := turns[current+1]
			if ui.Button(f, target.label+" →").Ghost().Small().Clicked() {
				if offset, ok := p.turnOffset(target); ok {
					p.jumpTo(offset)
				}
			}
		}
	})
}

func (p *Plugin) drawTurnHeader(f ui.C, env *pluginapi.Env, group loadedTurn) {
	t := p.theme
	box := ui.Row(f).WidthPercent(100).MinHeight(26).Gap(8).Items(ui.Center).PadX(4).Children(func() {
		ui.Text(f, group.label).FontSize(t.Fonts.Caption).Color(t.LabelCaption)
	})
	p.turnBoxes[group.key] = box
}

func turnUsage(turn loadedTurn, data pluginapi.Snapshot) (input, output, total int64, records int) {
	for i := turn.start; i <= turn.end && i < len(data.Msgs); i++ {
		usage := data.Msgs[i].Usage
		if usage == nil {
			continue
		}
		records++
		input += usage.InputTokens
		output += usage.OutputTokens
		// totalTokens 是单次请求的上下文量，多步 Turn 不相加；只有一条
		// 真值记录时才原样展示它。
		total = usage.TotalTokens
	}
	if records != 1 {
		total = 0
	}
	return input, output, total, records
}

func turnTimes(turn loadedTurn, data pluginapi.Snapshot) (start, end int64) {
	accept := func(value int64) {
		if value <= 0 {
			return
		}
		if start == 0 || value < start {
			start = value
		}
		if value > end {
			end = value
		}
	}
	for i := turn.start; i <= turn.end && i < len(data.Msgs); i++ {
		key := messageRenderKey(&pluginapi.Env{Data: data}, data.Msgs[i], i)
		accept(data.MessageMeta[key].Time)
	}
	for _, event := range data.TurnEvents {
		if sameLoadedTurn(turn, event.TurnID, event.Turn) {
			accept(event.Time)
		}
	}
	return start, end
}

func turnStatus(turn loadedTurn, data pluginapi.Snapshot) string {
	status := ""
	for _, event := range data.TurnEvents {
		if !sameLoadedTurn(turn, event.TurnID, event.Turn) {
			continue
		}
		switch event.Kind {
		case "error":
			status = "失败"
		case "stopped":
			status = "已停止"
		case "done":
			status = "已完成"
		}
	}
	if status == "" && data.Busy && sameLoadedTurn(turn, data.ActiveTurnID, data.ActiveTurn) {
		status = "进行中"
	}
	return status
}

func formatTurnDuration(milliseconds int64) string {
	if milliseconds <= 0 {
		return ""
	}
	if milliseconds < 1000 {
		return strconv.FormatInt(milliseconds, 10) + "ms"
	}
	if milliseconds < 60000 {
		whole := milliseconds / 1000
		tenths := (milliseconds % 1000) / 100
		if tenths == 0 {
			return strconv.FormatInt(whole, 10) + "s"
		}
		return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(tenths, 10) + "s"
	}
	minutes := milliseconds / 60000
	seconds := (milliseconds % 60000) / 1000
	return strconv.FormatInt(minutes, 10) + "m" + strconv.FormatInt(seconds, 10) + "s"
}

func (p *Plugin) drawTurnFooter(f ui.C, env *pluginapi.Env, group loadedTurn) {
	data := env.Data
	input, output, total, records := turnUsage(group, data)
	start, end := turnTimes(group, data)
	status := turnStatus(group, data)
	if records == 0 && start == 0 && status == "" {
		return // 所有真值都未知时，整个页脚隐藏。
	}
	t := p.theme
	parts := make([]string, 0, 6)
	if input > 0 {
		parts = append(parts, "↑ "+strconv.FormatInt(input, 10))
	}
	if output > 0 {
		parts = append(parts, "↓ "+strconv.FormatInt(output, 10))
	}
	if total > 0 {
		parts = append(parts, "tokens "+strconv.FormatInt(total, 10))
	}
	if clock := formatMessageClock(start); clock != "" {
		parts = append(parts, clock)
	}
	if duration := formatTurnDuration(end - start); duration != "" {
		parts = append(parts, duration)
	}
	if status != "" {
		parts = append(parts, status)
	}
	if len(parts) == 0 {
		return
	}
	color := t.LabelCaption
	if status == "失败" {
		color = t.Error
	}
	ui.Row(f).WidthPercent(100).PadX(4).MinHeight(22).Items(ui.Center).Children(func() {
		ui.Text(f, strings.Join(parts, " · ")).FontSize(t.Fonts.Tiny).Color(color)
	})
}
