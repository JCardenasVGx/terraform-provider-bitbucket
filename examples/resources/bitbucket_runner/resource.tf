resource "bitbucket_runner" "shared" {
  name   = "example-shared-runner"
  labels = ["self.hosted", "linux"]
}

# The runner is only registered in Bitbucket here. Installing and running the
# actual runner daemon/container with these credentials happens outside of
# Terraform (e.g. via the Docker image Bitbucket provides).
output "shared_runner_oauth_client_id" {
  value = bitbucket_runner.shared.oauth_client_id
}

output "shared_runner_oauth_client_secret" {
  value     = bitbucket_runner.shared.oauth_client_secret
  sensitive = true
}
