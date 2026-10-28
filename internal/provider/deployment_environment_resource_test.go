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

func newMockDeploymentEnvironmentServer(t *testing.T) *httptest.Server {
	t.Helper()

	type environment struct {
		UUID            string `json:"uuid,omitempty"`
		Name            string `json:"name,omitempty"`
		EnvironmentType *struct {
			Name string `json:"name"`
		} `json:"environment_type,omitempty"`
		Restrictions *struct {
			AdminOnly bool `json:"admin_only"`
		} `json:"restrictions,omitempty"`
	}

	var mu sync.Mutex
	store := map[string]environment{}
	nextID := 1

	mux := http.NewServeMux()
	mux.HandleFunc("/repositories/acc-test-workspace/acc-repo/environments/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/repositories/acc-test-workspace/acc-repo/environments/")

		if rest == "" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var body environment
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			body.UUID = "env-" + strconv.Itoa(nextID)
			nextID++
			store[body.UUID] = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(body)
			return
		}

		if strings.HasSuffix(rest, "/changes") || strings.HasSuffix(rest, "/changes/") {
			uuid := strings.TrimSuffix(strings.TrimSuffix(rest, "/"), "/changes")
			var body struct {
				Change struct {
					Name         string `json:"name,omitempty"`
					Restrictions *struct {
						AdminOnly bool `json:"admin_only"`
					} `json:"restrictions,omitempty"`
				} `json:"change"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			env := store[uuid]
			if body.Change.Name != "" {
				env.Name = body.Change.Name
			}
			if body.Change.Restrictions != nil {
				env.Restrictions = body.Change.Restrictions
			}
			store[uuid] = env
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(env)
			return
		}

		uuid := rest
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			env, ok := store[uuid]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]string{"message": "not found"}})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(env)
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

func TestAccDeploymentEnvironmentResource(t *testing.T) {
	srv := newMockDeploymentEnvironmentServer(t)
	t.Setenv("BITBUCKET_BASE_URL", srv.URL)
	t.Setenv("BITBUCKET_WORKSPACE", "acc-test-workspace")
	t.Setenv("BITBUCKET_USERNAME", "acc-test-user")
	t.Setenv("BITBUCKET_PASSWORD", "acc-test-pass")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "bitbucket_deployment_environment" "test" {
  repository_slug  = "acc-repo"
  name             = "production"
  environment_type = "Production"
  admin_only       = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_deployment_environment.test", "name", "production"),
					resource.TestCheckResourceAttr("bitbucket_deployment_environment.test", "environment_type", "Production"),
					resource.TestCheckResourceAttr("bitbucket_deployment_environment.test", "admin_only", "true"),
				),
			},
			{
				ResourceName:      "bitbucket_deployment_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDFromAttrs("bitbucket_deployment_environment.test", []string{"repository_slug", "environment_uuid"}, "/"),
			},
			{
				Config: `
resource "bitbucket_deployment_environment" "test" {
  repository_slug  = "acc-repo"
  name             = "production-renamed"
  environment_type = "Production"
  admin_only       = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("bitbucket_deployment_environment.test", "name", "production-renamed"),
					resource.TestCheckResourceAttr("bitbucket_deployment_environment.test", "admin_only", "false"),
				),
			},
		},
	})
}
