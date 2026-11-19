# Viva 用 replace 指向自家 fork（携带 vivaui/viva#355）

> 账本条目：task-114。**当前状态：replace 生效中，不是临时补丁。**
> 本文件回答三件事：为什么有这条 replace、它现在证明得了什么、以及**上游合了之后怎么退回**。

---

## 1. 为什么用 replace 而不是等 PR 合并

用户 2026-11-19 裁定：

> **「不是已经做了 pr 吗？我不是说过 pr 不用等合并，先用吗」**

即本项目既有规则：**PR 不合并卡不住我们**——先用自己的 fork 分支推进；上游长期不收，就把内容纳入自家依赖（另开 vendoring 单）。

对应 PR：[`vivaui/viva#355`](https://github.com/vivaui/viva/pull/355)（OPEN，0 review）
分支：`MYCF711/viva` 的 `feat/menu-shadow-and-row-height`，HEAD `5f703bf`（已 merge 最新 `origin/main`）

## 2. `go.mod` 里现在是什么

```text
replace github.com/vivaui/viva => github.com/MYCF711/viva v0.6.6-0.20261119123539-854dcf475230
```

`go list -m` 的实测输出：

```text
github.com/vivaui/viva v0.6.5 => github.com/MYCF711/viva v0.6.6-0.20261119123539-854dcf475230
```

### 为什么是伪版本（而不是 `go get <分支名>`）

`go get github.com/MYCF711/viva@feat/menu-shadow-and-row-height` 直接失败：

```text
go: github.com/MYCF711/viva@feat/menu-shadow-and-row-height: invalid version: disallowed version string
```

`go get` 不接受带斜杠的分支名。改按伪版本取时，走 git 拉 `MYCF711/viva` 又因本机网络栈失败
（`fatal: fetch-pack: invalid index-pack output`，同一错误在 `git clone` 上可复现，与模块系统无关）。

**取到的伪版本是可复现的**：`v0.6.6-0.20261119123539-854dcf475230` 的形状是
`v<前一 tag 的下一个补丁>-0.<commit 的 UTC 时间>-<12 位 commit 前缀>`，`go.sum` 里两行哈希
（`h1:` 与 `/go.mod h1:`）钉住模块内容。CI 只要能取到 `MYCF711/viva` 的同名提交，
校验结果与本机一致——**不是本地路径，CI 可用**。

### 伪版本对应的内容有一处必须知道的改写

伪版本指向的提交在 fork 分支 `5f703bf` 之上**只有一个改动：`go.mod` 的 `module` 行**
（`github.com/vivaui/viva` → `github.com/MYCF711/viva`）。除这一行外，源码与 `5f703bf` 逐字节相同。

- 做这个改写的唯一原因：Go 要求被 `replace` 到的模块自己声明的 module path 与 replace 右值一致，
  否则报 `module declares its path as: github.com/vivaui/viva but was required as: github.com/MYCF711/viva`。
- **不是 vendoring**：没有任何 fork 源码被复制进本仓。
- 内部 import（`github.com/vivaui/viva/...`）**没有改**，由同一条 replace 解析回该模块自身。

## 3. 新字段用上了什么、怎么证明

`vivaui/viva#355` 给三个菜单形状（dropdown / context menu / menubar popup）的 `*Options`
加了两个字段：

| 字段 | 类型 | 位置（fork） |
|---|---|---|
| `Shadow` | `*gfx.ShadowLevel` | `components/dropdown_menu.go:164-170` |
| `RowHeight` | `float32` | `components/dropdown_menu.go:158-163` |

> 注意：`RowHeight` 在 Viva v0.6.5 的 `ui.MenuItems` 上**已有**（`ui/menu.go:89`），
> #355 把它补到 `components` 层的三个 `*Options` 结构上；**`Shadow` 是真正新增的那个**。

**证据（不是只编译过）**：`plugins/dscore/viva_fork_menu_surface_test.go`
`TestVivaForkDropdownMenuExposesShadowAndRowHeight`，5 个子测试：

1. `shadow overrides the default` —— 菜单面铺出的阴影逐层等于同 theme/radius/bounds 下
   `gfx.Canvas.Elevate(..., ShadowLG)` 的结果，**并且不等于** `ShadowMD` 那档
   （反向判据：字段被接受但没生效时会红）
2. `nil shadow keeps shadow-md` —— 不传时仍是 `ShadowMD`（默认值没被破坏）
3. `shadow wins over the submenu escalation` —— 显式档压过子菜单的 `ShadowLG` 升级
4. `row height overrides the theme` —— 行高真的变 24（默认是主题 control height 32）
5. `row height zero keeps the theme` —— 传 0 时回落到主题值

**变异实测**（证明断言有牙）：① 把期望档改成 `ShadowMD` → 5 处 FAIL；
② 去掉 `RowHeight: 24` → `row = {…Height:32}, want 24 high`。恢复后全绿。

## 4. 用上它的地方：`#64` 的 ⑧ 菜单面阴影

`#64` ⑧ 的原状：菜单面用 Viva 的 `gfx.ShadowMD` 预设档，而官方是
`--dsw-elevation-prominent`。**本轮修了两处，第三处卡在 `ui` 层没有透传**。

### 已修（本仓内，走官方令牌）

| 落点 | 改动 |
|---|---|
| `internal/uikernel/official_model_menu.go:269` | `Shadow(gfx.ShadowMD)` → `Draw` 钩子 + `dscore.PaintElevation(d, t.Radii.MD, t.Shadows.Prominent, t.BorderL1)` |
| `internal/uikernel/entry_menu.go:155` | 同上 |

官方依据在这两个文件头本来就写着（`ModelSelect.module.css:107-135` 的 primitives
MenuSurface = padding 4 / radius md / **elevation-prominent**），只是代码用了预设档。

数值差别（`design-platform.css` 的 `gradient-shadow-text.css:36-37`）：

| | 第一层 | 第二层 | 描边 |
|---|---|---|---|
| 官方 prominent | `0 3px 8px .04` | `0 0 20px .05` | 0.5px |
| `gfx.ShadowMD` | `0 4px 6px -1px .10` | `0 2px 4px -2px .10` | 无 |

**证据**：`internal/uikernel/official_model_menu_shadow_test.go` 与
`internal/uikernel/entry_menu_shadow_test.go`，判据双向——① 画布上出现 prominent 的两层；
② 不出现 shadow-md 的两层（负扩展是关键特征）。变异实测：改回 `ShadowMD` → 红；
把 `Prominent` 换成 `Panel`（blur 16 vs 20）→ 红。

### 仍卡上游（`ui.Menu` 这条路径）

`plugins/dsforge/changes.go` 的 `drawScopeMenu` 用 `ui.Menu`，这条路径**拿不到** `Shadow`：

- `ui/menu_paint.go:120-125` 构造 `DropdownMenuOptions` 时不传 `Shadow`；
- `ui.MenuItems` 只有 `RowHeight`（`base/menu.go:89`），**没有** `Shadow` 方法；
- `#355` 只做到 `components` 层的三个 `*Options`。

试过的解法与为什么放弃：`MenuItems.Unpainted()` + `DrawPanel` 能换面板阴影，但
`Unpainted` **同时关掉行的绘制**（`base/menu.go:293-296`），要用就得用 `DrawItem`
把 Viva 的整行渲染（文字/勾选/快捷键/分隔线/悬停底）在本仓重抄——
那是把上游渲染逻辑复制进本仓，比差一档阴影贵得多，且必然走样
（`TestScopeMenuIsAnOverlayWithOfficialOrder` 当场抓到条目消失）。

**解除条件**：`ui.MenuItems` 有 `Shadow` 透传后，在 `drawScopeMenu` 里一行接上。

### 一处必须说清的边界

uikernel 那两处用的是 `ui.Box` 的 `Draw` 钩子，**不是** `DropdownMenuOptions.Shadow`。
原因：`DropdownMenuOptions.Shadow` 只接受 `gfx.ShadowLevel` 的 5 档枚举
（`None/XS/SM/MD/LG`），官方 prominent 的位移/模糊/透明度和「无描边」在枚举里没有对应档——
用 `ShadowLG` 替 `ShadowMD` 只会**更偏离**官方（`0 10px 15px -3px` vs `0 3px 8px`）。

所以本任务里两件事各司其职：**`Shadow`/`RowHeight` 字段用于证明 fork 依赖可消费、行为正确并钉住它**（§3）；**画面用能精确表达官方档的令牌路径**（本节）。

## 5. 退回上游的步骤（#355 合并之后）

按顺序做，每步都能停：

```bash
# ① 确认上游已合并，且拿得到含该字段的 tag
git -C ../viva fetch origin --tags
git -C ../viva tag -l --sort=-v:refname | head -3     # 找到 >= 收录 #355 的那个 v0.6.x

# ② 本地先验：该 tag 的 dropdown_menu.go 真的有 Shadow
grep -n "Shadow \*gfx.ShadowLevel" ../viva/components/dropdown_menu.go

# ③ 删掉 go.mod 末尾的注释块 + replace 行（整段删除，两行都要删）
#    replace github.com/vivaui/viva => github.com/MYCF711/viva v0.6.6-0.20261119123539-854dcf475230

# ④ 升到官方版本并收敛
go get github.com/vivaui/viva@v0.6.x
go mod tidy

# ⑤ 一起跑（本仓门禁）
python scripts/parity-count.py --check
CGO_ENABLED=0 go build ./...
go vet ./...
CGO_ENABLED=0 go test ./... -count=1
test -z "$(gofmt -l cmd internal plugins pluginapi)"
```

**⑤ 里要特别看的两个测试**（它们直接依赖 fork 字段）：

- `plugins/dscore/viva_fork_menu_surface_test.go` —— 上游版本**已含** `Shadow`/`RowHeight` 时它继续绿；
  **不含**时它编译失败。编译失败是**期望信号**：先确认版本，再决定是删本测试还是等下一个 tag。
- `internal/uikernel/*_shadow_test.go` —— 与 replace 无关（走官方令牌），会一直绿。

**⑥ 删除本文件与 go.mod 注释**，并在 `docs/official-parity.md` 的 #64 行把 ⑧ 的记法从
「卡上游」改成「已闭合（上游版本 X）」。