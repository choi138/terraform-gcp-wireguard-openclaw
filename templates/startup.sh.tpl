#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

if ! command -v docker >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y docker.io
  systemctl enable --now docker
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

install -d -m 700 /opt/wg-easy

# Create env file for wg-easy (protects $ in PASSWORD_HASH values).
cat > /opt/wg-easy/wg-easy.env <<'WGEOF'
WG_HOST=${wg_host}
WG_PORT=${wg_port}
WG_DEFAULT_DNS=${wg_default_dns}
PORT=${wgeasy_ui_port}
INSECURE=true
WGEOF

%{ if wgeasy_password_hash != "" }
cat >> /opt/wg-easy/wg-easy.env <<'WGEOF'
PASSWORD_HASH=${wgeasy_password_hash}
WGEOF
%{ endif }

%{ if wgeasy_password != "" }
# Generate a bcrypt hash using the wg-easy helper.
cat > /opt/wg-easy/.wgeasy_password <<'PWEOF'
${wgeasy_password}
PWEOF
WGEASY_PASSWORD="$(cat /opt/wg-easy/.wgeasy_password)"
rm -f /opt/wg-easy/.wgeasy_password

HASH_LINE="$(docker run --rm ghcr.io/wg-easy/wg-easy:14 wgpw "$${WGEASY_PASSWORD}" | tail -n 1 || true)"
HASH="$(echo "$${HASH_LINE}" | sed -n "s/^PASSWORD_HASH='\\(.*\\)'$/\\1/p")"
if [ -z "$${HASH}" ]; then
  HASH="$(echo "$${HASH_LINE}" | sed -n "s/^PASSWORD_HASH=\\(.*\\)$/\\1/p")"
fi
if [ -z "$${HASH}" ]; then
  echo "Failed to generate PASSWORD_HASH. Set wgeasy_password_hash instead." >&2
  exit 1
fi
printf 'PASSWORD_HASH=%s\n' "$${HASH}" >> /opt/wg-easy/wg-easy.env
%{ endif }

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
