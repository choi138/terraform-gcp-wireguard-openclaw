output "vpn_public_ip" {
  description = "Static external IP for the VPN server."
  value       = google_compute_address.vpn_ip.address
}

output "wgeasy_ui_url" {
  description = "wg-easy UI URL (HTTP)."
  value       = "http://${google_compute_address.vpn_ip.address}:${var.wgeasy_ui_port}"
}

output "wireguard_port" {
  description = "WireGuard UDP port."
  value       = var.wg_port
}

output "vpn_internal_ip" {
  description = "Internal IP address of the VPN VM."
  value       = google_compute_address.vpn_internal_ip.address
}

output "openclaw_internal_ip" {
  description = "Internal IP address of the OpenClaw VM."
  value       = google_compute_address.openclaw_internal_ip.address
}

output "openclaw_gateway_url" {
  description = "OpenClaw gateway URL (VPN-only)."
  value       = "http://${google_compute_address.openclaw_internal_ip.address}:${var.openclaw_gateway_port}"
}
