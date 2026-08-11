variable "location" {
  description = "Azure region for production backup storage."
  type        = string
  default     = "westus2"
}

variable "resource_group_name" {
  description = "Resource group containing production backup resources."
  type        = string
  default     = "caspar-water-production-backups"
}

variable "storage_account_name" {
  description = "Globally unique lowercase Azure storage account name."
  type        = string
  default     = "casparwaterprod"

  validation {
    condition     = can(regex("^[a-z0-9]{3,24}$", var.storage_account_name))
    error_message = "storage_account_name must contain 3-24 lowercase letters or digits."
  }
}

variable "monthly_budget_usd" {
  description = "Monthly resource-group budget in USD."
  type        = number
  default     = 10

  validation {
    condition     = var.monthly_budget_usd > 0
    error_message = "monthly_budget_usd must be positive."
  }
}

variable "budget_start_date" {
  description = "First day of the current month in RFC3339 format."
  type        = string
}

variable "credential_end_date" {
  description = "RFC3339 expiry shared by the initial pond client secrets."
  type        = string
}

variable "budget_contact_emails" {
  description = "Recipients for actual and forecasted Azure budget alerts."
  type        = list(string)
  sensitive   = true

  validation {
    condition     = length(var.budget_contact_emails) > 0
    error_message = "At least one budget alert recipient is required."
  }
}
