// locales.go —— 官方 `dsh-client-ui-skill/src/client/locales.ts` 的**逐字**移植。
//
// # 为什么一个字都不改
//
// 官方 `index.ts` 的装配行是：
//
//	ctx.effect(() => ctx.locale.register(NS, { zh, en, ja, es, fr, de, ru, pt }), 'ui-skill: dictionaries')
//
// 也就是说这 8 条字典是**产品文案**（用户可见），不是内部标识符。改了就是
// 「我们编了一段官方没有的话」——任务书边界里明确禁止。
//
// # 三条官方事实
//
//	① `zh` 是**键集的唯一真源**（官方用 `satisfies Record<string, string>`），
//	   其余 7 条用 `Record<SkillKey, string>` 与它对齐——少一个键编译期就红。
//	   本文件的 `SkillKey` 与 8 条字典是同一套约束，测试逐条核键集。
//	② `row.instructions` 在官方是 `说明` / `Instructions`，而 CSS 里
//	   `.instructionsHeader{text-transform:uppercase}` 会把它显示成
//	   `说明` / `INSTRUCTIONS`。**两件事都在官方**：数据是小写混合，
//	   变换在样式层。实现不许把大写写进字典。
//	③ `menu.userOnly` 的用法在 `index.ts:183`：
//	   `skill.modelInvocable ? description : `${t('menu.userOnly')} · ${description}``
//	   ——只有 `modelInvocable === false` 时才加前缀，且用 ` · ` 连接（U+00B7 两侧各一个空格）。
package dsskill

// SkillKey 是 `skill` 命名空间的键（官方 `locales.ts:19` 的 `SkillKey`）。
type SkillKey = string

// 官方 8 个键，顺序与 `locales.ts` 的 `zh` 字面量一致。
const (
	// KeyRowTitle 是标题（`加载技能` / `Skill`）。
	KeyRowTitle SkillKey = "row.title"
	// KeyRowRunning 是读屏状态：加载中。
	KeyRowRunning SkillKey = "row.running"
	// KeyRowPreparing 是读屏状态：准备中。
	KeyRowPreparing SkillKey = "row.preparing"
	// KeyRowFailed 是读屏状态：失败。
	KeyRowFailed SkillKey = "row.failed"
	// KeyRowStopped 是中止时的可见摘要（**不是**读屏专用）。
	KeyRowStopped SkillKey = "row.stopped"
	// KeyRowInstructions 是展开体的标题（header + aria-label）。
	KeyRowInstructions SkillKey = "row.instructions"
	// KeyRowInspect 是 inspect 按钮文案。
	KeyRowInspect SkillKey = "row.inspect"
	// KeyMenuUserOnly 是斜杠菜单里「仅用户可调」的标记。
	KeyMenuUserOnly SkillKey = "menu.userOnly"
)

// officialSkillKeys 是全部 8 个键（顺序 = 官方 `zh` 字面量顺序）。
var officialSkillKeys = []SkillKey{
	KeyRowTitle,
	KeyRowRunning,
	KeyRowPreparing,
	KeyRowFailed,
	KeyRowStopped,
	KeyRowInstructions,
	KeyRowInspect,
	KeyMenuUserOnly,
}

// SkillDict 是一个语言的字典（键 → 文案）。
type SkillDict map[SkillKey]string

// zh 是简体中文（官方 `locales.ts:7-16`，键集真源）。
var zh = SkillDict{
	KeyRowTitle:        "加载技能",
	KeyRowRunning:      "正在加载 skill",
	KeyRowPreparing:    "准备加载技能",
	KeyRowFailed:       "skill 加载失败",
	KeyRowStopped:      "skill 加载已中止",
	KeyRowInstructions: "说明",
	KeyRowInspect:      "查看",
	KeyMenuUserOnly:    "仅用户",
}

// en 是 English（官方 `locales.ts:22-31`）。
var en = SkillDict{
	KeyRowTitle:        "Skill",
	KeyRowRunning:      "Loading skill",
	KeyRowPreparing:    "Preparing to load a skill",
	KeyRowFailed:       "Skill load failed",
	KeyRowStopped:      "Skill load stopped",
	KeyRowInstructions: "Instructions",
	KeyRowInspect:      "Inspect",
	KeyMenuUserOnly:    "user-only",
}

// ja 是日本語（官方 `locales.ts:34-43`）。
var ja = SkillDict{
	KeyRowTitle:        "スキル",
	KeyRowRunning:      "スキルを読み込み中",
	KeyRowPreparing:    "スキルの読み込みを準備中",
	KeyRowFailed:       "スキルの読み込みに失敗しました",
	KeyRowStopped:      "スキルの読み込みが停止しました",
	KeyRowInstructions: "手順",
	KeyRowInspect:      "詳細を確認",
	KeyMenuUserOnly:    "ユーザー専用",
}

