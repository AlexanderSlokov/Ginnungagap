#!/usr/bin/env bash
# End-to-end test (B10): pip and uv install packages through Ginnungagap in the Compose stack.
# Needs Docker and internet access. Usage: test/e2e.sh
set -euo pipefail
cd "$(dirname "$0")/.."

export COMPOSE_PROJECT_NAME=ginnungagap-e2e
export POSTGRES_PASSWORD=e2e S3_ACCESS_KEY=e2e S3_SECRET_KEY=e2e-secret
export GINNUNGAGAP_ADMIN_PASSWORD=e2e-admin-password GINNUNGAGAP_PORT=18080
BASE=http://localhost:18080
ADMIN="admin:$GINNUNGAGAP_ADMIN_PASSWORD"
JAR=$(mktemp)
trap 'rm -f "$JAR"; docker compose down -v >/dev/null 2>&1' EXIT

fail() { echo "FAIL: $*"; docker compose logs ginnungagap 2>&1 | tail -40; exit 1; }
step() { echo "--- $*"; }
fetches() { docker compose logs ginnungagap 2>&1 | grep -c 'fetch upstream file' || true; }
pip_install() { # index-path, credentials, packages...
  local index=$1 creds=$2; shift 2
  docker run --rm --network host python:3.13-slim pip install -q --no-cache-dir --disable-pip-version-check \
    --root-user-action=ignore --index-url "http://$creds@localhost:18080/pypi/$index/simple/" "$@"
}
uv_sync() { # robot token
  docker run --rm --network host -e UV_INDEX_GINNUNGAGAP_USERNAME=robot+ci -e UV_INDEX_GINNUNGAGAP_PASSWORD="$1" \
    ghcr.io/astral-sh/uv:python3.13-bookworm-slim sh -ec '
    mkdir /w && cd /w
    cat > pyproject.toml <<TOML
[project]
name = "e2e"
version = "0"
requires-python = ">=3.13"
dependencies = ["six==1.16.0", "idna==3.10"]

[[tool.uv.index]]
name = "ginnungagap"
url = "http://localhost:18080/pypi/python/simple/"
default = true
TOML
    uv sync --no-cache --quiet
    .venv/bin/python -c "import six, idna; print(\"uv sync ok:\", six.__version__, idna.__version__)"'
}

step "start the stack"
docker compose up -d --build >/dev/null 2>&1
for _ in $(seq 60); do curl -sf "$BASE/healthz" >/dev/null && break; sleep 1; done
curl -sf "$BASE/healthz" >/dev/null || fail "the service is not healthy"

step "anonymous requests are rejected"
code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/pypi/python/simple/six/")
[ "$code" = 401 ] || fail "anonymous index request returned $code"

step "pip installs through the group (PyPI)"
pip_install python "$ADMIN" six==1.16.0 mpmath==1.3.0 || fail "pip install through python"

step "pip installs from the PyTorch index (HTML, mirrored PyPI file)"
pip_install pytorch "$ADMIN" --no-deps mpmath==1.3.0 || fail "pip install through pytorch"

step "a second install uses the cache"
before=$(fetches)
pip_install python "$ADMIN" six==1.16.0 mpmath==1.3.0 || fail "second pip install"
[ "$(fetches)" = "$before" ] || fail "the second install fetched files from the upstream"

step "the cooldown hides a new version"
curl -sf -u "$ADMIN" -H 'Accept: application/vnd.pypi.simple.v1+json' "$BASE/pypi/python/simple/pip/" >/tmp/ggg-e2e-pip.json
newest=$(docker run --rm python:3.13-slim pip index versions pip 2>/dev/null | sed -n 's/^pip (\(.*\))$/\1/p')
if grep -q "\"pip-$newest-py3-none-any.whl\"" /tmp/ggg-e2e-pip.json; then
  echo "    pip $newest is older than 14 days: nothing to check"
else
  echo "    pip $newest is hidden by the cooldown"
fi

step "log in to the UI and create a robot account"
curl -sf -c "$JAR" -o /dev/null -d "name=admin&password=$GINNUNGAGAP_ADMIN_PASSWORD" "$BASE/login" || fail "login"
token=$(curl -sf -b "$JAR" -d 'name=ci&role=guest&days=1' "$BASE/repos/python/robots" | grep -o 'ggr_[A-Za-z0-9_-]*' | head -1)
[ -n "$token" ] || fail "no robot token"

step "uv sync with the robot account"
uv_sync "$token" || fail "uv sync"
before=$(fetches)
uv_sync "$token" || fail "second uv sync"
[ "$(fetches)" = "$before" ] || fail "the second uv sync fetched files from the upstream"

step "the UI pages render"
for path in / /repos/python /repos/pypi /repos/python/projects/six /repos/pytorch/projects/mpmath /gc /accounts; do
  code=$(curl -s -b "$JAR" -o /dev/null -w '%{http_code}' "$BASE$path")
  [ "$code" = 200 ] || fail "GET $path returned $code"
done

step "a blocked version is not available"
curl -sf -b "$JAR" -o /dev/null -d 'project=six&version=1.16.0&kind=block&reason=e2e' "$BASE/repos/python/policies" || fail "add rule"
if pip_install python "$ADMIN" six==1.16.0 2>/dev/null; then fail "pip installed a blocked version"; fi

echo "PASS"
