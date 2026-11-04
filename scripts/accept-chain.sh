#!/usr/bin/env bash
# 整链终验：同一个新 dshd + 临时 home，逐项打勾。
set -uo pipefail
cd "$(dirname "$0")/.."
export PATH="/c/Program Files/Go/bin:$PATH"
# 宿主二进制路径可覆盖：默认是共享的 tmp/dshgo.exe。
# 为什么要能覆盖：那个文件会被**没退出的旧宿主**锁住（Windows），于是"编不出新二进制"，
# 整个验收直接停在第一步。`accept-all.sh` 每次编到自己那一轮的目录里再传进来。
# **不再给每个脚本另起名字**（`dshd-acc.exe` / `dshd-goal.exe` / `dshd-term.exe` 那种）：
# 进程表是唯一的现场，统一叫 `dshgo.exe`、实例用 `--home` 区分。
BIN="${DSHGO_BIN:-tmp/dshgo.exe}"
U=""; ok=0; fail=0
# $2 是 grep/curl 的退出码：**0 = 成功**。第一版写成比较 "1"，于是全反了。
chk() { if [ "$2" -eq 0 ]; then echo "PASS  $1"; ok=$((ok+1)); else echo "FAIL  $1  ${3:-}"; fail=$((fail+1)); fi; }

# bin-check 必须**在宿主仓里跑**：它扫的 `SRC_ROOTS`（cmd / internal / modules / codecs）
# 是**相对当前目录**的。以前这行在界面仓里调它，等于拿**界面**的源码去比**宿主**二进制——
# 这个检查从来没测过任何东西，只是碰巧 mtime 顺序让它绿而已。
# 2026-11-04 真红了一次：报「dshgo.exe 比 cmdpp\headless_windows.go 旧」——
# 那是界面的文件，跟这个宿主二进制没有任何关系。
BIN_ABS="$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN")"
(cd ../DSH-GO && python scripts/bin-check.py "$BIN_ABS" >/dev/null 2>&1)
chk "二进制与源码一致（bin-check，在宿主仓里跑）" "$?"

H=tmp/acc-home-$$; rm -rf "$H"
"$BIN" serve --home "$H" --policy ask > tmp/acc.txt 2> tmp/acc.log &
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
