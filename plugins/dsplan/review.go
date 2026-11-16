package dsplan

import (
	"net/url"
	"strings"

	"github.com/MYCF711/dsh-ui/internal/planreview"
	"github.com/MYCF711/dsh-ui/pluginapi"
	"github.com/vivaui/viva/ui"
)

// reviewPreviewAddress 是官方 review-preview.ts:12-20 的临时审阅地址。
// 文档正文只放 Plugin.temporaryReviews（等价官方 navigation.params），不进布局持久化。
func reviewPreviewAddress(sessionID, requestKey string) string {
	if sessionID == "" || requestKey == "" {
		return ""
	}
	return "dsh-resource://plan-review/" + uriComponent(sessionID) + "/" + uriComponent(requestKey)
}

func parseReviewPreviewAddress(address string) (sessionID, requestKey string, ok bool) {
	const prefix = "dsh-resource://plan-review/"
	if !strings.HasPrefix(address, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(address, prefix)
	if rest == "" || strings.ContainsAny(rest, "?#") {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	for i := range parts {
		decoded, err := url.PathUnescape(parts[i])
		if err != nil || decoded == "" {
			return "", "", false
		}
		parts[i] = decoded
	}
	return parts[0], parts[1], true
}

// drawReviewAction 是官方 PlanReviewOpen：待审卡首次挂到屏幕上时自动打开一次，
// 同时保留不回答问题的「查看全文」手动入口。openedReviews 让手动收起右栏跨帧有效。
func (p *Plugin) drawReviewAction(f ui.C, env *pluginapi.Env) {
	if env == nil || p.theme == nil {
		return
	}
	review, ok := planreview.Pending(env.Data.Msgs)
	if !ok {
		return
	}
	identity := "review:" + review.RequestKey
	if review.CallID != "" {
		identity = "call:" + review.CallID
	}
	openedKey := env.Data.Cur + "\x00" + identity
	if !p.openedReviews[openedKey] {
		p.openReview(env, review)
		if p.openedReviews == nil {
			p.openedReviews = map[string]bool{}
		}
		p.openedReviews[openedKey] = true
	}
	link := ui.Row(f).Items(ui.Center).Gap(4).
		Hit(env.Tree.ID(env.Feat, "plan-review-open-"+review.RequestKey), zhOpen)
	if link.Clicked() {
		p.openReview(env, review)
	}
	link.Children(func() {
		ui.Text(f, zhFull).FontSize(14).LineHeight(20).Color(p.theme.LabelSecondary)
	})
}

func (p *Plugin) openReview(env *pluginapi.Env, review planreview.Review) {
	if env == nil {
		return
	}
	if review.CallID != "" {
		// 有 callId 时官方始终打开 durable plan 资源；资源缺失由 preview 显示 notFound，
		// 不拿临时正文冒充已落盘的那一次调用。
		p.open(env, env.Data.Cur, Document{CallID: review.CallID})
		return
	}
	address := reviewPreviewAddress(env.Data.Cur, p.reviewWindow+":"+review.RequestKey)
	if address == "" {
		return
	}
	title, _ := planreview.Summary(review.Plan)
	if p.temporaryReviews == nil {
		p.temporaryReviews = map[string]Document{}
	}
	p.temporaryReviews[address] = Document{Title: title, Markdown: review.Plan}
	p.openAddress = address
	if env.Act.SelectRightTab != nil {
		env.Act.SelectRightTab(RightTabPlan)
	}
	if !env.Data.ShowRightbar && env.Act.ToggleRightbar != nil {
		env.Act.ToggleRightbar()
	}
}
