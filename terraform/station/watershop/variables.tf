variable "host" {
  description = "SSH host"
  default     = "watershop.casparwater.us"
}

variable "user" {
  description = "SSH user"
  default     = "jmacd"
}

variable "ssh_key" {
  description = "Path to SSH private key"
  default     = "~/.ssh/watershop"
}

# MinIO (on watershop, used by staging instances)
variable "minio_endpoint" {
  default = "http://watershop.casparwater.us:9000"
}

variable "minio_access_key" {
  sensitive = true
}

variable "minio_secret_key" {
  sensitive = true
}

variable "azure_storage_account" {
  description = "Azure storage account used by production pond containers."
  type        = string
  default     = ""
}

variable "azure_tenant_id" {
  description = "Azure tenant containing the pond service principals."
  type        = string
  default     = ""
  sensitive   = true
}

variable "azure_producer_credentials" {
  description = "Container-scoped Azure writer credentials by producer instance."
  type = map(object({
    client_id     = string
    client_secret = string
  }))
  sensitive = true
  default = {
    noyo-prod = {
      client_id     = ""
      client_secret = ""
    }
    septic-prod = {
      client_id     = ""
      client_secret = ""
    }
    water-prod = {
      client_id     = ""
      client_secret = ""
    }
  }
}

variable "azure_site_credentials" {
  description = "Azure read-only credentials used by site-prod."
  type = object({
    client_id     = string
    client_secret = string
  })
  sensitive = true
  default = {
    client_id     = ""
    client_secret = ""
  }
}

# HydroVu API
variable "hydrovu_key_id" {
  sensitive = true
}

variable "hydrovu_key_value" {
  sensitive = true
}

# NFS mount paths
variable "water_data_dir" {
  default = "/home/shared/water/archive/data"
}

variable "septic_data_dir" {
  default = "/home/shared/septic/archive/data"
}

variable "deploy_staging" {
  description = "Deploy staging instances"
  type        = bool
  default     = true
}

variable "deploy_selfmon" {
  description = "Deploy the native watershop self-monitor instance"
  type        = bool
  default     = true
}

# Production deployment is opt-in so routine staging/selfmon applies never
# quiesce the continuously running production instances. Production upgrades
# explicitly set this flag after the `prod-<arch>` image has been promoted.
variable "deploy_production" {
  description = "Include production instances in this apply. Enable only for an intentional production deployment."
  type        = bool
  default     = false
}

variable "activate_production_timers" {
  description = "Keep production timers active. Set false only during a controlled production initialization or reset."
  type        = bool
  default     = true
}

variable "weekly_report_email_instances" {
  description = "Site instances with the private weekly email timer enabled."
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for name in var.weekly_report_email_instances :
      contains(["site-staging", "site-prod"], name)
    ])
    error_message = "weekly_report_email_instances may contain only site-staging or site-prod."
  }
}

variable "weekly_report_email_credentials" {
  description = "Private ACS Email endpoint, access key, and report recipient."
  type = object({
    endpoint   = string
    access_key = string
    recipient  = string
  })
  sensitive = true
  default = {
    endpoint   = ""
    access_key = ""
    recipient  = ""
  }
}

# Instances to wipe and re-initialize. DESTRUCTIVE and manual-only: an
# instance named here has its local volume + host dir removed and, for staging,
# its S3 backup bucket emptied before re-initialization. Production producer
# resets are refused because their existing Azure containers cannot attach to a
# newly initialized pond ID; that operation requires a separate controlled
# container replacement and seed. Defaults to empty so routine applies never
# reset; pass explicitly, e.g.
# -var 'reset_instances=["water-staging","site-staging"]'.
variable "reset_instances" {
  description = "Non-production-producer instances to wipe and re-initialize"
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for name in var.reset_instances :
      contains([
        "noyo-prod",
        "noyo-staging",
        "septic-prod",
        "septic-staging",
        "site-prod",
        "site-staging",
        "water-prod",
        "water-staging",
        "watershop-selfmon",
      ], name)
    ])
    error_message = "reset_instances contains an unknown or unsupported reset target."
  }

  validation {
    condition = alltrue([
      for name in var.reset_instances :
      !contains(["noyo-prod", "septic-prod", "site-prod", "water-prod"], name)
    ])
    error_message = "Production resets require a controlled Azure replacement and seed; they cannot use reset_instances."
  }

  validation {
    condition = (
      length(setintersection(
        toset(var.reset_instances),
        toset(["noyo-staging", "septic-staging", "water-staging"]),
      )) == 0 ||
      contains(var.reset_instances, "site-staging")
    )
    error_message = "Resetting a staging producer changes its pond identity; include site-staging in reset_instances so its old graft is replaced."
  }
}

# Git branch for Caspar Water site content (git-ingest)
variable "git_ref" {
  description = "Git branch/ref for staging Caspar Water content (production always uses main)"
  default     = "main"
}

# Git branch for Noyo subsite content, which lives in a separate repository.
variable "noyo_git_ref" {
  description = "Git branch/ref for staging Noyo content (production always uses main)"
  default     = "main"
}

# Cloud host IP for production site deploy (rsync target)
variable "cloud_ip" {
  description = "IP address of the cloud (Linode) host serving casparwater.us"
  default     = "173.255.212.226"
}
