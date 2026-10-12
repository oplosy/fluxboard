# Backups outside the application's reach (report B07): AWS Backup copies the
# database and the bucket daily into a vault with its own KMS key and a vault
# lock, so app credentials (or someone holding them) cannot delete them. This
# is in addition to RDS PITR (database) and bucket versioning (objects).

resource "aws_kms_key" "backup" {
  description             = "${local.name} backup vault"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}

resource "aws_backup_vault" "main" {
  name        = "${local.name}-vault"
  kms_key_arn = aws_kms_key.backup.arn
}

# Governance-mode lock: recovery points cannot be deleted or shortened below
# min_retention_days except by principals with backup:DeleteBackupVaultLockConfiguration.
# (Compliance mode, set via changeable_for_days, becomes irreversible; decide
# that deliberately per account.)
resource "aws_backup_vault_lock_configuration" "main" {
  backup_vault_name  = aws_backup_vault.main.name
  min_retention_days = 7
  max_retention_days = max(var.backup_vault_retention_days, 7)
}

resource "aws_backup_plan" "daily" {
  name = "${local.name}-daily"

  rule {
    rule_name         = "daily"
    target_vault_name = aws_backup_vault.main.name
    schedule          = "cron(0 3 * * ? *)" # 03:00 UTC, after the RDS backup window
    start_window      = 60
    completion_window = 360

    lifecycle {
      delete_after = var.backup_vault_retention_days
    }
  }
}

data "aws_iam_policy_document" "backup_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["backup.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "backup" {
  name               = "${local.name}-backup"
  assume_role_policy = data.aws_iam_policy_document.backup_assume.json
}

resource "aws_iam_role_policy_attachment" "backup" {
  for_each = toset([
    "arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup",
    "arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForRestores",
    "arn:aws:iam::aws:policy/AWSBackupServiceRolePolicyForS3Backup",
    "arn:aws:iam::aws:policy/AWSBackupServiceRolePolicyForS3Restore",
  ])

  role       = aws_iam_role.backup.name
  policy_arn = each.value
}

resource "aws_backup_selection" "data" {
  name         = "${local.name}-data"
  plan_id      = aws_backup_plan.daily.id
  iam_role_arn = aws_iam_role.backup.arn

  resources = [
    aws_db_instance.db.arn,
    aws_s3_bucket.objects.arn,
  ]

  # S3 backups require versioning on the bucket.
  depends_on = [aws_s3_bucket_versioning.objects]
}
