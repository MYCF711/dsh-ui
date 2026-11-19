// official_models_retry.go —— ProviderEditor 的 retryPolicy 本地草稿与 Apply 提交。
//
// 官方不是逐行写：RetryPolicyEditor.onChange 只改 React draft（ProviderEditor.tsx:596-613），
// EditorFooter 的 Apply 才做一次 settings.mutate（:314-384,673-686），pathOps 只产生
// before/after 之间最小的 set/unset（:130-159）。本文件实现同一写入边界。
package dssettings

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/MYCF711/dsh-ui/internal/apiclient"
)

// retryEditorState 是一张打开中的 provider 卡。original 是 user 层，inherited 是
// base 层；policy=nil 表示继承，不是空策略。所有数值保留 json.Number 直到写盘。
//
// credentialRef 是这张卡 Apply 时要写的凭据引用（官方 `keyRef`，ProviderEditor.tsx:224）。
// 它在**开卡那一刻**就钉住：写设置时 revision 变了不会连累它，但凭据域与设置文档
// 是两套 revision，重算一次反而可能落到用户没打开过的那张卡上。
type retryEditorState struct {
	route         string
	credentialRef string
	original      map[string]any
	inherited     map[string]any
	policy        map[string]any
	revision      int
	writable      bool
	generation    uint64
	// keyRemovalRef 非空 = 用户已请求删掉这个凭据引用（官方 `keyDraft = {ref}`，
	// ProviderEditor.tsx:203-205）。它是**待办**，还没发请求。
	keyRemovalRef string
	// keyRemovalApplied = 官方 `keyDraft.removed === true`（ProviderEditor.tsx:354）：
	// `credentials.unset` 已经成功。设了它就撤不回——官方注释「A later settings
	// failure cannot restore a secret that was deleted」，所以后面的设置写失败时
	// 卡片不关、Cancel 变 close、说明改成 `keyRemoveApplied`。
	keyRemovalApplied bool
}

// keyRemovalPending 表示"这一帧有一笔待删的凭据"（官方 `keyRemoval !== undefined`）。
func (s *retryEditorState) keyRemovalPending() bool {
	return s != nil && s.keyRemovalRef != ""
}

// requestKeyRemoval 记下待删的引用（官方 `saved.clear()` → `setKeyDraft({ref})`，
// ProviderEditor.tsx:496）。重复请求幂等：已经记着就保持原状。
func (s *retryEditorState) requestKeyRemoval() {
	if s == nil || s.credentialRef == "" {
		return
	}
	s.keyRemovalRef = s.credentialRef
}

func defaultRetryPolicy() map[string]any {
	codes := make([]any, 0, len(DefaultRetryableCodes))
	for _, code := range DefaultRetryableCodes {
		codes = append(codes, code)
	}
	return map[string]any{
		"mode": RetryModeNormal, "maxRetries": json.Number(strconv.Itoa(RetryDefaultMaxRetries)),
		"retryableCodes": codes,
		"backoff": map[string]any{
			"initialDelayMs": json.Number(strconv.Itoa(RetryDefaultInitialDelayMs)),
			"maxDelayMs":     json.Number(strconv.Itoa(RetryDefaultMaxDelayMs)),
			"jitterRatio":    json.Number("0.1"),
		},
	}
}