// es 是 Español（官方 `locales.ts:46-55`）。
var es = SkillDict{
	KeyRowTitle:        "Habilidad",
	KeyRowRunning:      "Habilidad de carga",
	KeyRowPreparing:    "Preparándose para cargar una habilidad",
	KeyRowFailed:       "La carga de habilidades falló",
	KeyRowStopped:      "Carga de habilidades detenida",
	KeyRowInstructions: "Instrucciones",
	KeyRowInspect:      "inspeccionar",
	KeyMenuUserOnly:    "solo usuario",
}

// fr 是 Français（官方 `locales.ts:58-67`）。
var fr = SkillDict{
	KeyRowTitle:        "Compétence",
	KeyRowRunning:      "Compétence de chargement",
	KeyRowPreparing:    "Préparation du chargement d’une compétence",
	KeyRowFailed:       "Échec du chargement des compétences",
	KeyRowStopped:      "Chargement de compétences arrêté",
	KeyRowInstructions: "Instructions",
	KeyRowInspect:      "Inspecter",
	KeyMenuUserOnly:    "utilisateur uniquement",
}

// de 是 Deutsch（官方 `locales.ts:70-79`）。
var de = SkillDict{
	KeyRowTitle:        "Geschicklichkeit",
	KeyRowRunning:      "Ladefähigkeit",
	KeyRowPreparing:    "Das Laden eines Skills wird vorbereitet",
	KeyRowFailed:       "Das Laden des Skills ist fehlgeschlagen",
	KeyRowStopped:      "Das Laden der Fertigkeiten wurde gestoppt",
	KeyRowInstructions: "Anweisungen",
	KeyRowInspect:      "Inspizieren",
	KeyMenuUserOnly:    "Nur für Benutzer",
}

// ru 是 Русский（官方 `locales.ts:82-91`）。
var ru = SkillDict{
	KeyRowTitle:        "Навык",
	KeyRowRunning:      "Навык загрузки",
	KeyRowPreparing:    "Подготовка к загрузке навыка",
	KeyRowFailed:       "Не удалось загрузить навык",
	KeyRowStopped:      "Загрузка навыков остановлена",
	KeyRowInstructions: "Инструкции",
	KeyRowInspect:      "Осмотреть",
	KeyMenuUserOnly:    "только для пользователей",
}

// pt 是 Português（官方 `locales.ts:94-103`）。
var pt = SkillDict{
	KeyRowTitle:        "Habilidade",
	KeyRowRunning:      "Habilidade de carregamento",
	KeyRowPreparing:    "Preparando-se para carregar uma habilidade",
	KeyRowFailed:       "Falha no carregamento da habilidade",
	KeyRowStopped:      "Carga de habilidade interrompida",
	KeyRowInstructions: "Instruções",
	KeyRowInspect:      "Inspecionar",
	KeyMenuUserOnly:    "somente usuário",
}

// officialSkillDictionaries 是官方 `ctx.locale.register(NS, …)` 的全部 8 条，
// 键即语言码（BCP 47 的短形式，与官方 `locale/*.json` 文件名一一对应）。
var officialSkillDictionaries = map[string]SkillDict{
	"zh": zh,
	"en": en,
	"ja": ja,
	"es": es,
	"fr": fr,
	"de": de,
	"ru": ru,
	"pt": pt,
}

// SkillText 取一条文案，走官方的回退阶梯：
//
//	目标语言有该键   → 用它
//	目标语言没有     → 退回 zh（官方 `ctx.locale.bind(NS)` 的字典缺键回退）
//	连 zh 都没有     → 返回键名本身（**不返回空串**：空串会让整行消失，
//	                   而缺文案是缺陷、应该看得见）
//
// lang 允许 “（空 = zh）与 `zh-CN` / `en-US` 这类带地区的标签（按 `-` 取主语言）。
func SkillText(lang string, key SkillKey) string {
	primary := lang
	if i := indexByte(primary, '-'); i >= 0 {
		primary = primary[:i]
	}
	if primary == "" {
		primary = "zh"
	}
	if dict, ok := officialSkillDictionaries[primary]; ok {
		if text, ok := dict[key]; ok {
			return text
		}
	}
	if text, ok := zh[key]; ok {
		return text
	}
	return key
}

// indexByte 是 `strings.IndexByte` 的本地等价（避免为一个字符引一个包进来，
// 本包其余地方也不需要 strings）。
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
