resource "bitbucket_project_default_reviewer" "lead" {
  project_key = bitbucket_project.platform.key
  user        = "jcardenas"
}
