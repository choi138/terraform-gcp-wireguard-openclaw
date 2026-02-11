variable "project_id" {
  type        = string
  description = "GCP project ID."
}

variable "region" {
  type        = string
  description = "GCP region for the VM and the static IP."
}

variable "zone" {
  type        = string
  description = "GCP zone for the VM."
}

variable "instance_name" {
  type        = string
  description = "Compute Engine instance name."
}

variable "machine_type" {
  type        = string
  description = "GCE machine type (e.g., e2-micro)."
}

variable "vpn_internal_ip_address" {
  type        = string
  description = "Optional fixed internal IP for the VPN VM. If null/empty, a reserved internal IP is auto-allocated."
  default     = null
}

variable "ssh_source_ranges" {
  type        = list(string)
  description = "Allowed CIDR ranges for SSH (TCP 22). Use your public IP/32."
  default     = ["0.0.0.0/32"]
}

variable "ui_source_ranges" {
  type        = list(string)
  description = "Allowed CIDR ranges for wg-easy UI (TCP 51821). Keep this very restricted."
  default     = ["0.0.0.0/32"]
}

variable "wg_default_dns" {
  type        = string
  description = "Default DNS for WireGuard clients (e.g., 1.1.1.1 or 1.1.1.1,8.8.8.8)."
}

variable "wg_port" {
  type        = number
  description = "WireGuard UDP port."
  default     = 51820

  validation {
    condition     = var.wg_port >= 1 && var.wg_port <= 65535
    error_message = "wg_port must be between 1 and 65535."
  }
}

variable "wgeasy_ui_port" {
  type        = number
  description = "wg-easy UI TCP port."
  default     = 51821

  validation {
    condition     = var.wgeasy_ui_port >= 1 && var.wgeasy_ui_port <= 65535
    error_message = "wgeasy_ui_port must be between 1 and 65535."
  }
}

variable "wg_host" {
  type        = string
  description = "Optional public hostname or IP for WG_HOST. If null/empty, the static IP is used."
  default     = null
}

variable "wgeasy_password" {
  type        = string
  description = "Plaintext admin password. On boot, a bcrypt hash is generated (wgpw) and stored as PASSWORD_HASH. Set exactly one of wgeasy_password or wgeasy_password_hash."
  default     = null
  sensitive   = true

  validation {
    condition = (
      (var.wgeasy_password == null ? "" : trimspace(var.wgeasy_password)) != ""
    ) != (
      (var.wgeasy_password_hash == null ? "" : trimspace(var.wgeasy_password_hash)) != ""
    )
    error_message = "Set exactly one of wgeasy_password or wgeasy_password_hash."
  }
}

variable "wgeasy_password_hash" {
  type        = string
  description = "bcrypt password hash for wg-easy (PASSWORD_HASH). Recommended if you want to avoid plaintext on the VM. Set exactly one of wgeasy_password or wgeasy_password_hash."
  default     = null
  sensitive   = true
}

variable "enable_project_oslogin" {
  type        = bool
  description = "Optionally enable OS Login at the project level (single metadata item)."
  default     = false
}

variable "openclaw_instance_name" {
  type        = string
  description = "OpenClaw instance name."
  default     = "openclaw"
}

variable "openclaw_machine_type" {
  type        = string
  description = "OpenClaw machine type."
  default     = "e2-micro"
}

variable "openclaw_internal_ip_address" {
  type        = string
  description = "Optional fixed internal IP for the OpenClaw VM. If null/empty, a reserved internal IP is auto-allocated."
  default     = null
}

variable "openclaw_zone" {
  type        = string
  description = "OpenClaw zone (defaults to var.zone if null/empty)."
  default     = null
}

variable "openclaw_gateway_port" {
  type        = number
  description = "OpenClaw gateway port (TCP)."
  default     = 18789

  validation {
    condition     = var.openclaw_gateway_port >= 1 && var.openclaw_gateway_port <= 65535
    error_message = "openclaw_gateway_port must be between 1 and 65535."
  }
}

variable "openclaw_gateway_password" {
  type        = string
  description = "Gateway password for OpenClaw (required)."
  sensitive   = true

  validation {
    condition     = trimspace(var.openclaw_gateway_password) != ""
    error_message = "openclaw_gateway_password must be set."
  }
}

variable "openclaw_version" {
  type        = string
  description = "OpenClaw CLI version to install (pinned for security)."
  default     = "2026.1.30"

  validation {
    condition     = trimspace(var.openclaw_version) != ""
    error_message = "openclaw_version must be set."
  }
}

variable "openclaw_anthropic_api_key" {
  type        = string
  description = "Anthropic API key for OpenClaw (optional; set via TF_VAR_openclaw_anthropic_api_key)."
  default     = ""
  sensitive   = true
}

variable "openclaw_model_primary" {
  type        = string
  description = "Primary OpenClaw model (provider/model)."
  default     = "anthropic/claude-opus-4-6"

  validation {
    condition     = trimspace(var.openclaw_model_primary) != ""
    error_message = "openclaw_model_primary must be set."
  }
}

variable "openclaw_model_fallbacks" {
  type        = list(string)
  description = "Fallback OpenClaw models (provider/model), tried in order."
  default     = ["anthropic/claude-opus-4-5"]

  validation {
    condition     = alltrue([for m in var.openclaw_model_fallbacks : trimspace(m) != ""])
    error_message = "openclaw_model_fallbacks cannot contain empty strings."
  }
}

variable "openclaw_telegram_bot_token" {
  type        = string
  description = "Telegram bot token for OpenClaw (optional)."
  default     = null
  sensitive   = true
}

variable "openclaw_enable_public_ip" {
  type        = bool
  description = "Attach a public IP to the OpenClaw VM (not recommended)."
  default     = false
}
