resource "bitbucket_pipeline_config" "api" {
  repository_slug = bitbucket_repository.api.slug
  enabled         = true
}
