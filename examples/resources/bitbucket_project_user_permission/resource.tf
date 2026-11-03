resource "bitbucket_project_user_permission" "owner" {
  project_key = bitbucket_project.platform.key
  user        = "jcardenas"
  permission  = "admin"
}
