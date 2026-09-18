#!/usr/bin/env bash
# brrewery host installer — idempotent bootstrap for production paths.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT"
BINARY_DEST="/usr/local/bin/brrewery"
LIB_DIR="/var/lib/brrewery"
LOG_DIR="/var/log/brrewery"
INSTALL_LOG="/var/log/brrewery-install.log"
ANSIBLE_DEST="/usr/share/brrewery/ansible"
VENDOR_DEST="/usr/share/brrewery/vendor"
QBT_PATCHES_DIR="/var/lib/brrewery/patches/qbittorrent"
SSL_DIR="/etc/ssl/brrewery"
NGINX_ETC="/etc/nginx"
REPO_URL="${BRREWERY_REPO_URL:-https://github.com/torrentin0/brrewery.git}"
# Release tag to install (e.g. v1.2.0 or v1.2.0-rc.1). Empty resolves the
# newest published GitHub release, pre-releases included.
RELEASE_TAG="${BRREWERY_VERSION:-}"
RELEASE_DIR=""
ACME_HOME="/root/.acme.sh"

if [[ "${EUID:-}" -ne 0 ]]; then
  echo "Run as root: sudo $0" >&2
  exit 1
fi

# Domain for the Let's Encrypt certificate. Pass BRREWERY_DOMAIN=<fqdn> for
# non-interactive runs; an empty value skips issuance.
DOMAIN="${BRREWERY_DOMAIN:-}"
if [[ -z "$DOMAIN" && -t 0 ]]; then
  echo "Enter the domain to issue a Let's Encrypt certificate for. It must already"
  echo "resolve to this host (e.g. brrewery.example.com). Leave empty to skip."
  read -r -p "Domain: " DOMAIN || true
fi
DOMAIN="${DOMAIN//[[:space:]]/}"
if [[ -z "$DOMAIN" ]]; then
  echo
  echo "! No domain provided — no Let's Encrypt certificate will be generated and"
  echo "  TLS encryption will not be handled by brrewery. The dashboard falls back"
  echo "  to a self-signed placeholder certificate (browsers will warn). Re-run"
  echo "  this installer with BRREWERY_DOMAIN=<domain> to add one later."
  echo
elif [[ ! "$DOMAIN" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*$ ]]; then
  echo "Invalid domain: $DOMAIN" >&2
  exit 1
fi

: >"$INSTALL_LOG"
chmod 0640 "$INSTALL_LOG"

run_with_spinner() {
  local message="$1"
  shift

  local -a spinner=(⣷ ⣯ ⣟ ⡿ ⢿ ⣻ ⣽ ⣾)
  local i=0
  local pid
  local exit_code

  {
    printf '\n=== %s ===\n' "$message"
    "$@"
  } >>"$INSTALL_LOG" 2>&1 </dev/null &
  pid=$!

  while kill -0 "$pid" 2>/dev/null; do
    printf "\r\033[2K%s %s" "${spinner[i++ % ${#spinner[@]}]}" "$message"
    sleep 0.08
  done

  wait "$pid"
  exit_code=$?

  if [[ "$exit_code" -eq 0 ]]; then
    printf "\r\033[2K✓ %s\n" "$message"
    return 0
  fi

  printf "\r\033[2K✗ %s\n" "$message"
  echo "$message failed. Last 40 log lines ($INSTALL_LOG):" >&2
  tail -n 40 "$INSTALL_LOG" >&2 || true
  return "$exit_code"
}

run_with_log() {
  local message="$1"
  shift

  printf "\n→ %s\n" "$message"
  {
    printf "\n=== %s ===\n" "$message"
    "$@"
  } 2>&1 | tee -a "$INSTALL_LOG"
  echo
}

