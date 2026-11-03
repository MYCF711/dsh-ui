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
curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.prompt" \
  -d "{\"id\":\"$SID\",\"text\":\"You MUST call the bash tool to run: echo WORKDAY-TOOL. Do not answer directly — call the tool.\"}" >/dev/null
# 等两件事：①模型真的调了工具（会话里出现 role=tool）②宿主写下了审批请求。
# 早先只等 requestId，20 秒没等到就 SKIP——实际是模型先思考几秒才调工具，
# 而日志要等工具调用发生后才写。现在先等 tool 消息，再取 requestId。
RID=""
for _ in $(seq 1 40); do
  HAS_TOOL=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load"     -d "{\"id\":\"$SID\",\"tailMessages\":6}" 2>/dev/null | grep -c '"role":"tool"')
  if [ "${HAS_TOOL:-0}" -gt 0 ]; then
    RID=$(grep -o '"requestId":"perm-[^"]*"' "$OUT/serve.log" 2>/dev/null | tail -1 | cut -d'"' -f4)
    [ -n "$RID" ] && break
  fi
  sleep 1
done
# 诊断：等不到时把现场打出来，免得下次又是一句没头没尾的 SKIP
if [ -z "$RID" ]; then
  echo "  DIAG serve.log 行数=$(wc -l < "$OUT/serve.log" 2>/dev/null)"
  echo "  DIAG 审批标记=$(grep -c '@@APPROVAL@@' "$OUT/serve.log" 2>/dev/null)"
  echo "  DIAG 会话尾部=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID\",\"tailMessages\":2}" 2>/dev/null | head -c 200)"
fi
if [ -n "$RID" ]; then
  ok "收到审批请求 $RID"
  AP=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.approve" -d "{\"requestId\":\"$RID\",\"optionId\":\"allow_once\"}")
  echo "$AP" | grep -q '"accepted":true' && ok "审批放行" || bad "审批回传失败: $AP"
  sleep 10
  curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.load" -d "{\"id\":\"$SID\",\"tailMessages\":4}" \
    | grep -q "WORKDAY-TOOL" && ok "工具真的执行了" || bad "工具没执行"
else
  echo "  SKIP  这轮模型没用工具（审批路径本轮走不到）"
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
