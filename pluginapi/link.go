// link.go —— 壳与宿主之间**流连接**的三态事实（给 `Snapshot.Link` 用）。
//
// # 为什么是三个值而不是一个布尔
//
// 旧字段 `Snapshot.Disconnected bool` 的 `false` 同时承担两种意思：
//
//	① 连接是好的（有证据的"连着"）
//	② 壳还没拿到任何连接事实（首帧 / 还没订阅 / 老壳没接）
//
// 界面无法区分，于是"写一句已连接"在 ② 的情况下就是编的。这个仓已经因为
// 这类"缺状态被当成正常"踩过真机事故（宿主被杀 33 秒后界面还写「已连接 · 就绪」）。
// 所以这里把"不知道"提升成一等状态：**不知道就写「未知」**。
//
// # 谁写它
//
// 壳（`cmd/app`）在真实 apiclient 回调里写，只有三个事实来源：
//
//	apiclient.Mux.Subscribe 进行中  → LinkConnecting（官方 data-phase=connecting）
//	apiclient.Mux.Subscribe 成功    → LinkConnected
//	apiclient.Mux.Subscribe 失败    → LinkDown（并且带退避重试）
//
// 插件只读，不自己发探测请求（绘制期发请求会卡帧，且插件不知道 token）。
package pluginapi

// LinkState 是壳对"这一帧与宿主的流连接"的判定。
type LinkState uint8

const (
	// LinkUnknown 是"还没有任何一条确定的连接事实"。
	//
	// 场景：进程刚起、还没打开会话（没订阅）、壳是旧的没接这个字段。
	// **不要**把它当成"连着"或"断了"：它是"不知道"，界面必须如实写「未知」。
	LinkUnknown LinkState = iota
	// LinkConnected 表示 WS 订阅已建立 —— 这是"连着"的唯一证据。
	LinkConnected
	// LinkDown 表示订阅失败/断开，壳正在按退避重试。
	LinkDown
	// LinkConnecting 表示**一次订阅正在飞行中**（首连 / 退避到点后的重试 /
	// 用户点了「立即重连」之后的重新订阅）。
	//
	// 它是官方 `ConnectionIndicator` 的 `connecting` 态（`data-phase=connecting`，
	// 文案 `connection.connecting`）。与 `LinkDown` 的区别是"现在有没有一次尝试
	// 正在进行"：`LinkDown` 是"上一次尝试失败了、正在等退避"。
	//
	// 追加在末尾是为了不挪动前三个值：`LinkUnknown` 必须是零值（缺事实 ≠ 连着）。
	LinkConnecting
)

// String 只服务日志与测试失败信息（界面文案由插件按官方 i18n 决定）。
func (s LinkState) String() string {
	switch s {
	case LinkConnected:
		return "connected"
	case LinkDown:
		return "down"
	case LinkConnecting:
		return "connecting"
	default:
		return "unknown"
	}
}
