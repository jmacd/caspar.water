output "email_endpoint" {
  value = "https://${azurerm_communication_service.reports.hostname}"
}

output "email_access_key" {
  value     = azurerm_communication_service.reports.primary_key
  sensitive = true
}

output "sender_domain" {
  value = azurerm_email_communication_service_domain.reports.from_sender_domain
}

output "sender_address" {
  value = "${azurerm_email_communication_service_domain_sender_username.reports.name}@${azurerm_email_communication_service_domain.reports.from_sender_domain}"
}

output "verification_records" {
  value = local.records
}
