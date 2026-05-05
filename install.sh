#!/bin/sh
# Install discord-reminder on a Linux host (systemd).
#
# Modes:
#   LOCAL  - if this script lives next to a prebuilt binary and packaging dir
#            (typical when extracted from a release tarball), files are installed
#            from $SCRIPT_DIR.
#   REMOTE - otherwise the latest release tarball is downloaded from
#            github.com/$REPO, sha256-verified, then installed.
#
# Usage:
#   sudo ./install.sh                       # LOCAL or REMOTE (auto-detected)
#   curl -fsSL https://raw.githubusercontent.com/m1sk9/discord-reminder/main/install.sh | sudo sh
#
# Env overrides:
#   REPO=m1sk9/discord-reminder
#   VERSION=latest          # or a specific tag like v1.2.3
#   PREFIX=/usr/local/bin
#   CONFDIR=/etc/discord-reminder
#   USER_NAME=discord-reminder

set -eu

REPO="${REPO:-m1sk9/discord-reminder}"
VERSION="${VERSION:-latest}"
PREFIX="${PREFIX:-/usr/local/bin}"
CONFDIR="${CONFDIR:-/etc/discord-reminder}"
USER_NAME="${USER_NAME:-discord-reminder}"

UNIT_NAME="discord-reminder.service"
UNIT_DST="/etc/systemd/system/${UNIT_NAME}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

log()  { printf '[install] %s\n' "$*"; }
fail() { printf '[install] error: %s\n' "$*" >&2; exit 1; }

# 0. preflight
[ "$(id -u)" -eq 0 ] || fail "must be run as root"
[ "$(uname -s)" = "Linux" ] || fail "Linux only (got: $(uname -s))"
command -v systemctl >/dev/null 2>&1 || fail "systemctl not found"
command -v tar       >/dev/null 2>&1 || fail "tar not found"
command -v useradd   >/dev/null 2>&1 || fail "useradd not found"

# 1. resolve source dir (LOCAL or REMOTE)
SRC_DIR=""
TMP_ROOT=""
cleanup() { [ -n "$TMP_ROOT" ] && rm -rf "$TMP_ROOT"; }
trap cleanup EXIT INT TERM

if [ -f "$SCRIPT_DIR/discord-reminder" ] && [ -f "$SCRIPT_DIR/packaging/$UNIT_NAME" ]; then
  log "using local artifacts in $SCRIPT_DIR"
  SRC_DIR="$SCRIPT_DIR"
else
  command -v curl     >/dev/null 2>&1 || fail "curl not found"
  command -v sha256sum >/dev/null 2>&1 || fail "sha256sum not found"

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)  goarch=amd64 ;;
    aarch64|arm64) goarch=arm64 ;;
    *) fail "unsupported architecture: $arch" ;;
  esac

  # resolve "latest" by following the redirect of /releases/latest
  if [ "$VERSION" = "latest" ]; then
    log "resolving latest release tag from $REPO"
    redirected="$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
      "https://github.com/${REPO}/releases/latest")" \
      || fail "could not query GitHub for latest release"
    VERSION="${redirected##*/}"
    case "$VERSION" in
      v*) ;;
      *)  fail "unexpected redirect target: $redirected" ;;
    esac
  fi
  log "version: $VERSION (linux/$goarch)"

  asset="discord-reminder-${VERSION}-linux-${goarch}.tar.gz"
  base_url="https://github.com/${REPO}/releases/download/${VERSION}"

  TMP_ROOT="$(mktemp -d)"

  log "downloading $asset"
  curl -fsSL --proto '=https' --tlsv1.2 \
    -o "${TMP_ROOT}/${asset}" "${base_url}/${asset}" \
    || fail "download failed: ${base_url}/${asset}"
  curl -fsSL --proto '=https' --tlsv1.2 \
    -o "${TMP_ROOT}/${asset}.sha256" "${base_url}/${asset}.sha256" \
    || fail "download failed: ${base_url}/${asset}.sha256"

  log "verifying sha256"
  (cd "$TMP_ROOT" && sha256sum -c "${asset}.sha256" >/dev/null) \
    || fail "sha256 verification failed"

  log "extracting"
  tar -C "$TMP_ROOT" -xzf "${TMP_ROOT}/${asset}"

  staged="${TMP_ROOT}/discord-reminder-${VERSION}-linux-${goarch}"
  [ -d "$staged" ] || fail "extracted directory not found: $staged"
  SRC_DIR="$staged"
fi

BIN_SRC="${SRC_DIR}/discord-reminder"
UNIT_SRC="${SRC_DIR}/packaging/${UNIT_NAME}"
CONFIG_EXAMPLE="${SRC_DIR}/config.toml.example"
SECRETS_EXAMPLE="${SRC_DIR}/secrets.toml.example"

[ -f "$BIN_SRC" ]         || fail "missing: $BIN_SRC"
[ -f "$UNIT_SRC" ]        || fail "missing: $UNIT_SRC"
[ -f "$CONFIG_EXAMPLE" ]  || fail "missing: $CONFIG_EXAMPLE"
[ -f "$SECRETS_EXAMPLE" ] || fail "missing: $SECRETS_EXAMPLE"

# 2. system user
if id "$USER_NAME" >/dev/null 2>&1; then
  log "user $USER_NAME already exists"
else
  log "creating system user $USER_NAME"
  useradd --system --no-create-home --shell /usr/sbin/nologin "$USER_NAME"
fi

# 3. install binary
log "installing binary to $PREFIX/discord-reminder"
install -m 0755 -o root -g root "$BIN_SRC" "$PREFIX/discord-reminder"

# 4. config skeleton (do not overwrite existing)
mkdir -p "$CONFDIR"
chown root:root "$CONFDIR"
chmod 0755 "$CONFDIR"

if [ -f "$CONFDIR/config.toml" ]; then
  log "$CONFDIR/config.toml exists, leaving as is"
else
  log "installing config.toml from example"
  install -m 0644 -o root -g root "$CONFIG_EXAMPLE" "$CONFDIR/config.toml"
fi

if [ -f "$CONFDIR/secrets.toml" ]; then
  log "$CONFDIR/secrets.toml exists, leaving as is"
else
  log "installing secrets.toml from example (mode 0640, group=$USER_NAME)"
  install -m 0640 -o root -g "$USER_NAME" "$SECRETS_EXAMPLE" "$CONFDIR/secrets.toml"
fi

# 5. systemd unit
log "installing systemd unit to $UNIT_DST"
install -m 0644 -o root -g root "$UNIT_SRC" "$UNIT_DST"

systemctl daemon-reload
systemctl enable "$UNIT_NAME" >/dev/null

# 6. summary
cat <<EOF

[install] done (version: ${VERSION}).

Next steps:
  1. Edit configuration (replace placeholders):
       \$EDITOR $CONFDIR/config.toml
       \$EDITOR $CONFDIR/secrets.toml
  2. Start the service:
       systemctl start $UNIT_NAME
  3. Tail logs:
       journalctl -u $UNIT_NAME -f
EOF
