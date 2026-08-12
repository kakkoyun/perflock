#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

[[ "${TRACE:-}" == "1" ]] && set -o xtrace

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
SCRIPT_NAME="$(basename "${BASH_SOURCE[0]}")"
readonly SCRIPT_NAME
readonly BIN="${SCRIPT_DIR}/perflock"
readonly LAUNCHD_LABEL="com.github.aclements.perflock"
readonly LAUNCHD_PLIST="/Library/LaunchDaemons/${LAUNCHD_LABEL}.plist"

info() {
  printf '[INFO] %s\n' "$*" >&2
}

warn() {
  printf '[WARN] %s\n' "$*" >&2
}

die() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME}

Installs the perflock binary and system daemon on Linux or macOS.
Run this script as root after building ./cmd/perflock.

Windows users should run init/windows/install.ps1 from an elevated PowerShell.
EOF
}

check_root() {
  [[ "${EUID}" -eq 0 ]] || die "Run this installer with sudo."
}

check_binary() {
  [[ -x "${BIN}" ]] || die "${BIN} does not exist. Run 'go build ./cmd/perflock' first."
}

install_linux() {
  local install_path="/usr/bin/perflock"

  info "Installing perflock to ${install_path}"
  install -m 0755 "${BIN}" "${install_path}"

  if [[ -d /run/systemd/system ]] && command -v systemctl >/dev/null 2>&1; then
    info "Installing systemd service"
    install -m 0644 "${SCRIPT_DIR}/init/systemd/perflock.service" /etc/systemd/system/perflock.service
    systemctl daemon-reload
    systemctl enable --now perflock.service
    return
  fi

  if [[ -d /etc/init ]] && command -v service >/dev/null 2>&1; then
    info "Installing Upstart service"
    install -m 0644 "${SCRIPT_DIR}/init/upstart/perflock.conf" /etc/init/perflock.conf
    if "${install_path}" -list >/dev/null 2>&1; then
      info "The perflock daemon is already running"
    else
      service perflock start
    fi
    return
  fi

  warn "No supported init system found; starting the daemon without boot registration"
  if ! "${install_path}" -list >/dev/null 2>&1; then
    "${install_path}" -daemon >>/var/log/perflock.log 2>&1 &
  fi
}

install_macos() {
  local install_path="/usr/local/bin/perflock"

  info "Installing perflock to ${install_path}"
  install -d -m 0755 /usr/local/bin
  install -m 0755 "${BIN}" "${install_path}"

  info "Installing launchd service"
  install -m 0644 "${SCRIPT_DIR}/init/launchd/${LAUNCHD_LABEL}.plist" "${LAUNCHD_PLIST}"
  if launchctl print "system/${LAUNCHD_LABEL}" >/dev/null 2>&1; then
    launchctl bootout "system/${LAUNCHD_LABEL}"
  fi
  launchctl bootstrap system "${LAUNCHD_PLIST}"
}

install_for_platform() {
  case "$1" in
  Linux)
    install_linux
    ;;
  Darwin)
    install_macos
    ;;
  MINGW* | MSYS* | CYGWIN*)
    die "Use init/windows/install.ps1 from an elevated PowerShell."
    ;;
  *)
    die "Unsupported operating system: $1"
    ;;
  esac
}

main() {
  if [[ $# -gt 0 ]]; then
    case "$1" in
    -h | --help)
      usage
      return
      ;;
    --)
      shift
      ;;
    *)
      usage >&2
      die "Unknown argument: $1"
      ;;
    esac
  fi
  [[ $# -eq 0 ]] || die "This installer does not accept positional arguments."

  check_root
  check_binary

  install_for_platform "$(uname -s)"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
