resource "bitbucket_default_reviewer" "lead" {
  repository_slug = bitbucket_repository.api.slug
  user            = "jcardenas"
}
