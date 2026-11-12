package dsconversation

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/vivaui/viva/gfx"
)

// 这些测试只管一件事：**记录记成哪个键、撤回是不是真的清掉、别人给的键有没有被碰**。
// 它们不承诺任何上报——宿主没有 feedback API（见 feedback.go 顶部）。

// fakeConfig 是一个最小的 ConfigGet/ConfigSet 替身：行为和壳一致——
// ConfigSet 是**合并**语义（只覆盖给出的键，空串也照写）。
type fakeConfig struct {
	store map[string]string
	sets  []map[string]string
}

func newFakeConfig(seed map[string]string) *fakeConfig {
	store := map[string]string{}
	for k, v := range seed {
		store[k] = v
	}
	return &fakeConfig{store: store}
}

func (f *fakeConfig) actions() pluginapi.Actions {
	return pluginapi.Actions{
		ConfigGet: func() map[string]string {
			out := map[string]string{}
			for k, v := range f.store {
				out[k] = v
			}
			return out
		},
		ConfigSet: func(kv map[string]string) {
			copied := map[string]string{}
			for k, v := range kv {
				f.store[k] = v
				copied[k] = v
			}
			f.sets = append(f.sets, copied)
		},
	}
}

func TestFeedbackKeyIsSessionAndIndex(t *testing.T) {
	if got, want := feedbackKey("sess-1", 7), "feedback.sess-1.7"; got != want {
		t.Fatalf("键 = %q，期望 %q", got, want)
	}
	// 同一会话的不同下标是两条独立记录，不能被 overwrite 合并成一条。
	if feedbackKey("s", 1) == feedbackKey("s", 2) {
		t.Fatal("不同消息下标生成了同一个键")
	}
}

func TestToggleFeedbackWritesThenRetracts(t *testing.T) {
	cfg := newFakeConfig(nil)
	act := cfg.actions()
	p := &Plugin{}
	key := feedbackKey("sess-a", 3)

	p.toggleFeedback(act, "sess-a", 3, feedbackUp)
	if got := p.feedbackRating("sess-a", 3); got != feedbackUp {
		t.Fatalf("内存态 = %q，期望 up", got)
	}
	if cfg.store[key] != "up" {
		t.Fatalf("配置里 %s = %q，期望 up", key, cfg.store[key])
	}

	// 同值再点 = 撤回：内存里没了，配置里写墓碑空串（ConfigSet 没有删除动作，
	// 但空串读回来等于"没记录"，不会让界面以为还有评分）。
	p.toggleFeedback(act, "sess-a", 3, feedbackUp)
	if got := p.feedbackRating("sess-a", 3); got != "" {
		t.Fatalf("撤回后内存态 = %q，期望空", got)
	}
	if got := cfg.store[key]; got != "" {
		t.Fatalf("撤回后配置里 %s = %q，期望空串墓碑", key, got)
	}
	if len(cfg.sets) != 2 {
		t.Fatalf("ConfigSet 调用次数 = %d，期望 2", len(cfg.sets))
	}
	if cfg.sets[1][key] != "" {
		t.Fatalf("撤回那一次 ConfigSet 写的是 %q，期望空串", cfg.sets[1][key])
	}

	// 另一侧评分直接替换，不产生第二条记录。
	p.toggleFeedback(act, "sess-a", 3, feedbackDown)
	if cfg.store[key] != "down" {
		t.Fatalf("改判后 %s = %q，期望 down", key, cfg.store[key])
	}
}

func TestFeedbackSeedsFromConfigOnce(t *testing.T) {
	cfg := newFakeConfig(map[string]string{
		"feedback.sess-r.2": feedbackDown,
	})
	p := &Plugin{}
	act := cfg.actions()
	p.seedFeedback(act)
	p.seedFeedback(act) // 第二次是空转

	if got := p.feedbackRating("sess-r", 2); got != feedbackDown {
		t.Fatalf("从配置读回的评分 = %q，期望 down", got)
	}
	// 壳缓存随后变了也不该覆盖内存（本进程的点击才是真相）。
	cfg.store["feedback.sess-r.2"] = feedbackUp
	p.seedFeedback(act)
	if got := p.feedbackRating("sess-r", 2); got != feedbackDown {
		t.Fatalf("第二次 seeding 覆盖了内存态: %q", got)
	}
}

