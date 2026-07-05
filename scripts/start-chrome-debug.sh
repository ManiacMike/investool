#!/usr/bin/env bash
# 启动一个带「远程调试端口」的独立 Chrome,供研报采集(douyin_to_report.mjs / CDP)驱动。
#
# 用独立的 user-data-dir,不影响你日常的 Chrome。首次启动后请在该窗口里
# 登录「豆包」与「抖音」,登录态会记在该目录,后续再跑同一命令即可复用。
#
# 用法:
#   scripts/start-chrome-debug.sh            # 默认端口 9222
#   CDP_PORT=9333 scripts/start-chrome-debug.sh
#
# 环境变量:
#   CDP_PORT   远程调试端口(默认 9222,需与后端 CDP_PORT 一致)
#   PROFILE_DIR  独立配置目录(默认 ~/chrome-douyin-debug)

set -euo pipefail

PORT="${CDP_PORT:-9222}"
PROFILE_DIR="${PROFILE_DIR:-$HOME/chrome-douyin-debug}"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

if [ ! -x "$CHROME" ]; then
  echo "✗ 未找到 Chrome:$CHROME" >&2
  echo "  请改用你的 Chrome 实际安装路径,或安装 Google Chrome。" >&2
  exit 1
fi

# 端口已就绪则直接复用,不重复启动
if curl -s -m 1 "http://localhost:${PORT}/json/version" >/dev/null 2>&1; then
  echo "✓ 端口 ${PORT} 已有调试 Chrome 在跑,直接复用。"
  curl -s "http://localhost:${PORT}/json/version" | sed 's/.*"webSocketDebuggerUrl":"\([^"]*\)".*/  ws: \1/'
  echo "  现在可在页面点「采集研报」。若尚未登录,请在该 Chrome 窗口登录豆包/抖音。"
  exit 0
fi

echo "→ 启动调试 Chrome(端口 ${PORT},配置目录 ${PROFILE_DIR}) ..."
mkdir -p "$PROFILE_DIR"

# 后台启动,日志丢弃;新开豆包页方便登录
"$CHROME" \
  --remote-debugging-port="${PORT}" \
  --user-data-dir="${PROFILE_DIR}" \
  --no-first-run \
  --no-default-browser-check \
  "https://www.doubao.com/chat/" \
  >/dev/null 2>&1 &

# 等端口就绪(最多 ~15s)
for i in $(seq 1 30); do
  if curl -s -m 1 "http://localhost:${PORT}/json/version" >/dev/null 2>&1; then
    echo "✓ 调试端口 ${PORT} 就绪。"
    echo
    echo "  下一步:在刚弹出的 Chrome 窗口里"
    echo "    1) 登录「豆包」(https://www.doubao.com)"
    echo "    2) 新开标签登录「抖音」(https://www.douyin.com,主页枚举需要)"
    echo "  然后回到 zen-research-report 页面点「采集研报」。"
    exit 0
  fi
  sleep 0.5
done

echo "✗ 等待端口 ${PORT} 超时。请确认 Chrome 是否弹出;若日常 Chrome 未退出可能占用冲突。" >&2
exit 1
