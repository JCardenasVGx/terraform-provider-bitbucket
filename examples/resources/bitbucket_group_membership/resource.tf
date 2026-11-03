resource "bitbucket_group_membership" "jcardenas" {
  group_slug = bitbucket_group.release_managers.slug
  user       = "jcardenas"
}
