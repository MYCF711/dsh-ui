#!/usr/bin/env bash
# 审批整链：--policy ask 下让模型调一个写类工具，从 journal 取 approval 帧，
# 用 HTTP session.approve 回一次，再验工具真的跑了。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
# 宿主二进制路径可覆盖：默认是共享的 tmp/dshgo.exe。
# 为什么要能覆盖：那个文件会被**没退出的旧宿主**锁住（Windows），于是"编不出新二进制"，
# 整个验收直接停在第一步。`accept-all.sh` 每次编到自己那一轮的目录里再传进来。
# **不再给每个脚本另起名字**（`dshd-acc.exe` / `dshd-goal.exe` / `dshd-term.exe` 那种）：
# 进程表是唯一的现场，统一叫 `dshgo.exe`、实例用 `--home` 区分。
BIN="${DSHGO_BIN:-tmp/dshgo.exe}"
ok=0; fail=0
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

H=tmp/appr-home-$$; rm -rf "$H"
"$BIN" serve --home "$H" --policy ask > tmp/appr.txt 2> tmp/appr.log &
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
U=$(grep -o 'http://[^"]*' tmp/appr.txt | head -1)
T=$(grep -o '"token":"[^"]*' tmp/appr.txt | cut -d'"' -f4)
curl -s -c tmp/appr.ck "$U/connect?token=$T" -o /dev/null
ID=$(curl -s -b tmp/appr.ck -XPOST $U/api/session.create -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
echo "会话 $ID  宿主 $U"

# **点名工具**：不点名模型常常只回一句话，审批就永远等不到。
python -c "
import json
json.dump({'id':'$ID','text':'用 write 工具在 appr-demo.txt 里写一行 hi。这一步需要授权，请直接调用工具。'},open('tmp/appr-p.json','w',encoding='utf-8'),ensure_ascii=False)
"
curl -s -b tmp/appr.ck -XPOST $U/api/session.prompt --data-binary @tmp/appr-p.json >/dev/null

# 轮询 history 找 approval 帧（load 里没有 approvals，只有 journal 有）
RID=""; OPT=""
for i in $(seq 1 40); do
  curl -s -b tmp/appr.ck -XPOST $U/api/session.history -d "{\"sessionId\":\"$ID\",\"id\":\"$ID\",\"since\":0}" > tmp/appr-hist.json
  RID=$(python -c "
import json
try: fr=json.load(open('tmp/appr-hist.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
for f in fr:
    d=f.get('data') or {}
    if str(f.get('type','')).startswith('approval/') and d.get('requestId'): print(d['requestId']); break
" 2>/dev/null)
  [ -n "$RID" ] && break
  sleep 2
done
[ -n "$RID" ]; chk "收到 approval/request（requestId 非空）" "$?" "轮询 ${i} 次"

OPT=$(python -c "
import json
try: fr=json.load(open('tmp/appr-hist.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
for f in fr:
    d=f.get('data') or {}
    if str(f.get('type','')).startswith('approval/'):
        o=d.get('options') or []
        print(o[0].get('optionId') or o[0].get('id') if o and isinstance(o[0],dict) else (d.get('optionId') or ''))
        break
" 2>/dev/null)
[ -n "$OPT" ]; chk "approval 帧带选项（optionId 非空）" "$?" "取到 '$OPT'"

# **批准要循环**：模型一次 turn 里可能发起多个工具调用，每个都要单独批。
# 第一版只批一次，于是第一个工具（ls）跑了、第二个（write）卡在审批里，
# 最后判"写工具没跑"——那不是链路的问题，是脚本的假设错了。
if [ -n "$RID" ] && [ -n "$OPT" ]; then
  RES=$(curl -s -b tmp/appr.ck -XPOST $U/api/session.approve -d "{\"requestId\":\"$RID\",\"optionId\":\"$OPT\"}")
  echo "$RES" | grep -q '"accepted":true'; chk "session.approve 被接受（$OPT）" "$?" "$RES"

  APPROVED=1
  for i in $(seq 1 25); do
    sleep 3
    curl -s -b tmp/appr.ck -XPOST $U/api/session.history -d "{\"sessionId\":\"$ID\",\"id\":\"$ID\",\"since\":0}" > tmp/appr-hist2.json
    NEXT=$(python -c "
import json
try: fr=json.load(open('tmp/appr-hist2.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
seen=set(); out=''
for f in fr:
    d=f.get('data') or {}
    if str(f.get('type','')).startswith('approval/request'):
        rid=d.get('requestId')
        o=d.get('options') or []
        if rid and rid not in seen and o and isinstance(o[0],dict):
            out=rid+'|'+(o[0].get('optionId') or '')
print(out)
" 2>/dev/null)
    if [ -n "$NEXT" ]; then
      R2="${NEXT%%|*}"; O2="${NEXT##*|}"
      curl -s -b tmp/appr.ck -XPOST $U/api/session.approve -d "{\"requestId\":\"$R2\",\"optionId\":\"$O2\"}" >/dev/null
      APPROVED=$((APPROVED+1))
    fi
    # 写工具的 tool/result 一出就收工
    python -c "
import json
try: fr=json.load(open('tmp/appr-hist2.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
raise SystemExit(0 if any(str(f.get('type',''))=='tool/result' and 'appr-demo' in str((f.get('data') or {})) for f in fr) else 1)
" && break
  done
  echo "（本轮共批准 $APPROVED 次）"
  curl -s -b tmp/appr.ck -XPOST $U/api/session.load -d "{\"id\":\"$ID\",\"tailMessages\":50}" > tmp/appr-msgs.json
  python -c "
import json
d=json.load(open('tmp/appr-msgs.json',encoding='utf-8'))['result']
m=d.get('messages') or []
raise SystemExit(0 if any(x.get('role')=='tool' and x.get('tool')=='write' for x in m) else 1)
"
  chk "批准之后写工具真的跑了" "$?"
else
  echo "SKIP  批准与执行 —— 没拿到 approval 帧（模型这次没调工具）"
fi

# 重放：同一个 requestId 再来一次应当是 404（不能重复批准）
if [ -n "$RID" ] && [ -n "$OPT" ]; then
  curl -s -b tmp/appr.ck -XPOST $U/api/session.approve -d "{\"requestId\":\"$RID\",\"optionId\":\"$OPT\"}" | grep -q "no_such_request"
  chk "同一个 requestId 不能重复批准（no_such_request）" "$?"
fi

# **退出码就是判据**：FAIL 非零必须以非零退出，否则调用方（`&&`、CI、`accept-all.sh`）会把一次失败的验收当成通过。
# 这一行原来写的是无条件 `exit 0` —— 脚本明明打了 FAIL=1，却对调用方说"我过了"。
echo "----- 合计 OK=$ok FAIL=$fail"
[ "$fail" -eq 0 ] || exit 1
