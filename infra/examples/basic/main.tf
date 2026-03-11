terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.22.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

module "wireguard" {
  source = "../.."

  project_id = var.project_id
  region     = var.region
  zone       = var.zone

  instance_name = var.instance_name
  machine_type  = var.machine_type

  ssh_source_ranges = var.ssh_source_ranges
  ui_source_ranges  = var.ui_source_ranges

  wg_default_dns = var.wg_default_dns
  wg_port        = var.wg_port
  wgeasy_ui_port = var.wgeasy_ui_port
  wg_host        = var.wg_host

  wgeasy_password_secret      = var.wgeasy_password_secret
  wgeasy_password_hash_secret = var.wgeasy_password_hash_secret

  enable_project_oslogin = var.enable_project_oslogin

  openclaw_instance_name           = var.openclaw_instance_name
  openclaw_machine_type            = var.openclaw_machine_type
  openclaw_zone                    = var.openclaw_zone
  openclaw_gateway_port            = var.openclaw_gateway_port
  openclaw_gateway_password_secret = var.openclaw_gateway_password_secret

  openclaw_anthropic_api_key_secret  = var.openclaw_anthropic_api_key_secret
  openclaw_openai_api_key_secret     = var.openclaw_openai_api_key_secret
  openclaw_model_primary             = var.openclaw_model_primary
  openclaw_model_fallbacks           = var.openclaw_model_fallbacks
  openclaw_telegram_bot_token_secret = var.openclaw_telegram_bot_token_secret
  openclaw_enable_public_ip          = var.openclaw_enable_public_ip
}
