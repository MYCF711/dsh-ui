// skill_catalog_wiring.go —— 把官方 `skills.list` 接到 `dsskill` 的 `openSkill` 接缝上。
//
// 为什么需要这一层：`plugins/dsskill/register.go` 的 `SetInspect` 一开始就留好了，
// 但**故意留成 nil**——官方 `openSkill` 要用 `skills/list` 的目录把 `/name` 解析成
// 路径（`'/' + skill.name === ref` 的整串匹配），而当时本仓宿主没有这个方法。
// 宿主补上 `skills.list` 之后（DSH-GO `cmd/dshd/serve_agent_api_skills.go`），
// 这里负责在会话切换时把目录取回来并注入 resolver。
//
// 三条纪律：
//  1. **按会话取，不全局取一次**：目录随该会话的 cwd / agent preset 变（官方
//     `skill-catalog.ts` 从 `projections.workingDirectory ?? header.cwd` 取 cwd）。
//  2. **拿不到就撤掉，不假装有**：失败时注入 nil —— 官方 `inspect !== undefined`
//     才画按钮，所以失败的自然结果是"按钮不出现"，而不是点了没反应。
//  3. **不在绘制路径上发 HTTP**：`SetInspect` 只换一个函数值；取目录跑在
//     `openSession` 的异步支路里，拿不到也不阻塞首帧。
package main

import (
	"context"
	"log"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
	"github.com/MYCF711/dsh-ui/internal/pluginapi"
	"github.com/MYCF711/dsh-ui/plugins/dsfiles"
	"github.com/MYCF711/dsh-ui/plugins/dsskill"
)

// skillCatalogTimeout 是一次目录拉取的上限。
//
// 3s：它与 `loadSession` 同量级（官方这条是 cold-readable 的元数据读，
// 不加载正文），所以不该比首屏加载等更久。
const skillCatalogTimeout = 3 * time.Second

// refreshSkillCatalog 重新拉一次某会话的 skill 目录，并把它接到 `dsskill` 上。
//
// 必须在会话切换时调用（`openSession`），因为目录是**按会话**的。
// 任何失败都只降级成"没有目录"（`SetInspect(nil)`），不冒泡成错误弹窗：
// 官方在没目录时的形状就是"不画 inspect 按钮"。
func (m *model) refreshSkillCatalog(sessionID string) {
	// 这个函数是从 openSession 起的 goroutine 里跑的，而**壳不一定已经装好**：
	// 夹具与某些启动路径上 kernel / cli 还是 nil。先挡掉，别让一个可选能力
	// 把整条会话加载崩掉（2026-11-13 CI 上就是这么崩的：nil kernel -> SIGSEGV）。
	if m == nil || m.kernel == nil || m.cli == nil {
		return
	}
	skill := m.skillPlugin()
	if skill == nil {
		return // 没装 dsskill：没有接缝可接，静默跳过。
	}
	if sessionID == "" {
		skill.SetInspect(nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), skillCatalogTimeout)
	defer cancel()

	result, err := m.cli.SkillsList(ctx, sessionID)
	if err != nil {
		// 拉失败 = 这一轮没有目录。撤掉 resolver（按钮随之消失），
		// 与官方 `inspect === undefined` 同形状；不把失败画成"空目录"。
		skill.SetInspect(nil)
		log.Printf("skill 目录拉取失败（会话 %s）：%v", sessionID, err)
		return
	}

	// 目录快照按值捕获：切会话后旧 resolver 不该再读到新会话的目录。
	catalog := result.Skills
	// 同一份目录也喂给 composer 的 `/` 候选，别为一个东西拉两遍。
	candidates := make([]pluginapi.SkillCandidate, 0, len(catalog))
	for _, item := range catalog {
		candidates = append(candidates, pluginapi.SkillCandidate{Name: item.Name, Description: item.Description, ModelInvocable: item.ModelInvocable})
	}
	m.mu.Lock()
	if m.skills == nil {
		m.skills = map[string][]pluginapi.SkillCandidate{}
	}
	m.skills[sessionID] = candidates
	m.mu.Unlock()
	skill.SetInspect(func(ref string) {
		path := apiclient.ResolveSkillPath(catalog, ref)
		if path == "" {
			// 官方 `?.path` 找不到就是不画按钮；这里同样什么都不做，
			// 而不是拿名字当路径猜一个。
			return
		}
		m.openSkillPath(path)
	})
	m.inv.Invalidate()
}

// skillPlugin 找装配好的 `dsskill` 实例（没装返回 nil）。
func (m *model) skillPlugin() *dsskill.Plugin {
	if m == nil || m.kernel == nil {
		return nil
	}
	for _, pl := range m.kernel.Plugins() {
		if s, ok := pl.(*dsskill.Plugin); ok {
			return s
		}
	}
	return nil
}

// openSkillPath 打开 skill 的 SKILL.md 预览。
//
// 复用 `dsfiles.Open`：官方 `openSkill` 是把路径交给"打开文件"那条既有能力，
// 不是新开一个预览器（`dsh-client-ui-skill` 只负责"名字 -> 路径"这一步）。
func (m *model) openSkillPath(path string) {
	if m == nil || m.kernel == nil {
		return
	}
	for _, pl := range m.kernel.Plugins() {
		if f, ok := pl.(*dsfiles.Plugin); ok {
			f.Open(m.cwd, path)
			m.inv.Invalidate()
			return
		}
	}
	log.Printf("skill 路径 %q 解析出来了，但没有可用的预览能力", path)
}