func newRetryEditorState(describe *apiclient.SettingsDescribeResult, route string) (*retryEditorState, error) {
	if describe == nil {
		return nil, fmt.Errorf("宿主未接 settings.describe")
	}
	namespace, ok := describe.Namespace(apiclient.SettingsNSLLM)
	if !ok {
		return nil, fmt.Errorf("宿主未提供 llm 命名空间")
	}
	path := []string{"providers", route, "retryPolicy"}
	base, _ := valueAtPath(namespace.Base, path)
	effective, _ := valueAtPath(namespace.Value, path)
	user, _ := valueAtPath(namespace.User, path)
	state := &retryEditorState{route: route, revision: namespace.Revision, writable: describe.Writable}
	if section, found := describe.Section(apiclient.SettingsNSLLM); found {
		state.revision, state.writable = section.Revision, section.Writable
	}
	state.original, _ = user.(map[string]any)
	state.original = cloneObject(state.original)
	state.inherited, _ = base.(map[string]any)
	if state.inherited == nil && state.original == nil {
		// 老宿主只给 Value 时，它不是 user 草稿；把 effective 当继承层显示。
		state.inherited, _ = effective.(map[string]any)
	}
	state.inherited = cloneObject(state.inherited)
	state.policy = cloneObject(state.original)
	if state.policy != nil {
		if _, hasMode := state.policy["mode"]; !hasMode {
			state.policy["mode"] = state.inheritedMode()
		}
	}
	return state, nil
}

func (s *retryEditorState) clone() *retryEditorState {
	if s == nil {
		return nil
	}
	copy := *s
	copy.original = cloneObject(s.original)
	copy.inherited = cloneObject(s.inherited)
	copy.policy = cloneObject(s.policy)
	return &copy
}

func mergePolicy(layers ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, layer := range layers {
		for key, value := range layer {
			if nested, ok := value.(map[string]any); ok {
				parent, _ := out[key].(map[string]any)
				out[key] = mergePolicy(parent, nested)
			} else {
				out[key] = value
			}
		}
	}
	return out
}

func (s *retryEditorState) resolved() map[string]any {
	return mergePolicy(defaultRetryPolicy(), s.inherited, s.policy)
}

func (s *retryEditorState) view() (RetryPolicyView, error) {
	raw, err := json.Marshal(s.resolved())
	if err != nil {
		return RetryPolicyView{Route: s.route}, err
	}
	return ParseRetryPolicyView(s.route, raw)
}

func (s *retryEditorState) inheritedMode() string {
	resolved := mergePolicy(defaultRetryPolicy(), s.inherited)
	if mode, ok := resolved["mode"].(string); ok {
		return mode
	}
	return RetryModeNormal
}

// rows 复用官方 8 code/边界投影，再补「默认（继承）」mode。显示值来自 resolved，
// 但 original/policy 仍只保存用户层，因此保存不会把继承值全部写进 user。
func (s *retryEditorState) rows() []PanelRow {
	view, err := s.view()
	if err != nil {
		return []PanelRow{{ID: "retry-" + s.route + "-invalid", Title: "重试策略", Kind: ItemSelector,
			Disabled: true, Reason: err.Error()}}
	}
	rows := RetryPolicyRows(s.route, view, s.revision, s.writable)
	if len(rows) == 0 {
		return rows
	}
	inheritedMode := s.inheritedMode()
	rows[0].Choices = append([]PanelChoice{{Value: "", Label: "默认（" + inheritedMode + "）", Desc: "删除 user 层 retryPolicy，继承 base"}}, rows[0].Choices...)
	if s.policy == nil {
		rows[0].Selected = ""
		rows[0].Value = "默认（" + inheritedMode + "）"
	} else if mode, ok := s.policy["mode"].(string); ok {
		rows[0].Selected = mode
	}
	return rows
}

func (s *retryEditorState) ensurePolicy() {
	if s.policy != nil {
		return
	}
	s.policy = map[string]any{"mode": s.inheritedMode()}
}

func (s *retryEditorState) setField(path []string, value any) {
	s.ensurePolicy()
	if len(path) == 1 {
		s.policy[path[0]] = value
		return
	}
	backoff, _ := s.policy["backoff"].(map[string]any)
	if backoff == nil {
		backoff = map[string]any{}
		s.policy["backoff"] = backoff
	}
	backoff[path[1]] = value
}

