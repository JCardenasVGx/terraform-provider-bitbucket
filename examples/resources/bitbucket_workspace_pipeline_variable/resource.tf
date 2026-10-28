resource "bitbucket_workspace_pipeline_variable" "shared_registry" {
  key     = "DOCKER_REGISTRY"
  value   = "registry.example.internal"
  secured = false
}
