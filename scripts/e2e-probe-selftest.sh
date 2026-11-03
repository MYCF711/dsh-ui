#!/usr/bin/env bash
# e2e-probe-selftest.sh —— 给 e2e 断言层做反证：**每条探针都要能被打成红**。
#
# 为什么要它：一个永远 PASS 的探针比没有探针更糟——它会给「11 PASS / 0 FAIL」
# 这种汇总盖章，但里面可能有几条根本没验到东西。所以这里把旧的判定式
# （`grep -q 某词`）和新探针跑在**同一份坏输入**上，左右对比：
#
#   旧 GREEN / 新 RED  = 证明旧探针确实会假阳性，新探针不认它
#   旧 RED   / 新 RED  = 旧探针在这份坏输入上是好的（没被我改坏）
#   旧 NONE  / 新 RED  = 旧版根本没有断言（只有 echo），现在是真断言了
#   旧 GREEN / 新 GREEN = 真阳性对照：证明新探针不是"永远红"
#
# 夹具全部来自 2026-11-04 在本机真宿主（`dshd serve --policy ask`）上抓到的真实响应，
# 只做了裁剪；`user-only-marker.json` 那份是**真机抓的**：用户消息里写着 WORKDAY-1，
# 模型答的是 OK —— 旧探针在它上面 PASS，这就是那类假阳性的实物。
#
# 用法：bash scripts/e2e-probe-selftest.sh          # 表格 + 结论
#       bash scripts/e2e-probe-selftest.sh -v       # 失配时把探针原话也打出来
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PY="${PY:-python}"
command -v "$PY" >/dev/null 2>&1 || { echo "需要 python"; exit 1; }
VERBOSE="${1:-}"

F=tmp/u1h6-fixtures
rm -rf "$F"
mkdir -p "$F"

# ---------------------------------------------------------------- 夹具
# 1) 真机抓的：用户 prompt 里有 WORKDAY-1，assistant 答的是 OK。
#    旧探针 `grep -q WORKDAY-1` 在它上面 PASS —— 模型一个字没答也算过。
cat > "$F/user-only-marker.json" <<'EOF'
{"result":{"messages":[{"seq":2,"role":"user","text":"Never output the token WORKDAY-1 again. Reply with exactly: OK","time":1793729921667},{"seq":3,"role":"assistant","text":"OK","time":1793729922694,"usage":{"inputTokens":7309,"outputTokens":3,"totalTokens":7312}}],"total":2,"truncated":false,"permission":"ask","usageTotal":{"inputTokens":7309,"outputTokens":3,"totalTokens":7312},"contextTokens":7312,"journalEpoch":2,"lastSeq":5}}
EOF

# 1b) 同一形状，marker 换成第二个会话的：用户 prompt 里有 WORKDAY-OTHER，模型答 OK。
cat > "$F/user-only-other.json" <<'EOF'
{"result":{"messages":[{"seq":2,"role":"user","text":"Reply with exactly: WORKDAY-OTHER. No tools.","time":1793729921667},{"seq":3,"role":"assistant","text":"OK","time":1793729922694}],"total":2,"truncated":false,"permission":"ask","journalEpoch":2,"lastSeq":5}}
EOF

# 2) 真机抓的：assistant 真的答了（真阳性对照）。
cat > "$F/good-answer.json" <<'EOF'
{"result":{"messages":[{"seq":2,"role":"user","text":"Reply with exactly: WORKDAY-1. No tools.","time":1793729911158},{"seq":3,"role":"assistant","text":"WORKDAY-1","time":1793729911742,"usage":{"inputTokens":7306,"outputTokens":6,"totalTokens":7312}}],"total":2,"truncated":false,"permission":"ask","usageTotal":{"inputTokens":7306,"outputTokens":6,"totalTokens":7312},"contextTokens":7312,"journalEpoch":1,"lastSeq":5}}
EOF

# 3) 真机抓的：空会话 load（0 条）。
cat > "$F/empty-load.json" <<'EOF'
{"result":{"messages":[],"total":0,"truncated":false,"permission":"ask","journalEpoch":1}}
EOF

# 4) 真机抓的：load 不存在的会话（HTTP 404）。
cat > "$F/error-load.json" <<'EOF'
{"error":{"code":"no_such_session","hint":"先 session.list 拿 id；确认 --home 指向的目录里有这个会话","message":"没有这个会话","retryable":false}}
EOF

# 5) 真机抓的：cancel 时没有在途轮次。
cat > "$F/error-cancel.json" <<'EOF'
{"error":{"code":"error","hint":"","message":"没有这一轮","retryable":false}}
EOF

