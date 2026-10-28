resource "bitbucket_repository" "api" {
  slug        = "payments-api"
  name        = "payments-api"
  description = "Servicio de pagos"
  project_key = bitbucket_project.platform.key
  is_private  = true
  fork_policy = "no_forks"
  has_issues  = true
  has_wiki    = false
}