# Download the brrewery release archive (binary with the frontend embedded,
# ansible playbooks and contrib config) built by the Release GitHub workflow,
# verify its checksum and unpack it to $RELEASE_DIR.
fetch_release() {
  if [[ -x "$SOURCE_DIR/brrewery" ]]; then
    RELEASE_DIR="$SOURCE_DIR"
    return 0
  fi

  if [[ "$(uname -m)" != "x86_64" ]]; then
    echo "Unsupported architecture: $(uname -m) (release binaries are linux/amd64 only)" >&2
    exit 1
  fi

  local slug version archive base_url
  slug="${REPO_URL#https://github.com/}"
  slug="${slug%.git}"

  if [[ -z "$RELEASE_TAG" ]]; then
    # /releases/latest excludes pre-releases, so list releases and take the
    # newest non-draft entry instead.
    RELEASE_TAG="$(
      curl -fsSL "https://api.github.com/repos/${slug}/releases?per_page=10" | python3 -c '
import json, sys
for release in json.load(sys.stdin):
    if not release.get("draft"):
        print(release["tag_name"])
        break
'
    )"
  fi
  if [[ -z "$RELEASE_TAG" ]]; then
    echo "Failed to resolve the latest brrewery release for ${slug}" >&2
    exit 1
  fi

  version="${RELEASE_TAG#v}"
  archive="brrewery_${version}_linux_amd64.tar.gz"
  base_url="https://github.com/${slug}/releases/download/${RELEASE_TAG}"

  RELEASE_DIR="$(mktemp -d /tmp/brrewery-release.XXXXXX)"
  trap 'rm -rf "$RELEASE_DIR"' EXIT

  run_with_spinner "Downloading brrewery ${RELEASE_TAG}" bash -c "
    cd \"$RELEASE_DIR\" &&
      curl -fsSL -o \"$archive\" \"$base_url/$archive\" &&
      curl -fsSL -o checksums.txt \"$base_url/checksums.txt\" &&
      grep \" $archive\$\" checksums.txt | sha256sum -c - &&
      tar -xzf \"$archive\"
  "

  if [[ ! -x "$RELEASE_DIR/brrewery" ]]; then
    echo "Release archive $archive is missing the binary" >&2
    exit 1
  fi
}

if command -v apt >/dev/null 2>&1; then
  run_with_spinner "Installing dependencies" bash -c '
    apt update -qq &&
      DEBIAN_FRONTEND=noninteractive apt install -y -qq \
        nginx git vnstat sudo ansible openssl curl ca-certificates python3 cron
  '
else
  echo "Unsupported distro: apt is required." >&2
  exit 1
fi

fetch_release

# Prefer a local source checkout (developer install) for the ansible playbooks
# and contrib config; otherwise use the copies shipped in the release archive.
if [[ ! -d "$SOURCE_DIR/ansible" || ! -d "$SOURCE_DIR/contrib" ]]; then
  SOURCE_DIR="$RELEASE_DIR"
fi

run_with_spinner "Creating directories" bash -c "
  install -d -m 0750 \"$LIB_DIR\" \"$LIB_DIR/jobs\" \"$LOG_DIR\" \"$ANSIBLE_DEST\" \"$VENDOR_DEST\" \"$QBT_PATCHES_DIR\" \"$SSL_DIR\" &&
    install -d -m 0755 \"$(dirname "$BINARY_DEST")\"
"

if [[ ! -d "$SOURCE_DIR/ansible" || ! -d "$SOURCE_DIR/contrib" ]]; then
  echo "Missing ansible/ or contrib/ in $SOURCE_DIR" | tee -a "$INSTALL_LOG" >&2
  exit 1
fi

run_with_spinner "Installing binary and ansible playbooks" bash -c "
  install -m 0755 \"$RELEASE_DIR/brrewery\" \"$BINARY_DEST\" &&
    rm -rf \"${ANSIBLE_DEST:?}\"/* &&
    cp -a \"$SOURCE_DIR/ansible/.\" \"$ANSIBLE_DEST/\"
"

# The frontend is embedded in the binary, so there are no web assets to deploy.
# Installs made before that change left a bundle in /var/www/brrewery that the
# vhost no longer reads; drop it so a stale copy can't be mistaken for live.
if [[ -d /var/www/brrewery ]]; then
  run_with_spinner "Removing legacy web root" bash -c "rm -rf /var/www/brrewery"
fi

if [[ ! -f "$SSL_DIR/fullchain.pem" ]]; then
  run_with_spinner "Generating self-signed TLS certificate" bash -c "
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
      -keyout \"$SSL_DIR/privkey.pem\" \
      -out \"$SSL_DIR/fullchain.pem\" \
      -subj \"/CN=brrewery.local\"
  "
  run_with_spinner "Setting TLS certificate permissions" bash -c "
    chmod 0640 \"$SSL_DIR/privkey.pem\" &&
      chmod 0644 \"$SSL_DIR/fullchain.pem\"
  "
fi

