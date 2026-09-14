terraform {
  required_version = ">= 1.16.0, < 2.0.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.64"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = local.tags
  }
}

# Authenticates with CLOUDFLARE_API_TOKEN scoped to the mol.la zone: Zone Read,
# DNS Edit, Zone Settings Edit, Transform Rules Edit, Zone WAF Edit.
provider "cloudflare" {}
