variable "region" {
  description = "AWS region for the database, bucket and backups. Must match what the privacy page promises about data location (report B09)."
  type        = string
}

variable "environment" {
  description = "Environment name, used in resource names (e.g. prod, staging)."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,15}$", var.environment))
    error_message = "environment must be 2-16 lowercase letters, digits or dashes."
  }
}

variable "vpc_id" {
  description = "VPC the database lives in."
  type        = string
}

variable "db_subnet_ids" {
  description = "Private subnets (at least two AZs) for the database subnet group."
  type        = list(string)

  validation {
    condition     = length(var.db_subnet_ids) >= 2
    error_message = "RDS needs subnets in at least two availability zones."
  }
}

variable "app_security_group_ids" {
  description = "Security groups of the api/worker/migration runners; only these may reach Postgres."
  type        = list(string)

  validation {
    condition     = length(var.app_security_group_ids) > 0
    error_message = "At least one app security group must be allowed to reach the database."
  }
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.medium"
}

variable "db_allocated_storage_gb" {
  description = "Initial storage; autoscaling grows it up to db_max_allocated_storage_gb."
  type        = number
  default     = 50
}

variable "db_max_allocated_storage_gb" {
  description = "Storage autoscaling ceiling."
  type        = number
  default     = 200
}

variable "db_multi_az" {
  description = "Synchronous standby in a second AZ (faster failover, double cost)."
  type        = bool
  default     = true
}

variable "pitr_retention_days" {
  description = "RDS automated backup retention = the point-in-time recovery window."
  type        = number
  default     = 14

  validation {
    condition     = var.pitr_retention_days >= 7 && var.pitr_retention_days <= 35
    error_message = "pitr_retention_days must be 7-35 (RDS maximum is 35)."
  }
}

variable "backup_vault_retention_days" {
  description = "How long the daily AWS Backup copies of the database and bucket are kept in the locked vault."
  type        = number
  default     = 35

  validation {
    condition     = var.backup_vault_retention_days >= 7
    error_message = "backup_vault_retention_days must be at least 7."
  }
}

variable "noncurrent_version_retention_days" {
  description = "Days an overwritten or deleted object version stays recoverable in the bucket."
  type        = number
  default     = 30

  validation {
    condition     = var.noncurrent_version_retention_days >= 1
    error_message = "noncurrent_version_retention_days must be at least 1."
  }
}

variable "web_origins" {
  description = "Web origins allowed to PUT/GET presigned URLs (bucket CORS), e.g. [\"https://app.example.com\"]."
  type        = list(string)

  validation {
    condition     = length(var.web_origins) > 0 && alltrue([for o in var.web_origins : startswith(o, "https://")])
    error_message = "web_origins must be non-empty https origins."
  }
}
