#!/usr/bin/env bash
# slot-proof.sh —— 公开槽「实画」的可复现证据。
#
# 要证明的事：说"我没法 mock，只能靠真机截图"是不合格的。这里的每个槽都有
# **一条可复现的断言**，而不是"看一眼截图觉得对"：
#
#   · 真机（真 dshd 宿主 + 真窗口，窗口自己挪到屏幕外，不弹给用户）；
#   · 每个槽裁一张图（tmp/u2-slot-*.png）+ 一条"屏幕上能看到什么"；
#   · 断言的是**像素**：该槽的矩形里有多少像素不是它容器的底色；
#     浮层槽额外做 on/off 差分——面板底色 #2c2c2e 只可能由 dsshortcuts 画出来。
#
# 为什么浮层要单独做 on/off：base 那次运行 F1 面板是关的，F1 那次是开的。
# 两次运行的差别只有 `-keys f1` 和面板本身，所以差分直接证明"这个槽被画了"。
#
# 用法：
#   bash scripts/slot-proof.sh              # 全流程（约 30 秒）
#   KEEP=1 bash scripts/slot-proof.sh       # 保留宿主 home 与日志
#   APP=… SERVE=… bash scripts/slot-proof.sh
#
# 只回收本脚本自己起的宿主（先按 PID，再用 ready 行里的端口反查兜底）。
# 绝不按进程名杀——这台机器上可能同时跑着别人的 dshd。
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
export PATH="/c/Program Files/Go/bin:$PATH"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export CGO_ENABLED=0

OUT="${OUT:-tmp/u2-slot-proof}"
APP="${APP:-tmp/u2-app.exe}"
SERVE="${SERVE:-tmp/dshd-serve.exe}"
HOST_SRC="${HOST_SRC:-../DSH-GO}"
BASE_PNG="${BASE_PNG:-tmp/u2-base.png}"
F1_PNG="${F1_PNG:-tmp/u2-f1.png}"
CROP_DIR="${CROP_DIR:-tmp}"
WIN_W=1180
WIN_H=800

pass=0
fail=0
step() { printf '\n== %s ==\n' "$*"; }
ok()   { pass=$((pass + 1)); printf '  PASS  %s\n' "$*"; }
bad()  { fail=$((fail + 1)); printf '  FAIL  %s\n' "$*"; }

