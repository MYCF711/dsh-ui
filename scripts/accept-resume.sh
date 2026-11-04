#!/usr/bin/env bash
# 续历史 / 断线恢复：跑一轮有工具调用的 turn，换一条连接按 seq 补洞，
# 验补回来的帧和原连接看到的是同一份（reasoning/tool/approval 不丢）。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
# 宿主二进制路径可覆盖：默认是共享的 tmp/dshd-acc.exe。
# 为什么要能覆盖：那个文件会被**没退出的旧宿主**锁住（Windows），于是"编不出新二进制"，
# 整个验收直接停在第一步。`accept-all.sh` 每次编到自己那一轮的目录里再传进来。
BIN="${DSHD_ACC:-tmp/dshd-acc.exe}"
ok=0; fail=0
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

H=tmp/resm-home-$$; rm -rf "$H"
"$BIN" serve --home "$H" --policy allow > tmp/resm.txt 2> tmp/resm.log &
sleep 6
U=$(grep -o 'http://[^"]*' tmp/resm.txt | head -1)
T=$(grep -o '"token":"[^"]*' tmp/resm.txt | cut -d'"' -f4)
curl -s -c tmp/resm-a.ck "$U/connect?token=$T" -o /dev/null
ID=$(curl -s -b tmp/resm-a.ck -XPOST $U/api/session.create -d '{"mode":"default"}' | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
echo "会话 $ID  宿主 $U"

# 造一段有 tool 的历史
python -c "
import json
json.dump({'id':'$ID','text':'用 bash 工具依次执行 ls 和 echo resume-ok，然后总结。'},open('tmp/resm-p.json','w',encoding='utf-8'),ensure_ascii=False)
"
curl -s -b tmp/resm-a.ck -XPOST $U/api/session.prompt --data-binary @tmp/resm-p.json >/dev/null

h() { curl -s -b "$1" -XPOST $U/api/session.history -d "{\"sessionId\":\"$ID\",\"id\":\"$ID\",\"sinceSeq\":${2:-0}}" > "$3"; }
for i in $(seq 1 40); do
  h tmp/resm-a.ck 0 tmp/resm-h1.json
  python -c "
import json
try: fr=json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result'].get('frames') or []
except Exception: fr=[]
raise SystemExit(0 if any(str(f.get('type',''))=='tool/result' for f in fr) else 1)
" && break
  sleep 2
done
h tmp/resm-a.ck 0 tmp/resm-h1.json
N1=$(python -c "
import json
d=json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result']
print(len(d.get('frames') or []))
")
T1=$(python -c "
import json
d=json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result']
fr=d.get('frames') or []
print(' '.join(sorted(set(str(f.get('type','')) for f in fr))))
")
LAST=$(python -c "
import json
print(json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result'].get('lastSeq',0))
")
[ "$N1" -ge 4 ]; chk "原连接看到完整历史（$N1 帧，lastSeq=$LAST）" "$?"
case "$T1" in *tool/*) HAS_TOOL=1;; *) HAS_TOOL=0;; esac
[ "$HAS_TOOL" = "1" ]; chk "历史里有 tool 帧" "$?" "$T1"

# **换一条连接**：cookie 还在（这才是真实的重连）。
#
# 第一版想用同一个 token 再换一个 cookie —— **401**。查实：`/connect` 的 token
# 是**一次性**的。这符合实际：重连时浏览器本地那条 cookie 并没有丢，
# 丢的是 TCP/WS 连接。所以这里复制 cookie 文件，而不是再去 connect 一次。
cp tmp/resm-a.ck tmp/resm-b.ck
grep -q dshd_session tmp/resm-b.ck; chk "重连沿用原 cookie（token 是一次性的）" "$?"

# 从 0 拉：应当和原连接看到的一致
h tmp/resm-b.ck 0 tmp/resm-h2.json
# **不能拿 h1 和 h2 比**：h1 是轮询期间抓的（turn 可能还没完），h2 是之后抓的。
# 第一版就是这么比的，于是"帧数不同"被判成 FAIL——**两个时间点的快照本来就不该相等**。
# 正确的判据：h2 以 h1 为前缀（历史只增不改，这是 append-only 的直接推论）。
python -c "
import json
a=json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result'].get('frames') or []
b=json.load(open('tmp/resm-h2.json',encoding='utf-8'))['result'].get('frames') or []
ta=[(f.get('seq'),str(f.get('type',''))) for f in a]
tb=[(f.get('seq'),str(f.get('type',''))) for f in b]
raise SystemExit(0 if tb[:len(ta)]==ta else 1)
"
chk "历史 append-only（h1 是 h2 的前缀：${N1} → ${N2:-?} 帧）" "$?"

# **补洞**：从 lastSeq 往前一格开始要，应当只回最后一帧
MID=$((LAST > 0 ? LAST - 1 : 0))
h tmp/resm-b.ck "$MID" tmp/resm-h3.json
python -c "
import json
d=json.load(open('tmp/resm-h3.json',encoding='utf-8'))['result']
fr=d.get('frames') or []
b=json.load(open('tmp/resm-h2.json',encoding='utf-8'))['result'].get('frames') or []
want=[f.get('seq') for f in b if (f.get('seq') or 0) > $MID]
got=[f.get('seq') for f in fr]
raise SystemExit(0 if got==want and len(got)>=1 else 1)
"
chk "sinceSeq=$MID 只补回后几帧（不是整段重发）" "$?"

# journalEpoch 要一致（不一致说明是另一份 journal，补洞会串）
python -c "
import json
a=json.load(open('tmp/resm-h1.json',encoding='utf-8'))['result']
b=json.load(open('tmp/resm-h2.json',encoding='utf-8'))['result']
ea,eb=a.get('journalEpoch'),b.get('journalEpoch')
raise SystemExit(0 if ea==eb and ea not in (None,'') else 1)
"
chk "两条连接的 journalEpoch 一致" "$?"

# **退出码就是判据**：FAIL 非零必须以非零退出，否则调用方（`&&`、CI、`accept-all.sh`）会把一次失败的验收当成通过。
# 这一行原来写的是无条件 `exit 0` —— 脚本明明打了 FAIL=1，却对调用方说"我过了"。
echo "----- 合计 OK=$ok FAIL=$fail"
[ "$fail" -eq 0 ] || exit 1
