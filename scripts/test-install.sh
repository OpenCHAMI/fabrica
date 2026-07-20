#!/bin/sh
# SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
#
# SPDX-License-Identifier: MIT

set -u

script_dir=$(CDPATH='' cd "$(dirname "$0")" && pwd)
installer="$script_dir/install.sh"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/fabrica-install-tests.XXXXXX") || exit 1
passed=0
failed=0

cleanup() {
    rm -rf "$test_root"
}
trap cleanup EXIT HUP INT TERM

pass() {
    passed=$((passed + 1))
    printf 'ok - %s\n' "$1"
}

fail() {
    failed=$((failed + 1))
    printf 'not ok - %s: %s\n' "$1" "$2" >&2
}

make_case() {
    case_name=$1
    case_dir="$test_root/$case_name"
    mock_bin="$case_dir/mock-bin"
    fixture_dir="$case_dir/fixture"
    install_dir="$case_dir/install"
    case_tmp="$case_dir/tmp"
    mkdir -p "$mock_bin" "$fixture_dir/archive" "$install_dir" "$case_tmp"

    cat >"$fixture_dir/archive/fabrica" <<'EOF'
#!/bin/sh
printf 'mock fabrica\n'
EOF
    chmod 0755 "$fixture_dir/archive/fabrica"
    tar -czf "$fixture_dir/fabrica.tar.gz" -C "$fixture_dir/archive" fabrica

    if command -v sha256sum >/dev/null 2>&1; then
        fixture_checksum=$(sha256sum "$fixture_dir/fabrica.tar.gz" | awk '{print $1}')
    else
        fixture_checksum=$(shasum -a 256 "$fixture_dir/fabrica.tar.gz" | awk '{print $1}')
    fi

    cat >"$mock_bin/uname" <<'EOF'
#!/bin/sh
case "${1:-}" in
    -s) printf '%s\n' "$MOCK_OS" ;;
    -m) printf '%s\n' "$MOCK_ARCH" ;;
    *) exit 1 ;;
esac
EOF
    chmod 0755 "$mock_bin/uname"

    cat >"$mock_bin/curl" <<'EOF'
#!/bin/sh
output=
url=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output)
            shift
            output=$1
            ;;
        http://*|https://*) url=$1 ;;
    esac
    shift
done
[ -n "$output" ] && [ -n "$url" ] || exit 2
printf '%s\n' "$url" >>"$MOCK_LOG"
case "$url" in
    */releases/latest)
        printf '{"tag_name":"v9.8.7","prerelease":false}\n' >"$output"
        ;;
    */checksums.txt)
        archive_name=$(cat "$MOCK_LAST_ARCHIVE")
        if [ "${MOCK_BAD_CHECKSUM:-0}" = 1 ]; then
            checksum=0000000000000000000000000000000000000000000000000000000000000000
        else
            checksum=$MOCK_CHECKSUM
        fi
        printf '%s  %s\n' "$checksum" "$archive_name" >"$output"
        ;;
    */fabrica_*.tar.gz)
        archive_name=${url##*/}
        printf '%s\n' "$archive_name" >"$MOCK_LAST_ARCHIVE"
        cp "$MOCK_ARCHIVE" "$output"
        ;;
    *) exit 22 ;;
esac
EOF
    chmod 0755 "$mock_bin/curl"
}

run_installer() {
    MOCK_LOG="$case_dir/downloads.log" \
    MOCK_LAST_ARCHIVE="$case_dir/archive-name" \
    MOCK_ARCHIVE="$fixture_dir/fabrica.tar.gz" \
    MOCK_CHECKSUM="$fixture_checksum" \
    MOCK_OS=$1 \
    MOCK_ARCH=$2 \
    FABRICA_INSTALL_DIR=${FABRICA_INSTALL_DIR_OVERRIDE:-$install_dir} \
    TMPDIR="$case_tmp" \
    PATH="$mock_bin:$PATH" \
    sh "$installer" >"$case_dir/stdout" 2>"$case_dir/stderr"
}

run_installer_with_predictable_symlink() {
    MOCK_LOG="$case_dir/downloads.log" \
    MOCK_LAST_ARCHIVE="$case_dir/archive-name" \
    MOCK_ARCHIVE="$fixture_dir/fabrica.tar.gz" \
    MOCK_CHECKSUM="$fixture_checksum" \
    MOCK_OS=Linux \
    MOCK_ARCH=x86_64 \
    FABRICA_INSTALL_DIR="$install_dir" \
    TMPDIR="$case_tmp" \
    PATH="$mock_bin:$PATH" \
    INSTALLER="$installer" \
    SENTINEL="$case_dir/sentinel" \
    PREDICTABLE_PATH="$case_dir/predictable-path" \
    sh -c '
        predictable="$FABRICA_INSTALL_DIR/.fabrica.install.$$"
        printf "%s\n" "$predictable" >"$PREDICTABLE_PATH"
        ln -s "$SENTINEL" "$predictable"
        exec sh "$INSTALLER"
    ' >"$case_dir/stdout" 2>"$case_dir/stderr"
}

make_case linux-x86-64
if run_installer Linux amd64 && [ -x "$install_dir/fabrica" ] &&
    grep -F '/fabrica_9.8.7_linux_x86_64.tar.gz' "$case_dir/downloads.log" >/dev/null; then
    pass "Linux amd64 maps to linux x86_64"
else
    fail "Linux amd64 maps to linux x86_64" "installer failed or selected the wrong archive"
fi

make_case darwin-arm64
if run_installer Darwin aarch64 && [ -x "$install_dir/fabrica" ] &&
    grep -F '/fabrica_9.8.7_darwin_arm64.tar.gz' "$case_dir/downloads.log" >/dev/null; then
    pass "Darwin aarch64 maps to darwin arm64"
