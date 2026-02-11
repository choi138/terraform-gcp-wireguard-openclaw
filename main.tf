locals {
  vpn_tag      = "wg-vpn"
  openclaw_tag = "openclaw"
  # Trim to avoid accidental newline/space in credentials.
  wgeasy_password      = var.wgeasy_password == null ? "" : trimspace(var.wgeasy_password)
  wgeasy_password_hash = var.wgeasy_password_hash == null ? "" : trimspace(var.wgeasy_password_hash)
  wg_host              = (var.wg_host != null && trimspace(var.wg_host) != "") ? var.wg_host : google_compute_address.vpn_ip.address
  openclaw_password    = var.openclaw_gateway_password == null ? "" : trimspace(var.openclaw_gateway_password)
  openclaw_version     = trimspace(var.openclaw_version)
  openclaw_bot_token   = var.openclaw_telegram_bot_token == null ? "" : trimspace(var.openclaw_telegram_bot_token)
  openclaw_api_key     = var.openclaw_anthropic_api_key == null ? "" : trimspace(var.openclaw_anthropic_api_key)
  openclaw_model_primary = trimspace(var.openclaw_model_primary)
  openclaw_model_fallbacks = [
    for model in var.openclaw_model_fallbacks : trimspace(model)
    if trimspace(model) != ""
  ]
  openclaw_model_fallbacks_json = jsonencode(local.openclaw_model_fallbacks)
  openclaw_zone        = (var.openclaw_zone == null || trimspace(var.openclaw_zone) == "") ? var.zone : var.openclaw_zone
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

  metadata = {
    enable-oslogin = "TRUE"
  }

  metadata_startup_script = templatefile("${path.module}/templates/startup.sh.tpl", {
    wg_host             = local.wg_host
    wg_port             = var.wg_port
    wg_default_dns      = var.wg_default_dns
    wgeasy_ui_port       = var.wgeasy_ui_port
    wgeasy_password      = local.wgeasy_password
    wgeasy_password_hash = local.wgeasy_password_hash
  })
}

resource "google_compute_instance" "openclaw" {
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

  metadata = {
    enable-oslogin = "TRUE"
  }

  metadata_startup_script = templatefile("${path.module}/templates/startup-openclaw.sh.tpl", {
    openclaw_gateway_port         = var.openclaw_gateway_port
    openclaw_gateway_password     = local.openclaw_password
    openclaw_version              = local.openclaw_version
    openclaw_telegram_bot_token   = local.openclaw_bot_token
    openclaw_anthropic_api_key    = local.openclaw_api_key
    openclaw_model_primary        = local.openclaw_model_primary
    openclaw_model_fallbacks_json = local.openclaw_model_fallbacks_json
  })
}
