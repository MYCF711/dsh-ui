// models_page_wiring.go —— Models 页三路官方目录的同步缓存与后台刷新。
//
// 插件在绘制期只读快照，绝不同步等 HTTP。首次读取返回 Loading=true 并启动
// 一次后台刷新；三路请求可顺序执行，但都在 goroutine 内。每一路独立保留真实
// 数据或错误原文，501 不得折叠成空目录。全部结束后一次发布并 Invalidate。
package main

import (
	"context"
	"time"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

const modelsPageTimeout = 8 * time.Second

// modelsPageSnapshot 同步返回缓存副本；首读触发一次后台三路请求。
func (m *model) modelsPageSnapshot() *apiclient.ModelsPageSnapshot {
	m.mu.Lock()
	start := !m.modelsPageLoaded && !m.modelsPagePending && m.cli != nil
	if start {
		m.modelsPagePending = true
	}
	snapshot := cloneModelsPageSnapshot(m.modelsPageCache)
	snapshot.Loading = m.modelsPagePending
	m.mu.Unlock()

	if start {
		go m.refreshModelsPage()
	}
	return &snapshot
}

// modelsPageRefresh 显式请求后台重读；在飞请求不叠加，也不启动 ticker。
func (m *model) modelsPageRefresh() {
	m.mu.Lock()
	if m.modelsPagePending {
		// 在飞请求可能已经读过旧 default；必须排队再读，不能直接丢刷新。
		m.modelsPageRefreshQueued = true
		m.mu.Unlock()
		return
	}
	if m.cli == nil {
		m.mu.Unlock()
		return
	}
	m.modelsPagePending = true
	m.mu.Unlock()
	go m.refreshModelsPage()
}

// refreshModelsPage 必须在 goroutine 中调用。三路结果全部到齐后一次发布。
func (m *model) refreshModelsPage() {
	if m.cli == nil {
		m.mu.Lock()
		m.modelsPagePending = false
		m.modelsPageRefreshQueued = false
		m.mu.Unlock()
		return
	}

	var next apiclient.ModelsPageSnapshot
	ctx, cancel := context.WithTimeout(context.Background(), modelsPageTimeout)
	registered, err := m.cli.ListProviders(ctx)
	cancel()
	if err != nil {
		next.RegisteredError = err.Error()
	} else {
		next.Registered = registered
	}

	ctx, cancel = context.WithTimeout(context.Background(), modelsPageTimeout)
	configurable, err := m.cli.ListConfigurableProviders(ctx)
	cancel()
	if err != nil {
		next.ConfigurableError = err.Error()
	} else {
		next.Configurable = configurable
	}

	ctx, cancel = context.WithTimeout(context.Background(), modelsPageTimeout)
	catalog, err := m.cli.SessionModelCatalog(ctx)
	cancel()
	if err != nil {
		next.CatalogError = err.Error()
	} else {
		next.Catalog = &catalog
	}

	m.mu.Lock()
	// 与官方 ModelsSettingsStore 一致：刷新失败保留该路上一次真值，同时显示
	// 本次错误；不能把瞬时 501/网络失败投成“provider/模型消失了”。
	if next.RegisteredError != "" {
		next.Registered = m.modelsPageCache.Registered
	}
	if next.ConfigurableError != "" {
		next.Configurable = m.modelsPageCache.Configurable
	}
	if next.CatalogError != "" {
		next.Catalog = m.modelsPageCache.Catalog
	}
	m.modelsPageCache = next
	m.modelsPageLoaded = true
	rerun := m.modelsPageRefreshQueued
	m.modelsPageRefreshQueued = false
	m.modelsPagePending = rerun
	m.mu.Unlock()
	m.inv.Invalidate()
	if rerun {
		go m.refreshModelsPage()
	}
}

// cloneModelsPageSnapshot 深拷缓存，插件不能改写壳正在持有的切片或目录。
func cloneModelsPageSnapshot(in apiclient.ModelsPageSnapshot) apiclient.ModelsPageSnapshot {
	out := in
	out.Registered = cloneModelsPageSlice(in.Registered)
	out.Configurable = cloneModelsPageSlice(in.Configurable)
	for i := range out.Configurable {
		out.Configurable[i].SettingsPath = cloneModelsPageSlice(in.Configurable[i].SettingsPath)
		out.Configurable[i].BaseURLEnv = cloneModelsPageSlice(in.Configurable[i].BaseURLEnv)
		if in.Configurable[i].NativeAuth != nil {
			auth := *in.Configurable[i].NativeAuth
			auth.Env = cloneModelsPageSlice(in.Configurable[i].NativeAuth.Env)
			out.Configurable[i].NativeAuth = &auth
		}
	}
	out.Catalog = cloneModelsPageCatalog(in.Catalog)
	return out
}

func cloneModelsPageCatalog(in *apiclient.ModelCatalog) *apiclient.ModelCatalog {
	if in == nil {
		return nil
	}
	out := *in
	out.RoutableProviders = cloneModelsPageSlice(in.RoutableProviders)
	out.Groups = cloneModelsPageSlice(in.Groups)
	out.Failures = cloneModelsPageSlice(in.Failures)
	for i := range out.Groups {
		out.Groups[i].Models = cloneModelsPageSlice(in.Groups[i].Models)
		for j := range out.Groups[i].Models {
			if in.Groups[i].Models[j].Reasoning == nil {
				continue
			}
			reasoning := *in.Groups[i].Models[j].Reasoning
			reasoning.Efforts = cloneModelsPageSlice(in.Groups[i].Models[j].Reasoning.Efforts)
			out.Groups[i].Models[j].Reasoning = &reasoning
		}
	}
	return &out
}

// cloneModelsPageSlice 保留 nil 与真实空切片的区别。
func cloneModelsPageSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}