# 6) 拒绝路径的会话：marker 只出现在**用户消息**和 tool 的 **args** 里，输出是拒绝文案。
cat > "$F/tool-denied.json" <<'EOF'
{"result":{"messages":[{"seq":2,"role":"user","text":"用 bash 工具执行 echo WORKDAY-TOOL，把输出告诉我"},{"seq":4,"role":"tool","text":"Error: 用户拒绝了 bash 提示","tool":"bash","args":"echo WORKDAY-TOOL"},{"seq":5,"role":"assistant","text":"工具没能执行"}],"total":3,"truncated":false,"permission":"ask"}}
EOF

# 7) allow 之后的样子：tool 的输出里真有 marker。
cat > "$F/tool-ran.json" <<'EOF'
{"result":{"messages":[{"seq":4,"role":"tool","text":"[exit 0] WORKDAY-TOOL","tool":"bash","args":"echo WORKDAY-TOOL"},{"seq":5,"role":"assistant","text":"输出是 WORKDAY-TOOL"}],"total":2,"truncated":false,"permission":"ask"}}
EOF

# 8) approve 的两种返回（差异只在 optionId 回显）+ cancel。
echo '{"result":{"accepted":true,"optionId":"allow_once"}}' > "$F/approve-ok.json"
echo '{"result":{"accepted":true}}' > "$F/approve-no-opt.json"
echo '{"result":{"cancelled":true}}' > "$F/cancel-ok.json"

# 8b) 会话 2 的正常回答（切会话那两步的真阳性对照组）。
cat > "$F/s2-good.json" <<'EOF'
{"result":{"messages":[{"seq":3,"role":"user","text":"Reply with exactly: WORKDAY-OTHER. No tools."},{"seq":4,"role":"assistant","text":"WORKDAY-OTHER"}],"total":2,"truncated":false,"permission":"ask"}}
EOF

# 9) 在途 turn：尾部空的 assistant。
cat > "$F/running.json" <<'EOF'
{"result":{"messages":[{"seq":7,"role":"assistant","text":""}],"total":1,"truncated":false,"permission":"ask"}}
EOF
cat > "$F/finished.json" <<'EOF'
{"result":{"messages":[{"seq":7,"role":"assistant","text":"写完了"}],"total":1,"truncated":false,"permission":"ask"}}
EOF

# 10) 截图：一张"日志说存了但文件不在"，一张真存在（头 + 尺寸够）。
printf '2026/11/04 02:19:06 shot: 已保存 %s · 工作集 65.1 MB\n' "$F/missing.png" > "$F/log-says-saved.txt"
printf '2026/11/04 02:19:06 shot: 已保存 %s · 工作集 65.1 MB\n' "$F/ok.png" > "$F/log-real.txt"
printf '2026/11/04 02:19:06 INFO 插件已装配 id=ds-core\n2026/11/04 02:19:09 mux 已订阅 session:abc since=13 epoch=1\n' > "$F/log-noshot.txt"
printf '2026/11/04 02:19:06 shot: 已保存 %s\n' "$F/ok.png" > "$F/log-nomem.txt"
{
  printf '\x89PNG\r\n\x1a\n'
  printf '\x00\x00\x00\x0dIHDR'
  printf '\x00\x00\x04\x9c'   # w=1180
  printf '\x00\x00\x03\x20'   # h=800
  printf '\x08\x06\x00\x00\x00'
  head -c 9000 /dev/zero
} > "$F/ok.png"
{ printf '\x89PNG\r\n\x1a\n'; head -c 4 /dev/zero; } > "$F/tiny.png"
printf 'garbage' > "$F/notpng.png"

# 12) 全量 load：会话 1 到了 8 条，tailMessages=6 拿回来的是 truncated 的窗口。
#     2026-11-04 实跑时，"两会话隔离"就是在 6 条窗口里判的——没有 --require-full
#     的话，负向断言覆盖不到整个会话，却仍然是绿的。
cat > "$F/truncated-load.json" <<'EOF'
{"result":{"messages":[{"seq":4,"role":"user","text":"hi"},{"seq":5,"role":"assistant","text":"yo"}],"total":8,"truncated":true,"permission":"ask"}}
EOF
cat > "$F/full-load.json" <<'EOF'
{"result":{"messages":[{"seq":2,"role":"user","text":"Reply with exactly: WORKDAY-1. No tools."},{"seq":3,"role":"assistant","text":"WORKDAY-1"},{"seq":4,"role":"user","text":"hi"},{"seq":5,"role":"assistant","text":"yo"}],"total":4,"truncated":false,"permission":"ask"}}
EOF

# 11) HTTP 码文件。
echo 200 > "$F/code-200"; echo 404 > "$F/code-404"

