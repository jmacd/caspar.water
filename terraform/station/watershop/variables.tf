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

# Cloudflare R2
variable "r2_endpoint" {
  sensitive = true
}

variable "r2_access_key" {
  sensitive = true
}

variable "r2_secret_key" {
  sensitive = true
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

variable "deploy_production" {
  description = "Deploy production instances"
  type        = bool
  default     = false
}

variable "reset_instances" {
  description = "Instances to wipe and re-initialize (volumes + S3 buckets destroyed)"
  type        = list(string)
  default     = []
}

# Git branch for site content (git-ingest)
variable "git_ref" {
  description = "Git branch/ref for staging site content (production always uses main)"
  default     = "main"
}

# Cloud host IP for production site deploy (rsync target)
variable "cloud_ip" {
  description = "IP address of the cloud (Linode) host serving casparwater.us"
  default     = "173.255.212.226"
}
