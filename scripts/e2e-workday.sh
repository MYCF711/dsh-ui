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
# ---- 2026-11-04 U1-H6：所有断言换成「有具体值才算过」 ----
#
# 旧版的探针是 `curl … | grep -q "WORKDAY-1"`。这条在**模型一个字都没答**时也会 PASS：
# 用户 prompt 原文里就写着 WORKDAY-1，`session.load` 把 user 消息一起返回，
# grep 命中的是用户自己的话。同类假阳性还有三类（只 grep 不看信封 / 负向断言没有
# 正向控制 / 空 body 也算过），逐个列在 docs/progress.md 的 U1-H6 条目里。
#
# 现在每条断言都落到 scripts/e2e-probe.py：**先验信封，再验结构，最后验具体字段**，
# 并且每条都能被坏输入打成红 —— 反证见 scripts/e2e-probe-selftest.sh。
#
# 口径两条：
#   1. 能力没验到 = FAIL，不是 SKIP。SKIP 默认也计入 exit 非 0；
#      确实允许降级时显式 `ALLOW_SKIP=1`，降级后的 SKIP 仍会在汇总里单独计数。
#   2. 两个二进制**每次都重建**。用旧二进制跑出来的"绿"不算证据（bin-check 的教训）。
#
# 用法：bash scripts/e2e-workday.sh            # 全流程
#       KEEP=1 bash scripts/e2e-workday.sh     # 保留临时目录（排查用）
#       ALLOW_SKIP=1 bash scripts/e2e-workday.sh  # 允许把未验到的步骤记成 SKIP
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="/c/Program Files/Go/bin:$PATH"

PY="${PY:-python}"
command -v "$PY" >/dev/null 2>&1 || { echo "需要 python：断言层是 scripts/e2e-probe.py"; exit 1; }
PROBE=("$PY" scripts/e2e-probe.py)

OUT="${OUT:-tmp/e2e-workday}"
# `rm -rf` 在 Windows 上会**部分失败**（上一轮宿主还没死透时，logs/ 与
# sessions/*.zstd 仍被它锁着），而且失败是静默的：目录删不掉一半，这一轮就带着
# **上一轮的会话**跑——"新建的会话在 list 里"、"两会话隔离"这类断言全都不作数了。
# 实测就撞到过一次（日志开头两行 `rm: cannot remove ...Device or resource busy`）。
# 所以：删不掉就重试一次，仍然删不干净就**明确退出**，绝不带脏 home 跑。
if ! rm -rf "$OUT" 2>/tmp/u1h6-rm.err; then
  sleep 2
  rm -rf "$OUT" 2>>/tmp/u1h6-rm.err
fi
if [ -d "$OUT" ] && [ -n "$(ls -A "$OUT" 2>/dev/null)" ]; then
  echo "清不掉 $OUT —— 上一轮的宿主可能还活着（Windows 锁着它的 logs/sessions）。"
  echo "先按端口认领再杀：netstat -ano | grep <上一轮的端口>，确认 --home 指向 $OUT/home 后再 taskkill //PID。"
  head -3 /tmp/u1h6-rm.err | sed 's/^/  /'
  exit 1
fi
mkdir -p "$OUT"

# 两个二进制放在**本次运行自己的目录**里，不用 tmp/ 那种共享路径：
# Windows 上正在运行的 exe 会被锁，而并行会话也会用同一个 tmp/ 路径起宿主
# （实测撞到过：`open ../dsh-ui/tmp/dshgo.exe: The process cannot access the file`,
# 占着它的是**别的会话**的宿主，既不能杀也不该杀）。每次跑自己的副本，互不影响。
#
# **名字就是产品的名字**：宿主 `dshgo.exe`、界面 `DSH GUI GO.exe`。
# 界面这一根含空格、分大小写（产品名全程，用户口径），所以下面每一处调用
# 都必须整体加引号——`$APP` 已经带引号，别再拼出裸路径。
# 以前这里叫 `dshd-serve.exe` / `app-e2e.exe`——每个脚本一套别名，
# 进程表里 34 个进程就分不清谁是谁（用户就是这么被坑的）。区分实例靠 `--home`。
BIN="$OUT/bin"
mkdir -p "$BIN"
SERVE="$BIN/dshgo.exe"
APP="$BIN/DSH GUI GO.exe"
pass=0
fail=0
skip=0
step() { printf '\n== %s ==\n' "$*"; }
ok()   { pass=$((pass+1)); printf '  PASS  %s\n' "$*"; }
bad()  { fail=$((fail+1)); printf '  FAIL  %s\n' "$*"; }
skp()  { skip=$((skip+1)); printf '  SKIP  %s\n' "$*"; }

