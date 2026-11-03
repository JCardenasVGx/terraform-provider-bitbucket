package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockBranchingModelServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	state := map[string]any{
		"development": map[string]any{"name": "", "use_mainbranch": true},
		"production":  map[string]any{"enabled": false, "name": "", "use_mainbranch": false},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/branching-model/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(state)
		case http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			state = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAccBranchingModelResource(t *testing.T) {
	srv := newMockBranchingModelServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_branching_model" "test" {
  repository_slug            = "acc-repo"
  development_use_mainbranch = true
  production_enabled         = true
  production_branch          = "production"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_branching_model.test", "production_enabled", "true"),
					resource.TestCheckResourceAttr("bitbucket_branching_model.test", "production_branch", "production"),
					resource.TestCheckResourceAttr("bitbucket_branching_model.test", "id", "acc-test-workspace/acc-repo"),
				),
			},
		},
	})
}
