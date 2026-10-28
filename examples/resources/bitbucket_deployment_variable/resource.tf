resource "bitbucket_deployment_variable" "prod_db_url" {
  repository_slug  = bitbucket_repository.api.slug
  environment_uuid = bitbucket_deployment_environment.production.environment_uuid
  key              = "DB_URL"
  value            = "postgres://prod-example"
  secured          = true
}
