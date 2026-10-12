#!/usr/bin/env bash
#
# 打包脚本 —— 遵循「agent-control-plane-deployment」部署系统规范。
#
# 控制面流水线：POST /api/deploy-notify {serviceId:"knowledge-base"}
#
# 约定：cwd = 仓库根；APP_VERSION = <8 位短 hash>；必须产出 outputs/，
# 其中必须包含 scripts/restart.sh。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT}"

VERSION="${APP_VERSION:-dev}"
OUT="${ROOT}/outputs"

echo "[build] knowledge-base version=${VERSION}"
rm -rf "${OUT}"
mkdir -p "${OUT}/bin" "${OUT}/scripts"

BUILD_CACHE="${KB_BUILD_CACHE:-${HOME:-/tmp}/.cache/knowledge-base/build}"
export GOMODCACHE="${BUILD_CACHE}/gomodcache"
export GOCACHE="${BUILD_CACHE}/gocache"
export GOPATH="${BUILD_CACHE}/gopath"
export GOTMPDIR="${BUILD_CACHE}/gotmp"
mkdir -p "${GOMODCACHE}" "${GOCACHE}" "${GOPATH}" "${GOTMPDIR}"
[ -n "${GOPROXY:-}" ] || export GOPROXY="https://goproxy.cn,direct"

LDFLAGS="-s -w -X main.version=${VERSION}"

CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" \
  -o "${OUT}/bin/knowledge-based" ./cmd/knowledge-based

cp "${ROOT}/scripts/start.sh" "${ROOT}/scripts/stop.sh" "${ROOT}/scripts/restart.sh" \
  "${OUT}/scripts/"

chmod +x "${OUT}/bin/"* "${OUT}/scripts/"*.sh

[ -f "${OUT}/scripts/restart.sh" ] || { echo "[build][错误] outputs/ 缺少 scripts/restart.sh" >&2; exit 1; }

if [ "${SKIP_REGISTER_CONTRACT:-0}" = "1" ]; then
  echo "[build] 跳过契约登记（SKIP_REGISTER_CONTRACT=1）"
else
  echo "[build] 登记服务契约到服务中心（注解 → OpenAPI）"
  if ! VERSION="${VERSION}" bash "${ROOT}/scripts/register-contract.sh"; then
    if [ "${REGISTER_CONTRACT_STRICT:-0}" = "1" ]; then
      echo "[build][错误] 契约登记失败（REGISTER_CONTRACT_STRICT=1）" >&2
      exit 1
    fi
    echo "[build][警告] 契约登记失败（服务中心可能不可达）；发版继续，可事后补跑 scripts/register-contract.sh" >&2
  fi
fi

echo "[build] outputs 就绪："
ls -1 "${OUT}" "${OUT}/bin" "${OUT}/scripts" | sed 's/^/  /'