# ---------------------------------------------------------------- 用例表
# 编号~说明~旧判据~新判据~期望旧~期望新
#   GREEN = 该判据在这份输入上通过   RED = 不通过   NONE = 旧版压根没有断言
CASES=$(cat <<'EOF'
P1-假~第1轮"回答到位"：用户 prompt 里就有 WORKDAY-1~grep -q "WORKDAY-1" $F/user-only-marker.json~$PY scripts/e2e-probe.py role-text $F/user-only-marker.json --role assistant --contains WORKDAY-1~GREEN~RED
P1-真~同上，assistant 真答了（真阳性对照）~grep -q "WORKDAY-1" $F/good-answer.json~$PY scripts/e2e-probe.py role-text $F/good-answer.json --role assistant --contains WORKDAY-1~GREEN~GREEN
P3-假~工具"真的执行了"：marker 只在用户消息和 tool.args 里~grep -q "WORKDAY-TOOL" $F/tool-denied.json~$PY scripts/e2e-probe.py tool-ran $F/tool-denied.json --contains WORKDAY-TOOL --no-error-prefix~GREEN~RED
P3-真~同上，allow 之后 tool 输出里有 marker~grep -q "WORKDAY-TOOL" $F/tool-ran.json~$PY scripts/e2e-probe.py tool-ran $F/tool-ran.json --contains WORKDAY-TOOL --no-error-prefix~GREEN~GREEN
P6-假~两会话隔离：load 回 404 错误信封，负向断言照样"通过"~! grep -q "WORKDAY-OTHER" $F/error-load.json~$PY scripts/e2e-probe.py not-contains $F/error-load.json --contains WORKDAY-OTHER~GREEN~RED
P6-假2~两会话隔离：空 load 也算"不含"~! grep -q "WORKDAY-OTHER" $F/empty-load.json~$PY scripts/e2e-probe.py not-contains $F/empty-load.json --contains WORKDAY-OTHER~GREEN~RED
P6-真~会话 1 里确实没有会话 2 的内容~! grep -q "WORKDAY-OTHER" $F/good-answer.json~$PY scripts/e2e-probe.py not-contains $F/good-answer.json --contains WORKDAY-OTHER~GREEN~GREEN
P7-假~会话2内容在：用户消息回显 WORKDAY-OTHER~grep -q "WORKDAY-OTHER" $F/user-only-other.json~$PY scripts/e2e-probe.py role-text $F/user-only-other.json --role assistant --contains WORKDAY-OTHER~GREEN~RED
P7-真~会话2内容在：assistant 真答了 WORKDAY-OTHER~grep -q "WORKDAY-OTHER" $F/s2-good.json~$PY scripts/e2e-probe.py role-text $F/s2-good.json --role assistant --contains WORKDAY-OTHER~GREEN~GREEN
P4-空~取消前的"turn 在跑"判据：load 是空会话~grep -q '"role":"assistant","text":""' $F/empty-load.json~$PY scripts/e2e-probe.py has-running-turn $F/empty-load.json~RED~RED
P4-真~取消前：尾部真是空 assistant~grep -q '"role":"assistant","text":""' $F/running.json~$PY scripts/e2e-probe.py has-running-turn $F/running.json~GREEN~GREEN
P4-反~取消前：turn 已收尾（不能冒充在途）~grep -q '"role":"assistant","text":""' $F/finished.json~$PY scripts/e2e-probe.py has-running-turn $F/finished.json~RED~RED
P6-窗~隔离：只看 6 条尾巴就算"全字段不含"（旧断言没有全量概念）~test -n "$(grep -o '"messages"' $F/truncated-load.json)"~$PY scripts/e2e-probe.py load-ok $F/truncated-load.json --min-messages 2 --require-full~GREEN~RED
P6-全~隔离：拿的是全量 load（truncated=false）~test -n "$(grep -o '"messages"' $F/full-load.json)"~$PY scripts/e2e-probe.py load-ok $F/full-load.json --min-messages 4 --require-full~GREEN~GREEN
P2-弱~审批 accepted 没回显 optionId（旧断言只 grep accepted）~grep -q '"accepted":true' $F/approve-no-opt.json~$PY scripts/e2e-probe.py accepted $F/approve-no-opt.json~GREEN~RED
P2-真~审批 accepted + optionId 回显~grep -q '"accepted":true' $F/approve-ok.json~$PY scripts/e2e-probe.py accepted $F/approve-ok.json~GREEN~GREEN
P5-真~cancel 真受理~grep -q '"cancelled":true' $F/cancel-ok.json~$PY scripts/e2e-probe.py cancelled $F/cancel-ok.json~GREEN~GREEN
P5-假~cancel 回的是错误信封（HTTP 200 也可能，这里连 body 都不对）~grep -q '"cancelled":true' $F/error-cancel.json~$PY scripts/e2e-probe.py cancelled $F/error-cancel.json~RED~RED
P8-假~截图：日志说"已保存"但文件不存在~grep -q "shot: 已保存" $F/log-says-saved.txt~$PY scripts/e2e-probe.py shot $F/missing.png --log $F/log-says-saved.txt~GREEN~RED
P8-假2~截图：文件在但是 8 字节的假 PNG~grep -q "shot: 已保存" $F/log-real.txt~$PY scripts/e2e-probe.py shot $F/tiny.png --log $F/log-real.txt~GREEN~RED
P8-假3~截图：文件在但不是 PNG~grep -q "shot: 已保存" $F/log-real.txt~$PY scripts/e2e-probe.py shot $F/notpng.png --log $F/log-real.txt~GREEN~RED
P8-真~截图：日志路径一致 + 魔数 + 尺寸都成立~grep -q "shot: 已保存" $F/log-real.txt~$PY scripts/e2e-probe.py shot $F/ok.png --log $F/log-real.txt~GREEN~GREEN
P9-假~内存：日志里根本没有"工作集"（旧版只是 echo，空行也过）~~$PY scripts/e2e-probe.py mem --log $F/log-nomem.txt~NONE~RED
P9-真~内存：日志里有工作集读数~~$PY scripts/e2e-probe.py mem --log $F/log-real.txt~NONE~GREEN
P10-假~信封：HTTP 200 但 body 是 error~grep -q '"result"' $F/error-load.json~$PY scripts/e2e-probe.py http-ok $F/error-load.json --code $F/code-200~RED~RED
P10-真~信封：HTTP 200 + result~grep -q '"result"' $F/good-answer.json~$PY scripts/e2e-probe.py http-ok $F/good-answer.json --code $F/code-200~GREEN~GREEN
P10-反~HTTP 码不对（404）但 body 恰好有 result~grep -q '"result"' $F/good-answer.json~$PY scripts/e2e-probe.py http-ok $F/good-answer.json --code $F/code-404~GREEN~RED
P1-结~结构：空 load 不算"有 messages"~test -n "$(grep -o '"messages":\[\]' $F/empty-load.json)"~$PY scripts/e2e-probe.py load-ok $F/empty-load.json --min-messages 1~GREEN~RED
P1-权~结构：permission 不是 ask 就不算验到策略~grep -q '"permission":"ask"' $F/error-load.json~$PY scripts/e2e-probe.py load-ok $F/error-load.json --permission ask~RED~RED
P13-假~界面进程非 0 退出（旧版从不看退出码，截图存下来就算过）~~test 7 -eq 0~NONE~RED
P13-真~界面进程 exit 0 才算过~~test 0 -eq 0~NONE~GREEN
EOF
)

