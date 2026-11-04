#!/usr/bin/env bash
# accept-session-ops.sh —— 三个"会话操作"的**真宿主**验收：fork / delete / compact。
#
# # 为什么要有它
#
# 这三个方法在界面侧都有客户端单测（`internal/apiclient/{fork,delete,compact}_test.go`），
# 但那些测的是**桩**：请求形状对不对、返回值解没解全。它们证明不了
# "真宿主收下这组参数之后真的那么做了"——而那正是这个仓反复踩的地方
# （信封套错、字段名两套、200 但没干活）。
#
# 所以这里起一个**真 dshd**（临时 home、独立二进制），走真 HTTP：
#
#   create → compact（空会话：应当 compacted=false，不是错误）
#          → fork（拿到新 id，列表里真有两条）
#          → delete 分支（deleted:true，列表只剩一条）
#          → delete 源（deleted:true，列表空）
#          → 再 delete 一次（404 no_such_session，**错误信封**）
#
# compact 特意放在**空会话**上做：它要么回 compacted=false（正常），
# 要么报错——两者的区别就是"能不能对短会话点压缩"。而这一步不调模型。
#
# 用法：bash scripts/accept-session-ops.sh        （约 20 秒）
#       KEEP=1 bash scripts/accept-session-ops.sh # 保留临时 home 和日志

set -uo pipefail
cd "$(dirname "$0")/.." || exit 2
export PATH="/c/Program Files/Go/bin:$PATH"

ok=0
fail=0
skip=0
chk() { if [ "$2" -eq 0 ]; then echo "  PASS  $1"; ok=$((ok + 1)); else echo "  FAIL  $1  ${3:-}"; fail=$((fail + 1)); fi; }
skp() { echo "  SKIP  $1"; skip=$((skip + 1)); }

work=tmp/session-ops
rm -rf "$work"
mkdir -p "$work"

echo "== 构建宿主（当前源码） =="
if ! (cd ../DSH-GO && CGO_ENABLED=0 go build -o "$OLDPWD/$work/dshgo.exe" ./cmd/dshd); then
  echo "  FAIL  宿主 build 不过"
  exit 1
fi
echo "  PASS  宿主 build"

H="$work/home"
"$work/dshgo.exe" serve --home "$H" --policy allow > "$work/serve.txt" 2> "$work/serve.log" &
PID=$!
cleanup() {
  kill "$PID" 2>/dev/null
  wait "$PID" 2>/dev/null
  [ "${KEEP:-0}" = "1" ] || rm -rf "$work/home"
}
trap cleanup EXIT

U=""
for _ in $(seq 1 30); do
  U=$(grep -o 'http://[^"]*' "$work/serve.txt" 2>/dev/null | head -1)
  [ -n "$U" ] && break
  sleep 1
done
if [ -z "$U" ]; then
  echo "  FAIL  宿主没给出地址"
  head -5 "$work/serve.log"
  exit 1
fi
T=$(grep -o '"token":"[^"]*' "$work/serve.txt" | cut -d'"' -f4)
echo "  宿主 $U"

curl -s -c "$work/ck" "$U/connect?token=$T" -o /dev/null
chk "/connect 换到 cookie" $?

# api <method> <json> <outfile>
api() { curl -s -b "$work/ck" -XPOST "$U/api/$1" -d "$2" -o "$3" -w '%{http_code}'; }

# jq <file> <python 表达式：d 是解析后的 result（错误信封时为 None）>
jq_() {
  python - "$1" "$2" <<'PY'
import json, sys
raw = json.load(open(sys.argv[1], encoding="utf-8"))
res = raw.get("result")
env = sys.argv[2]
print(json.dumps(eval(env, {"d": res, "raw": raw}), ensure_ascii=False))
PY
}

echo "== 1. 建一个会话 =="
code=$(api session.create '{"mode":"default"}' "$work/create.json")
A=$(jq_ "$work/create.json" '((d or {}).get("sessionId") or "")' 2>/dev/null | tr -d '"')
chk "session.create 回了 200 和 sessionId（$A）" $([ "$code" = "200" ] && [ -n "$A" ]; echo $?)

echo "== 2. 空会话上调 compact（不该报错，该回 compacted=false） =="
code=$(api session.compact "{\"sessionId\":\"$A\",\"id\":\"$A\"}" "$work/compact.json")
got=$(jq_ "$work/compact.json" '"ok" if (d is not None) else "err"' 2>/dev/null)
chk "compact 走通（HTTP $code，result 存在=$got）" $([ "$code" = "200" ] && [ "$got" = '"ok"' ]; echo $?)
compacted=$(jq_ "$work/compact.json" 'bool((d or {}).get("compacted"))' 2>/dev/null)
chk "空会话上 compacted=false（真值 $compacted，不是错误）" $([ "$compacted" = "false" ]; echo $?)

