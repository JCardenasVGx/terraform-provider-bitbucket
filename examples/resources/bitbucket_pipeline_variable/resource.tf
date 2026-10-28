resource "bitbucket_pipeline_variable" "db_url" {
  repository_slug = bitbucket_repository.api.slug
  key             = "DB_URL"
  value           = "postgres://example"
  secured         = true
}
