resource "bitbucket_repository_user_permission" "owner" {
  repository_slug = bitbucket_repository.api.slug
  user            = "jcardenas"
  permission      = "admin"
}
