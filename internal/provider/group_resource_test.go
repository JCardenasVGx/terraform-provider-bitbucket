package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockGroupServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	type group struct {
		Name       string `json:"name"`
		Slug       string `json:"slug,omitempty"`
		Permission string `json:"permission,omitempty"`
		AutoAdd    bool   `json:"auto_add,omitempty"`
	}
	var stored *group

	mux := http.NewServeMux()
	mux.HandleFunc("/groups/acc-test-workspace/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			if stored == nil {
				_ = json.NewEncoder(w).Encode([]group{})
				return
			}
			_ = json.NewEncoder(w).Encode([]group{*stored})
		case http.MethodPost:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			stored = &group{Name: body["name"], Slug: "release-managers"}
			g := *stored
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(g)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/groups/acc-test-workspace/release-managers/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			var body group
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			body.Slug = "release-managers"
			stored = &body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case http.MethodDelete:
			mu.Lock()
			stored = nil
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

func TestAccGroupResource(t *testing.T) {
	srv := newMockGroupServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_group" "test" {
  name = "Release Managers"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_group.test", "name", "Release Managers"),
					resource.TestCheckResourceAttr("bitbucket_group.test", "slug", "release-managers"),
				),
			},
			{
				ResourceName:      "bitbucket_group.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "release-managers",
			},
			{
				Config: `
resource "bitbucket_group" "test" {
  name       = "Release Managers"
  permission = "write"
}
`,
				Check: resource.TestCheckResourceAttr("bitbucket_group.test", "permission", "write"),
			},
		},
	})
}
