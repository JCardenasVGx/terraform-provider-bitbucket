resource "bitbucket_repository_runner" "api" {
  repository_slug = bitbucket_repository.api.slug
  name            = "payments-api-runner"
  labels          = ["self.hosted", "linux"]
}
