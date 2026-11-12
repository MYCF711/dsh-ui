package dssidebar

import (
	"testing"

	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dscore"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 这个文件测"删除会话"这条链的前半：**两次点击**。
//
// 删除是破坏性的（宿主那边是从磁盘上删），所以这里要钉住三件事：
//   1. 点一次只是"待确认"，**不删**；
//   2. 点删除**不会顺手把会话切过去**（× 嵌在可点的行里）；
//   3. 用户去开别的会话时，待确认状态必须被清掉——留着它，下一次误点就直接删了。

type deleteProbe struct {
	plugin   *Plugin
	env      *pluginapi.Env
	canvas   *gfx.Canvas
	tree     *gfx.Tree
	feature  gfx.Scope
	deleted  []string
	opened   []string
	notified []string
}

// newDeleteProbe 造一个"点了就记下来"的侧栏。
func newDeleteProbe(t *testing.T, withCapability bool) *deleteProbe {
	t.Helper()
	p, _, env, tree, canvas := v2Fixture()
	probe := &deleteProbe{plugin: p, env: env, canvas: canvas, tree: tree, feature: env.Feat}
	env.Act.OpenSession = func(id string) { probe.opened = append(probe.opened, id) }
	env.Act.Notify = func(msg string) { probe.notified = append(probe.notified, msg) }
	if withCapability {
		env.Act.DeleteSession = func(id string) { probe.deleted = append(probe.deleted, id) }
	}
	return probe
}

// draw 画一帧会话行。click 非空时把事件目标设成那个 id。
func (d *deleteProbe) draw(t *testing.T, click string) {
	t.Helper()
	tree := d.tree
	canvas := v2Canvas(tree)
	if click != "" {
		canvas.BeginEventPass(tree.ID(d.feature, click))
		defer canvas.EndEventPass()
	}
	row := sess("s1", "一个会话", `D:\work`, "2026-11-03T20:00:00+08:00")
	var view flex.View
	view.Draw(canvas, dscore.VivaTheme(d.plugin.theme), ui.Rect{Width: 268, Height: 40}, func(f *flex.Frame) {
		d.plugin.drawSession(f, d.env, row)
	})
}

func TestDeleteNeedsTwoClicks(t *testing.T) {
	d := newDeleteProbe(t, true)

	// 第一下：只进"待确认"。
	d.draw(t, "del-s1")
	if len(d.deleted) != 0 {
		t.Fatalf("点一次就删了（%v）——这是破坏性动作，必须两次", d.deleted)
	}
	if d.plugin.pendingDelete != "s1" {
		t.Fatalf("点一次之后 pendingDelete = %q，want s1", d.plugin.pendingDelete)
	}
	if len(d.notified) == 0 {
		t.Error("第一次点应当提示「再点一次就删掉」——不提示的话用户不知道发生了什么")
	}

	// 第二下：真的删掉，并且把待确认状态清掉。
	d.draw(t, "del-s1")
	if len(d.deleted) != 1 || d.deleted[0] != "s1" {
		t.Fatalf("第二次点应当删掉 s1，实际 deleted=%v", d.deleted)
	}
	if d.plugin.pendingDelete != "" {
		t.Errorf("删完 pendingDelete 应当是空，实际 %q（留着它，下一次误点会删掉另一个）",
			d.plugin.pendingDelete)
	}
}

// TestDeleteBoxDoesNotAlsoOpenTheSession 是嵌套命中的安全断言。
//
// × 是画在**可点的行**里面的。如果事件冒泡到外层，点删除就会顺手把会话切过去——
// 用户想删一个、结果打开了它，下一帧还要再多点一次。
func TestDeleteBoxDoesNotAlsoOpenTheSession(t *testing.T) {
	d := newDeleteProbe(t, true)
	// 先确认"这个 id 确实画出来了"：点到行里别的地方会打开会话。
	d.draw(t, "s-s1")
	if len(d.opened) != 1 {
		t.Fatalf("点行本身应当打开会话，实际 opened=%v", d.opened)
	}

	d2 := newDeleteProbe(t, true)
	d2.draw(t, "del-s1")
	if len(d2.opened) != 0 {
		t.Errorf("点删除不该同时打开会话（opened=%v）——事件冒泡到外层行了", d2.opened)
	}
}

// TestDeleteHiddenWithoutCapability：壳没接 DeleteSession 时，
// 点那个位置什么都不该发生——而这个位置本来也不该画出来。
func TestDeleteHiddenWithoutCapability(t *testing.T) {
	d := newDeleteProbe(t, false)
	d.draw(t, "del-s1")
	if len(d.deleted) != 0 {
		t.Errorf("壳没接删除能力，却调了删除（%v）", d.deleted)
	}
	if d.plugin.pendingDelete != "" {
		t.Errorf("壳没接删除能力，却进了待确认态（%q）", d.plugin.pendingDelete)
	}
}

// TestOpeningSessionCancelsPendingDelete：注意力挪走了，待确认要作废。
func TestOpeningSessionCancelsPendingDelete(t *testing.T) {
	d := newDeleteProbe(t, true)
	d.draw(t, "del-s1")
	if d.plugin.pendingDelete != "s1" {
		t.Fatalf("前置条件没成立：pendingDelete = %q", d.plugin.pendingDelete)
	}

	d.draw(t, "s-s1")
	if len(d.opened) != 1 {
		t.Fatalf("点行应当打开会话，实际 %v", d.opened)
	}
	if d.plugin.pendingDelete != "" {
		t.Errorf("打开别的会话之后 pendingDelete 应当清空，实际 %q", d.plugin.pendingDelete)
	}
}