func TestFeedbackIgnoresForeignAndInvalidEntries(t *testing.T) {
	cfg := newFakeConfig(map[string]string{
		"feedback.good.1": feedbackUp,
		"feedback.bad.1":  "maybe",   // 不是我们写的两个合法值
		"theme":           "light",   // 别的键
		"feedback.tomb.1": "",        // 撤回墓碑
		"feedback.":       "garbage", // 前缀对但内容不是记录
	})
	p := &Plugin{}
	p.seedFeedback(cfg.actions())

	if got := p.feedbackRating("good", 1); got != feedbackUp {
		t.Fatalf("合法记录没读回来: %q", got)
	}
	if got := p.feedbackRating("bad", 1); got != "" {
		t.Fatalf("非法值 %q 被当成了评分", got)
	}
	if got := p.feedbackRating("tomb", 1); got != "" {
		t.Fatalf("墓碑空串被当成了评分: %q", got)
	}
	if len(p.feedback) != 1 {
		t.Fatalf("内存表里混进了非记录键: %v", p.feedback)
	}
}

func TestFeedbackWithoutConfigActionsStaysInMemory(t *testing.T) {
	// 壳没接配置通道：点击仍然生效，只是重启即失——而**不做任何写入**。
	act := pluginapi.Actions{}
	p := &Plugin{}
	p.toggleFeedback(act, "sess-m", 5, feedbackUp)
	if got := p.feedbackRating("sess-m", 5); got != feedbackUp {
		t.Fatalf("没有配置通道时内存态 = %q，期望 up", got)
	}
}

func TestFeedbackEmptySessionDoesNotRecord(t *testing.T) {
	cfg := newFakeConfig(nil)
	p := &Plugin{}
	p.toggleFeedback(cfg.actions(), "", 0, feedbackUp)
	if len(cfg.sets) != 0 {
		t.Fatalf("没有会话 id 却写了配置: %v", cfg.sets)
	}
	if len(p.feedback) != 0 {
		t.Fatalf("没有会话 id 却记了一条: %v", p.feedback)
	}
}

func TestThumbIconMirrorsAndFills(t *testing.T) {
	outline := thumbIcon(true, false)
	filled := thumbIcon(true, true)
	if outline.Stroke <= 0 {
		t.Fatal("未评分的拇指该是描边（空心）")
	}
	if filled.Stroke != 0 {
		t.Fatal("已评分的拇指该是填充（实心）")
	}
	if len(outline.Commands) != len(filled.Commands) {
		t.Fatalf("空心/实心路径长度不同：%d vs %d（切换会跳形）", len(outline.Commands), len(filled.Commands))
	}

	down := thumbIcon(false, false)
	up := thumbIcon(true, false)
	if len(down.Commands) != len(up.Commands) {
		t.Fatalf("上/下拇指路径长度不同：%d vs %d", len(down.Commands), len(up.Commands))
	}
	// 镜像：同一个点，y 关于 8 对称。
	var upPoint bool
	for i := range up.Commands {
		if up.Commands[i].Kind != gfx.PathCommandMove {
			continue
		}
		upPoint = true
		if down.Commands[i].Point.Y != 16-up.Commands[i].Point.Y || down.Commands[i].Point.X != up.Commands[i].Point.X {
			t.Fatalf("👎 不是 👍 的镜像：第 %d 个 MoveTo %+v vs %+v",
				i, down.Commands[i].Point, up.Commands[i].Point)
		}
	}
	if !upPoint {
		t.Fatal("拇指路径里没有 MoveTo")
	}
}
