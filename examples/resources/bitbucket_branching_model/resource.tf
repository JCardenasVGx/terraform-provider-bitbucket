resource "bitbucket_branching_model" "api" {
  repository_slug            = bitbucket_repository.api.slug
  development_use_mainbranch = true
  production_enabled         = true
  production_branch          = "production"
}
