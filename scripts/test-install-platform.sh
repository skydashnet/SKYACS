#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_DIR="$(mktemp -d -t skyacs-platform-test.XXXXXXXX)"
trap 'rm -rf "$TEMP_DIR"' EXIT

write_release() {
  local id="$1" version="$2"
  cat >"$TEMP_DIR/os-release" <<EOF
ID=$id
VERSION_ID="$version"
PRETTY_NAME="$id $version"
EOF
}

expect_supported() {
  write_release "$1" "$2"
  SKYACS_INSTALLER_TEST=1 SKYACS_OS_RELEASE_FILE="$TEMP_DIR/os-release" bash "$ROOT_DIR/install.sh" >/dev/null
}

expect_unsupported() {
  write_release "$1" "$2"
  if SKYACS_INSTALLER_TEST=1 SKYACS_OS_RELEASE_FILE="$TEMP_DIR/os-release" bash "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
    printf 'expected %s %s to be rejected\n' "$1" "$2" >&2
    exit 1
  fi
}

expect_supported ubuntu 22.04
expect_supported ubuntu 24.04
expect_supported ubuntu 26.04
expect_supported debian 12
expect_supported debian 13

expect_unsupported ubuntu 20.04
expect_unsupported debian 11
expect_unsupported linuxmint 22

write_release debian 13
if SKYACS_INSTALLER_TEST=1 SKYACS_OS_RELEASE_FILE="$TEMP_DIR/os-release" SKYACS_REF='../main' bash "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
  printf 'expected a path-traversal source ref to be rejected\n' >&2
  exit 1
fi
if SKYACS_INSTALLER_TEST=1 SKYACS_OS_RELEASE_FILE="$TEMP_DIR/os-release" SKYACS_INSTALL_DIR=relative/path bash "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
  printf 'expected a relative install directory to be rejected\n' >&2
  exit 1
fi
if SKYACS_INSTALLER_TEST=1 SKYACS_OS_RELEASE_FILE="$TEMP_DIR/os-release" SKYACS_SERVICE_USER='root user' bash "$ROOT_DIR/install.sh" >/dev/null 2>&1; then
  printf 'expected an invalid service username to be rejected\n' >&2
  exit 1
fi

printf 'installer platform matrix passed\n'
