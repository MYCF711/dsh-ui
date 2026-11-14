// official_panels_wiring.go —— 把两个官方面板的数据面接进产品。
//
// 这里接的是两个**新落地**的官方包：
//
//	plugins/dsactioncards  ← dsh-client-ui-action-cards       （审计 #2 MISSING）
//	plugins/dsattachment   ← dsh-client-ui-attachment 的 rail （审计 #6 PARTIAL 的缺口服）
//
// 两个插件都拿不到 `*apiclient.Client`（红线 R2：插件只拿 *Scope），
// 所以各自暴露一个包级注入点，由壳在这里转一手：
//
//	dsactioncards.SetSource(CardSource)
//	dsattachment.SetSink(AttachmentSink)
//
// 另外 `dsattachment.HandleFileDrop` 要由壳挂到 `app.Config.OnFileDrop`——
// 那是 Viva 的系统拖放入口（`viva@v0.6.0/app/app.go:150-156`），
// 事件里 `gfx.FileDropEvent.Paths` 是**绝对路径**（`gfx/input.go:262-274`），
// 正好是宿主 `attachment.put` 要的入参（`HOST:internal/transport/server.go:844-862`）。
package main

import (
	"context"
	"encoding/json"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/plugins/dsactioncards"
	"github.com/MYCF711/dsh-ui/plugins/dsattachment"
	"github.com/MYCF711/dsh-ui/plugins/dsdeliverables"
)

// cardSource 是壳给动作卡插件的数据源。
//
// `View` 走的是**会话投影**而不是独立方法：官方没有 `actionCards.view` 这个宿主方法，
// 卡片视图是 `session.projections` 返回的 `values.actionCards`
// （`HOST:internal/transport/handlers_session_official.go:947`、
// `HOST:internal/transport/handlers_action_cards_view.go:38-52`）。
// `ActionCardsView` 就是为了这条路径写的，别去拼一个 named-method 请求。
type cardSource struct{ cli *apiclient.Client }

func (s cardSource) View(ctx context.Context, sessionID string) (*apiclient.ActionCardView, error) {
	return s.cli.ActionCardsView(ctx, sessionID)
}

func (s cardSource) Act(ctx context.Context, sessionID, callID, verb string, payload map[string]json.RawMessage) (json.RawMessage, error) {
	return s.cli.ActionCardAct(ctx, sessionID, callID, verb, payload)
}

// attachSink 是壳给附件 rail 的上传能力。
type attachSink struct {
	cli *apiclient.Client
	m   *model
}

func (s attachSink) Put(ctx context.Context, path string) (*apiclient.Attachment, error) {
	return s.cli.AttachmentPut(ctx, path)
}

func (s attachSink) Attach(ctx context.Context, sessionID string, a apiclient.Attachment) error {
	return s.cli.SessionAttach(ctx, sessionID, a)
}

// CurrentSession 返回当前会话 id；空串表示"还不知道"（插件据此不把附件挂上去，
// 而不是挂到别人的会话里）。
func (s attachSink) CurrentSession() string {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	return s.m.cur
}

// wireOfficialPanels 注入两个面板的数据面。`cli == nil`（无头测试）时什么都不注入，
// 两个插件拿到 nil 就一个节点都不画——这也正是官方「空槽不占位」的语义。
func wireOfficialPanels(m *model) {
	if m.cli == nil {
		return
	}
	dsactioncards.SetSource(cardSource{cli: m.cli})
	dsattachment.SetSink(attachSink{cli: m.cli, m: m})
	// #14 的按文件 diff：数据面走 `/api/changes.*`（裸 JSON 路由）。
	dsdeliverables.SetChangesSource(changesSink{cli: m.cli})
}

// refreshActionCards 让动作卡插件按会话刷一次投影。
//
// 为什么由壳驱动：卡片的数据是**会话级投影**，刷新时机（load / history 之后、
// 收到新 journal 帧之后）只有壳知道；插件自己每帧发请求会把 UI 拖死。
// 按能力查表而不是持实例：插件没装时这一下是空操作。
func (m *model) refreshActionCards(sessionID string) {
	if m.kernel == nil || sessionID == "" {
		return
	}
	for _, p := range m.kernel.Plugins() {
		if refresher, ok := p.(interface {
			Refresh(context.Context, string) error
		}); ok && p.ID() == "ds-actioncards" {
			go func(r interface {
				Refresh(context.Context, string) error
			}) {
				_ = r.Refresh(context.Background(), sessionID)
			}(refresher)
			return
		}
	}
}

// changesSink 是壳给交付物面板的「变更 diff」能力（#14）。
//
// `dsdeliverables` 拿不到 `*apiclient.Client`（红线 R2），所以壳在这里转一手；
// 插件拿到 nil 就退回"写过的文件 + 全文预览"的旧路径——**不画空 diff**。
//
// 三条走的是**文档相对路由**（`/api/changes.summary|diff|open`），不是
// `POST /api/<method>`：官方消费端直接 `fetch` + `response.json()`，
// 响应体是**裸对象**（没有 `result` 信封）。见
// `HOST:docs/transport-contract.md` §15.3。
type changesSink struct{ cli *apiclient.Client }

func (s changesSink) Summary(ctx context.Context, sessionID string, seq int) (*apiclient.ChangesSummary, error) {
	return s.cli.ChangesSummary(ctx, sessionID, seq)
}

func (s changesSink) Diff(ctx context.Context, sessionID string, seq, index int) (*apiclient.ChangesDiff, error) {
	return s.cli.ChangesDiff(ctx, sessionID, seq, index)
}

func (s changesSink) Open(ctx context.Context, sessionID string, seq, index int, action, application string) error {
	return s.cli.ChangesOpen(ctx, sessionID, seq, index, action, application)
}
