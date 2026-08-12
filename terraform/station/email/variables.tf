variable "resource_group_name" {
  description = "Existing Azure resource group for Caspar Water infrastructure."
  type        = string
  default     = "caspar-water-production-backups"
}

variable "data_location" {
  description = "ACS data residency region."
  type        = string
  default     = "United States"
}

variable "domain_name" {
  description = "Customer-managed sender domain."
  type        = string
  default     = "casparwater.us"
}

variable "email_service_name" {
  description = "Azure Email Communication Service name."
  type        = string
  default     = "caspar-water-email"
}

variable "communication_service_name" {
  description = "Azure Communication Service name."
  type        = string
  default     = "caspar-water-reports"
}

variable "linode_token" {
  description = "Linode API token used only to create domain verification records."
  type        = string
  sensitive   = true
}
