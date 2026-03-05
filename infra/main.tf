locals {
  vpn_tag      = "wg-vpn"
  openclaw_tag = "openclaw"

  # Trim to avoid accidental newline/space in credentials.
  wgeasy_password_secret      = var.wgeasy_password_secret == null ? "" : trimspace(var.wgeasy_password_secret)
  wgeasy_password_hash_secret = var.wgeasy_password_hash_secret == null ? "" : trimspace(var.wgeasy_password_hash_secret)

  wg_host = (var.wg_host != null && trimspace(var.wg_host) != "") ? var.wg_host : google_compute_address.vpn_ip.address

  openclaw_password_secret = trimspace(var.openclaw_gateway_password_secret)
  openclaw_version         = trimspace(var.openclaw_version)

  openclaw_bot_token_secret = var.openclaw_telegram_bot_token_secret == null ? "" : trimspace(var.openclaw_telegram_bot_token_secret)
  openclaw_api_key_secret   = var.openclaw_anthropic_api_key_secret == null ? "" : trimspace(var.openclaw_anthropic_api_key_secret)

  openclaw_model_primary = trimspace(var.openclaw_model_primary)
  openclaw_model_fallbacks = [
    for model in var.openclaw_model_fallbacks : trimspace(model)
    if trimspace(model) != ""
  ]

  openclaw_model_fallbacks_json = jsonencode(local.openclaw_model_fallbacks)
  openclaw_zone                 = (var.openclaw_zone == null || trimspace(var.openclaw_zone) == "") ? var.zone : var.openclaw_zone

  wgeasy_password_secret_id      = local.wgeasy_password_secret == "" ? "" : split("/versions/", local.wgeasy_password_secret)[0]
  wgeasy_password_hash_secret_id = local.wgeasy_password_hash_secret == "" ? "" : split("/versions/", local.wgeasy_password_hash_secret)[0]

  wgeasy_password_secret_version = local.wgeasy_password_secret == "" ? "" : (
    can(regex("/versions/[^/]+$", local.wgeasy_password_secret)) ? local.wgeasy_password_secret : "${local.wgeasy_password_secret}/versions/latest"
  )

  wgeasy_password_hash_secret_version = local.wgeasy_password_hash_secret == "" ? "" : (
    can(regex("/versions/[^/]+$", local.wgeasy_password_hash_secret)) ? local.wgeasy_password_hash_secret : "${local.wgeasy_password_hash_secret}/versions/latest"
  )

  openclaw_password_secret_id  = local.openclaw_password_secret == "" ? "" : split("/versions/", local.openclaw_password_secret)[0]
  openclaw_api_key_secret_id   = local.openclaw_api_key_secret == "" ? "" : split("/versions/", local.openclaw_api_key_secret)[0]
  openclaw_bot_token_secret_id = local.openclaw_bot_token_secret == "" ? "" : split("/versions/", local.openclaw_bot_token_secret)[0]

  openclaw_password_secret_version = local.openclaw_password_secret == "" ? "" : (
    can(regex("/versions/[^/]+$", local.openclaw_password_secret)) ? local.openclaw_password_secret : "${local.openclaw_password_secret}/versions/latest"
  )

  openclaw_api_key_secret_version = local.openclaw_api_key_secret == "" ? "" : (
    can(regex("/versions/[^/]+$", local.openclaw_api_key_secret)) ? local.openclaw_api_key_secret : "${local.openclaw_api_key_secret}/versions/latest"
  )

  openclaw_bot_token_secret_version = local.openclaw_bot_token_secret == "" ? "" : (
    can(regex("/versions/[^/]+$", local.openclaw_bot_token_secret)) ? local.openclaw_bot_token_secret : "${local.openclaw_bot_token_secret}/versions/latest"
  )

  vpn_secret_ids      = toset(compact([local.wgeasy_password_secret_id, local.wgeasy_password_hash_secret_id]))
  openclaw_secret_ids = toset(compact([local.openclaw_password_secret_id, local.openclaw_api_key_secret_id, local.openclaw_bot_token_secret_id]))

  openclaw_telegram_enabled = local.openclaw_bot_token_secret_version != ""

  vpn_service_account_id      = "vpn-${substr(md5(var.instance_name), 0, 24)}"
  openclaw_service_account_id = "openclaw-${substr(md5(var.openclaw_instance_name), 0, 19)}"
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# Default VPC (data source so it's easy to swap to a custom VPC later).
data "google_compute_network" "default" {
  name = "default"
}

data "google_compute_subnetwork" "default" {
  name   = "default"
  region = var.region
}

# Dedicated VM identities (least-privilege secret access).
resource "google_service_account" "vpn" {
  account_id   = local.vpn_service_account_id
  display_name = "WireGuard VM service account (${var.instance_name})"
}

resource "google_service_account" "openclaw" {
  account_id   = local.openclaw_service_account_id
  display_name = "OpenClaw VM service account (${var.openclaw_instance_name})"
}

# Secret Manager access is granted only to referenced secrets.
resource "google_secret_manager_secret_iam_member" "vpn_secret_access" {
  for_each = local.vpn_secret_ids

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.vpn.email}"
}

