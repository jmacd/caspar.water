terraform {
  required_version = ">= 1.5"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    linode = {
      source  = "linode/linode"
      version = "~> 3.0"
    }
  }
}

provider "azurerm" {
  features {}
}

provider "linode" {
  token = var.linode_token
}

data "azurerm_resource_group" "email" {
  name = var.resource_group_name
}

data "linode_domain" "casparwater" {
  domain = var.domain_name
}

resource "azurerm_email_communication_service" "reports" {
  name                = var.email_service_name
  resource_group_name = data.azurerm_resource_group.email.name
  data_location       = var.data_location

  tags = {
    service = "caspar-water"
    purpose = "weekly-reports"
  }
}

resource "azurerm_email_communication_service_domain" "reports" {
  name                             = var.domain_name
  email_service_id                 = azurerm_email_communication_service.reports.id
  domain_management                = "CustomerManaged"
  user_engagement_tracking_enabled = false

  tags = {
    service = "caspar-water"
    purpose = "weekly-reports"
  }
}

resource "azurerm_communication_service" "reports" {
  name                = var.communication_service_name
  resource_group_name = data.azurerm_resource_group.email.name
  data_location       = var.data_location

  tags = {
    service = "caspar-water"
    purpose = "weekly-reports"
  }
}

resource "azurerm_communication_service_email_domain_association" "reports" {
  communication_service_id = azurerm_communication_service.reports.id
  email_service_domain_id  = azurerm_email_communication_service_domain.reports.id
}

locals {
  verification = azurerm_email_communication_service_domain.reports.verification_records[0]
  records = {
    domain = local.verification.domain[0]
    spf    = local.verification.spf[0]
    dkim   = local.verification.dkim[0]
    dkim2  = local.verification.dkim2[0]
    dmarc  = local.verification.dmarc[0]
  }
  record_names = {
    for name, record in local.records :
    name => contains(["@", var.domain_name, "${var.domain_name}."], record.name) ? "" : trimsuffix(
      trimsuffix(record.name, "."),
      ".${var.domain_name}",
    )
  }
}

resource "linode_domain_record" "email_verification" {
  for_each = local.records

  domain_id   = data.linode_domain.casparwater.id
  name        = local.record_names[each.key]
  record_type = upper(each.value.type)
  target      = trimsuffix(each.value.value, ".")
  ttl_sec     = each.value.ttl
}
