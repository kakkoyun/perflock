#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

TEST_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly TEST_ROOT

# shellcheck disable=SC1091 # Resolved from this test file's repository root.
source "${TEST_ROOT}/install.bash"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  [[ "${got}" == "${want}" ]] || fail "${label}: got '${got}', want '${want}'"
}

test_help() {
  local output
  output="$(main --help)"
  [[ "${output}" == *"Installs the perflock binary and system daemon"* ]] || fail "help text missing purpose"
  [[ "${output}" == *"init/windows/install.ps1"* ]] || fail "help text missing Windows installer"
}

test_platform_dispatch() {
  local called=""
  install_linux() { called="linux"; }
  install_macos() { called="macos"; }
  die() { called="error:$*"; }

  install_for_platform Linux
  assert_eq "${called}" "linux" "Linux dispatch"
  install_for_platform Darwin
  assert_eq "${called}" "macos" "Darwin dispatch"
  install_for_platform MINGW64_NT
  assert_eq "${called}" "error:Use init/windows/install.ps1 from an elevated PowerShell." "Windows dispatch"
  install_for_platform Plan9
  assert_eq "${called}" "error:Unsupported operating system: Plan9" "unsupported dispatch"
}

test_help
test_platform_dispatch
printf 'PASS: install.bash behavior\n'
