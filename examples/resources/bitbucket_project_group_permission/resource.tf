resource "bitbucket_project_group_permission" "release_managers" {
  project_key = bitbucket_project.platform.key
  group_slug  = "release-managers"
  permission  = "write"
}
