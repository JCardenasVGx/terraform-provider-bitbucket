resource "bitbucket_branch_restriction" "require_approvals" {
  repository_slug   = bitbucket_repository.api.slug
  kind              = "require_approvals_to_merge"
  branch_match_kind = "glob"
  pattern           = "main"
  value             = 2
}

resource "bitbucket_branch_restriction" "no_force_push" {
  repository_slug   = bitbucket_repository.api.slug
  kind              = "force"
  branch_match_kind = "glob"
  pattern           = "main"
}

# Block direct pushes to main except for the "release-managers" group.
resource "bitbucket_branch_restriction" "restrict_push" {
  repository_slug   = bitbucket_repository.api.slug
  kind              = "push"
  branch_match_kind = "glob"
  pattern           = "main"
  groups            = ["release-managers"]
}
