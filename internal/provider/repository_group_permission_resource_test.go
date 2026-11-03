package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockRepositoryGroupPermissionServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	var permission string
	granted := false

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/permissions-config/groups/release-managers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			if !granted {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"permission": permission, "group": map[string]string{"slug": "release-managers"}})
		case http.MethodPut:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			permission = body["permission"]
			granted = true
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"permission": permission, "group": map[string]string{"slug": "release-managers"}})
		case http.MethodDelete:
			mu.Lock()
			granted = false
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

func TestAccRepositoryGroupPermissionResource(t *testing.T) {
	srv := newMockRepositoryGroupPermissionServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_repository_group_permission" "test" {
  repository_slug = "acc-repo"
  group_slug       = "release-managers"
  permission       = "write"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_repository_group_permission.test", "permission", "write"),
					resource.TestCheckResourceAttr("bitbucket_repository_group_permission.test", "id", "acc-test-workspace/acc-repo/release-managers"),
				),
			},
			{
				ResourceName:      "bitbucket_repository_group_permission.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "acc-repo/release-managers",
			},
			{
				Config: `
resource "bitbucket_repository_group_permission" "test" {
  repository_slug = "acc-repo"
  group_slug       = "release-managers"
  permission       = "admin"
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_repository_group_permission.test", "permission", "admin"),
			},
		},
	})
}
