#!/usr/bin/env bash
# accept-theme.sh —— 主题的验收：两套令牌 + 运行时切换。
#
# # 为什么要有它
#
# 主题在 2026-11-04 之前只能靠 `-theme dark|light` 在启动时定死。加上运行时切换
# 之后，"点一下命令面板就变颜色"这件事**没有任何自动化的东西会去验**——
# 单测能证明模式解析对、指针身份没变，证明不了**屏幕上真的变浅了**。
#
# 这个脚本跑三次真进程，每次都用**平均亮度**做判据（不是"图里有东西"）：
#
#   1. `-theme dark`        → 截图亮度必须很低
#   2. `-theme light`       → 截图亮度必须很高
#   3. `-theme dark` + 键盘 F2、↓×9、Enter
#                           → 日志出现「主题：light」，且截图亮度和第 2 步一致
#
# 第 3 步是重点：它走的是**产品路径**（命令面板 → Actions.SetTheme → 排队 →
# 下一帧 Paint 才改），和启动参数那条完全不同。
#
# # 为什么判据是亮度
#
# 深浅两套令牌的差别是**全局**的：背景、卡片、文字全换。所以"平均亮度差一倍以上"
# 是一个不可能碰巧成立的信号，而且它不依赖任何 OCR 或颜色阈值猜测。
#
# 用法：bash scripts/accept-theme.sh          # 全流程（约 40 秒）
#       KEEP=1 bash scripts/accept-theme.sh   # 保留 tmp/theme-accept 里的中间产物

set -uo pipefail

cd "$(dirname "$0")/.." || exit 2
root=$(pwd)
export PATH="/c/Program Files/Go/bin:$PATH"

work="$root/tmp/theme-accept"
rm -rf "$work"
mkdir -p "$work"

if [ "${KEEP:-0}" != "1" ]; then
  trap 'rm -rf "$work"' EXIT
fi

ok=0
bad=0
pass() { echo "  PASS  $*"; ok=$((ok + 1)); }
fail() { echo "  FAIL  $*"; bad=$((bad + 1)); }

echo "== 构建 =="
if ! CGO_ENABLED=0 go build -o "$work/app.exe" ./cmd/app; then
  echo "  FAIL  build 不过"
  exit 1
fi
pass "build"

# 平均亮度：0（全黑）到 255（全白）。
luma() {
  python - "$1" <<'PY'
import sys
from PIL import Image, ImageStat
im = Image.open(sys.argv[1]).convert("L")
print("%.1f" % ImageStat.Stat(im).mean[0])
PY
}

# run <截图名> <期望:dark|light> <期望日志里的主题:string|""> [额外参数...]
run() {
  local name="$1" want="$2" wantlog="$3"
  shift 3
  local log="$work/$name.log"
  local png="$work/$name.png"

  # 每次都是**新进程**。三步都显式给了 `-theme`，所以落盘的 `ui.theme`
  # 不会覆盖它们（显式命令行优先，见 cmd/app/theme.go）。
  #
  # 顺带说明：没有宿主在跑时 `ConfigSet` 是失败的（HTTP 打不通，失败只写状态栏），
  # 所以这个脚本**不会**把 `ui.theme` 真的落盘、也就不会污染下一次普通启动。
  timeout 90 "$work/app.exe" -shot "$png" -shot-wait 9 "$@" >"$log" 2>&1
  local rc=$?
  if [ $rc -ne 0 ]; then
    fail "$name：进程退出码 $rc"
    return
  fi
  if [ ! -s "$png" ]; then
    fail "$name：没有截图"
    return
  fi
  local l
  l=$(luma "$png")
  case "$want" in
    dark)  if python -c "import sys; sys.exit(0 if float('$l') < 60 else 1)"; then pass "$name 截图亮度 $l（深）"; else fail "$name 截图亮度 $l，期望 < 60（深）"; fi ;;
    light) if python -c "import sys; sys.exit(0 if float('$l') > 200 else 1)"; then pass "$name 截图亮度 $l（浅）"; else fail "$name 截图亮度 $l，期望 > 200（浅）"; fi ;;
  esac
  if [ -n "$wantlog" ]; then
    if grep -q "$wantlog" "$log"; then
      pass "$name 日志里出现「$wantlog」"
    else
      fail "$name 日志里没有「$wantlog」"
    fi
  fi
  echo "$l"
}

echo "== 1. 启动参数 dark =="
run dark dark "" -theme dark

echo "== 2. 启动参数 light =="
run light light "" -theme light

echo "== 3. 运行时切换（命令面板没键盘点不动，所以走键盘） =="
# F2 打开命令面板 → 光标在第一条 → ↓×9 落到「主题：浅色」→ Enter。
# 列表顺序（无宿主时）：新建会话 / 开关右栏 / 重试上一条 / 打断 turn /
# 栏宽×4 / 主题×3(深、浅、跟随系统) / 权限×5。
# 顺序变了这里会**选错命令**——判据是日志里必须出现「主题：light」，
# 选错了就不会出现，所以这个脚本不会因此变绿。
run toggle light "主题：light" -theme dark \
  -keys f2,down,down,down,down,down,down,down,down,down,enter -keys-after 3

echo "== 4. 不认识的模式必须当场拒 =="
timeout 30 "$work/app.exe" -theme sepia -shot "$work/bad.png" >"$work/bad.log" 2>&1
if [ $? -ne 0 ] && grep -q "只能是 dark|light|system" "$work/bad.log"; then
  pass "非法模式被拒（并写清了合法值）"
else
  fail "非法模式没有被拒"
fi

echo
echo "----- 合计 OK=$ok FAIL=$bad"
[ "$bad" -eq 0 ] || exit 1