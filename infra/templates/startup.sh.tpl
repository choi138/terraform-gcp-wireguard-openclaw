#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

if ! command -v docker >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y docker.io
  systemctl enable --now docker
fi

if ! command -v curl >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y curl ca-certificates
fi

# Ensure SSH daemon is present and running (helps recover access if SSH is down).
if ! systemctl is-active --quiet ssh; then
  apt-get update -y
  apt-get install -y openssh-server
  systemctl enable --now ssh
fi

# Best-effort module load for WireGuard.
modprobe wireguard || true
modprobe ip_tables || true

fetch_secret() {
  local secret_version="$1"
  local token response data_b64 value

  token="$(curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" \
    | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  if [ -z "$token" ]; then
    echo "Failed to get metadata access token for Secret Manager." >&2
    return 1
  fi

  response="$(curl -fsS -H "Authorization: Bearer $token" \
    "https://secretmanager.googleapis.com/v1/$secret_version:access")" || {
    echo "Failed to access secret version: $secret_version" >&2
    return 1
  }

  data_b64="$(printf '%s' "$response" | tr -d '\n' | sed -n 's/.*"data"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  if [ -z "$data_b64" ]; then
    echo "Secret payload was empty for: $secret_version" >&2
    return 1
  fi

  value="$(printf '%s' "$data_b64" | tr '_-' '/+' | base64 -d 2>/dev/null || true)"
  if [ -z "$value" ]; then
    echo "Failed to decode secret payload for: $secret_version" >&2
    return 1
  fi

  printf '%s' "$value"
}

build_password_hash() {
  local plain_password="$1"
  local hash_line hash

  hash_line="$(docker run --rm ghcr.io/wg-easy/wg-easy:14 wgpw "$plain_password" | tail -n 1 || true)"
  hash="$(echo "$hash_line" | sed -n "s/^PASSWORD_HASH='\\(.*\\)'$/\\1/p")"
  if [ -z "$hash" ]; then
    hash="$(echo "$hash_line" | sed -n "s/^PASSWORD_HASH=\\(.*\\)$/\\1/p")"
  fi
  if [ -z "$hash" ]; then
    echo "Failed to generate PASSWORD_HASH from provided password." >&2
    return 1
  fi

  printf '%s' "$hash"
}

install -d -m 700 /opt/wg-easy

# Create env file for wg-easy (protects $ in PASSWORD_HASH values).
cat > /opt/wg-easy/wg-easy.env <<'WGEOF'
WG_HOST=${wg_host}
WG_PORT=${wg_port}
WG_DEFAULT_DNS=${wg_default_dns}
PORT=${wgeasy_ui_port}
INSECURE=true
WGEOF

%{ if wgeasy_password_hash_secret_version != "" }
WGEASY_HASH_FROM_SECRET="$(fetch_secret "${wgeasy_password_hash_secret_version}")" || {
  echo "Unable to read wgeasy_password_hash_secret_version from Secret Manager." >&2
  exit 1
}
printf 'PASSWORD_HASH=%s\n' "$WGEASY_HASH_FROM_SECRET" >> /opt/wg-easy/wg-easy.env
%{ endif }

%{ if wgeasy_password_secret_version != "" }
WGEASY_PASSWORD_FROM_SECRET="$(fetch_secret "${wgeasy_password_secret_version}")" || {
  echo "Unable to read wgeasy_password_secret_version from Secret Manager." >&2
  exit 1
}
WGEASY_HASH_FROM_SECRET_PASSWORD="$(build_password_hash "$WGEASY_PASSWORD_FROM_SECRET")" || {
  echo "Unable to generate PASSWORD_HASH from secret-based wg-easy password." >&2
  exit 1
}
printf 'PASSWORD_HASH=%s\n' "$WGEASY_HASH_FROM_SECRET_PASSWORD" >> /opt/wg-easy/wg-easy.env
%{ endif }

if ! grep -q '^PASSWORD_HASH=' /opt/wg-easy/wg-easy.env; then
  echo "wg-easy PASSWORD_HASH was not configured from Secret Manager." >&2
  exit 1
fi

# Replace container on every boot to pick up config changes.
docker rm -f wg-easy >/dev/null 2>&1 || true

docker run -d \
  --name wg-easy \
  --restart unless-stopped \
  --env-file /opt/wg-easy/wg-easy.env \
  -p ${wg_port}:${wg_port}/udp \
  -p ${wgeasy_ui_port}:${wgeasy_ui_port}/tcp \
  -v /opt/wg-easy:/etc/wireguard \
  --device /dev/net/tun \
  --cap-add=NET_ADMIN \
  --cap-add=SYS_MODULE \
  --sysctl net.ipv4.ip_forward=1 \
  --sysctl net.ipv4.conf.all.src_valid_mark=1 \
  ghcr.io/wg-easy/wg-easy:14
