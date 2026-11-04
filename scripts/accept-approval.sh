#!/usr/bin/env bash
# 审批整链：--policy ask 下让模型调一个写类工具，从 journal 取 approval 帧，
# 用 HTTP session.approve 回一次，再验工具真的跑了。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
ok=0; fail=0
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

H=tmp/appr-home-$$; rm -rf "$H"
./tmp/dshd-acc.exe serve --home "$H" --policy ask > tmp/appr.txt 2> tmp/appr.log &
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

echo "----- 合计 OK=$ok FAIL=$fail"
exit 0
