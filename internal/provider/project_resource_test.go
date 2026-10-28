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

// newMockProjectServer is a tiny stateful fake of the Bitbucket Cloud
// projects API, just enough to drive a full Create/Read/Update/Delete
// acceptance test cycle without touching production.
func newMockProjectServer(t *testing.T) *httptest.Server {
	t.Helper()

	type project struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		IsPrivate   bool   `json:"is_private"`
	}

	var mu sync.Mutex
	store := map[string]project{}

	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/acc-test-workspace/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var p project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		store[p.Key] = p
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(p)
	})
	mux.HandleFunc("/workspaces/acc-test-workspace/projects/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/workspaces/acc-test-workspace/projects/"):]

		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			p, ok := store[key]
			mu.Unlock()
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type": "error", "error": map[string]string{"message": "not found"},
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(p)
		case http.MethodPut:
			var p project
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			store[key] = p
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(p)
		case http.MethodDelete:
			mu.Lock()
			delete(store, key)
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

func TestAccProjectResource(t *testing.T) {
	srv := newMockProjectServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_project" "test" {
  key         = "ACC"
  name        = "Acceptance Test Project"
  description = "created by acceptance test"
  is_private  = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_project.test", "key", "ACC"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "name", "Acceptance Test Project"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "description", "created by acceptance test"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "is_private", "true"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "id", "acc-test-workspace/ACC"),
				),
			},
			{
				ResourceName:      "bitbucket_project.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "ACC",
			},
			{
				Config: `
resource "bitbucket_project" "test" {
  key         = "ACC"
  name        = "Renamed Project"
  description = "updated by acceptance test"
  is_private  = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_project.test", "name", "Renamed Project"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "description", "updated by acceptance test"),
					resource.TestCheckResourceAttr("bitbucket_project.test", "is_private", "false"),
				),
			},
		},
		CheckDestroy: func(s *terraform.State) error {
			resp, err := http.Get(srv.URL + "/workspaces/acc-test-workspace/projects/ACC")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				return fmt.Errorf("expected project ACC to be deleted, mock server returned status %d", resp.StatusCode)
			}
			return nil
		},
	})
}
