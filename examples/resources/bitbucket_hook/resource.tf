resource "bitbucket_hook" "ci_notify" {
  repository_slug = bitbucket_repository.api.slug
  url             = "https://ci.example.com/bitbucket-webhook"
  active          = true
  events          = ["repo:push", "pullrequest:created", "pullrequest:fulfilled"]
}
