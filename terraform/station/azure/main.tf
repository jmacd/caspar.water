terraform {
  required_version = ">= 1.5"

  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azuread" {}

provider "azurerm" {
  features {}
  storage_use_azuread = true
}

data "azuread_client_config" "current" {}

locals {
  producers = toset([
    "noyo-prod",
    "septic-prod",
    "water-prod",
  ])
  target_producers = {
    for producer in local.producers :
    producer => "${producer}-0002"
  }
  identities = setunion(local.producers, toset(["site-prod"]))
}

resource "azurerm_resource_group" "backups" {
  name     = var.resource_group_name
  location = var.location

  tags = {
    service = "caspar-water"
    purpose = "production-backups"
  }
}

resource "azurerm_storage_account" "backups" {
  name                             = var.storage_account_name
  resource_group_name              = azurerm_resource_group.backups.name
  location                         = azurerm_resource_group.backups.location
  account_kind                     = "StorageV2"
  account_tier                     = "Standard"
  account_replication_type         = "LRS"
  access_tier                      = "Hot"
  min_tls_version                  = "TLS1_2"
  https_traffic_only_enabled       = true
  allow_nested_items_to_be_public  = false
  public_network_access_enabled    = true
  shared_access_key_enabled        = false
  default_to_oauth_authentication  = true
  cross_tenant_replication_enabled = false

  blob_properties {
    change_feed_enabled = false
    versioning_enabled  = false
  }

  tags = {
    service = "caspar-water"
    purpose = "production-backups"
  }
}

resource "azurerm_storage_container" "ponds" {
  for_each = local.producers

  name                  = each.key
  storage_account_id    = azurerm_storage_account.backups.id
  container_access_type = "private"
}

# The production migration writes only to a fresh container generation.
# Keeping these resources separate preserves the original containers and their
# Terraform addresses until the post-cutover retention decision.
resource "azurerm_storage_container" "migration_targets" {
  for_each = local.target_producers

  name                  = each.value
  storage_account_id    = azurerm_storage_account.backups.id
  container_access_type = "private"
}

resource "azuread_application" "pond" {
  for_each = local.identities

  display_name = "caspar-water-${each.key}"
  owners       = [data.azuread_client_config.current.object_id]
}

resource "azuread_service_principal" "pond" {
  for_each = local.identities

  client_id                    = azuread_application.pond[each.key].client_id
  app_role_assignment_required = false
  owners                       = [data.azuread_client_config.current.object_id]
}

resource "azuread_application_password" "pond" {
  for_each = local.identities

  application_id = azuread_application.pond[each.key].id
  display_name   = "watertown"
  end_date       = var.credential_end_date
}

resource "azurerm_role_assignment" "producer" {
  for_each = local.producers

  scope                = azurerm_storage_container.ponds[each.key].id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azuread_service_principal.pond[each.key].object_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_role_assignment" "site" {
  for_each = local.producers

  scope                = azurerm_storage_container.ponds[each.key].id
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = azuread_service_principal.pond["site-prod"].object_id
  principal_type       = "ServicePrincipal"
}

# Reuse the existing least-privilege identities for the one-time migration:
# each producer can seed only its own target, while site-prod remains
# read-only across the producer targets.
resource "azurerm_role_assignment" "migration_target_producer" {
  for_each = local.target_producers

  scope                = azurerm_storage_container.migration_targets[each.key].id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azuread_service_principal.pond[each.key].object_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_role_assignment" "migration_target_site" {
  for_each = local.target_producers

  scope                = azurerm_storage_container.migration_targets[each.key].id
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = azuread_service_principal.pond["site-prod"].object_id
  principal_type       = "ServicePrincipal"
}

resource "azurerm_consumption_budget_resource_group" "backups" {
  name              = "caspar-water-production-backups"
  resource_group_id = azurerm_resource_group.backups.id
  amount            = var.monthly_budget_usd
  time_grain        = "Monthly"

  time_period {
    start_date = var.budget_start_date
  }

  notification {
    enabled        = true
    threshold      = 50
    operator       = "GreaterThanOrEqualTo"
    threshold_type = "Actual"
    contact_emails = var.budget_contact_emails
  }

  notification {
    enabled        = true
    threshold      = 80
    operator       = "GreaterThanOrEqualTo"
    threshold_type = "Forecasted"
    contact_emails = var.budget_contact_emails
  }

  notification {
    enabled        = true
    threshold      = 100
    operator       = "GreaterThanOrEqualTo"
    threshold_type = "Actual"
    contact_emails = var.budget_contact_emails
  }
}
