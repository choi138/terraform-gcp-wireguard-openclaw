output "vpn_public_ip" {
  value = module.wireguard.vpn_public_ip
}

output "wgeasy_ui_url" {
  value = module.wireguard.wgeasy_ui_url
}

output "wireguard_port" {
  value = module.wireguard.wireguard_port
}

output "openclaw_internal_ip" {
  value = module.wireguard.openclaw_internal_ip
}

output "openclaw_gateway_url" {
  value = module.wireguard.openclaw_gateway_url
}
