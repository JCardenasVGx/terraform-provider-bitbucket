resource "bitbucket_workspace_hook" "ci_notify" {
  url    = "https://ci.example.com/bitbucket-webhook"
  active = true
  events = ["repo:push"]
}
