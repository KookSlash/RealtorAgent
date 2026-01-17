#!/usr/bin/env bash

stack_env() {
  STACK="${STACK:-run}"
  export STACK

  case "${STACK}" in
    test)
      LOCALSTACK_PORT="${LOCALSTACK_PORT_TEST:-14566}"
      POSTGRES_PORT="${POSTGRES_PORT_TEST:-15432}"
      ;;
    *)
      LOCALSTACK_PORT="${LOCALSTACK_PORT_RUN:-${LOCALSTACK_PORT:-4566}}"
      POSTGRES_PORT="${POSTGRES_PORT_RUN:-${POSTGRES_PORT:-5432}}"
      ;;
  esac

  export LOCALSTACK_PORT POSTGRES_PORT
}
