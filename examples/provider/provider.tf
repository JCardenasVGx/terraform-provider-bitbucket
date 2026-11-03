terraform {
  required_providers {
    bitbucket = {
      source  = "registry.terraform.io/JCardenasVGx/bitbucket"
      version = "~> 1.0"
    }
  }
}

# workspace, username and password can also be set via the BITBUCKET_WORKSPACE,
# BITBUCKET_USERNAME and BITBUCKET_PASSWORD environment variables instead of
# hardcoding them here.
provider "bitbucket" {
  workspace = "mi-workspace"
}

# Alternative: authenticate with a Repository/Project/Workspace Access
# Token instead of username+password (sent as "Authorization: Bearer").
# bitbucket_group and bitbucket_group_membership still need username and
# password, since Bitbucket's legacy v1.0 groups API doesn't accept Access
# Tokens — set both if you use those resources.
#
# provider "bitbucket" {
#   workspace = "mi-workspace"
#   token     = var.bitbucket_access_token # or BITBUCKET_TOKEN
# }
