# Plan-only checks of the input guards, against a mocked AWS provider (no
# account needed): terraform init -backend=false && terraform test

mock_provider "aws" {
  mock_data "aws_iam_policy_document" {
    defaults = {
      json = "{\"Version\":\"2012-10-17\",\"Statement\":[]}"
    }
  }
}

variables {
  region                 = "eu-central-1"
  environment            = "prod"
  vpc_id                 = "vpc-0123456789abcdef0"
  db_subnet_ids          = ["subnet-0aaaaaaaaaaaaaaaa", "subnet-0bbbbbbbbbbbbbbbb"]
  app_security_group_ids = ["sg-0cccccccccccccccc"]
  web_origins            = ["https://app.example.com"]
}

run "defaults_plan" {
  command = plan

  assert {
    condition     = aws_db_instance.db.backup_retention_period == 14 && aws_db_instance.db.deletion_protection && !aws_db_instance.db.publicly_accessible
    error_message = "the database must keep a 14-day PITR window, deletion protection and no public access by default"
  }
  assert {
    condition     = aws_db_instance.db.manage_master_user_password && aws_db_instance.db.storage_encrypted
    error_message = "the master password must live in Secrets Manager and storage must be encrypted"
  }
}

run "object_versions_shorter_than_pitr" {
  command = plan
  variables {
    pitr_retention_days               = 21
    noncurrent_version_retention_days = 14
  }
  expect_failures = [aws_s3_bucket_lifecycle_configuration.objects]
}

run "pitr_window_out_of_range" {
  command = plan
  variables {
    pitr_retention_days = 3
  }
  expect_failures = [var.pitr_retention_days]
}

run "plain_http_origin" {
  command = plan
  variables {
    web_origins = ["http://app.example.com"]
  }
  expect_failures = [var.web_origins]
}

run "single_subnet" {
  command = plan
  variables {
    db_subnet_ids = ["subnet-0aaaaaaaaaaaaaaaa"]
  }
  expect_failures = [var.db_subnet_ids]
}
