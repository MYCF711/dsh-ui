package dsconversation

import (
	"strings"
	"testing"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// 这个文件测的是**显示层的一道决定**：用户消息里看起来像凭据的东西不摊在屏幕上。
//
// 为什么值得单测：遮罩的规则本身在 internal/secretmask 里已经测过了，
// 这里要钉的是**边界**——"遮谁"。写错成"全遮"会让助手的示例 key 也变星号，
// 写错成"不遮"就等于没接。两头都得有反证。

const sampleKey = "sk-abcdefghijklmnopqrstuvwxyz0123"

func msg(role, text string) apiclient.Message {
	return apiclient.Message{Role: role, Text: text}
}

func TestUserDisplayTextMasksUserSecrets(t *testing.T) {
	got := userDisplayText(msg("user", "我的 key 是 "+sampleKey))

	if strings.Contains(got, sampleKey) {
		t.Fatalf("完整的 key 还在屏幕上：%q", got)
	}
	// 首尾要留着：用户得能认出"是哪一个 key"。全打星号会让
	// "我想确认是不是用错 key"变成不可能。
	if !strings.Contains(got, "sk-abc") || !strings.Contains(got, "0123") {
		t.Errorf("首尾没保留，认不出是哪个 key：%q", got)
	}
	if !strings.Contains(got, "我的 key 是 ") {
		t.Errorf("凭据周围的正常文字被吃掉了：%q", got)
	}
}

func TestUserDisplayTextLeavesPlainTextAlone(t *testing.T) {
	// 反面：多遮的代价是**正常聊天内容被吞掉**，而且每次都会发生。
	// 这几条是"看起来像但其实是正常话"的样本。
	for _, s := range []string{
		"帮我看一下 plugins/dsopen/open.go 为什么画不出来",
		"用 Bearer 认证就行，不要用 Basic",
		"这个版本号是 1.2.3-build.4567",
		"",
	} {
		if got := userDisplayText(msg("user", s)); got != s {
			t.Errorf("正常文本被改了：%q → %q", s, got)
		}
	}
}

// TestUserDisplayTextOnlyMasksUserRole 是这条规则的正向对照。
//
// 没有它的话，把 userDisplayText 写成"对谁都用 Mask"照样全绿——
// 而那是另一个决定（助手写出来的示例 key 也会变星号，让人以为那是真凭据）。
func TestUserDisplayTextOnlyMasksUserRole(t *testing.T) {
	for _, role := range []string{"assistant", "thinking", "tool", ""} {
		text := "示例：" + sampleKey
		if got := userDisplayText(msg(role, text)); got != text {
			t.Errorf("role=%q 的消息被遮了，但这条规则只管用户消息：%q", role, got)
		}
	}
}

// TestMaskPreservesLength 钉住一条**被依赖的性质**：遮罩不改变长度。
//
// 虚拟化按 `estimateMsgHeight` 留位、按真实文本绘制。两边用的是不同字符串
// （一份原件、一份遮过），它们只有在**长度相同**时才必然量出同一个高度。
// 现在确实相同——`maskOne` 是"留首尾 6+4、中间换成同数量的星号"。
//
// 这条不是在测 secretmask 的实现细节，是在测**本插件对它的依赖**：
// 哪天有人把遮罩改成 `***`（塌缩），高度估算就会和绘制错位，
// 表现是消息下面多一截空白或少一截——不会崩，所以也没人会来报。
func TestMaskPreservesLength(t *testing.T) {
	for _, text := range []string{
		"我的 key 是 " + sampleKey,
		"Bearer ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 拿去用",
		"两条 sk-aaaaaaaaaaaaaaaaaaaa 和 sk-bbbbbbbbbbbbbbbbbbbb 一起",
		sampleKey,
		"没有凭据的普通一句话",
	} {
		got := userDisplayText(msg("user", text))
		if len(got) != len(text) {
			t.Errorf("遮罩改了长度（%d → %d）：%q → %q\n"+
				"高度估算按长度算，长度变了它就和绘制错位。",
				len(text), len(got), text, got)
		}
	}
}

// TestEstimateHeightFollowsDrawnText 是上一条的正面用法：
// 估算和绘制同源时，"有没有凭据"不该改变高度。
func TestEstimateHeightFollowsDrawnText(t *testing.T) {
	p := &Plugin{}
	// 等长的两串，一串含凭据、一串不含。遮罩长度不变 ⇒ 高度必须一样。
	withSecret := strings.Repeat("x", 200) + sampleKey
	plain := strings.Repeat("x", 200) + strings.Repeat("y", len(sampleKey))
	if len(withSecret) != len(plain) {
		t.Fatalf("样本不等长（%d vs %d），这条测试就没有意义了", len(withSecret), len(plain))
	}
	a := p.estimateMsgHeight(msg("user", withSecret), 400, "k1")
	b := p.estimateMsgHeight(msg("user", plain), 400, "k2")
	if a != b {
		t.Errorf("等长的两条消息估出不同高度（%v vs %v）——"+
			"估算对'有没有凭据'敏感，说明它量的不是画出来的那份", a, b)
	}
	// 反向：估算不能退化成常量，否则上面两个相等毫无意义。
	short := p.estimateMsgHeight(msg("user", sampleKey), 400, "k3")
	if short >= a {
		t.Errorf("短的估高 %v 不该 ≥ 长的 %v（估算退化成常量了）", short, a)
	}
}
