output "storage_account_name" {
  value = azurerm_storage_account.backups.name
}

output "tenant_id" {
  value = data.azuread_client_config.current.tenant_id
}

output "producer_credentials" {
  sensitive = true
  value = {
    for name in local.producers : name => {
      client_id     = azuread_application.pond[name].client_id
      client_secret = azuread_application_password.pond[name].value
    }
  }
}

output "site_credentials" {
  sensitive = true
  value = {
    client_id     = azuread_application.pond["site-prod"].client_id
    client_secret = azuread_application_password.pond["site-prod"].value
  }
}

output "container_urls" {
  value = {
    for name, container in azurerm_storage_container.ponds :
    name => container.url
  }
}
