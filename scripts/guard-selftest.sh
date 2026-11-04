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

# 恢复副本里的某个文件（每条用例之后都要复位，否则后一条会踩前一条的改动）
restore() { (cd "$root" && git archive HEAD "$1") | tar -x -C "$iso"; }

# expect_red <用例名> <测试名> —— 在副本里跑一个测试，**要求它失败**
expect_red() {
  local name="$1" testname="$2"
  local out rc
  out=$(cd "$iso" && CGO_ENABLED=0 go test ./... -run "$testname" -count=1 2>&1)
  rc=$?
  if [ $rc -eq 0 ]; then
    chk "$name：弄坏之后守卫**仍然绿**" 1 "守卫没达到弄坏必红（$testname 没跑或判据失效）"
    return 1
  fi
  if ! echo "$out" | grep -qE "^(FAIL|--- FAIL)"; then
    chk "$name：失败但不是守卫报的" 1 "$(echo "$out" | tail -2 | tr '\n' ' ')"
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
echo "== 对照组：副本原样时四组守卫都必须是绿的 =="
# 没有这一段，把四个守卫全改成"永远失败"也能过上面四条——那就从"假绿"翻到"假红"。
out=$(cd "$iso" && CGO_ENABLED=0 go test ./... \
  -run 'TestEveryHandledKeyIsDocumented|TestNoTwoPanelsShareOneSlotAndOrder|TestSnapshotCoversEveryCalledMethod|TestEveryUsedActionIsWired' \
  -count=1 2>&1)
chk "原样副本上四组守卫全绿" "$?" "$(echo "$out" | grep -E '^(FAIL|--- FAIL)' | head -2 | tr '\n' ' ')"

echo
echo "----- 合计 OK=$ok FAIL=$bad"
[ "$bad" -eq 0 ] || exit 1
