#!/usr/bin/env bash
#
# 启动 knowledge-base —— 遵循「agent-control-plane-deployment」部署系统规范。
#
# 控制面以 restartCmd 调用：cwd = runtimeDir，且注入
#   SERVICE_PORT = 服务契约端口（优先）
#   PORT         = healthUrl 端口（SERVICE_PORT 未设时用）
#   RUNTIME_DIR  = runtimeDir
#   APP_VERSION  = 本次部署的 8 位短 hash
#
# 监听端口优先级：SERVICE_PORT > PORT > 4360。
# 服务自身只认 SERVICE_PORT / KNOWLEDGE_BASE_ADDR，不用裸 PORT
# （同机其它运行时会导出 PORT，直接继承会顶到别人的端口上）。
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${RUNTIME_DIR:-$(cd "${DIR}/.." && pwd)}"
PLATFORM_PORT="${SERVICE_PORT:-${PORT:-4360}}"
APP_VERSION="${APP_VERSION:-dev}"

BIN="${RUNTIME_DIR}/bin/knowledge-based"
BACKEND="${RUNTIME_DIR}/backend"
ENV_FILE="${BACKEND}/.env"
DATA_DIR="${BACKEND}/data"
PID_FILE="${BACKEND}/runtime.pid"
LOG_FILE="${BACKEND}/server.log"

log() { echo "[start] $*"; }
die() { echo "[start][错误] $*" >&2; exit 1; }

case "${PLATFORM_PORT}" in
  *[!0-9]*|"") die "端口必须是 1-65535 的整数，当前：${PLATFORM_PORT}" ;;
esac
if [ "${PLATFORM_PORT}" -lt 1 ] || [ "${PLATFORM_PORT}" -gt 65535 ]; then
  die "端口超出范围：${PLATFORM_PORT}"
fi

[ -x "${BIN}" ] || die "缺少可执行文件 ${BIN}（发版包内容不完整？）"

mkdir -p "${BACKEND}" "${DATA_DIR}"

if [ ! -f "${ENV_FILE}" ]; then
  umask 077
  cat > "${ENV_FILE}" <<'EOF'
# knowledge-base 运行期配置（首次启动自动生成，权限 600，请勿提交到 git）
# 监听端口由 SERVICE_PORT / PORT 决定，不写在这里。
# 默认绑 127.0.0.1。远程对外时取消下一行注释：
# KB_BIND=0.0.0.0
EOF
  chmod 600 "${ENV_FILE}"
  log "已生成 ${ENV_FILE}"
fi

# shellcheck disable=SC1090
set -a; . "${ENV_FILE}"; set +a

export SERVICE_PORT="${PLATFORM_PORT}"
BIND="${KB_BIND:-127.0.0.1}"
export KNOWLEDGE_BASE_ADDR="${BIND}:${SERVICE_PORT}"
export APP_VERSION
HEALTH_HOST="${BIND}"
if [ "${HEALTH_HOST}" = "0.0.0.0" ] || [ "${HEALTH_HOST}" = "::" ]; then
  HEALTH_HOST="127.0.0.1"
fi
HEALTH_URL="http://${HEALTH_HOST}:${SERVICE_PORT}/health"

if [ -f "${PID_FILE}" ]; then
  old="$(tr -d '[:space:]' < "${PID_FILE}" || true)"
  if [ -n "${old}" ] && kill -0 "${old}" 2>/dev/null; then
    log "已在运行 pid=${old}"
    exit 0
  fi
  rm -f "${PID_FILE}"
fi

# rsync --delete 会清掉 pid 文件，旧进程可能还占着契约端口。
if command -v lsof >/dev/null 2>&1; then
  holder="$(lsof -nP -iTCP:"${SERVICE_PORT}" -sTCP:LISTEN -t 2>/dev/null | head -1 || true)"
  if [ -n "${holder}" ]; then
    holder_cmd="$(ps -o command= -p "${holder}" 2>/dev/null || true)"
    case "${holder_cmd}" in
    *"${BIN}"*)
      log "端口 ${SERVICE_PORT} 上是本 runtime 的残留进程 pid=${holder}，先执行 stop"
      bash "${DIR}/stop.sh" || die "无法停掉占用端口 ${SERVICE_PORT} 的本进程 pid=${holder}"
      ;;
    *)
      die "端口 ${SERVICE_PORT} 已被 pid=${holder} 占用：${holder_cmd}"
      ;;
    esac
  fi
fi

log "启动 部署版本=${APP_VERSION} 监听=${KNOWLEDGE_BASE_ADDR} 日志=${LOG_FILE}"
nohup "${BIN}" >> "${LOG_FILE}" 2>&1 &
echo $! > "${PID_FILE}"
pid="$(cat "${PID_FILE}")"

for _ in $(seq 1 40); do
  if ! kill -0 "${pid}" 2>/dev/null; then
    rm -f "${PID_FILE}"
    echo "[start][错误] 进程已退出，最近日志：" >&2
    tail -20 "${LOG_FILE}" >&2 || true
    exit 1
  fi
  if curl -fsS -m 2 "${HEALTH_URL}" >/dev/null 2>&1; then
    log "启动成功 pid=${pid} health=${HEALTH_URL}"
    exit 0
  fi
  sleep 0.5
done

echo "[start][错误] 20s 内 ${HEALTH_URL} 未就绪，最近日志：" >&2
tail -20 "${LOG_FILE}" >&2 || true
kill "${pid}" 2>/dev/null || true
rm -f "${PID_FILE}"
exit 1
