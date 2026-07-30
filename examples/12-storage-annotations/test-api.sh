#!/usr/bin/env bash
# Copyright © 2026 OpenCHAMI Contributors
# SPDX-License-Identifier: MIT

set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
PROJECT_DIR="${PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DATABASE_URL="${DATABASE_URL:-file:${PROJECT_DIR}/data.db?_fk=1}"
RUN_ID="${RUN_ID:-$$}"
response_dir="$(mktemp -d "${TMPDIR:-/tmp}/fabrica-example-12-api.XXXXXX")"
BODY=''

cleanup() {
	rm -rf "${response_dir}"
}
trap cleanup EXIT INT TERM

for command in curl jq go; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		printf 'required command not found: %s\n' "${command}" >&2
		exit 1
	fi
done

request() {
	local expected="$1"
	local method="$2"
	local path="$3"
	local data="${4:-}"
	local response_file="${response_dir}/response.json"
	local status
	local curl_status
	local -a args=(--fail-with-body --silent --show-error --output "${response_file}" --write-out '%{http_code}' --request "${method}" "${BASE_URL}${path}")
	: >"${response_file}"
	if [[ -n "${data}" ]]; then
		args+=(--header 'Content-Type: application/json' --data "${data}")
	fi

	set +e
	status="$(curl "${args[@]}")"
	curl_status=$?
	set -e
	BODY="$(<"${response_file}")"

	if [[ "${status}" != "${expected}" ]]; then
		printf 'HTTP receipt: method=%s path=%s expected=%s actual=%s curl=%s body=%s\n' "${method}" "${path}" "${expected}" "${status}" "${curl_status}" "${BODY}" >&2
		exit 1
	fi
	if (( expected < 400 && curl_status != 0 )); then
		printf 'curl unexpectedly failed for HTTP %s\n' "${expected}" >&2
		exit 1
	fi
	if (( expected >= 400 && curl_status != 22 )); then
		printf 'curl failure receipt mismatch for HTTP %s: exit=%s\n' "${expected}" "${curl_status}" >&2
		exit 1
	fi
	printf 'HTTP receipt: method=%s path=%s status=%s shape_checked=true\n' "${method}" "${path}" "${status}"
}

assert_json() {
	local expression="$1"
	local receipt="$2"
	if ! jq -e "${expression}" >/dev/null <<<"${BODY}"; then
		printf 'JSON contract failed: %s body=%s\n' "${receipt}" "${BODY}" >&2
		exit 1
	fi
	printf 'contract receipt: %s=true\n' "${receipt}"
}

request 200 GET /health
assert_json 'type == "object" and .status == "healthy"' health

username="alice-${RUN_ID}"
email="alice-${RUN_ID}@example.com"
name="alice-${RUN_ID}"
old_password='old-password'
new_password='new-password'
old_hint='first private hint'
new_hint='updated private hint'

create_payload="$(jq -nc \
	--arg name "${name}" \
	--arg username "${username}" \
	--arg email "${email}" \
	--arg password "${old_password}" \
	--arg hint "${old_hint}" \
	'{apiVersion:"example.fabrica.dev/v1",kind:"User",metadata:{name:$name,namespace:"example",labels:{team:"platform"},annotations:{purpose:"annotation-demo"}},spec:{username:$username,email:$email,password:$password,recoveryHint:$hint,observedAt:"2026-07-28T12:00:00Z",aliases:["primary","demo"]}}')"

request 201 POST /users "${create_payload}"
assert_json 'type == "object" and (.metadata.uid | type == "string" and length > 0)' create_shape
assert_json '.spec.role == "user" and .spec.active == true and .spec.retries == 3 and .spec.quota == -1 and .spec.score == 1.5' create_persisted_defaults
uid="$(jq -r '.metadata.uid' <<<"${BODY}")"
if ! jq -e --arg password "${old_password}" --arg hint "${old_hint}" '.spec.password == "" and .spec.recoveryHint == "" and .spec.password != $password and .spec.recoveryHint != $hint' >/dev/null <<<"${BODY}"; then
	printf 'create response exposed a sensitive value: %s\n' "${BODY}" >&2
	exit 1
fi
printf 'contract receipt: create_sensitive_zeroed=true plaintext_exposed=false\n'

request 200 GET "/users/${uid}"
assert_json 'type == "object" and .spec.role == "user" and .spec.active == true and .spec.retries == 3 and .spec.quota == -1 and .spec.score == 1.5' typed_defaults
assert_json '.metadata.namespace == "example" and .metadata.labels.team == "platform" and .metadata.annotations.purpose == "annotation-demo"' metadata_roundtrip
assert_json '(.spec.observedAt | type == "string") and (.spec.aliases | type == "array") and (.spec.aliases | all(type == "string"))' representative_types
assert_json '.spec.password == "" and .spec.recoveryHint == ""' read_sensitive_zeroed

