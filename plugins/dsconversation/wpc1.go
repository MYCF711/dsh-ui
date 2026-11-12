package dsconversation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

func messageRenderKey(env *pluginapi.Env, msg apiclient.Message, index int) string {
	return pubapi.MessageKey(env.Data.Cur, msg, index)
}

// toolRenderID 适配 dstool.Service 目前的 int 展开键，同时保留它的 Renderer
// 分派。映射只存在于本插件进程内；完整 MessageKey 才拥有会话与消息身份，
// 这里的 int 不进入 Transport、不冒充 v4 seq，也不会被其他消息直接构造。
func (p *Plugin) toolRenderID(key string) int {
	if p.toolKeys == nil {
		p.toolKeys = map[string]int{}
	}
	if id, ok := p.toolKeys[key]; ok {
		return id
	}
	p.nextToolKey++
	if p.nextToolKey <= 0 {
		// 不静默回卷：回卷会让新消息与旧消息共享展开状态。
		panic("dstool 局部渲染键已耗尽")
	}
	p.toolKeys[key] = p.nextToolKey
	return p.nextToolKey
}

func formatMessageClock(value int64) string {
	if value <= 0 {
		return ""
	}
	// Transport time 是 Unix 毫秒；兼容旧 mockd 的 Unix 秒，但不把别的数值猜成时间。
	var at time.Time
	switch {
	case value >= 1_000_000_000_000:
		at = time.UnixMilli(value)
	case value >= 1_000_000_000:
		at = time.Unix(value, 0)
	default:
		return ""
	}
	return at.Local().Format("15:04")
}

func (p *Plugin) drawMessageMeta(f ui.C, env *pluginapi.Env, msg apiclient.Message, key string) {
	meta := env.Data.MessageMeta[key]
	clock := formatMessageClock(meta.Time)
	if clock == "" && !meta.Stopped {
		return
	}
	t := p.theme
	ui.Row(f).WidthPercent(100).Gap(8).Items(ui.Center).Children(func() {
		if clock != "" {
			ui.Text(f, clock).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
		}
		// TurnID 由真实 Turn shell 统一显示，不在每条消息下重复一遍。
		if meta.Stopped {
			ui.Text(f, "已停止").FontSize(t.Fonts.Tiny).Color(t.LabelTertiary)
		}
	})
}

func (p *Plugin) drawTurnEvents(f ui.C, env *pluginapi.Env) {
	t := p.theme
	for _, event := range env.Data.TurnEvents {
		event := event
		switch event.Kind {
		case "error":
			ui.Column(f).WidthPercent(100).Pad(8).Gap(4).Radius(t.Radii.SM).Background(t.BgLayer1).Children(func() {
				ui.Row(f).Gap(8).Items(ui.Center).Children(func() {
					// 标题是**给人看的**，不是协议词。旁边那个"已停止"卡一直是这么写的，
					// 只有这张卡漏了，把 `turn/error` 直接端到用户面前——他看不懂，
					// 而且这四个字看起来像界面坏了。协议词要留就留在日志里。
					ui.Text(f, "这一轮失败了").FontSize(t.Fonts.Caption).Color(t.Error)
					if event.TurnID != "" {
						ui.Text(f, event.TurnID).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
					}
					if clock := formatMessageClock(event.Time); clock != "" {
						ui.Text(f, clock).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
					}
				})
				if event.Error != "" {
					ui.Wrap(f, event.Error).FontSize(t.Fonts.Caption).Color(t.LabelSecondary)
				}
				if env.Act.Retry != nil {
					if ui.Button(f, "重试").Ghost().Small().Clicked() {
						env.Act.Retry()
					}
				}
			})
		case "stopped":
			ui.Row(f).WidthPercent(100).Gap(8).Items(ui.Center).Children(func() {
				ui.Text(f, "已停止").FontSize(t.Fonts.Caption).Color(t.LabelTertiary)
				if event.TurnID != "" {
					ui.Text(f, event.TurnID).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				}
				if clock := formatMessageClock(event.Time); clock != "" {
					ui.Text(f, clock).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				}
			})
		}
	}
}

func (p *Plugin) drawOutgoing(f ui.C, env *pluginapi.Env) {
	t := p.theme
	for _, outgoing := range env.Data.Outgoing {
		outgoing := outgoing
		ui.Column(f).WidthPercent(100).Pad(8).Gap(4).Radius(t.Radii.MD).Background(t.BgLayer1).Children(func() {
			ui.Wrap(f, outgoing.Text).FontSize(t.Fonts.Body).Color(t.LabelPrimary)
			ui.Row(f).Gap(8).Items(ui.Center).Children(func() {
				label := "发送中"
				color := t.LabelCaption
				switch outgoing.State {
				case pubapi.OutgoingSent:
					label = "已发送"
				case pubapi.OutgoingFailed:
					label, color = "发送失败", t.Error
				}
				ui.Text(f, label).FontSize(t.Fonts.Caption).Color(color)
				if outgoing.TurnID != "" {
					ui.Text(f, outgoing.TurnID).FontSize(t.Fonts.Tiny).Color(t.LabelCaption)
				}
				if outgoing.State == pubapi.OutgoingFailed && env.Act.RetrySend != nil {
					if ui.Button(f, "重试发送").Ghost().Small().Clicked() {
						env.Act.RetrySend(outgoing.Key)
					}
				}
			})
			if outgoing.Error != "" && outgoing.State == pubapi.OutgoingFailed {
				ui.Wrap(f, outgoing.Error).FontSize(t.Fonts.Caption).Color(t.Error)
			}
		})
	}
}

// rewriteExplicitFileLinks 只改写显式 Markdown file: destination，让 Viva 为 label
// 生成 hit fragment；原文/普通字符串/行内码/ fenced code 均保持不变。fake → 原目的地
// 的映射只存活于该消息缓存，点击时还原为 LinkKindFile。
func rewriteExplicitFileLinks(source string) (string, map[string]string) {
	files := map[string]string{}
	lines := strings.SplitAfter(source, "\n")
	var fence string
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)
		if indent <= 3 && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			continue
		}
		if fence == "" {
			lines[i] = rewriteFileLinksInText(line, files)
		}
	}
	return strings.Join(lines, ""), files
}

func rewriteFileLinksInText(line string, files map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '`' {
			n := 1
			for i+n < len(line) && line[i+n] == '`' {
				n++
			}
			marker := strings.Repeat("`", n)
			end := strings.Index(line[i+n:], marker)
			if end >= 0 {
				end += i + n + n
				out.WriteString(line[i:end])
				i = end
				continue
			}
		}
		if i+2 < len(line) && line[i] == ']' && line[i+1] == '(' {
			start := i + 2
			end := strings.IndexByte(line[start:], ')')
			if end >= 0 {
				end += start
				destination := strings.TrimSuffix(strings.TrimPrefix(line[start:end], "<"), ">")
				if strings.HasPrefix(strings.ToLower(destination), "file://") && !strings.ContainsAny(destination, " \t\r\n") {
					fake := "https://file.invalid/" + strconv.Itoa(len(files)+1)
					files[fake] = destination
					out.WriteString(line[i:start])
					out.WriteString(fake)
					out.WriteByte(')')
					i = end + 1
					continue
				}
			}
		}
		out.WriteByte(line[i])
		i++
	}
	return out.String()
}

func actionFailure(env *pluginapi.Env, action string, err error) {
	if err != nil && env.Act.Notify != nil {
		env.Act.Notify(fmt.Sprintf("%s失败: %v", action, err))
	}
}