func sameStringCodes(left any, right []string) bool {
	list, ok := left.([]any)
	if !ok || len(list) != len(right) {
		return false
	}
	// 官方 every(inherited.includes)：集合相同即可，顺序不重要。
	seen := make(map[string]int, len(list))
	for _, value := range list {
		text, ok := value.(string)
		if !ok {
			return false
		}
		seen[text]++
	}
	for _, code := range right {
		seen[code]--
		if seen[code] < 0 {
			return false
		}
	}
	return true
}

// change 只改内存草稿；它没有任何 writer 调用，也不返回 WriteSaved。
func (s *retryEditorState) change(row PanelRow) error {
	field := row.Path[len(row.Path)-1]
	if len(row.Path) >= 4 && row.Path[2] == "retryPolicy" && row.Path[3] == "mode" {
		next := nextChoice(row.Choices, row.Selected)
		if next == "" {
			s.policy = nil
		} else {
			s.ensurePolicy()
			s.policy["mode"] = next
		}
		return nil
	}
	if len(row.Path) >= 4 && row.Path[3] == "retryableCodes" {
		view, err := s.view()
		if err != nil {
			return err
		}
		code := row.Title
		ops, err := RetryPolicyToggleCodeOps(s.route, view.RetryableCodes, code)
		if err != nil {
			return err
		}
		codes, ok := ops[0].Value.([]string)
		if !ok {
			return fmt.Errorf("retryableCodes 草稿不是字符串数组")
		}
		inherited := mergePolicy(defaultRetryPolicy(), s.inherited)["retryableCodes"]
		if sameStringCodes(inherited, codes) {
			// 官方 select：回到继承集合就删除该字段，不写一份继承副本。
			if s.policy != nil {
				delete(s.policy, "retryableCodes")
				if len(s.policy) == 0 {
					s.policy = nil
				}
			}
			return nil
		}
		s.setField([]string{"retryableCodes"}, codes)
		return nil
	}
	current, err := strconv.ParseFloat(row.Value, 64)
	if err != nil {
		return fmt.Errorf("%s 当前值不是数字：%q", field, row.Value)
	}
	next := current + row.Bounds.Step
	if next > row.Bounds.Max || next < row.Bounds.Min {
		next = row.Bounds.Min
	}
	path := []string{field}
	if len(row.Path) >= 5 && row.Path[3] == "backoff" {
		path = []string{"backoff", field}
	}
	if field == "maxRetries" {
		s.setField(path, int(next))
	} else {
		s.setField(path, next)
	}
	return nil
}

func nextChoice(choices []PanelChoice, selected string) string {
	for index, choice := range choices {
		if choice.Value == selected {
			return choices[(index+1)%len(choices)].Value
		}
	}
	if len(choices) > 0 {
		return choices[0].Value
	}
	return ""
}

// ops 是官方 pathOps 的 Go 等价物：递归对象、数组/标量整体 set、缺失键 unset。
func (s *retryEditorState) ops() []apiclient.SettingsOp {
	base := []string{"providers", s.route, "retryPolicy"}
	if s.policy == nil {
		if s.original == nil {
			return nil
		}
		return []apiclient.SettingsOp{apiclient.SettingsUnsetOp(base)}
	}
	return diffSettingsObject(base, s.original, s.policy)
}

func diffSettingsObject(base []string, before, after map[string]any) []apiclient.SettingsOp {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	var ops []apiclient.SettingsOp
	for _, key := range ordered {
		oldValue, had := before[key]
		newValue, has := after[key]
		path := append(append([]string(nil), base...), key)
		if !has {
			ops = append(ops, apiclient.SettingsUnsetOp(path))
			continue
		}
		newObject, newIsObject := newValue.(map[string]any)
		oldObject, oldIsObject := oldValue.(map[string]any)
		if newIsObject && (!had || oldIsObject) {
			ops = append(ops, diffSettingsObject(path, oldObject, newObject)...)
			continue
		}
		if had && reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		ops = append(ops, apiclient.SettingsSetOp(path, newValue))
	}
	return ops
}

func (s *retryEditorState) validate() error {
	_, err := s.view()
	return err
}
