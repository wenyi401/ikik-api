#!/bin/bash

set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "${TEST_DIR}/.." && pwd)"
SCRIPT="${DEPLOY_DIR}/apple-container.sh"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ikik-api-apple-test.XXXXXX")"
STATE_DIR="${TEST_ROOT}/state"
ENV_FILE="${TEST_ROOT}/ikik-api.env"

cleanup() {
    rm -rf "${TEST_ROOT}"
}
trap cleanup EXIT

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_exists() {
    [[ -e "$1" ]] || fail "Expected path to exist: $1"
}

assert_missing() {
    [[ ! -e "$1" ]] || fail "Expected path to be absent: $1"
}

export FAKE_CONTAINER_STATE="${STATE_DIR}"
mkdir -p "${TEST_ROOT}/bin"
ln -s "${TEST_DIR}/fixtures/bin/container" "${TEST_ROOT}/bin/plutil"
export PATH="${TEST_ROOT}/bin:${TEST_DIR}/fixtures/bin:${PATH}"
export SUB2API_ENV_FILE="${ENV_FILE}"

mkdir -p "${STATE_DIR}"

"${SCRIPT}" init
if stat -c '%a' "${ENV_FILE}" >/dev/null 2>&1; then
    ENV_FILE_MODE="$(stat -c '%a' "${ENV_FILE}")"
else
    ENV_FILE_MODE="$(stat -f '%Lp' "${ENV_FILE}")"
fi
[[ "${ENV_FILE_MODE}" == "600" ]] || fail "init did not create a mode-600 env file"
grep -q '^POSTGRES_PASSWORD=change_this_secure_password$' "${ENV_FILE}" && fail "init retained the placeholder password"

chmod 644 "${ENV_FILE}"
if "${SCRIPT}" up >/dev/null 2>&1; then
    fail "up accepted an insecure env file"
fi
chmod 600 "${ENV_FILE}"

"${SCRIPT}" up
assert_exists "${STATE_DIR}/containers/ikik-api-apple"
assert_exists "${STATE_DIR}/containers/ikik-api-apple-postgres"
assert_exists "${STATE_DIR}/containers/ikik-api-apple-redis"
assert_exists "${STATE_DIR}/running/ikik-api-apple"
"${SCRIPT}" status >/dev/null

"${SCRIPT}" up --recreate
assert_exists "${STATE_DIR}/running/ikik-api-apple"
"${SCRIPT}" down
assert_missing "${STATE_DIR}/running/ikik-api-apple"
assert_missing "${STATE_DIR}/running/ikik-api-apple-postgres"
assert_missing "${STATE_DIR}/running/ikik-api-apple-redis"

"${SCRIPT}" destroy --yes
assert_missing "${STATE_DIR}/containers/ikik-api-apple"
assert_missing "${STATE_DIR}/networks/ikik-api-apple"
assert_exists "${STATE_DIR}/volumes/ikik-api-apple-data"

"${SCRIPT}" up
"${SCRIPT}" destroy --volumes --yes
assert_missing "${STATE_DIR}/volumes/ikik-api-apple-data"
assert_missing "${STATE_DIR}/volumes/ikik-api-apple-postgres-data"
assert_missing "${STATE_DIR}/volumes/ikik-api-apple-redis-data"

touch "${STATE_DIR}/system-running"
touch "${STATE_DIR}/containers/ikik-api-apple"
touch "${STATE_DIR}/unowned/container/ikik-api-apple"
if "${SCRIPT}" status >/dev/null 2>&1; then
    fail "status accepted an unowned same-name container"
fi

printf 'Apple container lifecycle tests passed.\n'
