package main

import (
	"sort"
	"testing"

	pubapi "github.com/MYCF711/dsh-ui/pluginapi"
)

// 这个文件是"两个面板抢同一层"的守卫。
//
// # 为什么要它
//
// 壳的每个槽位按 `Order` 从小到大画。**同槽同 Order 的两个面，谁在上面
// 由插件的装配顺序决定**——那不是设计，是巧合。而叠加类面板（overlay）是
// 全窗口覆盖的：谁后画谁盖住对方。
//
// 2026-11-04 真撞了一次：`dscommands`（F2 命令面板）和 `dsdeliverables`
// （交付物面板）都写 `SlotShellOverlay, Order: 96`。两个都开着时，一个盖住另一个，
// 而"盖住"这件事没有任何人写过。这种缺陷**不会崩、不会报错**，只是某个面板
// 偶尔"打不开"——而人第一反应是"这个功能没做"。
//
// 判据：同一个槽位里 `Order` 必须互不相同。
// 唯一例外：允许**完全相同**的一组（目前没有），真要例外得写进 allowSameOrder 并说明理由。

// allowSameOrder 是"故意同层"的白名单，(槽位, Order) → 理由。
//
// 留空是刻意的：真需要同层时，先想清楚"谁该在上面"——
// 想清楚了就会给出两个不同的数；想不清楚说明这两个面本来就不该同时出现。
var allowSameOrder = map[string]string{}

type panelSeat struct {
	plugin  string
	painter string
}

// TestNoTwoPanelsShareOneSlotAndOrder 是守卫本身。
func TestNoTwoPanelsShareOneSlotAndOrder(t *testing.T) {
	plugins, err := pubapi.Builtin().Plugins()
	if err != nil {
		t.Fatalf("拿内置插件失败：%v", err)
	}
	if len(plugins) < 20 {
		t.Fatalf("只拿到 %d 个插件——init 没跑到？这个守卫必须有全量装配才有意义", len(plugins))
	}

	seats := map[string]map[int][]panelSeat{} // 槽位 → Order → 占位者
	for _, p := range plugins {
		withPanels, ok := p.(pubapi.Panels)
		if !ok {
			continue
		}
		for _, painter := range withPanels.Panels() {
			slot := string(painter.Slot)
			if seats[slot] == nil {
				seats[slot] = map[int][]panelSeat{}
			}
			seats[slot][painter.Order] = append(seats[slot][painter.Order],
				panelSeat{plugin: p.ID(), painter: painter.ID})
		}
	}

	var bad []string
	for slot, byOrder := range seats {
		for order, who := range byOrder {
			if len(who) < 2 {
				continue
			}
			key := slot + "|" + itoa(order)
			if _, allowed := allowSameOrder[key]; allowed {
				continue
			}
			names := make([]string, len(who))
			for i, s := range who {
				names[i] = s.plugin + "/" + s.painter
			}
			sort.Strings(names)
			bad = append(bad, "槽 "+slot+" 的 Order "+itoa(order)+" 被 "+itoa(len(who))+" 个面占了："+
				joinNames(names))
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Fatalf("同一个槽位的 Order 必须互不相同——同 Order 谁在上面由装配顺序决定，"+
			"那不是设计：\n  %s\n"+
			"给其中一个换个 Order（modal 用 99、预览类用 97/98 这一带），不要把它们压在同一个数上。",
			joinLines(bad))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	u := uint64(n)
	if neg {
		u = -u
	}
	var buf [21]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "、"
		}
		out += n
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += l
	}
	return out
}
