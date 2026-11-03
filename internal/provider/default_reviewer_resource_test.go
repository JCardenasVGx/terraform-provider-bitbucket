package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockDefaultReviewerServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	exists := false

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/default-reviewers/jcardenas", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "{u}", "display_name": "Jesus Cardenas"})
		case http.MethodPut:
			mu.Lock()
			exists = true
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "{u}", "display_name": "Jesus Cardenas"})
		case http.MethodDelete:
			mu.Lock()
			exists = false
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAccDefaultReviewerResource(t *testing.T) {
	srv := newMockDefaultReviewerServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_default_reviewer" "test" {
  repository_slug = "acc-repo"
  user             = "jcardenas"
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_default_reviewer.test", "id", "acc-test-workspace/acc-repo/jcardenas"),
			},
			{
				ResourceName:      "bitbucket_default_reviewer.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "acc-repo/jcardenas",
			},
		},
	})
}
