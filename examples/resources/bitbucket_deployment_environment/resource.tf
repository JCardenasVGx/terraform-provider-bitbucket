resource "bitbucket_deployment_environment" "production" {
  repository_slug  = bitbucket_repository.api.slug
  name             = "production"
  environment_type = "Production"
  admin_only       = true
}
