terraform {
  required_providers {
    bitbucket = {
      source = "JCardenasVGx/bitbucket"
    }
  }
}

provider "bitbucket" {
  workspace = "mi-workspace"
  # username y password tambien se pueden setear via
  # BITBUCKET_USERNAME / BITBUCKET_PASSWORD
}

resource "bitbucket_project" "platform" {
  key         = "PLAT"
  name        = "Platform"
  description = "Repos de plataforma gestionados por IaC"
  is_private  = true
}

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

data "bitbucket_project" "platform" {
  key = bitbucket_project.platform.key
}

data "bitbucket_repository" "api" {
  slug = bitbucket_repository.api.slug
}

resource "bitbucket_pipeline_config" "api" {
  repository_slug = bitbucket_repository.api.slug
  enabled         = true
}

resource "bitbucket_pipeline_variable" "db_url" {
  repository_slug = bitbucket_repository.api.slug
  key             = "DB_URL"
  value           = "postgres://example"
  secured         = true
}

resource "bitbucket_workspace_pipeline_variable" "shared_registry" {
  key     = "DOCKER_REGISTRY"
  value   = "registry.example.internal"
  secured = false
}

resource "bitbucket_runner" "shared" {
  name   = "example-shared-runner"
  labels = ["self.hosted", "linux"]
}

resource "bitbucket_repository_runner" "api" {
  repository_slug = bitbucket_repository.api.slug
  name            = "payments-api-runner"
  labels          = ["self.hosted", "linux"]
}

resource "bitbucket_deployment_environment" "production" {
  repository_slug  = bitbucket_repository.api.slug
  name             = "production"
  environment_type = "Production"
  admin_only       = true
}

resource "bitbucket_deployment_variable" "prod_db_url" {
  repository_slug  = bitbucket_repository.api.slug
  environment_uuid = bitbucket_deployment_environment.production.environment_uuid
  key              = "DB_URL"
  value            = "postgres://prod-example"
  secured          = true
}

output "repository_id" {
  value = bitbucket_repository.api.id
}

output "runner_oauth_client_id" {
  value = bitbucket_runner.shared.oauth_client_id
}

output "runner_oauth_client_secret" {
  value     = bitbucket_runner.shared.oauth_client_secret
  sensitive = true
}
