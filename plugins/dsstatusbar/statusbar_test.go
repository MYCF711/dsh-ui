package dsstatusbar

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
)

// 这个文件只测**纯函数**：不碰 flex / canvas / 窗口，所以能在 CI 里裸跑。
// draw 里那些 f.Text(...) 靠无头截图验收（见 docs/plugin-api.md §8）。

func TestTok(t *testing.T) {
	// tok 是状态栏上「↑9.5M ↓571.3k」的格式化。
	// 边界重要性：
	//   · 999/1000 是 k 的门槛，差一位就会显示成「1000」而不是「1.0k」；
	//   · 1999 必须进成「2.0k」——分位四舍五入进到 10 之后要能把整位加一，
	//     否则会打印出「1.10k」这种没人认识的东西；
	//   · 1e6 是 M 的门槛，且 M 之后不再叠加（不会出现「1.5kM」）。
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"零", 0, "0"},
		{"三位数原样", 999, "999"},
		{"整一千", 1000, "1.0k"},
		{"六千五", 6500, "6.5k"},
		{"分位进位到整位", 1999, "2.0k"},
		{"十万级", 190800, "190.8k"},
		{"百万门槛", 1000000, "1.0M"},
		{"一百五十万", 1500000, "1.5M"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tok(tc.in); got != tc.want {
				t.Fatalf("tok(%d) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFmt1(t *testing.T) {
	// fmt1 是手写的一位小数格式化（不为它 import fmt）。
	// 边界重要性：
	//   · 9.96 必须进成 10.0——「小数位进满 10 要把整数位加一」这一支不写，
	//     会输出 9.10，用户看到的是「九点一零」而不是「十」；
	//   · 负数被夹到 0：速率在界面上没有负数的语义，显示 -1.2 只会让人怀疑数据坏了。
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"零", 0, "0.0"},
		{"小于半", 0.4, "0.4"},
		{"正好半", 0.5, "0.5"},
		{"小数进满到整数", 9.96, "10.0"},
		{"负数夹到零", -1.2, "0.0"},
		{"普通两位", 86.34, "86.3"},
		{"速率常见值", 40.7, "40.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fmt1(tc.in); got != tc.want {
				t.Fatalf("fmt1(%v) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRate(t *testing.T) {
	// rate 是官方 `formatTokensPerSecond` 的移植：≥10 取整（`Math.round`），
	// <10 一位小数；单位写作「 tok/s」（一个空格、小写）。
	// 边界重要性：
	//   · 9.96 按官方先看 "<10" 再 `Math.round(99.6)/10 = 10` → "10"，不能出 "10.0"；
	//   · 4.35 → "4.4"（一位小数）；
	//   · 负数/NaN 没有速率语义 → 夹到 0（不能画 "NaN tok/s"）。
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0 tok/s"},
		{4.35, "4.4 tok/s"},
		{9.96, "10 tok/s"},
		{10, "10 tok/s"},
		{40.7, "41 tok/s"},
		{132.2, "132 tok/s"},
		{-1.2, "0 tok/s"},
	}
	for _, tc := range cases {
		if got := rate(tc.in); got != tc.want {
			t.Fatalf("rate(%v) = %q，want %q", tc.in, got, tc.want)
		}
	}
}

func TestItoa(t *testing.T) {
	// 与 dsconversation 里那份同名同义（两个包各留一份，避免为了一个数字互相依赖）。
	// 负数分支必须踩到：少写负号会让「N 轮」在异常输入下显示成正数。
	cases := []struct {
		name string
		in   int
		want string
	}{
		{"零", 0, "0"},
		{"个位", 3, "3"},
		{"多位", 2026, "2026"},
		{"负数", -7, "-7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := itoa(tc.in); got != tc.want {
				t.Fatalf("itoa(%d) = %q，want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestUsageKnownGatesTokenDisplay 是一条**行为性**断言：状态栏靠 Usage.Known()
// 决定「显不显示 token 那一段」。规则是「宿主没给就一个字都不画」——
// 显示 "0 tok" 比不显示更糟，用户会以为模型没消耗。
//
// 注意 cacheReadTokens 单独有值不算「已知」：缓存读量是同一批 token 的另一种记法，
// 只出现它意味着 input/output 缺失，这时候画出来仍然是误导。
func TestUsageKnownGatesTokenDisplay(t *testing.T) {
	cases := []struct {
		name  string
		usage pluginapi.Usage
		want  bool
	}{
		{"全零：不显示", pluginapi.Usage{}, false},
		{"只有缓存读：仍不显示", pluginapi.Usage{CacheReadTokens: 16384}, false},
		{"有上下文总量：显示", pluginapi.Usage{TotalTokens: 23717}, true},
		{"有累计输入：显示", pluginapi.Usage{InputTokens: 6502}, true},
		{"有累计输出：显示", pluginapi.Usage{OutputTokens: 831}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.usage.Known(); got != tc.want {
				t.Fatalf("Usage%+v.Known() = %v，want %v", tc.usage, got, tc.want)
			}
		})
	}
}

// TestUsageSegments 钉住 dock 的现有宿主真值。官方 sessionStats/cache 尚未进入
// Snapshot，所以这里只保留 whole-log total、可证明的 input/output 和原始 ContextTokens；
// 绝不能从消息窗口补轮/步，也不能补缓存率或 decode 速率。contextWindow 的百分比见
// context_window_test.go。
func TestUsageSegments(t *testing.T) {
	cases := []struct {
		name   string
		data   pluginapi.Snapshot
		want   string
		banned []string
	}{
		{
			name: "全零：整段不画", data: pluginapi.Snapshot{}, want: "",
			banned: []string{"↑", "↓", "总量", "上下文", "%"},
		},
		{
			name: "宿主 whole-log total + 原始上下文",
			data: pluginapi.Snapshot{
				UsageTotal:    pluginapi.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
				ContextTokens: 2000,
			},
			want:   "总量 150 · 上下文 2.0k",
			banned: []string{"↑", "↓", "上下文 100", "上下文 50", "%", "tok/s"},
		},
		{
			name: "只有原始上下文：保留 token 数但没有容量就不画百分比",
			data: pluginapi.Snapshot{ContextTokens: 2000}, want: "上下文 2.0k",
			banned: []string{"↑", "↓", "%"},
		},
		{
			name: "上下文为零是未知：不拿 whole-log total 顶替",
			data: pluginapi.Snapshot{
				UsageTotal: pluginapi.Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
			},
			want: "总量 150", banned: []string{"上下文 0", "%"},
		},
		{
			name: "旧 total 字段缺席时保留可证明 input output",
			data: pluginapi.Snapshot{
				UsageTotal: pluginapi.Usage{InputTokens: 100, OutputTokens: 50},
			},
			want: "↑100 ↓50", banned: []string{"总量 0", "上下文", "%"},
		},
		{
			name: "只有 input 真值：不把未知 output 补成零",
			data: pluginapi.Snapshot{UsageTotal: pluginapi.Usage{InputTokens: 100}},
			want: "↑100", banned: []string{"↓", "↑0", "总量"},
		},
		{
			name: "只有 output 真值：不把未知 input 补成零",
			data: pluginapi.Snapshot{UsageTotal: pluginapi.Usage{OutputTokens: 50}},
			want: "↓50", banned: []string{"↑", "↓0", "总量"},
		},
		{
			name: "旧 mockd 路径：UsageTotal 全零时退回 Usage 但不冒充 decode 速率",
			data: pluginapi.Snapshot{Usage: pluginapi.Usage{
				InputTokens: 6502, OutputTokens: 831, TotalTokens: 23717,
				TokPerSec: 40.7, HasRate: true,
			}},
			want: "↑6.5k ↓831 · 上下文 23.7k", banned: []string{"tok/s", "总量 23.7k", "%"},
		},
		{
			name: "旧 Usage 只有缓存读：既没有总量也没有缓存分母",
			data: pluginapi.Snapshot{Usage: pluginapi.Usage{CacheReadTokens: 16384}}, want: "",
			banned: []string{"↑", "↓", "总量", "上下文", "缓存", "%"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Join(usageSegments(tc.data), " · ")
			if text != tc.want {
				t.Fatalf("用量段 = %q，want %q", text, tc.want)
			}
			for _, bad := range tc.banned {
				if strings.Contains(text, bad) {
					t.Fatalf("dock 文本 %q 里不该出现 %q", text, bad)
				}
			}
		})
	}
}
