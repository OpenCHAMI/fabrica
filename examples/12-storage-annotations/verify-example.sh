#!/usr/bin/env bash
# Copyright © 2026 OpenCHAMI Contributors
# SPDX-License-Identifier: MIT

set -euo pipefail

example_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${example_dir}/../.." && pwd)"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/fabrica-example-12.XXXXXX")"
project_dir="${work_dir}/project"
fabrica_bin="${work_dir}/fabrica"

cleanup() {
	rm -rf "${work_dir}"
}
trap cleanup EXIT INT TERM

for command in go gopls jq; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		printf 'required command not found: %s\n' "${command}" >&2
		exit 1
	fi
done

for source_file in go.mod go.sum .fabrica.yaml apis.yaml apis/example.fabrica.dev/v1/user_types.go cmd/server/main.go materialize-generated-fixtures.sh test-api.sh testdata/server-main.go.txt testdata/server-storage.go.txt testdata/verify-storage-main.go.txt; do
	if [[ ! -f "${example_dir}/${source_file}" ]]; then
		printf 'incomplete Example 12: missing %s\n' "${source_file}" >&2
		exit 1
	fi
done

(
	cd "${example_dir}"
	go mod tidy -diff
	go_files=()
	while IFS= read -r file; do
		go_files+=("${file}")
	done < <(go list -f '{{range .GoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}{{range .TestGoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}' ./...)
	gopls_output="$(gopls check -- "${go_files[@]}" 2>&1)"
	if [[ -n "${gopls_output}" ]]; then
		printf '%s\n' "${gopls_output}" >&2
		exit 1
	fi
)
printf 'Example 12 source module preflight passed: tidy_diff=clean gopls=clean\n'

mkdir -p "${project_dir}"
cp -R "${example_dir}/." "${project_dir}/"
(
	cd "${project_dir}"
	go mod edit -replace "github.com/openchami/fabrica=${repo_root}"
)

(
	cd "${repo_root}"
	go build -o "${fabrica_bin}" ./cmd/fabrica
)

cli_help="$("${fabrica_bin}" --help)"
for operation in inspect validate; do
	if grep -qE "^[[:space:]]+${operation}[[:space:]]" <<<"${cli_help}"; then
		"${fabrica_bin}" "${operation}" --project-path "${project_dir}" --json | jq -e '.status == "ok"'
	else
		printf 'CLI capability unavailable: fabrica %s (N/A)\n' "${operation}"
	fi
done

generate_help="$("${fabrica_bin}" generate --help)"
if grep -q -- '--dry-run' <<<"${generate_help}" && grep -q -- '--json' <<<"${generate_help}"; then
	(
		cd "${project_dir}"
		"${fabrica_bin}" generate --force --fabrica-source "${repo_root}" --dry-run --json | jq -e '.status == "dry_run"'
	)
else
	printf 'CLI capability unavailable: fabrica generate --dry-run --json (N/A)\n'
fi

(
	cd "${project_dir}"
	"${fabrica_bin}" generate --force --debug --fabrica-source "${repo_root}"
	bash ./materialize-generated-fixtures.sh
	go mod tidy
	go mod tidy -diff
	go generate ./internal/storage/ent
	go test -count=1 ./...
	go build ./...
	go_files=()
	while IFS= read -r file; do
		go_files+=("${file}")
	done < <(go list -f '{{range .GoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}{{range .TestGoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}{{range .XTestGoFiles}}{{printf "%s/%s\n" $.Dir .}}{{end}}' ./...)
	gopls_output="$(gopls check -- "${go_files[@]}" 2>&1)"
	if [[ -n "${gopls_output}" ]]; then
		printf '%s\n' "${gopls_output}" >&2
		exit 1
	fi

	server_bin="${work_dir}/server"
	database_url="file:${work_dir}/example.db?_fk=1"
	port="$((18080 + $$ % 1000))"
	go build -o "${server_bin}" ./cmd/server/
	"${server_bin}" -host 127.0.0.1 -port "${port}" -database-url "${database_url}" >"${work_dir}/server.log" 2>&1 &
	server_pid=$!
	cleanup_server() {
		if kill -0 "${server_pid}" 2>/dev/null; then
			kill "${server_pid}"
			wait "${server_pid}" 2>/dev/null || true
		fi
	}
	trap cleanup_server EXIT INT TERM
	for _ in {1..100}; do
		if curl --fail-with-body --silent "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
			break
		fi
		kill -0 "${server_pid}"
		sleep 0.1
	done
	BASE_URL="http://127.0.0.1:${port}" PROJECT_DIR="${project_dir}" DATABASE_URL="${database_url}" bash ./test-api.sh
	cleanup_server
	trap - EXIT INT TERM
)

printf 'Example 12 generation/build regression passed in isolated temp copy\n'
