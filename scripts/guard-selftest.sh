#!/usr/bin/env bash
# guard-selftest.sh —— 每条守卫都要能"弄坏必红"。
#
# # 为什么
#
# 这个仓的规矩是"**守卫必须可反证**"。但反证是**人做的事**：写守卫时手工改一次、
# 看它红、再改回来。人一旦省了这一步，守卫就退化成一句安慰——
# 而它照样绿，谁也不知道它其实什么都没看（第 74 轮我真撞到过：那条 Actions 守卫
# 第一版只认 `*ast.Ident`，漏掉了仓里最常见的 `env.Act.X`，只扫到一半字段，
# **拆掉接线它也不红**）。
#
# 所以这条脚本把"弄坏"自动化：在 `git archive HEAD` 出来的**隔离副本**里，
# 每条守卫各弄坏一处，要求它当场红。共享工作树一个字节都不动。
#
# # 为什么在副本里
#
# 弄坏要改源码。在共享树里改会让并行会话看到一个中途状态——这正是本仓
# 反复吃亏的地方（`check.sh` 原地 `mv modules` 同源）。副本里改，改完删掉。
#
# 用法：bash scripts/guard-selftest.sh
#       KEEP=1 bash scripts/guard-selftest.sh   # 留着副本，排查用

set -uo pipefail
cd "$(dirname "$0")/.." || exit 2
root=$(pwd)
export PATH="/c/Program Files/Go/bin:$PATH"

ok=0
bad=0
chk() { if [ "$2" -eq 0 ]; then echo "  PASS  $1"; ok=$((ok + 1)); else echo "  FAIL  $1  ${3:-}"; bad=$((bad + 1)); fi; }

# 副本放在**兄弟目录**：`go.mod` 里的相对 replace 才能解析（和 headcheck 同一理由）。
iso="$root/../dsh-ui-guardselftest.$$"
rm -rf "$iso"
mkdir -p "$iso"
if ! git archive HEAD | tar -x -C "$iso"; then
  echo "解不出 HEAD"
  exit 2
fi
cleanup() { [ "${KEEP:-0}" = "1" ] || rm -rf "$iso"; }
trap cleanup EXIT
echo "== 隔离副本：$iso =="
# ⚠️ 这个副本来自 `git archive HEAD`——**验的是提交里的守卫，不是工作树里的**。
# 所以新写的守卫必须先提交，才能被它验到；否则 `-run` 一条都匹配不到，
# 而 `go test` 在零匹配时仍然 exit 0（这条自检的"跑没跑到"判据就是为这个加的）。


# 恢复副本里的某个文件（每条用例之后都要复位，否则后一条会踩前一条的改动）
restore() { (cd "$root" && git archive HEAD "$1") | tar -x -C "$iso"; }

# expect_red <用例名> <测试名> —— 在副本里跑一个测试，**要求它失败**
# ran_a_test <输出> <测试名> —— 输出里必须真的出现过 `=== RUN <测试名>`。
#
# 为什么需要这一步：`go test -run <名字>` 在**一条都没匹配到**时仍然 **exit 0**。
# 于是"名字写错/匹配不到"和"跑了且绿"在退出码上分不出来——对照组会空转着绿。
ran_a_test() {
  printf '%s' "$1" | grep -qE "^=== RUN[[:space:]]+$2(/|$)"
}

# expect_red <用例名> <测试名> —— 在副本里跑一个测试，**要求它真的跑到、并且失败**
expect_red() {
  local name="$1" testname="$2"
  local out rc
  # -v 是为了拿到 `=== RUN`；没有它，"没跑到"和"跑了且绿"在输出上分不出来。
  out=$(cd "$iso" && CGO_ENABLED=0 go test ./... -run "$testname" -count=1 -v 2>&1)
  rc=$?
  if ! ran_a_test "$out" "$testname"; then
    chk "$name：用例根本没跑到" 1 "输出里没有『=== RUN $testname』——-run 没匹配到，这个用例是空转的。""如果这条守卫是刚写的还没提交：**先提交它**（本脚本验的是 HEAD，不是工作树），再跑这条自检"
    printf '%s\n' "$out" | tail -3 | sed 's/^/        /'
    return 1
  fi
  if [ $rc -eq 0 ]; then
    chk "$name：弄坏之后守卫仍然绿" 1 "守卫没达到弄坏必红（$testname 跑了，但没报错）"
    return 1
  fi
  if ! printf '%s\n' "$out" | grep -qE "^(FAIL|--- FAIL)"; then
    chk "$name：失败但不是守卫报的" 1 "$(printf '%s\n' "$out" | tail -2 | tr '\n' ' ')"
    return 1
  fi
  chk "$name：弄坏 → 守卫红" 0
  return 0
}