(
	cd "${PROJECT_DIR}"
go run ./cmd/verify-storage --database-url "${DATABASE_URL}" --username "${username}" --plaintext "${old_password}" --recovery-hint "${old_hint}"

request 200 PATCH "/users/${uid}" '{"observedAt":"2026-07-28T13:00:00Z"}'
assert_json '.spec.observedAt == "2026-07-28T13:00:00Z" and .spec.password == "" and .spec.recoveryHint == ""' patch_without_credentials
go run ./cmd/verify-storage --database-url "${DATABASE_URL}" --username "${username}" --plaintext "${old_password}" --recovery-hint "${old_hint}"
)

request 200 GET /users
assert_json 'type == "array" and length == 1 and (.[0].spec.password == "") and (.[0].spec.recoveryHint == "")' flat_list

duplicate_payload="$(jq -c --arg name "duplicate-${RUN_ID}" '.metadata.name=$name' <<<"${create_payload}")"
request 409 POST /users "${duplicate_payload}"
assert_json 'type == "object" and .code == 409 and .error == "storage conflict"' unique_create_conflict

second_username="bob-${RUN_ID}"
second_name="bob-${RUN_ID}"
second_email="bob-${RUN_ID}@example.com"
second_payload="$(jq -c \
	--arg name "${second_name}" \
	--arg username "${second_username}" \
	--arg email "${second_email}" \
	'.metadata.name=$name | .spec.username=$username | .spec.email=$email' <<<"${create_payload}")"
request 201 POST /users "${second_payload}"
second_uid="$(jq -r '.metadata.uid' <<<"${BODY}")"
assert_json 'type == "object" and (.metadata.uid | type == "string" and length > 0)' second_create_shape

update_payload="$(jq -nc \
	--arg uid "${uid}" \
	--arg name "${name}" \
	--arg username "changed-${RUN_ID}" \
	--arg email "updated-${RUN_ID}@example.com" \
	--arg password "${new_password}" \
	--arg hint "${new_hint}" \
	'{apiVersion:"example.fabrica.dev/v1",kind:"User",metadata:{name:$name,uid:$uid,namespace:"changed",labels:{team:"changed"},annotations:{purpose:"changed"}},spec:{username:$username,email:$email,password:$password,recoveryHint:$hint,role:"admin",active:false,retries:9,quota:10,score:2.5,observedAt:"2026-07-29T12:00:00Z",aliases:["updated"]}}')"
request 200 PUT "/users/${uid}" "${update_payload}"
assert_json '.spec.password == "" and .spec.recoveryHint == ""' update_sensitive_zeroed

request 200 GET "/users/${uid}"
if ! jq -e --arg username "${username}" '(.spec.username == $username) and (.spec.email | startswith("updated-"))' >/dev/null <<<"${BODY}"; then
	printf 'immutable or mutable update contract failed: %s\n' "${BODY}" >&2
	exit 1
fi
assert_json '.spec.role == "admin" and .spec.active == false and .spec.retries == 9 and .spec.quota == 10 and .spec.score == 2.5' mutable_update
assert_json '.metadata.namespace == "example" and .metadata.labels.team == "platform" and .metadata.annotations.purpose == "annotation-demo"' server_metadata_preserved
printf 'contract receipt: immutable_username_unchanged=true mutable_fields_updated=true\n'

conflicting_update_payload="$(jq -c \
	--arg uid "${second_uid}" \
	--arg name "${second_name}" \
	--arg username "${second_username}" \
	--arg email "updated-${RUN_ID}@example.com" \
	'.metadata.uid=$uid | .metadata.name=$name | .spec.username=$username | .spec.email=$email' <<<"${update_payload}")"
request 409 PUT "/users/${second_uid}" "${conflicting_update_payload}"
assert_json 'type == "object" and .code == 409 and .error == "storage conflict"' unique_update_conflict
request 200 GET "/users/${second_uid}"
if ! jq -e --arg email "${second_email}" '.spec.email == $email' >/dev/null <<<"${BODY}"; then
	printf 'conflicting update changed persisted data: %s\n' "${BODY}" >&2
	exit 1
fi
printf 'contract receipt: conflicting_update_rolled_back=true\n'

(
	cd "${PROJECT_DIR}"
	go run ./cmd/verify-storage --database-url "${DATABASE_URL}" --username "${username}" --plaintext "${new_password}" --recovery-hint "${new_hint}"
)

request 200 PUT "/users/${uid}/status" '{"state":"active","loginCount":4}'
assert_json '.status.state == "active" and .status.loginCount == 4 and .spec.password == ""' status_update
go run ./cmd/verify-storage --database-url "${DATABASE_URL}" --username "${username}" --plaintext "${new_password}" --recovery-hint "${new_hint}"

request 400 POST /users '{"kind":'
assert_json 'type == "object" and .code == 400' malformed_request

request 200 DELETE "/users/${second_uid}"
assert_json 'type == "object"' second_delete
request 200 DELETE "/users/${uid}"
if ! jq -e --arg uid "${uid}" 'type == "object" and .uid == $uid' >/dev/null <<<"${BODY}"; then
	printf 'delete response contract failed: %s\n' "${BODY}" >&2
	exit 1
fi
printf 'contract receipt: delete=true\n'
request 404 GET "/users/${uid}"
assert_json 'type == "object" and .code == 404' deleted_not_found

printf 'Example 12 API verification passed: CRUD/defaults/unique/immutable/bcrypt/redaction/list/status/error contracts=true\n'
