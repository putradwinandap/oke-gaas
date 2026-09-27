#!/bin/sh
set -eu

export POSTGRES_DB="${POSTGRES_DB:-oke_gaas_ci}"
export POSTGRES_USER="${POSTGRES_USER:-oke_gaas_ci}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-ci-database-password-0123456789abcdef}"
export OKE_GAAS_ADMIN_API_KEY="${OKE_GAAS_ADMIN_API_KEY:-ci-admin-api-key-0123456789abcdef}"
export OKE_GAAS_HTTP_PORT="${OKE_GAAS_HTTP_PORT:-18080}"

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