cleanup() {
  [ -n "${SERVE_PID:-}" ] && kill "$SERVE_PID" 2>/dev/null
  local u port pid
  u=$(grep -o '127.0.0.1:[0-9]*' "$OUT/ready.txt" 2>/dev/null | head -1)
  port=${u##*:}
  if [ -n "$port" ]; then
    pid=$(netstat -ano 2>/dev/null | awk -v p=":$port" '$2 ~ p && $4 == "LISTENING" {print $5; exit}')
    [ -n "$pid" ] && taskkill //PID "$pid" //F >/dev/null 2>&1
  fi
  if [ "${KEEP:-0}" != "1" ]; then rm -rf "$OUT/home"; fi
}
trap cleanup EXIT

command -v go >/dev/null || { echo "需要 Go（/c/Program Files/Go/bin）"; exit 1; }
command -v python >/dev/null || { echo "需要 python（截图断言用 PIL）"; exit 1; }
mkdir -p "$OUT"
python -c 'import PIL' 2>/dev/null || { echo "需要 Pillow：python -m pip install pillow"; exit 1; }

# ── 0. 无头帧：真 kernel + 真 dsshell + 真 dsshortcuts ─────────────────────
#
# 这一步不接宿主，但它跑的是**真的 flex 帧**（不是 mock Frame）：面板是否被
# 提交到画布、画在哪个矩形、绘制顺序里在 hero 之前还是之后，都在这里断言。
# 它比截图更早失败、更容易定位；截图负责证明"真窗口上也是这个样子"。
step "无头帧：shell.overlay 槽里的面板真的被画出来"
if U2_CAPTURE_DIR="$ROOT/tmp" go test ./plugins/dsshortcuts -run TestSheetIsPaintedInOverlaySlot -count=1 -v > "$OUT/headless.log" 2>&1; then
  ok "TestSheetIsPaintedInOverlaySlot（断言：Notify 触发重绘、居中 520 宽、hero 之后绘制、Esc 关闭）"
else
  bad "无头帧测试失败，细节见 $OUT/headless.log"
  sed -n '1,40p' "$OUT/headless.log"
fi

step "构建真机产物"
if go build -o "$APP" ./cmd/app; then ok "app → $APP"; else bad "app 构建失败"; exit 1; fi
if [ ! -x "$SERVE" ]; then
  if (cd "$HOST_SRC" && go build -o "$ROOT/$SERVE" ./cmd/dshd); then
    ok "宿主 → $SERVE（从 $HOST_SRC 构建）"
  else
    bad "宿主构建失败：$HOST_SRC/cmd/dshd"; exit 1
  fi
else
  # 二进制没有版本指纹，跑起来和新的长得一样（AGENTS §5.1）。这里只提示不拦：
  # 槽位证据不依赖宿主的某个新特性，旧宿主也能证明"界面在画"。
  newer=$(find "$HOST_SRC" -name '*.go' -newer "$SERVE" 2>/dev/null | head -1)
  if [ -n "$newer" ]; then
    printf '  NOTE  %s 比宿主源码旧（如 %s）；要最新宿主就重建那个二进制\n' \
      "$SERVE" "${newer#"$HOST_SRC"/}"
  fi
  ok "宿主已就绪 → $SERVE"
fi

rm -rf "$OUT/home"
mkdir -p "$OUT/home"

step "起真宿主（临时 home、OS 分配端口、policy allow）"
"$SERVE" serve --home "$OUT/home" --policy allow > "$OUT/ready.txt" 2> "$OUT/serve.log" &
SERVE_PID=$!
for _ in $(seq 1 40); do
  grep -q '"url"' "$OUT/ready.txt" 2>/dev/null && break
  sleep 0.5
done
URL=$(grep -o 'http://[^"]*' "$OUT/ready.txt" | head -1)
TOK=$(grep -o '"token":"[^"]*' "$OUT/ready.txt" | cut -d'"' -f4)
if [ -n "$URL" ] && [ -n "$TOK" ]; then ok "宿主就绪 $URL"; else bad "宿主没起来（看 $OUT/serve.log）"; exit 1; fi

curl -s -c "$OUT/ck.txt" "$URL/connect?token=$TOK" -o /dev/null
CK=$(grep dshd_session "$OUT/ck.txt" | awk '{print $7}')
S1=$(curl -s -b "$OUT/ck.txt" -XPOST "$URL/api/session.create" -d '{"mode":"default"}' \
       | grep -o '"sessionId":"[^"]*' | cut -d'"' -f4)
if [ -n "$CK" ] && [ -n "$S1" ]; then ok "换到 cookie、建好会话 ${S1:0:8}…"; else bad "cookie/会话创建失败"; exit 1; fi

step "真窗口 #1：全部默认槽（F1 面板关闭）→ $BASE_PNG"
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$S1" \
  -size "${WIN_W}x${WIN_H}" -rightbar -title "dsh-ui-u2-base" \
  -shot "$BASE_PNG" -shot-wait 5 > "$OUT/app-base.log" 2>&1
if [ -s "$BASE_PNG" ]; then ok "抓到 $BASE_PNG"; else bad "没抓到 $BASE_PNG（看 $OUT/app-base.log）"; exit 1; fi
grep -q "插件已装配 id=ds-shortcuts" "$OUT/app-base.log" \
  && ok "dsshortcuts 已装配（不是没装上的空槽）" || bad "dsshortcuts 没有装配"

step "真窗口 #2：-keys f1 → $F1_PNG"
"$APP" -addr "$URL" -cookie "dshd_session=$CK" -session "$S1" \
  -size "${WIN_W}x${WIN_H}" -rightbar -title "dsh-ui-u2-f1" \
  -keys f1 -keys-after 2 \
  -shot "$F1_PNG" -shot-wait 6 > "$OUT/app-f1.log" 2>&1
if [ -s "$F1_PNG" ]; then ok "抓到 $F1_PNG"; else bad "没抓到 $F1_PNG（看 $OUT/app-f1.log）"; exit 1; fi
grep -q "handled=true" "$OUT/app-f1.log" \
  && ok "F1 被插件接住（-keys 走 HandleKey，日志 handled=true）" \
  || bad "F1 没有被接住（看 $OUT/app-f1.log）"

# ── 像素断言：每个槽的矩形里"不是容器底色"的像素占比 ────────────────────
#
# 底色取自主题令牌（docs/official-specs.md / plugins/dscore）：
#   侧栏 SidebarFill #1b1b1c、主区/底栏 BgBase #151517、浮层面板 InputMajor #2c2c2e。
# 阈值取实测值的 1/2 左右：既能挡住"槽是空的"，又不会被字体渲染差异绊倒。
step "逐槽像素断言 + 裁剪（$CROP_DIR/u2-slot-*.png）"
if python - "$BASE_PNG" "$F1_PNG" "$CROP_DIR" "$WIN_W" "$WIN_H" <<'PY'
import os
import sys
import warnings

from PIL import Image, ImageChops

# Pillow 12 把 getdata 标成弃用；这里只是读像素，警告没有信息量。
warnings.filterwarnings("ignore", category=DeprecationWarning)

base_path, f1_path, crop_dir, win_w, win_h = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4]), int(sys.argv[5])
base = Image.open(base_path).convert("RGB")
f1 = Image.open(f1_path).convert("RGB")

