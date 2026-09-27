#!/bin/sh
set -eu

POSTGRES_DB_VALUE="${POSTGRES_DB:-oke_gaas_ci}"
POSTGRES_USER_VALUE="${POSTGRES_USER:-oke_gaas_ci}"
POSTGRES_PASSWORD_VALUE="${POSTGRES_PASSWORD:-ci-database-password-0123456789abcdef}"
ADMIN_API_KEY_VALUE="${OKE_GAAS_ADMIN_API_KEY:-ci-admin-api-key-0123456789abcdef}"
HTTP_PORT_VALUE="${OKE_GAAS_HTTP_PORT:-18080}"

assert_example_secret_blank() {
  key="$1"
  expected="${key}="
  actual="$(grep "^${key}=" .env.example || true)"
  if [ "$actual" != "$expected" ]; then
    echo ".env.example must define ${key} exactly once and keep it blank"
    exit 1
  fi
}

assert_example_secret_blank POSTGRES_PASSWORD
assert_example_secret_blank OKE_GAAS_ADMIN_API_KEY

if POSTGRES_DB="$POSTGRES_DB_VALUE" \
  POSTGRES_USER="$POSTGRES_USER_VALUE" \
  POSTGRES_PASSWORD= \
  OKE_GAAS_ADMIN_API_KEY="$ADMIN_API_KEY_VALUE" \
  OKE_GAAS_HTTP_PORT="$HTTP_PORT_VALUE" \
  docker compose config >/dev/null 2>&1; then
  echo "docker compose config unexpectedly accepted an empty POSTGRES_PASSWORD"
  exit 1
fi

if POSTGRES_DB="$POSTGRES_DB_VALUE" \
  POSTGRES_USER="$POSTGRES_USER_VALUE" \
  POSTGRES_PASSWORD="$POSTGRES_PASSWORD_VALUE" \
  OKE_GAAS_ADMIN_API_KEY= \
  OKE_GAAS_HTTP_PORT="$HTTP_PORT_VALUE" \
  docker compose config >/dev/null 2>&1; then
  echo "docker compose config unexpectedly accepted an empty OKE_GAAS_ADMIN_API_KEY"
  exit 1
fi

export POSTGRES_DB="$POSTGRES_DB_VALUE"
export POSTGRES_USER="$POSTGRES_USER_VALUE"
export POSTGRES_PASSWORD="$POSTGRES_PASSWORD_VALUE"
export OKE_GAAS_ADMIN_API_KEY="$ADMIN_API_KEY_VALUE"
export OKE_GAAS_HTTP_PORT="$HTTP_PORT_VALUE"

cleanup() {
  docker compose down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM
cleanup

docker compose config >/dev/null
docker compose up -d --build

healthy=0
attempt=1
while [ "$attempt" -le 60 ]; do
  if curl --fail --silent --show-error "http://127.0.0.1:${OKE_GAAS_HTTP_PORT}/health" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 2
  attempt=$((attempt + 1))
done

if [ "$healthy" -ne 1 ]; then
  docker compose ps -a
  docker compose logs --no-color
  exit 1
fi

docker compose exec -T api sh -c 'test "$(id -u)" -ne 0'

curl --fail-with-body --silent --show-error \
  --request POST \
  --header "Authorization: Bearer ${OKE_GAAS_ADMIN_API_KEY}" \
  --header "Content-Type: application/json" \
  --data '{"name":"Self-host CI"}' \
  "http://127.0.0.1:${OKE_GAAS_HTTP_PORT}/v1/projects" \
  >/dev/null