# ---------------------------------------------------------------- 跑
printf '\n== e2e 断言层反证（旧判据 vs 新探针，同一份坏输入）==\n\n'
printf '%-8s %-52s %-7s %-7s %s\n' "编号" "说明" "旧" "新" "结论"
printf -- '--------------------------------------------------------------------------------------------------\n'

n=0
nbad=0
while IFS='~' read -r id desc old new want_old want_new; do
  [ -z "$id" ] && continue
  n=$((n+1))
  ores=NONE
  if [ -n "$old" ]; then
    if eval "$old" >/dev/null 2>&1; then ores=GREEN; else ores=RED; fi
  fi
  nres=RED
  if eval "$new" >/dev/null 2>&1; then nres=GREEN; fi

  verdict="ok"
  [ "$ores" = "$want_old" ] || verdict="旧≠期望"
  [ "$nres" = "$want_new" ] || verdict="新≠期望"
  [ "$verdict" = "ok" ] || nbad=$((nbad+1))
  printf '%-8s %-52s %-7s %-7s %s\n' "$id" "$desc" "$ores" "$nres" "$verdict"
  if [ "$verdict" != "ok" ] && [ "$VERBOSE" = "-v" ]; then
    printf '         旧命令: %s\n         新命令: %s\n' "$old" "$new"
    eval "$new" 2>&1 | sed 's/^/         新探针: /'
  fi
done <<< "$CASES"

printf -- '--------------------------------------------------------------------------------------------------\n'
echo "用例 $n 个，失配 $nbad 个"
if [ "$nbad" -ne 0 ]; then
  echo "结论：断言层没达到「坏输入必红」——先修探针，别信 e2e 的绿。"
  exit 1
fi
echo "结论：旧判据在假阳性输入上全绿、新探针全红；真阳性输入两边都绿（不是"永远红"）。"
echo "夹具：$F"
