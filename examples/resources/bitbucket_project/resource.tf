resource "bitbucket_project" "platform" {
  key         = "PLAT"
  name        = "Platform"
  description = "Repos de plataforma gestionados por IaC"
  is_private  = true
}
