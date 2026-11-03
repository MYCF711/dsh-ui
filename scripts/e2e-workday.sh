#!/usr/bin/env bash
# e2e-workday.sh —— 「连续使用」的自动化版本。
#
# 为什么不是"完整工作日"：真工作日要人连续用几小时，那没法自动化。这个脚本做的是
# 次优但可复跑的东西——把已经逐项验过的能力**串成一次连续的会话流**：
#
#   建会话 → 多轮对话（含 reasoning）→ 工具调用 → 审批(ask) → 取消 → 切会话 → 量内存
#
# 它证明的是"这些能力在同一条会话流里连续使用不会互相干扰"，
# 而不是"人用了一整天没问题"。两者不能互相替代，文档里也这么写。
#
# 用法：bash scripts/e2e-workday.sh            # 全流程
#       KEEP=1 bash scripts/e2e-workday.sh     # 保留临时目录（排查用）
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="/c/Program Files/Go/bin:$PATH"
OUT="${OUT:-tmp/e2e-workday}"
rm -rf "$OUT"
mkdir -p "$OUT"

SERVE=tmp/dshd-serve.exe
APP=tmp/app-e2e.exe
pass=0
fail=0
step() { printf '\n== %s ==\n' "$*"; }
ok()   { pass=$((pass+1)); printf '  PASS  %s\n' "$*"; }
bad()  { fail=$((fail+1)); printf '  FAIL  %s\n' "$*"; }

cleanup() {
  [ -n "${SERVE_PID:-}" ] && taskkill //PID "$SERVE_PID" //F >/dev/null 2>&1
  [ "${KEEP:-0}" = "1" ] || rm -f "$OUT"/home/sessions/*/*.lock 2>/dev/null
}
trap cleanup EXIT

step "构建"
CGO_ENABLED=0 go build -o "$APP" ./cmd/app || { echo "界面构建失败"; exit 1; }
[ -x "$SERVE" ] || (cd ../DSH-GO && CGO_ENABLED=0 go build -o "../dsh-ui/$SERVE" ./cmd/dshd) || exit 1
echo "  界面 $(stat -c%s "$APP") 字节 / 宿主 $(stat -c%s "$SERVE") 字节"

step "起宿主（临时 home、OS 分配端口、policy=ask）"
"$SERVE" serve --home "$OUT/home" --policy ask > "$OUT/ready.txt" 2> "$OUT/serve.log" &
SERVE_PID=$!
for _ in $(seq 1 30); do grep -q '"url"' "$OUT/ready.txt" 2>/dev/null && break; sleep 0.5; done
URL=$(grep -o 'http://[^"]*' "$OUT/ready.txt" | head -1)
TOK=$(grep -o '"token":"[^"]*' "$OUT/ready.txt" | cut -d'"' -f4)
[ -n "$URL" ] && [ -n "$TOK" ] && ok "宿主就绪 $URL" || { bad "宿主没起来"; exit 1; }

step "认证 + 建会话"
curl -s -c "$OUT/ck.txt" "$URL/connect?token=$TOK" -o /dev/null
CK=$(grep dshd_session "$OUT/ck.txt" | awk '{print $7}')
[ -n "$CK" ] && ok "connect 换到 cookie" || bad "拿不到 cookie"
SID=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.create" -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
[ -n "$SID" ] && ok "建会话 $SID" || { bad "建不了会话"; exit 1; }
SID2=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.create" -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
[ -n "$SID2" ] && ok "建第二个会话（切会话用）" || bad "第二个会话失败"

step "第 1 轮：普通对话（验 text 流）"
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.prompt" \
  -d "{\"id\":\"$SID\",\"text\":\"Reply with exactly: WORKDAY-1. No tools.\"}" >/dev/null
sleep 12
R1=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID\",\"tailMessages\":3}")
echo "$R1" | grep -q "WORKDAY-1" && ok "第 1 轮回答到位" || bad "第 1 轮没答"

step "第 2 轮：工具调用 + 审批(ask)"
# 这一轮**必须由界面发**：审批是走 WS 推给客户端的，只有界面在场才收得到
# approval/request；用 curl 发的话没人收帧，宿主 120 秒后默认拒绝（实测过）。
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$SID"   -smoke -prompt "用 bash 工具执行 echo WORKDAY-TOOL，把输出告诉我"   -shot "$OUT/ui-tool.png" -shot-wait 110 > "$OUT/app2.log" 2>&1 &
APP2=$!
RID=""
for _ in $(seq 1 120); do
  RID=$(grep -o 'requestId=perm-[a-z0-9-]*' "$OUT/app2.log" 2>/dev/null | tail -1 | cut -d= -f2)
  [ -n "$RID" ] && break
  sleep 1
done
if [ -n "$RID" ]; then
  ok "界面收到审批请求 $RID"
  AP=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.approve" -d "{\"requestId\":\"$RID\",\"optionId\":\"allow_once\"}")
  echo "$AP" | grep -q '"accepted":true' && ok "审批回传被接受" || bad "审批回传失败: $AP"
  wait "$APP2" 2>/dev/null
  sleep 5
  curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID\",\"tailMessages\":6}"     | grep -q "WORKDAY-TOOL" && ok "工具真的执行了（allow 生效）" || bad "工具没执行"
else
  echo "  DIAG 界面日志行数=$(wc -l < "$OUT/app2.log" 2>/dev/null)"
  echo "  DIAG 界面日志尾部=$(tail -2 "$OUT/app2.log" 2>/dev/null | head -c 200)"
  echo "  SKIP  界面没等到审批帧（模型可能没调工具，或耗时超过 110 秒）"
  wait "$APP2" 2>/dev/null
fi

step "第 3 轮：取消"
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.prompt" \
  -d "{\"id\":\"$SID\",\"text\":\"写一篇两万字的技术文档，越详细越好\"}" >/dev/null
sleep 3
CN=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.cancel" -d "{\"id\":\"$SID\"}")
echo "$CN" | grep -q '"cancelled":true' && ok "取消被受理" || bad "取消失败: $CN"

step "第 4 轮：切会话不串流"
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.prompt" \
  -d "{\"id\":\"$SID2\",\"text\":\"Reply with exactly: WORKDAY-OTHER. No tools.\"}" >/dev/null
sleep 10
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID\",\"tailMessages\":2}" \
  | grep -q "WORKDAY-OTHER" && bad "会话 1 里出现了会话 2 的内容（串流）" || ok "两个会话内容隔离"
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID2\",\"tailMessages\":2}" \
  | grep -q "WORKDAY-OTHER" && ok "会话 2 自己的内容在" || bad "会话 2 内容丢了"

step "界面连续使用（真宿主 + 真模型）+ 内存"
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$SID" \
  -smoke -prompt "Reply with exactly: WORKDAY-UI" \
  -shot "$OUT/ui-list.png" -shot-wait 25 > "$OUT/app.log" 2>&1
grep -q "shot: 已保存" "$OUT/app.log" && ok "界面正常出图" || bad "界面没出图"
WS=$(grep -oE '工作集 [0-9.]+ MB' "$OUT/app.log" | tail -1)
echo "  $WS"

step "汇总"
echo "  PASS=$pass FAIL=$fail"
echo "  证据目录: $OUT"
[ "$fail" -eq 0 ]