# chk <描述> <命令…> —— 命令 exit 0 才记 PASS；失败时把探针的原话打出来。
chk() {
  local desc="$1"; shift
  local out rc
  out=$("$@" 2>&1); rc=$?
  if [ $rc -eq 0 ]; then
    ok "$desc"
    [ -n "$out" ] && printf '        %s\n' "$out"
  else
    bad "$desc"
    [ -n "$out" ] && printf '        %s\n' "$out"
  fi
}

# unverified <描述> —— 这一步该验的能力**没验到**。默认 FAIL：
# 「没验到」和「验过且通过」是两件事，混成一个 SKIP 就会得到
# 「11 PASS / 0 FAIL」这种看起来全绿、其实有一条根本没跑的汇总。
unverified() {
  if [ "${ALLOW_SKIP:-0}" = "1" ]; then
    skp "$*"
  else
    bad "$*  ←（未验到；确需降级请显式 ALLOW_SKIP=1）"
  fi
}

# POST <出参文件> <方法> <请求体>：body 落文件，HTTP 码落 <文件>.code。
# 落文件是为了让探针能对**同一份响应**先验结构再验字段——管道里 grep 一次就没了。
POST() {
  local f="$1" m="$2" body="$3"
  curl -s -o "$f" -w '%{http_code}' -b "$OUT/ck.txt" -XPOST "$URL/api/$m" -d "$body" > "$f.code"
}

