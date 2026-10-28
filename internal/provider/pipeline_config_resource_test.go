package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func newMockPipelineConfigServer(t *testing.T) (*httptest.Server, func() bool) {
	t.Helper()

	var mu sync.Mutex
	enabled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/pipelines_config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			e := enabled
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"enabled": e})
		case http.MethodPut:
			var body map[string]bool
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			enabled = body["enabled"]
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return enabled
	}
}

func TestAccPipelineConfigResource(t *testing.T) {
	srv, isEnabled := newMockPipelineConfigServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_pipeline_config" "test" {
  repository_slug = "acc-repo"
  enabled         = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_pipeline_config.test", "enabled", "true"),
					resource.TestCheckResourceAttr("bitbucket_pipeline_config.test", "id", "acc-test-workspace/acc-repo"),
				),
			},
			{
				ResourceName:      "bitbucket_pipeline_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "acc-repo",
			},
			{
				Config: `
resource "bitbucket_pipeline_config" "test" {
  repository_slug = "acc-repo"
  enabled         = false
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_pipeline_config.test", "enabled", "false"),
			},
		},
		// There is no real "delete" endpoint for pipeline config: destroying
		// this resource just disables Pipelines again.
		CheckDestroy: func(s *terraform.State) error {
			if isEnabled() {
				return fmt.Errorf("expected pipelines to be disabled after destroy")
			}
			return nil
		},
	})
}
