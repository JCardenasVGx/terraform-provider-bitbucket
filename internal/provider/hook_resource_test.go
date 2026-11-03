package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func newMockHookServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	type hook struct {
		UUID        string   `json:"uuid,omitempty"`
		URL         string   `json:"url"`
		Description string   `json:"description,omitempty"`
		Active      bool     `json:"active"`
		Events      []string `json:"events"`
	}
	var stored *hook

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/hooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body hook
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		body.UUID = "hook-1"
		stored = &body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)
	})
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/hooks/hook-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			if stored == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(stored)
		case http.MethodPut:
			var body hook
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			body.UUID = "hook-1"
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

func TestAccHookResource(t *testing.T) {
	srv := newMockHookServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_hook" "test" {
  repository_slug = "acc-repo"
  url              = "https://example.com/hook"
  active           = true
  events           = ["repo:push"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_hook.test", "url", "https://example.com/hook"),
					resource.TestCheckResourceAttr("bitbucket_hook.test", "events.0", "repo:push"),
					resource.TestCheckResourceAttr("bitbucket_hook.test", "id", "acc-test-workspace/acc-repo/hook-1"),
				),
			},
			{
				ResourceName:      "bitbucket_hook.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "acc-repo/hook-1",
			},
			{
				Config: `
resource "bitbucket_hook" "test" {
  repository_slug = "acc-repo"
  url              = "https://example.com/hook"
  active           = false
  events           = ["repo:push", "pullrequest:created"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_hook.test", "active", "false"),
					resource.TestCheckResourceAttr("bitbucket_hook.test", "events.1", "pullrequest:created"),
				),
			},
		},
	})
}
