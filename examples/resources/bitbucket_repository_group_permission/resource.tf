resource "bitbucket_repository_group_permission" "release_managers" {
  repository_slug = bitbucket_repository.api.slug
  group_slug      = "release-managers"
  permission      = "write"
}
