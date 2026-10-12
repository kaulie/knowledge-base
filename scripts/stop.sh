#!/usr/bin/env bash
#
# 停止 knowledge-base（TERM → 等待 → KILL）。
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${RUNTIME_DIR:-$(cd "${DIR}/.." && pwd)}"
BIN="${RUNTIME_DIR}/bin/knowledge-based"
PID_FILE="${RUNTIME_DIR}/backend/runtime.pid"
PORT="${SERVICE_PORT:-${PORT:-4360}}"
WAIT_SECONDS="${KB_STOP_TIMEOUT:-15}"

log() { echo "[stop] $*"; }

is_our_process() {
  local pid="$1" cmd=""
  [ -n "${pid}" ] || return 1
  cmd="$(ps -o command= -p "${pid}" 2>/dev/null || true)"
  case "${cmd}" in
  *"${BIN}"*) return 0 ;;
  esac
  return 1
}

is_running() {
  local pid="$1"
  [ -n "${pid}" ] || return 1
  kill -0 "${pid}" 2>/dev/null
}

stop_pid() {
  local pid="$1"
  if ! is_running "${pid}"; then
    return 0
  fi
  log "TERM → pid=${pid}"
  kill "${pid}" 2>/dev/null || true
  local attempts="$(( WAIT_SECONDS * 2 ))"
  local _
  for _ in $(seq 1 "${attempts}"); do
    if ! is_running "${pid}"; then
      log "已停止 pid=${pid}"
      return 0
    fi
    sleep 0.5
  done
  log "未在 ${WAIT_SECONDS}s 内退出，KILL → pid=${pid}"
  kill -9 "${pid}" 2>/dev/null || true
  log "已强制停止 pid=${pid}"
}

stopped=0

if [ -f "${PID_FILE}" ]; then
  pid="$(tr -d '[:space:]' < "${PID_FILE}" || true)"
  if is_running "${pid}"; then
    stop_pid "${pid}"
    stopped=1
  else
    log "进程 ${pid:-?} 不存在，清理 pid 文件"
  fi
  rm -f "${PID_FILE}"
else
  log "没有 pid 文件"
fi

if command -v lsof >/dev/null 2>&1; then
  case "${PORT}" in
    *[!0-9]*|"") ;;
    *)
      for holder in $(lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN -t 2>/dev/null || true); do
        if is_our_process "${holder}"; then
          log "端口 ${PORT} 上是本 runtime 的残留进程 pid=${holder}，继续停"
          stop_pid "${holder}"
          stopped=1
        fi
      done
      ;;
  esac
fi

rm -f "${PID_FILE}"
if [ "${stopped}" -eq 0 ]; then
  log "视为未运行"
fi
