package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRepositoryRunnerResource(t *testing.T) {
	srv := newMockRunnerServer(t, "/repositories/acc-test-workspace/acc-repo")
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_repository_runner" "test" {
  repository_slug = "acc-repo"
  name            = "repo-runner"
  labels          = ["self.hosted"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_repository_runner.test", "name", "repo-runner"),
					resource.TestCheckResourceAttrSet("bitbucket_repository_runner.test", "oauth_client_id"),
					resource.TestCheckResourceAttrSet("bitbucket_repository_runner.test", "oauth_client_secret"),
				),
			},
			{
				ResourceName:            "bitbucket_repository_runner.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"oauth_client_secret"},
				ImportStateIdFunc:       importStateIDFromAttrs("bitbucket_repository_runner.test", []string{"repository_slug", "runner_uuid"}, "/"),
			},
		},
	})
}