cleanup() {
  # 按端口回收宿主：MSYS 的 `$!` 和 Windows 的 PID 不是一回事，
  # `taskkill //PID "$SERVE_PID"` 会静默失败——上一版就是这样漏下一个宿主，
  # 下一轮 `go build -o tmp/dshgo.exe` 直接撞「文件被占用」。
  # 端口是 OS 分配给**本脚本刚起的那个**宿主的，按它找 PID 才算有归属。
  if [ -n "${URL:-}" ]; then
    local port="${URL##*:}" pid
    pid=$(netstat -ano 2>/dev/null | awk -v p=":$port" '$2 ~ p"$" && $4 == "LISTENING" {print $5; exit}')
    [ -n "$pid" ] && taskkill //PID "$pid" //F >/dev/null 2>&1
    # taskkill 返回了不等于句柄已释放：等端口真的不再 LISTENING 再走，
    # 否则下一次跑的 `rm -rf $OUT` 会因为 logs/ 与 sessions/*.zstd 还被锁而**部分失败**。
    for _ in $(seq 1 15); do
      netstat -ano 2>/dev/null | awk -v p=":$port" '$2 ~ p"$" && $4 == "LISTENING" {f=1} END {exit !f}' || break
      sleep 0.3
    done
  fi
  [ -n "${SERVE_PID:-}" ] && taskkill //PID "$SERVE_PID" //F >/dev/null 2>&1
  [ "${KEEP:-0}" = "1" ] || rm -f "$OUT"/home/sessions/*/*.lock 2>/dev/null
}
trap cleanup EXIT

step "构建（两个二进制都重建：验的必须是当前源码）"
CGO_ENABLED=0 go build -o "$APP" ./cmd/app || { echo "界面构建失败"; exit 1; }
(cd ../DSH-GO && CGO_ENABLED=0 go build -o "../dsh-ui/$SERVE" ./cmd/dshd) || {
  echo "宿主构建失败（若提示「文件被占用」，是上一轮的宿主没被回收：scripts 的 cleanup 按端口杀；也可先 tasklist 查 dshgo.exe 的 --home 再只杀自己那个）"
  exit 1
}
echo "  界面 $(stat -c%s "$APP") 字节 / 宿主 $(stat -c%s "$SERVE") 字节"

step "起宿主（临时 home、OS 分配端口、policy=ask）"
"$SERVE" serve --home "$OUT/home" --policy ask > "$OUT/ready.txt" 2> "$OUT/serve.log" &
SERVE_PID=$!
for _ in $(seq 1 30); do grep -q '"url"' "$OUT/ready.txt" 2>/dev/null && break; sleep 0.5; done
URL=$(grep -o 'http://[^"]*' "$OUT/ready.txt" | head -1)
TOK=$(grep -o '"token":"[^"]*' "$OUT/ready.txt" | cut -d'"' -f4)
have_ready() { [ -n "$URL" ] && [ -n "$TOK" ]; }
chk "宿主输出 url 与 token（ready.txt 里有具体值）" have_ready
[ -n "$URL" ] && [ -n "$TOK" ] || { echo "  宿主没起来，后面无从谈起"; exit 1; }
echo "  宿主 $URL"

step "认证 + 建会话"
curl -s -c "$OUT/ck.txt" "$URL/connect?token=$TOK" -o "$OUT/connect.json" -w '%{http_code}' > "$OUT/connect.code"
CK=$(grep dshd_session "$OUT/ck.txt" | awk '{print $7}')
have_cookie() { [ -n "$CK" ]; }
chk "connect 换到 cookie" have_cookie
# cookie 有值不等于宿主认它：拿它真发一个请求。
POST "$OUT/list0.json" session.list '{}'
chk "cookie 真被宿主认（session.list 200 + result 信封）" "${PROBE[@]}" http-ok "$OUT/list0.json" --code "$OUT/list0.json.code"

POST "$OUT/create1.json" session.create '{"mode":"default"}'
SID=$(grep -o '"sessionId":"[^"]*' "$OUT/create1.json" | cut -d'"' -f4)
chk "建会话（sessionId 非空）" test -n "$SID"
[ -n "$SID" ] || { echo "  建不了会话，后面无从谈起"; exit 1; }
echo "  会话 1: $SID"
POST "$OUT/list1.json" session.list '{}'
chk "会话真在宿主里（session.list 里能查到它）" "${PROBE[@]}" list-has "$OUT/list1.json" --id "$SID"

POST "$OUT/create2.json" session.create '{"mode":"default"}'
SID2=$(grep -o '"sessionId":"[^"]*' "$OUT/create2.json" | cut -d'"' -f4)
chk "建第二个会话（切会话用）" test -n "$SID2"
[ -n "$SID2" ] || { echo "  第二会话建不出来，切会话那条验不了"; exit 1; }

step "第 1 轮：普通对话（验 text 流）"
POST "$OUT/r1-prompt.json" session.prompt "{\"id\":\"$SID\",\"text\":\"Reply with exactly: WORKDAY-1. No tools.\"}"
chk "第 1 轮 prompt 被受理（HTTP 200 + result）" "${PROBE[@]}" http-ok "$OUT/r1-prompt.json" --code "$OUT/r1-prompt.json.code"
# 轮询到出现 assistant 正文为止（最多 40 秒）；判据是**assistant 的 text**，
# 不是「整个 body 里有没有这个词」——用户 prompt 里就写着 WORKDAY-1。
for _ in $(seq 1 40); do
  POST "$OUT/load-r1.json" session.load "{\"id\":\"$SID\",\"tailMessages\":6}"
  "${PROBE[@]}" load-ok "$OUT/load-r1.json" >/dev/null 2>&1 &&
    "${PROBE[@]}" role-text "$OUT/load-r1.json" --role assistant --contains "WORKDAY-1" >/dev/null 2>&1 && break
  sleep 1
done
chk "load 结构完整（messages 非空、total 自洽、role 合法）" "${PROBE[@]}" load-ok "$OUT/load-r1.json" --min-messages 1 --min-total 2
chk "宿主骗不了人：permission 是 ask（不是 mode 冒充权限）" "${PROBE[@]}" load-ok "$OUT/load-r1.json" --permission ask
chk "第 1 轮回答到位（**assistant 正文**含 WORKDAY-1，用户回显不算）" \
  "${PROBE[@]}" role-text "$OUT/load-r1.json" --role assistant --contains "WORKDAY-1"

step "第 2 轮：工具调用 + 审批(ask)"
# 这一轮**必须由界面发**：审批是走 WS 推给客户端的，只有界面在场才收得到
# approval/request；用 curl 发的话没人收帧，宿主 120 秒后默认拒绝（实测过）。
# -shot-wait 150：模型从 prompt 到调工具实测 60–90 秒，界面要活到审批出现之后。
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$SID"   -smoke -prompt "用 bash 工具执行 echo WORKDAY-TOOL，把输出告诉我"   -shot "$OUT/ui-tool.png" -shot-wait 150 > "$OUT/app2.log" 2>&1 &
APP2=$!
RID=""
for _ in $(seq 1 150); do
  RID=$(grep -o 'requestId=perm-[a-z0-9-]*' "$OUT/app2.log" 2>/dev/null | tail -1 | cut -d= -f2)
  [ -n "$RID" ] && break
  sleep 1
done
if [ -z "$RID" ]; then
  echo "  DIAG 界面日志行数=$(wc -l < "$OUT/app2.log" 2>/dev/null)"
  echo "  DIAG 界面日志尾部=$(tail -2 "$OUT/app2.log" 2>/dev/null | head -c 200)"
  echo "  DIAG 宿主审批标记=$(grep -c '@@APPROVAL@@' "$OUT/serve.log" 2>/dev/null || echo 0)"
  wait "$APP2" 2>/dev/null
  unverified "审批请求没出现（日志里没有 requestId=perm-…）：审批这条**没验到**"
else
  ok "界面收到审批请求 $RID"
  POST "$OUT/approve.json" session.approve "{\"requestId\":\"$RID\",\"optionId\":\"allow_once\"}"
  chk "审批回传被接受（accepted=true 且回显 optionId）" "${PROBE[@]}" accepted "$OUT/approve.json"
  wait "$APP2" 2>/dev/null
  APP2RC=$?
  chk "界面进程 exit 0" test "$APP2RC" -eq 0
  # allow 之后工具才真执行；给它几秒落历史。
  for _ in $(seq 1 20); do
    POST "$OUT/load-tool.json" session.load "{\"id\":\"$SID\",\"tailMessages\":10}"
    "${PROBE[@]}" tool-ran "$OUT/load-tool.json" --contains "WORKDAY-TOOL" --no-error-prefix >/dev/null 2>&1 && break
    sleep 1
  done
  chk "工具真的执行了（**tool 消息的输出**含 WORKDAY-TOOL，且不以 Error 开头、不是 args 回显）" \
    "${PROBE[@]}" tool-ran "$OUT/load-tool.json" --contains "WORKDAY-TOOL" --no-error-prefix
fi

step "第 3 轮：取消"
# 判据用**宿主自己的回答**，不用我们猜：
#   - `cancelled=true`  = 它当时手上真有一轮在跑，并且取消了；
#   - `没有这一轮`      = 上一轮已经收尾（那不算产品 bug，只是没验到）→ 再发一次。
#
# 上一版是"等历史里出现空的 assistant"来猜在途。实测**这个窗口只有首个 text 事件
# 之前的几百毫秒**：3 次尝试、每次 6 秒轮询，一次都没猜中（DIAG 里能看到尾部要么
# 只有 user、要么已经是完整的回答）。猜中也不等于取消成功——所以改成让宿主自己答。
#
# prompt 用 ASCII：中文走 `curl -d` 时 MSYS 传参会把字节搞坏（实测宿主里存成
# `дһƪ�����ּ����ĵ���` 这种乱码，模型收到的不是那句话）。
# 任务要"能流一阵又不会被模型判定为做不到"：20000 字会被当场回绝（实测），
# 1500 字的小作文会真的流起来。
LONG_PROMPT="Write a 1500-word essay about the history of the C programming language, with dates and names. Do not summarize."
CANCEL_OK=0
for attempt in 1 2 3 4; do
  POST "$OUT/cancel-prompt$attempt.json" session.prompt "{\"id\":\"$SID\",\"text\":\"$LONG_PROMPT\"}"
  sleep 1.2
  POST "$OUT/load-precancel.json" session.load "{\"id\":\"$SID\",\"tailMessages\":1}"
  "${PROBE[@]}" has-running-turn "$OUT/load-precancel.json" >/dev/null 2>&1 &&
    echo "  （历史里能看到「在途 turn」：尾部是空 assistant）" ||
    echo "  （历史里没看到在途 turn —— 这个窗口很窄，不影响判断，以取消的返回值为准）"
  POST "$OUT/cancel.json" session.cancel "{\"id\":\"$SID\"}"
  if "${PROBE[@]}" cancelled "$OUT/cancel.json" >/dev/null 2>&1; then
    CANCEL_OK=1
    echo "  第 $attempt 次取消了在途轮次"
    break
  fi
  echo "  DIAG 第 $attempt 次没取消到：$("${PROBE[@]}" cancelled "$OUT/cancel.json" 2>&1 | head -c 200)"
done
chk "取消被受理（宿主回 cancelled=true，说明当时真有一轮在跑）" test "$CANCEL_OK" -eq 1

step "第 4 轮：切会话不串流"
POST "$OUT/r4-prompt.json" session.prompt "{\"id\":\"$SID2\",\"text\":\"Reply with exactly: WORKDAY-OTHER. No tools.\"}"
for _ in $(seq 1 40); do
  POST "$OUT/load-s2.json" session.load "{\"id\":\"$SID2\",\"tailMessages\":6}"
  "${PROBE[@]}" load-ok "$OUT/load-s2.json" >/dev/null 2>&1 &&
    "${PROBE[@]}" role-text "$OUT/load-s2.json" --role assistant --contains "WORKDAY-OTHER" >/dev/null 2>&1 && break
  sleep 1
done
chk "会话 2 自己的回答在（assistant 正文含 WORKDAY-OTHER）" \
  "${PROBE[@]}" role-text "$OUT/load-s2.json" --role assistant --contains "WORKDAY-OTHER"
# 负向断言必须先有正向控制，否则「load 失败」会冒充「两个会话隔离」。
# 而且必须是**全量** load：上一版要的是 tailMessages=6，而那时会话 1 已经有 8 条，
# 「全字段都不含 WORKDAY-OTHER」只是在 6 条的窗口里成立——第一版正向控制
# （第 1 轮的内容还在不在）当场就红了，抓到的就是这个覆盖漏洞。
POST "$OUT/load-s1.json" session.load "{\"id\":\"$SID\",\"tailMessages\":100}"
chk "会话 1 的 load 是全量且结构完整（负向断言的前提）" \
  "${PROBE[@]}" load-ok "$OUT/load-s1.json" --min-messages 4 --require-full
chk "会话 1 里还留着第 1 轮的内容（正向控制）" \
  "${PROBE[@]}" role-text "$OUT/load-s1.json" --role assistant --contains "WORKDAY-1"
chk "会话 1 里没有会话 2 的内容（不串流）" \
  "${PROBE[@]}" not-contains "$OUT/load-s1.json" --contains "WORKDAY-OTHER"

step "界面连续使用（真宿主 + 真模型）+ 内存"
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$SID" \
  -smoke -prompt "Reply with exactly: WORKDAY-UI" \
  -shot "$OUT/ui-list.png" -shot-wait 15 > "$OUT/app.log" 2>&1
UIRC=$?
chk "界面连续使用 exit 0" test "$UIRC" -eq 0
# 不只是"日志里有那行"：文件要在、要是真 PNG、尺寸要像真窗口（不是最小化/空白）。
chk "界面出图（日志路径一致 + PNG 魔数 + 真实窗口尺寸）" "${PROBE[@]}" shot "$OUT/ui-list.png" --log "$OUT/app.log"
# 原来这里只是 echo，取不到就静默空行——等于没测。
chk "工作集读数取到了且在合理区间" "${PROBE[@]}" mem --log "$OUT/app.log" --max-mb 400

step "汇总"
echo "  PASS=$pass FAIL=$fail SKIP=$skip"
echo "  证据目录: $OUT"
[ "$fail" -eq 0 ] && [ "$skip" -eq 0 ]
