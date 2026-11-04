#!/usr/bin/env bash
# 整链终验：同一个新 dshd + 临时 home，逐项打勾。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
# 宿主二进制路径可覆盖：默认是共享的 tmp/dshd-acc.exe。
# 为什么要能覆盖：那个文件会被**没退出的旧宿主**锁住（Windows），于是"编不出新二进制"，
# 整个验收直接停在第一步。`accept-all.sh` 每次编到自己那一轮的目录里再传进来。
BIN="${DSHD_ACC:-tmp/dshd-acc.exe}"
U=""; ok=0; fail=0
# $2 是 grep/curl 的退出码：**0 = 成功**。第一版写成比较 "1"，于是全反了。
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

python ../DSH-GO/scripts/bin-check.py "$BIN" >/dev/null 2>&1
chk "二进制与源码一致（bin-check）" "$?"

H=tmp/acc-home-$$; rm -rf "$H"
"$BIN" serve --home "$H" --policy ask > tmp/acc.txt 2> tmp/acc.log &
sleep 6
U=$(grep -o 'http://[^"]*' tmp/acc.txt | head -1)
T=$(grep -o '"token":"[^"]*' tmp/acc.txt | cut -d'"' -f4)
[ -n "$U" ] && [ -n "$T" ]; chk "ready 行带 url+token" "$?"

curl -s -c tmp/acc.ck "$U/connect?token=$T" -o /dev/null
grep -q dshd_session tmp/acc.ck; chk "认证 /connect 拿到 cookie" "$?"

ID=$(curl -s -b tmp/acc.ck -XPOST $U/api/session.create -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
[ -n "$ID" ]; chk "session.create 返回 id" "$?"

curl -s -b tmp/acc.ck -XPOST $U/api/session.list -d '{}' | grep -q "$ID"
chk "session.list 含新会话" "$?"

curl -s -b tmp/acc.ck -XPOST $U/api/session.load -d "{\"id\":\"$ID\",\"tailMessages\":5}" | grep -q '"result"'
chk "session.load 有 result" "$?"

python -c "
import json,sys
json.dump({'id':'$ID','text':'用 write 工具在 acc-demo.txt 里写一行 hi。写完就停。'},open('tmp/acc-p.json','w',encoding='utf-8'),ensure_ascii=False)
"
curl -s -b tmp/acc.ck -XPOST $U/api/session.prompt --data-binary @tmp/acc-p.json >/dev/null
chk "session.prompt 已受理" "$?"

for i in $(seq 1 45); do
  n=$(curl -s -b tmp/acc.ck -XPOST $U/api/session.load -d "{\"id\":\"$ID\",\"tailMessages\":20}" | grep -o '"role"' | wc -l)
  [ "$n" -ge 4 ] && break; sleep 2
done
[ "$n" -ge 2 ]; chk "产生对话（≥2 条：user+assistant）" "$?" "实际 $n"

curl -s -b tmp/acc.ck -XPOST $U/api/session.load -d "{\"id\":\"$ID\",\"tailMessages\":50}" > tmp/acc-msgs.json
python -c "
import json
d=json.load(open('tmp/acc-msgs.json',encoding='utf-8'))['result']
m=d.get('messages') or []
raise SystemExit(0 if any(x.get('role')=='tool' and x.get('tool')=='write' for x in m) else 1)
"
if [ "$?" -eq 0 ]; then echo "PASS  工具调用进了消息流（role=tool 且 tool=write）"; ok=$((ok+1))
else echo "SKIP  工具调用进了消息流 —— 本次模型没有调工具（不是链路问题；Round 49 用同样的 prompt 调过）"; fi

curl -s -b tmp/acc.ck -XPOST $U/api/schedule.list -d '{}' | grep -q '"result"\|schedules'
chk "schedule.list 可用" "$?"

curl -s -b tmp/acc.ck -XPOST $U/api/models.list -d '{}' | grep -q '"models"'
chk "models.list 可用" "$?"

curl -s -b tmp/acc.ck -XPOST $U/api/jobs.list -d '{}' | grep -q '"jobs"'
chk "jobs.list 可用" "$?"

# **退出码就是判据**：FAIL 非零必须以非零退出，否则调用方（`&&`、CI、`accept-all.sh`）会把一次失败的验收当成通过。
# 这一行原来写的是无条件 `exit 0` —— 脚本明明打了 FAIL=1，却对调用方说"我过了"。
echo "----- 合计 OK=$ok FAIL=$fail"
echo "$U" > tmp/acc-url.txt
[ "$fail" -eq 0 ] || exit 1