else
    fail "Darwin aarch64 maps to darwin arm64" "installer failed or selected the wrong archive"
fi

make_case explicit-version
if FABRICA_VERSION=v1.2.3 run_installer Linux x86_64 &&
    grep -F '/releases/download/v1.2.3/fabrica_1.2.3_linux_x86_64.tar.gz' "$case_dir/downloads.log" >/dev/null &&
    ! grep -F '/releases/latest' "$case_dir/downloads.log" >/dev/null; then
    pass "FABRICA_VERSION accepts a leading v and skips discovery"
else
    fail "FABRICA_VERSION accepts a leading v and skips discovery" "explicit version was not used"
fi
unset FABRICA_VERSION

make_case checksum-mismatch
if MOCK_BAD_CHECKSUM=1 run_installer Linux x86_64; then
    fail "checksum mismatch is rejected" "installer unexpectedly succeeded"
elif [ ! -e "$install_dir/fabrica" ] && grep -F 'checksum verification failed' "$case_dir/stderr" >/dev/null; then
    pass "checksum mismatch is rejected"
else
    fail "checksum mismatch is rejected" "expected checksum failure was not reported"
fi
unset MOCK_BAD_CHECKSUM

make_case unsupported-platform
if run_installer FreeBSD x86_64; then
    fail "unsupported platforms are rejected" "installer unexpectedly succeeded"
elif grep -F 'unsupported operating system' "$case_dir/stderr" >/dev/null; then
    pass "unsupported platforms are rejected"
else
    fail "unsupported platforms are rejected" "expected platform error was not reported"
fi

make_case unwritable-destination
blocked_path="$case_dir/blocked"
printf 'not a directory\n' >"$blocked_path"
FABRICA_INSTALL_DIR_OVERRIDE="$blocked_path/bin"
if run_installer Linux x86_64; then
    fail "unwritable destinations are rejected" "installer unexpectedly succeeded"
elif grep -F 'choose a writable directory with FABRICA_INSTALL_DIR' "$case_dir/stderr" >/dev/null; then
    pass "unwritable destinations are rejected"
else
    fail "unwritable destinations are rejected" "expected actionable destination error was not reported"
fi
unset FABRICA_INSTALL_DIR_OVERRIDE

make_case final-destination-directory
mkdir "$install_dir/fabrica"
printf 'directory marker\n' >"$install_dir/fabrica/marker"
if run_installer Linux x86_64; then
    fail "final destination directories are rejected" "installer unexpectedly succeeded"
elif grep -F 'is a directory or resolves to a directory' "$case_dir/stderr" >/dev/null &&
    [ ! -e "$install_dir/fabrica/fabrica" ] &&
    [ "$(cat "$install_dir/fabrica/marker")" = 'directory marker' ] &&
    [ -z "$(find "$install_dir" -name '.fabrica.install.*' -print)" ]; then
    pass "final destination directories are rejected without mutation"
else
    fail "final destination directories are rejected" "destination changed or staged content remained"
fi

make_case final-destination-symlink-directory
symlink_target="$case_dir/symlink-target"
mkdir "$symlink_target"
printf 'symlink target marker\n' >"$symlink_target/marker"
ln -s "$symlink_target" "$install_dir/fabrica"
if run_installer Linux x86_64; then
    fail "final destination symlinks to directories are rejected" "installer unexpectedly succeeded"
else
    final_listing=$(LC_ALL=C ls -ld "$install_dir/fabrica")
    case "$final_listing" in
        l*) final_is_symlink=true ;;
        *) final_is_symlink=false ;;
    esac
    if grep -F 'is a directory or resolves to a directory' "$case_dir/stderr" >/dev/null &&
        [ "$final_is_symlink" = true ] &&
        [ ! -e "$symlink_target/fabrica" ] &&
        [ "$(cat "$symlink_target/marker")" = 'symlink target marker' ] &&
        [ -z "$(find "$install_dir" -name '.fabrica.install.*' -print)" ]; then
        pass "final destination symlinks to directories are rejected without mutation"
    else
        fail "final destination symlinks to directories are rejected" "symlink target changed or staged content remained"
    fi
fi

make_case predictable-staging-collision
printf 'do not overwrite\n' >"$case_dir/sentinel"
if run_installer_with_predictable_symlink; then
    predictable_path=$(cat "$case_dir/predictable-path")
    sentinel_contents=$(cat "$case_dir/sentinel")
    predictable_listing=$(LC_ALL=C ls -ld "$predictable_path")
    case "$predictable_listing" in
        l*) predictable_is_symlink=true ;;
        *) predictable_is_symlink=false ;;
    esac
    if [ "$sentinel_contents" = 'do not overwrite' ] &&
        [ "$predictable_is_symlink" = true ] &&
        [ -x "$install_dir/fabrica" ]; then
        pass "predictable staging symlinks are not followed or overwritten"
    else
        fail "predictable staging symlinks are not followed or overwritten" "the pre-existing path or its target changed"
    fi
else
    fail "predictable staging symlinks are not followed or overwritten" "installer failed with an unrelated error"
fi

make_case temporary-cleanup
if MOCK_BAD_CHECKSUM=1 run_installer Linux x86_64; then
    fail "temporary files are cleaned after failure" "installer unexpectedly succeeded"
elif [ -z "$(find "$case_tmp" -name 'fabrica-install.*' -print)" ]; then
    pass "temporary files are cleaned after failure"
else
    fail "temporary files are cleaned after failure" "temporary installer directory remains"
fi

printf '%s passed; %s failed\n' "$passed" "$failed"
[ "$failed" -eq 0 ]
