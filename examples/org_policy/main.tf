terraform {
  required_providers {
    bitbucket = {
      source = "JCardenasVGx/bitbucket"
    }
  }
}

provider "bitbucket" {
  workspace = "mi-workspace"
}

resource "bitbucket_project" "platform" {
  key  = "PLAT"
  name = "Platform"
}

# Un solo lugar define qué repos existen...
locals {
  platform_repos = toset([
    "payments-api",
    "payments-worker",
    "notifications-api",
  ])
}

resource "bitbucket_repository" "platform" {
  for_each    = local.platform_repos
  slug        = each.value
  project_key = bitbucket_project.platform.key
}

# ...y la MISMA política se aplica a todos ellos de una sola vez,
# igual que un Organization Ruleset de GitHub, pero resuelto en Terraform.
module "standard_branch_policy" {
  source   = "../../modules/branch_policy"
  for_each = bitbucket_repository.platform

  repository_slug        = each.value.slug
  branch_patterns        = ["main", "release/*"]
  require_approvals      = 2
  block_force_push       = true
  block_delete           = true
  require_passing_builds = true
}

output "policy_restriction_ids" {
  value = { for slug, mod in module.standard_branch_policy : slug => mod.restriction_ids }
}
