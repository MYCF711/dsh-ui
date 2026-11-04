package dsconversation

import (
	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/secretmask"
)

// userDisplayText 是**用户消息在界面上显示的那一份**。
//
// 官方部件的说法是 "Web display masking for recognized credentials in sent user text"——
// 遮的是**显示**，不是存储，也不是发出去的内容。用户粘进来的东西在会话里
// 仍然是原件（那是他自己发的），界面这一层不把凭据摊在屏幕上：
// 开会投屏、截图、旁边有人，这三种场景都不需要用户先想到"我这句话里有 key"。
//
// # 为什么只有用户消息
//
// 助手消息里的凭据不遮，两个原因：
//   - 助手写出来的 key 是它自己的（或者是我让它编的例子），遮了反而让人以为那是真凭据；
//   - 更要紧的是**别把遮罩当安全边界**：屏幕上遮住了、剪贴板和会话日志里还是明文，
//     那么"遮"这件事只解决偷看，不解决泄露。把它说成"防泄露"就会有人拿它当依据。
//
// # 为什么复制不跟着遮
//
// `Actions.Copy` 拿的是 `msg.Text` 原件。看起来不一致，但这是故意的：
// 复制是用户**取回自己发过的东西**，遮掉就等于把他粘进来的 key 弄丢了。
// 屏幕遮的是"被别人看见"，复制给的是"给我自己"——两件事不该用同一个开关。
//
// 代价必须写清楚：**看到的是遮过的，复制得到的是原文**。
func userDisplayText(m apiclient.Message) string {
	if m.Role != "user" {
		return m.Text
	}
	return secretmask.Mask(m.Text)
}