# 窗口截图含不可见的 8px 边框和 31px 标题栏：内容原点由窗口尺寸反推
# （和 cmd/app/headless_windows.go 的 1196x839 / 1180x800 是同一对数）。
ox = (base.width - win_w) // 2
oy = base.height - win_h - ox
if (ox, oy) not in ((8, 31),) and not (0 <= ox <= 40 and 0 <= oy <= 60):
    print(f"  截图尺寸 {base.width}x{base.height} 对不上 {win_w}x{win_h}，算不出内容原点")
    sys.exit(2)

SIDEBAR = (0x1B, 0x1B, 0x1C)
BG = (0x15, 0x15, 0x17)
SHEET = (0x2C, 0x2C, 0x2E)


def near(pixel, color, tol=3):
    return all(abs(pixel[i] - color[i]) <= tol for i in range(3))


def non_bg(image, box, bg):
    crop = image.crop((ox + box[0], oy + box[1], ox + box[2], oy + box[3]))
    pixels = list(crop.getdata())
    hit = sum(1 for p in pixels if not near(p, bg))
    return hit / len(pixels), crop


def fraction(image, box, color):
    crop = image.crop((ox + box[0], oy + box[1], ox + box[2], oy + box[3]))
    pixels = list(crop.getdata())
    return sum(1 for p in pixels if near(p, color)) / len(pixels)


