#!/usr/bin/env bash

compose_project() {
  local stack="${STACK:-run}"
  if [[ "${stack}" == "run" || -z "${stack}" ]]; then
    printf "default"
    return 0
  fi
  printf "realtoragent-%s" "${stack}"
}

compose_infra() {
  if [[ "${STACK:-run}" == "run" || -z "${STACK:-}" ]]; then
    docker compose -f "${ROOT_DIR}/infra/docker-compose.yml" "$@"
    return
  fi
  docker compose -p "$(compose_project)" -f "${ROOT_DIR}/infra/docker-compose.yml" "$@"
}

compose_infra_readapi() {
  if [[ "${STACK:-run}" == "run" || -z "${STACK:-}" ]]; then
    docker compose -f "${ROOT_DIR}/infra/docker-compose.yml" \
      -f "${ROOT_DIR}/infra/docker-compose.readapi.yml" \
      "$@"
    return
  fi
  docker compose -p "$(compose_project)" \
    -f "${ROOT_DIR}/infra/docker-compose.yml" \
    -f "${ROOT_DIR}/infra/docker-compose.readapi.yml" \
    "$@"
}
