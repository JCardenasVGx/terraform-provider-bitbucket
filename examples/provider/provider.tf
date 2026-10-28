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
