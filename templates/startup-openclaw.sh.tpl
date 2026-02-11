#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

# Ensure SSH server is installed and running (helps internal access).
if ! systemctl is-active --quiet ssh; then
  apt-get update -y
  apt-get install -y openssh-server
  systemctl enable --now ssh
fi

# Install Node.js 22 (via NodeSource) if missing.
if ! command -v node >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y curl ca-certificates gnupg
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y nodejs
fi

# Install OpenClaw CLI if missing.
if ! command -v openclaw >/dev/null 2>&1; then
  npm config set fund false
  npm config set audit false
  npm config set progress false
  npm config set update-notifier false
  NPM_ROOT="$(npm root -g 2>/dev/null || true)"
  if [ -z "$${NPM_ROOT}" ]; then
    NPM_ROOT="/usr/lib/node_modules"
  fi
  if [ -d "$${NPM_ROOT}/openclaw" ]; then
    rm -rf "$${NPM_ROOT}/openclaw"
  fi
  npm install -g openclaw@${openclaw_version} --omit=dev --no-audit --no-fund
fi

# Resolve OpenClaw binary path (npm global bin can be /usr/bin or /usr/local/bin).
OPENCLAW_BIN="$(command -v openclaw || true)"
if [ -z "$${OPENCLAW_BIN}" ]; then
  NPM_BIN="$(npm bin -g 2>/dev/null || true)"
  if [ -n "$${NPM_BIN}" ] && [ -x "$${NPM_BIN}/openclaw" ]; then
    OPENCLAW_BIN="$${NPM_BIN}/openclaw"
  elif [ -x /usr/bin/openclaw ]; then
    OPENCLAW_BIN="/usr/bin/openclaw"
  elif [ -x /usr/local/bin/openclaw ]; then
    OPENCLAW_BIN="/usr/local/bin/openclaw"
  fi
fi

if [ -z "$${OPENCLAW_BIN}" ]; then
  echo "OpenClaw CLI not found after install." >&2
  exit 1
fi

# Create service user and directories.
if ! id -u openclaw >/dev/null 2>&1; then
  useradd -m -s /bin/bash openclaw
fi

install -d -m 700 -o openclaw -g openclaw /home/openclaw/.openclaw
install -d -m 700 -o openclaw -g openclaw /home/openclaw/.openclaw/state
install -d -m 700 /opt/openclaw

if [ -z "${openclaw_gateway_password}" ]; then
  echo "OpenClaw gateway password is required." >&2
  exit 1
fi

# Environment file to avoid shell expansion of secrets.
cat > /opt/openclaw/openclaw.env <<'ENVEOF'
OPENCLAW_GATEWAY_PASSWORD=${openclaw_gateway_password}
%{ if openclaw_anthropic_api_key != "" }
ANTHROPIC_API_KEY=${openclaw_anthropic_api_key}
%{ endif }
%{ if openclaw_telegram_bot_token != "" }
TELEGRAM_BOT_TOKEN=${openclaw_telegram_bot_token}
%{ endif }
ENVEOF

chmod 600 /opt/openclaw/openclaw.env

# Write OpenClaw config.
cat > /home/openclaw/.openclaw/openclaw.json <<'JSON'
{
  "gateway": {
    "mode": "local",
    "bind": "lan",
    "port": ${openclaw_gateway_port},
    "auth": {
      "mode": "password"
    }
  },
  "channels": {
    "telegram": {
%{ if openclaw_telegram_bot_token != "" }
      "enabled": true,
      "botToken": "${openclaw_telegram_bot_token}",
%{ else }
      "enabled": false,
%{ endif }
      "dmPolicy": "pairing"
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "${openclaw_model_primary}",
        "fallbacks": ${openclaw_model_fallbacks_json}
      }
    }
  }
}
JSON

chown openclaw:openclaw /home/openclaw/.openclaw/openclaw.json
chmod 600 /home/openclaw/.openclaw/openclaw.json

# Convenience wrapper for OpenClaw CLI (short command).
cat > /usr/local/bin/oc <<'OCEOF'
#!/bin/bash
set -euo pipefail
exec sudo -u openclaw env \
  OPENCLAW_CONFIG_PATH=/home/openclaw/.openclaw/openclaw.json \
  OPENCLAW_STATE_DIR=/home/openclaw/.openclaw/state \
  openclaw "$@"
OCEOF

chmod 755 /usr/local/bin/oc

# Systemd service for OpenClaw gateway.
cat > /etc/systemd/system/openclaw.service <<UNIT
[Unit]
Description=OpenClaw Gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=openclaw
Group=openclaw
Environment=OPENCLAW_CONFIG_PATH=/home/openclaw/.openclaw/openclaw.json
Environment=OPENCLAW_STATE_DIR=/home/openclaw/.openclaw/state
EnvironmentFile=/opt/openclaw/openclaw.env
ExecStart=$${OPENCLAW_BIN} gateway
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now openclaw
