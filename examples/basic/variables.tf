variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "zone" {
  type = string
}

variable "instance_name" {
  type = string
}

variable "machine_type" {
  type = string
}

variable "ssh_source_ranges" {
  type = list(string)
}

variable "ui_source_ranges" {
  type = list(string)
}

variable "wg_default_dns" {
  type = string
}

variable "wg_port" {
  type    = number
  default = 51820
}

variable "wgeasy_ui_port" {
  type    = number
  default = 51821
}

variable "wg_host" {
  type    = string
  default = null
}

variable "wgeasy_password" {
  type      = string
  default   = null
  sensitive = true
}

variable "wgeasy_password_hash" {
  type      = string
  default   = null
  sensitive = true
}

variable "enable_project_oslogin" {
  type    = bool
  default = false
}

variable "openclaw_instance_name" {
  type    = string
  default = "openclaw"
}

variable "openclaw_machine_type" {
  type    = string
  default = "e2-micro"
}

variable "openclaw_zone" {
  type    = string
  default = null
}

variable "openclaw_gateway_port" {
  type    = number
  default = 18789
}

variable "openclaw_gateway_password" {
  type      = string
  sensitive = true
}

variable "openclaw_anthropic_api_key" {
  type      = string
  default   = ""
  sensitive = true
}

variable "openclaw_model_primary" {
  type    = string
  default = "anthropic/claude-opus-4-6"
}

variable "openclaw_model_fallbacks" {
  type    = list(string)
  default = ["anthropic/claude-opus-4-5"]
}

variable "openclaw_telegram_bot_token" {
  type      = string
  default   = null
  sensitive = true
}

variable "openclaw_enable_public_ip" {
  type    = bool
  default = false
}