resource "google_secret_manager_secret_iam_member" "openclaw_secret_access" {
  for_each = local.openclaw_secret_ids

  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.openclaw.email}"
}

# Static external IP for the VPN endpoint.
resource "google_compute_address" "vpn_ip" {
  name   = "${var.instance_name}-ip"
  region = var.region
}

# Reserved internal IP for the VPN VM.
resource "google_compute_address" "vpn_internal_ip" {
  name         = "${var.instance_name}-internal-ip"
  region       = var.region
  address_type = "INTERNAL"
  subnetwork   = data.google_compute_subnetwork.default.id
  address      = (var.vpn_internal_ip_address != null && trimspace(var.vpn_internal_ip_address) != "") ? var.vpn_internal_ip_address : null
}

# WireGuard UDP ingress.
resource "google_compute_firewall" "wireguard" {
  name    = "${var.instance_name}-wg"
  network = data.google_compute_network.default.id

  direction     = "INGRESS"
  source_ranges = ["0.0.0.0/0"]
  target_tags   = [local.vpn_tag]

  allow {
    protocol = "udp"
    ports    = [tostring(var.wg_port)]
  }
}

# SSH ingress (restricted).
resource "google_compute_firewall" "ssh" {
  name    = "${var.instance_name}-ssh"
  network = data.google_compute_network.default.id

  direction     = "INGRESS"
  source_ranges = var.ssh_source_ranges
  target_tags   = [local.vpn_tag]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

# wg-easy UI ingress (restricted).
resource "google_compute_firewall" "wgeasy_ui" {
  name    = "${var.instance_name}-wgeasy-ui"
  network = data.google_compute_network.default.id

  direction     = "INGRESS"
  source_ranges = var.ui_source_ranges
  target_tags   = [local.vpn_tag]

  allow {
    protocol = "tcp"
    ports    = [tostring(var.wgeasy_ui_port)]
  }
}

# OpenClaw gateway ingress (VPN-only via source tag).
resource "google_compute_firewall" "openclaw_gateway" {
  name    = "${var.openclaw_instance_name}-gateway"
  network = data.google_compute_network.default.id

  direction   = "INGRESS"
  source_tags = [local.vpn_tag]
  target_tags = [local.openclaw_tag]

  allow {
    protocol = "tcp"
    ports    = [tostring(var.openclaw_gateway_port)]
  }
}

# OpenClaw SSH ingress (VPN-only via source tag).
resource "google_compute_firewall" "openclaw_ssh" {
  name    = "${var.openclaw_instance_name}-ssh"
  network = data.google_compute_network.default.id

  direction   = "INGRESS"
  source_tags = [local.vpn_tag]
  target_tags = [local.openclaw_tag]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

# Cloud NAT for private OpenClaw VM outbound access.
resource "google_compute_router" "openclaw_router" {
  name    = "${var.openclaw_instance_name}-router"
  network = data.google_compute_network.default.id
  region  = var.region
}

# Reserved internal IP for the OpenClaw VM.
resource "google_compute_address" "openclaw_internal_ip" {
  name         = "${var.openclaw_instance_name}-internal-ip"
  region       = var.region
  address_type = "INTERNAL"
  subnetwork   = data.google_compute_subnetwork.default.id
  address      = (var.openclaw_internal_ip_address != null && trimspace(var.openclaw_internal_ip_address) != "") ? var.openclaw_internal_ip_address : null
}

resource "google_compute_router_nat" "openclaw_nat" {
  name   = "${var.openclaw_instance_name}-nat"
  region = var.region
  router = google_compute_router.openclaw_router.name

  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# Optional project-level OS Login enablement (single metadata item).
resource "google_compute_project_metadata_item" "oslogin" {
  count   = var.enable_project_oslogin ? 1 : 0
  project = var.project_id

  key   = "enable-oslogin"
  value = "TRUE"
}

resource "google_compute_instance" "vpn" {
  depends_on = [google_secret_manager_secret_iam_member.vpn_secret_access]

  name         = var.instance_name
  machine_type = var.machine_type
  zone         = var.zone

  # Required for forwarding VPN traffic.
  can_ip_forward = true

  tags = [local.vpn_tag]

  boot_disk {
    initialize_params {
      # Ubuntu LTS image family.
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
    }
  }

  network_interface {
    network    = data.google_compute_network.default.id
    subnetwork = data.google_compute_subnetwork.default.id
    network_ip = google_compute_address.vpn_internal_ip.address

    access_config {
      nat_ip = google_compute_address.vpn_ip.address
    }
  }

  service_account {
    email  = google_service_account.vpn.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  metadata = {
    enable-oslogin = "TRUE"
    startup-script = templatefile("${path.module}/templates/startup.sh.tpl", {
      wg_host                             = local.wg_host
      wg_port                             = var.wg_port
      wg_default_dns                      = var.wg_default_dns
      wgeasy_ui_port                      = var.wgeasy_ui_port
      wgeasy_password_secret_version      = local.wgeasy_password_secret_version
      wgeasy_password_hash_secret_version = local.wgeasy_password_hash_secret_version
    })
  }
}

resource "google_compute_instance" "openclaw" {
  depends_on = [
    google_compute_router_nat.openclaw_nat,
    google_secret_manager_secret_iam_member.openclaw_secret_access,
  ]

  name         = var.openclaw_instance_name
  machine_type = var.openclaw_machine_type
  zone         = local.openclaw_zone

  tags = [local.openclaw_tag]

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
    }
  }

  network_interface {
    network    = data.google_compute_network.default.id
    subnetwork = data.google_compute_subnetwork.default.id
    network_ip = google_compute_address.openclaw_internal_ip.address

    dynamic "access_config" {
      for_each = var.openclaw_enable_public_ip ? [1] : []
      content {}
    }
  }

  service_account {
    email  = google_service_account.openclaw.email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  metadata = {
    enable-oslogin = "TRUE"
    startup-script = templatefile("${path.module}/templates/startup-openclaw.sh.tpl", {
      openclaw_gateway_port                      = var.openclaw_gateway_port
      openclaw_gateway_password_secret_version   = local.openclaw_password_secret_version
      openclaw_version                           = local.openclaw_version
      openclaw_telegram_bot_token_secret_version = local.openclaw_bot_token_secret_version
      openclaw_telegram_enabled                  = local.openclaw_telegram_enabled
      openclaw_anthropic_api_key_secret_version  = local.openclaw_api_key_secret_version
      openclaw_model_primary                     = local.openclaw_model_primary
      openclaw_model_fallbacks_json              = local.openclaw_model_fallbacks_json
    })
  }
}
