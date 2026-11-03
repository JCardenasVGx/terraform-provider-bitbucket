data "bitbucket_user" "jcardenas" {
  selected_user = "jcardenas"
}

resource "bitbucket_default_reviewer" "lead" {
  repository_slug = bitbucket_repository.api.slug
  user            = data.bitbucket_user.jcardenas.uuid
}
