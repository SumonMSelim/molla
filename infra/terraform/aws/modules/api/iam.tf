locals {
  lambda_assume = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role" "api" {
  name               = "${var.name_prefix}-api"
  assume_role_policy = local.lambda_assume
  tags               = var.tags
}

resource "aws_iam_role" "redirect" {
  name               = "${var.name_prefix}-redirect"
  assume_role_policy = local.lambda_assume
  tags               = var.tags
}

resource "aws_iam_role" "invalidate" {
  name               = "${var.name_prefix}-invalidate"
  assume_role_policy = local.lambda_assume
  tags               = var.tags
}

resource "aws_iam_role" "admin" {
  name = "${var.name_prefix}-admin"
  tags = var.tags
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { AWS = var.admin_principal_arns }
    }]
  })
}

resource "aws_iam_role_policy" "api" {
  name = "api"
  role = aws_iam_role.api.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.api.arn}:*"
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:TransactWriteItems",
        ]
        Resource = [
          var.table_arns["links"],
          var.table_arns["idempotency"],
          var.table_arns["counters"],
          var.table_arns["credentials"],
          var.table_arns["stats"],
        ]
      },
      {
        Sid      = "InvokeInvalidation"
        Effect   = "Allow"
        Action   = ["lambda:InvokeFunction"]
        Resource = aws_lambda_function.invalidate.arn
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = var.kms_key_arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "redirect" {
  name = "redirect"
  role = aws_iam_role.redirect.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.redirect.arn}:*"
      },
      {
        Effect   = "Allow"
        Action   = ["dynamodb:GetItem"]
        Resource = var.table_arns["links"]
      },
      {
        Effect   = "Allow"
        Action   = ["kinesis:PutRecord"]
        Resource = var.stream_arn
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:CreateNetworkInterface",
          "ec2:DescribeNetworkInterfaces",
          "ec2:DeleteNetworkInterface",
          "ec2:AssignPrivateIpAddresses",
          "ec2:UnassignPrivateIpAddresses",
        ]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = var.kms_key_arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "invalidate" {
  name = "invalidate"
  role = aws_iam_role.invalidate.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.invalidate.arn}:*"
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:CreateNetworkInterface",
          "ec2:DescribeNetworkInterfaces",
          "ec2:DeleteNetworkInterface",
        ]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt"]
        Resource = var.kms_key_arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "admin" {
  name = "admin"
  role = aws_iam_role.admin.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "LinkSoftDelete"
        Effect   = "Allow"
        Action   = ["dynamodb:GetItem", "dynamodb:UpdateItem"]
        Resource = var.table_arns["links"]
      },
      {
        Sid      = "IssueCredentials"
        Effect   = "Allow"
        Action   = ["dynamodb:PutItem", "dynamodb:GetItem"]
        Resource = var.table_arns["credentials"]
      },
      {
        Sid      = "InvokeInvalidation"
        Effect   = "Allow"
        Action   = ["lambda:InvokeFunction"]
        Resource = aws_lambda_function.invalidate.arn
      },
      {
        Sid      = "CallerIdentity"
        Effect   = "Allow"
        Action   = ["sts:GetCallerIdentity"]
        Resource = "*"
      },
      {
        Sid      = "TableKMS"
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = var.kms_key_arn
      }
    ]
  })
}
