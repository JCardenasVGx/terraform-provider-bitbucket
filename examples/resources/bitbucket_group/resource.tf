resource "bitbucket_group" "release_managers" {
  name       = "Release Managers"
  permission = "write"
  auto_add   = false
}