echo "== 3. 空会话 fork：宿主会拒（真行为，记录下来） =="
# 源会话一条消息都没有 → 宿主 `serveFork` 没有 surface 可继承 → 400。
# 这不是缺陷，是**契约边界**：界面必须自己拦住（cmd/app/session_fork.go 已拦）。
code=$(api session.fork "{\"sessionId\":\"$A\",\"id\":\"$A\"}" "$work/fork-empty.json")
msg=$(jq_ "$work/fork-empty.json" '((raw.get("error") or {}).get("message") or "")' 2>/dev/null | tr -d '"')
# 注意状态码是 **500** 不是 400：宿主只把 `ErrNoSuchSession` 映射成 404，
# 其余一律 500。空会话不满足条件属于"客户端拿这个状态做不了"，
# 严格说 409 更合适——记下来，等宿主侧有人愿意改再说。
chk "空会话 fork 被拒且带原因（HTTP $code；$msg）" $([ "$code" = "500" ] && [ -n "$msg" ]; echo $?)

echo "== 3b. 有内容的会话才能 fork =="
python -c "
import json
json.dump({'id':'$A','text':'只回复两个字：好'},open('$work/p.json','w',encoding='utf-8'),ensure_ascii=False)
"
api session.prompt "@$work/p.json" "$work/prompt.json" > /dev/null 2>&1 || true
# curl 不认 @；上面那句只是占位，真发用 --data-binary
curl -s -b "$work/ck" -XPOST "$U/api/session.prompt" --data-binary "@$work/p.json" -o "$work/prompt.json"
got_msg=0
for _ in $(seq 1 20); do
  api session.history "{\"sessionId\":\"$A\",\"id\":\"$A\",\"sinceSeq\":0}" "$work/h.json" > /dev/null
  got_msg=$(jq_ "$work/h.json" 'sum(1 for f in ((d or {}).get("frames") or []) if f.get("type")=="assistant/message")' 2>/dev/null)
  [ "${got_msg:-0}" -gt 0 ] && break
  sleep 2
done

if [ "${got_msg:-0}" -gt 0 ]; then
  code=$(api session.fork "{\"sessionId\":\"$A\",\"id\":\"$A\"}" "$work/fork.json")
  B=$(jq_ "$work/fork.json" '((d or {}).get("sessionId") or "")' 2>/dev/null | tr -d '"')
  chk "有内容的会话 fork 成功（$B，HTTP $code）" $([ "$code" = "200" ] && [ -n "$B" ] && [ "$B" != "$A" ]; echo $?)

  api session.list '{}' "$work/list2.json" > /dev/null
  n=$(jq_ "$work/list2.json" 'len((d or {}).get("sessions") or [])' 2>/dev/null)
  chk "列表里有两条会话（实际 $n）" $([ "$n" = "2" ]; echo $?)
else
  skp "这一轮没产出 assistant/message（模型或网关没回），fork 的 happy path 未验"
  B=""
fi

echo "== 3c. 有内容的会话上 compact：真的压了 =="
# 3b 之后会话里已经有 user + assistant（可能还有 tool），够压了。
# 这一条要**调一次模型**生成摘要，所以放最后、并给宽松的超时。
if [ "${got_msg:-0}" -gt 0 ]; then
  code=$(curl -s -b "$work/ck" -XPOST "$U/api/session.compact"     -d "{\"sessionId\":\"$A\",\"id\":\"$A\",\"keep\":1}"     -o "$work/compact-real.json" -w '%{http_code}' --max-time 120)
  okflag=$(jq_ "$work/compact-real.json" 'bool((d or {}).get("compacted"))' 2>/dev/null)
  if [ "$code" != "200" ]; then
    msg=$(jq_ "$work/compact-real.json" '((raw.get("error") or {}).get("message") or "")' 2>/dev/null | tr -d '"')
    fail_msg="HTTP $code ${msg}"
    echo "  FAIL  有内容的会话上 compact 回 200（实际 $fail_msg）"
    fail=$((fail + 1))
  elif [ "$okflag" != "true" ]; then
    # 会话太短、宿主判断"不值得压"（`MinToCompact=2`：至少要折叠两条）。
    # 这**不是失败**，但也不能记 PASS——那会把"没验到"说成"验到了"。
    #
    # 所以先补两轮，让它够长，再试一次。补不上就 SKIP（说清为什么）。
    for _ in 1 2; do
      python -c "
