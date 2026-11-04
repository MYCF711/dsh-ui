#!/usr/bin/env bash
# accept-all.sh —— **一条命令跑完整产品验收**，最后给一张表。
#
# # 为什么要有它
#
# 这个仓的验收是一条条脚本（审批 / 取消 / 续历史 / 整链 / 会话操作 / 主题 / 工作日）。
# 它们各自都能单独跑，但**没有一次"全都跑一遍"的固定入口**，于是：
#
#   - 谁也不会在改完一处之后把七条都跑一遍（十几分钟）；
#   - 单独跑绿不等于一起跑绿：端口、临时 home、被上一个脚本遗留的进程，
#     这些都是**只在连跑时才出现**的干扰；
#   - `AGENTS.md` §0.6 要的"真实终验"是一串动作，但没人把它写成一条命令。
#
# # 判据只有一条：**退出码**
#
# 每条脚本自己的输出格式不一样（`OK=n FAIL=m` / `PASS=n FAIL=m` / `ALL GREEN`），
# 去解析那些字符串只会引入一类新的脆弱。统一用**退出码**当判据，
# 另外把每条的最后几行原样贴出来——人要看细节时不用再翻日志。
#
# 用法：
#   bash scripts/accept-all.sh                  # 全跑（十几分钟）
#   bash scripts/accept-all.sh --only theme,session-ops
#   bash scripts/accept-all.sh --fast           # 跳过要真模型的两条（e2e / 三个 turn 脚本）
#   KEEP=1 bash scripts/accept-all.sh           # 保留每条脚本自己的证据目录
#
# 前置：模型网关要通（除了 theme）；不通时相关脚本会自己红，这张表会如实记录。

set -uo pipefail
cd "$(dirname "$0")/.." || exit 2
export PATH="/c/Program Files/Go/bin:$PATH"

OUT="${OUT:-tmp/accept-all}"
ONLY=""
FAST=0
while [ $# -gt 0 ]; do
  case "$1" in
  --only) ONLY="$2"; shift 2 ;;
  --fast) FAST=1; shift ;;
  -h | --help) sed -n '2,30p' "$0"; exit 0 ;;
  *) echo "不认识的参数：$1"; exit 2 ;;
  esac
done

rm -rf "$OUT"
mkdir -p "$OUT"

# 名字 | 脚本 | 需要真模型(1/0)
SUITE=(
  "probe-selftest|scripts/e2e-probe-selftest.sh|0"
  "theme|scripts/accept-theme.sh|0"
  "session-ops|scripts/accept-session-ops.sh|1"
  "approval|scripts/accept-approval.sh|1"
  "cancel|scripts/accept-cancel.sh|1"
  "resume|scripts/accept-resume.sh|1"
  "chain|scripts/accept-chain.sh|1"
  "e2e-workday|scripts/e2e-workday.sh|1"
)

want() {
  [ -z "$ONLY" ] && return 0
  case ",$ONLY," in *",$1,"*) return 0 ;; esac
  return 1
}

echo "== 前置 0：把验收用的宿主二进制编出来 =="
# `accept-chain.sh` / `accept-approval.sh` / `accept-resume.sh` 用的是**共享路径**
# `tmp/dshd-acc.exe`，而它们各自都会验一次"这个二进制与源码一致"——
# 不先编，那条断言就会红（第一次合跑就是这么红的：链上 10 过 1 红）。
# 在这里编一次，全部脚本共用，也保证"绝不拿陈旧二进制跑真机"。
# 编到**本轮自己的目录**里，并通过 `DSHD_ACC` 传给各脚本。
# 共享的 `tmp/dshd-acc.exe` 会被没退出的旧宿主锁住（Windows 上就编不进去），
# 一旦锁住整个验收停在第一步——这个坑今天就撞到了（七个 6 小时前的残留宿主）。
ACC_BIN="$OUT/dshd-acc.exe"
if ! (cd ../DSH-GO && CGO_ENABLED=0 go build -o "../dsh-ui/$ACC_BIN" ./cmd/dshd); then
  echo "  FAIL  编不出 $ACC_BIN——后面的真机脚本没有意义"
  exit 1
fi
export DSHD_ACC="$ACC_BIN"
echo "  PASS  已编 $ACC_BIN（本轮专用）"

echo "== 前置：模型网关 =="
gw=$(curl.exe -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3425/v1/models 2>/dev/null)
echo "  magpie /v1/models → $gw（不是 200 时，要真模型的脚本会失败——那是环境，不是产品）"

rows=()
fail=0
skip=0
for entry in "${SUITE[@]}"; do
  IFS='|' read -r name script needs_model <<<"$entry"
  want "$name" || continue
  if [ ! -f "$script" ]; then
    rows+=("$name|缺脚本|$script")
    fail=$((fail + 1))
    continue
  fi
  if [ "$FAST" = "1" ] && [ "$needs_model" = "1" ]; then
    rows+=("$name|跳过(--fast)|需要真模型")
    skip=$((skip + 1))
    continue
  fi
  log="$OUT/$name.log"
  printf '\n===== 跑 %s（%s）=====\n' "$name" "$script"
  start=$(date +%s)
  bash "$script" >"$log" 2>&1
  rc=$?
  dur=$(($(date +%s) - start))
  # 各脚本的汇总行格式不同；抓最后一条看起来像汇总的，抓不到就抓最后一行。
  summary=$(grep -E '合计|PASS=|ALL GREEN|FAIL' "$log" | tail -1 | sed 's/^[[:space:]]*//' | cut -c1-70)
  [ -z "$summary" ] && summary=$(tail -1 "$log" | cut -c1-70)
  # 第二道判据：脚本自报的汇总行里 **FAIL 非零** 也算红。
  # 为什么：脚本的退出码曾有四条是无条件 `exit 0`（打了 FAIL 却对调用方说"我过了"），
  # 只信退出码就会把一次失败的验收记成通过——第一次合跑就是这么发现的。
  self_fail=0
  echo "$summary" | grep -qE 'FAIL=[1-9]' && self_fail=1
  if [ $rc -eq 0 ] && [ "$self_fail" = "0" ]; then
    rows+=("$name|PASS ${dur}s|$summary")
    printf '  → PASS（%ss）%s\n' "$dur" "$summary"
  else
    why="rc=$rc"
    [ "$self_fail" = "1" ] && why="$why，自报 FAIL"
    rows+=("$name|FAIL $why ${dur}s|$summary")
    fail=$((fail + 1))
    printf '  → FAIL（rc=%s，%ss）%s\n' "$rc" "$dur" "$summary"
    printf '     日志：%s（尾部 6 行）\n' "$log"
    tail -6 "$log" | sed 's/^/       /'
  fi
done

echo
echo "================ 汇总 ================"
printf '%-16s %-18s %s\n' "脚本" "结果" "它自己的汇总行"
printf '%-16s %-18s %s\n' "----------------" "------------------" "------------------------------------"
for r in "${rows[@]}"; do
  IFS='|' read -r n res sm <<<"$r"
  printf '%-16s %-18s %s\n' "$n" "$res" "$sm"
done
echo "======================================"
echo "FAIL=$fail SKIP=$skip  日志目录：$OUT"
[ "$fail" -eq 0 ] || exit 1
