terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
  }

  # State holds resource ids, not the database password (RDS keeps that in
  # Secrets Manager). Configure a remote backend (S3 + lock) per account before
  # the first apply; see README.md.
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "fluxboard"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