echo
echo "== 1. 键位表守卫（F1 表漏一个真在用的键） =="
python - "$iso" <<'PY'
import io, sys, re
p = sys.argv[1] + '/plugins/dsshortcuts/shortcuts.go'
s = io.open(p, encoding='utf-8').read()
# 删掉 F5 那一行（F5 在 dsterminal 里真的被处理）
s2 = re.sub(r'\n\t\{"F5",[^\n]*\n', '\n', s, count=1)
if s2 == s:
    sys.exit('找不到 F5 那一行，用例失效')
io.open(p, 'w', encoding='utf-8', newline='').write(s2)
PY
expect_red "键位表漏记 F5" "TestEveryHandledKeyIsDocumented"
restore plugins/dsshortcuts/shortcuts.go

echo
echo "== 2. 槽位 Order 守卫（命令面板和交付物面板撞回 96） =="
python - "$iso" <<'PY'
import io, sys
p = sys.argv[1] + '/plugins/dscommands/commands.go'
s = io.open(p, encoding='utf-8').read()
old = '\t\tOrder: 99,\n'
if s.count(old) != 1:
    sys.exit('找不到 Order: 99（守卫的判据是不是改了？）')
io.open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, '\t\tOrder: 96,\n'))
PY
expect_red "两个面板撞 Order 96" "TestNoTwoPanelsShareOneSlotAndOrder"
restore plugins/dscommands/commands.go

echo
echo "== 3. 契约快照守卫（客户端在调的方法从快照里删掉） =="
python - "$iso" <<'PY'
import io, json, sys
p = sys.argv[1] + '/testdata/api.snapshot.json'
d = json.load(io.open(p, encoding='utf-8'))
if 'session.approve' not in d.get('methods', {}):
    sys.exit('快照里没有 session.approve，用例失效')
del d['methods']['session.approve']
io.open(p, 'w', encoding='utf-8', newline='').write(json.dumps(d, ensure_ascii=False, indent=2) + '\n')
PY
expect_red "快照漏了 session.approve" "TestSnapshotCoversEveryCalledMethod"
restore testdata/api.snapshot.json

echo
echo "== 4. Action 接线守卫（拆掉 OpenFile 的接线） =="
python - "$iso" <<'PY'
import io, sys, re
p = sys.argv[1] + '/cmd/app/main.go'
s = io.open(p, encoding='utf-8').read()
s2 = re.sub(r'\n\t\tOpenFile: +m\.openFile,\n', '\n', s, count=1)
if s2 == s:
    sys.exit('找不到 OpenFile 接线，用例失效')
io.open(p, 'w', encoding='utf-8', newline='').write(s2)
PY
expect_red "OpenFile 没接线" "TestEveryUsedActionIsWired"
restore cmd/app/main.go

echo
echo "== 5. 插件装配守卫（目录在、import 没写） =="
# 挑一个**空导入**（`_ "…/plugins/dsgoal"`）：删掉它不会让编译失败，
# 所以这里红的必须是守卫本身，而不是"编译不过"。
python - "$iso" <<'PY'
import io, sys, re
p = sys.argv[1] + '/cmd/app/main.go'
s = io.open(p, encoding='utf-8').read()
s2 = re.sub(r'\n\t_ "github\.com/MYCF711/dsh-ui/plugins/dsgoal"\n', '\n', s, count=1)
if s2 == s:
    sys.exit('找不到 dsgoal 的空导入，用例失效')
io.open(p, 'w', encoding='utf-8', newline='').write(s2)
PY
expect_red "插件没 import" "TestEveryPluginDirIsImported"
restore cmd/app/main.go

echo
echo "== 6. VERSIONINFO 守卫（brand.go 改了、入库的 .syso 没跟着重新生成） =="
# 这是本仓真撞过的形态：改名之后 build/链接/运行/单测全绿，只有
# `(Get-Item exe).VersionInfo` 里还是旧名字 —— 而"任务管理器描述列"正是它的唯一用途。
python - "$iso" <<'PY'
import io, sys
p = sys.argv[1] + '/cmd/app/brand.go'
s = io.open(p, encoding='utf-8').read()
import re
s2, n = re.subn(r'const productName = "[^"]*"', 'const productName = "DSH GUI GO 改过了"', s, count=1)
if n != 1:
    sys.exit('找不到 productName 常量，用例失效')
