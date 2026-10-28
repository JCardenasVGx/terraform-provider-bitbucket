package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type mockRunner struct {
	UUID   string   `json:"uuid,omitempty"`
	Name   string   `json:"name,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Oauth  *struct {
		ID     string `json:"id,omitempty"`
		Secret string `json:"secret,omitempty"`
	} `json:"oauth_client,omitempty"`
}

// newMockRunnerServer serves both workspace- and repository-scoped runner
// endpoints under the given basePath, e.g. "/workspaces/acc-test-workspace"
// or "/repositories/acc-test-workspace/acc-repo".
func newMockRunnerServer(t *testing.T, basePath string) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	store := map[string]mockRunner{}
	nextID := 1

	mux := http.NewServeMux()
	mux.HandleFunc(basePath+"/pipelines-config/runners", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name   string   `json:"name"`
			Labels []string `json:"labels,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		uuid := "runner-" + strconv.Itoa(nextID)
		nextID++
		rn := mockRunner{UUID: uuid, Name: req.Name, Labels: req.Labels}
		rn.Oauth = &struct {
			ID     string `json:"id,omitempty"`
			Secret string `json:"secret,omitempty"`
		}{ID: "client-" + uuid, Secret: "secret-" + uuid}
		store[uuid] = rn
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(rn)
	})
	mux.HandleFunc(basePath+"/pipelines-config/runners/", func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimPrefix(r.URL.Path, basePath+"/pipelines-config/runners/")

		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			rn, ok := store[uuid]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			// The OAuth secret is only ever returned once, at creation.
			out := rn
			if out.Oauth != nil {
				out.Oauth = &struct {
					ID     string `json:"id,omitempty"`
					Secret string `json:"secret,omitempty"`
				}{ID: out.Oauth.ID}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case http.MethodPut:
			var req struct {
				Name   string   `json:"name"`
				Labels []string `json:"labels,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			rn := store[uuid]
			rn.Name = req.Name
			rn.Labels = req.Labels
			store[uuid] = rn
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rn)
		case http.MethodDelete:
			mu.Lock()
			delete(store, uuid)
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

func TestAccRunnerResource(t *testing.T) {
	srv := newMockRunnerServer(t, "/workspaces/acc-test-workspace")
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_runner" "test" {
  name   = "shared-runner"
  labels = ["self.hosted", "linux"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_runner.test", "name", "shared-runner"),
					resource.TestCheckResourceAttr("bitbucket_runner.test", "labels.#", "2"),
					resource.TestCheckResourceAttrSet("bitbucket_runner.test", "oauth_client_id"),
					resource.TestCheckResourceAttrSet("bitbucket_runner.test", "oauth_client_secret"),
				),
			},
			{
				ResourceName:            "bitbucket_runner.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"oauth_client_secret"},
				ImportStateIdFunc:       importStateIDFromAttrs("bitbucket_runner.test", []string{"runner_uuid"}, ""),
			},
		},
	})
}
