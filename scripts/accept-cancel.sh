#!/usr/bin/env bash
# 取消整链：发一个长生成，等 turn 真的开始，取消，验 journal 出现中断收尾。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
# 宿主二进制路径可覆盖：默认是共享的 tmp/dshd-acc.exe。
# 为什么要能覆盖：那个文件会被**没退出的旧宿主**锁住（Windows），于是"编不出新二进制"，
# 整个验收直接停在第一步。`accept-all.sh` 每次编到自己那一轮的目录里再传进来。
BIN="${DSHD_ACC:-tmp/dshd-acc.exe}"
ok=0; fail=0
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

H=tmp/canc-home-$$; rm -rf "$H"
# **allow**：取消要验的是"中断"，不是审批——把审批卡在中间会让它变成又一个审批测试。
"$BIN" serve --home "$H" --policy allow > tmp/canc.txt 2> tmp/canc.log &
SRV=$!
# **退出就回收自己起的那个宿主**：正常结束、断言失败、Ctrl+C、被 TERM 都一样。
#
# 为什么必须有：这些脚本原来起了 serve 就再也没人管它。异常或 Ctrl+C 之后宿主留成孤儿，
# 下一次跑会撞两件事——① Windows 上它还锁着 exe，"编不出新二进制"，整个验收停在第一步；
# ② 它占着旧的 home，下一轮读到的可能是别人的会话。
# 2026-11-04 现场：`dshd-acc.exe` 积了 7 个 6 小时前的残留（宿主侧同一形态查出来 34 个）。
#
# 只回收 **$!** 那一个 PID，不按镜像名扫——按名字杀会连带别人的宿主。
#
# 一个要知道的延迟：bash 收到 TERM/INT 时会把 trap **推迟到当前那条命令结束**
#（一次 curl 或 sleep 的时长），所以回收可能晚几秒到几十秒（实测最长约 20 秒）。
# 真正跑不到 trap 的是**硬杀**（Windows TerminateProcess）——那种情况靠
# `accept-all.sh` 每轮自己编二进制兜底，下一轮不会因为"exe 被锁"直接停在第一步。
cleanup() { kill "$SRV" 2>/dev/null; wait "$SRV" 2>/dev/null; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
sleep 6
U=$(grep -o 'http://[^"]*' tmp/canc.txt | head -1)
T=$(grep -o '"token":"[^"]*' tmp/canc.txt | cut -d'"' -f4)
curl -s -c tmp/canc.ck "$U/connect?token=$T" -o /dev/null
ID=$(curl -s -b tmp/canc.ck -XPOST $U/api/session.create -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
echo "会话 $ID  宿主 $U"

# 长生成：让它一直在吐字，才有可中断的窗口。
python -c "
import json
json.dump({'id':'$ID','text':'用 bash 工具执行 sleep 30，等它跑完再告诉我结果。'},open('tmp/canc-p.json','w',encoding='utf-8'),ensure_ascii=False)
"
curl -s -b tmp/canc.ck -XPOST $U/api/session.prompt --data-binary @tmp/canc-p.json >/dev/null

hist() { curl -s -b tmp/canc.ck -XPOST $U/api/session.history -d "{\"sessionId\":\"$ID\",\"id\":\"$ID\",\"since\":0}" > tmp/canc-hist.json; }
types() { python -c "
import json
try: fr=json.load(open('tmp/canc-hist.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
print(' '.join(str(f.get('type','')) for f in fr))
" 2>/dev/null; }

# 等 turn 真的开始（不是等固定秒数——网关慢的时候固定等待会假红）
STARTED=0
for i in $(seq 1 30); do
  hist; TYPES=$(types)
  case "$TYPES" in *turn/start*) STARTED=1; break;; esac
  sleep 1
done
[ "$STARTED" = "1" ]; chk "turn 真的开始了（turn/start 帧）" "$?"

# **立刻取消**：窗口在 bash sleep 30 执行期间。第一版 sleep 2 是错的——
# 模型两秒内就 turn/end 了，cancel 收到的是"没有这一轮"。
RES=$(curl -s -b tmp/canc.ck -XPOST $U/api/session.cancel -d "{\"sessionId\":\"$ID\"}")
echo "$RES" | grep -q '"cancelled":true'; chk "session.cancel 被接受" "$?" "$RES"

# 取消之后 turn 必须收尾：不能再有新的 assistant 增量
sleep 4; hist; AFTER=$(types)
case "$AFTER" in *turn/done*|*turn/error*|*cancelled*) CLOSED=1;; *) CLOSED=0;; esac
[ "$CLOSED" = "1" ]; chk "取消后 turn 收尾（done/error/cancelled）" "$?" "帧序：$AFTER"

# 收尾之后不该再有内容增长（这是"取消真的生效"与"只是返回了 true"的区别）
hist; N1=$(python -c "
import json
d=json.load(open('tmp/canc-hist.json',encoding='utf-8'))['result']
print(len(d.get('frames') or []))
")
sleep 5; hist; N2=$(python -c "
import json
d=json.load(open('tmp/canc-hist.json',encoding='utf-8'))['result']
print(len(d.get('frames') or []))
")
[ "$N2" -le "$((N1 + 1))" ]; chk "取消后帧数基本不再增长（$N1 → $N2）" "$?"

# **退出码就是判据**：FAIL 非零必须以非零退出，否则调用方（`&&`、CI、`accept-all.sh`）会把一次失败的验收当成通过。
# 这一行原来写的是无条件 `exit 0` —— 脚本明明打了 FAIL=1，却对调用方说"我过了"。
echo "----- 合计 OK=$ok FAIL=$fail"
echo "帧序：$(types)"
[ "$fail" -eq 0 ] || exit 1
