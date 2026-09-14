# GitHub Actions authenticates to AWS via OIDC: no long-lived access keys are
# stored in GitHub. Two roles: molla-gha-plan (read-only, runs on every PR
# touching infra/terraform/aws) and molla-gha-apply (full workload write,
# runs only on a v* tag push or a manual workflow_dispatch, both gated by the
# github_environment so a human approval is required in GitHub before the
# role can be assumed).

data "tls_certificate" "github" {
  url = "https://token.actions.githubusercontent.com/.well-known/openid-configuration"
}

resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.github.certificates[0].sha1_fingerprint]
  tags            = var.tags
}

locals {
  # workflow_dispatch and a tag push into an environment-gated job both
  # present sub as repo:<repo>:environment:<name>; there is no ref-pattern
  # form for workflow_dispatch, so the environment claim is the only
  # condition that covers both triggers.
  oidc_sub_apply = "repo:${var.github_repository}:environment:${var.github_environment}"
  # Separate, unprotected environment for the plan role: it has no required
  # reviewer, so terraform-plan stays a fully automatic PR check instead of
  # blocking on the same approval as a deploy.
  oidc_sub_plan = "repo:${var.github_repository}:environment:${var.github_plan_environment}"

  assume_via_oidc_apply = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = local.oidc_sub_apply
        }
      }
    }]
  })

  assume_via_oidc_plan = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = local.oidc_sub_plan
        }
      }
    }]
  })
}

resource "aws_iam_role" "plan" {
  name               = "${var.name_prefix}-gha-plan"
  assume_role_policy = local.assume_via_oidc_plan
  tags               = var.tags
}

# AWS-managed, read-only: terraform plan touches every resource type the
# workload manages (VPC, DynamoDB, ElastiCache, Lambda, API Gateway,
# CloudFront, WAF-adjacent, KMS, S3, Kinesis, IAM, SSM, budgets, Cost
# Explorer...). Hand-scoping read-only Describe/List/Get permissions across
# that surface is easy to get subtly wrong -- ReadOnlyAccess is the standard,
# safe choice for a plan-only role that can never mutate anything.
resource "aws_iam_role_policy_attachment" "plan_read_only" {
  role       = aws_iam_role.plan.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

resource "aws_iam_role" "apply" {
  name               = "${var.name_prefix}-gha-apply"
  assume_role_policy = local.assume_via_oidc_apply
  tags               = var.tags
}

# Deliberately broad (PowerUserAccess minus IAM) rather than a hand-scoped
# policy: this role applies the full Terraform configuration, which creates
# and modifies IAM roles/policies for the Lambdas themselves (modules/api's
# aws_iam_role.api/redirect/invalidate/aggregate/admin). A tightly scoped
# policy would need constant upkeep as the configuration grows and would
# still need iam:* to manage those roles, which is most of the risk
# PowerUserAccess already excludes. The role itself is reachable only via
# OIDC from this repo's production environment, which is the actual
# blast-radius control.
resource "aws_iam_role_policy_attachment" "apply_power_user" {
  role       = aws_iam_role.apply.name
  policy_arn = "arn:aws:iam::aws:policy/PowerUserAccess"
}

resource "aws_iam_role_policy" "apply_iam" {
  name = "iam-for-workload-roles"
  role = aws_iam_role.apply.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "ManageWorkloadRoles"
        Effect = "Allow"
        Action = [
          "iam:CreateRole",
          "iam:DeleteRole",
          "iam:GetRole",
          "iam:TagRole",
          "iam:UntagRole",
          "iam:PutRolePolicy",
          "iam:DeleteRolePolicy",
          "iam:GetRolePolicy",
          "iam:AttachRolePolicy",
          "iam:DetachRolePolicy",
          "iam:PassRole",
          "iam:ListRolePolicies",
          "iam:ListAttachedRolePolicies",
          "iam:ListRoleTags",
          "iam:ListInstanceProfilesForRole",
          "iam:UpdateRole",
          "iam:UpdateRoleDescription",
          "iam:UpdateAssumeRolePolicy",
        ]
        Resource = "arn:aws:iam::*:role/${var.name_prefix}-*"
      },
      {
        # The OIDC provider is not a role, so the statement above doesn't
        # cover it; without this the apply role can't even refresh it.
        Sid    = "ManageGitHubOIDCProvider"
        Effect = "Allow"
        Action = [
          "iam:GetOpenIDConnectProvider",
          "iam:CreateOpenIDConnectProvider",
          "iam:DeleteOpenIDConnectProvider",
          "iam:UpdateOpenIDConnectProviderThumbprint",
          "iam:TagOpenIDConnectProvider",
          "iam:UntagOpenIDConnectProvider",
          "iam:AddClientIDToOpenIDConnectProvider",
          "iam:RemoveClientIDFromOpenIDConnectProvider",
        ]
        Resource = "arn:aws:iam::*:oidc-provider/token.actions.githubusercontent.com"
      },
      {
        Sid      = "ServiceLinkedRoles"
        Effect   = "Allow"
        Action   = ["iam:CreateServiceLinkedRole"]
        Resource = "*"
      }
    ]
  })
}

# ReadOnlyAccess carries no kms:Decrypt, and the provider reads SSM
# SecureString parameters with decryption, so refreshing them during plan
# needs this on the workload key.
resource "aws_iam_role_policy" "plan_kms" {
  name = "decrypt-workload-key"
  role = aws_iam_role.plan.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["kms:Decrypt", "kms:DescribeKey"]
      Resource = var.kms_key_arn
    }]
  })
}