# 槽 → (矩形, 容器底色, 阈值, "屏幕上能看到什么")
# 矩形是内容坐标；阈值是"非底色像素"占比的下限。
slots = [
    ("sidebar.header", (12, 4, 268, 44), SIDEBAR, 0.04,
     "左上角的 ＋ 新建按钮、右侧「全部收起」"),
    ("sidebar.list", (12, 100, 268, 160), SIDEBAR, 0.20,
     "分组头「dsh-ui · 1」和选中的会话行（(无标题) + 相对时间）"),
    ("sidebar.footer", (12, 744, 268, 788), SIDEBAR, 0.15,
     "「1 个会话」+ 底部连接状态"),
    ("main.header", (292, 4, 672, 40), BG, 0.002,
     "面包屑「dsh-ui / (无标题)」"),
    ("main.body", (292, 250, 672, 420), BG, 0.02,
     "居中的品牌标记、「探索未至之境」、「预览版」"),
    ("main.composer", (292, 690, 672, 780), BG, 0.30,
     "输入卡片、占位文字「说点什么…」、审批/状态/发送按钮"),
    ("rightbar", (692, 4, 800, 40), SIDEBAR, 0.02,
     "右栏标题「轨迹」"),
    ("rightbar.body", (692, 60, 1000, 160), SIDEBAR, 0.002,
     "「本会话还没有工具调用」空态"),
    ("shell.bottom", (0, 772, 1180, 800), BG, 0.05,
     "底部整行状态条：会话数、连接状态、绿点 + 就绪"),
]

bad = 0
os.makedirs(crop_dir, exist_ok=True)
for name, box, bg, need, what in slots:
    got, crop = non_bg(base, box, bg)
    shot = os.path.join(crop_dir, f"u2-slot-{name.replace('.', '-')}.png")
    crop.resize((crop.width * 2, crop.height * 2), Image.NEAREST).save(shot)
    if got >= need:
        print(f"  PASS  {name:16s} 非底色 {got:6.2%} ≥ {need:.2%} · 能看到：{what} · {shot}")
    else:
        print(f"  FAIL  {name:16s} 非底色 {got:6.2%} < {need:.2%}（槽没画东西）· {shot}")
        bad += 1

# 浮层：on/off 差分。base 里这块矩形是主区正文，F1 里被面板盖住。
sheet_box = (330, 96, 850, 480)
f1_sheet = fraction(f1, sheet_box, SHEET)
base_sheet = fraction(base, sheet_box, SHEET)
diff = ImageChops.difference(
    base.crop((ox + sheet_box[0], oy + sheet_box[1], ox + sheet_box[2], oy + sheet_box[3])),
    f1.crop((ox + sheet_box[0], oy + sheet_box[1], ox + sheet_box[2], oy + sheet_box[3])),
).convert("L")
changed = sum(1 for value in diff.getdata() if value > 20) / (diff.width * diff.height)
sheet_shot = os.path.join(crop_dir, "u2-slot-shell-overlay.png")
f1.crop((ox + sheet_box[0], oy + sheet_box[1], ox + sheet_box[2], oy + sheet_box[3])).resize(
    ((sheet_box[2] - sheet_box[0]) * 2, (sheet_box[3] - sheet_box[1]) * 2), Image.NEAREST).save(sheet_shot)
if f1_sheet >= 0.30 and base_sheet <= 0.01 and changed >= 0.40:
    print(f"  PASS  {'shell.overlay':16s} 面板底色 {f1_sheet:6.2%}（关：{base_sheet:.2%}）· on/off 差分 {changed:.2%} · 能看到：居中 520 宽的键位表（8 条真实绑定） · {sheet_shot}")
else:
    print(f"  FAIL  {'shell.overlay':16s} 面板底色 {f1_sheet:.2%} / 关时 {base_sheet:.2%} / 差分 {changed:.2%}（面板没画在浮层槽里）· {sheet_shot}")
    bad += 1

sys.exit(1 if bad else 0)
PY
then
  pass=$((pass + 10))
  ok "10 个槽全部有内容（明细见上）"
else
  bad "有槽没画出来（明细见上）"
fi

step "汇总"
echo "  PASS=$pass FAIL=$fail"
echo "  证据：$BASE_PNG（默认槽）、$F1_PNG（F1 浮层）、$CROP_DIR/u2-slot-*.png（逐槽裁剪）"
echo "        日志：$OUT/app-base.log、$OUT/app-f1.log、$OUT/serve.log"
[ "$fail" -eq 0 ]
