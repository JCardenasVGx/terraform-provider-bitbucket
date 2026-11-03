resource "bitbucket_project_branching_model" "platform" {
  project_key                = bitbucket_project.platform.key
  development_use_mainbranch = true
  production_enabled         = true
  production_branch          = "production"
}
