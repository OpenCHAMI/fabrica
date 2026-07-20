#!/bin/sh
# SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
#
# SPDX-License-Identifier: MIT

set -eu

repository="openchami/fabrica"
github_url="https://github.com/$repository"
api_url="https://api.github.com/repos/$repository"
install_dir=${FABRICA_INSTALL_DIR:-${XDG_BIN_HOME:-$HOME/.local/bin}}
tmp_dir=
staged_file=

fail() {
    printf 'fabrica installer: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    if [ -n "$staged_file" ]; then
        rm -f "$staged_file"
    fi
    if [ -n "$tmp_dir" ]; then
        rm -rf "$tmp_dir"
    fi
}

trap cleanup EXIT HUP INT TERM

for command_name in uname mktemp tar awk grep cp chmod mv rm mkdir ls; do
    command -v "$command_name" >/dev/null 2>&1 ||
        fail "required command '$command_name' was not found"
done

if command -v curl >/dev/null 2>&1; then
    download() {
        url=$1
        output=$2
        curl -fsSL --retry 3 --output "$output" "$url" ||
            fail "could not download $url; check your network connection and GitHub availability"
    }
elif command -v wget >/dev/null 2>&1; then
    download() {
        url=$1
        output=$2
        wget -q -O "$output" "$url" ||
            fail "could not download $url; check your network connection and GitHub availability"
    }
else
    fail "curl or wget is required to download Fabrica"
fi

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/fabrica-install.XXXXXX") ||
    fail "could not create a temporary directory"

if [ -n "${FABRICA_VERSION:-}" ]; then
    version=$FABRICA_VERSION
else
    release_json="$tmp_dir/latest-release.json"
    download "$api_url/releases/latest" "$release_json"
    version=$(awk '
        match($0, /"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"/) {
            value = substr($0, RSTART, RLENGTH)
            sub(/^.*"tag_name"[[:space:]]*:[[:space:]]*"/, "", value)
            sub(/"$/, "", value)
            print value
            exit
        }
    ' "$release_json")
    [ -n "$version" ] ||
        fail "GitHub's latest stable release response did not contain a version"
fi

case "$version" in
    v*) version=${version#v} ;;
esac

printf '%s\n' "$version" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$' >/dev/null 2>&1 ||
    fail "invalid FABRICA_VERSION '$version'; expected semantic version such as 1.2.3 or v1.2.3"

uname_os=$(uname -s) || fail "could not determine the operating system"
case "$uname_os" in
    Linux) target_os=linux ;;
    Darwin) target_os=darwin ;;
    *) fail "unsupported operating system '$uname_os'; supported systems are Linux and Darwin" ;;
esac

uname_arch=$(uname -m) || fail "could not determine the machine architecture"
if [ "$target_os" = darwin ] && [ "$uname_arch" = x86_64 ] && command -v sysctl >/dev/null 2>&1; then
    if [ "$(sysctl -in sysctl.proc_translated 2>/dev/null || printf '0')" = 1 ]; then
        uname_arch=arm64
    fi
fi

case "$uname_arch" in
    x86_64|amd64) target_arch=x86_64 ;;
    arm64|aarch64) target_arch=arm64 ;;
    *) fail "unsupported architecture '$uname_arch'; supported architectures are x86_64 and arm64" ;;
esac

archive="fabrica_${version}_${target_os}_${target_arch}.tar.gz"
release_url="$github_url/releases/download/v${version}"
archive_path="$tmp_dir/$archive"
checksums_path="$tmp_dir/checksums.txt"

download "$release_url/$archive" "$archive_path"
download "$release_url/checksums.txt" "$checksums_path"

expected_checksum=$(awk -v target="$archive" '
    $2 == target {
        checksum = $1
        matches++
    }
    END {
        if (matches != 1) exit 1
        print checksum
    }
' "$checksums_path") ||
    fail "checksums.txt must contain exactly one entry for $archive"

case "$expected_checksum" in
    *[!0-9A-Fa-f]*|'') fail "invalid SHA-256 checksum entry for $archive" ;;
esac
[ "${#expected_checksum}" -eq 64 ] || fail "invalid SHA-256 checksum entry for $archive"

if command -v sha256sum >/dev/null 2>&1; then
    actual_checksum=$(sha256sum "$archive_path" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    actual_checksum=$(shasum -a 256 "$archive_path" | awk '{print $1}')
else
    fail "sha256sum or shasum is required to verify the downloaded archive"
fi

[ "$actual_checksum" = "$expected_checksum" ] ||
    fail "checksum verification failed for $archive; the archive was not installed"

extract_dir="$tmp_dir/extract"
mkdir "$extract_dir" || fail "could not prepare the temporary extraction directory"
tar -xzf "$archive_path" -C "$extract_dir" fabrica ||
    fail "could not extract the fabrica binary from $archive"

binary="$extract_dir/fabrica"
[ -f "$binary" ] || fail "the archive did not contain a regular fabrica file"
case "$(ls -ld "$binary")" in
    -*) ;;
    *) fail "the fabrica archive entry is not a regular file" ;;
esac
chmod 0755 "$binary" || fail "could not mark the fabrica binary executable"
[ -x "$binary" ] || fail "the extracted fabrica file is not executable"

if [ ! -d "$install_dir" ]; then
    mkdir -p "$install_dir" 2>/dev/null ||
        fail "cannot create install directory '$install_dir'; choose a writable directory with FABRICA_INSTALL_DIR"
fi
[ -d "$install_dir" ] || fail "install destination '$install_dir' is not a directory"
[ -w "$install_dir" ] ||
    fail "install directory '$install_dir' is not writable; choose another with FABRICA_INSTALL_DIR (this installer never uses sudo)"

final_path="$install_dir/fabrica"
[ ! -d "$final_path" ] ||
    fail "install destination '$final_path' is a directory or resolves to a directory; remove or rename it before installing"

staged_file=$(mktemp "$install_dir/.fabrica.install.XXXXXX") ||
    fail "could not create an exclusive staging file in '$install_dir'"
cp "$binary" "$staged_file" ||
    fail "could not stage Fabrica in '$install_dir'; check available space and permissions"
chmod 0755 "$staged_file" || fail "could not set permissions on the staged Fabrica binary"
mv -f "$staged_file" "$final_path" ||
    fail "could not replace '$final_path' atomically"
staged_file=

printf 'Installed Fabrica %s to %s\n' "$version" "$final_path"
case ":${PATH:-}:" in
    *:"$install_dir":*) ;;
    *) printf 'Add %s to PATH to run fabrica from your shell.\n' "$install_dir" ;;
esac
