#!/usr/bin/env bash
# headcheck.sh —— 验证 **HEAD 单独**能不能编译。
#
# # 为什么需要这个
#
# 这个仓库的"工作树能编译"是**假命题**：同时有三四个会话在改，
# 任何时刻工作树里都有一堆未跟踪的新文件在补已提交代码的窟窿。
# 所以 `go build ./...` 绿了，不代表别人 clone 下来能 build。
#
# 2026-11-04 真事故：GoalBar 那次提交 `git add pluginapi/plugin.go`（按文件名），
# 把别人未提交的 `MessageMetadata` 引用带上去了，而定义它的
# `conversation.go` 还躺在未跟踪里——HEAD 红了整整几轮没人发现。
#
# # 为什么用 git archive 而不是 git stash
#
# `git stash -u` 会把这台机器上**别人**的未提交改动一起收走。
# 没收干净或 pop 失败，那些改动就没了。`git archive` 只读 HEAD，**不碰工作树**。
#
# # 为什么要放在兄弟目录
#
# go.mod 里有 `replace ../dsh-ui-plugins` 之类的相对路径。
# 解到 /tmp 下那些 replace 全部解析不了，会得到一堆假的红。
#
# 用法：bash scripts/headcheck.sh [build|test]
set -uo pipefail

mode="${1:-build}"
cd "$(dirname "$0")/.." || exit 2
root=$(pwd)
# **兄弟目录**：和仓库同级，相对 replace 才能解析。
out="$root/../dsh-ui-headcheck.$$"

cleanup() { rm -rf "$out"; }
trap cleanup EXIT

mkdir -p "$out" || { echo "建不了临时目录 $out"; exit 2; }
if ! git archive HEAD | tar -x -C "$out"; then
  echo "解不出 HEAD"
  exit 2
fi

export PATH="/c/Program Files/Go/bin:$PATH"
cd "$out" || exit 2
echo "== 干净检出：$out =="

CGO_ENABLED=0 go build ./... 2>&1 | head -20
build_rc=${PIPESTATUS[0]}
if [ "$build_rc" -ne 0 ]; then
  echo "FAIL  干净检出 build 不过（工作树可能是绿的，但 HEAD 是红的）"
  exit 1
fi
echo "PASS  干净检出 build"

if [ "$mode" = "test" ]; then
  CGO_ENABLED=0 go test ./... -count=1 2>&1 | grep -E "^(ok|FAIL|--- FAIL)" | head -30
  test_rc=${PIPESTATUS[0]}
  if [ "$test_rc" -ne 0 ]; then
    echo "FAIL  干净检出 test 不过"
    exit 1
  fi
  echo "PASS  干净检出 test"
fi
exit 0
