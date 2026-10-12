#!/usr/bin/env bash
#
# 契约自动登记（knowledge-base → 服务中心）：
#
#   代码里的 swag 注解 ──swag init──▶ docs/swagger.json ──register.sh──▶ 注册中心
#
# 脚本本体在服务中心仓库 `client/ci/register-go-service.sh`。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

log() { echo "[register-contract] $*"; }
die() { echo "[register-contract][错误] $*" >&2; exit 1; }

SERVICE_NAME="${SERVICE_NAME:-knowledge-base}"
DEPARTMENT_ID="${DEPARTMENT_ID:-D0005}"
INSTANCES="${INSTANCES:-127.0.0.1:4360}"
REGISTRY_URL="${REGISTRY_URL:-http://127.0.0.1:4240}"
REGISTRY_NS="${REGISTRY_NS:-default}"
OWNER="${OWNER:-kaulie}"
HEALTH_PATH="${HEALTH_PATH:-/health}"
TAGS="${TAGS:-knowledge}"
GIT_REPO="${GIT_REPO:-https://github.com/kaulie/knowledge-base}"
SWAG_MAIN="${SWAG_MAIN:-cmd/knowledge-based/main.go}"
SWAG_OUT="${SWAG_OUT:-docs}"
SWAG_ARGS="${SWAG_ARGS:---outputTypes json}"
REGISTRY_REPO="${REGISTRY_REPO:-https://github.com/kaulie/service-registry}"

if [ -z "${VERSION:-}" ]; then
  VERSION="${APP_VERSION:-}"
fi
if [ -z "${VERSION}" ]; then
  VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
fi

if ! command -v swag >/dev/null 2>&1; then
  GOBIN_DIR="$(go env GOPATH 2>/dev/null)/bin"
  if [ -x "${GOBIN_DIR}/swag" ]; then
    PATH="${GOBIN_DIR}:${PATH}"
    export PATH
  fi
fi

find_client_ci() {
  local cand
  for cand in \
    "${REGISTRY_CLIENT_CI:-}" \
    "${SERVICE_REGISTRY_DIR:+${SERVICE_REGISTRY_DIR}/client/ci/register-go-service.sh}" \
    "${ROOT}/client/ci/register-go-service.sh" \
    "${HOME}/Projects/service-registry/client/ci/register-go-service.sh" \
    ${HOME}/agent-workspace/service-registry/*/client/ci/register-go-service.sh \
    /tmp/event-center/client/ci/register-go-service.sh
  do
    [ -n "${cand}" ] && [ -f "${cand}" ] && { echo "${cand}"; return 0; }
  done
  return 1
}

CLIENT_CI=""
if ! CLIENT_CI="$(find_client_ci)"; then
  CACHE="${REGISTRY_CLIENT_CACHE:-${TMPDIR:-/tmp}/knowledge-base-registry-client}"
  if [ -f "${CACHE}/client/ci/register-go-service.sh" ]; then
    log "复用缓存的服务中心脚本：${CACHE}"
  else
    log "本机没有服务中心客户端脚本，浅克隆 ${REGISTRY_REPO} → ${CACHE}"
    rm -rf "${CACHE}"
    git clone --depth 1 "${REGISTRY_REPO}" "${CACHE}" >/dev/null 2>&1 ||
      die "克隆失败：${REGISTRY_REPO}（可用 REGISTRY_CLIENT_CI 直接指定脚本路径）"
  fi
  CLIENT_CI="${CACHE}/client/ci/register-go-service.sh"
fi
log "使用服务中心脚本：${CLIENT_CI}"

log "登记 ${SERVICE_NAME}：部门=${DEPARTMENT_ID} 实例=${INSTANCES} 版本=${VERSION} → ${REGISTRY_URL}"
SERVICE_NAME="${SERVICE_NAME}" \
DEPARTMENT_ID="${DEPARTMENT_ID}" \
INSTANCES="${INSTANCES}" \
REGISTRY_URL="${REGISTRY_URL}" \
REGISTRY_NS="${REGISTRY_NS}" \
VERSION="${VERSION}" \
OWNER="${OWNER}" \
HEALTH_PATH="${HEALTH_PATH}" \
TAGS="${TAGS}" \
GIT_REPO="${GIT_REPO}" \
SWAG_MAIN="${SWAG_MAIN}" \
SWAG_OUT="${SWAG_OUT}" \
SWAG_ARGS="${SWAG_ARGS}" \
  bash "${CLIENT_CI}"
