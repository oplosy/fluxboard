# PostgreSQL 16 on RDS (report B07): automated backups give point-in-time
# recovery over pitr_retention_days (RDS ships WAL every ~5 minutes, inside the
# RPO <= 15 min target); the master password lives in Secrets Manager, never in
# Terraform state; the instance is private, encrypted and TLS-only.

locals {
  name = "fluxboard-${var.environment}"
}

resource "aws_db_subnet_group" "db" {
  name       = "${local.name}-db"
  subnet_ids = var.db_subnet_ids
}

resource "aws_security_group" "db" {
  name        = "${local.name}-db"
  description = "Postgres, reachable only from the app security groups"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "db_from_app" {
  for_each = toset(var.app_security_group_ids)

  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = each.value
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "Postgres from app"
}

resource "aws_db_parameter_group" "db" {
  name   = "${local.name}-pg16"
  family = "postgres16"

  # Reject non-TLS connections; app DSNs need sslmode=require (or verify-full).
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_db_instance" "db" {
  identifier     = "${local.name}-db"
  engine         = "postgres"
  engine_version = "16"
  instance_class = var.db_instance_class
  db_name        = "fluxboard"

  # Bootstrap user only: it runs deploy/postgres/init/01-roles.sql once; the
  # app connects as fluxboard_owner (migrations) and fluxboard_app (RLS).
  username                    = "fluxboard_master"
  manage_master_user_password = true

  allocated_storage     = var.db_allocated_storage_gb
  max_allocated_storage = var.db_max_allocated_storage_gb
  storage_type          = "gp3"
  storage_encrypted     = true

  multi_az               = var.db_multi_az
  db_subnet_group_name   = aws_db_subnet_group.db.name
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  parameter_group_name   = aws_db_parameter_group.db.name

  backup_retention_period  = var.pitr_retention_days
  backup_window            = "02:00-03:00"
  maintenance_window       = "sun:03:30-sun:04:30"
  copy_tags_to_snapshot    = true
  delete_automated_backups = false

  auto_minor_version_upgrade = true
  deletion_protection        = true
  skip_final_snapshot        = false
  final_snapshot_identifier  = "${local.name}-db-final"

  performance_insights_enabled    = true
  enabled_cloudwatch_logs_exports = ["postgresql"]
}
