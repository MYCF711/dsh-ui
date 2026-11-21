// conn_indicator_test.go —— 连接指示器的事实必须是**当前会话**的。
//
// 为什么单独钉这一条：`Snapshot.Link` 以前由三个**包级布尔**喂出来，而写它们
// 的是**每一条会话**的订阅循环。于是"最后写的那条会话说了算"：
//
//	· 后台会话（或刚删掉、还在取消路上的那条）一次订阅失败，
//	  前台明明连着，侧栏那颗胶囊也翻成「连接异常」；
//	· 反过来，后台会话重连成功又会把前台的「断」盖成「连接成功」。
//
// 两个方向都在说谎，而且**只在多会话或宿主抖一下的时候出现**——单会话单测
// 永远绿。修法是把三枚事实挂到会话自己的订阅对象上（`muxSub`），快照按
// `m.cur` 取那一份。本文件钉三件事：
//
//	① 别的会话（或旧一代订阅）写事实，不改前台指示器；
//	② 切到哪条会话，指示器就报哪条会话的真值（含"还没事实 = 沿用上一条"）；
//	③ 删会话把它的连接事实一起带走（`releaseSession` 删订阅对象）；
//	④ 切会话那一交接带上"当前显示的事实"：冷载/拨号窗口不闪成「未知/连接中」
//	   （后者会被官方 800ms 最小可见撑成一次假警报）。
package main

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// connFactsLocked 是测试侧的读数口：当前会话那一份连接事实。
// 调用方必须持有 m.mu。
func connFactsLocked(m *model) (cur string, hasSub, up, down, connecting bool) {
	cur = m.cur
	if sub := m.subs[cur]; sub != nil {
		return cur, true, sub.up, sub.down, sub.connecting
	}
	return cur, false, false, false, false
}

// TestConnLinkIsPerSession 是①+②：指示器只读当前会话那一份。
func TestConnLinkIsPerSession(t *testing.T) {
	m := &model{cur: "front", subs: map[string]*muxSub{
		"front": {up: true},         // 前台连着
		"back":  {down: true},       // 后台订阅失败（还在退避）
		"boot":  {connecting: true}, // 后台刚建订阅、这次尝试还在飞行中
		"bg2":   {},                 // 后台另一条，下面会把它写成"连着"
	}}

	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("后台会话失败把前台指示器翻成 %v，want LinkConnected（旧实现是最后一个写者说了算）", got)
	}

	// 旧一代订阅（被 reconnectNow 换掉的那个对象）写不进新代的事实。
	m.mu.Lock()
	m.connFailedLocked("front", &muxSub{}) // 指针不等 ⇒ 丢弃
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("旧代订阅的失败盖住了新代事实：%v，want LinkConnected", got)
	}

	// 后台会话"连上了"也一样写不进来（另一个方向）。
	m.mu.Lock()
	m.connUpLocked("bg2", m.subs["bg2"])
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("后台会话重连成功盖住了前台指示器：%v，want LinkConnected", got)
	}
	if m.connLast != pluginapi.LinkUnknown {
		t.Fatalf("后台会话的确定事实改写了 connLast = %v（后台不许影响显示）", m.connLast)
	}

	// 切到正在首连的会话：如实报「连接中」，不沿用前台那条的结论。
	m.mu.Lock()
	m.cur = "boot"
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnecting {
		t.Fatalf("切到首连中的会话：指示器 = %v，want LinkConnecting", got)
	}

	// 切到自己失败的会话：报「断」，不是"永远只说好话"。
	m.mu.Lock()
	m.cur = "back"
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("切到自己失败的会话：指示器 = %v，want LinkDown", got)
	}

	// 切到还没有订阅的会话：没有事实 = 未知，不借别的会话的结论。
	m.mu.Lock()
	m.cur = "never"
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkUnknown {
		t.Fatalf("切到没有订阅的会话：指示器 = %v，want LinkUnknown（不知道就不许说已连接）", got)
	}
}

// TestConnFactsTransitions 钉住一次订阅尝试的三段迁移都由当前会话自己写。
func TestConnFactsTransitions(t *testing.T) {
	m := &model{cur: "s", subs: map[string]*muxSub{"s": {}}}
	sub := m.subs["s"]

	m.mu.Lock()
	m.connAttemptLocked("s", sub)
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnecting {
		t.Fatalf("尝试开始 = %v，want LinkConnecting", got)
	}

	m.mu.Lock()
	m.connUpLocked("s", sub)
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("订阅成功 = %v，want LinkConnected", got)
	}

	m.mu.Lock()
	m.connFailedLocked("s", sub)
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("连过又断 = %v，want LinkDown", got)
	}

	// 退避到点再试：down 还在（"上次失败、正在等"），connecting 也真，
	// 结论仍是「断」——与 `linkStateOf` 的优先级一致（断了就别说正在连）。
	m.mu.Lock()
	m.connAttemptLocked("s", sub)
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("退避重试中 = %v，want LinkDown（断的证据优先）", got)
	}
}

// TestReleaseSessionDropsConnectionFacts 是③：删会话把连接事实一起带走。
func TestReleaseSessionDropsConnectionFacts(t *testing.T) {
	m := &model{cur: "front", subs: map[string]*muxSub{
		"front": {up: true},
		"dead":  {down: true},
	}}

	m.releaseSession("dead")
	if _, ok := m.subs["dead"]; ok {
		t.Fatal("删会话后订阅还在：那条死会话的连接事实会继续被读到")
	}
	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("删掉后台会话把前台指示器翻成 %v，want LinkConnected", got)
	}

	// 删掉的正是当前会话：没有会话就没有流事实，指示器回到「未知」
	// （旧实现会停在那条死会话最后一次写入的结论上）。
	m.releaseSession("front")
	if got := m.connLink(); got != pluginapi.LinkUnknown {
		t.Fatalf("删掉当前会话后指示器 = %v，want LinkUnknown", got)
	}
}

// TestConnSwitchCarriesDisplayedFact 是④：切会话不闪，但切到哪条就报哪条的真值。
func TestConnSwitchCarriesDisplayedFact(t *testing.T) {
	m := &model{cur: "a", subs: map[string]*muxSub{
		"a": {up: true},   // 当前连着
		"b": {down: true}, // 后台已断
	}}

	m.mu.Lock()
	m.switchCurLocked("b")
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("切到已断的后台会话：%v，want LinkDown（它自己的事实最优先）", got)
	}

	// 再切到一条从没订过的会话：沿用「离开时那一帧」的 Down，不是「未知」。
	m.mu.Lock()
	m.switchCurLocked("cold")
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("冷开会话的载入窗口 = %v，want LinkDown（沿用上一条已显示的事实）", got)
	}
	// 它的订阅正在拨号（只有 connecting、还没有确定事实）：继续沿用，不闪成
	// 「重新连接中…」——那一态会被官方 800ms 最小可见撑成假警报。
	m.mu.Lock()
	m.subs["cold"] = &muxSub{connecting: true}
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkDown {
		t.Fatalf("拨号中的冷开会话 = %v，want LinkDown（沿用，不闪）", got)
	}

	// 回到那条连着的会话：立刻报它自己的真值。
	m.mu.Lock()
	m.switchCurLocked("a")
	m.mu.Unlock()
	if got := m.connLink(); got != pluginapi.LinkConnected {
		t.Fatalf("切回连着的会话 = %v，want LinkConnected", got)
	}
}