run_with_spinner "Configuring nginx" bash -c "
  install -d -m 0755 \"$NGINX_ETC/sites-available\" \"$NGINX_ETC/sites-enabled\" \"$NGINX_ETC/servers-available\" \"$NGINX_ETC/servers-enabled\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/nginx.conf\" \"$NGINX_ETC/nginx.conf\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/general.conf\" \"$NGINX_ETC/general.conf\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/security.conf\" \"$NGINX_ETC/security.conf\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/proxy.conf\" \"$NGINX_ETC/proxy.conf\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/ssl.conf\" \"$NGINX_ETC/ssl.conf\" &&
    install -m 0644 \"$SOURCE_DIR/contrib/nginx/sites-available/default\" \"$NGINX_ETC/sites-available/default\" &&
    rm -rf \"$NGINX_ETC/sites-enabled/brrewery\" \"$NGINX_ETC/sites-enabled/brrewery.conf\" \"$NGINX_ETC/sites-available/brrewery.conf\" \"$NGINX_ETC/nginxconfig.io\" &&
    ln -sf ../sites-available/default \"$NGINX_ETC/sites-enabled/default\" &&
    nginx -t &&
    systemctl enable nginx &&
    (systemctl reload nginx || systemctl start nginx)
"

if [[ -n "$DOMAIN" ]]; then
  # acme.sh --nginx mode only accepts a plain-HTTP server block whose
  # server_name contains the domain, so write it into the installed vhost.
  run_with_spinner "Setting nginx server_name to $DOMAIN" bash -c "
    sed -i \"s/server_name _;/server_name $DOMAIN;/\" \"$NGINX_ETC/sites-available/default\" &&
      nginx -t &&
      systemctl reload nginx
  "

  if [[ ! -x "$ACME_HOME/acme.sh" ]]; then
    run_with_spinner "Installing acme.sh" bash -c "
      systemctl enable --now cron &&
        rm -rf /tmp/brrewery-acme.sh &&
        git clone --depth 1 https://github.com/acmesh-official/acme.sh /tmp/brrewery-acme.sh &&
        cd /tmp/brrewery-acme.sh &&
        ./acme.sh --install --home \"$ACME_HOME\" &&
        rm -rf /tmp/brrewery-acme.sh
    "
  fi

  # Exit code 2 means a valid cert already exists and renewal was skipped —
  # treat it as success so re-running the installer stays idempotent.
  if run_with_spinner "Requesting Let's Encrypt certificate for $DOMAIN" bash -c "
    \"$ACME_HOME/acme.sh\" --home \"$ACME_HOME\" --issue --server letsencrypt \
      --nginx \"$NGINX_ETC/sites-available/default\" -d \"$DOMAIN\" ||
      { rc=\$?; [[ \$rc -eq 2 ]] || exit \$rc; }
  "; then
    # --install-cert persists the file targets and reloadcmd in the acme.sh
    # domain config, so the cron job installed above re-deploys the renewed
    # cert into $SSL_DIR and reloads nginx automatically.
    run_with_spinner "Deploying Let's Encrypt certificate" bash -c "
      \"$ACME_HOME/acme.sh\" --home \"$ACME_HOME\" --install-cert -d \"$DOMAIN\" \
        --key-file \"$SSL_DIR/privkey.pem\" \
        --fullchain-file \"$SSL_DIR/fullchain.pem\" \
        --reloadcmd \"chmod 0640 $SSL_DIR/privkey.pem; chmod 0644 $SSL_DIR/fullchain.pem; systemctl reload nginx\"
    "
  else
    echo "! Let's Encrypt issuance for $DOMAIN failed (does the domain resolve to" >&2
    echo "  this host and is port 80 reachable?). Continuing with the self-signed" >&2
    echo "  certificate — fix DNS/firewall and re-run the installer to retry." >&2
  fi
fi

run_with_spinner "Configuring systemd unit" bash -c "
  install -m 0644 \"$SOURCE_DIR/contrib/systemd/brrewery.service\" /etc/systemd/system/brrewery.service &&
    systemctl daemon-reload &&
    systemctl enable brrewery &&
    systemctl restart brrewery
"

run_with_log "Creating admin user" bash -c "
  if ! \"$BINARY_DEST\" create-admin; then
    echo 'Create admin user (interactive)'
    \"$BINARY_DEST\" create-admin
  fi
"

echo "✓ brrewery installed"
