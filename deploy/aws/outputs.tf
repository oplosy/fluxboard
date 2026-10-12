output "db_endpoint" {
  description = "host:port for DATABASE_URL / DATABASE_URL_MIGRATE (use sslmode=require or verify-full)."
  value       = aws_db_instance.db.endpoint
}

output "db_instance_id" {
  description = "RDS instance identifier (point-in-time restores start from it, see docs/runbooks/restore.md)."
  value       = aws_db_instance.db.identifier
}

output "db_master_secret_arn" {
  description = "Secrets Manager secret with the bootstrap master password (run 01-roles.sql with it once)."
  value       = aws_db_instance.db.master_user_secret[0].secret_arn
}

output "objects_bucket" {
  description = "MINIO_BUCKET for the api/worker (MINIO_ENDPOINT=s3.<region>.amazonaws.com, MINIO_USE_SSL=true)."
  value       = aws_s3_bucket.objects.bucket
}

output "app_objects_policy_arn" {
  description = "IAM policy to attach to the api/worker identity."
  value       = aws_iam_policy.app_objects.arn
}

output "backup_vault" {
  description = "AWS Backup vault holding the daily database and bucket copies."
  value       = aws_backup_vault.main.name
}
