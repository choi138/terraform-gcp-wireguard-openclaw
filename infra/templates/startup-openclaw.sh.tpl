#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

fetch_secret() {
  local secret_version="$1"
  local token response data_b64 value

  token="$(curl -fsS --connect-timeout 5 --max-time 20 --retry 5 --retry-delay 2 -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" \
    | sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  if [ -z "$token" ]; then
    echo "Failed to get metadata access token for Secret Manager." >&2
    return 1
  fi

  response="$(curl -fsS --connect-timeout 5 --max-time 20 --retry 5 --retry-delay 2 -H "Authorization: Bearer $token" \
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

# Ensure SSH server is installed and host keys exist (helps internal access).
if ! dpkg -s openssh-server >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y openssh-server
fi

if [ ! -s /etc/ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -A
fi

systemctl enable ssh
systemctl restart ssh || true

# Install Node.js 22 (via NodeSource) if missing.
if ! command -v node >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y curl ca-certificates gnupg
  curl -fsS --connect-timeout 5 --max-time 60 --retry 5 --retry-delay 2 \
    -o /tmp/nodesource_setup.sh https://deb.nodesource.com/setup_22.x
  bash /tmp/nodesource_setup.sh
  rm -f /tmp/nodesource_setup.sh
  apt-get install -y nodejs
fi

# Install OpenClaw CLI if missing.
if ! command -v openclaw >/dev/null 2>&1; then
  npm config set fund false
  npm config set audit false
  npm config set progress false
  npm config set update-notifier false
  NPM_ROOT="$(npm root -g 2>/dev/null || true)"
  if [ -z "$NPM_ROOT" ]; then
    NPM_ROOT="/usr/lib/node_modules"
  fi
  if [ -d "$NPM_ROOT/openclaw" ]; then
    rm -rf "$NPM_ROOT/openclaw"
  fi
  npm install -g openclaw@${openclaw_version} --omit=dev --no-audit --no-fund
fi

# Resolve OpenClaw binary path (npm global bin can be /usr/bin or /usr/local/bin).
OPENCLAW_BIN="$(command -v openclaw || true)"
if [ -z "$OPENCLAW_BIN" ]; then
  NPM_BIN="$(npm bin -g 2>/dev/null || true)"
  if [ -n "$NPM_BIN" ] && [ -x "$NPM_BIN/openclaw" ]; then
    OPENCLAW_BIN="$NPM_BIN/openclaw"
  elif [ -x /usr/bin/openclaw ]; then
    OPENCLAW_BIN="/usr/bin/openclaw"
  elif [ -x /usr/local/bin/openclaw ]; then
    OPENCLAW_BIN="/usr/local/bin/openclaw"
  fi
fi

if [ -z "$OPENCLAW_BIN" ]; then
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

OPENCLAW_GATEWAY_PASSWORD_VALUE="$(fetch_secret "${openclaw_gateway_password_secret_version}")" || {
  echo "Unable to read openclaw_gateway_password_secret_version from Secret Manager." >&2
  exit 1
}

if [ -z "$OPENCLAW_GATEWAY_PASSWORD_VALUE" ]; then
  echo "OpenClaw gateway password is required from Secret Manager." >&2
  exit 1
fi

OPENCLAW_ANTHROPIC_API_KEY_VALUE=""
%{ if openclaw_anthropic_api_key_secret_version != "" }
OPENCLAW_ANTHROPIC_API_KEY_VALUE="$(fetch_secret "${openclaw_anthropic_api_key_secret_version}")" || {
  echo "Unable to read openclaw_anthropic_api_key_secret_version from Secret Manager." >&2
  exit 1
}
%{ endif }

OPENCLAW_TELEGRAM_BOT_TOKEN_VALUE=""
%{ if openclaw_telegram_bot_token_secret_version != "" }
OPENCLAW_TELEGRAM_BOT_TOKEN_VALUE="$(fetch_secret "${openclaw_telegram_bot_token_secret_version}")" || {
  echo "Unable to read openclaw_telegram_bot_token_secret_version from Secret Manager." >&2
  exit 1
}
%{ endif }

# Environment file to avoid shell expansion of secrets.
cat > /opt/openclaw/openclaw.env <<'ENVEOF'
ENVEOF
printf 'OPENCLAW_GATEWAY_PASSWORD=%s\n' "$OPENCLAW_GATEWAY_PASSWORD_VALUE" >> /opt/openclaw/openclaw.env

if [ -n "$OPENCLAW_ANTHROPIC_API_KEY_VALUE" ]; then
  printf 'ANTHROPIC_API_KEY=%s\n' "$OPENCLAW_ANTHROPIC_API_KEY_VALUE" >> /opt/openclaw/openclaw.env
fi

if [ -n "$OPENCLAW_TELEGRAM_BOT_TOKEN_VALUE" ]; then
  printf 'TELEGRAM_BOT_TOKEN=%s\n' "$OPENCLAW_TELEGRAM_BOT_TOKEN_VALUE" >> /opt/openclaw/openclaw.env
fi

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
%{ if openclaw_telegram_enabled }
      "enabled": true,
%{ else }
      "enabled": false,
%{ endif }
      "dmPolicy": "pairing"
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": ${jsonencode(openclaw_model_primary)},
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
ExecStart=$OPENCLAW_BIN gateway
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now openclaw
