#!/usr/bin/env bash
# Runs all Go tests against PostgreSQL, SeaweedFS, and Valkey in a separate Compose project.
# Usage: test/integration.sh [go test arguments]
set -euo pipefail
cd "$(dirname "$0")/.."

export COMPOSE_PROJECT_NAME=ginnungagap-test
export POSTGRES_PASSWORD=test S3_ACCESS_KEY=test S3_SECRET_KEY=test-secret
compose() { docker compose -f compose.yaml -f test/compose.test.yaml "$@"; }
trap 'compose down -v >/dev/null 2>&1' EXIT

compose up -d --wait postgres valkey >/dev/null 2>&1
compose up -d seaweedfs >/dev/null 2>&1

export TEST_DATABASE_URL="postgres://ginnungagap:test@$(compose port postgres 5432)/ginnungagap"
export TEST_S3_ENDPOINT="$(compose port seaweedfs 8333)"
export TEST_S3_ACCESS_KEY=test TEST_S3_SECRET_KEY=test-secret TEST_S3_BUCKET=ginnungagap
export TEST_VALKEY_ADDR="$(compose port valkey 6379)"

# SeaweedFS has no health check in compose.yaml. Wait until it creates the bucket.
for _ in $(seq 60); do
  compose logs seaweedfs 2>&1 | grep -q "created bucket ginnungagap" && break
  sleep 1
done

go test -count=1 "${@:-./...}"