io.open(p, 'w', encoding='utf-8', newline='').write(s2)
PY
expect_red "改了产品名但没重生成 .syso" "TestVersionInfoResourceMatchesBrand"
restore cmd/app/brand.go

echo
echo "== 7. 视觉区域契约（区域矩形与骨架布局分叉） =="
# 这是 V-06-5 的像素判据的**区域定义**：`testdata/visual-v06/regions.json`
# 里的矩形必须逐像素等于 `uikernel.ComputeShellLayout` 算出来的容器。
# 弄坏的形态是"侧栏宽度 280 → 281"——一个看起来无害的 1px，
# 但它会让截图比对的区域整体偏 1 列，而脚本照样报 PASS（比错了部件）。
#
# ⚠ 本脚本验的是 `git archive HEAD`（**提交里的**守卫）。V-06-5 的新文件
# 还没提交时这条用例在副本里没有输入，**跳过并说明**——绝不静默绿
# （静默绿等于这条守卫不存在）。
if ! git ls-tree -r --name-only HEAD -- testdata/visual-v06/regions.json | grep -q .; then
  echo "  NOTE  跳过：HEAD 里还没有 testdata/visual-v06/regions.json（V-06-5 未提交）；提交后这条自动生效"
else
  python - "$iso" <<'PY'
import io, json, sys
p = sys.argv[1] + '/testdata/visual-v06/regions.json'
d = json.load(io.open(p, encoding='utf-8'))
regions = d['region_sets']['default']
for region in regions:
    if region['name'] == 'sidebar':
        if region['rect'] != [0, 0, 280, 832]:
            sys.exit('sidebar 矩形不是 [0,0,280,832]（用例失效）')
        region['rect'] = [0, 0, 281, 832]
        break
else:
    sys.exit('找不到 sidebar 区域，用例失效')
io.open(p, 'w', encoding='utf-8', newline='').write(
    json.dumps(d, ensure_ascii=False, indent=2) + '\n')
PY
  expect_red "区域矩形与骨架布局分叉" "TestVisualRegionsMatchShellLayout"
  restore testdata/visual-v06/regions.json
fi

echo
echo "== 对照组：副本原样时各组守卫都必须是绿的 =="
# 没有这一段，把守卫全改成"永远失败"也能过上面几条——那就从"假绿"翻到"假红"。
GUARDS='TestEveryHandledKeyIsDocumented|TestNoTwoPanelsShareOneSlotAndOrder|TestSnapshotCoversEveryCalledMethod|TestEveryUsedActionIsWired|TestEveryPluginDirIsImported|TestVersionInfoResourceMatchesBrand'
GUARD_LIST='TestEveryHandledKeyIsDocumented TestNoTwoPanelsShareOneSlotAndOrder TestSnapshotCoversEveryCalledMethod TestEveryUsedActionIsWired TestEveryPluginDirIsImported TestVersionInfoResourceMatchesBrand'
# 视觉区域契约只在**提交里有输入**时进对照组（见上：未提交时那条用例没有输入）。
if git ls-tree -r --name-only HEAD -- testdata/visual-v06/regions.json | grep -q .; then
  GUARDS="$GUARDS|TestVisualRegionsMatchShellLayout"
  GUARD_LIST="$GUARD_LIST TestVisualRegionsMatchShellLayout"
fi
out=$(cd "$iso" && CGO_ENABLED=0 go test ./... -run "$GUARDS" -count=1 -v 2>&1)
rc=$?
# 先确认每条**都真的跑了**——一条都没跑到时 `go test` 也是 exit 0。
missing=""
for g in $GUARD_LIST; do
  ran_a_test "$out" "$g" || missing="$missing $g"
done
count=$(printf '%s\n' "$GUARD_LIST" | wc -w)
if [ -n "$missing" ]; then
  chk "对照组：$count 条守卫都跑到" 1 "没跑到：$missing"
else
  chk "对照组：$count 条守卫都跑到" 0
fi
chk "原样副本上各组守卫全绿" "$rc" "$(printf '%s\n' "$out" | grep -E '^(FAIL|--- FAIL)' | head -2 | tr '\n' ' ')"

echo
echo "----- 合计 OK=$ok FAIL=$bad"
[ "$bad" -eq 0 ] || exit 1
