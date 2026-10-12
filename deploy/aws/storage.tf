# Attachment/avatar/logo bucket (the app's MINIO_BUCKET; minio-go speaks S3).
# Versioning keeps overwritten and deleted objects recoverable for
# noncurrent_version_retention_days; the bucket is private, encrypted and
# TLS-only, and the app reaches objects through presigned URLs.

resource "aws_s3_bucket" "objects" {
  bucket_prefix = "${local.name}-objects-"
}

resource "aws_s3_bucket_ownership_controls" "objects" {
  bucket = aws_s3_bucket.objects.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "objects" {
  bucket = aws_s3_bucket.objects.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "objects" {
  bucket = aws_s3_bucket.objects.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "objects" {
  bucket = aws_s3_bucket.objects.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "objects" {
  bucket = aws_s3_bucket.objects.id

  # Versioning must be on before noncurrent-version rules apply.
  depends_on = [aws_s3_bucket_versioning.objects]

  lifecycle {
    # A database restored to any point in the PITR window references objects
    # as they were then; their old versions must still exist.
    precondition {
      condition     = var.noncurrent_version_retention_days >= var.pitr_retention_days
      error_message = "noncurrent_version_retention_days must be >= pitr_retention_days, or a point-in-time restore can reference object versions that were already expired."
    }
  }

  rule {
    id     = "expire-old-versions"
    status = "Enabled"
    filter {}

    noncurrent_version_expiration {
      noncurrent_days = var.noncurrent_version_retention_days
    }
    expiration {
      expired_object_delete_marker = true
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

resource "aws_s3_bucket_cors_configuration" "objects" {
  bucket = aws_s3_bucket.objects.id

  # Browser uploads/downloads go straight to S3 through presigned URLs.
  cors_rule {
    allowed_methods = ["GET", "PUT", "HEAD"]
    allowed_origins = var.web_origins
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3000
  }
}

data "aws_iam_policy_document" "objects_tls_only" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.objects.arn,
      "${aws_s3_bucket.objects.arn}/*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "objects" {
  bucket = aws_s3_bucket.objects.id
  policy = data.aws_iam_policy_document.objects_tls_only.json

  depends_on = [aws_s3_bucket_public_access_block.objects]
}

# Least-privilege access for the api/worker identity (attach to its role or
# user): object read/write/delete in this bucket only.
data "aws_iam_policy_document" "app_objects" {
  statement {
    sid       = "ListBucket"
    actions   = ["s3:ListBucket", "s3:GetBucketLocation"]
    resources = [aws_s3_bucket.objects.arn]
  }

  statement {
    sid       = "ObjectAccess"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.objects.arn}/*"]
  }
}

resource "aws_iam_policy" "app_objects" {
  name        = "${local.name}-app-objects"
  description = "Fluxboard api/worker access to the object bucket"
  policy      = data.aws_iam_policy_document.app_objects.json
}