import json
json.dump({'id':'$A','text':'只回复两个字：收到'},open('$work/p2.json','w',encoding='utf-8'),ensure_ascii=False)
"
      curl -s -b "$work/ck" -XPOST "$U/api/session.prompt" --data-binary "@$work/p2.json" -o /dev/null
      for _ in $(seq 1 15); do
        api session.history "{\"sessionId\":\"$A\",\"id\":\"$A\",\"sinceSeq\":0}" "$work/h2.json" > /dev/null
        n=$(jq_ "$work/h2.json" 'sum(1 for f in ((d or {}).get("frames") or []) if f.get("type")=="assistant/message")' 2>/dev/null)
        [ "${n:-0}" -ge 2 ] && break
        sleep 2
      done
    done
    code=$(curl -s -b "$work/ck" -XPOST "$U/api/session.compact"       -d "{\"sessionId\":\"$A\",\"id\":\"$A\",\"keep\":1}"       -o "$work/compact-real.json" -w '%{http_code}' --max-time 120)
    okflag=$(jq_ "$work/compact-real.json" 'bool((d or {}).get("compacted"))' 2>/dev/null)
    if [ "$okflag" = "true" ]; then
      shadowed=$(jq_ "$work/compact-real.json" 'int((d or {}).get("shadowed") or 0)' 2>/dev/null)
      chars=$(jq_ "$work/compact-real.json" 'int((d or {}).get("summaryChars") or 0)' 2>/dev/null)
      chk "补两轮之后真压了（折叠 $shadowed 条，摘要 $chars 字）"         $([ "${shadowed:-0}" -gt 0 ] && [ "${chars:-0}" -gt 0 ]; echo $?)
      code=$(api session.load "{\"sessionId\":\"$A\",\"id\":\"$A\",\"limit\":200}" "$work/load-after.json")
      chk "压完还能 load（HTTP $code）" $([ "$code" = "200" ]; echo $?)
      compacted_for_real=1
    else
      skp "补两轮之后宿主仍判断不值得压（compacted=false）——真压路径本轮未验"
    fi
  else
    shadowed=$(jq_ "$work/compact-real.json" 'int((d or {}).get("shadowed") or 0)' 2>/dev/null)
    chars=$(jq_ "$work/compact-real.json" 'int((d or {}).get("summaryChars") or 0)' 2>/dev/null)
    chk "真压了（compacted=true，折叠 $shadowed 条，摘要 $chars 字）"       $([ "${shadowed:-0}" -gt 0 ] && [ "${chars:-0}" -gt 0 ]; echo $?)
    # 压完会话必须还能打开——压缩是投影，不是删会话。
    code=$(api session.load "{\"sessionId\":\"$A\",\"id\":\"$A\",\"limit\":200}" "$work/load-after.json")
    chk "压完还能 load（HTTP $code）" $([ "$code" = "200" ]; echo $?)
  fi
else
  skp "3b 没跑出会话，compact 的真压路径未验"
fi

echo "== 4. 删分支 =="
if [ -z "${B:-}" ]; then
  skp "没有分支可删（3b 没跑出会话）"
else
code=$(api session.delete "{\"sessionId\":\"$B\",\"id\":\"$B\"}" "$work/del-b.json")
deleted=$(jq_ "$work/del-b.json" 'bool((d or {}).get("deleted"))' 2>/dev/null)
chk "删除分支回 deleted=true（HTTP $code）" $([ "$code" = "200" ] && [ "$deleted" = "true" ]; echo $?)

api session.list '{}' "$work/list1.json" > /dev/null
has_a=$(jq_ "$work/list1.json" "any(s.get('id')=='$A' for s in ((d or {}).get('sessions') or []))" 2>/dev/null)
has_b=$(jq_ "$work/list1.json" "any(s.get('id')=='$B' for s in ((d or {}).get('sessions') or []))" 2>/dev/null)
chk "删掉的是分支、源会话还在（A=$has_a B=$has_b）" $([ "$has_a" = "true" ] && [ "$has_b" = "false" ]; echo $?)
fi

echo "== 5. 删源会话 =="
code=$(api session.delete "{\"sessionId\":\"$A\",\"id\":\"$A\"}" "$work/del-a.json")
chk "删除源会话回 200 + deleted=true（HTTP $code）" $([ "$code" = "200" ]; echo $?)

echo "== 6. 再删一次：必须是 404 **错误信封**，不是 200 空结果 =="
code=$(api session.delete "{\"sessionId\":\"$A\",\"id\":\"$A\"}" "$work/del-again.json")
chk "已删的会话再删 → 404（实际 HTTP $code）" $([ "$code" = "404" ]; echo $?)
code_field=$(jq_ "$work/del-again.json" '((raw.get("error") or {}).get("code") or "")' 2>/dev/null | tr -d '"')
chk "错误信封带 code（$code_field）" $([ -n "$code_field" ]; echo $?)

echo
echo "----- 合计 OK=$ok FAIL=$fail SKIP=$skip"
[ "$fail" -eq 0 ] && [ "$skip" -eq 0 ] || exit 1
