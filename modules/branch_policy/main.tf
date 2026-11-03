locals {
  patterns = toset(var.branch_patterns)
}

resource "bitbucket_branch_restriction" "require_approvals" {
  for_each = var.require_approvals > 0 ? local.patterns : toset([])

  repository_slug   = var.repository_slug
  kind              = "require_approvals_to_merge"
  branch_match_kind = "glob"
  pattern           = each.value
  value             = var.require_approvals
}

resource "bitbucket_branch_restriction" "block_force_push" {
  for_each = var.block_force_push ? local.patterns : toset([])

  repository_slug   = var.repository_slug
  kind              = "force"
  branch_match_kind = "glob"
  pattern           = each.value
}

resource "bitbucket_branch_restriction" "block_delete" {
  for_each = var.block_delete ? local.patterns : toset([])

  repository_slug   = var.repository_slug
  kind              = "delete"
  branch_match_kind = "glob"
  pattern           = each.value
}

resource "bitbucket_branch_restriction" "require_passing_builds" {
  for_each = var.require_passing_builds ? local.patterns : toset([])

  repository_slug   = var.repository_slug
  kind              = "require_passing_builds_to_merge"
  branch_match_kind = "glob"
  pattern           = each.value
}

resource "bitbucket_branch_restriction" "restrict_push" {
  for_each = (length(var.restrict_push_to_groups) > 0 || length(var.restrict_push_to_users) > 0) ? local.patterns : toset([])

  repository_slug   = var.repository_slug
  kind              = "push"
  branch_match_kind = "glob"
  pattern           = each.value
  groups            = var.restrict_push_to_groups
  users             = var.restrict_push_to_users
}
