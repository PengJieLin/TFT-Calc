terraform {
  backend "s3" {
    bucket         = "tft-terraform-state-storage" # The bucket you just created
    key            = "tft-optimizer/terraform.tfstate"
    region         = "us-east-2"
    encrypt        = true
    # dynamodb_table = "terraform-lock" (Prevents two people from applying at once)
  }
}

resource "aws_s3_bucket" "data_bucket" {
  # Note: Bucket names must be globally unique across all of AWS!
  bucket = var.bucket
  force_destroy = true
}

# 1. The IAM Role (The 'Identity' of your Go App)
resource "aws_iam_role" "lambda_role" {
  name = "tft_optimizer_lambda_role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })
}

# 2. The Permission Policy (Allowing the Role to read S3)
resource "aws_iam_role_policy" "s3_read_policy" {
  name = "tft_s3_read_policy"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # 1. Permission to read the actual files
        Effect   = "Allow"
        Action   = ["s3:GetObject"]
        Resource = "${aws_s3_bucket.data_bucket.arn}/*"
      },
      {
        # 2. Permission to "look" inside the bucket (Fixes your 403)
        Effect   = "Allow"
        Action   = ["s3:ListBucket"]
        Resource = "${aws_s3_bucket.data_bucket.arn}" # Note: No /* here!
      }
    ]
  })
}

# 3. The Lambda Function itself
resource "aws_lambda_function" "tft_optimizer" {
  function_name = "tft-optimizer-lambda"
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"            # Required name for Go on AL2023
  runtime       = "provided.al2023"      # The modern Go runtime
  filename      = "deployment.zip"       # We will create this in the next step

  source_code_hash = filebase64sha256("deployment.zip")

  timeout          = 30

  # This connects your Go code's os.Getenv("DATA_BUCKET") to the actual bucket
  environment {
    variables = {
      DATA_BUCKET = aws_s3_bucket.data_bucket.id
    }
  }
}